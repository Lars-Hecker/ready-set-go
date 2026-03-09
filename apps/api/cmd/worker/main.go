package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"baseapp/gen/db"
	"baseapp/infra/config"
	"baseapp/infra/notification"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		slog.Info("shutdown signal received")
		cancel()
	}()

	// Initialize database
	pool, err := pgxpool.New(ctx, config.Require("DATABASE_URL"))
	if err != nil {
		slog.Error("db", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	queries := db.New(pool)

	// Initialize Redis connection for asynq
	redisAddr := config.Get("REDIS_URL", "localhost:6379")
	redisOpt := asynq.RedisClientOpt{Addr: redisAddr}

	// Initialize asynq client for enqueueing sub-tasks
	asynqClient := asynq.NewClient(redisOpt)
	defer asynqClient.Close()

	// Initialize notification senders (optional - only if configured)
	var emailSender notification.EmailSender
	if sesCfg := config.SESConfigFromEnv(); sesCfg.Region != "" && sesCfg.FromAddress != "" {
		var err error
		emailSender, err = notification.NewSESService(ctx, sesCfg)
		if err != nil {
			slog.Error("ses", "err", err)
			os.Exit(1)
		}
		slog.Info("SES email sender initialized")
	}

	var pushSender notification.PushSender
	if snsCfg := config.SNSConfigFromEnv(); snsCfg.Region != "" {
		var err error
		pushSender, err = notification.NewSNSService(ctx, snsCfg)
		if err != nil {
			slog.Error("sns", "err", err)
			os.Exit(1)
		}
		slog.Info("SNS push sender initialized")
	}

	var webPushSender notification.WebPushSender
	if wpCfg := config.WebPushConfigFromEnv(); wpCfg.VAPIDPublicKey != "" {
		webPushSender = notification.NewWebPushService(wpCfg)
		slog.Info("Web Push sender initialized")
	}

	// Create notification handlers
	notifyHandlers := notification.NewHandlers(notification.HandlersConfig{
		Queries:     queries,
		Email:       emailSender,
		Push:        pushSender,
		WebPush:     webPushSender,
		AsynqClient: asynqClient,
	})

	// Create asynq server
	srv := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Concurrency: 10,
			Queues: map[string]int{
				"notifications": 6,
				"default":       3,
				"low":           1,
			},
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				slog.Error("task failed", "type", task.Type(), "err", err)
			}),
		},
	)

	// Register handlers
	mux := asynq.NewServeMux()
	notifyHandlers.Register(mux)

	// Start server in goroutine
	go func() {
		slog.Info("worker started", "redis", redisAddr)
		if err := srv.Run(mux); err != nil {
			slog.Error("asynq server", "err", err)
			cancel()
		}
	}()

	<-ctx.Done()
	srv.Shutdown()
	slog.Info("worker stopped")
}
