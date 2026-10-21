// Package wgconf reads the parts of a WireGuard .conf the admin UI shows and
// checks that an uploaded file is a usable client profile.
package wgconf

import (
	"errors"
	"strings"
)

// Metadata is what an uploaded profile tells about itself. Keys are never
// part of it: they stay inside the encrypted config.
type Metadata struct {
	Address  string // [Interface] Address (comma-separated as written)
	Endpoint string // first [Peer] Endpoint
}

var (
	ErrNoInterface  = errors.New("the file has no [Interface] section with a PrivateKey")
	ErrNoPeer       = errors.New("the file has no [Peer] section with a PublicKey")
	ErrNotWireGuard = errors.New("this doesn't look like a WireGuard configuration")
)

// Parse validates a client config and returns its display metadata.
func Parse(config string) (Metadata, error) {
	var meta Metadata
	section := ""
	sawInterface, sawPrivateKey, sawPeer, sawPeerKey := false, false, false, false
	for _, raw := range strings.Split(config, "\n") {
		line := strings.TrimSpace(raw)
		if i := strings.IndexAny(line, "#;"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			switch section {
			case "interface":
				sawInterface = true
			case "peer":
				sawPeer = true
			}
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		switch {
		case section == "interface" && key == "privatekey" && value != "":
			sawPrivateKey = true
		case section == "interface" && key == "address" && meta.Address == "":
			meta.Address = value
		case section == "peer" && key == "publickey" && value != "":
			sawPeerKey = true
		case section == "peer" && key == "endpoint" && meta.Endpoint == "":
			meta.Endpoint = value
		}
	}
	switch {
	case !sawInterface && !sawPeer:
		return meta, ErrNotWireGuard
	case !sawInterface || !sawPrivateKey:
		return meta, ErrNoInterface
	case !sawPeer || !sawPeerKey:
		return meta, ErrNoPeer
	}
	return meta, nil
}
