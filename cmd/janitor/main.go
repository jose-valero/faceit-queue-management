package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/jackc/pgx/v5/pgxpool"
)

func handler(ctx context.Context) (string, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return "no DATABASE_URL", nil
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return fmt.Sprintf("parse: %v", err), nil
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return fmt.Sprintf("pool: %v", err), nil
	}
	defer pool.Close()

	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, _ = pool.Exec(cctx, `DELETE FROM webhook_dedup WHERE received_at < now() - INTERVAL '7 days';`)
	_, _ = pool.Exec(cctx, `
DELETE FROM faceit_match_status
WHERE updated_at < now() - INTERVAL '30 days'
  AND status IN ('finished','cancelled','aborted');`)

	return "ok", nil
}

func main() { lambda.Start(handler) }
