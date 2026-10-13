package domain

import "time"

type OutBoxEvents struct {
	ID               string          `json:"id"`
	AggregateType    string          `json:"aggregate_type"`
	AggregateID      string          `json:"aggregate_id"`
	AggregateVersion int             `json:"aggregate_version"`
	EventType        string          `json:"event_type"`
	Payload          RedisAPIKeyData `json:"payload"`
	Status           string          `json:"status"`
	Attempts         int             `json:"attempts"`
	AvailableAt      time.Time       `json:"available_at"`
	LockedBy         string          `json:"locked_by"`
	LockedUntil      *time.Time      `json:"locked_until"`
	CreatedAt        time.Time       `json:"created_at"`
	ProcessedAt      *time.Time      `json:"processed_at"`
	LastError        string          `json:"last_error"`
}

type RedisAPIKeyData struct {
	KeyID    string       `json:"key_id"`
	TenantID string       `json:"tenant_id"`
	Hash     string       `json:"hash"`
	Status   APIKeyStatus `json:"status"`
	Version  int          `json:"version"`
}
