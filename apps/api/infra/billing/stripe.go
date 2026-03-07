// Package billing provides Stripe integration for subscriptions and payments.
package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/customer"
	"github.com/stripe/stripe-go/v82/paymentintent"
	"github.com/stripe/stripe-go/v82/subscription"
	"github.com/stripe/stripe-go/v82/webhook"

	"baseapp/gen/db"
)

var (
	ErrInvalidWebhook     = errors.New("invalid webhook signature")
	ErrInvalidConfig      = errors.New("invalid billing config")
	ErrCustomerNotFound   = errors.New("customer not found")
	ErrSubscriptionExists = errors.New("workspace already has active subscription")
	ErrMissingMetadata    = errors.New("missing required metadata")
)

// Config holds Stripe configuration.
type Config struct {
	SecretKey     string
	WebhookSecret string
	SuccessURL    string
	CancelURL     string
}

// Service provides billing operations backed by Stripe.
type Service struct {
	q      *db.Queries
	config Config
}

// NewService creates a billing service. Returns an error if required config fields are missing.
func NewService(q *db.Queries, config Config) (*Service, error) {
	if config.SecretKey == "" {
		return nil, fmt.Errorf("%w: SecretKey is required", ErrInvalidConfig)
	}
	if config.WebhookSecret == "" {
		return nil, fmt.Errorf("%w: WebhookSecret is required", ErrInvalidConfig)
	}
	if config.SuccessURL == "" {
		return nil, fmt.Errorf("%w: SuccessURL is required", ErrInvalidConfig)
	}
	if config.CancelURL == "" {
		return nil, fmt.Errorf("%w: CancelURL is required", ErrInvalidConfig)
	}
	stripe.Key = config.SecretKey
	return &Service{q: q, config: config}, nil
}

// --- Customer Management ---

// EnsureCustomer creates or retrieves a Stripe customer for a workspace.
func (s *Service) EnsureCustomer(ctx context.Context, workspaceID uuid.UUID, email string) (string, error) {
	bd, err := s.q.GetBillingDetailsByWorkspace(ctx, workspaceID)
	if err == nil && bd.StripeCustomerID.Valid {
		return bd.StripeCustomerID.String, nil
	}

	params := &stripe.CustomerParams{
		Email: stripe.String(email),
		Metadata: map[string]string{
			"workspace_id": workspaceID.String(),
		},
	}
	cust, err := customer.New(params)
	if err != nil {
		return "", fmt.Errorf("create stripe customer: %w", err)
	}

	_, err = s.q.UpsertBillingDetails(ctx, db.UpsertBillingDetailsParams{
		WorkspaceID:      workspaceID,
		StripeCustomerID: pgtext(cust.ID),
		BillingEmail:     pgtext(email),
	})
	if err != nil {
		return "", fmt.Errorf("upsert billing details: %w", err)
	}

	return cust.ID, nil
}

// GetCustomerID retrieves the Stripe customer ID for a workspace.
func (s *Service) GetCustomerID(ctx context.Context, workspaceID uuid.UUID) (string, error) {
	bd, err := s.q.GetBillingDetailsByWorkspace(ctx, workspaceID)
	if err != nil {
		return "", ErrCustomerNotFound
	}
	if !bd.StripeCustomerID.Valid {
		return "", ErrCustomerNotFound
	}
	return bd.StripeCustomerID.String, nil
}

// --- Subscription Management ---

