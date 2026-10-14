package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/anuraghagawane/luma/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type APIKeyRepo struct {
	client *pgxpool.Pool
}

func NewAPIKeyRepo(client *pgxpool.Pool) *APIKeyRepo {
	return &APIKeyRepo{client: client}
}

func (r *APIKeyRepo) AllAPIKey(ctx context.Context, tenantID string) ([]domain.APIKey, error) {
	query := "select name, key_id, status, created_at, updated_at, version from api_keys where tenant_id = $1 order by created_at desc, key_id;"

	rows, err := r.client.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrive api keys: %w", err)
	}
	defer rows.Close()

	var apikeys []domain.APIKey
	for rows.Next() {
		var apikey domain.APIKey
		err := rows.Scan(&apikey.Name, &apikey.KeyID, &apikey.Status, &apikey.CreatedAt, &apikey.UpdatedAt, &apikey.Version)
		if err != nil {
			return nil, fmt.Errorf("failed to scan api keys: %w", err)
		}

		apikeys = append(apikeys, apikey)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to retrive api keys: %w", err)
	}
	return apikeys, nil
}

func (r *APIKeyRepo) CreateAPIKeyAndEvent(ctx context.Context, apiKeyObj domain.APIKey) error {
	db, err := r.client.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	defer db.Rollback(ctx)

	query := "INSERT into api_keys (key_id, tenant_id, name, hash, version, created_by, updated_by) VALUES ($1, $2, $3, $4, $5, $6, $7);"

	_, err = db.Exec(ctx, query, apiKeyObj.KeyID, apiKeyObj.TenantID, apiKeyObj.Name, apiKeyObj.Hash, apiKeyObj.Version, apiKeyObj.CreatedBy, apiKeyObj.UpdatedBy)
	if err != nil {
		return fmt.Errorf("failed to create api key: %w", err)
	}

	redisPayload := domain.GenerateKeyRedisPayload(apiKeyObj)

	query = "INSERT into outbox_events (aggregate_type, aggregate_id, aggregate_version, event_type, payload) values ($1, $2, $3, $4, $5);"

	_, err = db.Exec(ctx, query, "API_KEY", apiKeyObj.KeyID, apiKeyObj.Version, "API_KEY_STATE_CHANGED", redisPayload)
	if err != nil {
		return fmt.Errorf("failed to create outbox event: %w", err)
	}

	err = db.Commit(ctx)
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	return nil
}

func (r *APIKeyRepo) DeleteAPIKeyAndCreateEvent(ctx context.Context, keyID string, tenantID string, userID string) error {
	db, err := r.client.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to create the transaction: %w", err)
	}

	defer db.Rollback(ctx)
	query := "SELECT status FROM api_keys WHERE key_id = $1 AND tenant_id = $2"

	var status string
	err = db.QueryRow(ctx, query, keyID, tenantID).Scan(&status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrKeyNotFound
		}
		return fmt.Errorf("lookup key: %w", err)
	}

	if status != string(domain.APIKeyActive) {
		return domain.ErrKeyAlreadyRevoked
	}
	query = "UPDATE api_keys SET status = 'REVOKED', version = version + 1, updated_by = $1, updated_at = NOW() where key_id = $2 and tenant_id = $3 RETURNING key_id, tenant_id, hash, status, version;"

	var redisPayload domain.APIKeyRedisPayload
	err = db.QueryRow(ctx, query, userID, keyID, tenantID).Scan(&redisPayload.KeyID, &redisPayload.TenantID, &redisPayload.Hash, &redisPayload.Status, &redisPayload.Version)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrKeyNotFound
		}
		return fmt.Errorf("failed to delete key: %w", err)
	}

	query = "INSERT into outbox_events (aggregate_type, aggregate_id, aggregate_version, event_type, payload) values ($1, $2, $3, $4, $5);"

	_, err = db.Exec(ctx, query, "API_KEY", redisPayload.KeyID, redisPayload.Version, "API_KEY_STATE_CHANGED", redisPayload)
	if err != nil {
		return fmt.Errorf("failed to create outbox event: %w", err)
	}

	err = db.Commit(ctx)
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}
	return nil
}
