//go:build integration

package kafka_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/anuraghagawane/luma/internal/config"
	"github.com/anuraghagawane/luma/internal/domain"
	"github.com/anuraghagawane/luma/internal/infra/kafka"
	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
)

func createNewTopic(t *testing.T, topicName string, seeds []string) (*kadm.Client, error) {
	t.Helper()
	cl, err := kgo.NewClient(
		kgo.SeedBrokers(seeds...),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create client")
	}
	t.Cleanup(func() {
		cl.Close()
	})

	ctx := context.Background()
	adminClient := kadm.NewClient(cl)
	t.Cleanup(func() {
		adminClient.Close()
	})
	var partitions int32 = 1
	var replicationFactor int16 = 1
	t.Cleanup(func() {
		resp, err := adminClient.DeleteTopic(ctx, topicName)
		if err != nil {
			t.Fatalf("failed to delete the topic")
		}
		if resp.Err != nil {
			t.Fatalf("Server failed to create topic: %v", resp.Err)
		}
	})

	resp, err := adminClient.CreateTopic(ctx, partitions, replicationFactor, nil, topicName)
	if err != nil {
		return nil, fmt.Errorf("Failed to execute request: %v", err)
	}

	if resp.Err != nil {
		return nil, fmt.Errorf("Server failed to create topic: %v", resp.Err)
	}

	return adminClient, nil
}

func ensureTopicExists(ctx context.Context, adminClient *kadm.Client, topicName string) error {
	topics, err := adminClient.ListTopics(ctx)
	if err != nil {
		return fmt.Errorf("list Kafka topics: %w", err)
	}
	if topics.Has(topicName) {
		return nil
	}

	response, err := adminClient.CreateTopic(ctx, 1, 1, nil, topicName)
	if err != nil {
		return fmt.Errorf("create Kafka topic %q: %w", topicName, err)
	}
	if response.Err != nil {
		return fmt.Errorf("create Kafka topic %q: %w", topicName, response.Err)
	}
	return nil
}

func waitForGroupAssignment(ctx context.Context, adminClient *kadm.Client, groupID, topicName string, stopped <-chan error) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		groups, err := adminClient.DescribeGroups(ctx, groupID)
		if err == nil {
			group, ok := groups[groupID]
			if ok && group.Err == nil && group.State == "Stable" && len(group.AssignedPartitions()[topicName]) > 0 {
				return nil
			}
		}

		select {
		case startErr := <-stopped:
			if startErr == nil {
				return fmt.Errorf("consumer stopped before assignment")
			}
			return fmt.Errorf("consumer stopped before assignment: %w", startErr)
		case <-ticker.C:
		case <-ctx.Done():
			return fmt.Errorf("wait for group %q assignment: %w", groupID, ctx.Err())
		}
	}
}

func TestFranzProducer_PublishAndReadMessage(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cfg, err := config.LoadEnv()
	if err != nil {
		t.Fatalf("Error while parsing env: %v", err)
	}

	// kafka
	seeds := []string{cfg.KafkaBroker}

	topicName := "test_logs_" + uuid.NewString()
	_, err = createNewTopic(t, topicName, seeds)
	if err != nil {
		t.Fatalf("Error createNewTopic: %v", err)
	}

	producer, err := kafka.NewFranzProducer(seeds)
	if err != nil {
		t.Fatalf("Error NewFranzProducer: %v", err)
	}
	t.Cleanup(func() { _ = producer.Close() })

	log := domain.Log{
		EventID:   uuid.NewString(),
		Tenant:    "abc",
		Service:   "svc",
		Host:      "h1",
		LogLevel:  domain.DEBUG,
		Message:   "account created",
		Timestamp: time.Now().Unix(),
	}

	pubData, err := json.Marshal(log)
	if err != nil {
		t.Fatalf("failed to unmarshall the log: %v", err)
	}

	err = producer.Publish(ctx, topicName, []byte(log.EventID), pubData)
	if err != nil {
		t.Fatalf("Error Publish: %v", err)
	}

	groupID := "group" + topicName

	consumer, err := kgo.NewClient(
		kgo.SeedBrokers(seeds...),
		kgo.ConsumerGroup(groupID),
		kgo.ConsumeTopics(topicName),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		t.Fatalf("failed to create kafka client %v", err)
	}
	t.Cleanup(func() { consumer.Close() })

	var fetchedRecord *kgo.Record
	for fetchedRecord == nil && ctx.Err() == nil {
		records := consumer.PollFetches(ctx)
		if records.Err() != nil {
			t.Fatalf("Error PollFetches: %v", records.Err())
		}
		records.EachRecord(func(record *kgo.Record) {
			if record.Topic == topicName {
				fetchedRecord = record
			}
		})
	}

	if fetchedRecord == nil {
		t.Fatalf("record not received before deadline: %v", ctx.Err())
	}

	var rec domain.Log

	err = json.Unmarshal(fetchedRecord.Value, &rec)
	if err != nil {
		t.Fatalf("failed to unmarshall the log: %v", err)
	}

	if string(fetchedRecord.Key) != log.EventID {
		t.Errorf("key: %v, want %v", string(fetchedRecord.Key), log.EventID)
	}

	if rec.EventID != log.EventID {
		t.Errorf("EventID: %v, want %v", rec.EventID, log.EventID)
		return
	}

	if rec.Host != log.Host {
		t.Errorf("Host: %v, want %v", rec.Host, log.Host)
		return
	}

	if rec.Tenant != log.Tenant {
		t.Errorf("Tenant: %v, want %v", rec.Tenant, log.Tenant)
		return
	}

	if rec.Service != log.Service {
		t.Errorf("Service: %v, want %v", rec.Service, log.Service)
		return
	}

	if rec.Message != log.Message {
		t.Errorf("Message: %v, want %v", rec.Message, log.Message)
		return
	}

	if rec.Timestamp != log.Timestamp {
		t.Errorf("Timestamp: %v, want %v", rec.Timestamp, log.Timestamp)
		return
	}

	if rec.LogLevel != log.LogLevel {
		t.Errorf("LogLevel: %v, want %v", rec.LogLevel, log.LogLevel)
		return
	}
}

