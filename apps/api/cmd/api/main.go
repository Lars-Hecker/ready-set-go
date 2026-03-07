package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	domainfile "baseapp/domain/file"
	domainuser "baseapp/domain/user"
	"baseapp/gen/api/file/filepbconnect"
	"baseapp/gen/api/user/userpbconnect"
	"baseapp/gen/db"
	"baseapp/infra/admin"
	"baseapp/infra/auth"
	"baseapp/infra/auth/session"
	"baseapp/infra/file"
)

func main() {
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, requireEnv("DATABASE_URL"))
	if err != nil {
		slog.Error("db", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	queries := db.New(pool)
	sessionMgr := session.NewSessionManager(queries, session.NewNoOpGeoLookup())

	// Initialize S3 file service (optional - only if configured)
	var fileSvc *file.S3Service
	if s3Cfg := file.S3ConfigFromEnv(); s3Cfg.Bucket != "" {
		var err error
		fileSvc, err = file.NewS3Service(ctx, s3Cfg)
		if err != nil {
			slog.Error("s3", "err", err)
			os.Exit(1)
		}
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
	if fileSvc != nil {
		mux.Handle(filepbconnect.NewFileServiceHandler(
			domainfile.NewHandler(queries, fileSvc), interceptors,
		))
	}
	mux.Handle("/", admin.Handler())

	addr := ":" + envOr("PORT", "8080")
	slog.Info("listening", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		slog.Error("server", "err", err)
		os.Exit(1)
	}
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("env required", "key", key)
		os.Exit(1)
	}
	return v
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
