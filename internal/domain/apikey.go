package domain

import (
	"context"
	"errors"
	"time"
)

type APIKeyStatus string

const (
	APIKeyActive  APIKeyStatus = "ACTIVE"
	APIKeyRevoked APIKeyStatus = "REVOKED"
)

var (
	ErrKeyNotFound       = errors.New("key not found")
	ErrKeyAlreadyRevoked = errors.New("key already revoked/deleted")
)

type APIKey struct {
	KeyID     string       `json:"key_id"`
	TenantID  string       `json:"tenant_id"`
	Name      string       `json:"name"`
	Key       string       `json:"-"`
	Hash      string       `json:"hash"`
	Status    APIKeyStatus `json:"status"`
	Version   int          `json:"version"`
	CreatedAt time.Time    `json:"created_at"`
	CreatedBy string       `json:"created_by"`
	UpdatedAt time.Time    `json:"updated_at"`
	UpdatedBy string       `json:"updated_by"`
}

type APIKeyRedisPayload struct {
	KeyID    string       `json:"key_id"`
	TenantID string       `json:"tenant_id"`
	Hash     string       `json:"hash"`
	Status   APIKeyStatus `json:"status"`
	Version  int          `json:"version"`
}

type APIKeyRepository interface {
	AllAPIKey(ctx context.Context, tenantID string) ([]APIKey, error)
	CreateAPIKeyAndEvent(ctx context.Context, apiKeyObj APIKey) error
	DeleteAPIKeyAndCreateEvent(ctx context.Context, keyID string, tenantID string, userID string) error
}

func GenerateKeyRedisPayload(apiKey APIKey) APIKeyRedisPayload {
	return APIKeyRedisPayload{
		KeyID:    apiKey.KeyID,
		TenantID: apiKey.TenantID,
		Hash:     apiKey.Hash,
		Status:   apiKey.Status,
		Version:  apiKey.Version,
	}
}
