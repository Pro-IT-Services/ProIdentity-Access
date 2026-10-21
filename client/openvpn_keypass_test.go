package main

import "testing"

func TestConfigKeyEncrypted(t *testing.T) {
	// PEM headers built from parts so the source holds no key-looking block.
	pem := func(kind string) string { return "-----" + "BEGIN " + kind + "-----" }
	cases := map[string]bool{
		"<key>\n" + pem("ENCRYPTED PRIVATE KEY") + "\nMIIF\n</key>":                                       true,
		"<key>\n" + pem("RSA PRIVATE KEY") + "\nProc-Type: 4,ENCRYPTED\nDEK-Info: AES-256-CBC,AB\n</key>": true,
		"<key>\n" + pem("PRIVATE KEY") + "\nMIIE\n</key>":                                                 false,
		"client\nremote vpn.example.com 1194\n":                                                           false,
	}
	for cfg, want := range cases {
		if got := configKeyEncrypted(cfg); got != want {
			t.Errorf("configKeyEncrypted(%q) = %v, want %v", cfg[:20], got, want)
		}
	}
}
