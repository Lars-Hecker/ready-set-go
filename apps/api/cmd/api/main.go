package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"connectrpc.com/connect"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	domainfile "baseapp/domain/file"
	domainuser "baseapp/domain/user"
	domainworkspace "baseapp/domain/workspace"
	"baseapp/gen/api/file/filepbconnect"
	"baseapp/gen/api/user/userpbconnect"
	"baseapp/gen/api/workspace/workspacepbconnect"
	"baseapp/gen/db"
	"baseapp/infra/admin"
	"baseapp/infra/ai"
	"baseapp/infra/auth"
	"baseapp/infra/auth/session"
	"baseapp/infra/config"
	"baseapp/infra/file"
)

func main() {
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, config.Require("DATABASE_URL"))
	if err != nil {
		slog.Error("db", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	queries := db.New(pool)
	sessionMgr := session.NewSessionManager(queries, session.NewNoOpGeoLookup())

	// Initialize Redis (optional - only if configured)
	var rdb *redis.Client
	var asynqClient *asynq.Client
	if redisURL := os.Getenv("REDIS_URL"); redisURL != "" {
		rdb = redis.NewClient(&redis.Options{Addr: redisURL})
		if err := rdb.Ping(ctx).Err(); err != nil {
			slog.Error("redis", "err", err)
			os.Exit(1)
		}
		// Initialize asynq client for enqueueing background tasks
		asynqClient = asynq.NewClient(asynq.RedisClientOpt{Addr: redisURL})
	}
	_ = asynqClient // TODO: Pass to domain handlers that need to send notifications

	// Initialize S3 file service (optional - only if configured)
	var fileSvc *file.S3Service
	if s3Cfg := config.S3ConfigFromEnv(); s3Cfg.Bucket != "" {
		var err error
		fileSvc, err = file.NewS3Service(ctx, s3Cfg)
		if err != nil {
			slog.Error("s3", "err", err)
			os.Exit(1)
		}
	}

	// Initialize AI service (optional - only if configured)
	var aiSvc *ai.Service
	if aiCfg := ai.ConfigFromEnv(); aiCfg.GoogleAPIKey != "" {
		var err error
		aiSvc, err = ai.NewService(ctx, aiCfg, rdb)
		if err != nil {
			slog.Error("ai", "err", err)
			os.Exit(1)
		}
		_ = aiSvc // TODO: Wire up to handlers that need AI
	}

	publicProcs := map[string]bool{
		userpbconnect.AuthServiceSignupProcedure: true,
		userpbconnect.AuthServiceLoginProcedure:  true,
	}
	interceptors := connect.WithInterceptors(auth.NewAuthInterceptor(sessionMgr, publicProcs))

	mux := http.NewServeMux()
	mux.Handle(userpbconnect.NewAuthServiceHandler(
		domainuser.NewAuthHandler(queries, sessionMgr), interceptors,
	))
	mux.Handle(userpbconnect.NewUserServiceHandler(
		domainuser.NewUserHandler(queries), interceptors,
	))
	mux.Handle(workspacepbconnect.NewWorkspaceServiceHandler(
		domainworkspace.NewHandler(pool, queries), interceptors,
	))
	if fileSvc != nil {
		mux.Handle(filepbconnect.NewFileServiceHandler(
			domainfile.NewHandler(queries, fileSvc), interceptors,
		))
	}
	mux.Handle("/", admin.Handler())

	addr := ":" + config.Get("PORT", "8080")
	slog.Info("listening", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		slog.Error("server", "err", err)
		os.Exit(1)
	}
}
