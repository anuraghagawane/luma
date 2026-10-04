package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/anuraghagawane/luma/internal/api/auth"
	"github.com/anuraghagawane/luma/internal/api/query"
	"github.com/anuraghagawane/luma/internal/config"
	"github.com/anuraghagawane/luma/internal/repository/elastic"
	"github.com/anuraghagawane/luma/internal/repository/postgres"
)

func main() {
	fmt.Println("Luma Query starting")

	cfg, err := config.LoadEnv()
	if err != nil {
		log.Fatalf("Error while parsing env: %v", err)
	}

	dbpool, err := postgres.NewPool(context.Background(), cfg.PostgresDBUrl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to create connection pool: %v\n", err)
		os.Exit(1)
	}
	defer dbpool.Close()

	elasticAddresses := []string{cfg.ElasticBroker}
	elasticIndex := "logs"
	logRepo, err := elastic.NewLogRepo(elasticAddresses, elasticIndex)
	if err != nil {
		log.Fatalf("Failed to initiate Log repository %v", err)
	}

	userRepo := postgres.NewUserRepo(dbpool)

	tokenManager, err := auth.NewTokenManager(cfg.JwtSecret, cfg.TokenLifetime)
	if err != nil {
		log.Fatalf("Failed to initiate token manager: %v", err)
	}

	queryTimeout, err := time.ParseDuration("1m")
	if err != nil {
		log.Fatalf("failed to initialize QueryHandler: %v", err)
	}
	queryHandler := query.NewHandler(logRepo, queryTimeout)
	authHandler := auth.NewHandler(userRepo, tokenManager)

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/createaccount", authHandler.HandleCreateAccount)
	mux.HandleFunc("/v1/login", authHandler.HandleLogin)
	mux.Handle("/v1/logs", auth.AuthMiddleware(tokenManager)(http.HandlerFunc(queryHandler.HandleLogQuery)))

	server := &http.Server{
		Addr:              ":" + cfg.QueryPort,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Fatal(server.ListenAndServe())
}
