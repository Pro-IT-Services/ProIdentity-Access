CREATE TABLE IF NOT EXISTS wg_servers (
    id             CHAR(36)     PRIMARY KEY,
    name           VARCHAR(128) NOT NULL,
    endpoint       VARCHAR(255) NOT NULL,
    port           INT          NOT NULL DEFAULT 51820,
    interface_name VARCHAR(50)  NOT NULL,
    private_key    TEXT         NOT NULL,
    public_key     TEXT         NOT NULL,
    subnet         VARCHAR(50)  NOT NULL,
    dns            VARCHAR(255) DEFAULT NULL,
    external       TINYINT(1)   NOT NULL DEFAULT 0,
    is_active      TINYINT(1)   NOT NULL DEFAULT 1,
    created_at     TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS group_server_access (
    group_id  CHAR(36) NOT NULL,
    server_id CHAR(36) NOT NULL,
    PRIMARY KEY (group_id, server_id),
    FOREIGN KEY (group_id)  REFERENCES `groups`(id)   ON DELETE CASCADE,
    FOREIGN KEY (server_id) REFERENCES wg_servers(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS user_server_access (
    user_id   CHAR(36) NOT NULL,
    server_id CHAR(36) NOT NULL,
    PRIMARY KEY (user_id, server_id),
    FOREIGN KEY (user_id)   REFERENCES users(id)      ON DELETE CASCADE,
    FOREIGN KEY (server_id) REFERENCES wg_servers(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

ALTER TABLE sessions ADD COLUMN IF NOT EXISTS server_id CHAR(36) DEFAULT NULL AFTER user_id;

TRUNCATE TABLE ip_pool;
ALTER TABLE ip_pool ADD COLUMN IF NOT EXISTS server_id CHAR(36) NOT NULL DEFAULT '' AFTER ip;
ALTER TABLE ip_pool DROP PRIMARY KEY, ADD PRIMARY KEY (server_id, ip);
ALTER TABLE ip_pool MODIFY COLUMN server_id CHAR(36) NOT NULL;
