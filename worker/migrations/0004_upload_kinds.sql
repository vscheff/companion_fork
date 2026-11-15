ALTER TABLE receipts ADD COLUMN kind TEXT NOT NULL DEFAULT 'wowcensus';
ALTER TABLE receipts ADD COLUMN mode TEXT NOT NULL DEFAULT 'full';
ALTER TABLE receipts ADD COLUMN addon TEXT;
CREATE INDEX receipts_kind_status ON receipts (kind, status, created_at);
