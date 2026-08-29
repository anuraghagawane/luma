package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/anuraghagawane/luma/internal/api"
	"github.com/anuraghagawane/luma/internal/config"
	"github.com/anuraghagawane/luma/internal/infra/kafka"
)

func main() {
	fmt.Println("Luma started...")

	cfg, err := config.LoadEnv()
	if err != nil {
		log.Fatalf("Error while parsing env: %v", err)
	}
	seeds := []string{cfg.KafkaBroker}
	producer, err := kafka.NewFranzProducer(seeds)
	if err != nil {
		log.Fatalf("Init error: %v", err)
	}
	defer producer.Close()

	logHandler := api.NewLogHandler(producer)

	http.HandleFunc("/v1/log", logHandler.HandleLog)
	http.HandleFunc("/v1/bulklog", logHandler.HandleBulkLog)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
