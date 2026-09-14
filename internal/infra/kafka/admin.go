package kafka

import (
	"context"
	"fmt"
	"log"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
)

func CreateTopic(topicName string, seeds []string) {
	cl, err := kgo.NewClient(
		kgo.SeedBrokers(seeds...),
	)
	if err != nil {
		log.Fatalf("No able to create topic: %v", err)
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
