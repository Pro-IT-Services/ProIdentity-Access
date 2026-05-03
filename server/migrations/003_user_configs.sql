-- Per-user config encryption key and user-uploaded config storage

ALTER TABLE users ADD COLUMN IF NOT EXISTS config_key VARBINARY(32) DEFAULT NULL;

CREATE TABLE IF NOT EXISTS user_configs (
    id           CHAR(36)     PRIMARY KEY,
    user_id      CHAR(36)     NOT NULL,
    name         VARCHAR(255) NOT NULL,
    encrypted_content LONGBLOB NOT NULL,
    created_at   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_user_configs_user_id (user_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
