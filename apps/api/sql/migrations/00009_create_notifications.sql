-- +goose Up

-- device_tokens: stores push notification tokens
CREATE TABLE device_tokens (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    platform     TEXT NOT NULL CHECK (platform IN ('ios', 'android', 'web')),
    token        TEXT NOT NULL,
    endpoint_arn TEXT,           -- SNS endpoint ARN for mobile
    p256dh       TEXT,           -- Web Push ECDH public key
    auth         TEXT,           -- Web Push auth secret
    device_name  TEXT,
    is_active    BOOLEAN NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_device_tokens_token ON device_tokens (token);
CREATE INDEX idx_device_tokens_user ON device_tokens (user_id) WHERE is_active = TRUE;

-- notification_preferences: user opt-in/out settings
CREATE TABLE notification_preferences (
    user_id       UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    email_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    push_enabled  BOOLEAN NOT NULL DEFAULT TRUE,
    channel_prefs JSONB NOT NULL DEFAULT '{}',
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- notification_log: audit trail for all notifications
CREATE TABLE notification_log (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID REFERENCES users(id) ON DELETE SET NULL,
    channel      TEXT NOT NULL,  -- 'email', 'web_push', 'mobile_push'
    recipient    TEXT NOT NULL,  -- email address, device token, or endpoint
    subject      TEXT,
    body         TEXT,
    status       TEXT NOT NULL DEFAULT 'pending',  -- 'pending', 'sent', 'failed'
    error        TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    sent_at      TIMESTAMPTZ
);
CREATE INDEX idx_notification_log_user ON notification_log (user_id);
CREATE INDEX idx_notification_log_created ON notification_log (created_at);

-- +goose Down
DROP TABLE IF EXISTS notification_log;
DROP TABLE IF EXISTS notification_preferences;
DROP TABLE IF EXISTS device_tokens;
