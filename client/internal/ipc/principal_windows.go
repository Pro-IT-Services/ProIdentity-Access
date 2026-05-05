//go:build windows

package ipc

import (
	"fmt"
	"net"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var procGetNamedPipeClientProcessID = syscall.NewLazyDLL("kernel32.dll").NewProc("GetNamedPipeClientProcessId")

type fdConn interface {
	Fd() uintptr
}

func PeerPrincipal(conn net.Conn) (Principal, error) {
	fc, ok := conn.(fdConn)
	if !ok {
		return currentUserPrincipal(), nil
	}

	var pid uint32
	r1, _, err := procGetNamedPipeClientProcessID.Call(fc.Fd(), uintptr(unsafe.Pointer(&pid)))
	if r1 == 0 {
		return Principal{}, fmt.Errorf("GetNamedPipeClientProcessId: %w", err)
	}

	proc, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return Principal{}, fmt.Errorf("open client process %d: %w", pid, err)
	}
	defer windows.CloseHandle(proc)

	var token windows.Token
	if err := windows.OpenProcessToken(proc, windows.TOKEN_QUERY, &token); err != nil {
		return Principal{}, fmt.Errorf("open client token %d: %w", pid, err)
	}
	defer token.Close()

	tu, err := token.GetTokenUser()
	if err != nil {
		return Principal{}, fmt.Errorf("read client token user %d: %w", pid, err)
	}

	sid := tu.User.Sid
	username := sid.String()
	if account, domain, _, err := sid.LookupAccount(""); err == nil {
		if domain != "" {
			username = domain + `\` + account
		} else {
			username = account
		}
	}

	return Principal{Platform: "windows", UserID: sid.String(), Username: username}, nil
}
