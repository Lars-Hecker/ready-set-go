package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"

	"baseapp/gen/db"
)

// Handlers processes notification tasks from the queue.
type Handlers struct {
	queries     *db.Queries
	email       EmailSender
	push        PushSender
	webPush     WebPushSender
	asynqClient *asynq.Client
}

// HandlersConfig contains dependencies for notification handlers.
type HandlersConfig struct {
	Queries     *db.Queries
	Email       EmailSender
	Push        PushSender
	WebPush     WebPushSender
	AsynqClient *asynq.Client
}

// NewHandlers creates a new notification task handlers instance.
func NewHandlers(cfg HandlersConfig) *Handlers {
	return &Handlers{
		queries:     cfg.Queries,
		email:       cfg.Email,
		push:        cfg.Push,
		webPush:     cfg.WebPush,
		asynqClient: cfg.AsynqClient,
	}
}

// Register registers all notification handlers with the asynq server mux.
func (h *Handlers) Register(mux *asynq.ServeMux) {
	mux.HandleFunc(TypeSendEmail, h.handleSendEmail)
	mux.HandleFunc(TypeSendPush, h.handleSendPush)
	mux.HandleFunc(TypeSendWebPush, h.handleSendWebPush)
	mux.HandleFunc(TypeSendToUser, h.handleSendToUser)
}

func (h *Handlers) handleSendEmail(ctx context.Context, t *asynq.Task) error {
	var p EmailTaskPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	if h.email == nil {
		return h.updateLogStatus(ctx, p.LogID, "failed", "email sender not configured")
	}

	err := h.email.SendEmail(ctx, p.Message)
	if err != nil {
		slog.Error("send email failed", "log_id", p.LogID, "to", p.Message.To, "err", err)
		_ = h.updateLogStatus(ctx, p.LogID, "failed", err.Error())
		return err
	}

	slog.Info("email sent", "log_id", p.LogID, "to", p.Message.To)
	return h.updateLogStatus(ctx, p.LogID, "sent", "")
}

func (h *Handlers) handleSendPush(ctx context.Context, t *asynq.Task) error {
	var p PushTaskPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	if h.push == nil {
		return h.updateLogStatus(ctx, p.LogID, "failed", "push sender not configured")
	}

	err := h.push.SendPush(ctx, p.EndpointARN, p.Message)
	if err != nil {
		slog.Error("send push failed", "log_id", p.LogID, "endpoint", p.EndpointARN, "err", err)
		_ = h.updateLogStatus(ctx, p.LogID, "failed", err.Error())
		return err
	}

	slog.Info("push sent", "log_id", p.LogID, "endpoint", p.EndpointARN)
	return h.updateLogStatus(ctx, p.LogID, "sent", "")
}

func (h *Handlers) handleSendWebPush(ctx context.Context, t *asynq.Task) error {
	var p WebPushTaskPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	if h.webPush == nil {
		return h.updateLogStatus(ctx, p.LogID, "failed", "webpush sender not configured")
	}

	err := h.webPush.SendWebPush(ctx, p.Subscription, p.Message)
	if err != nil {
		slog.Error("send webpush failed", "log_id", p.LogID, "endpoint", p.Subscription.Endpoint, "err", err)

		// Deactivate expired subscriptions
		if errors.Is(err, ErrSubscriptionExpired) {
			_ = h.queries.DeactivateDeviceTokenByToken(ctx, p.Subscription.Endpoint)
		}

		_ = h.updateLogStatus(ctx, p.LogID, "failed", err.Error())
		return err
	}

	slog.Info("webpush sent", "log_id", p.LogID, "endpoint", p.Subscription.Endpoint)
	return h.updateLogStatus(ctx, p.LogID, "sent", "")
}

func (h *Handlers) handleSendToUser(ctx context.Context, t *asynq.Task) error {
	var p UserNotifyPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	// Get user preferences (default to enabled if not set or on error)
	prefs, err := h.queries.GetNotificationPreferences(ctx, p.UserID)
	emailEnabled := true
	pushEnabled := true
	if err != nil {
		slog.Debug("notification preferences not found, using defaults", "user_id", p.UserID)
	} else {
		emailEnabled = prefs.EmailEnabled
		pushEnabled = prefs.PushEnabled
	}

	// Send email if enabled
	if emailEnabled && p.Email != "" && h.email != nil {
		h.enqueueEmail(ctx, p)
	}

	// Send push to all active devices if enabled
	if pushEnabled {
		h.enqueuePushNotifications(ctx, p)
	}

	return nil
}

