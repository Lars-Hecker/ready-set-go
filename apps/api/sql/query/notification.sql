-- name: CreateDeviceToken :one
INSERT INTO device_tokens (user_id, platform, token, endpoint_arn, p256dh, auth, device_name)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (token) DO UPDATE SET
    user_id = EXCLUDED.user_id,
    platform = EXCLUDED.platform,
    endpoint_arn = EXCLUDED.endpoint_arn,
    p256dh = EXCLUDED.p256dh,
    auth = EXCLUDED.auth,
    device_name = EXCLUDED.device_name,
    is_active = TRUE,
    last_used_at = now()
RETURNING *;

-- name: GetDeviceToken :one
SELECT * FROM device_tokens WHERE id = $1;

-- name: GetDeviceTokenByToken :one
SELECT * FROM device_tokens WHERE token = $1;

-- name: ListActiveDeviceTokensByUser :many
SELECT * FROM device_tokens
WHERE user_id = $1 AND is_active = TRUE
ORDER BY last_used_at DESC;

-- name: DeactivateDeviceToken :exec
UPDATE device_tokens
SET is_active = FALSE
WHERE id = $1;

-- name: DeactivateDeviceTokenByToken :exec
UPDATE device_tokens
SET is_active = FALSE
WHERE token = $1;

-- name: UpdateDeviceTokenLastUsed :exec
UPDATE device_tokens
SET last_used_at = now()
WHERE id = $1;

-- name: GetNotificationPreferences :one
SELECT * FROM notification_preferences WHERE user_id = $1;

-- name: UpsertNotificationPreferences :one
INSERT INTO notification_preferences (user_id, email_enabled, push_enabled, channel_prefs)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id) DO UPDATE SET
    email_enabled = EXCLUDED.email_enabled,
    push_enabled = EXCLUDED.push_enabled,
    channel_prefs = EXCLUDED.channel_prefs,
    updated_at = now()
RETURNING *;

-- name: CreateNotificationLog :one
INSERT INTO notification_log (user_id, channel, recipient, subject, body, status)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateNotificationLogStatus :exec
UPDATE notification_log
SET status = $2, error = $3, sent_at = CASE WHEN $2 = 'sent' THEN now() ELSE sent_at END
WHERE id = $1;

-- name: GetNotificationLog :one
SELECT * FROM notification_log WHERE id = $1;

-- name: ListNotificationLogsByUser :many
SELECT * FROM notification_log
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;
