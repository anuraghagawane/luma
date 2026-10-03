package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

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

	tokenManager, err := auth.NewTokenManager(cfg.JwtSecret)
	if err != nil {
		log.Fatalf("Failed to initiate token manager: %v", err)
	}

	queryHandler := query.NewHandler(logRepo)
	authHandler := auth.NewHandler(userRepo, tokenManager)

	http.HandleFunc("/v1/logs", queryHandler.HandleLogQuery)
	http.HandleFunc("/v1/createaccount", authHandler.HandleCreateAccount)
	http.HandleFunc("/v1/login", authHandler.HandleLogin)

	log.Fatal(http.ListenAndServe(":"+cfg.QueryPort, nil))
}
