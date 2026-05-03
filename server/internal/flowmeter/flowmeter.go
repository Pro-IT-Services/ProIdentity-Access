// Package flowmeter samples the kernel conntrack table to attribute VPN traffic
// per (user, resource, dst_port, proto). Linux only.
//
//go:build linux

package flowmeter

import (
	"context"
	"log"
	"net/netip"
	"os"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/ti-mo/conntrack"
)

// flowKey uniquely identifies a flow we're tracking byte counts for.
type flowKey struct {
	src, dst   netip.Addr
	sport      uint16
	dport      uint16
	proto      uint8
}

type counters struct {
	bytesTX, bytesRX uint64
	pktsTX, pktsRX   uint64
}

// Meter periodically samples conntrack and persists per-flow deltas.
type Meter struct {
	db       *sqlx.DB
	interval time.Duration

	mu   sync.Mutex
	prev map[flowKey]counters
}

// New returns a Meter; interval defaults to 30s if non-positive.
func New(db *sqlx.DB, interval time.Duration) *Meter {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &Meter{db: db, interval: interval, prev: map[flowKey]counters{}}
}

// EnableConntrackAccounting writes the sysctl that turns on per-flow byte counters.
// Without this, conntrack only tracks 5-tuples without bytes/packets.
func EnableConntrackAccounting() error {
	return os.WriteFile("/proc/sys/net/netfilter/nf_conntrack_acct", []byte("1"), 0644)
}

// Start spawns the sampling goroutine. Returns immediately.
func (m *Meter) Start(ctx context.Context) error {
	conn, err := conntrack.Dial(nil)
	if err != nil {
		return err
	}
	go func() {
		defer conn.Close()
		t := time.NewTicker(m.interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				m.sample(conn)
			}
		}
	}()
	return nil
}

type resourceRow struct {
	ID        string `db:"id"`
	IPAddress string `db:"ip_address"`
	Type      string `db:"type"`
	Mask      *int   `db:"mask"`
}

type sessionRow struct {
	UserID     string  `db:"user_id"`
	AssignedIP string  `db:"assigned_ip"`
	ServerID   *string `db:"server_id"`
}

// resourceMatcher matches a destination IP to a resource row.
type resourceMatcher struct {
	hosts    map[netip.Addr]string             // exact host -> resource id
	networks []struct {
		prefix netip.Prefix
		id     string
	}
}

func buildMatcher(rs []resourceRow) *resourceMatcher {
	m := &resourceMatcher{hosts: map[netip.Addr]string{}}
	for _, r := range rs {
		ip, err := netip.ParseAddr(r.IPAddress)
		if err != nil {
			continue
		}
		if r.Type == "network" && r.Mask != nil {
			p, err := netip.ParsePrefix(r.IPAddress + "/" + itoa(*r.Mask))
			if err == nil {
				m.networks = append(m.networks, struct {
					prefix netip.Prefix
					id     string
				}{p.Masked(), r.ID})
			}
		} else {
			m.hosts[ip] = r.ID
		}
	}
	return m
}

func (m *resourceMatcher) match(ip netip.Addr) string {
	if id, ok := m.hosts[ip]; ok {
		return id
	}
	for _, n := range m.networks {
		if n.prefix.Contains(ip) {
			return n.id
		}
	}
	return ""
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	x := n
	if x < 0 {
		x = -x
	}
	for x > 0 {
		digits = append([]byte{byte('0' + x%10)}, digits...)
		x /= 10
	}
	return string(digits)
}

