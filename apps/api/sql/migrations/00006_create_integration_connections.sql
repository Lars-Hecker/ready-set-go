-- +goose Up
CREATE TABLE integration_connections (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider      VARCHAR(50) NOT NULL,
    access_token  TEXT NOT NULL,
    refresh_token TEXT,
    expires_at    TIMESTAMPTZ,
    scopes        JSONB NOT NULL DEFAULT '[]',
    extra_data    JSONB NOT NULL DEFAULT '{}',
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_integrations_user_provider
    ON integration_connections (user_id, provider);

CREATE INDEX idx_integrations_provider_active
    ON integration_connections (provider)
    WHERE is_active = TRUE;

-- +goose Down
DROP TABLE IF EXISTS integration_connections;
