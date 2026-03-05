-- 000001_create_profiles.up.sql
CREATE TABLE profiles (
    id          SERIAL PRIMARY KEY,
    auth_id     BIGINT NOT NULL UNIQUE,
    name        VARCHAR(100),
    avatar_url  VARCHAR(255),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);