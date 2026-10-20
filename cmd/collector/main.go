package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/anuraghagawane/luma/internal/api/collector"
	"github.com/anuraghagawane/luma/internal/config"
	"github.com/anuraghagawane/luma/internal/infra/cache"
	"github.com/anuraghagawane/luma/internal/infra/kafka"
	"github.com/anuraghagawane/luma/internal/repository/postgres"
	"github.com/redis/go-redis/v9"
)

func main() {
	fmt.Println("Luma started...")

	cfg, err := config.LoadEnv()
	if err != nil {
		log.Fatalf("Error while parsing env: %v", err)
	}

	// kafka
	seeds := []string{cfg.KafkaBroker}
	producer, err := kafka.NewFranzProducer(seeds)
	if err != nil {
		log.Fatalf("Init error: %v", err)
	}
	defer producer.Close()

	// postgres
	dbpool, err := postgres.NewPool(context.Background(), cfg.PostgresDBUrl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to create connection pool: %v\n", err)
		os.Exit(1)
	}
	defer dbpool.Close()

	// redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisHost,
		Password: cfg.RedisPassword, // no password
		DB:       0,                 // use default DB
		Protocol: 2,
	})

	defer func() { _ = rdb.Close() }()

	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = rdb.Ping(pingCtx).Result()
	if err != nil {
		log.Fatalf("Failed to connect to redis: %v", err)
	}

	cache := cache.NewCacheClient(rdb)
	apiKeyRepo := postgres.NewAPIKeyRepo(dbpool)

	logHandler := collector.NewHandler(producer)
	apiKeyAuthenticator := collector.NewAPIKeyAuthenticator(cache, apiKeyRepo)

	http.Handle("/v1/log", collector.APIKeyMiddleware(apiKeyAuthenticator)(http.HandlerFunc(logHandler.HandleLog)))
	http.Handle("/v1/bulklog", collector.APIKeyMiddleware(apiKeyAuthenticator)(http.HandlerFunc(logHandler.HandleBulkLog)))
	log.Fatal(http.ListenAndServe(":"+cfg.CollectorPort, nil))
}
