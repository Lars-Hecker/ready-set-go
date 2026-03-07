-- name: CreateSession :one
INSERT INTO sessions (
    user_id,
    refresh_token_hash,
    device_type,
    device_name,
    browser,
    os,
    ip_address,
    country_code,
    city,
    expires_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
RETURNING *;

-- name: GetSessionByTokenHash :one
SELECT * FROM sessions
WHERE refresh_token_hash = $1
  AND revoked_at IS NULL
  AND expires_at > now();

-- name: GetActiveSessionsByUserID :many
SELECT * FROM sessions
WHERE user_id = $1
  AND revoked_at IS NULL
  AND expires_at > now()
ORDER BY last_activity_at DESC;

-- name: RotateRefreshToken :one
UPDATE sessions
SET refresh_token_hash = $2,
    last_activity_at = now(),
    expires_at = $3
WHERE id = $1
  AND revoked_at IS NULL
RETURNING *;

-- name: RevokeSession :exec
UPDATE sessions
SET revoked_at = now()
WHERE id = $1
  AND user_id = $2
  AND revoked_at IS NULL;

-- name: RevokeAllUserSessions :exec
UPDATE sessions
SET revoked_at = now()
WHERE user_id = $1
  AND revoked_at IS NULL;

-- name: RevokeAllUserSessionsExcept :exec
UPDATE sessions
SET revoked_at = now()
WHERE user_id = $1
  AND id != $2
  AND revoked_at IS NULL;

-- name: TouchSession :exec
UPDATE sessions
SET last_activity_at = now()
WHERE id = $1
  AND revoked_at IS NULL;