// CreateCheckoutSession creates a Stripe Checkout session for subscription.
func (s *Service) CreateCheckoutSession(ctx context.Context, workspaceID uuid.UUID, priceID string) (string, error) {
	// Check for existing subscription
	_, err := s.q.GetSubscriptionByWorkspace(ctx, workspaceID)
	if err == nil {
		return "", ErrSubscriptionExists
	}

	customerID, err := s.GetCustomerID(ctx, workspaceID)
	if err != nil {
		return "", err
	}

	params := &stripe.CheckoutSessionParams{
		Customer: stripe.String(customerID),
		Mode:     stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(priceID),
				Quantity: stripe.Int64(1),
			},
		},
		SuccessURL: stripe.String(s.config.SuccessURL),
		CancelURL:  stripe.String(s.config.CancelURL),
		Metadata: map[string]string{
			"workspace_id": workspaceID.String(),
		},
		SubscriptionData: &stripe.CheckoutSessionSubscriptionDataParams{
			Metadata: map[string]string{
				"workspace_id": workspaceID.String(),
			},
		},
	}

	sess, err := session.New(params)
	if err != nil {
		return "", fmt.Errorf("create checkout session: %w", err)
	}

	return sess.URL, nil
}

// GetSubscription retrieves the current subscription for a workspace.
func (s *Service) GetSubscription(ctx context.Context, workspaceID uuid.UUID) (db.Subscription, error) {
	return s.q.GetSubscriptionByWorkspace(ctx, workspaceID)
}

// CancelSubscription cancels a workspace's subscription at period end.
func (s *Service) CancelSubscription(ctx context.Context, workspaceID uuid.UUID) error {
	sub, err := s.q.GetSubscriptionByWorkspace(ctx, workspaceID)
	if err != nil {
		return err
	}

	if !sub.StripeSubscriptionID.Valid {
		return errors.New("no stripe subscription to cancel")
	}

	_, err = subscription.Update(sub.StripeSubscriptionID.String, &stripe.SubscriptionParams{
		CancelAtPeriodEnd: stripe.Bool(true),
	})
	return err
}

// --- One-Time Payments (Top-ups) ---

// CreatePaymentIntent creates a payment intent for a one-time charge.
func (s *Service) CreatePaymentIntent(ctx context.Context, workspaceID uuid.UUID, amountCents int64, currency, description string) (db.Payment, string, error) {
	customerID, err := s.GetCustomerID(ctx, workspaceID)
	if err != nil {
		return db.Payment{}, "", err
	}

	params := &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(amountCents),
		Currency: stripe.String(currency),
		Customer: stripe.String(customerID),
		Metadata: map[string]string{
			"workspace_id": workspaceID.String(),
			"description":  description,
		},
		AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
			Enabled: stripe.Bool(true),
		},
	}

	pi, err := paymentintent.New(params)
	if err != nil {
		return db.Payment{}, "", fmt.Errorf("create payment intent: %w", err)
	}

	payment, err := s.q.CreatePayment(ctx, db.CreatePaymentParams{
		WorkspaceID:         workspaceID,
		StripePaymentIntent: pgtext(pi.ID),
		AmountCents:         int32(amountCents),
		Currency:            currency,
		Status:              string(pi.Status),
		Description:         pgtext(description),
		Metadata:            []byte("{}"),
	})
	if err != nil {
		return db.Payment{}, "", fmt.Errorf("record payment: %w", err)
	}

	return payment, pi.ClientSecret, nil
}

// CreatePaymentCheckout creates a Checkout session for one-time payment.
func (s *Service) CreatePaymentCheckout(ctx context.Context, workspaceID uuid.UUID, amountCents int64, currency, description string) (string, error) {
	customerID, err := s.GetCustomerID(ctx, workspaceID)
	if err != nil {
		return "", err
	}

	params := &stripe.CheckoutSessionParams{
		Customer: stripe.String(customerID),
		Mode:     stripe.String(string(stripe.CheckoutSessionModePayment)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency:   stripe.String(currency),
					UnitAmount: stripe.Int64(amountCents),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name:        stripe.String(description),
						Description: stripe.String("One-time payment"),
					},
				},
				Quantity: stripe.Int64(1),
			},
		},
		SuccessURL: stripe.String(s.config.SuccessURL),
		CancelURL:  stripe.String(s.config.CancelURL),
		Metadata: map[string]string{
			"workspace_id": workspaceID.String(),
			"description":  description,
		},
		PaymentIntentData: &stripe.CheckoutSessionPaymentIntentDataParams{
			Metadata: map[string]string{
				"workspace_id": workspaceID.String(),
			},
		},
	}

	sess, err := session.New(params)
	if err != nil {
		return "", fmt.Errorf("create payment checkout: %w", err)
	}

	return sess.URL, nil
}

