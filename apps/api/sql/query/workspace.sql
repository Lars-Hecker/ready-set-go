-- name: CreateWorkspace :one
INSERT INTO workspaces (name, slug)
VALUES ($1, $2)
RETURNING *;

-- name: GetWorkspaceByID :one
SELECT * FROM workspaces
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetWorkspaceBySlug :one
SELECT * FROM workspaces
WHERE slug = $1 AND deleted_at IS NULL;

-- name: UpdateWorkspace :one
UPDATE workspaces
SET name       = coalesce(sqlc.narg('name'), name),
    slug       = coalesce(sqlc.narg('slug'), slug),
    updated_at = now()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteWorkspace :exec
UPDATE workspaces
SET deleted_at = now(), is_active = false, updated_at = now()
WHERE id = $1 AND deleted_at IS NULL;

-- name: ListWorkspacesForUser :many
SELECT w.* FROM workspaces w
JOIN workspace_memberships m ON m.workspace_id = w.id
WHERE m.user_id = $1
  AND m.status = 'active'
  AND m.deleted_at IS NULL
  AND w.deleted_at IS NULL
ORDER BY w.created_at DESC;

-- Membership queries

-- name: CreateMembership :one
INSERT INTO workspace_memberships (user_id, workspace_id, role, status, created_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetMembership :one
SELECT * FROM workspace_memberships
WHERE user_id = $1 AND workspace_id = $2 AND deleted_at IS NULL;

-- name: GetMembershipByID :one
SELECT * FROM workspace_memberships
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetPrimaryOwner :one
SELECT * FROM workspace_memberships
WHERE workspace_id = $1 AND role = 0 AND deleted_at IS NULL;

-- name: UpdateMembershipRole :one
UPDATE workspace_memberships
SET role = $1, updated_at = now()
WHERE id = $2 AND deleted_at IS NULL
RETURNING *;

-- name: UpdateMembershipStatus :one
UPDATE workspace_memberships
SET status = $1, updated_at = now()
WHERE id = $2 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteMembership :exec
UPDATE workspace_memberships
SET deleted_at = now(), status = 'removed', updated_at = now()
WHERE id = $1 AND deleted_at IS NULL;

-- name: ListMemberships :many
SELECT * FROM workspace_memberships
WHERE workspace_id = $1 AND deleted_at IS NULL
ORDER BY role ASC, created_at ASC;

-- name: ListActiveMemberships :many
SELECT * FROM workspace_memberships
WHERE workspace_id = $1 AND status = 'active' AND deleted_at IS NULL
ORDER BY role ASC, created_at ASC;

-- name: CountActiveMembers :one
SELECT count(*) FROM workspace_memberships
WHERE workspace_id = $1 AND status = 'active' AND deleted_at IS NULL;
