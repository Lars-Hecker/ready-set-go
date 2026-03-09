// Package billing provides payment provider abstractions for subscriptions and payments.
package billing

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"baseapp/gen/db"
)

var (
	ErrInvalidWebhook     = errors.New("invalid webhook signature")
	ErrInvalidConfig      = errors.New("invalid billing config")
	ErrCustomerNotFound   = errors.New("customer not found")
	ErrSubscriptionExists = errors.New("workspace already has active subscription")
	ErrMissingMetadata    = errors.New("missing required metadata")
)

// CheckoutRequest contains parameters for creating a checkout session.
type CheckoutRequest struct {
	WorkspaceID uuid.UUID
	CustomerID  string
	PriceID     string // For subscriptions
	AmountCents int64  // For one-time payments
	Currency    string
	Description string
	SuccessURL  string
	CancelURL   string
}

// CheckoutResponse contains the result of creating a checkout session.
type CheckoutResponse struct {
	URL string
}

// PaymentIntentRequest contains parameters for creating a payment intent.
type PaymentIntentRequest struct {
	WorkspaceID uuid.UUID
	CustomerID  string
	AmountCents int64
	Currency    string
	Description string
}

// PaymentIntentResponse contains the result of creating a payment intent.
type PaymentIntentResponse struct {
	ID           string
	ClientSecret string
	Status       string
}

// CustomerRequest contains parameters for creating a customer.
type CustomerRequest struct {
	WorkspaceID uuid.UUID
	Email       string
}

// WebhookEvent represents a parsed webhook event from the payment provider.
type WebhookEvent struct {
	Type string
	Data WebhookEventData
}

// WebhookEventData contains parsed data from webhook events.
type WebhookEventData struct {
	// Subscription events
	SubscriptionID     string
	SubscriptionStatus string
	WorkspaceID        uuid.UUID
	PeriodStart        time.Time
	PeriodEnd          time.Time

	// Payment events
	PaymentIntentID string
	PaymentStatus   string
	AmountCents     int64
	Currency        string
	Description     string

	// Checkout events
	CheckoutMode string // "subscription" or "payment"
}

// Provider defines the interface for payment providers.
type Provider interface {
	// CreateCustomer creates a customer in the payment provider.
	CreateCustomer(ctx context.Context, req CustomerRequest) (customerID string, err error)

	// CreateSubscriptionCheckout creates a checkout session for a subscription.
	CreateSubscriptionCheckout(ctx context.Context, req CheckoutRequest) (CheckoutResponse, error)

	// CreatePaymentCheckout creates a checkout session for a one-time payment.
	CreatePaymentCheckout(ctx context.Context, req CheckoutRequest) (CheckoutResponse, error)

	// CreatePaymentIntent creates a payment intent for client-side confirmation.
	CreatePaymentIntent(ctx context.Context, req PaymentIntentRequest) (PaymentIntentResponse, error)

	// CancelSubscription cancels a subscription at period end.
	CancelSubscription(ctx context.Context, subscriptionID string) error

	// ParseWebhook parses and verifies an incoming webhook request.
	ParseWebhook(r *http.Request) (*WebhookEvent, error)
}

// Service orchestrates billing operations using a payment provider.
type Service struct {
	q        *db.Queries
	provider Provider
}

// NewService creates a billing service with the given provider.
func NewService(q *db.Queries, provider Provider) *Service {
	return &Service{q: q, provider: provider}
}

// EnsureCustomer creates or retrieves a customer for a workspace.
func (s *Service) EnsureCustomer(ctx context.Context, workspaceID uuid.UUID, email string) (string, error) {
	bd, err := s.q.GetBillingDetailsByWorkspace(ctx, workspaceID)
	if err == nil && bd.StripeCustomerID.Valid {
		return bd.StripeCustomerID.String, nil
	}

	customerID, err := s.provider.CreateCustomer(ctx, CustomerRequest{
		WorkspaceID: workspaceID,
		Email:       email,
	})
	if err != nil {
		return "", err
	}

	_, err = s.q.UpsertBillingDetails(ctx, db.UpsertBillingDetailsParams{
		WorkspaceID:      workspaceID,
		StripeCustomerID: pgtext(customerID),
		BillingEmail:     pgtext(email),
	})
	if err != nil {
		return "", err
	}

	return customerID, nil
}

