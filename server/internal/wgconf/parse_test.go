package wgconf

import "testing"

const sample = `[Interface]
PrivateKey = aaaa
Address = 10.66.0.2/32, fd00::2/128
DNS = 10.66.0.1

[Peer]
PublicKey = bbbb
Endpoint = vpn.example.com:51820 # office
AllowedIPs = 10.66.0.0/24
`

func TestParse(t *testing.T) {
	meta, err := Parse(sample)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Address != "10.66.0.2/32, fd00::2/128" || meta.Endpoint != "vpn.example.com:51820" {
		t.Fatalf("got %+v", meta)
	}
}

func TestParseRejects(t *testing.T) {
	cases := map[string]error{
		"client\nremote x 1194\n":                                     ErrNotWireGuard,
		"[Interface]\nAddress = 10.0.0.2/32\n[Peer]\nPublicKey = b\n": ErrNoInterface,
		"[Interface]\nPrivateKey = a\n":                               ErrNoPeer,
	}
	for cfg, want := range cases {
		if _, err := Parse(cfg); err != want {
			t.Errorf("Parse(%q) = %v, want %v", cfg, err, want)
		}
	}
}
