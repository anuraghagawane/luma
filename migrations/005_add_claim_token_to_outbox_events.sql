ALTER TABLE outbox_events ADD COLUMN IF NOT EXISTS claim_token UUID;
