package admin

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func sampleReportData() ([]*accessUser, []historySession, reportMeta) {
	loc, _ := time.LoadLocation("Europe/Bratislava")
	seen := time.Date(2026, 10, 25, 14, 30, 0, 0, time.UTC)
	mask := 24
	users := []*accessUser{
		{
			ID: "u1", Username: "jana", Email: "jana.novakova@example.com", Name: "Jana Nováková",
			Active: true, TOTP: true, Roles: []string{"Účtovníctvo"},
			Servers: []*accessServer{{Name: "Pobočka Žilina", Subnet: "10.20.0.0/24", Active: true, Bundles: []accessBundle{
				{Name: "Kancelária", Resources: []accessResource{
					{Name: "Súborový server", Address: "10.20.0.5", Ports: "445"},
					{Name: "Tlačiarne", Address: resourceAddress("10.20.9.0", "network", &mask)},
				}},
			}}},
			OpenVPN:   []accessProfile{{Name: "Bytový podnik Námestovo", Detail: "TUN, TOTP"}},
			WireGuard: []accessProfile{{Name: "PTH", Detail: "shared, 10.180.2.6/32 via vpn.example.com:51820"}},
			Devices:   []accessDevice{{Name: "JANA-LAPTOP", Active: true, LastSeen: &seen}},
		},
		{ID: "u2", Username: "ödön", Email: "odon@example.com", Name: "Ödön Kőszegi", Active: false},
		{ID: "u3", Username: "admin", Email: "admin@example.com", Admin: true, Active: true, TOTP: true},
	}
	t1 := time.Date(2026, 10, 20, 7, 58, 0, 0, time.UTC)
	t2 := t1.Add(3*time.Hour + 12*time.Minute)
	t3 := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	sessions := []historySession{
		{ID: "s1", Username: "jana", Email: "jana.novakova@example.com", Server: "Pobočka Žilina", Device: "JANA-LAPTOP",
			SourceIP: "203.0.113.7", VPNIP: "10.20.0.37", Reason: "requested", Start: &t1, End: &t2},
		{ID: "s2", Username: "admin", Server: "Pobočka Žilina", Device: "ADMIN-PC", SourceIP: "198.51.100.2", VPNIP: "10.20.0.2", Start: &t3},
	}
	meta := reportMeta{VPNName: "ProIdentity Test", GeneratedAt: time.Date(2026, 10, 26, 18, 0, 0, 0, loc), GeneratedBy: "admin", Location: loc}
	return users, sessions, meta
}

func TestReportsRender(t *testing.T) {
	users, sessions, meta := sampleReportData()

	md := string(accessMarkdown(users, meta))
	for _, want := range []string{"## Jana Nováková (jana)", "Súborový server `10.20.0.5`, ports 445", "`10.20.9.0/24`", "No access assigned.", "PTH (shared"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown misses %q", want)
		}
	}

	csv := accessCSV(users, meta)
	if !bytes.HasPrefix(csv, []byte{0xEF, 0xBB, 0xBF}) || !bytes.Contains(csv, []byte("jana;Jana Nováková;")) {
		t.Errorf("access CSV: missing BOM or ';' fields: %q", csv[:80])
	}
	if !bytes.Contains(csv, []byte(";none;")) {
		t.Errorf("access CSV: user without access has no 'none' row")
	}

	hcsv := string(historyCSV(sessions, meta))
	// 07:58 UTC is 08:58 in Bratislava (CET, after the October change).
	if !strings.Contains(hcsv, "2026-10-20 09:58:00;2026-10-20 13:10:00;192.0;3 h 12 min;jana") {
		t.Errorf("history CSV row wrong:\n%s", hcsv)
	}
	if !strings.Contains(hcsv, "still connected") {
		t.Errorf("open session not marked")
	}

	apdf, err := accessPDF(users, meta)
	if err != nil || !bytes.HasPrefix(apdf, []byte("%PDF")) {
		t.Fatalf("access PDF: %v", err)
	}
	hpdf, err := historyPDF(sessions, meta, "2026-10-01", "2026-10-26", "", false)
	if err != nil || !bytes.HasPrefix(hpdf, []byte("%PDF")) {
		t.Fatalf("history PDF: %v", err)
	}
	// REPORT_OUT=dir writes the samples for a visual check.
	if dir := os.Getenv("REPORT_OUT"); dir != "" {
		_ = os.WriteFile(filepath.Join(dir, "access.pdf"), apdf, 0o644)
		_ = os.WriteFile(filepath.Join(dir, "history.pdf"), hpdf, 0o644)
		_ = os.WriteFile(filepath.Join(dir, "access.md"), []byte(md), 0o644)
		_ = os.WriteFile(filepath.Join(dir, "access.csv"), csv, 0o644)
		_ = os.WriteFile(filepath.Join(dir, "history.csv"), []byte(hcsv), 0o644)
	}
}
