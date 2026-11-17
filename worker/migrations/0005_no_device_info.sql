DROP INDEX IF EXISTS devices_reg_ip;
ALTER TABLE devices DROP COLUMN reg_ip_hash;
UPDATE devices SET label = 'companion' WHERE label LIKE 'companion%';
