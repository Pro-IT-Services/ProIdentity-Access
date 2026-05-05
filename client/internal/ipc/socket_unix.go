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
	// Any local user may connect; server.go authenticates the OS peer user
	// with socket credentials and isolates all operations by that owner.
	_ = os.Chmod(socketPath, 0666)
	return l, nil
}
