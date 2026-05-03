-- Roles carry a set of admin permissions (string keys). NULL = no perms granted
-- by this role. Permissions are union'd across all of a user's roles.
-- Users with is_admin=1 implicitly hold every permission (kept for bootstrap +
-- emergency access).
ALTER TABLE `groups` ADD COLUMN permissions JSON NULL;
