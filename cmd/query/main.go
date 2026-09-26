package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/anuraghagawane/luma/internal/api"
	"github.com/anuraghagawane/luma/internal/config"
	"github.com/anuraghagawane/luma/internal/repository/elastic"
)

func main() {
	fmt.Println("Luma Query starting")

	cfg, err := config.LoadEnv()
	if err != nil {
		log.Fatalf("Error while parsing env: %v", err)
	}

	elasticAddresses := []string{cfg.ElasticBroker}
	elasticIndex := "logs"
	logRepo, err := elastic.NewLogRepo(elasticAddresses, elasticIndex)
	if err != nil {
		log.Fatalf("Failed to initiate Log repository %v", err)
	}

	queryHandler := api.NewQueryHandler(logRepo)
	http.HandleFunc("/v1/logs", queryHandler.HandleLogQuery)

	log.Fatal(http.ListenAndServe(":"+cfg.QueryPort, nil))
}
