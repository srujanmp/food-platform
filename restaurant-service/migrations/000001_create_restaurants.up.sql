CREATE TABLE IF NOT EXISTS restaurants (
    id              SERIAL          PRIMARY KEY,
    owner_id        BIGINT          NOT NULL,
    name            VARCHAR(255)    NOT NULL,
    address         TEXT,
    latitude        DECIMAL(10,8),
    longitude       DECIMAL(11,8),
    cuisine         VARCHAR(100),
    avg_rating      DECIMAL(3,2)    DEFAULT 0.0,
    is_open         BOOLEAN         NOT NULL DEFAULT TRUE,
    is_approved     BOOLEAN         NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ     NOT NULL DEFAULT now()
);

CREATE INDEX idx_restaurants_owner_id ON restaurants(owner_id);
