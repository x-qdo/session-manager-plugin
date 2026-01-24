package portsession

import (
	"errors"
	"net"
	"sync"
	"testing"

	"github.com/aws/session-manager-plugin/src/log"
	"github.com/aws/session-manager-plugin/src/sessionmanagerplugin/session"
	"github.com/stretchr/testify/assert"
)

type mockInterceptor struct {
	acquireClientConnCalled bool
	acquireListenerCalled   bool
	closeCalled             bool
	clientConn              net.Conn
	clientConnHandled       bool
	clientConnErr           error
	listener                net.Listener
	listenerHandled         bool
	listenerErr             error
	closeErr                error
}

func (m *mockInterceptor) AcquireClientConn(_ log.T, _ *session.Session, _ PortParameters) (net.Conn, bool, error) {
	m.acquireClientConnCalled = true
	return m.clientConn, m.clientConnHandled, m.clientConnErr
}

func (m *mockInterceptor) AcquireListener(_ log.T, _ *session.Session, _ PortParameters) (net.Listener, bool, error) {
	m.acquireListenerCalled = true
	return m.listener, m.listenerHandled, m.listenerErr
}

func (m *mockInterceptor) Close() error {
	m.closeCalled = true
	return m.closeErr
}

func TestRegisterPortSessionInterceptor(t *testing.T) {
	defer RegisterPortSessionInterceptor(nil)

	assert.Nil(t, GetPortSessionInterceptor())
	assert.False(t, HasPortSessionInterceptor())

	ic := &mockInterceptor{}
	RegisterPortSessionInterceptor(ic)
	assert.Equal(t, ic, GetPortSessionInterceptor())
	assert.True(t, HasPortSessionInterceptor())

	ic2 := &mockInterceptor{}
	RegisterPortSessionInterceptor(ic2)
	assert.Equal(t, ic2, GetPortSessionInterceptor())
	assert.True(t, HasPortSessionInterceptor())

	RegisterPortSessionInterceptor(nil)
	assert.Nil(t, GetPortSessionInterceptor())
	assert.False(t, HasPortSessionInterceptor())
}

func TestGetPortSessionInterceptor_ConcurrentAccess(t *testing.T) {
	defer RegisterPortSessionInterceptor(nil)

	ic := &mockInterceptor{}
	RegisterPortSessionInterceptor(ic)

	var wg sync.WaitGroup
	const goroutines = 100

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got := GetPortSessionInterceptor()
			assert.Equal(t, ic, got)
		}()
	}
	wg.Wait()
}

func TestRegisterPortSessionInterceptor_ConcurrentWriteRead(t *testing.T) {
	defer RegisterPortSessionInterceptor(nil)

	var wg sync.WaitGroup
	const goroutines = 50

	interceptors := make([]*mockInterceptor, goroutines)
	for i := 0; i < goroutines; i++ {
		interceptors[i] = &mockInterceptor{}
	}

	for i := 0; i < goroutines; i++ {
		wg.Add(2)
		go func(idx int) {
			defer wg.Done()
			RegisterPortSessionInterceptor(interceptors[idx])
		}(i)
		go func() {
			defer wg.Done()
			_ = GetPortSessionInterceptor()
			_ = HasPortSessionInterceptor()
		}()
	}
	wg.Wait()
}

func TestNopInterceptor_AcquireClientConn(t *testing.T) {
	nop := NopInterceptor{}
	conn, handled, err := nop.AcquireClientConn(mockLog, nil, PortParameters{})

	assert.Nil(t, conn)
	assert.False(t, handled)
	assert.NoError(t, err)
}

func TestNopInterceptor_AcquireListener(t *testing.T) {
	nop := NopInterceptor{}
	listener, handled, err := nop.AcquireListener(mockLog, nil, PortParameters{})

	assert.Nil(t, listener)
	assert.False(t, handled)
	assert.NoError(t, err)
}

