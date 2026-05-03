CREATE TABLE IF NOT EXISTS audit_logs (
    id              CHAR(36)     PRIMARY KEY,
    ts              DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    -- Actor: user_id may be NULL for anonymous events (failed login).
    actor_user_id   CHAR(36)     NULL,
    actor_username  VARCHAR(64)  NULL,
    -- Action verb (HTTP method) and the resource path it acted on.
    method          VARCHAR(8)   NOT NULL,
    path            VARCHAR(255) NOT NULL,
    -- Optional structured fields filled by explicit log calls.
    action          VARCHAR(64)  NULL, -- e.g. "auth.login", "user.create", "session.terminate"
    target_type     VARCHAR(32)  NULL, -- e.g. "user", "server", "session"
    target_id       VARCHAR(64)  NULL,
    target_label    VARCHAR(128) NULL, -- human-readable name at log time
    -- Outcome.
    status_code     INT          NOT NULL,
    success         TINYINT(1)   NOT NULL,
    error_message   VARCHAR(255) NULL,
    -- Network context.
    ip              VARCHAR(45)  NULL,
    user_agent      VARCHAR(255) NULL,
    -- Free-form JSON for extra context (e.g. before/after diffs).
    detail          JSON         NULL,
    INDEX idx_audit_ts (ts),
    INDEX idx_audit_actor (actor_user_id, ts),
    INDEX idx_audit_target (target_type, target_id, ts),
    INDEX idx_audit_action (action, ts)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
