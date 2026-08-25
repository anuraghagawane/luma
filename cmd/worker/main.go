package main

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
)

func main() {
	topicName := "foo"
	seeds := []string{"localhost:9092"}

	cl, err := kgo.NewClient(
		kgo.SeedBrokers(seeds...),
		kgo.ConsumerGroup("my-group-identifier"),
		kgo.ConsumeTopics("foo"),
	)
	if err != nil {
		panic(err)
	}

	fmt.Println("Client is created")

	defer cl.Close()

	createTopic(cl, topicName)

	ctx := context.Background()

	var wg sync.WaitGroup

	wg.Add(1)
	record := &kgo.Record{Topic: topicName, Value: []byte("bar")}
	cl.Produce(ctx, record, func(_ *kgo.Record, err error) {
		defer wg.Done()
		if err != nil {
			fmt.Printf("record had a produce error: %v\n", err)
		}
	})
	wg.Wait()

	if err := cl.ProduceSync(ctx, record).FirstErr(); err != nil {
		fmt.Printf("record had a produce error while synchronously producing: %v\n", err)
	}

	for {
		fetches := cl.PollFetches(ctx)

		if errs := fetches.Errors(); len(errs) > 0 {
			panic(fmt.Sprint(errs))
		}

		iter := fetches.RecordIter()

		for !iter.Done() {
			record := iter.Next()
			fmt.Println(string(record.Value), "from an iterator!")
		}

		fetches.EachPartition(func(p kgo.FetchTopicPartition) {
			for _, record := range p.Records {
				fmt.Println(string(record.Value), "from range inside a callback!")
			}

			p.EachRecord(func(record *kgo.Record) {
				fmt.Println(string(record.Value), "from a second callback!")
			})
		})
	}
}

func createTopic(cl *kgo.Client, topicName string) {
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
