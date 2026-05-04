package ippool

import (
	"encoding/binary"
	"fmt"
	"net"

	"github.com/jmoiron/sqlx"
)

// InitServer ensures the ip_pool table is populated for the given server and CIDR.
func InitServer(db *sqlx.DB, serverID, cidr string) error {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("parse cidr %q: %w", cidr, err)
	}

	var count int
	if err := db.Get(&count, "SELECT COUNT(*) FROM ip_pool WHERE server_id=?", serverID); err != nil {
		return fmt.Errorf("count ip_pool: %w", err)
	}
	if count > 0 {
		return nil
	}

	network := ipNet.IP.To4()
	if network == nil {
		return fmt.Errorf("only IPv4 subnets supported")
	}
	base := binary.BigEndian.Uint32(network)

	ones, bits := ipNet.Mask.Size()
	total := 1 << (bits - ones)

	tx, err := db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare("INSERT IGNORE INTO ip_pool (server_id, ip, in_use) VALUES (?, ?, 0)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	// skip .0 (network), .1 (gateway), last (broadcast)
	for i := 2; i < total-1; i++ {
		addr := make(net.IP, 4)
		binary.BigEndian.PutUint32(addr, base+uint32(i))
		if _, err := stmt.Exec(serverID, addr.String()); err != nil {
			return fmt.Errorf("insert ip %s: %w", addr, err)
		}
	}
	return tx.Commit()
}

// Allocate marks a free IP as in-use for the given server and session, returning the IP.
func Allocate(db *sqlx.DB, serverID, sessionID string) (string, error) {
	tx, err := db.Beginx()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	var ip string
	err = tx.Get(&ip, "SELECT ip FROM ip_pool WHERE server_id=? AND in_use=0 LIMIT 1 FOR UPDATE", serverID)
	if err != nil {
		return "", fmt.Errorf("no free IPs available on this server")
	}

	if _, err := tx.Exec("UPDATE ip_pool SET in_use=1, session_id=? WHERE server_id=? AND ip=?",
		sessionID, serverID, ip); err != nil {
		return "", err
	}
	return ip, tx.Commit()
}

// Release marks the IP as free again for the given server.
func Release(db *sqlx.DB, serverID, ip string) error {
	_, err := db.Exec("UPDATE ip_pool SET in_use=0, session_id=NULL WHERE server_id=? AND ip=?", serverID, ip)
	return err
}
