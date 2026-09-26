-- OpenVPN profiles and per-user assignments.
--
-- Profiles hold a full .ovpn (encrypted at rest with the server profile key).
-- A profile can be assigned to many users (shared corporate profile) or a
-- single user. `requires_totp` marks profiles whose password field expects an
-- external TOTP code appended to the password. `dev_type` is auto-detected from
-- the uploaded config (tun/tap); `allow_custom_ip` enables a user-set address
-- for tap profiles (desktop only — mobile is tun-only).

CREATE TABLE IF NOT EXISTS openvpn_profiles (
    id               CHAR(36)     PRIMARY KEY,
    name             VARCHAR(255) NOT NULL,
    description      TEXT         NULL,
    config_encrypted LONGBLOB     NOT NULL,
    requires_totp    TINYINT(1)   NOT NULL DEFAULT 0,
    auth_user_pass   TINYINT(1)   NOT NULL DEFAULT 1,
    dev_type         VARCHAR(8)   NOT NULL DEFAULT 'tun',
    allow_custom_ip  TINYINT(1)   NOT NULL DEFAULT 0,
    created_by       CHAR(36)     NULL,
    created_at       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS openvpn_profile_assignments (
    id          CHAR(36)     PRIMARY KEY,
    profile_id  CHAR(36)     NOT NULL,
    user_id     CHAR(36)     NOT NULL,
    custom_ip   VARCHAR(64)  NULL,
    assigned_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uq_openvpn_assignment (profile_id, user_id),
    INDEX idx_openvpn_assign_user (user_id),
    FOREIGN KEY (profile_id) REFERENCES openvpn_profiles(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id)    REFERENCES users(id)            ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
