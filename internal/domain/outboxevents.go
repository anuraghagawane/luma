package domain

import (
	"context"
	"errors"
	"time"
)

type OutBoxEvents struct {
	ID               string             `json:"id"`
	AggregateType    string             `json:"aggregate_type"`
	AggregateID      string             `json:"aggregate_id"`
	AggregateVersion int                `json:"aggregate_version"`
	EventType        string             `json:"event_type"`
	Payload          APIKeyRedisPayload `json:"payload"`
	Status           string             `json:"status"`
	Attempts         int                `json:"attempts"`
	AvailableAt      time.Time          `json:"available_at"`
	LockedBy         string             `json:"locked_by"`
	LockedUntil      *time.Time         `json:"locked_until"`
	ClaimToken       string             `json:"claim_token"`
	CreatedAt        time.Time          `json:"created_at"`
	ProcessedAt      *time.Time         `json:"processed_at"`
	LastError        string             `json:"last_error"`
}

var ErrLostClaim = errors.New("claim lost")

type OutboxEventRepository interface {
	ClaimBatch(ctx context.Context, workerID string) ([]OutBoxEvents, error)
	MarkProcessed(ctx context.Context, id string, ClaimToken string) error
	ScheduleRetry(ctx context.Context, id string, cause error, claimToken string, delay time.Duration) error
	MarkDead(ctx context.Context, id string, cause error, claimToken string) error
}
