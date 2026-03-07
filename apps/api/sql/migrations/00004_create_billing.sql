-- +goose Up

-- Static plan catalog
CREATE TABLE plans (
    id         TEXT PRIMARY KEY,
    name       VARCHAR(100) NOT NULL,
    prefs      JSONB NOT NULL DEFAULT '{}',
    is_active  BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO plans (id, name) VALUES
    ('free',       'Free'),
    ('pro',        'Pro'),
    ('enterprise', 'Enterprise');

-- One billing profile per workspace
CREATE TABLE billing_details (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id        UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    stripe_customer_id  TEXT,
    billing_email       VARCHAR(255),
    billing_address     JSONB NOT NULL DEFAULT '{}',
    tax_ids             JSONB NOT NULL DEFAULT '[]',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_billing_details_workspace
    ON billing_details (workspace_id);

CREATE UNIQUE INDEX idx_billing_details_stripe_customer
    ON billing_details (stripe_customer_id)
    WHERE stripe_customer_id IS NOT NULL;

-- One active subscription per workspace
CREATE TABLE subscriptions (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id           UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    plan_id                TEXT NOT NULL REFERENCES plans(id),
    stripe_subscription_id TEXT,
    status                 TEXT NOT NULL DEFAULT 'active',
    current_period_start   TIMESTAMPTZ,
    current_period_end     TIMESTAMPTZ,
    canceled_at            TIMESTAMPTZ,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_subscriptions_workspace
    ON subscriptions (workspace_id)
    WHERE canceled_at IS NULL;

CREATE UNIQUE INDEX idx_subscriptions_stripe
    ON subscriptions (stripe_subscription_id)
    WHERE stripe_subscription_id IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS subscriptions;
DROP TABLE IF EXISTS billing_details;
DROP TABLE IF EXISTS plans;
