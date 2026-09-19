package kafka

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/anuraghagawane/luma/internal/domain"
	"github.com/twmb/franz-go/pkg/kgo"
)

// Mock log repository
type MockLogRepository struct {
	mu          sync.Mutex
	callCount   int
	failErr     error
	failUntil   int
	indexedLogs []domain.Log
}

func (m *MockLogRepository) Index(ctx context.Context, id string, document domain.Log) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.callCount++
	m.indexedLogs = append(m.indexedLogs, document)

	if m.callCount <= m.failUntil {
		return m.failErr
	}

	return nil
}

func (m *MockLogRepository) GetCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

func (m *MockLogRepository) GetIndexedLogs() []domain.Log {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.indexedLogs
}

// Mock Producer
type MockFranzProducer struct {
	mu           sync.Mutex
	publishCount int
	failErr      error
	messages     []PublishedMessage
}

type PublishedMessage struct {
	Topic string
	Key   []byte
	Value []byte
}

func (m *MockFranzProducer) Publish(ctx context.Context, topic string, key []byte, value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.publishCount++
	m.messages = append(m.messages, PublishedMessage{
		Topic: topic,
		Key:   key,
		Value: value,
	})

	return m.failErr
}

func (m *MockFranzProducer) Close() error {
	return nil
}

func (m *MockFranzProducer) GetPublishCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.publishCount
}

func (m *MockFranzProducer) GetPublishedMessages() []PublishedMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.messages
}

func createTestLog(eventID string) domain.Log {
	return domain.Log{
		EventID:   eventID,
		Tenant:    "test-tenant",
		Host:      "test-host",
		LogLevel:  domain.ERROR,
		Message:   "test message",
		Timestamp: 1234567890,
	}
}

func TestConsumerProcessMessage(t *testing.T) {
	tests := []struct {
		name               string
		indexFailUntil     int
		totalIndexCall     int
		indexErr           error
		expectedRetries    bool
		expectDLQ          bool
		expectOffsetCommit bool
	}{
		{
			name:               "success_first_attempt",
			indexFailUntil:     0,
			totalIndexCall:     1,
			indexErr:           nil,
			expectedRetries:    false,
			expectDLQ:          false,
			expectOffsetCommit: true,
		},
		{
			name:               "retry_then_success",
			indexFailUntil:     2,
			totalIndexCall:     3,
			indexErr:           &domain.LogError{Type: domain.ErrorTypeRetryable},
			expectedRetries:    true,
			expectDLQ:          false,
			expectOffsetCommit: true,
		},
		{
			name:               "permanent_error_to_dlq",
			indexFailUntil:     1,
			totalIndexCall:     1,
			indexErr:           &domain.LogError{Type: domain.ErrorTypePermanent},
			expectedRetries:    false,
			expectDLQ:          true,
			expectOffsetCommit: true,
		},
		{
			name:               "all_retries_exhausted_to_dlq",
			indexFailUntil:     MAX_RETRIES,
			totalIndexCall:     MAX_RETRIES,
			indexErr:           &domain.LogError{Type: domain.ErrorTypeRetryable},
			expectedRetries:    true,
			expectDLQ:          true,
			expectOffsetCommit: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &MockLogRepository{
				failUntil: tt.indexFailUntil,
				failErr:   tt.indexErr,
			}

			mockDLQProducer := &MockFranzProducer{}

			logEntry := createTestLog("event-123")

			mockConsumer := &FranzConsumer{
				client:      nil,
				topic:       "logs",
				ctx:         context.Background(),
				logRepo:     mockRepo,
				dlqTopic:    "logs-dlq",
				dlqProducer: mockDLQProducer,
			}
			record := &kgo.Record{
				Topic:     "log",
				Partition: 0,
				Offset:    10,
				Key:       []byte("event-1"),
				Value:     []byte(`{"eventid":"event-1"}`),
			}

			commitCalled := false
			dlqCalled := false

			mockCommitFn := func(record *kgo.Record) { commitCalled = true }
			mockSendDLQFn := func(logEntry domain.Log, failureErr error, errorType string, attempts int) error {
				dlqCalled = true
				return nil
			}

			mockBackOffFn := func(int) time.Duration {
				return 10 * time.Millisecond
			}

			mockConsumer.processMessage(record, logEntry, mockCommitFn, mockSendDLQFn, mockBackOffFn)

			if mockRepo.callCount != tt.totalIndexCall {
				t.Errorf("Call count not matching called: %d, expected: %d", mockRepo.callCount, tt.totalIndexCall)
			}

			if tt.expectDLQ != dlqCalled {
				t.Errorf("Invalid DLQ call, Expected: %v, Got: %v", tt.expectDLQ, dlqCalled)
			}

			if tt.expectOffsetCommit != commitCalled {
				t.Errorf("Wrong commit behaviour, Expected call: %v, got call: %v", tt.expectOffsetCommit, commitCalled)
			}
		})
	}
}
