CREATE TABLE IF NOT EXISTS outbox_events (
    id          SERIAL PRIMARY KEY,
    event_type  VARCHAR(100),
    payload     JSONB,
    published   BOOLEAN DEFAULT false,
    created_at  TIMESTAMPTZ DEFAULT now()
);
