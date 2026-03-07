-- +goose Up
CREATE TYPE membership_status AS ENUM (
    'invited',
    'active',
    'rejected',
    'disabled',
    'removed'
);

CREATE TABLE workspace_memberships (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id               UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id          UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    role                  SMALLINT NOT NULL DEFAULT 4,
    valid_from            TIMESTAMPTZ,
    valid_until           TIMESTAMPTZ,
    prefs                 JSONB NOT NULL DEFAULT '{}',
    status                membership_status NOT NULL DEFAULT 'invited',
    invitation_expires_at TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at            TIMESTAMPTZ,
    created_by            UUID REFERENCES users(id) ON DELETE SET NULL
);

-- One active membership per user per workspace
CREATE UNIQUE INDEX idx_memberships_user_workspace
    ON workspace_memberships (user_id, workspace_id)
    WHERE deleted_at IS NULL;

-- Index on members of a workspace
CREATE INDEX idx_memberships_workspace
    ON workspace_memberships (workspace_id)
    WHERE deleted_at IS NULL;

-- +goose StatementBegin
COMMENT ON COLUMN workspace_memberships.role IS
    '0=primary_owner, 1=owner, 2=admin, 3=member, 4=guest';
-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS workspace_memberships;
DROP TYPE IF EXISTS membership_status;
