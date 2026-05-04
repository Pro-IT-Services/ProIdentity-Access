CREATE TABLE IF NOT EXISTS wg_server_endpoints (
    id CHAR(36) PRIMARY KEY,
    server_id CHAR(36) NOT NULL,
    name VARCHAR(128) NOT NULL DEFAULT '',
    host VARCHAR(255) NOT NULL,
    port INT NOT NULL DEFAULT 51820,
    priority INT NOT NULL DEFAULT 0,
    enabled TINYINT(1) NOT NULL DEFAULT 1,
    last_resolved_ip VARCHAR(64) DEFAULT NULL,
    last_resolved_at TIMESTAMP NULL DEFAULT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uniq_wg_server_endpoint_priority (server_id, priority),
    KEY idx_wg_server_endpoints_server (server_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT INTO wg_server_endpoints (id, server_id, name, host, port, priority, enabled)
SELECT UUID(), s.id, 'Primary', s.endpoint, s.port, 0, 1
FROM wg_servers s
WHERE NOT EXISTS (
    SELECT 1 FROM wg_server_endpoints e WHERE e.server_id = s.id
);
