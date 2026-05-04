package ipc

const (
	pipeName   = `\\.\pipe\proidentity`
	socketPath = "/var/run/proidentity.sock"
)

// Dial and Listen are implemented in socket_windows.go / socket_unix.go.
