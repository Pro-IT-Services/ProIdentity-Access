package admin

import "testing"

func TestValidateHostRejectsConfigInjection(t *testing.T) {
	bad := []string{
		"vpn.example.com\nDNS = 1.1.1.1",
		"vpn.example.com:51820",
		"vpn.example.com/path",
		"bad host.example",
	}
	for _, value := range bad {
		if _, err := validateHost("endpoint", value); err == nil {
			t.Fatalf("validateHost(%q) succeeded, want error", value)
		}
	}
}

func TestValidateResourceAddressNormalizesCIDR(t *testing.T) {
	mask := 24
	ip, gotMask, err := validateResourceAddress("network", "192.168.100.31", &mask)
	if err != nil {
		t.Fatalf("validateResourceAddress returned error: %v", err)
	}
	if ip != "192.168.100.0" {
		t.Fatalf("ip = %q, want masked network address", ip)
	}
	if gotMask == nil || *gotMask != 24 {
		t.Fatalf("mask = %v, want /24", gotMask)
	}
}

func TestValidateSettingValueRedactedSecretIsIgnored(t *testing.T) {
	got, err := validateSettingValue("push_auth_api_key", configuredSecretMarker)
	if err != nil {
		t.Fatalf("validateSettingValue returned error: %v", err)
	}
	if got != "" {
		t.Fatalf("got %q, want empty value for unchanged redacted secret", got)
	}
}

func TestValidateSettingValueRejectsUnknownSetting(t *testing.T) {
	if _, err := validateSettingValue("unexpected_setting", "1"); err == nil {
		t.Fatal("unknown setting succeeded, want error")
	}
}
