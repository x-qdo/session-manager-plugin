package portsession

import (
	"bytes"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/aws/session-manager-plugin/src/datachannel"
	"github.com/aws/session-manager-plugin/src/log"
	"github.com/aws/session-manager-plugin/src/message"
	"github.com/aws/session-manager-plugin/src/sessionmanagerplugin/session"
)

// testInterceptorOnce provides a single connection via AcquireClientConn, then returns handled=false on subsequent calls.
type testInterceptorOnce struct {
	mu    sync.Mutex
	conn  net.Conn
	used  bool
	close func() error
}

func (t *testInterceptorOnce) AcquireClientConn(l log.T, s *session.Session, p PortParameters) (net.Conn, bool, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.used || t.conn == nil {
		return nil, false, nil
	}
	t.used = true
	return t.conn, true, nil
}
func (t *testInterceptorOnce) AcquireListener(l log.T, s *session.Session, p PortParameters) (net.Listener, bool, error) {
	return nil, false, nil
}
func (t *testInterceptorOnce) Close() error {
	if t.close != nil {
		return t.close()
	}
	return nil
}

// Test that BasicPortForwarding uses an interceptor-provided client connection and does not bind a local port.
func TestBasicPortForwarding_InterceptorProvidesConn(t *testing.T) {
	// Save and restore globals mutated in the test
	origGetNewListener := getNewListener
	origSendMessageCall := datachannel.SendMessageCall
	defer func() {
		getNewListener = origGetNewListener
		datachannel.SendMessageCall = origSendMessageCall
		RegisterPortSessionInterceptor(nil)
	}()

	// Fail the test if the code attempts to open a local listener
	getNewListener = func(listenerType string, listenerAddress string) (net.Listener, error) {
		t.Fatalf("getNewListener should not be called when interceptor provides a connection")
		return nil, nil
	}

	// Pipe between interceptor and test
	pluginSide, testSide := net.Pipe()
	defer pluginSide.Close()
	defer testSide.Close()

	// Register interceptor that provides the plugin-end of the pipe
	ic := &testInterceptorOnce{conn: pluginSide}
	RegisterPortSessionInterceptor(ic)

	// Capture datachannel outbound bytes to verify payload
	var captured []byte
	datachannel.SendMessageCall = func(l log.T, dc *datachannel.DataChannel, input []byte, inputType int) error {
		captured = append([]byte(nil), input...)
		return nil
	}

	// Prepare a mock session and port session with BasicPortForwarding
	mockSession := getSessionMock()
	portSession := PortSession{
		Session:        mockSession,
		portParameters: PortParameters{PortNumber: "5432", Type: "LocalPortForwarding"},
		portSessionType: &BasicPortForwarding{
			session:        mockSession,
			portParameters: PortParameters{PortNumber: "5432", Type: "LocalPortForwarding"},
		},
	}

	// Run SetSessionHandlers in background; it will read from the interceptor-provided conn
	done := make(chan struct{})
	go func() {
		_ = portSession.SetSessionHandlers(mockLog)
		close(done)
	}()

	// Give InitializeStreams time to run and acquire the interceptor connection
	time.Sleep(50 * time.Millisecond)

	// Write a payload from the "client" side into the interceptor pipe and then close it to force reconnect path
	expected := []byte("hello-through-interceptor")
	_, _ = testSide.Write(expected)
	_ = testSide.Close()

	// Wait a bit for ReadStream to forward and for the goroutine to finish (it will error on reconnect with no new conn)
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		// timeout is fine; we only need the message to be captured
	}

	// Validate the captured message contains our payload
	msg := &message.ClientMessage{}
	if err := msg.DeserializeClientMessage(mockLog, captured); err != nil {
		t.Fatalf("failed to deserialize client message: %v", err)
	}
	if !bytes.Equal(expected, msg.Payload) {
		t.Fatalf("forwarded payload mismatch. expected=%q got=%q", string(expected), string(msg.Payload))
	}
}

// Test that BasicPortForwarding attempts interceptor again on reconnect and returns when no new connection is available.
func TestBasicPortForwarding_InterceptorReconnect_NoNewConn(t *testing.T) {
	// Save and restore globals
	origSendMessageCall := datachannel.SendMessageCall
	defer func() {
		datachannel.SendMessageCall = origSendMessageCall
		RegisterPortSessionInterceptor(nil)
	}()

	// Provide an interceptor that yields a single conn and then declines further
	pluginSide, testSide := net.Pipe()

	ic := &testInterceptorOnce{conn: pluginSide}
	RegisterPortSessionInterceptor(ic)

	// Collect any sent data to ensure at least one forward happened
	var sent [][]byte
	datachannel.SendMessageCall = func(l log.T, dc *datachannel.DataChannel, input []byte, inputType int) error {
		cp := append([]byte(nil), input...)
		sent = append(sent, cp)
		return nil
	}

	// Setup session and port session
	mockSession := getSessionMock()
	bf := &BasicPortForwarding{
		sessionId:      "sess-1",
		session:        mockSession,
		portParameters: PortParameters{PortNumber: "3306", Type: "LocalPortForwarding"},
	}
	ps := PortSession{
		Session:         mockSession,
		portParameters:  PortParameters{PortNumber: "3306", Type: "LocalPortForwarding"},
		portSessionType: bf,
	}

	// Start session handlers
	errCh := make(chan error, 1)
	go func() {
		errCh <- ps.SetSessionHandlers(mockLog)
	}()

	// Allow init
	time.Sleep(50 * time.Millisecond)

	// Send one payload then close client to trigger reconnect
	payload := []byte("first")
	_, _ = testSide.Write(payload)
	_ = testSide.Close()

	// Expect SetSessionHandlers to eventually return an error because interceptor returns no new connection
	var err error
	select {
	case err = <-errCh:
	case <-time.After(1 * time.Second):
		t.Fatal("SetSessionHandlers did not return after client disconnect with no new interceptor connection")
	}
	if err == nil {
		t.Fatalf("expected error when interceptor declined to provide a new connection on reconnect")
	}

	// Ensure we forwarded at least one payload via datachannel before failing
	if len(sent) == 0 {
		t.Fatalf("expected at least one forwarded message")
	}
	msg := &message.ClientMessage{}
	if derr := msg.DeserializeClientMessage(mockLog, sent[len(sent)-1]); derr != nil {
		t.Fatalf("failed to deserialize last forwarded message: %v", derr)
	}
	if !bytes.Contains(msg.Payload, []byte("first")) {
		t.Fatalf("last forwarded message did not contain expected payload. got=%q", string(msg.Payload))
	}
}
