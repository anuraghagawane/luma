package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/anuraghagawane/luma/internal/config"
	"github.com/anuraghagawane/luma/internal/infra/kafka"
	"github.com/anuraghagawane/luma/internal/repository/elastic"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
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
	createTopic(topicName, seeds)
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

func createTopic(topicName string, seeds []string) {
	cl, err := kgo.NewClient(
		kgo.SeedBrokers(seeds...),
	)
	if err != nil {
		panic(err)
	}

	fmt.Println("Client is created")

	defer cl.Close()

	ctx := context.Background()
	adminClient := kadm.NewClient(cl)
	var partitions int32 = 3
	var replicationFactor int16 = 1

	fmt.Println("Checking if topic exists...")
	topicDetails, err := adminClient.ListTopics(ctx)
	if err != nil {
		log.Fatalf("Failed to list topics: %v", err)
	}

	if topicDetails.Has(topicName) {
		fmt.Printf("Topic '%s' already exists. Skipping creation.\n", topicName)
		return
	}

	fmt.Printf("Topic '%s' not found. Creating now...\n", topicName)
	resp, err := adminClient.CreateTopic(ctx, partitions, replicationFactor, nil, topicName)
	if err != nil {
		log.Fatalf("Failed to execute request: %v", err)
	}

	if resp.Err != nil {
		log.Fatalf("Server failed to create topic: %v", resp.Err)
	}

	fmt.Printf("Successfully created topic: %s\n", resp.Topic)
}