func TestNopInterceptor_Close(t *testing.T) {
	nop := NopInterceptor{}
	err := nop.Close()

	assert.NoError(t, err)
}

func TestMockInterceptor_AcquireClientConn_ReturnsConn(t *testing.T) {
	defer RegisterPortSessionInterceptor(nil)

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	ic := &mockInterceptor{
		clientConn:        clientConn,
		clientConnHandled: true,
	}
	RegisterPortSessionInterceptor(ic)

	gotIc := GetPortSessionInterceptor()
	conn, handled, err := gotIc.AcquireClientConn(mockLog, nil, PortParameters{PortNumber: "5432"})

	assert.True(t, ic.acquireClientConnCalled)
	assert.Equal(t, clientConn, conn)
	assert.True(t, handled)
	assert.NoError(t, err)
}

func TestMockInterceptor_AcquireClientConn_ReturnsError(t *testing.T) {
	defer RegisterPortSessionInterceptor(nil)

	expectedErr := errors.New("connection failed")
	ic := &mockInterceptor{
		clientConnErr: expectedErr,
	}
	RegisterPortSessionInterceptor(ic)

	gotIc := GetPortSessionInterceptor()
	conn, handled, err := gotIc.AcquireClientConn(mockLog, nil, PortParameters{})

	assert.True(t, ic.acquireClientConnCalled)
	assert.Nil(t, conn)
	assert.False(t, handled)
	assert.Equal(t, expectedErr, err)
}

func TestMockInterceptor_AcquireListener_ReturnsListener(t *testing.T) {
	defer RegisterPortSessionInterceptor(nil)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create test listener: %v", err)
	}
	defer listener.Close()

	ic := &mockInterceptor{
		listener:        listener,
		listenerHandled: true,
	}
	RegisterPortSessionInterceptor(ic)

	gotIc := GetPortSessionInterceptor()
	gotListener, handled, err := gotIc.AcquireListener(mockLog, nil, PortParameters{PortNumber: "3306"})

	assert.True(t, ic.acquireListenerCalled)
	assert.Equal(t, listener, gotListener)
	assert.True(t, handled)
	assert.NoError(t, err)
}

func TestMockInterceptor_AcquireListener_ReturnsError(t *testing.T) {
	defer RegisterPortSessionInterceptor(nil)

	expectedErr := errors.New("listener failed")
	ic := &mockInterceptor{
		listenerErr: expectedErr,
	}
	RegisterPortSessionInterceptor(ic)

	gotIc := GetPortSessionInterceptor()
	gotListener, handled, err := gotIc.AcquireListener(mockLog, nil, PortParameters{})

	assert.True(t, ic.acquireListenerCalled)
	assert.Nil(t, gotListener)
	assert.False(t, handled)
	assert.Equal(t, expectedErr, err)
}

func TestMockInterceptor_Close(t *testing.T) {
	defer RegisterPortSessionInterceptor(nil)

	ic := &mockInterceptor{}
	RegisterPortSessionInterceptor(ic)

	gotIc := GetPortSessionInterceptor()
	err := gotIc.Close()

	assert.True(t, ic.closeCalled)
	assert.NoError(t, err)
}

func TestMockInterceptor_CloseWithError(t *testing.T) {
	defer RegisterPortSessionInterceptor(nil)

	expectedErr := errors.New("close failed")
	ic := &mockInterceptor{
		closeErr: expectedErr,
	}
	RegisterPortSessionInterceptor(ic)

	gotIc := GetPortSessionInterceptor()
	err := gotIc.Close()

	assert.True(t, ic.closeCalled)
	assert.Equal(t, expectedErr, err)
}

