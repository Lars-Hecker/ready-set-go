-- name: CreateFile :one
INSERT INTO files (workspace_id, uploaded_by, s3_key, title, file_size_bytes, mime_type, status, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, 'pending', $7)
RETURNING *;

-- name: GetFile :one
SELECT * FROM files WHERE id = $1;

-- name: GetFileByKey :one
SELECT * FROM files WHERE s3_key = $1;

-- name: ConfirmUpload :one
UPDATE files
SET status = 'completed', expires_at = NULL, updated_at = now()
WHERE id = $1 AND status = 'pending'
RETURNING *;

-- name: MarkUploadFailed :exec
UPDATE files
SET status = 'failed', updated_at = now()
WHERE id = $1;

-- name: ListFilesByWorkspace :many
SELECT * FROM files
WHERE workspace_id = $1 AND status = 'completed'
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: DeleteFile :exec
DELETE FROM files WHERE id = $1;

-- name: DeleteExpiredPendingFiles :many
DELETE FROM files
WHERE status = 'pending' AND expires_at < now()
RETURNING s3_key;