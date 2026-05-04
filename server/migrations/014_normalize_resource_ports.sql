-- Normalize legacy resource port values saved with UI labels such as
-- "ports: 53,80" or "ports: ports: 53,80".

UPDATE resources
SET ports = TRIM(SUBSTRING(ports, 7))
WHERE LOWER(TRIM(ports)) LIKE 'ports:%';

UPDATE resources
SET ports = TRIM(SUBSTRING(ports, 7))
WHERE LOWER(TRIM(ports)) LIKE 'ports:%';

UPDATE resources
SET ports = NULL
WHERE ports IS NOT NULL AND LOWER(TRIM(ports)) IN ('', 'all', 'all ports');
