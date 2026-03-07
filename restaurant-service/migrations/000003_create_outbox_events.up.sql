CREATE TABLE IF NOT EXISTS outbox_events (
    id              SERIAL          PRIMARY KEY,
    event_type      VARCHAR(100)    NOT NULL,
    payload         JSONB           NOT NULL,
    published       BOOLEAN         NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT now()
);

CREATE INDEX idx_outbox_events_published ON outbox_events(published) WHERE published = FALSE;
