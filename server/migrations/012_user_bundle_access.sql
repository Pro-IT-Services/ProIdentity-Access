-- Per-user bundle assignments. A user gets specific bundles on specific servers,
-- rather than inheriting every bundle attached to a server.
-- server_bundle_access remains as "allowed bundles" (which bundles CAN be assigned).

CREATE TABLE IF NOT EXISTS user_bundle_access (
    user_id   CHAR(36) NOT NULL,
    server_id CHAR(36) NOT NULL,
    bundle_id CHAR(36) NOT NULL,
    PRIMARY KEY (user_id, server_id, bundle_id),
    INDEX idx_uba_server (server_id),
    INDEX idx_uba_bundle (bundle_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Backfill: every user who currently has server access gets all bundles that
-- server currently has, so existing access is preserved.
INSERT IGNORE INTO user_bundle_access (user_id, server_id, bundle_id)
SELECT usa.user_id, sba.server_id, sba.bundle_id
FROM user_server_access usa
JOIN server_bundle_access sba ON sba.server_id = usa.server_id;
