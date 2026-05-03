CREATE TABLE IF NOT EXISTS denied_attempts (
    id          BIGINT       AUTO_INCREMENT PRIMARY KEY,
    first_ts    DATETIME(3)  NOT NULL,
    last_ts     DATETIME(3)  NOT NULL,
    count       INT          NOT NULL DEFAULT 1,
    user_id     CHAR(36)     NULL,    -- NULL when src ip didn't map to a session
    src_ip      VARCHAR(45)  NOT NULL,
    dst_ip      VARCHAR(45)  NOT NULL,
    dst_port    INT          NULL,    -- NULL for ICMP and unknown protos
    proto       VARCHAR(8)   NOT NULL,
    INDEX idx_denied_user_last (user_id, last_ts),
    INDEX idx_denied_last (last_ts),
    INDEX idx_denied_dst (dst_ip, last_ts)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