// --- Webhook Handling ---

// webhookHandler is a function that handles a specific webhook event type.
type webhookHandler func(s *Service, ctx context.Context, event *stripe.Event) error

// webhookHandlers maps Stripe event types to their handlers.
var webhookHandlers = map[stripe.EventType]webhookHandler{
	stripe.EventTypeCheckoutSessionCompleted:      (*Service).handleCheckoutCompleted,
	stripe.EventTypeCustomerSubscriptionCreated:   (*Service).handleSubscriptionUpdated,
	stripe.EventTypeCustomerSubscriptionUpdated:   (*Service).handleSubscriptionUpdated,
	stripe.EventTypeCustomerSubscriptionDeleted:   (*Service).handleSubscriptionDeleted,
	stripe.EventTypeInvoicePaid:                   (*Service).handleInvoicePaid,
	stripe.EventTypeInvoicePaymentFailed:          (*Service).handleInvoicePaymentFailed,
	stripe.EventTypePaymentIntentSucceeded:        (*Service).handlePaymentIntentSucceeded,
	stripe.EventTypePaymentIntentPaymentFailed:    (*Service).handlePaymentIntentFailed,
}

// ParseWebhook verifies and parses an incoming webhook request.
func (s *Service) ParseWebhook(r *http.Request) (*stripe.Event, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 65536))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	sig := r.Header.Get("Stripe-Signature")
	event, err := webhook.ConstructEvent(body, sig, s.config.WebhookSecret)
	if err != nil {
		return nil, ErrInvalidWebhook
	}

	return &event, nil
}

// HandleWebhook processes a webhook event and updates the database accordingly.
func (s *Service) HandleWebhook(ctx context.Context, event *stripe.Event) error {
	handler, ok := webhookHandlers[event.Type]
	if !ok {
		return nil // Unknown event type, ignore
	}
	return handler(s, ctx, event)
}

func (s *Service) handleCheckoutCompleted(ctx context.Context, event *stripe.Event) error {
	var sess stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
		return err
	}

	workspaceID, err := uuid.Parse(sess.Metadata["workspace_id"])
	if err != nil {
		return fmt.Errorf("%w: workspace_id in checkout session", ErrMissingMetadata)
	}

	// Subscription checkout - the subscription webhook will handle details
	if sess.Mode == stripe.CheckoutSessionModeSubscription {
		return nil
	}

	// One-time payment checkout
	if sess.PaymentIntent == nil {
		return nil
	}

	pi, err := paymentintent.Get(sess.PaymentIntent.ID, nil)
	if err != nil {
		return err
	}

	_, err = s.q.CreatePayment(ctx, db.CreatePaymentParams{
		WorkspaceID:         workspaceID,
		StripePaymentIntent: pgtext(pi.ID),
		AmountCents:         int32(pi.Amount),
		Currency:            string(pi.Currency),
		Status:              string(pi.Status),
		Description:         pgtext(sess.Metadata["description"]),
		Metadata:            []byte("{}"),
	})
	if err != nil {
		// Might already exist if created via CreatePaymentIntent
		return s.q.UpdatePaymentStatus(ctx, db.UpdatePaymentStatusParams{
			StripePaymentIntent: pgtext(pi.ID),
			Status:              string(pi.Status),
		})
	}
	return nil
}

