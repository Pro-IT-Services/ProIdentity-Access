package session

import (
	"context"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"proidentity/internal/model"
)

type EndpointCandidate struct {
	Role     string `json:"role"`
	Host     string `json:"host"`
	IP       string `json:"ip"`
	Port     int    `json:"port"`
	Priority int    `json:"priority"`
	Endpoint string `json:"endpoint"`
}

func (m *Manager) resolvedEndpoints(srv model.WGServer) ([]EndpointCandidate, error) {
	rows := []model.WGServerEndpoint{}
	err := m.db.Select(&rows, `
		SELECT * FROM wg_server_endpoints
		WHERE server_id = ? AND enabled = 1
		ORDER BY priority ASC, created_at ASC`, srv.ID)
	if err != nil {
		return nil, fmt.Errorf("load endpoints: %w", err)
	}
	if len(rows) == 0 {
		rows = append(rows, model.WGServerEndpoint{
			ServerID: srv.ID,
			Name:     "Primary",
			Host:     srv.Endpoint,
			Port:     srv.Port,
			Priority: 0,
			Enabled:  true,
		})
	}

	out := make([]EndpointCandidate, 0, len(rows))
	for i, row := range rows {
		host := strings.TrimSpace(row.Host)
		if host == "" {
			continue
		}
		port := row.Port
		if port <= 0 {
			port = srv.Port
		}
		ip, err := resolveEndpointHost(host)
		if err != nil {
			log.Printf("warn: resolve WireGuard endpoint %s:%d for server %s: %v", host, port, srv.Name, err)
			continue
		}
		role := "failover"
		if i == 0 || row.Priority == 0 {
			role = "primary"
		}
		candidate := EndpointCandidate{
			Role:     role,
			Host:     host,
			IP:       ip,
			Port:     port,
			Priority: row.Priority,
			Endpoint: net.JoinHostPort(ip, strconv.Itoa(port)),
		}
		out = append(out, candidate)
		if row.ID != "" {
			_, _ = m.db.Exec(
				"UPDATE wg_server_endpoints SET last_resolved_ip=?, last_resolved_at=NOW() WHERE id=?",
				ip, row.ID,
			)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no configured endpoint for %s could be resolved", srv.Name)
	}
	return out, nil
}

func resolveEndpointHost(host string) (string, error) {
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil {
		return ip.String(), nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", host, err)
	}
	for _, addr := range ips {
		if v4 := addr.IP.To4(); v4 != nil {
			return v4.String(), nil
		}
	}
	for _, addr := range ips {
		if ip := addr.IP.To16(); ip != nil {
			return ip.String(), nil
		}
	}
	return "", fmt.Errorf("resolve %s: no usable IP address", host)
}