// GetCustomerID retrieves the customer ID for a workspace.
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

// CreateCheckoutSession creates a checkout session for subscription.
func (s *Service) CreateCheckoutSession(ctx context.Context, workspaceID uuid.UUID, priceID, successURL, cancelURL string) (string, error) {
	_, err := s.q.GetSubscriptionByWorkspace(ctx, workspaceID)
	if err == nil {
		return "", ErrSubscriptionExists
	}

	customerID, err := s.GetCustomerID(ctx, workspaceID)
	if err != nil {
		return "", err
	}

	resp, err := s.provider.CreateSubscriptionCheckout(ctx, CheckoutRequest{
		WorkspaceID: workspaceID,
		CustomerID:  customerID,
		PriceID:     priceID,
		SuccessURL:  successURL,
		CancelURL:   cancelURL,
	})
	if err != nil {
		return "", err
	}

	return resp.URL, nil
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
		return errors.New("no subscription to cancel")
	}

	return s.provider.CancelSubscription(ctx, sub.StripeSubscriptionID.String)
}

// CreatePaymentIntent creates a payment intent for a one-time charge.
func (s *Service) CreatePaymentIntent(ctx context.Context, workspaceID uuid.UUID, amountCents int64, currency, description string) (db.Payment, string, error) {
	customerID, err := s.GetCustomerID(ctx, workspaceID)
	if err != nil {
		return db.Payment{}, "", err
	}

	resp, err := s.provider.CreatePaymentIntent(ctx, PaymentIntentRequest{
		WorkspaceID: workspaceID,
		CustomerID:  customerID,
		AmountCents: amountCents,
		Currency:    currency,
		Description: description,
	})
	if err != nil {
		return db.Payment{}, "", err
	}

	payment, err := s.q.CreatePayment(ctx, db.CreatePaymentParams{
		WorkspaceID:         workspaceID,
		StripePaymentIntent: pgtext(resp.ID),
		AmountCents:         int32(amountCents),
		Currency:            currency,
		Status:              resp.Status,
		Description:         pgtext(description),
		Metadata:            []byte("{}"),
	})
	if err != nil {
		return db.Payment{}, "", err
	}

	return payment, resp.ClientSecret, nil
}

// CreatePaymentCheckout creates a checkout session for one-time payment.
func (s *Service) CreatePaymentCheckout(ctx context.Context, workspaceID uuid.UUID, amountCents int64, currency, description, successURL, cancelURL string) (string, error) {
	customerID, err := s.GetCustomerID(ctx, workspaceID)
	if err != nil {
		return "", err
	}

	resp, err := s.provider.CreatePaymentCheckout(ctx, CheckoutRequest{
		WorkspaceID: workspaceID,
		CustomerID:  customerID,
		AmountCents: amountCents,
		Currency:    currency,
		Description: description,
		SuccessURL:  successURL,
		CancelURL:   cancelURL,
	})
	if err != nil {
		return "", err
	}

	return resp.URL, nil
}

// ParseWebhook verifies and parses an incoming webhook request.
func (s *Service) ParseWebhook(r *http.Request) (*WebhookEvent, error) {
	return s.provider.ParseWebhook(r)
}

// HandleWebhook processes a webhook event and updates the database.
func (s *Service) HandleWebhook(ctx context.Context, event *WebhookEvent) error {
	switch event.Type {
	case "checkout.session.completed":
		return s.handleCheckoutCompleted(ctx, event)
	case "customer.subscription.created", "customer.subscription.updated":
		return s.handleSubscriptionUpdated(ctx, event)
	case "customer.subscription.deleted":
		return s.handleSubscriptionDeleted(ctx, event)
	case "invoice.paid":
		return s.handleInvoicePaid(ctx, event)
	case "invoice.payment_failed":
		return s.handleInvoicePaymentFailed(ctx, event)
	case "payment_intent.succeeded":
		return s.handlePaymentIntentSucceeded(ctx, event)
	case "payment_intent.payment_failed":
		return s.handlePaymentIntentFailed(ctx, event)
	}
	return nil
}

