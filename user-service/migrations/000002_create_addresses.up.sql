-- 000002_create_addresses.up.sql
CREATE TABLE addresses (
    id          SERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL,
    label       VARCHAR(50),
    line1       TEXT,
    city        VARCHAR(100),
    pincode     VARCHAR(10),
    latitude    DECIMAL(10,8),
    longitude   DECIMAL(11,8),
    is_default  BOOLEAN NOT NULL DEFAULT false
);

CREATE INDEX idx_addresses_user_id ON addresses(user_id);