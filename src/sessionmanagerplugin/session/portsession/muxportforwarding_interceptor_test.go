package portsession

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/aws/session-manager-plugin/src/log"
	"github.com/aws/session-manager-plugin/src/sessionmanagerplugin/session"
	"github.com/stretchr/testify/assert"
	"github.com/xtaci/smux"
)

// chanListener is a simple in-memory listener that returns connections pushed into a channel.
type chanListener struct {
	ch     chan net.Conn
	closed bool
	mu     sync.Mutex
}

func newChanListener() *chanListener {
	return &chanListener{ch: make(chan net.Conn, 1)}
}

func (l *chanListener) Accept() (net.Conn, error) {
	c, ok := <-l.ch
	if !ok {
		return nil, errors.New("listener closed")
	}
	return c, nil
}

func (l *chanListener) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.closed {
		close(l.ch)
		l.closed = true
	}
	return nil
}

func (l *chanListener) Addr() net.Addr { return dummyAddr("interceptor-listener") }

type dummyAddr string

func (d dummyAddr) Network() string { return "interceptor" }
func (d dummyAddr) String() string  { return string(d) }

// testMuxInterceptor provides a custom listener via AcquireListener.
type testMuxInterceptor struct {
	l     net.Listener
	err   error
	calls int
}

func (t *testMuxInterceptor) AcquireClientConn(_log log.T, _sess *session.Session, _params PortParameters) (net.Conn, bool, error) {
	// Not used in mux tests
	return nil, false, nil
}
func (t *testMuxInterceptor) AcquireListener(_log log.T, _sess *session.Session, _params PortParameters) (net.Listener, bool, error) {
	t.calls++
	if t.err != nil {
		return nil, false, t.err
	}
	return t.l, true, nil
}
func (t *testMuxInterceptor) Close() error { return nil }

// Test that handleClientConnections uses the interceptor-provided listener and bridges
// traffic bidirectionally to a smux stream.
func TestMuxPortForwarding_InterceptorProvidesListener_TrafficFlows(t *testing.T) {
	// Save/restore interceptor
	defer RegisterPortSessionInterceptor(nil)

	// Create an in-memory listener we control
	l := newChanListener()
	ic := &testMuxInterceptor{l: l}
	RegisterPortSessionInterceptor(ic)

	// Create a smux server/client pair over an in-memory pipe
	srvSide, cliSide := net.Pipe()
	defer srvSide.Close()
	defer cliSide.Close()

	srvSess, err := smux.Server(srvSide, smux.DefaultConfig())
	if err != nil {
		t.Fatalf("failed to start smux server: %v", err)
	}
	defer srvSess.Close()

	cliSess, err := smux.Client(cliSide, smux.DefaultConfig())
	if err != nil {
		t.Fatalf("failed to start smux client: %v", err)
	}
	defer cliSess.Close()

	// Build MuxPortForwarding with client session pre-initialized
	mpf := &MuxPortForwarding{
		sessionId:      "mux-sess-1",
		session:        getSessionMock(),
		portParameters: PortParameters{PortNumber: "1234", Type: "LocalPortForwarding"},
		muxClient:      &MuxClient{conn: cliSide, session: cliSess},
	}

	// Start handleClientConnections
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = mpf.handleClientConnections(mockLog, ctx)
	}()

	// Provide one accepted connection via the interceptor listener
	clientConn, pluginConn := net.Pipe()
	defer clientConn.Close()
	defer pluginConn.Close()

	select {
	case l.ch <- pluginConn:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout pushing connection into interceptor listener")
	}

	// On the server side, accept the stream opened by handleClientConnections
	var srvStream *smux.Stream
	acceptDone := make(chan struct{})
	go func() {
		defer close(acceptDone)
		s, aerr := srvSess.AcceptStream()
		if aerr != nil {
			t.Logf("AcceptStream error: %v", aerr)
			return
		}
		srvStream = s
	}()

	select {
	case <-acceptDone:
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for server to accept smux stream")
	}
	if srvStream == nil {
		t.Fatal("server stream is nil")
	}
	defer srvStream.Close()

	// Test client -> server direction
	c2s := []byte("hello-from-client")
	if _, err := clientConn.Write(c2s); err != nil {
		t.Fatalf("write client->plugin failed: %v", err)
	}

	readBuf := make([]byte, len(c2s))
	if _, err := io.ReadFull(srvStream, readBuf); err != nil {
		t.Fatalf("read server stream failed: %v", err)
	}
	assert.Equal(t, c2s, readBuf, "payload from client should arrive on server stream")

	// Test server -> client direction
	s2c := []byte("hello-from-server")
	if _, err := srvStream.Write(s2c); err != nil {
		t.Fatalf("write server->stream failed: %v", err)
	}
	readBuf2 := make([]byte, len(s2c))
	if _, err := io.ReadFull(clientConn, readBuf2); err != nil {
		t.Fatalf("read client side failed: %v", err)
	}
	assert.Equal(t, s2c, readBuf2, "payload from server stream should arrive on client connection")

	// Validate interceptor was called
	assert.GreaterOrEqual(t, ic.calls, 1, "AcquireListener should be called at least once")
}

// Test that an interceptor error is propagated by handleClientConnections.
func TestMuxPortForwarding_InterceptorError(t *testing.T) {
	defer RegisterPortSessionInterceptor(nil)

	ic := &testMuxInterceptor{err: errors.New("boom")}
	RegisterPortSessionInterceptor(ic)

	mpf := &MuxPortForwarding{
		sessionId:      "mux-sess-err",
		session:        getSessionMock(),
		portParameters: PortParameters{PortNumber: "1234", Type: "LocalPortForwarding"},
		muxClient:      &MuxClient{}, // not used due to early error
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := mpf.handleClientConnections(mockLog, ctx)
	assert.EqualError(t, err, "boom")
}
