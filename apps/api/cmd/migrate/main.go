package main

import (
	"context"
	"flag"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	migrate "baseapp/sql"
)

func main() {
	var direction string
	flag.StringVar(&direction, "direction", "up", "Migration direction: up or down")
	flag.Parse()

	dbURL := requireEnv("DATABASE_URL")

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		slog.Error("db connection failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		slog.Error("db ping failed", "err", err)
		os.Exit(1)
	}

	switch direction {
	case "up":
		if err := migrate.Up(dbURL); err != nil {
			slog.Error("migration up failed", "err", err)
			os.Exit(1)
		}
		slog.Info("migrations applied successfully")
	case "down":
		if err := migrate.Down(dbURL); err != nil {
			slog.Error("migration down failed", "err", err)
			os.Exit(1)
		}
		slog.Info("migrations rolled back successfully")
	default:
		slog.Error("invalid direction", "direction", direction)
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
