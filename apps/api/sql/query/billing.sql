-- Plans

-- name: GetPlan :one
SELECT * FROM plans WHERE id = $1;

-- name: GetActivePlans :many
SELECT * FROM plans WHERE is_active = TRUE ORDER BY id;

-- Billing Details

-- name: GetBillingDetailsByWorkspace :one
SELECT * FROM billing_details WHERE workspace_id = $1;

-- name: GetBillingDetailsByStripeCustomer :one
SELECT * FROM billing_details WHERE stripe_customer_id = $1;

-- name: CreateBillingDetails :one
INSERT INTO billing_details (workspace_id, stripe_customer_id, billing_email)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateBillingDetailsStripeCustomer :exec
UPDATE billing_details
SET stripe_customer_id = $2, updated_at = now()
WHERE workspace_id = $1;

-- name: UpdateBillingDetailsEmail :exec
UPDATE billing_details
SET billing_email = $2, updated_at = now()
WHERE workspace_id = $1;

-- name: UpdateBillingDetailsAddress :exec
UPDATE billing_details
SET billing_address = $2, updated_at = now()
WHERE workspace_id = $1;

-- name: UpsertBillingDetails :one
INSERT INTO billing_details (workspace_id, stripe_customer_id, billing_email)
VALUES ($1, $2, $3)
ON CONFLICT (workspace_id) DO UPDATE
SET stripe_customer_id = COALESCE(EXCLUDED.stripe_customer_id, billing_details.stripe_customer_id),
    billing_email = COALESCE(EXCLUDED.billing_email, billing_details.billing_email),
    updated_at = now()
RETURNING *;

-- Subscriptions

-- name: GetSubscriptionByWorkspace :one
SELECT * FROM subscriptions
WHERE workspace_id = $1 AND canceled_at IS NULL;

-- name: GetSubscriptionByStripeID :one
SELECT * FROM subscriptions WHERE stripe_subscription_id = $1;

-- name: CreateSubscription :one
INSERT INTO subscriptions (
    workspace_id, plan_id, stripe_subscription_id, status,
    current_period_start, current_period_end
)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateSubscriptionStatus :exec
UPDATE subscriptions
SET status = $2, updated_at = now()
WHERE stripe_subscription_id = $1;

-- name: UpdateSubscriptionPeriod :exec
UPDATE subscriptions
SET current_period_start = $2, current_period_end = $3, updated_at = now()
WHERE stripe_subscription_id = $1;

-- name: UpdateSubscriptionPlan :exec
UPDATE subscriptions
SET plan_id = $2, updated_at = now()
WHERE stripe_subscription_id = $1;

-- name: CancelSubscription :exec
UPDATE subscriptions
SET canceled_at = now(), status = 'canceled', updated_at = now()
WHERE stripe_subscription_id = $1;

-- Payments (one-time charges / top-ups)

-- name: GetPaymentByID :one
SELECT * FROM payments WHERE id = $1;

-- name: GetPaymentByStripeIntent :one
SELECT * FROM payments WHERE stripe_payment_intent = $1;

-- name: GetPaymentsByWorkspace :many
SELECT * FROM payments
WHERE workspace_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CreatePayment :one
INSERT INTO payments (
    workspace_id, stripe_payment_intent, amount_cents, currency, status, description, metadata
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: UpdatePaymentStatus :exec
UPDATE payments
SET status = $2, updated_at = now()
WHERE stripe_payment_intent = $1;

-- name: UpdatePaymentStatusByID :exec
UPDATE payments
SET status = $2, updated_at = now()
WHERE id = $1;