package secretstore

import "errors"

// ErrNotFound is returned when a named secret has not been stored yet.
var ErrNotFound = errors.New("secret not found")

const (
	ManagedToken           = "managed-token"
	ManagedClientPrivKey   = "managed-client-private-key"
	ManagedServerPublicKey = "managed-server-public-key"
	DaemonConfigKey        = "daemon-config-key"
)
