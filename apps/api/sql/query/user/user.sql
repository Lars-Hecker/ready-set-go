-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: CreateUser :one
INSERT INTO users (email, name)
VALUES ($1, $2)
RETURNING *;

-- name: UpdateUserProfile :one
UPDATE users
SET name       = coalesce(sqlc.narg('name'), name),
    username   = coalesce(sqlc.narg('username'), username),
    avatar_url = coalesce(sqlc.narg('avatar_url'), avatar_url),
    updated_at = now()
WHERE id = sqlc.arg('id')
RETURNING *;
