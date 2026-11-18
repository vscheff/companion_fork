-- Legacy public registration stored a SHA-256 hash and up to two version/OS label parts.
UPDATE devices
SET label = 'companion'
WHERE length(reg_ip_hash) = 64
  AND reg_ip_hash NOT GLOB '*[^0-9a-f]*'
  AND label GLOB 'companion [A-Za-z0-9_.+-]*'
  AND label NOT GLOB '*[^A-Za-z0-9_.+ -]*'
  AND label NOT GLOB 'companion * * *'
  AND label NOT GLOB '* ';

DROP INDEX IF EXISTS devices_reg_ip;
ALTER TABLE devices DROP COLUMN reg_ip_hash;
