//go:build linux

package ipc

import (
	"fmt"
	"net"
	"os/user"

	"golang.org/x/sys/unix"
)

func PeerPrincipal(conn net.Conn) (Principal, error) {
	uc, ok := conn.(*net.UnixConn)
	if !ok {
		return currentUserPrincipal(), nil
	}
	raw, err := uc.SyscallConn()
	if err != nil {
		return Principal{}, err
	}
	var cred *unix.Ucred
	var serr error
	if err := raw.Control(func(fd uintptr) {
		cred, serr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	}); err != nil {
		return Principal{}, err
	}
	if serr != nil {
		return Principal{}, serr
	}
	uid := fmt.Sprintf("%d", cred.Uid)
	username := uid
	if u, err := user.LookupId(uid); err == nil && u != nil {
		username = u.Username
	}
	return Principal{Platform: "linux", UserID: uid, Username: username}, nil
}
