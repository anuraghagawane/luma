package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/anuraghagawane/luma/internal/config"
	"github.com/anuraghagawane/luma/internal/infra/kafka"
	"github.com/anuraghagawane/luma/internal/repository/elastic"
)

func main() {
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

	seeds := []string{cfg.KafkaBroker}
	topicName := "log"
	kafka.CreateTopic(topicName, seeds)
	kafka.CreateTopic("log-dlq", seeds)
	consumer, err := kafka.NewFranzConsumer(seeds, "log-consumer", topicName, logRepo)
	if err != nil {
		log.Fatalf("Init error: %v", err)
	}

	defer consumer.Close()

	go func() {
		err := consumer.Start()
		if err != nil {
			log.Printf("error while starting consumer: %v", err)
		}
	}()

	// Graceful shutdown handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	log.Println("Shutting down service...")
}
