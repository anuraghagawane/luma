// Package kafka is an internal package which is thin wrapper above the franz-go
package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/anuraghagawane/luma/internal/domain"
	"github.com/twmb/franz-go/pkg/kgo"
)

type FranzConsumer struct {
	client      *kgo.Client
	topic       string
	ctx         context.Context
	cancel      context.CancelFunc
	logRepo     domain.LogRepository
	dlqTopic    string
	dlqProducer domain.EventProducer
}

func NewFranzConsumer(brokers []string, groupID string, topic string, logRepo domain.LogRepository) (*FranzConsumer, error) {
	ctx, cancel := context.WithCancel(context.Background())
	cl, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(groupID),
		kgo.ConsumeTopics(topic),
		kgo.DisableAutoCommit(),
	)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create kafka client %w", err)
	}

	dlqProducer, err := NewFranzProducer(brokers)
	if err != nil {
		cancel()
		cl.Close()
		return nil, fmt.Errorf("failed to create DLQ producer: %w", err)
	}

	return &FranzConsumer{
		client:      cl,
		topic:       topic,
		ctx:         ctx,
		cancel:      cancel,
		logRepo:     logRepo,
		dlqTopic:    "log-dlq",
		dlqProducer: dlqProducer,
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

				fc.processMessage(record, logEntry, fc.commitOffset, fc.sendToDLQ, calculateBackoff)
			}
		}
	}
}

type (
	offsetCommiter func(record *kgo.Record)
	dlqSender      func(logEntry domain.Log, failureErr error, errorType string, attempts int) error
	backoffFunc    func(int) time.Duration
)

func (fc *FranzConsumer) processMessage(record *kgo.Record, logEntry domain.Log, commitFn offsetCommiter, sendDLQFn dlqSender, backoffFn backoffFunc) {
	err := fc.logRepo.Index(fc.ctx, logEntry.EventID, logEntry)

	for attempt := range MAX_RETRIES {
		if err == nil {
			break
		}

		if !isRetryable(err) {
			break
		}

		if attempt < MAX_RETRIES-1 {
			backoff := backoffFn(attempt)
			log.Printf("Retrying after %v (attempt %d/%d)", backoff, attempt+1, MAX_RETRIES)
			time.Sleep(backoff)

			err = fc.logRepo.Index(fc.ctx, logEntry.EventID, logEntry)
		}
	}

	if err == nil {
		log.Printf("Successfully indexed %s", logEntry.EventID)
		commitFn(record)
	} else if isRetryable(err) {
		log.Printf("Max retries exhausted for %s, sending to DLQ", logEntry.EventID)
		dlqErr := sendDLQFn(logEntry, err, "retryable", MAX_RETRIES)
		if dlqErr == nil {
			commitFn(record)
		}
	} else {
		log.Printf("Permanent error for %s, sending to DLQ", logEntry.EventID)
		dlqErr := sendDLQFn(logEntry, err, "permanent", MAX_RETRIES)
		if dlqErr == nil {
			commitFn(record)
		}
	}
}

func calculateBackoff(attempt int) time.Duration {
	backoff := time.Duration(math.Pow(2, float64(attempt))) * INITIAL_BACKOFF_MS * time.Millisecond

	if backoff > (MAX_BACKOFF_MS*time.Millisecond) || backoff < 0 {
		return MAX_BACKOFF_MS * time.Millisecond
	}

	return backoff
}

func (fc *FranzConsumer) commitOffset(record *kgo.Record) {
	offsets := make(map[string]map[int32]kgo.EpochOffset)
	offsets[record.Topic] = make(map[int32]kgo.EpochOffset)
	offsets[record.Topic][record.Partition] = kgo.EpochOffset{
		Epoch:  record.LeaderEpoch,
		Offset: record.Offset + 1,
	}
	fc.client.CommitOffsetsSync(fc.ctx, offsets, nil)
	log.Printf("Committed offset %d for topic %s partition %d",
		record.Offset+1, record.Topic, record.Partition)
}

func (fc *FranzConsumer) sendToDLQ(logEntry domain.Log, failureErr error, errorType string, attempts int) error {
	dlqMsg := NewDLQMessage(logEntry, failureErr, errorType, attempts)

	msgData, err := dlqMsg.ToJSON()
	if err != nil {
		log.Printf("Failed to marshal DLQ message: %v", err)
		return err
	}

	err = fc.dlqProducer.Publish(fc.ctx, fc.dlqTopic, []byte(logEntry.EventID), msgData)
	if err != nil {
		log.Printf("Failed to send to DLQ: %v", err)
		return err
	}

	log.Printf("Sent to DLQ: %s (error: %s)", logEntry.EventID, failureErr)
	return nil
}

func (fc *FranzConsumer) Close() error {
	fc.cancel()
	fc.client.Close()
	fc.dlqProducer.Close()
	return nil
}
