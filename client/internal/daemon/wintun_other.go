//go:build !windows

package daemon

// EnsureWintun is a no-op on non-Windows platforms.
func EnsureWintun() error { return nil }
