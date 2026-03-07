CREATE TABLE IF NOT EXISTS deliveries (
    id           SERIAL PRIMARY KEY,
    order_id     BIGINT NOT NULL UNIQUE,
    driver_id    BIGINT NOT NULL REFERENCES drivers(id),
    status       VARCHAR(30) NOT NULL DEFAULT 'ASSIGNED',
    assigned_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    delivered_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ DEFAULT now(),
    updated_at   TIMESTAMPTZ DEFAULT now()
);
