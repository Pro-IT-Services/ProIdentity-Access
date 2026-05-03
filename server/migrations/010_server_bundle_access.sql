-- Direct attachment of bundles to servers, decoupling the access path from
-- groups/roles. New flow: Person -> Server -> Bundle -> Resource.
CREATE TABLE IF NOT EXISTS server_bundle_access (
    server_id CHAR(36) NOT NULL,
    bundle_id CHAR(36) NOT NULL,
    PRIMARY KEY (server_id, bundle_id),
    INDEX idx_sba_bundle (bundle_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Backfill: derive direct server->bundle from the existing role chain,
-- so users keep the same access after the switch.
INSERT IGNORE INTO server_bundle_access (server_id, bundle_id)
SELECT DISTINCT gsa.server_id, ga.resource_group_id
FROM group_server_access gsa
JOIN group_access ga ON ga.group_id = gsa.group_id;

-- Backfill: derive direct user->server from group membership × group->server,
-- so existing users still see their servers from /api/v1/servers.
INSERT IGNORE INTO user_server_access (user_id, server_id)
SELECT DISTINCT ug.user_id, gsa.server_id
FROM user_groups ug
JOIN group_server_access gsa ON gsa.group_id = ug.group_id;
