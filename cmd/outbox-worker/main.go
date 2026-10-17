package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/anuraghagawane/luma/internal/config"
	"github.com/anuraghagawane/luma/internal/infra/cache"
	"github.com/anuraghagawane/luma/internal/repository/postgres"
	"github.com/anuraghagawane/luma/internal/worker/outbox"
	"github.com/redis/go-redis/v9"
)

func main() {
	if err := run(); err != nil {
		log.Printf("outbox worker stopped: %v", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg, err := config.LoadEnv()
	if err != nil {
		log.Fatalf("Error while parsing env: %v", err)
	}

	dbpool, err := postgres.NewPool(ctx, cfg.PostgresDBUrl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to create connection pool: %v\n", err)
		os.Exit(1)
	}
	defer dbpool.Close()
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisHost,
		Password: cfg.RedisPassword, // no password
		DB:       0,                 // use default DB
		Protocol: 2,
	})

	defer func() { _ = rdb.Close() }()

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err = rdb.Ping(pingCtx).Result()
	if err != nil {
		log.Fatalf("Failed to connect to redis: %v", err)
	}

	outboxRepo := postgres.NewOutboxEventRepo(dbpool)
	cache := cache.NewCacheClient(rdb)

	retryPolicy := outbox.RetryPolicy{MaxAttempts: 5, BaseDelay: 2 * time.Second, MaxDelay: 30 * time.Second}
	worker := outbox.NewWorker(cache, outboxRepo, 5*time.Second, retryPolicy)
	return worker.Run(ctx)
}
