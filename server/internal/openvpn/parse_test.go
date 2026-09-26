package openvpn

import "testing"

// New-style single-file profile: inline certs, dev tun, username/password auth.
const newStyle = `
client
dev tun
proto udp
remote vpn.example.com 1194
auth-user-pass
remote-cert-tls server
<ca>
inline-ca-omitted-for-test
</ca>
<cert>
inline-cert-omitted-for-test
</cert>
<key>
inline-key-omitted-for-test
</key>
`

// Old-style profile: external cert files, dev tap, no inline blocks.
const oldStyle = `
# Old TAP profile
client
dev tap0
proto tcp
remote 203.0.113.10 443
ca ca.crt
cert client.crt
key client.key
`

func TestParseNewStyle(t *testing.T) {
	m := Parse(newStyle)
	if m.DevType != "tun" {
		t.Errorf("DevType = %q, want tun", m.DevType)
	}
	if !m.AuthUserPass {
		t.Error("AuthUserPass = false, want true")
	}
	if !m.HasInlineCerts {
		t.Error("HasInlineCerts = false, want true")
	}
}

func TestParseOldStyleTAP(t *testing.T) {
	m := Parse(oldStyle)
	if m.DevType != "tap" {
		t.Errorf("DevType = %q, want tap", m.DevType)
	}
	if m.AuthUserPass {
		t.Error("AuthUserPass = true, want false")
	}
	if m.HasInlineCerts {
		t.Error("HasInlineCerts = true, want false")
	}
}

func TestParseDevTypeOverridesDev(t *testing.T) {
	// "dev-type tap" must win over "dev tun0" regardless of order.
	m := Parse("dev tun0\ndev-type tap\n")
	if m.DevType != "tap" {
		t.Errorf("DevType = %q, want tap (dev-type overrides dev)", m.DevType)
	}
	// A commented directive must be ignored, not parsed.
	m2 := Parse("# dev tap\ndev tun\n")
	if m2.DevType != "tun" {
		t.Errorf("DevType = %q, want tun (comment ignored)", m2.DevType)
	}
}

func TestParseDefaultsToTun(t *testing.T) {
	if m := Parse("client\nproto udp\n"); m.DevType != "tun" {
		t.Errorf("DevType = %q, want tun default", m.DevType)
	}
}
