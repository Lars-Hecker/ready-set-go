-- +goose Up
CREATE TYPE upload_status AS ENUM ('pending', 'completed', 'failed');

CREATE TABLE files (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    uploaded_by     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    s3_key          TEXT NOT NULL,
    title           VARCHAR(255) NOT NULL,
    file_size_bytes BIGINT,
    mime_type       VARCHAR(255),
    status          upload_status NOT NULL DEFAULT 'pending',
    expires_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_files_s3_key ON files (s3_key);
CREATE INDEX idx_files_workspace ON files (workspace_id);
CREATE INDEX idx_files_expires_at ON files (expires_at) WHERE status = 'pending';

-- +goose Down
DROP TABLE IF EXISTS files;
DROP TYPE IF EXISTS upload_status;
