package managed

import "testing"

func TestNormalizeServerURL(t *testing.T) {
	cases := map[string]string{
		"vpn.example.com":           "https://vpn.example.com",
		"  vpn.example.com/  ":      "https://vpn.example.com",
		"vpn.example.com:8443":      "https://vpn.example.com:8443",
		"https://vpn.example.com":   "https://vpn.example.com",
		"https://vpn.example.com//": "https://vpn.example.com",
		"http://localhost:8080":     "http://localhost:8080",
		"":                          "",
		"   ":                       "",
	}
	for in, want := range cases {
		if got := NormalizeServerURL(in); got != want {
			t.Errorf("NormalizeServerURL(%q) = %q, want %q", in, got, want)
		}
	}
}
