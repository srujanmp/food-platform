-- +migrate Down
ALTER TABLE payments
    DROP COLUMN IF EXISTS failure_reason,
    DROP COLUMN IF EXISTS failure_code,
    DROP COLUMN IF EXISTS provider_signature,
    DROP COLUMN IF EXISTS provider_order_id,
    DROP COLUMN IF EXISTS amount_paise;
