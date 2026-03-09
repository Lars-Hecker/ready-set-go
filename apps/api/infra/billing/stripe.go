package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/customer"
	"github.com/stripe/stripe-go/v82/paymentintent"
	"github.com/stripe/stripe-go/v82/subscription"
	"github.com/stripe/stripe-go/v82/webhook"
)

// StripeConfig holds Stripe-specific configuration.
type StripeConfig struct {
	SecretKey     string
	WebhookSecret string
}

// StripeProvider implements Provider using Stripe.
type StripeProvider struct {
	config StripeConfig
}

// NewStripeProvider creates a Stripe payment provider.
func NewStripeProvider(config StripeConfig) (*StripeProvider, error) {
	if config.SecretKey == "" {
		return nil, fmt.Errorf("%w: SecretKey is required", ErrInvalidConfig)
	}
	if config.WebhookSecret == "" {
		return nil, fmt.Errorf("%w: WebhookSecret is required", ErrInvalidConfig)
	}
	stripe.Key = config.SecretKey
	return &StripeProvider{config: config}, nil
}

func (p *StripeProvider) CreateCustomer(ctx context.Context, req CustomerRequest) (string, error) {
	params := &stripe.CustomerParams{
		Email: stripe.String(req.Email),
		Metadata: map[string]string{
			"workspace_id": req.WorkspaceID.String(),
		},
	}
	cust, err := customer.New(params)
	if err != nil {
		return "", fmt.Errorf("create stripe customer: %w", err)
	}
	return cust.ID, nil
}

func (p *StripeProvider) CreateSubscriptionCheckout(ctx context.Context, req CheckoutRequest) (CheckoutResponse, error) {
	params := &stripe.CheckoutSessionParams{
		Customer: stripe.String(req.CustomerID),
		Mode:     stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(req.PriceID),
				Quantity: stripe.Int64(1),
			},
		},
		SuccessURL: stripe.String(req.SuccessURL),
		CancelURL:  stripe.String(req.CancelURL),
		Metadata: map[string]string{
			"workspace_id": req.WorkspaceID.String(),
		},
		SubscriptionData: &stripe.CheckoutSessionSubscriptionDataParams{
			Metadata: map[string]string{
				"workspace_id": req.WorkspaceID.String(),
			},
		},
	}

	sess, err := session.New(params)
	if err != nil {
		return CheckoutResponse{}, fmt.Errorf("create checkout session: %w", err)
	}

	return CheckoutResponse{URL: sess.URL}, nil
}

func (p *StripeProvider) CreatePaymentCheckout(ctx context.Context, req CheckoutRequest) (CheckoutResponse, error) {
	params := &stripe.CheckoutSessionParams{
		Customer: stripe.String(req.CustomerID),
		Mode:     stripe.String(string(stripe.CheckoutSessionModePayment)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency:   stripe.String(req.Currency),
					UnitAmount: stripe.Int64(req.AmountCents),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name:        stripe.String(req.Description),
						Description: stripe.String("One-time payment"),
					},
				},
				Quantity: stripe.Int64(1),
			},
		},
		SuccessURL: stripe.String(req.SuccessURL),
		CancelURL:  stripe.String(req.CancelURL),
		Metadata: map[string]string{
			"workspace_id": req.WorkspaceID.String(),
			"description":  req.Description,
		},
		PaymentIntentData: &stripe.CheckoutSessionPaymentIntentDataParams{
			Metadata: map[string]string{
				"workspace_id": req.WorkspaceID.String(),
			},
		},
	}

	sess, err := session.New(params)
	if err != nil {
		return CheckoutResponse{}, fmt.Errorf("create payment checkout: %w", err)
	}

	return CheckoutResponse{URL: sess.URL}, nil
}

func (p *StripeProvider) CreatePaymentIntent(ctx context.Context, req PaymentIntentRequest) (PaymentIntentResponse, error) {
	params := &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(req.AmountCents),
		Currency: stripe.String(req.Currency),
		Customer: stripe.String(req.CustomerID),
		Metadata: map[string]string{
			"workspace_id": req.WorkspaceID.String(),
			"description":  req.Description,
		},
		AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
			Enabled: stripe.Bool(true),
		},
	}

	pi, err := paymentintent.New(params)
	if err != nil {
		return PaymentIntentResponse{}, fmt.Errorf("create payment intent: %w", err)
	}

	return PaymentIntentResponse{
		ID:           pi.ID,
		ClientSecret: pi.ClientSecret,
		Status:       string(pi.Status),
	}, nil
}

