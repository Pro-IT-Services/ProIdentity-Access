package firewall

import (
	"reflect"
	"strings"
	"testing"
)

func TestParsePortSpecsConvertsHyphenRangesForIptables(t *testing.T) {
	got, err := parsePortSpecs("ports: 1-3387,3390-65535")
	if err != nil {
		t.Fatalf("parsePortSpecs returned error: %v", err)
	}
	want := []string{"1:3387", "3390:65535"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parsePortSpecs() = %#v, want %#v", got, want)
	}
}

func TestParsePortSpecsAcceptsSinglesAndColonRanges(t *testing.T) {
	got, err := parsePortSpecs(" 22, 80, 1000:2000 ")
	if err != nil {
		t.Fatalf("parsePortSpecs returned error: %v", err)
	}
	want := []string{"22", "80", "1000:2000"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parsePortSpecs() = %#v, want %#v", got, want)
	}
}

func TestParsePortSpecsRejectsInvalidPorts(t *testing.T) {
	cases := []string{
		"0",
		"65536",
		"3389-1",
		"abc",
		"1-2-3",
	}
	for _, tc := range cases {
		if _, err := parsePortSpecs(tc); err == nil {
			t.Fatalf("parsePortSpecs(%q) returned nil error", tc)
		}
	}
}

func TestParsePortSpecsRejectsEmptyList(t *testing.T) {
	_, err := parsePortSpecs(" , ")
	if err == nil {
		t.Fatal("parsePortSpecs returned nil error")
	}
	if !strings.Contains(err.Error(), "empty port list") {
		t.Fatalf("parsePortSpecs error = %q, want empty port list", err.Error())
	}
}

func TestIptablesCommandIncludesExactRule(t *testing.T) {
	got := iptablesCommand("-A", "filter", "WG-VPN",
		"-s", "192.168.212.10/32",
		"-d", "192.168.100.32/32",
		"-p", "tcp",
		"--dport", "53",
		"-j", "ACCEPT",
	)
	want := "iptables -t filter -A WG-VPN -s 192.168.212.10/32 -d 192.168.100.32/32 -p tcp --dport 53 -j ACCEPT"
	if got != want {
		t.Fatalf("iptablesCommand() = %q, want %q", got, want)
	}
}
