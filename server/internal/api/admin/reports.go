package admin

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // report times in the admin's zone, even without system tzdata

	"github.com/jmoiron/sqlx"

	"proidentity/internal/audit"
)

// Reports: downloadable exports for the System page.
//
//   - Access report: every user and what they can reach (servers, bundles,
//     resources, assigned OpenVPN and WireGuard profiles, devices).
//   - Connection history: VPN sessions in a date range.
//
// Both are POST so the audit middleware records who exported what.
type ReportHandler struct{ DB *sqlx.DB }

const (
	maxReportDays     = 366
	maxReportSessions = 100000
)

type reportRequest struct {
	Format   string `json:"format"`    // pdf, md, csv
	TZ       string `json:"tz"`        // IANA zone of the admin's browser
	From     string `json:"from"`      // YYYY-MM-DD (connections)
	To       string `json:"to"`        // YYYY-MM-DD, inclusive (connections)
	UserID   string `json:"user_id"`   // optional filter (connections)
	ServerID string `json:"server_id"` // optional filter (connections)
}

func (req reportRequest) location() *time.Location {
	if loc, err := time.LoadLocation(strings.TrimSpace(req.TZ)); err == nil && req.TZ != "" {
		return loc
	}
	return time.UTC
}

// reportMeta is shown at the top of every report.
type reportMeta struct {
	VPNName     string
	GeneratedAt time.Time
	GeneratedBy string
	Location    *time.Location
}

func (h *ReportHandler) meta(r *http.Request, loc *time.Location) reportMeta {
	var name string
	_ = h.DB.Get(&name, "SELECT `value` FROM settings WHERE `key`='vpn_name'")
	if name == "" {
		name = "ProIdentity Access"
	}
	by := ""
	if c := claimsFrom(r); c != nil {
		by = c.Username
	}
	return reportMeta{VPNName: name, GeneratedAt: time.Now().In(loc), GeneratedBy: by, Location: loc}
}

func (m reportMeta) stamp() string {
	return m.GeneratedAt.Format("2006-01-02 15:04") + " (" + m.Location.String() + ")"
}

func sendFile(w http.ResponseWriter, name, contentType string, data []byte) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	_, _ = w.Write(data)
}

// csvBytes writes rows for spreadsheet use: UTF-8 with BOM and ';' between
// fields, which is what Excel expects in comma-decimal locales (sk, cs, de…).
func csvBytes(rows [][]string) []byte {
	var buf bytes.Buffer
	buf.Write([]byte{0xEF, 0xBB, 0xBF}) // UTF-8 BOM
	cw := csv.NewWriter(&buf)
	cw.Comma = ';'
	_ = cw.WriteAll(rows)
	return buf.Bytes()
}

// ── Access report ─────────────────────────────────────────────────────────

type accessResource struct{ Name, Address, Ports string }
type accessBundle struct {
	Name      string
	Resources []accessResource
}
type accessServer struct {
	Name, Subnet string
	Active       bool
	Bundles      []accessBundle
}
type accessProfile struct{ Name, Detail string }
type accessDevice struct {
	Name     string
	Active   bool
	LastSeen *time.Time
}
type accessUser struct {
	ID, Username, Email, Name string
	Admin, Active, TOTP       bool
	Roles                     []string
	Servers                   []*accessServer
	OpenVPN                   []accessProfile
	WireGuard                 []accessProfile
	Devices                   []accessDevice
}

func (u *accessUser) displayName() string {
	if u.Name != "" {
		return u.Name
	}
	return u.Username
}

func (u *accessUser) hasAccess() bool {
	return len(u.Servers)+len(u.OpenVPN)+len(u.WireGuard) > 0
}

func resourceAddress(ip, typ string, mask *int) string {
	if mask != nil && (typ == "network" || typ == "subnet" || *mask < 32) {
		return fmt.Sprintf("%s/%d", ip, *mask)
	}
	return ip
}