func (p *StripeProvider) CancelSubscription(ctx context.Context, subscriptionID string) error {
	_, err := subscription.Update(subscriptionID, &stripe.SubscriptionParams{
		CancelAtPeriodEnd: stripe.Bool(true),
	})
	return err
}

func (p *StripeProvider) ParseWebhook(r *http.Request) (*WebhookEvent, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 65536))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	sig := r.Header.Get("Stripe-Signature")
	event, err := webhook.ConstructEvent(body, sig, p.config.WebhookSecret)
	if err != nil {
		return nil, ErrInvalidWebhook
	}

	return p.convertEvent(&event)
}

func (p *StripeProvider) convertEvent(event *stripe.Event) (*WebhookEvent, error) {
	result := &WebhookEvent{Type: string(event.Type)}

	switch event.Type {
	case stripe.EventTypeCheckoutSessionCompleted:
		var sess stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
			return nil, err
		}
		result.Data = p.convertCheckoutSession(&sess)

	case stripe.EventTypeCustomerSubscriptionCreated,
		stripe.EventTypeCustomerSubscriptionUpdated,
		stripe.EventTypeCustomerSubscriptionDeleted:
		var sub stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			return nil, err
		}
		result.Data = p.convertSubscription(&sub)

	case stripe.EventTypeInvoicePaid, stripe.EventTypeInvoicePaymentFailed:
		var inv stripe.Invoice
		if err := json.Unmarshal(event.Data.Raw, &inv); err != nil {
			return nil, err
		}
		result.Data = p.convertInvoice(&inv)

	case stripe.EventTypePaymentIntentSucceeded, stripe.EventTypePaymentIntentPaymentFailed:
		var pi stripe.PaymentIntent
		if err := json.Unmarshal(event.Data.Raw, &pi); err != nil {
			return nil, err
		}
		result.Data = p.convertPaymentIntent(&pi)
	}

	return result, nil
}

func (p *StripeProvider) convertCheckoutSession(sess *stripe.CheckoutSession) WebhookEventData {
	data := WebhookEventData{
		CheckoutMode: string(sess.Mode),
	}

	if wsID, err := uuid.Parse(sess.Metadata["workspace_id"]); err == nil {
		data.WorkspaceID = wsID
	}
	data.Description = sess.Metadata["description"]

	if sess.PaymentIntent != nil {
		pi, err := paymentintent.Get(sess.PaymentIntent.ID, nil)
		if err == nil {
			data.PaymentIntentID = pi.ID
			data.PaymentStatus = string(pi.Status)
			data.AmountCents = pi.Amount
			data.Currency = string(pi.Currency)
		}
	}

	return data
}

func (p *StripeProvider) convertSubscription(sub *stripe.Subscription) WebhookEventData {
	data := WebhookEventData{
		SubscriptionID:     sub.ID,
		SubscriptionStatus: string(sub.Status),
		PeriodStart:        time.Unix(sub.StartDate, 0),
	}

	if sub.CancelAt > 0 {
		data.PeriodEnd = time.Unix(sub.CancelAt, 0)
	}

	if wsID, err := uuid.Parse(sub.Metadata["workspace_id"]); err == nil {
		data.WorkspaceID = wsID
	}

	return data
}

func (p *StripeProvider) convertInvoice(inv *stripe.Invoice) WebhookEventData {
	data := WebhookEventData{
		PeriodStart: time.Unix(inv.PeriodStart, 0),
		PeriodEnd:   time.Unix(inv.PeriodEnd, 0),
	}

	if inv.Parent != nil && inv.Parent.SubscriptionDetails != nil && inv.Parent.SubscriptionDetails.Subscription != nil {
		data.SubscriptionID = inv.Parent.SubscriptionDetails.Subscription.ID
	}

	return data
}

func (p *StripeProvider) convertPaymentIntent(pi *stripe.PaymentIntent) WebhookEventData {
	return WebhookEventData{
		PaymentIntentID: pi.ID,
		PaymentStatus:   string(pi.Status),
	}
}
