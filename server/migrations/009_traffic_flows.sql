-- Phase 2: per-flow traffic accounting via conntrack sampling.
-- One row = one (sample-window x user x dst x port x proto) tuple, with the
-- delta in bytes/packets observed during that window.
CREATE TABLE IF NOT EXISTS traffic_flows (
    id           BIGINT       AUTO_INCREMENT PRIMARY KEY,
    ts           DATETIME(3)  NOT NULL,           -- end of sample window
    user_id      CHAR(36)     NULL,               -- src ip mapped to user via sessions
    server_id    CHAR(36)     NULL,               -- which wg server the user came in on
    resource_id  CHAR(36)     NULL,               -- dst ip matched a resource (host or CIDR)
    src_ip       VARCHAR(45)  NOT NULL,
    dst_ip       VARCHAR(45)  NOT NULL,
    dst_port     INT          NULL,               -- NULL for ICMP / no L4
    proto        VARCHAR(8)   NOT NULL,
    bytes_tx     BIGINT       NOT NULL DEFAULT 0, -- client -> resource (delta)
    bytes_rx     BIGINT       NOT NULL DEFAULT 0, -- resource -> client (delta)
    pkts_tx      BIGINT       NOT NULL DEFAULT 0,
    pkts_rx      BIGINT       NOT NULL DEFAULT 0,
    INDEX idx_tf_user_ts     (user_id, ts),
    INDEX idx_tf_resource_ts (resource_id, ts),
    INDEX idx_tf_server_ts   (server_id, ts),
    INDEX idx_tf_ts          (ts)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
