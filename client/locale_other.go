//go:build !windows && !darwin

package main

import "os"

// detectSystemLanguage uses the POSIX locale environment.
func detectSystemLanguage() string { return langFromEnv(os.Getenv) }
