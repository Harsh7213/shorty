CREATE SCHEMA IF NOT EXISTS shorty;

CREATE TABLE IF NOT EXISTS shorty.users (
    id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email             VARCHAR(254) NOT NULL UNIQUE,
    display_name      TEXT NOT NULL DEFAULT '',
    password_hash     TEXT NOT NULL,
    password_setup_required BOOLEAN NOT NULL DEFAULT FALSE,
    email_verified_at TIMESTAMPTZ,
    deactivated_at    TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS shorty.email_verification_tokens (
    user_id     BIGINT PRIMARY KEY REFERENCES shorty.users(id) ON DELETE CASCADE,
    token_hash  CHAR(64) NOT NULL UNIQUE,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS shorty.organizations (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name          TEXT NOT NULL,
    owner_user_id BIGINT NOT NULL REFERENCES shorty.users(id) ON DELETE CASCADE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS shorty.organization_members (
    organization_id BIGINT NOT NULL REFERENCES shorty.organizations(id) ON DELETE CASCADE,
    user_id         BIGINT NOT NULL REFERENCES shorty.users(id) ON DELETE CASCADE,
    permissions     TEXT[] NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (organization_id, user_id)
);

CREATE TABLE IF NOT EXISTS shorty.links (
    id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    organization_id   BIGINT NOT NULL REFERENCES shorty.organizations(id) ON DELETE CASCADE,
    created_by_user_id BIGINT REFERENCES shorty.users(id) ON DELETE SET NULL,
    created_by_email  TEXT NOT NULL,
    created_by_display_name TEXT NOT NULL,
    long_url          TEXT NOT NULL,
    short_url_code    VARCHAR(10) NOT NULL UNIQUE,
    status            VARCHAR(10) NOT NULL DEFAULT 'active',
    revoked_at        TIMESTAMPTZ,
    expires_at        TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT link_status_check CHECK (status IN ('active', 'disabled', 'revoked')),
    CONSTRAINT link_revoked_at_check CHECK ((status = 'revoked') = (revoked_at IS NOT NULL))
);

CREATE INDEX IF NOT EXISTS organization_members_user_idx ON shorty.organization_members (user_id, organization_id);
CREATE UNIQUE INDEX IF NOT EXISTS organization_members_one_org_per_user_idx
    ON shorty.organization_members (user_id);

CREATE INDEX IF NOT EXISTS links_organization_idx ON shorty.links (organization_id);

CREATE TABLE IF NOT EXISTS shorty.click_events (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    link_id     BIGINT NOT NULL REFERENCES shorty.links(id) ON DELETE CASCADE,
    clicked_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS click_events_link_time_idx
    ON shorty.click_events (link_id, clicked_at);
