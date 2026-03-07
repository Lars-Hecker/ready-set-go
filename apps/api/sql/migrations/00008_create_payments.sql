-- +goose Up

-- Track one-time payments (top-ups, credits, etc.)
CREATE TABLE payments (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id          UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    stripe_payment_intent TEXT,
    amount_cents          INTEGER NOT NULL,
    currency              VARCHAR(3) NOT NULL DEFAULT 'usd',
    status                TEXT NOT NULL DEFAULT 'pending',
    description           TEXT,
    metadata              JSONB NOT NULL DEFAULT '{}',
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_payments_workspace ON payments (workspace_id);

CREATE UNIQUE INDEX idx_payments_stripe_intent
    ON payments (stripe_payment_intent)
    WHERE stripe_payment_intent IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS payments;