ALTER TABLE sessions
    ADD COLUMN source_ip VARCHAR(45) DEFAULT NULL AFTER assigned_ip,
    ADD COLUMN device_id CHAR(36) DEFAULT NULL AFTER source_ip,
    ADD COLUMN device_name VARCHAR(128) DEFAULT NULL AFTER device_id,
    ADD COLUMN user_agent TEXT DEFAULT NULL AFTER device_name;

CREATE TABLE IF NOT EXISTS vpn_connection_events (
    id CHAR(36) PRIMARY KEY,
    event_type VARCHAR(32) NOT NULL,
    reason VARCHAR(64) DEFAULT NULL,
    user_id CHAR(36) DEFAULT NULL,
    username VARCHAR(64) DEFAULT NULL,
    email VARCHAR(255) DEFAULT NULL,
    session_id CHAR(36) NOT NULL,
    server_id CHAR(36) DEFAULT NULL,
    server_name VARCHAR(128) DEFAULT NULL,
    assigned_ip VARCHAR(45) NOT NULL,
    source_ip VARCHAR(45) DEFAULT NULL,
    device_id CHAR(36) DEFAULT NULL,
    device_name VARCHAR(128) DEFAULT NULL,
    user_agent TEXT DEFAULT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    KEY idx_vpn_events_created_at (created_at),
    KEY idx_vpn_events_user (user_id, created_at),
    KEY idx_vpn_events_server (server_id, created_at),
    KEY idx_vpn_events_session (session_id),
    KEY idx_vpn_events_type (event_type, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