func (s *Service) handleSubscriptionUpdated(ctx context.Context, event *stripe.Event) error {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		return err
	}

	workspaceID, err := uuid.Parse(sub.Metadata["workspace_id"])
	if err != nil {
		return fmt.Errorf("%w: workspace_id in subscription", ErrMissingMetadata)
	}

	stripeSubID := pgtext(sub.ID)

	// Try to get existing subscription
	_, err = s.q.GetSubscriptionByStripeID(ctx, stripeSubID)
	if err != nil {
		// Create new subscription record
		// TODO: To support multiple plans, look up plan from price ID via config or database.
		_, err = s.q.CreateSubscription(ctx, db.CreateSubscriptionParams{
			WorkspaceID:          workspaceID,
			PlanID:               "pro",
			StripeSubscriptionID: stripeSubID,
			Status:               string(sub.Status),
			CurrentPeriodStart:   pgtimestamp(sub.StartDate),
			CurrentPeriodEnd:     pgtimestamp(sub.CancelAt),
		})
		return err
	}

	// Update existing subscription
	return s.q.UpdateSubscriptionStatus(ctx, db.UpdateSubscriptionStatusParams{
		StripeSubscriptionID: stripeSubID,
		Status:               string(sub.Status),
	})
}

func (s *Service) handleSubscriptionDeleted(ctx context.Context, event *stripe.Event) error {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		return err
	}
	return s.q.CancelSubscription(ctx, pgtext(sub.ID))
}

func (s *Service) handleInvoicePaid(ctx context.Context, event *stripe.Event) error {
	var inv stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &inv); err != nil {
		return err
	}

	// Check if this invoice is for a subscription
	if inv.Parent != nil && inv.Parent.SubscriptionDetails != nil && inv.Parent.SubscriptionDetails.Subscription != nil {
		subID := inv.Parent.SubscriptionDetails.Subscription.ID
		// Update subscription period from invoice
		_ = s.q.UpdateSubscriptionPeriod(ctx, db.UpdateSubscriptionPeriodParams{
			StripeSubscriptionID: pgtext(subID),
			CurrentPeriodStart:   pgtimestamp(inv.PeriodStart),
			CurrentPeriodEnd:     pgtimestamp(inv.PeriodEnd),
		})
		return s.q.UpdateSubscriptionStatus(ctx, db.UpdateSubscriptionStatusParams{
			StripeSubscriptionID: pgtext(subID),
			Status:               "active",
		})
	}
	return nil
}

func (s *Service) handleInvoicePaymentFailed(ctx context.Context, event *stripe.Event) error {
	var inv stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &inv); err != nil {
		return err
	}

	if inv.Parent != nil && inv.Parent.SubscriptionDetails != nil && inv.Parent.SubscriptionDetails.Subscription != nil {
		subID := inv.Parent.SubscriptionDetails.Subscription.ID
		return s.q.UpdateSubscriptionStatus(ctx, db.UpdateSubscriptionStatusParams{
			StripeSubscriptionID: pgtext(subID),
			Status:               "past_due",
		})
	}
	return nil
}

func (s *Service) handlePaymentIntentSucceeded(ctx context.Context, event *stripe.Event) error {
	var pi stripe.PaymentIntent
	if err := json.Unmarshal(event.Data.Raw, &pi); err != nil {
		return err
	}
	return s.q.UpdatePaymentStatus(ctx, db.UpdatePaymentStatusParams{
		StripePaymentIntent: pgtext(pi.ID),
		Status:              "succeeded",
	})
}

func (s *Service) handlePaymentIntentFailed(ctx context.Context, event *stripe.Event) error {
	var pi stripe.PaymentIntent
	if err := json.Unmarshal(event.Data.Raw, &pi); err != nil {
		return err
	}
	return s.q.UpdatePaymentStatus(ctx, db.UpdatePaymentStatusParams{
		StripePaymentIntent: pgtext(pi.ID),
		Status:              "failed",
	})
}

// --- Helpers ---

func pgtext(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}

func pgtimestamp(ts int64) pgtype.Timestamptz {
	if ts == 0 {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: time.Unix(ts, 0), Valid: true}
}
