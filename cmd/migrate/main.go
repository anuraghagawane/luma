package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/anuraghagawane/luma/internal/config"
	"github.com/anuraghagawane/luma/internal/repository/postgres"
)

func main() {
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

	migrator := postgres.NewMigrator(dbpool, "./migrations/", context.Background())
	migrator.RunMigration()

	migrator.Close()
}
