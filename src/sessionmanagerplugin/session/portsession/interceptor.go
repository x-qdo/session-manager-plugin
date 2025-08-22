package portsession

// Pluggable interceptor for port sessions.
//
// This package-level hook lets an embedding project take control of how
// a port session obtains its "client" connection, so you can avoid opening
// a local TCP listener and instead inject your own net.Conn implementation.
// This enables protocol-aware MITM/proxy/recording without modifying the
// core port session logic.
//
// Intended usage (by embedding project):
//  1) Implement PortSessionInterceptor.
//  2) Call RegisterPortSessionInterceptor() during program init.
//  3) Update Basic/Mux port session code to consult GetPortSessionInterceptor()
//     before opening any local listener. If the interceptor returns (conn, true, nil),
//     use the provided conn as the local stream and skip creating a local listener.
//  4) Optionally, for multiplexing scenarios, provide a custom listener via
//     AcquireListener() so the port session can accept multiple connections without
//     binding a public/local port.
//
// Minimal integration points to add in BasicPortForwarding/MuxPortForwarding:
//   if ic := GetPortSessionInterceptor(); ic != nil {
//       if conn, ok, err := ic.AcquireClientConn(log, &p.session, p.portParameters); err != nil {
//           return err
//       } else if ok {
//           p.stream = &conn
//           // skip starting the local listener
//       }
//   }
//
// For MuxPortForwarding:
//   if ic := GetPortSessionInterceptor(); ic != nil {
//       if l, ok, err := ic.AcquireListener(log, &p.session, p.portParameters); err != nil {
//           return err
//       } else if ok {
//           listener = l
//           // skip default listener (tcp/unix socket)
//       }
//   }
//
// Notes:
// - This file does not implement any concrete MITM or SQL parsing logic.
//   It only defines the interfaces and global registration points.
// - The embedding project can provide in-memory net.Conn/net.Listener
//   implementations (e.g., via net.Pipe or custom types) and perform any
//   desired interception or proxying elsewhere.

import (
	"net"
	"sync"

	"github.com/aws/session-manager-plugin/src/log"
	"github.com/aws/session-manager-plugin/src/sessionmanagerplugin/session"
)

// PortSessionInterceptor allows an embedding project to:
// - Provide a pre-established client-side net.Conn to use instead of opening a local listener.
// - Provide a net.Listener to accept multiple connections (for mux scenarios) without exposing a local port.
// - Perform cleanup when the port session ends.
type PortSessionInterceptor interface {
	// AcquireClientConn returns:
	// - conn: a pre-established "client" connection that the port session should use as its stream.
	// - handled: true if the interceptor is providing the connection and the caller should skip local listener setup.
	// - err: any error encountered while preparing the conn.
	//
	// For BasicPortForwarding, this allows completely bypassing the local TCP listener by supplying a custom net.Conn.
	AcquireClientConn(log log.T, sess *session.Session, params PortParameters) (conn net.Conn, handled bool, err error)

	// AcquireListener returns:
	// - l: a net.Listener the port session should use for accepting connections (e.g., for mux port sessions).
	// - handled: true if the interceptor is providing the listener and the caller should skip default listener setup.
	// - err: any error encountered while preparing the listener.
	//
	// For MuxPortForwarding, this allows replacing the default local TCP/unix listener with a custom in-memory or
	// otherwise controlled listener.
	AcquireListener(log log.T, sess *session.Session, params PortParameters) (l net.Listener, handled bool, err error)

	// Close gives the interceptor a chance to release resources when the port session ends.
	Close() error
}

var (
	interceptorMu sync.RWMutex
	interceptor   PortSessionInterceptor
)

// RegisterPortSessionInterceptor sets a global interceptor used by port sessions.
// Call this once at process initialization from the embedding project.
// Passing nil clears any existing interceptor.
func RegisterPortSessionInterceptor(i PortSessionInterceptor) {
	interceptorMu.Lock()
	defer interceptorMu.Unlock()
	interceptor = i
}

// GetPortSessionInterceptor returns the currently registered interceptor, or nil if none.
func GetPortSessionInterceptor() PortSessionInterceptor {
	interceptorMu.RLock()
	defer interceptorMu.RUnlock()
	return interceptor
}

// HasPortSessionInterceptor reports whether an interceptor is registered.
func HasPortSessionInterceptor() bool {
	return GetPortSessionInterceptor() != nil
}

// NopInterceptor is a no-op implementation that never handles interception.
// Useful as a placeholder in tests or when you want to explicitly disable interception.
type NopInterceptor struct{}

// AcquireClientConn (Nop) returns handled=false.
func (NopInterceptor) AcquireClientConn(log log.T, sess *session.Session, params PortParameters) (net.Conn, bool, error) {
	return nil, false, nil
}

// AcquireListener (Nop) returns handled=false.
func (NopInterceptor) AcquireListener(log log.T, sess *session.Session, params PortParameters) (net.Listener, bool, error) {
	return nil, false, nil
}

// Close (Nop) does nothing.
func (NopInterceptor) Close() error { return nil }
