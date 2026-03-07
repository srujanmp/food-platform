CREATE TABLE IF NOT EXISTS drivers (
    id          SERIAL PRIMARY KEY,
    auth_id     BIGINT NOT NULL UNIQUE,
    name        VARCHAR(100),
    phone       VARCHAR(20),
    latitude    DECIMAL(10,8),
    longitude   DECIMAL(11,8),
    is_available BOOLEAN DEFAULT true,
    created_at  TIMESTAMPTZ DEFAULT now(),
    updated_at  TIMESTAMPTZ DEFAULT now()
);
