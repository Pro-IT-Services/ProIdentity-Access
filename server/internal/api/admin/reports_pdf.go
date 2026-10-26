package admin

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/go-pdf/fpdf"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/goregular"
)

// PDF layout for the reports. The Go fonts are built in (BSD licensed) and
// cover Central European letters, so names print correctly without any
// system font.

type rgb struct{ r, g, b int }

var (
	colText    = rgb{17, 24, 39}
	colMuted   = rgb{107, 114, 128}
	colPrimary = rgb{37, 99, 235}
	colRule    = rgb{229, 231, 235}
	colStripe  = rgb{243, 244, 246}
	colWarn    = rgb{180, 83, 9}
)

type pdfDoc struct {
	*fpdf.Fpdf
	title string
}

func newPDF(orientation, title string, meta reportMeta) *pdfDoc {
	p := fpdf.New(orientation, "mm", "A4", "")
	p.AddUTF8FontFromBytes("go", "", goregular.TTF)
	p.AddUTF8FontFromBytes("go", "B", gobold.TTF)
	p.AddUTF8FontFromBytes("mono", "", gomono.TTF)
	p.SetTitle(title, true)
	p.SetAuthor(meta.VPNName, true)
	p.SetCreator("ProIdentity Access", true)
	p.SetMargins(14, 14, 14)
	p.SetAutoPageBreak(true, 16)
	p.AliasNbPages("{nb}")
	d := &pdfDoc{Fpdf: p, title: title}
	p.SetFooterFunc(func() {
		p.SetY(-11)
		d.font("", 7.5, colMuted)
		p.CellFormat(0, 4, fmt.Sprintf("%s  ·  %s  ·  Page %d of {nb}", meta.VPNName, title, p.PageNo()), "", 0, "C", false, 0, "")
	})
	p.AddPage()
	d.font("B", 17, colText)
	p.CellFormat(0, 8, title, "", 1, "L", false, 0, "")
	d.font("", 9, colMuted)
	p.CellFormat(0, 5, fmt.Sprintf("%s  ·  generated %s by %s", meta.VPNName, meta.stamp(), meta.GeneratedBy), "", 1, "L", false, 0, "")
	p.Ln(3)
	d.rule()
	return d
}

func (d *pdfDoc) font(style string, size float64, c rgb) {
	d.SetFont("go", style, size)
	d.SetTextColor(c.r, c.g, c.b)
}

func (d *pdfDoc) mono(size float64, c rgb) {
	d.SetFont("mono", "", size)
	d.SetTextColor(c.r, c.g, c.b)
}

func (d *pdfDoc) rule() {
	l, _, r, _ := d.GetMargins()
	w, _ := d.GetPageSize()
	d.SetDrawColor(colRule.r, colRule.g, colRule.b)
	d.SetLineWidth(0.3)
	y := d.GetY()
	d.Line(l, y, w-r, y)
	d.Ln(3)
}

func (d *pdfDoc) contentWidth() float64 {
	l, _, r, _ := d.GetMargins()
	w, _ := d.GetPageSize()
	return w - l - r
}

// ensure starts a new page when less than h mm are left.
func (d *pdfDoc) ensure(h float64) bool {
	_, ph := d.GetPageSize()
	_, _, _, bottom := d.GetMargins()
	if d.GetY()+h > ph-bottom {
		d.AddPage()
		return true
	}
	return false
}

