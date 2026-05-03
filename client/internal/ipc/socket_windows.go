//go:build windows

package ipc

import (
	"net"
	"time"

	"github.com/Microsoft/go-winio"
)

// SocketPath returns the IPC address for this platform.
func SocketPath() string { return pipeName }

// Dial connects to the daemon named pipe.
func Dial() (net.Conn, error) {
	return winio.DialPipe(pipeName, durationPtr(5*time.Second))
}

// Listen creates the daemon named pipe listener.
func Listen() (net.Listener, error) {
	return listenWindowsPipe()
}

func listenWindowsPipe() (net.Listener, error) {
	cfg := &winio.PipeConfig{
		// SY = Local System, BA = Built-in Administrators, IU = Interactive Users.
		// IU covers the logged-in user even when the daemon runs as SYSTEM (service).
		// Actual caller identity is verified by the token handshake in server.go.
		SecurityDescriptor: "D:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;GA;;;IU)",
		MessageMode:        false,
		InputBufferSize:    65536,
		OutputBufferSize:   65536,
	}
	return winio.ListenPipe(pipeName, cfg)
}

func durationPtr(d time.Duration) *time.Duration { return &d }