func (h *Handlers) enqueueEmail(ctx context.Context, p UserNotifyPayload) {
	emailMsg := EmailMessage{
		To:       p.Email,
		Subject:  p.Subject,
		TextBody: p.Message.Body,
	}
	logEntry, err := h.queries.CreateNotificationLog(ctx, db.CreateNotificationLogParams{
		UserID:    pgtype.UUID{Bytes: p.UserID, Valid: true},
		Channel:   string(ChannelEmail),
		Recipient: p.Email,
		Subject:   pgtype.Text{String: p.Subject, Valid: true},
		Body:      pgtype.Text{String: p.Message.Body, Valid: true},
		Status:    "pending",
	})
	if err != nil {
		slog.Error("create email log failed", "user_id", p.UserID, "err", err)
		return
	}
	task, err := NewSendEmailTask(logEntry.ID, emailMsg)
	if err != nil {
		slog.Error("create email task failed", "user_id", p.UserID, "err", err)
		return
	}
	if _, err := h.asynqClient.EnqueueContext(ctx, task); err != nil {
		slog.Error("enqueue email task failed", "user_id", p.UserID, "err", err)
	}
}

func (h *Handlers) enqueuePushNotifications(ctx context.Context, p UserNotifyPayload) {
	devices, err := h.queries.ListActiveDeviceTokensByUser(ctx, p.UserID)
	if err != nil {
		slog.Error("list devices failed", "user_id", p.UserID, "err", err)
		return
	}

	for _, device := range devices {
		if device.Platform == "web" && h.webPush != nil {
			h.enqueueWebPush(ctx, p, device)
		} else if device.EndpointArn.Valid && h.push != nil {
			h.enqueueMobilePush(ctx, p, device)
		}
	}
}

func (h *Handlers) enqueueWebPush(ctx context.Context, p UserNotifyPayload, device db.DeviceToken) {
	// Web push requires p256dh and auth keys
	if !device.P256dh.Valid || !device.Auth.Valid {
		slog.Warn("web push device missing encryption keys", "user_id", p.UserID, "device_id", device.ID)
		return
	}
	sub := WebPushSubscription{
		Endpoint: device.Token,
		P256dh:   device.P256dh.String,
		Auth:     device.Auth.String,
	}
	logEntry, err := h.queries.CreateNotificationLog(ctx, db.CreateNotificationLogParams{
		UserID:    pgtype.UUID{Bytes: p.UserID, Valid: true},
		Channel:   string(ChannelWebPush),
		Recipient: device.Token,
		Subject:   pgtype.Text{String: p.Message.Title, Valid: true},
		Body:      pgtype.Text{String: p.Message.Body, Valid: true},
		Status:    "pending",
	})
	if err != nil {
		slog.Error("create webpush log failed", "user_id", p.UserID, "err", err)
		return
	}
	task, err := NewSendWebPushTask(logEntry.ID, sub, p.Message)
	if err != nil {
		slog.Error("create webpush task failed", "user_id", p.UserID, "err", err)
		return
	}
	if _, err := h.asynqClient.EnqueueContext(ctx, task); err != nil {
		slog.Error("enqueue webpush task failed", "user_id", p.UserID, "err", err)
	}
}

func (h *Handlers) enqueueMobilePush(ctx context.Context, p UserNotifyPayload, device db.DeviceToken) {
	logEntry, err := h.queries.CreateNotificationLog(ctx, db.CreateNotificationLogParams{
		UserID:    pgtype.UUID{Bytes: p.UserID, Valid: true},
		Channel:   string(ChannelMobilePush),
		Recipient: device.EndpointArn.String,
		Subject:   pgtype.Text{String: p.Message.Title, Valid: true},
		Body:      pgtype.Text{String: p.Message.Body, Valid: true},
		Status:    "pending",
	})
	if err != nil {
		slog.Error("create push log failed", "user_id", p.UserID, "err", err)
		return
	}
	task, err := NewSendPushTask(logEntry.ID, device.EndpointArn.String, p.Message)
	if err != nil {
		slog.Error("create push task failed", "user_id", p.UserID, "err", err)
		return
	}
	if _, err := h.asynqClient.EnqueueContext(ctx, task); err != nil {
		slog.Error("enqueue push task failed", "user_id", p.UserID, "err", err)
	}
}

func (h *Handlers) updateLogStatus(ctx context.Context, logID uuid.UUID, status, errMsg string) error {
	errText := pgtype.Text{}
	if errMsg != "" {
		errText = pgtype.Text{String: errMsg, Valid: true}
	}
	return h.queries.UpdateNotificationLogStatus(ctx, db.UpdateNotificationLogStatusParams{
		ID:     logID,
		Status: status,
		Error:  errText,
	})
}
