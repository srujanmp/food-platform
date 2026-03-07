CREATE TABLE IF NOT EXISTS menu_items (
    id              SERIAL          PRIMARY KEY,
    restaurant_id   BIGINT          NOT NULL REFERENCES restaurants(id),
    name            VARCHAR(255)    NOT NULL,
    description     TEXT,
    price           DECIMAL(10,2)   NOT NULL,
    category        VARCHAR(100),
    is_veg          BOOLEAN         NOT NULL DEFAULT TRUE,
    is_available    BOOLEAN         NOT NULL DEFAULT TRUE,
    image_url       VARCHAR(255),
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ     NOT NULL DEFAULT now()
);

CREATE INDEX idx_menu_items_restaurant_id ON menu_items(restaurant_id);
CREATE INDEX idx_menu_items_category      ON menu_items(category);
