package portsession

import (
	"net"
	"testing"

	"github.com/aws/session-manager-plugin/src/log"
	"github.com/aws/session-manager-plugin/src/message"
	"github.com/aws/session-manager-plugin/src/sessionmanagerplugin/session"
)

// closingInterceptor implements PortSessionInterceptor and tracks Close invocation.
type closingInterceptor struct {
	closed bool
}

func (c *closingInterceptor) AcquireClientConn(_ log.T, _ *session.Session, _ PortParameters) (net.Conn, bool, error) {
	return nil, false, nil
}

func (c *closingInterceptor) AcquireListener(_ log.T, _ *session.Session, _ PortParameters) (net.Listener, bool, error) {
	return nil, false, nil
}

func (c *closingInterceptor) Close() error {
	c.closed = true
	return nil
}

// fakePortType implements IPortSession without side effects (no os.Exit).
type fakePortType struct{}

func (f *fakePortType) IsStreamNotSet() bool                      { return true }
func (f *fakePortType) InitializeStreams(_ log.T, _ string) error { return nil }
func (f *fakePortType) ReadStream(_ log.T) error                  { return nil }
func (f *fakePortType) WriteStream(_ message.ClientMessage) error { return nil }
func (f *fakePortType) Stop()                                     {}

func TestPortSessionStopInvokesInterceptorClose(t *testing.T) {
	// Register a tracking interceptor and ensure it's cleaned up after the test.
	ic := &closingInterceptor{}
	RegisterPortSessionInterceptor(ic)
	defer RegisterPortSessionInterceptor(nil)

	ps := &PortSession{
		portSessionType: &fakePortType{},
	}

	ps.Stop()

	if !ic.closed {
		t.Fatalf("expected interceptor Close to be called on PortSession.Stop")
	}
}
