// Package kafka is an internal package which is thin wrapper above the franz-go
package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/anuraghagawane/luma/internal/domain"
	"github.com/twmb/franz-go/pkg/kgo"
)

type FranzConsumer struct {
	client  *kgo.Client
	topic   string
	ctx     context.Context
	cancel  context.CancelFunc
	logRepo domain.LogRepository
}

func NewFranzConsumer(brokers []string, groupID string, topic string, logRepo domain.LogRepository) (*FranzConsumer, error) {
	ctx, cancel := context.WithCancel(context.Background())
	cl, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(groupID),
		kgo.ConsumeTopics(topic),
	)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create kafka client %w", err)
	}

	return &FranzConsumer{
		client:  cl,
		topic:   topic,
		ctx:     ctx,
		cancel:  cancel,
		logRepo: logRepo,
	}, nil
}

func (fc *FranzConsumer) Start() error {
	fmt.Println("starting consumer...")
	for {
		select {
		case <-fc.ctx.Done():
			return fc.ctx.Err()
		default:
			fetches := fc.client.PollFetches(fc.ctx)
			if fetches.IsClientClosed() {
				return nil
			}

			if err := fetches.Errors(); err != nil {
				log.Printf("kafka fetch errors: %v", err)
				continue
			}

			iter := fetches.RecordIter()
			for !iter.Done() {
				record := iter.Next()

				var logEntry domain.Log
				if err := json.Unmarshal(record.Value, &logEntry); err != nil {
					log.Printf("Failed to deserialize log on partition %d, offset %d: %v", record.Partition, record.Offset, err)
					continue
				}

				if err := fc.logRepo.Index(fc.ctx, logEntry.EventID, logEntry); err != nil {
					log.Printf("Failed to index log ID %s: %v", logEntry.EventID, err)
					continue
				}

				log.Println("Indexed", logEntry)
			}
		}
	}
}

func (fc *FranzConsumer) Close() error {
	fc.cancel()
	fc.client.Close()
	return nil
}