// fit shortens s with an ellipsis to fit width w at the current font.
func (d *pdfDoc) fit(s string, w float64) string {
	if d.GetStringWidth(s) <= w {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && d.GetStringWidth(string(r)+"…") > w {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}

// line writes one indented line; label in bold, text normal, optional mono tail.
func (d *pdfDoc) line(indent float64, label, text, monoTail string) {
	d.ensure(5)
	l, _, _, _ := d.GetMargins()
	d.SetX(l + indent)
	if label != "" {
		d.font("B", 9, colText)
		d.CellFormat(d.GetStringWidth(label)+1.5, 4.8, label, "", 0, "L", false, 0, "")
	}
	if text != "" {
		d.font("", 9, colText)
		d.CellFormat(d.GetStringWidth(text)+1.5, 4.8, text, "", 0, "L", false, 0, "")
	}
	if monoTail != "" {
		d.mono(8.5, colMuted)
		rest := d.contentWidth() - (d.GetX() - l)
		d.CellFormat(rest, 4.8, d.fit(monoTail, rest), "", 0, "L", false, 0, "")
	}
	d.Ln(4.8)
}

func (d *pdfDoc) heading(text string, size float64) {
	d.font("B", size, colPrimary)
	d.CellFormat(0, size*0.55, text, "", 1, "L", false, 0, "")
	d.Ln(0.8)
}

func (d *pdfDoc) bytes() ([]byte, error) {
	var buf bytes.Buffer
	if err := d.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ── Access report ─────────────────────────────────────────────────────────

func accessPDF(users []*accessUser, meta reportMeta) ([]byte, error) {
	d := newPDF("P", "Access report", meta)
	active, withAccess := 0, 0
	for _, u := range users {
		if u.Active {
			active++
		}
		if u.hasAccess() {
			withAccess++
		}
	}
	d.font("", 9.5, colText)
	d.MultiCell(0, 5, fmt.Sprintf("%d users: %d active, %d with VPN access assigned. "+
		"Access is what each person can reach: ProIdentity servers with the bundles and resources assigned on each, "+
		"plus assigned OpenVPN and WireGuard profiles.", len(users), active, withAccess), "", "L", false)
	d.Ln(3)

	for _, u := range users {
		// Keep a user's header with at least a few lines of their access.
		d.ensure(28)
		d.SetFillColor(colStripe.r, colStripe.g, colStripe.b)
		d.font("B", 11.5, colText)
		d.CellFormat(0, 7.5, " "+u.displayName(), "", 1, "L", true, 0, "")
		facts := []string{u.Username, u.Email, status(u.Active)}
		if u.Admin {
			facts = append(facts, "admin")
		}
		if u.TOTP {
			facts = append(facts, "two-factor on")
		} else {
			facts = append(facts, "two-factor off")
		}
		if len(u.Roles) > 0 {
			facts = append(facts, "roles: "+strings.Join(u.Roles, ", "))
		}
		d.font("", 8.5, colMuted)
		d.MultiCell(0, 4.5, " "+strings.Join(facts, "  ·  "), "", "L", false)
		d.Ln(1.5)

		if !u.hasAccess() {
			d.font("", 9, colWarn)
			d.CellFormat(0, 5, "   No access assigned.", "", 1, "L", false, 0, "")
		}
		if len(u.Servers) > 0 {
			d.heading("ProIdentity servers", 9.5)
			for _, s := range u.Servers {
				note := ""
				if !s.Active {
					note = " (server disabled)"
				}
				d.line(2, s.Name+note, "", s.Subnet)
				if len(s.Bundles) == 0 {
					d.font("", 9, colMuted)
					d.line(8, "", "No bundles", "")
				}
				for _, b := range s.Bundles {
					d.line(8, b.Name, "", "")
					for _, res := range b.Resources {
						tail := res.Address
						if res.Ports != "" {
							tail += "  ports " + res.Ports
						}
						d.line(14, "", res.Name, tail)
					}
				}
			}
			d.Ln(1)
		}
		for _, section := range []struct {
			title string
			list  []accessProfile
		}{{"OpenVPN profiles", u.OpenVPN}, {"WireGuard profiles", u.WireGuard}} {
			if len(section.list) == 0 {
				continue
			}
			d.ensure(10)
			d.heading(section.title, 9.5)
			for _, p := range section.list {
				d.line(2, p.Name, "", p.Detail)
			}
			d.Ln(1)
		}
		if len(u.Devices) > 0 {
			d.ensure(10)
			d.heading("Devices", 9.5)
			for _, dev := range u.Devices {
				d.line(2, dev.Name, "", status(dev.Active)+", last seen "+lastSeen(dev.LastSeen, meta.Location))
			}
		}
		d.Ln(3)
	}
	return d.bytes()
}

// ── Connection history ────────────────────────────────────────────────────

type pdfColumn struct {
	title string
	width float64 // mm
	mono  bool
}

func (d *pdfDoc) tableHeader(cols []pdfColumn) {
	d.SetFillColor(colPrimary.r, colPrimary.g, colPrimary.b)
	d.SetTextColor(255, 255, 255)
	d.SetFont("go", "B", 8)
	for _, c := range cols {
		d.CellFormat(c.width, 6.5, " "+c.title, "", 0, "L", true, 0, "")
	}
	d.Ln(-1)
}

func (d *pdfDoc) tableRow(cols []pdfColumn, values []string, stripe bool) {
	if d.ensure(5.5) {
		d.tableHeader(cols)
	}
	d.SetFillColor(colStripe.r, colStripe.g, colStripe.b)
	for i, c := range cols {
		if c.mono {
			d.mono(7.5, colText)
		} else {
			d.font("", 8, colText)
		}
		d.CellFormat(c.width, 5.5, " "+d.fit(values[i], c.width-2), "", 0, "L", stripe, 0, "")
	}
	d.Ln(-1)
}

func historyPDF(sessions []historySession, meta reportMeta, from, to, filter string, truncated bool) ([]byte, error) {
	d := newPDF("L", "Connection history", meta)
	byUser, total := summarize(sessions)
	period := from + " to " + to
	if filter != "" {
		period += "  ·  " + filter
	}
	d.font("", 9.5, colText)
	d.MultiCell(0, 5, fmt.Sprintf("Period %s (%s). %d sessions by %d users, %s connected in total.",
		period, meta.Location, len(sessions), len(byUser), orDash(fmtDuration(total))), "", "L", false)
	if truncated {
		d.font("", 9, colWarn)
		d.MultiCell(0, 5, fmt.Sprintf("Only the first %d sessions are included. Choose a shorter period or filter by user or server.", maxReportSessions), "", "L", false)
	}
	d.Ln(3)

	if len(sessions) == 0 {
		d.font("", 10, colMuted)
		d.CellFormat(0, 8, "No connections in this period.", "", 1, "L", false, 0, "")
		return d.bytes()
	}

	if len(byUser) > 1 {
		d.heading("Per user", 10)
		sumCols := []pdfColumn{{"User", 80, false}, {"Sessions", 30, false}, {"Connected time", 45, false}}
		d.tableHeader(sumCols)
		for i, s := range byUser {
			d.tableRow(sumCols, []string{s.User, fmt.Sprint(s.Sessions), orDash(fmtDuration(s.Total))}, i%2 == 1)
		}
		d.Ln(5)
	}

	d.ensure(20)
	d.heading("Sessions", 10)
	cols := []pdfColumn{
		{"Connected", 32, false}, {"Disconnected", 32, false}, {"Duration", 19, false},
		{"User", 35, false}, {"Server", 33, false}, {"Device", 33, false},
		{"Source IP", 28, true}, {"VPN IP", 26, true}, {"Ended", 31, false},
	}
	// Spread any spare width over the text columns.
	used := 0.0
	for _, c := range cols {
		used += c.width
	}
	if spare := d.contentWidth() - used; spare > 0 {
		for _, i := range []int{3, 4, 5} {
			cols[i].width += spare / 3
		}
	}
	d.tableHeader(cols)
	for i, s := range sessions {
		d.tableRow(cols, []string{
			fmtTime(s.Start, meta.Location), fmtTime(s.End, meta.Location), s.durationText(),
			s.Username, s.Server, s.Device, s.SourceIP, s.VPNIP, reasonText(s.Reason, s.End != nil),
		}, i%2 == 1)
	}
	return d.bytes()
}

func orDash(s string) string {
	if s == "" {
		return "0 min"
	}
	return s
}
