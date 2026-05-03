CREATE TABLE IF NOT EXISTS installations (
  id           CHAR(36)     PRIMARY KEY,
  device_name  VARCHAR(255) NOT NULL,
  client_public_key TEXT    NOT NULL,
  server_private_key TEXT   NOT NULL,
  server_public_key  TEXT   NOT NULL,
  user_id      CHAR(36)     NULL,
  is_active    TINYINT(1)   NOT NULL DEFAULT 1,
  last_seen    DATETIME     NULL,
  created_at   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_inst_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
