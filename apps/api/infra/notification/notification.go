package notification

import (
	"context"
	"errors"
)

// Channel represents a notification delivery channel.
type Channel string

const (
	ChannelEmail      Channel = "email"
	ChannelWebPush    Channel = "web_push"
	ChannelMobilePush Channel = "mobile_push"
)

// Common errors for notification handling.
var (
	ErrSubscriptionExpired = errors.New("subscription expired")
	ErrRateLimited         = errors.New("rate limited")
)

// EmailMessage represents an email to be sent.
type EmailMessage struct {
	To       string
	Subject  string
	HTMLBody string
	TextBody string
	ReplyTo  string
}

// PushMessage represents a push notification payload.
type PushMessage struct {
	Title    string
	Body     string
	Data     map[string]string
	ImageURL string
	Badge    int
}

// WebPushSubscription represents a Web Push API subscription.
type WebPushSubscription struct {
	Endpoint string
	P256dh   string
	Auth     string
}

// EmailSender sends email notifications.
type EmailSender interface {
	SendEmail(ctx context.Context, msg EmailMessage) error
}

// PushSender sends mobile push notifications via SNS.
type PushSender interface {
	SendPush(ctx context.Context, endpointARN string, msg PushMessage) error
	CreatePlatformEndpoint(ctx context.Context, platformAppARN, token string) (string, error)
	DeletePlatformEndpoint(ctx context.Context, endpointARN string) error
}

// WebPushSender sends web push notifications.
type WebPushSender interface {
	SendWebPush(ctx context.Context, sub WebPushSubscription, msg PushMessage) error
}
