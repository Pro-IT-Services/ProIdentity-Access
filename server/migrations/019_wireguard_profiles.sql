-- WireGuard profiles uploaded by an admin (full .conf for a WireGuard server
-- this product doesn't manage) and their per-user assignments.
--
-- A profile is one WireGuard peer: its private key can be up on only one
-- device at a time, so every profile carries a lock (holder_*). A device
-- claims it before connecting and renews the claim while connected; a claim
-- not renewed for a few minutes lapses. Profiles not marked `shared` can be
-- assigned to a single user only; shared ones to many users, who see who
-- holds the lock.

CREATE TABLE IF NOT EXISTS wireguard_profiles (
    id                     CHAR(36)     PRIMARY KEY,
    name                   VARCHAR(255) NOT NULL,
    description            TEXT         NULL,
    config_encrypted       LONGBLOB     NOT NULL,
    shared                 TINYINT(1)   NOT NULL DEFAULT 0,
    address                VARCHAR(255) NULL,
    endpoint               VARCHAR(255) NULL,
    holder_user_id         CHAR(36)     NULL,
    holder_installation_id CHAR(36)     NULL,
    holder_device_name     VARCHAR(255) NULL,
    held_since             DATETIME     NULL,
    holder_seen            DATETIME     NULL,
    created_by             CHAR(36)     NULL,
    created_at             DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at             DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (holder_user_id) REFERENCES users(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS wireguard_profile_assignments (
    id          CHAR(36)  PRIMARY KEY,
    profile_id  CHAR(36)  NOT NULL,
    user_id     CHAR(36)  NOT NULL,
    assigned_at DATETIME  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uq_wg_profile_assignment (profile_id, user_id),
    INDEX idx_wg_profile_assign_user (user_id),
    FOREIGN KEY (profile_id) REFERENCES wireguard_profiles(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id)    REFERENCES users(id)              ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
