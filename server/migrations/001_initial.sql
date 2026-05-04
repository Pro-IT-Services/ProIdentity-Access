-- ProIdentity initial schema

CREATE TABLE IF NOT EXISTS settings (
    `key`   VARCHAR(64)  PRIMARY KEY,
    `value` TEXT         NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Default settings
INSERT IGNORE INTO settings (`key`, `value`) VALUES
    ('vpn_subnet',      '10.8.0.0/24'),
    ('vpn_dns',         '1.1.1.1,8.8.8.8'),
    ('wg_endpoint',     'vpn.example.com'),
    ('wg_port',         '51820'),
    ('wg_interface',    'wg0'),
    ('session_timeout', '90'),
    ('keepalive_interval', '30'),
    ('server_name',     'ProIdentity'),
    ('webauthn_rp_id',  'localhost'),
    ('webauthn_rp_name','ProIdentity'),
    ('webauthn_origin', 'http://localhost:8080');

CREATE TABLE IF NOT EXISTS users (
    id           CHAR(36)     PRIMARY KEY,
    username     VARCHAR(64)  UNIQUE NOT NULL,
    email        VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    totp_secret  VARCHAR(64)  DEFAULT NULL,
    totp_enabled TINYINT(1)   NOT NULL DEFAULT 0,
    is_admin     TINYINT(1)   NOT NULL DEFAULT 0,
    is_active    TINYINT(1)   NOT NULL DEFAULT 1,
    created_at   TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS passkeys (
    id            CHAR(36)    PRIMARY KEY,
    user_id       CHAR(36)    NOT NULL,
    name          VARCHAR(64) NOT NULL DEFAULT 'Passkey',
    credential_id BLOB        NOT NULL,
    public_key    BLOB        NOT NULL,
    sign_count    BIGINT      NOT NULL DEFAULT 0,
    aaguid        VARCHAR(64) NOT NULL DEFAULT '',
    created_at    TIMESTAMP   NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `groups` (
    id          CHAR(36)     PRIMARY KEY,
    name        VARCHAR(64)  UNIQUE NOT NULL,
    description TEXT         DEFAULT NULL,
    created_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS user_groups (
    user_id  CHAR(36) NOT NULL,
    group_id CHAR(36) NOT NULL,
    PRIMARY KEY (user_id, group_id),
    FOREIGN KEY (user_id)  REFERENCES users(id)    ON DELETE CASCADE,
    FOREIGN KEY (group_id) REFERENCES `groups`(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS resources (
    id          CHAR(36)     PRIMARY KEY,
    name        VARCHAR(128) NOT NULL,
    ip_address  VARCHAR(45)  NOT NULL,
    description TEXT         DEFAULT NULL,
    created_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS resource_groups (
    id          CHAR(36)     PRIMARY KEY,
    name        VARCHAR(128) UNIQUE NOT NULL,
    description TEXT         DEFAULT NULL,
    created_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS resource_group_members (
    resource_group_id CHAR(36) NOT NULL,
    resource_id       CHAR(36) NOT NULL,
    PRIMARY KEY (resource_group_id, resource_id),
    FOREIGN KEY (resource_group_id) REFERENCES resource_groups(id) ON DELETE CASCADE,
    FOREIGN KEY (resource_id)       REFERENCES resources(id)       ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Which groups can access which resource groups
CREATE TABLE IF NOT EXISTS group_access (
    group_id          CHAR(36) NOT NULL,
    resource_group_id CHAR(36) NOT NULL,
    PRIMARY KEY (group_id, resource_group_id),
    FOREIGN KEY (group_id)          REFERENCES `groups`(id)        ON DELETE CASCADE,
    FOREIGN KEY (resource_group_id) REFERENCES resource_groups(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS sessions (
    id               CHAR(36)    PRIMARY KEY,
    user_id          CHAR(36)    NOT NULL,
    client_public_key VARCHAR(64) NOT NULL,
    assigned_ip      VARCHAR(45) NOT NULL,
    created_at       TIMESTAMP   NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_keepalive   TIMESTAMP   NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    INDEX idx_last_keepalive (last_keepalive),
    INDEX idx_user_id (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Pre-populated IP pool (filled at server startup from vpn_subnet setting)
CREATE TABLE IF NOT EXISTS ip_pool (
    ip         VARCHAR(45) PRIMARY KEY,
    in_use     TINYINT(1)  NOT NULL DEFAULT 0,
    session_id CHAR(36)    DEFAULT NULL,
    INDEX idx_in_use (in_use)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
