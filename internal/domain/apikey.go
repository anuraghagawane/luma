package domain

import (
	"context"
	"time"
)

type APIKeyStatus string

const (
	APIKeyActive  APIKeyStatus = "ACTIVE"
	APIKeyRevoked APIKeyStatus = "REVOKED"
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
	CreateAPIKeyAndEvent(ctx context.Context, apiKeyObj APIKey) error
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
