//go:build !windows

package main

// No native placement outside Windows; the frontend positions the window.
func placeTrayWindow(width, height int) bool { return false }