func (m *Meter) sample(conn *conntrack.Conn) {
	flows, err := conn.Dump(nil)
	if err != nil {
		log.Printf("flowmeter: conntrack dump: %v", err)
		return
	}

	// Snapshot lookup tables once per sample.
	var sessions []sessionRow
	if err := m.db.Select(&sessions, "SELECT user_id, assigned_ip, server_id FROM sessions"); err != nil {
		log.Printf("flowmeter: load sessions: %v", err)
		return
	}
	if len(sessions) == 0 {
		// No active VPN clients — clear cache to prevent unbounded growth.
		m.mu.Lock()
		m.prev = map[flowKey]counters{}
		m.mu.Unlock()
		return
	}
	ipUser := map[netip.Addr]sessionRow{}
	for _, s := range sessions {
		if a, err := netip.ParseAddr(s.AssignedIP); err == nil {
			ipUser[a] = s
		}
	}

	var resources []resourceRow
	_ = m.db.Select(&resources, "SELECT id, ip_address, type, mask FROM resources")
	matcher := buildMatcher(resources)

	now := time.Now()

	m.mu.Lock()
	prev := m.prev
	curr := map[flowKey]counters{}
	m.mu.Unlock()

	type insertRow struct {
		key      flowKey
		delta    counters
		userRow  sessionRow
		resID    string
	}
	rows := []insertRow{}

	for _, f := range flows {
		// We only care about flows whose orig source matches a VPN client.
		src := f.TupleOrig.IP.SourceAddress
		dst := f.TupleOrig.IP.DestinationAddress
		sess, ok := ipUser[src]
		if !ok {
			continue
		}

		k := flowKey{
			src:   src,
			dst:   dst,
			sport: f.TupleOrig.Proto.SourcePort,
			dport: f.TupleOrig.Proto.DestinationPort,
			proto: f.TupleOrig.Proto.Protocol,
		}
		c := counters{
			bytesTX: f.CountersOrig.Bytes,
			bytesRX: f.CountersReply.Bytes,
			pktsTX:  f.CountersOrig.Packets,
			pktsRX:  f.CountersReply.Packets,
		}
		curr[k] = c

		// Delta vs previous snapshot. New flows: full counter (since flow start).
		var d counters
		if p, had := prev[k]; had {
			d = counters{
				bytesTX: subSat(c.bytesTX, p.bytesTX),
				bytesRX: subSat(c.bytesRX, p.bytesRX),
				pktsTX:  subSat(c.pktsTX, p.pktsTX),
				pktsRX:  subSat(c.pktsRX, p.pktsRX),
			}
		} else {
			d = c
		}
		if d.bytesTX == 0 && d.bytesRX == 0 && d.pktsTX == 0 && d.pktsRX == 0 {
			continue // no traffic this window
		}
		rows = append(rows, insertRow{
			key:     k,
			delta:   d,
			userRow: sess,
			resID:   matcher.match(dst),
		})
	}

	m.mu.Lock()
	m.prev = curr
	m.mu.Unlock()

	if len(rows) == 0 {
		return
	}

	// Batch insert.
	tx, err := m.db.Beginx()
	if err != nil {
		log.Printf("flowmeter: begin tx: %v", err)
		return
	}
	const stmt = `INSERT INTO traffic_flows
		(ts, user_id, server_id, resource_id, src_ip, dst_ip, dst_port, proto,
		 bytes_tx, bytes_rx, pkts_tx, pkts_rx)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	for _, r := range rows {
		var resArg, srvArg, portArg any
		if r.resID != "" {
			resArg = r.resID
		}
		if r.userRow.ServerID != nil {
			srvArg = *r.userRow.ServerID
		}
		if r.key.dport != 0 {
			portArg = int(r.key.dport)
		}
		if _, err := tx.Exec(stmt, now, r.userRow.UserID, srvArg, resArg,
			r.key.src.String(), r.key.dst.String(), portArg, protoName(r.key.proto),
			r.delta.bytesTX, r.delta.bytesRX, r.delta.pktsTX, r.delta.pktsRX,
		); err != nil {
			log.Printf("flowmeter: insert: %v", err)
		}
	}
	if err := tx.Commit(); err != nil {
		log.Printf("flowmeter: commit: %v", err)
	}
}

// subSat is saturating subtraction — handles the (rare) case where conntrack
// re-creates an entry with the same key and the new counter is lower.
func subSat(a, b uint64) uint64 {
	if a < b {
		return 0
	}
	return a - b
}

func protoName(p uint8) string {
	switch p {
	case 1:
		return "icmp"
	case 6:
		return "tcp"
	case 17:
		return "udp"
	case 58:
		return "icmpv6"
	default:
		return "other"
	}
}
