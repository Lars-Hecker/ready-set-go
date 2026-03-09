package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	webpush "github.com/SherClockHolmes/webpush-go"
)

// WebPushConfig holds VAPID configuration for Web Push.
type WebPushConfig struct {
	VAPIDPublicKey  string
	VAPIDPrivateKey string
	VAPIDSubject    string // mailto: or https:// URL
}

// WebPushService implements WebPushSender using VAPID.
type WebPushService struct {
	vapidPublicKey  string
	vapidPrivateKey string
	vapidSubject    string
}

// NewWebPushService creates a new Web Push sender.
func NewWebPushService(cfg WebPushConfig) *WebPushService {
	return &WebPushService{
		vapidPublicKey:  cfg.VAPIDPublicKey,
		vapidPrivateKey: cfg.VAPIDPrivateKey,
		vapidSubject:    cfg.VAPIDSubject,
	}
}

// webPushPayload is the JSON structure sent to the browser.
type webPushPayload struct {
	Title    string            `json:"title"`
	Body     string            `json:"body"`
	Icon     string            `json:"icon,omitempty"`
	Badge    string            `json:"badge,omitempty"`
	Image    string            `json:"image,omitempty"`
	Data     map[string]string `json:"data,omitempty"`
	Tag      string            `json:"tag,omitempty"`
	Renotify bool              `json:"renotify,omitempty"`
}

// SendWebPush sends a push notification to a web browser.
func (s *WebPushService) SendWebPush(ctx context.Context, sub WebPushSubscription, msg PushMessage) error {
	payload := webPushPayload{
		Title: msg.Title,
		Body:  msg.Body,
		Image: msg.ImageURL,
		Data:  msg.Data,
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	subscription := &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys: webpush.Keys{
			P256dh: sub.P256dh,
			Auth:   sub.Auth,
		},
	}

	resp, err := webpush.SendNotificationWithContext(ctx, payloadJSON, subscription, &webpush.Options{
		VAPIDPublicKey:  s.vapidPublicKey,
		VAPIDPrivateKey: s.vapidPrivateKey,
		Subscriber:      s.vapidSubject,
		TTL:             86400, // 24 hours
	})
	if err != nil {
		return fmt.Errorf("send notification: %w", err)
	}
	defer resp.Body.Close()

	// Handle response status codes
	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated:
		return nil
	case http.StatusGone:
		// Subscription has expired or been unsubscribed
		return ErrSubscriptionExpired
	case http.StatusTooManyRequests:
		return ErrRateLimited
	case http.StatusBadRequest:
		return errors.New("invalid subscription")
	default:
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
}

// VAPIDPublicKey returns the public key for client-side subscription.
func (s *WebPushService) VAPIDPublicKey() string {
	return s.vapidPublicKey
}
