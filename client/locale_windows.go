//go:build windows

package main

import "golang.org/x/sys/windows"

var procGetUserDefaultUILanguage = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetUserDefaultUILanguage")

// detectSystemLanguage reads the user's Windows display language.
func detectSystemLanguage() string {
	if err := procGetUserDefaultUILanguage.Find(); err != nil {
		return langEN
	}
	r, _, _ := procGetUserDefaultUILanguage.Call()
	return langFromLANGID(uint16(r))
}
