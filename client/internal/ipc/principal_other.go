//go:build !windows && !linux && !darwin

package ipc

import "net"

func PeerPrincipal(conn net.Conn) (Principal, error) {
	return currentUserPrincipal(), nil
}