func (s *Service) handleCheckoutCompleted(ctx context.Context, event *WebhookEvent) error {
	if event.Data.CheckoutMode == "subscription" {
		return nil // Subscription webhook handles details
	}

	if event.Data.PaymentIntentID == "" {
		return nil
	}

	_, err := s.q.CreatePayment(ctx, db.CreatePaymentParams{
		WorkspaceID:         event.Data.WorkspaceID,
		StripePaymentIntent: pgtext(event.Data.PaymentIntentID),
		AmountCents:         int32(event.Data.AmountCents),
		Currency:            event.Data.Currency,
		Status:              event.Data.PaymentStatus,
		Description:         pgtext(event.Data.Description),
		Metadata:            []byte("{}"),
	})
	if err != nil {
		return s.q.UpdatePaymentStatus(ctx, db.UpdatePaymentStatusParams{
			StripePaymentIntent: pgtext(event.Data.PaymentIntentID),
			Status:              event.Data.PaymentStatus,
		})
	}
	return nil
}

func (s *Service) handleSubscriptionUpdated(ctx context.Context, event *WebhookEvent) error {
	subID := pgtext(event.Data.SubscriptionID)

	_, err := s.q.GetSubscriptionByStripeID(ctx, subID)
	if err != nil {
		_, err = s.q.CreateSubscription(ctx, db.CreateSubscriptionParams{
			WorkspaceID:          event.Data.WorkspaceID,
			PlanID:               "pro", // TODO: Look up from price ID
			StripeSubscriptionID: subID,
			Status:               event.Data.SubscriptionStatus,
			CurrentPeriodStart:   pgtimestamp(event.Data.PeriodStart),
			CurrentPeriodEnd:     pgtimestamp(event.Data.PeriodEnd),
		})
		return err
	}

	return s.q.UpdateSubscriptionStatus(ctx, db.UpdateSubscriptionStatusParams{
		StripeSubscriptionID: subID,
		Status:               event.Data.SubscriptionStatus,
	})
}

func (s *Service) handleSubscriptionDeleted(ctx context.Context, event *WebhookEvent) error {
	return s.q.CancelSubscription(ctx, pgtext(event.Data.SubscriptionID))
}

func (s *Service) handleInvoicePaid(ctx context.Context, event *WebhookEvent) error {
	if event.Data.SubscriptionID == "" {
		return nil
	}
	subID := pgtext(event.Data.SubscriptionID)
	_ = s.q.UpdateSubscriptionPeriod(ctx, db.UpdateSubscriptionPeriodParams{
		StripeSubscriptionID: subID,
		CurrentPeriodStart:   pgtimestamp(event.Data.PeriodStart),
		CurrentPeriodEnd:     pgtimestamp(event.Data.PeriodEnd),
	})
	return s.q.UpdateSubscriptionStatus(ctx, db.UpdateSubscriptionStatusParams{
		StripeSubscriptionID: subID,
		Status:               "active",
	})
}

func (s *Service) handleInvoicePaymentFailed(ctx context.Context, event *WebhookEvent) error {
	if event.Data.SubscriptionID == "" {
		return nil
	}
	return s.q.UpdateSubscriptionStatus(ctx, db.UpdateSubscriptionStatusParams{
		StripeSubscriptionID: pgtext(event.Data.SubscriptionID),
		Status:               "past_due",
	})
}

func (s *Service) handlePaymentIntentSucceeded(ctx context.Context, event *WebhookEvent) error {
	return s.q.UpdatePaymentStatus(ctx, db.UpdatePaymentStatusParams{
		StripePaymentIntent: pgtext(event.Data.PaymentIntentID),
		Status:              "succeeded",
	})
}

func (s *Service) handlePaymentIntentFailed(ctx context.Context, event *WebhookEvent) error {
	return s.q.UpdatePaymentStatus(ctx, db.UpdatePaymentStatusParams{
		StripePaymentIntent: pgtext(event.Data.PaymentIntentID),
		Status:              "failed",
	})
}

func pgtext(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}

func pgtimestamp(t time.Time) pgtype.Timestamptz {
	if t.IsZero() {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: t, Valid: true}
}
