// Package denial captures denied wg traffic via NFLOG and persists deduped
// summaries to the denied_attempts table. Linux only.
//
//go:build linux

package denial

import (
	"context"
	"encoding/binary"
	"log"
	"net"
	"os"
	"sync"
	"time"

	nflog "github.com/florianl/go-nflog/v2"
	"github.com/jmoiron/sqlx"
)

// NflogGroup must match firewall.NflogGroup.
const NflogGroup = 100

// Event is one parsed deny event.
type Event struct {
	Ts      time.Time
	SrcIP   string
	DstIP   string
	DstPort uint16 // 0 for ICMP / unknown
	Proto   string // tcp/udp/icmp/other
}

type key struct {
	src, dst string
	port     uint16
	proto    string
}
type acc struct {
	first, last time.Time
	count       int
}

// Collector subscribes to NFLOG, dedupes, and inserts rows.
type Collector struct {
	db         *sqlx.DB
	flushEvery time.Duration
	mu         sync.Mutex
	buf        map[key]*acc
}

func New(db *sqlx.DB, flushEvery time.Duration) *Collector {
	if flushEvery <= 0 {
		flushEvery = 30 * time.Second
	}
	return &Collector{db: db, flushEvery: flushEvery, buf: map[key]*acc{}}
}

// Start subscribes to NFLOG and runs until ctx is cancelled. Non-blocking — the
// reader and flusher run in goroutines spawned by go-nflog and us.
func (c *Collector) Start(ctx context.Context) error {
	cfg := nflog.Config{
		Group:    NflogGroup,
		Copymode: nflog.CopyPacket,
		Bufsize:  64 * 1024,
	}
	nf, err := nflog.Open(&cfg)
	if err != nil {
		return err
	}

	hook := func(attrs nflog.Attribute) int {
		if attrs.Payload == nil {
			return 0
		}
		if ev, ok := parsePacket(*attrs.Payload); ok {
			c.add(ev)
		}
		return 0
	}
	errFn := func(err error) int {
		log.Printf("nflog error: %v", err)
		return 0
	}

	if err := nf.RegisterWithErrorFunc(ctx, hook, errFn); err != nil {
		nf.Close()
		return err
	}

	go func() {
		t := time.NewTicker(c.flushEvery)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				c.flush()
				_ = nf.Close()
				return
			case <-t.C:
				c.flush()
			}
		}
	}()
	return nil
}

func (c *Collector) add(ev Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	k := key{src: ev.SrcIP, dst: ev.DstIP, port: ev.DstPort, proto: ev.Proto}
	a, ok := c.buf[k]
	if !ok {
		a = &acc{first: ev.Ts, last: ev.Ts}
		c.buf[k] = a
	}
	a.last = ev.Ts
	a.count++
}

func (c *Collector) flush() {
	c.mu.Lock()
	if len(c.buf) == 0 {
		c.mu.Unlock()
		return
	}
	batch := c.buf
	c.buf = map[key]*acc{}
	c.mu.Unlock()

	type sessRow struct {
		AssignedIP string `db:"assigned_ip"`
		UserID     string `db:"user_id"`
	}
	var rows []sessRow
	if err := c.db.Select(&rows, "SELECT assigned_ip, user_id FROM sessions"); err != nil {
		log.Printf("denial: load sessions for attribution failed: %v", err)
		// Continue and insert with NULL user_id.
	}
	ipUser := make(map[string]string, len(rows))
	for _, r := range rows {
		ipUser[r.AssignedIP] = r.UserID
	}

	for k, a := range batch {
		var userArg any
		if u := ipUser[k.src]; u != "" {
			userArg = u
		}
		var portArg any
		if k.port != 0 {
			portArg = int(k.port)
		}
		_, err := c.db.Exec(`
			INSERT INTO denied_attempts
				(first_ts, last_ts, count, user_id, src_ip, dst_ip, dst_port, proto)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			a.first, a.last, a.count, userArg,
			k.src, k.dst, portArg, k.proto,
		)
		if err != nil {
			log.Printf("denial: insert failed: %v", err)
		}
	}
}

// IsRoot is a convenience for callers to warn if NFLOG would EPERM.
func IsRoot() bool { return os.Geteuid() == 0 }

// parsePacket extracts (src, dst, dport, proto) from an IPv4/IPv6 packet.
func parsePacket(b []byte) (Event, bool) {
	if len(b) < 1 {
		return Event{}, false
	}
	switch b[0] >> 4 {
	case 4:
		return parseIPv4(b)
	case 6:
		return parseIPv6(b)
	}
	return Event{}, false
}

func parseIPv4(b []byte) (Event, bool) {
	if len(b) < 20 {
		return Event{}, false
	}
	ihl := int(b[0]&0x0f) * 4
	if ihl < 20 || len(b) < ihl {
		return Event{}, false
	}
	proto := b[9]
	ev := Event{
		Ts:    time.Now(),
		SrcIP: net.IP(b[12:16]).String(),
		DstIP: net.IP(b[16:20]).String(),
		Proto: protoName(proto),
	}
	if (proto == 6 || proto == 17) && len(b) >= ihl+4 {
		ev.DstPort = binary.BigEndian.Uint16(b[ihl+2 : ihl+4])
	}
	return ev, true
}

func parseIPv6(b []byte) (Event, bool) {
	if len(b) < 40 {
		return Event{}, false
	}
	proto := b[6] // next header (extension headers not chased)
	ev := Event{
		Ts:    time.Now(),
		SrcIP: net.IP(b[8:24]).String(),
		DstIP: net.IP(b[24:40]).String(),
		Proto: protoName(proto),
	}
	if (proto == 6 || proto == 17) && len(b) >= 44 {
		ev.DstPort = binary.BigEndian.Uint16(b[42:44])
	}
	return ev, true
}

func protoName(p byte) string {
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
