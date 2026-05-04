//go:build !windows

package ipc

import (
	"net"
	"os"
	"path/filepath"
)

// SocketPath returns the IPC address for this platform.
func SocketPath() string { return socketPath }

// Dial connects to the daemon unix socket.
func Dial() (net.Conn, error) {
	return net.Dial("unix", socketPath)
}

// Listen creates the daemon unix socket listener.
func Listen() (net.Listener, error) {
	return listenUnix()
}

func listenUnix() (net.Listener, error) {
	dir := filepath.Dir(socketPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	_ = os.Remove(socketPath)

	l, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, err
	}
	// 0666: any local user may connect; the token handshake in server.go
	// is the actual authentication barrier.
	_ = os.Chmod(socketPath, 0666)
	return l, nil
}
