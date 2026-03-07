-- +goose Up
CREATE TABLE sessions (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token_hash BYTEA NOT NULL,
    device_type      VARCHAR(20),
    device_name      VARCHAR(100),
    browser          VARCHAR(50),
    os               VARCHAR(50),
    ip_address       VARCHAR(45) NOT NULL,
    country_code     VARCHAR(2),
    city             VARCHAR(100),
    last_activity_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at       TIMESTAMPTZ NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at       TIMESTAMPTZ
);

CREATE INDEX idx_sessions_user_id ON sessions (user_id) WHERE revoked_at IS NULL;
CREATE INDEX idx_sessions_token_hash ON sessions (refresh_token_hash) WHERE revoked_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS sessions;
