-- +migrate Up
ALTER TABLE payments
    ADD COLUMN IF NOT EXISTS amount_paise BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS provider_order_id VARCHAR(255) UNIQUE,
    ADD COLUMN IF NOT EXISTS provider_signature TEXT,
    ADD COLUMN IF NOT EXISTS failure_code VARCHAR(100),
    ADD COLUMN IF NOT EXISTS failure_reason TEXT;

UPDATE payments SET amount_paise = ROUND(amount * 100);