func TestInterceptor_NotHandled_FallsBack(t *testing.T) {
	defer RegisterPortSessionInterceptor(nil)

	ic := &mockInterceptor{
		clientConnHandled: false,
		listenerHandled:   false,
	}
	RegisterPortSessionInterceptor(ic)

	gotIc := GetPortSessionInterceptor()

	conn, handled, err := gotIc.AcquireClientConn(mockLog, nil, PortParameters{})
	assert.Nil(t, conn)
	assert.False(t, handled)
	assert.NoError(t, err)

	listener, handled, err := gotIc.AcquireListener(mockLog, nil, PortParameters{})
	assert.Nil(t, listener)
	assert.False(t, handled)
	assert.NoError(t, err)
}

func TestInterceptor_PortParameters_PassedCorrectly(t *testing.T) {
	defer RegisterPortSessionInterceptor(nil)

	var capturedParams PortParameters
	ic := &captureParamsInterceptor{
		onAcquireClientConn: func(params PortParameters) {
			capturedParams = params
		},
	}
	RegisterPortSessionInterceptor(ic)

	expectedParams := PortParameters{
		PortNumber:          "5432",
		LocalPortNumber:     "15432",
		LocalUnixSocket:     "/tmp/test.sock",
		LocalConnectionType: "unix",
		Type:                "LocalPortForwarding",
	}

	gotIc := GetPortSessionInterceptor()
	_, _, _ = gotIc.AcquireClientConn(mockLog, nil, expectedParams)

	assert.Equal(t, expectedParams.PortNumber, capturedParams.PortNumber)
	assert.Equal(t, expectedParams.LocalPortNumber, capturedParams.LocalPortNumber)
	assert.Equal(t, expectedParams.LocalUnixSocket, capturedParams.LocalUnixSocket)
	assert.Equal(t, expectedParams.LocalConnectionType, capturedParams.LocalConnectionType)
	assert.Equal(t, expectedParams.Type, capturedParams.Type)
}

type captureParamsInterceptor struct {
	onAcquireClientConn func(params PortParameters)
	onAcquireListener   func(params PortParameters)
}

func (c *captureParamsInterceptor) AcquireClientConn(_ log.T, _ *session.Session, params PortParameters) (net.Conn, bool, error) {
	if c.onAcquireClientConn != nil {
		c.onAcquireClientConn(params)
	}
	return nil, false, nil
}

func (c *captureParamsInterceptor) AcquireListener(_ log.T, _ *session.Session, params PortParameters) (net.Listener, bool, error) {
	if c.onAcquireListener != nil {
		c.onAcquireListener(params)
	}
	return nil, false, nil
}

func (c *captureParamsInterceptor) Close() error { return nil }

func TestInterceptor_Session_PassedCorrectly(t *testing.T) {
	defer RegisterPortSessionInterceptor(nil)

	var capturedSession *session.Session
	ic := &captureSessionInterceptor{
		onAcquireClientConn: func(sess *session.Session) {
			capturedSession = sess
		},
	}
	RegisterPortSessionInterceptor(ic)

	mockSession := getSessionMock()
	gotIc := GetPortSessionInterceptor()
	_, _, _ = gotIc.AcquireClientConn(mockLog, &mockSession, PortParameters{})

	assert.NotNil(t, capturedSession)
	assert.Equal(t, mockSession.SessionId, capturedSession.SessionId)
}

type captureSessionInterceptor struct {
	onAcquireClientConn func(sess *session.Session)
	onAcquireListener   func(sess *session.Session)
}

func (c *captureSessionInterceptor) AcquireClientConn(_ log.T, sess *session.Session, _ PortParameters) (net.Conn, bool, error) {
	if c.onAcquireClientConn != nil {
		c.onAcquireClientConn(sess)
	}
	return nil, false, nil
}

func (c *captureSessionInterceptor) AcquireListener(_ log.T, sess *session.Session, _ PortParameters) (net.Listener, bool, error) {
	if c.onAcquireListener != nil {
		c.onAcquireListener(sess)
	}
	return nil, false, nil
}

func (c *captureSessionInterceptor) Close() error { return nil }