type LogRepo struct {
	indexed chan domain.Log
	err     error
}

func (r *LogRepo) Index(ctx context.Context, id string, document domain.Log) error {
	select {
	case r.indexed <- document:
		return r.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestFranzConsumer_IndexesValidMessage(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cfg, err := config.LoadEnv()
	if err != nil {
		t.Fatalf("Error while parsing env: %v", err)
	}

	// kafka
	seeds := []string{cfg.KafkaBroker}

	topicName := "test_logs_" + uuid.NewString()
	adminClient, err := createNewTopic(t, topicName, seeds)
	if err != nil {
		t.Fatalf("Error createNewTopic: %v", err)
	}

	logRepo := LogRepo{
		indexed: make(chan domain.Log),
	}

	groupID := "group" + topicName
	consumer, err := kafka.NewFranzConsumer(seeds, groupID, topicName, &logRepo)
	if err != nil {
		t.Fatalf("Error NewFranzConsumer(): %v", err)
	}
	t.Cleanup(func() { consumer.Close() })

	producer, err := kafka.NewFranzProducer(seeds)
	if err != nil {
		t.Fatalf("Error NewFranzProducer: %v", err)
	}
	t.Cleanup(func() { _ = producer.Close() })

	startErr := make(chan error, 1)
	go func() {
		startErr <- consumer.Start()
	}()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		groups, err := adminClient.DescribeGroups(ctx, groupID)
		if err == nil {
			group, ok := groups[groupID]
			if ok && group.Err == nil && group.State == "Stable" && group.AssignedPartitions().Lookup(topicName, 0) {
				break
			}
		}
		select {
		case err := <-startErr:
			t.Fatalf("consumer exited before partition assignment: %v", err)
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatalf("consumer was not assigned the topic partition: %v", ctx.Err())
		}
	}

	log := domain.Log{
		EventID:   uuid.NewString(),
		Tenant:    "abc",
		Service:   "svc",
		Host:      "h1",
		LogLevel:  domain.DEBUG,
		Message:   "account created",
		Timestamp: time.Now().Unix(),
	}

	pubData, err := json.Marshal(log)
	if err != nil {
		t.Fatalf("failed to unmarshall the log: %v", err)
	}

	err = producer.Publish(ctx, topicName, []byte(log.EventID), pubData)
	if err != nil {
		t.Fatalf("Error Publish: %v", err)
	}

	select {
	case rec := <-logRepo.indexed:
		if rec.EventID != log.EventID {
			t.Errorf("EventID: %v, want %v", rec.EventID, log.EventID)
			return
		}

		if rec.Host != log.Host {
			t.Errorf("Host: %v, want %v", rec.Host, log.Host)
			return
		}

		if rec.Tenant != log.Tenant {
			t.Errorf("Tenant: %v, want %v", rec.Tenant, log.Tenant)
			return
		}

		if rec.Service != log.Service {
			t.Errorf("Service: %v, want %v", rec.Service, log.Service)
			return
		}

		if rec.Message != log.Message {
			t.Errorf("Message: %v, want %v", rec.Message, log.Message)
			return
		}

		if rec.Timestamp != log.Timestamp {
			t.Errorf("Timestamp: %v, want %v", rec.Timestamp, log.Timestamp)
			return
		}

		if rec.LogLevel != log.LogLevel {
			t.Errorf("LogLevel: %v, want %v", rec.LogLevel, log.LogLevel)
			return
		}
		break

	case <-ctx.Done():
		t.Fatalf("consumer did not index the message: %v", ctx.Err())
	}
}

func TestFranzConsumer_SendsPermanentErrorToDLQ(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cfg, err := config.LoadEnv()
	if err != nil {
		t.Fatalf("Error while parsing env: %v", err)
	}
	seeds := []string{cfg.KafkaBroker}

	topicName := "test_logs_" + uuid.NewString()
	adminClient, err := createNewTopic(t, topicName, seeds)
	if err != nil {
		t.Fatalf("create input topic: %v", err)
	}

	const dlqTopic = "log-dlq"
	if err := ensureTopicExists(ctx, adminClient, dlqTopic); err != nil {
		t.Fatalf("ensure DLQ topic: %v", err)
	}

	dlqGroupID := "dlq-reader-" + uuid.NewString()
	dlqReader, err := kgo.NewClient(
		kgo.SeedBrokers(seeds...),
		kgo.ConsumerGroup(dlqGroupID),
		kgo.ConsumeTopics(dlqTopic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtEnd()),
	)
	if err != nil {
		t.Fatalf("create DLQ reader: %v", err)
	}
	dlqRecords := make(chan *kgo.Record, 16)
	dlqReaderErr := make(chan error, 1)
	dlqReaderDone := make(chan struct{})
	go func() {
		defer close(dlqReaderDone)
		for {
			fetches := dlqReader.PollFetches(ctx)
			if fetches.IsClientClosed() {
				return
			}
			if err := fetches.Err(); err != nil {
				select {
				case dlqReaderErr <- err:
				default:
				}
				return
			}
			fetches.EachRecord(func(record *kgo.Record) {
				select {
				case dlqRecords <- record:
				case <-ctx.Done():
				}
			})
		}
	}()
	t.Cleanup(func() {
		dlqReader.Close()
		select {
		case <-dlqReaderDone:
		case <-time.After(2 * time.Second):
			t.Error("DLQ reader did not stop after close")
		}
	})
	if err := waitForGroupAssignment(ctx, adminClient, dlqGroupID, dlqTopic, dlqReaderErr); err != nil {
		t.Fatalf("DLQ reader did not become ready: %v", err)
	}

	permanentErr := &domain.LogError{
		Type:    domain.ErrorTypePermanent,
		Message: "invalid log for indexing",
	}
	logRepo := &LogRepo{
		indexed: make(chan domain.Log, 1),
		err:     permanentErr,
	}
	consumerGroupID := "log-consumer-" + uuid.NewString()
	consumer, err := kafka.NewFranzConsumer(seeds, consumerGroupID, topicName, logRepo)
	if err != nil {
		t.Fatalf("create log consumer: %v", err)
	}
	t.Cleanup(func() { _ = consumer.Close() })

	startErr := make(chan error, 1)
	go func() {
		startErr <- consumer.Start()
	}()
	if err := waitForGroupAssignment(ctx, adminClient, consumerGroupID, topicName, startErr); err != nil {
		t.Fatalf("log consumer did not become ready: %v", err)
	}

	producer, err := kafka.NewFranzProducer(seeds)
	if err != nil {
		t.Fatalf("create producer: %v", err)
	}
	t.Cleanup(func() { _ = producer.Close() })

	logEntry := domain.Log{
		EventID:   uuid.NewString(),
		Tenant:    "tenant-a",
		Service:   "api",
		Host:      "host-a",
		LogLevel:  domain.ERROR,
		Message:   "invalid log",
		Timestamp: time.Now().Unix(),
	}
	payload, err := json.Marshal(logEntry)
	if err != nil {
		t.Fatalf("marshal log: %v", err)
	}
	if err := producer.Publish(ctx, topicName, []byte(logEntry.EventID), payload); err != nil {
		t.Fatalf("publish input log: %v", err)
	}

	select {
	case indexedLog := <-logRepo.indexed:
		if indexedLog.EventID != logEntry.EventID {
			t.Fatalf("indexed event ID = %q, want %q", indexedLog.EventID, logEntry.EventID)
		}
	case <-ctx.Done():
		t.Fatalf("consumer did not attempt indexing: %v", ctx.Err())
	}

	var dlqRecord *kgo.Record
	for dlqRecord == nil {
		select {
		case record := <-dlqRecords:
			if string(record.Key) == logEntry.EventID {
				dlqRecord = record
			}
		case err := <-dlqReaderErr:
			t.Fatalf("read DLQ message: %v", err)
		case <-ctx.Done():
			t.Fatalf("DLQ message not received before deadline: %v", ctx.Err())
		}
	}

	var dlqMessage kafka.DLQMessage
	if err := json.Unmarshal(dlqRecord.Value, &dlqMessage); err != nil {
		t.Fatalf("unmarshal DLQ message: %v", err)
	}
	if dlqMessage.Log.EventID != logEntry.EventID {
		t.Errorf("DLQ event ID = %q, want %q", dlqMessage.Log.EventID, logEntry.EventID)
	}
	if dlqMessage.Error != permanentErr.Error() {
		t.Errorf("DLQ error = %q, want %q", dlqMessage.Error, permanentErr.Error())
	}
	if dlqMessage.ErrorType != "permanent" {
		t.Errorf("DLQ error type = %q, want %q", dlqMessage.ErrorType, "permanent")
	}
	if dlqMessage.Attempts != kafka.MAX_RETRIES {
		t.Errorf("DLQ attempts = %d, want %d", dlqMessage.Attempts, kafka.MAX_RETRIES)
	}
}
