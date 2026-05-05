package ipc

import (
	"os/user"
	"runtime"
	"strings"
)

const LegacyOwnerID = "legacy"

// Principal is the OS-authenticated local user behind an IPC connection.
type Principal struct {
	Platform string
	UserID   string
	Username string
}

func (p Principal) Valid() bool {
	return strings.TrimSpace(p.UserID) != ""
}

func (p Principal) IsLegacy() bool {
	return p.UserID == LegacyOwnerID
}

func LegacyPrincipal() Principal {
	return Principal{Platform: runtime.GOOS, UserID: LegacyOwnerID, Username: LegacyOwnerID}
}

func currentUserPrincipal() Principal {
	u, err := user.Current()
	if err != nil || u == nil || u.Uid == "" {
		return LegacyPrincipal()
	}
	name := u.Username
	if name == "" {
		name = u.Name
	}
	return Principal{Platform: runtime.GOOS, UserID: u.Uid, Username: name}
}
