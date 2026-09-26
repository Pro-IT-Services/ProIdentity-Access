// Package openvpn provides lightweight metadata extraction from .ovpn profiles.
//
// It intentionally does NOT fully validate a config — OpenVPN itself validates
// at connect time. We only extract the few facts the server and clients need to
// present the right UI and pick the right adapter type: the device type
// (tun/tap), whether the profile authenticates with a username/password, and
// whether it embeds inline certificates (new-style single-file profiles).
package openvpn

import (
	"bufio"
	"strings"
)

// Metadata is the subset of a .ovpn profile the product needs.
type Metadata struct {
	DevType        string // "tun" or "tap"
	AuthUserPass   bool   // config has an auth-user-pass directive
	HasInlineCerts bool   // config embeds <ca>/<cert>/<key> blocks (new-style)
}

// Parse extracts Metadata from a .ovpn config. It handles both old-style
// profiles (external ca/cert/key files, "dev tap0") and new-style single-file
// profiles (inline <ca>…</ca> blocks, "dev tun"). Comments (# and ;) and the
// contents of inline blocks are ignored. Per OpenVPN semantics, an explicit
// "dev-type" directive overrides the type inferred from "dev".
func Parse(config string) Metadata {
	m := Metadata{DevType: "tun"}

	var devFromDev, devFromType string
	inInline := false

	sc := bufio.NewScanner(strings.NewReader(config))
	sc.Buffer(make([]byte, 0, 64*1024), 4<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		// Inline blocks like <ca> … </ca>.
		if strings.HasPrefix(line, "</") {
			inInline = false
			continue
		}
		if strings.HasPrefix(line, "<") {
			tag := strings.ToLower(strings.Trim(line, "<>"))
			if tag == "ca" || tag == "cert" || tag == "key" {
				m.HasInlineCerts = true
			}
			inInline = true
			continue
		}
		if inInline {
			continue
		}
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch strings.ToLower(fields[0]) {
		case "dev":
			if len(fields) > 1 {
				v := strings.ToLower(fields[1])
				switch {
				case strings.HasPrefix(v, "tap"):
					devFromDev = "tap"
				case strings.HasPrefix(v, "tun"):
					devFromDev = "tun"
				}
			}
		case "dev-type":
			if len(fields) > 1 {
				switch strings.ToLower(fields[1]) {
				case "tap":
					devFromType = "tap"
				case "tun":
					devFromType = "tun"
				}
			}
		case "auth-user-pass":
			m.AuthUserPass = true
		}
	}

	switch {
	case devFromType != "":
		m.DevType = devFromType
	case devFromDev != "":
		m.DevType = devFromDev
	}
	return m
}
