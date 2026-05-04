-- Add type, mask, and ports fields to resources

ALTER TABLE resources
  ADD COLUMN IF NOT EXISTS type  VARCHAR(16)  NOT NULL DEFAULT 'host'  AFTER ip_address,
  ADD COLUMN IF NOT EXISTS mask  TINYINT      DEFAULT NULL              AFTER type,
  ADD COLUMN IF NOT EXISTS ports VARCHAR(255) DEFAULT NULL              AFTER mask;
