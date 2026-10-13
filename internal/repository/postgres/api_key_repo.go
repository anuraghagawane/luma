package postgres

import (
	"context"
	"fmt"

	"github.com/anuraghagawane/luma/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type APIKeyRepo struct {
	client *pgxpool.Pool
}

func NewAPIKeyRepo(client *pgxpool.Pool) *APIKeyRepo {
	return &APIKeyRepo{client: client}
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
