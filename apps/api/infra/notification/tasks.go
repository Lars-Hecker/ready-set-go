package notification

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

// Task type constants for the notification queue.
const (
	TypeSendEmail   = "notification:send_email"
	TypeSendPush    = "notification:send_push"
	TypeSendWebPush = "notification:send_webpush"
	TypeSendToUser  = "notification:send_to_user"
)

// Default task options: 3 retries, 30s timeout, exponential backoff.
var defaultTaskOpts = []asynq.Option{
	asynq.MaxRetry(3),
	asynq.Timeout(30 * time.Second),
	asynq.Queue("notifications"),
}

// EmailTaskPayload is the payload for email notification tasks.
type EmailTaskPayload struct {
	LogID   uuid.UUID    `json:"log_id"`
	Message EmailMessage `json:"message"`
}

// PushTaskPayload is the payload for mobile push notification tasks.
type PushTaskPayload struct {
	LogID       uuid.UUID   `json:"log_id"`
	EndpointARN string      `json:"endpoint_arn"`
	Message     PushMessage `json:"message"`
}

// WebPushTaskPayload is the payload for web push notification tasks.
type WebPushTaskPayload struct {
	LogID        uuid.UUID           `json:"log_id"`
	Subscription WebPushSubscription `json:"subscription"`
	Message      PushMessage         `json:"message"`
}

// UserNotifyPayload is the payload for sending notifications to all user channels.
type UserNotifyPayload struct {
	UserID  uuid.UUID   `json:"user_id"`
	Email   string      `json:"email"`   // User's email address
	Subject string      `json:"subject"` // Email subject
	Message PushMessage `json:"message"` // Used for push notifications
}

// NewSendEmailTask creates a task to send an email.
func NewSendEmailTask(logID uuid.UUID, msg EmailMessage) (*asynq.Task, error) {
	payload, err := json.Marshal(EmailTaskPayload{LogID: logID, Message: msg})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeSendEmail, payload, defaultTaskOpts...), nil
}

// NewSendPushTask creates a task to send a mobile push notification.
func NewSendPushTask(logID uuid.UUID, endpointARN string, msg PushMessage) (*asynq.Task, error) {
	payload, err := json.Marshal(PushTaskPayload{LogID: logID, EndpointARN: endpointARN, Message: msg})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeSendPush, payload, defaultTaskOpts...), nil
}

// NewSendWebPushTask creates a task to send a web push notification.
func NewSendWebPushTask(logID uuid.UUID, sub WebPushSubscription, msg PushMessage) (*asynq.Task, error) {
	payload, err := json.Marshal(WebPushTaskPayload{LogID: logID, Subscription: sub, Message: msg})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeSendWebPush, payload, defaultTaskOpts...), nil
}

// NewSendToUserTask creates a task to send notifications to all user's enabled channels.
func NewSendToUserTask(userID uuid.UUID, email, subject string, msg PushMessage) (*asynq.Task, error) {
	payload, err := json.Marshal(UserNotifyPayload{
		UserID:  userID,
		Email:   email,
		Subject: subject,
		Message: msg,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeSendToUser, payload, defaultTaskOpts...), nil
}
