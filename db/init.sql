CREATE SCHEMA IF NOT EXISTS shorty;

CREATE TABLE IF NOT EXISTS shorty.links (
    id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    long_url          TEXT NOT NULL UNIQUE,
    short_url_code    VARCHAR(10) NOT NULL UNIQUE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS shorty.click_events (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    link_id     BIGINT NOT NULL REFERENCES shorty.links(id) ON DELETE CASCADE,
    clicked_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS click_events_link_time_idx
    ON shorty.click_events (link_id, clicked_at);