func (h *ReportHandler) loadAccess() ([]*accessUser, error) {
	var users []struct {
		ID        string `db:"id"`
		Username  string `db:"username"`
		Email     string `db:"email"`
		FirstName string `db:"first_name"`
		LastName  string `db:"last_name"`
		IsAdmin   bool   `db:"is_admin"`
		IsActive  bool   `db:"is_active"`
		TOTP      bool   `db:"totp_enabled"`
	}
	if err := h.DB.Select(&users, `
		SELECT id, username, email, first_name, last_name, is_admin, is_active, totp_enabled
		FROM users ORDER BY is_active DESC, username`); err != nil {
		return nil, err
	}
	out := make([]*accessUser, 0, len(users))
	byID := map[string]*accessUser{}
	for _, u := range users {
		au := &accessUser{
			ID: u.ID, Username: u.Username, Email: u.Email,
			Name:  strings.TrimSpace(u.FirstName + " " + u.LastName),
			Admin: u.IsAdmin, Active: u.IsActive, TOTP: u.TOTP,
		}
		out = append(out, au)
		byID[u.ID] = au
	}

	var roles []struct {
		UserID string `db:"user_id"`
		Name   string `db:"name"`
	}
	_ = h.DB.Select(&roles, "SELECT ug.user_id, g.name FROM user_groups ug JOIN `groups` g ON g.id = ug.group_id ORDER BY g.name")
	for _, r := range roles {
		if u := byID[r.UserID]; u != nil {
			u.Roles = append(u.Roles, r.Name)
		}
	}

	var servers []struct {
		UserID   string `db:"user_id"`
		ServerID string `db:"server_id"`
		Name     string `db:"name"`
		Subnet   string `db:"subnet"`
		Active   bool   `db:"is_active"`
	}
	if err := h.DB.Select(&servers, `
		SELECT usa.user_id, s.id AS server_id, s.name, s.subnet, s.is_active
		FROM user_server_access usa JOIN wg_servers s ON s.id = usa.server_id
		ORDER BY s.name`); err != nil {
		return nil, err
	}
	type key struct{ user, server string }
	serverOf := map[key]*accessServer{}
	for _, s := range servers {
		u := byID[s.UserID]
		if u == nil {
			continue
		}
		as := &accessServer{Name: s.Name, Subnet: s.Subnet, Active: s.Active}
		u.Servers = append(u.Servers, as)
		serverOf[key{s.UserID, s.ServerID}] = as
	}

	// Bundles count only on servers the user can reach (a session needs both).
	var bundles []struct {
		UserID   string  `db:"user_id"`
		ServerID string  `db:"server_id"`
		Bundle   string  `db:"bundle"`
		Resource *string `db:"resource"`
		IP       *string `db:"ip_address"`
		Type     *string `db:"type"`
		Mask     *int    `db:"mask"`
		Ports    *string `db:"ports"`
	}
	if err := h.DB.Select(&bundles, `
		SELECT uba.user_id, uba.server_id, rg.name AS bundle,
		       r.name AS resource, r.ip_address, r.type, r.mask, r.ports
		FROM user_bundle_access uba
		JOIN resource_groups rg ON rg.id = uba.bundle_id
		LEFT JOIN resource_group_members m ON m.resource_group_id = rg.id
		LEFT JOIN resources r ON r.id = m.resource_id
		ORDER BY rg.name, r.name`); err != nil {
		return nil, err
	}
	for _, b := range bundles {
		as := serverOf[key{b.UserID, b.ServerID}]
		if as == nil {
			continue
		}
		var bundle *accessBundle
		for i := range as.Bundles {
			if as.Bundles[i].Name == b.Bundle {
				bundle = &as.Bundles[i]
			}
		}
		if bundle == nil {
			as.Bundles = append(as.Bundles, accessBundle{Name: b.Bundle})
			bundle = &as.Bundles[len(as.Bundles)-1]
		}
		if b.Resource != nil {
			typ := ""
			if b.Type != nil {
				typ = *b.Type
			}
			ports := ""
			if b.Ports != nil {
				ports = *b.Ports
			}
			bundle.Resources = append(bundle.Resources, accessResource{
				Name: *b.Resource, Address: resourceAddress(deref(b.IP), typ, b.Mask), Ports: ports,
			})
		}
	}

	var ovpn []struct {
		UserID   string  `db:"user_id"`
		Name     string  `db:"name"`
		DevType  string  `db:"dev_type"`
		TOTP     bool    `db:"requires_totp"`
		CustomIP *string `db:"custom_ip"`
	}
	_ = h.DB.Select(&ovpn, `
		SELECT a.user_id, p.name, p.dev_type, p.requires_totp, a.custom_ip
		FROM openvpn_profile_assignments a JOIN openvpn_profiles p ON p.id = a.profile_id
		ORDER BY p.name`)
	for _, p := range ovpn {
		if u := byID[p.UserID]; u != nil {
			parts := []string{strings.ToUpper(p.DevType)}
			if p.TOTP {
				parts = append(parts, "TOTP")
			}
			if p.CustomIP != nil && *p.CustomIP != "" {
				parts = append(parts, "IP "+*p.CustomIP)
			}
			u.OpenVPN = append(u.OpenVPN, accessProfile{Name: p.Name, Detail: strings.Join(parts, ", ")})
		}
	}

	var wg []struct {
		UserID   string  `db:"user_id"`
		Name     string  `db:"name"`
		Shared   bool    `db:"shared"`
		Address  *string `db:"address"`
		Endpoint *string `db:"endpoint"`
	}
	_ = h.DB.Select(&wg, `
		SELECT a.user_id, p.name, p.shared, p.address, p.endpoint
		FROM wireguard_profile_assignments a JOIN wireguard_profiles p ON p.id = a.profile_id
		ORDER BY p.name`)
	for _, p := range wg {
		if u := byID[p.UserID]; u != nil {
			kind := "single user"
			if p.Shared {
				kind = "shared"
			}
			detail := kind
			if a := deref(p.Address); a != "" {
				detail += ", " + a
			}
			if e := deref(p.Endpoint); e != "" {
				detail += " via " + e
			}
			u.WireGuard = append(u.WireGuard, accessProfile{Name: p.Name, Detail: detail})
		}
	}

	var devices []struct {
		UserID   string     `db:"user_id"`
		Name     string     `db:"device_name"`
		Active   bool       `db:"is_active"`
		LastSeen *time.Time `db:"last_seen"`
	}
	_ = h.DB.Select(&devices, `
		SELECT user_id, device_name, is_active, last_seen FROM installations
		WHERE user_id IS NOT NULL ORDER BY last_seen DESC`)
	for _, d := range devices {
		if u := byID[d.UserID]; u != nil {
			u.Devices = append(u.Devices, accessDevice{Name: d.Name, Active: d.Active, LastSeen: d.LastSeen})
		}
	}
	return out, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func status(active bool) string {
	if active {
		return "active"
	}
	return "disabled"
}

func lastSeen(t *time.Time, loc *time.Location) string {
	if t == nil || t.IsZero() {
		return "never"
	}
	return t.In(loc).Format("2006-01-02 15:04")
}

// POST /api/v1/admin/reports/access {format: pdf|md|csv, tz}
func (h *ReportHandler) Access(w http.ResponseWriter, r *http.Request) {
	var req reportRequest
	if err := decode(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request")
		return
	}
	format := strings.ToLower(req.Format)
	if format != "pdf" && format != "md" && format != "csv" {
		jsonError(w, http.StatusBadRequest, "format must be pdf, md or csv")
		return
	}
	users, err := h.loadAccess()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	meta := h.meta(r, req.location())
	audit.Annotate(r, func(p *audit.PerRequest) {
		p.Action, p.TargetType, p.TargetLabel = "report.export", "report", "access ("+format+")"
	})
	name := "access-report-" + meta.GeneratedAt.Format("2006-01-02") + "." + format
	switch format {
	case "csv":
		sendFile(w, name, "text/csv; charset=utf-8", accessCSV(users, meta))
	case "md":
		sendFile(w, name, "text/markdown; charset=utf-8", accessMarkdown(users, meta))
	default:
		data, err := accessPDF(users, meta)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		sendFile(w, name, "application/pdf", data)
	}
}

func accessCSV(users []*accessUser, meta reportMeta) []byte {
	rows := [][]string{{"username", "name", "email", "status", "admin", "two_factor", "roles",
		"access", "server", "server_subnet", "bundle", "resource", "address", "ports", "profile", "detail"}}
	for _, u := range users {
		base := []string{u.Username, u.Name, u.Email, status(u.Active), yesNo(u.Admin), yesNo(u.TOTP), strings.Join(u.Roles, ", ")}
		row := func(kind, server, subnet, bundle, res, addr, ports, profile, detail string) {
			rows = append(rows, append(append([]string{}, base...), kind, server, subnet, bundle, res, addr, ports, profile, detail))
		}
		for _, s := range u.Servers {
			if len(s.Bundles) == 0 {
				row("server", s.Name, s.Subnet, "", "", "", "", "", "no bundles")
			}
			for _, b := range s.Bundles {
				if len(b.Resources) == 0 {
					row("bundle", s.Name, s.Subnet, b.Name, "", "", "", "", "empty bundle")
				}
				for _, res := range b.Resources {
					row("resource", s.Name, s.Subnet, b.Name, res.Name, res.Address, res.Ports, "", "")
				}
			}
		}
		for _, p := range u.OpenVPN {
			row("openvpn_profile", "", "", "", "", "", "", p.Name, p.Detail)
		}
		for _, p := range u.WireGuard {
			row("wireguard_profile", "", "", "", "", "", "", p.Name, p.Detail)
		}
		for _, d := range u.Devices {
			row("device", "", "", "", "", "", "", d.Name, status(d.Active)+", last seen "+lastSeen(d.LastSeen, meta.Location))
		}
		if !u.hasAccess() && len(u.Devices) == 0 {
			row("none", "", "", "", "", "", "", "", "")
		}
	}
	return csvBytes(rows)
}

// mdEscape keeps names from turning into Markdown formatting.
func mdEscape(s string) string {
	return strings.NewReplacer(`\`, `\\`, "*", `\*`, "_", `\_`, "`", "\\`", "[", `\[`, "]", `\]`, "#", `\#`, "|", `\|`, "<", "&lt;").Replace(s)
}

func accessMarkdown(users []*accessUser, meta reportMeta) []byte {
	var b strings.Builder
	active := 0
	for _, u := range users {
		if u.Active {
			active++
		}
	}
	fmt.Fprintf(&b, "# Access report: %s\n\n", mdEscape(meta.VPNName))
	fmt.Fprintf(&b, "Generated %s by %s. %d users (%d active).\n\n", meta.stamp(), mdEscape(meta.GeneratedBy), len(users), active)
	for _, u := range users {
		fmt.Fprintf(&b, "## %s (%s)\n\n", mdEscape(u.displayName()), mdEscape(u.Username))
		fmt.Fprintf(&b, "- Email: %s\n- Status: %s\n- Admin: %s\n- Two-factor: %s\n", mdEscape(u.Email), status(u.Active), yesNo(u.Admin), yesNo(u.TOTP))
		if len(u.Roles) > 0 {
			fmt.Fprintf(&b, "- Roles: %s\n", mdEscape(strings.Join(u.Roles, ", ")))
		}
		b.WriteString("\n")
		if !u.hasAccess() {
			b.WriteString("No access assigned.\n\n")
		}
		if len(u.Servers) > 0 {
			b.WriteString("### ProIdentity servers\n\n")
			for _, s := range u.Servers {
				inactive := ""
				if !s.Active {
					inactive = " (server disabled)"
				}
				fmt.Fprintf(&b, "- **%s** `%s`%s\n", mdEscape(s.Name), s.Subnet, inactive)
				if len(s.Bundles) == 0 {
					b.WriteString("  - No bundles\n")
				}
				for _, bu := range s.Bundles {
					fmt.Fprintf(&b, "  - %s\n", mdEscape(bu.Name))
					for _, res := range bu.Resources {
						ports := ""
						if res.Ports != "" {
							ports = ", ports " + mdEscape(res.Ports)
						}
						fmt.Fprintf(&b, "    - %s `%s`%s\n", mdEscape(res.Name), res.Address, ports)
					}
				}
			}
			b.WriteString("\n")
		}
		for _, section := range []struct {
			title string
			list  []accessProfile
		}{{"OpenVPN profiles", u.OpenVPN}, {"WireGuard profiles", u.WireGuard}} {
			if len(section.list) == 0 {
				continue
			}
			fmt.Fprintf(&b, "### %s\n\n", section.title)
			for _, p := range section.list {
				fmt.Fprintf(&b, "- %s (%s)\n", mdEscape(p.Name), mdEscape(p.Detail))
			}
			b.WriteString("\n")
		}
		if len(u.Devices) > 0 {
			b.WriteString("### Devices\n\n")
			for _, d := range u.Devices {
				fmt.Fprintf(&b, "- %s: %s, last seen %s\n", mdEscape(d.Name), status(d.Active), lastSeen(d.LastSeen, meta.Location))
			}
			b.WriteString("\n")
		}
	}
	return []byte(b.String())
}

// ── Connection history ────────────────────────────────────────────────────

type historySession struct {
	ID                              string
	Username, Email, Server, Device string
	SourceIP, VPNIP, Reason         string
	Start, End                      *time.Time
}

func (s historySession) duration() time.Duration {
	if s.Start == nil || s.End == nil {
		return 0
	}
	return s.End.Sub(*s.Start)
}

// durationText is empty only while the length is unknown (still connected,
// or the start is missing); a session that ended at once shows "<1 min".
func (s historySession) durationText() string {
	if s.Start == nil || s.End == nil {
		return ""
	}
	if d := s.duration(); d >= time.Minute {
		return fmtDuration(d)
	}
	return "<1 min"
}

func reasonText(reason string, ended bool) string {
	if !ended {
		return "still connected"
	}
	switch reason {
	case "requested":
		return "disconnected"
	case "expired":
		return "timed out"
	case "shutdown":
		return "server stopped"
	case "":
		return "ended"
	}
	return reason
}

func fmtDuration(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	d = d.Round(time.Minute)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h == 0 {
		if m == 0 {
			return "<1 min"
		}
		return fmt.Sprintf("%d min", m)
	}
	return fmt.Sprintf("%d h %d min", h, m)
}

func fmtTime(t *time.Time, loc *time.Location) string {
	if t == nil {
		return ""
	}
	return t.In(loc).Format("2006-01-02 15:04:05")
}

type historyEvent struct {
	EventType  string    `db:"event_type"`
	Reason     *string   `db:"reason"`
	Username   *string   `db:"username"`
	Email      *string   `db:"email"`
	SessionID  string    `db:"session_id"`
	ServerName *string   `db:"server_name"`
	AssignedIP string    `db:"assigned_ip"`
	SourceIP   *string   `db:"source_ip"`
	DeviceName *string   `db:"device_name"`
	CreatedAt  time.Time `db:"created_at"`
}

// loadHistory returns sessions with any event in [from, to), oldest first.
func (h *ReportHandler) loadHistory(from, to time.Time, userID, serverID string) ([]historySession, bool, error) {
	where := []string{"created_at >= ?", "created_at < ?"}
	args := []any{from.UTC(), to.UTC()}
	if userID != "" {
		where = append(where, "user_id = ?")
		args = append(args, userID)
	}
	if serverID != "" {
		where = append(where, "server_id = ?")
		args = append(args, serverID)
	}
	// Every event of each session that touches the range, so a session that
	// started before the range still gets its start time.
	q := `SELECT event_type, reason, username, email, session_id, server_name, assigned_ip,
	             source_ip, device_name, created_at
	      FROM vpn_connection_events
	      WHERE session_id IN (SELECT session_id FROM (
	              SELECT DISTINCT session_id FROM vpn_connection_events WHERE ` + strings.Join(where, " AND ") + `
	              LIMIT ` + strconv.Itoa(maxReportSessions+1) + `) s)
	      ORDER BY created_at`
	var events []historyEvent
	if err := h.DB.Select(&events, q, args...); err != nil {
		return nil, false, err
	}
	byID := map[string]*historySession{}
	order := []string{}
	for _, e := range events {
		s := byID[e.SessionID]
		if s == nil {
			s = &historySession{ID: e.SessionID}
			byID[e.SessionID] = s
			order = append(order, e.SessionID)
		}
		at := e.CreatedAt
		if s.Username == "" {
			s.Username, s.Email = deref(e.Username), deref(e.Email)
		}
		if s.Server == "" {
			s.Server = deref(e.ServerName)
		}
		if s.Device == "" {
			s.Device = deref(e.DeviceName)
		}
		if s.SourceIP == "" {
			s.SourceIP = deref(e.SourceIP)
		}
		if s.VPNIP == "" {
			s.VPNIP = e.AssignedIP
		}
		switch e.EventType {
		case "connected":
			if s.Start == nil {
				s.Start = &at
			}
		case "disconnected":
			s.End = &at
			s.Reason = deref(e.Reason)
		}
	}
	truncated := len(order) > maxReportSessions
	if truncated {
		order = order[:maxReportSessions]
	}
	out := make([]historySession, 0, len(order))
	for _, id := range order {
		out = append(out, *byID[id])
	}
	sort.SliceStable(out, func(i, j int) bool { return sessionTime(out[i]).Before(sessionTime(out[j])) })
	return out, truncated, nil
}

func sessionTime(s historySession) time.Time {
	if s.Start != nil {
		return *s.Start
	}
	if s.End != nil {
		return *s.End
	}
	return time.Time{}
}

// historySummary is the per-user total shown at the top of the PDF.
type historySummary struct {
	User     string
	Sessions int
	Total    time.Duration
}

func summarize(sessions []historySession) ([]historySummary, time.Duration) {
	by := map[string]*historySummary{}
	var total time.Duration
	for _, s := range sessions {
		x := by[s.Username]
		if x == nil {
			x = &historySummary{User: s.Username}
			by[s.Username] = x
		}
		x.Sessions++
		x.Total += s.duration()
		total += s.duration()
	}
	out := make([]historySummary, 0, len(by))
	for _, x := range by {
		out = append(out, *x)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Total > out[j].Total })
	return out, total
}

// POST /api/v1/admin/reports/connections {format: pdf|csv, tz, from, to, user_id?, server_id?}
func (h *ReportHandler) Connections(w http.ResponseWriter, r *http.Request) {
	var req reportRequest
	if err := decode(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request")
		return
	}
	format := strings.ToLower(req.Format)
	if format != "pdf" && format != "csv" {
		jsonError(w, http.StatusBadRequest, "format must be pdf or csv")
		return
	}
	loc := req.location()
	from, err1 := time.ParseInLocation("2006-01-02", req.From, loc)
	toDay, err2 := time.ParseInLocation("2006-01-02", req.To, loc)
	if err1 != nil || err2 != nil {
		jsonError(w, http.StatusBadRequest, "from and to must be dates (YYYY-MM-DD)")
		return
	}
	to := toDay.AddDate(0, 0, 1) // inclusive end day
	if !to.After(from) {
		jsonError(w, http.StatusBadRequest, "the end date is before the start date")
		return
	}
	if to.Sub(from) > maxReportDays*24*time.Hour {
		jsonError(w, http.StatusBadRequest, fmt.Sprintf("choose at most %d days", maxReportDays))
		return
	}
	sessions, truncated, err := h.loadHistory(from, to, strings.TrimSpace(req.UserID), strings.TrimSpace(req.ServerID))
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	meta := h.meta(r, loc)
	filter := h.historyFilterText(req)
	audit.Annotate(r, func(p *audit.PerRequest) {
		p.Action, p.TargetType = "report.export", "report"
		p.TargetLabel = "connection history " + req.From + " to " + req.To + " (" + format + ")"
	})
	name := "connection-history-" + req.From + "_" + req.To + "." + format
	if format == "csv" {
		sendFile(w, name, "text/csv; charset=utf-8", historyCSV(sessions, meta))
		return
	}
	data, err := historyPDF(sessions, meta, req.From, req.To, filter, truncated)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendFile(w, name, "application/pdf", data)
}

// historyFilterText names the user/server filter for the PDF header.
func (h *ReportHandler) historyFilterText(req reportRequest) string {
	parts := []string{}
	if req.UserID != "" {
		var name string
		_ = h.DB.Get(&name, "SELECT username FROM users WHERE id=?", req.UserID)
		parts = append(parts, "user "+name)
	}
	if req.ServerID != "" {
		var name string
		_ = h.DB.Get(&name, "SELECT name FROM wg_servers WHERE id=?", req.ServerID)
		parts = append(parts, "server "+name)
	}
	return strings.Join(parts, ", ")
}

func historyCSV(sessions []historySession, meta reportMeta) []byte {
	rows := [][]string{{"connected", "disconnected", "duration_minutes", "duration", "username", "email",
		"server", "device", "source_ip", "vpn_ip", "ended", "session_id"}}
	for _, s := range sessions {
		mins := ""
		if d := s.duration(); d > 0 {
			mins = strconv.FormatFloat(d.Minutes(), 'f', 1, 64)
		}
		rows = append(rows, []string{
			fmtTime(s.Start, meta.Location), fmtTime(s.End, meta.Location), mins, s.durationText(),
			s.Username, s.Email, s.Server, s.Device, s.SourceIP, s.VPNIP, reasonText(s.Reason, s.End != nil), s.ID,
		})
	}
	return csvBytes(rows)
}
