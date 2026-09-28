-- Text a password manager (e.g. RoboForm) matches the saved login against.
-- The desktop client puts it in its window title while the connect form for
-- the profile is open. NULL = use the profile name.
ALTER TABLE openvpn_profiles ADD COLUMN IF NOT EXISTS autofill_name VARCHAR(255) NULL AFTER name;
