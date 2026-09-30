//go:build darwin

package main

import (
	"context"
	"os"
	"os/exec"
	"time"
)

// detectSystemLanguage uses the first of the user's preferred languages
// (System Settings → General → Language & Region), falling back to the
// POSIX locale environment and then English.
func detectSystemLanguage() string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "/usr/bin/defaults", "read", "-g", "AppleLanguages").Output()
	if err == nil {
		if first := parseAppleLanguages(string(out)); first != "" {
			return langFromTag(first)
		}
	}
	return langFromEnv(os.Getenv)
}
