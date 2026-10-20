// Package cache implementes wrapper over redis lib
package cache

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"github.com/anuraghagawane/luma/internal/domain"
	"github.com/redis/go-redis/v9"
)

type CacheClient struct {
	rdb *redis.Client
}

func NewCacheClient(rdb *redis.Client) *CacheClient {
	return &CacheClient{rdb: rdb}
}

//go:embed scripts/apply_api_key.lua
var applyAPIKeyLua string

var applyAPIKeyScript = redis.NewScript(applyAPIKeyLua)

func (c *CacheClient) Apply(ctx context.Context, key string, payload domain.APIKeyRedisPayload, ttl time.Duration) (domain.CacheApplyOutcome, error) {
	result, err := applyAPIKeyScript.Run(ctx, c.rdb, []string{key}, payload.KeyID, payload.TenantID, payload.Hash, string(payload.Status), payload.Version, ttl.Seconds()).Int()
	if err != nil {
		return 0, fmt.Errorf("failed to run redis script: %w", err)
	}

	switch domain.CacheApplyOutcome(result) {
	case domain.CacheApplyUpdated,
		domain.CacheApplyUnchanged,
		domain.CacheApplyStale:
		return domain.CacheApplyOutcome(result), nil
	default:
		return 0, fmt.Errorf("redis script returned unexpected outcome %d", result)
	}
}

func (c *CacheClient) GetAPIKey(ctx context.Context, key string) (*domain.APIKeyRedisPayload, error) {
	var apiKeyPayload domain.APIKeyRedisPayload
	err := c.rdb.HGetAll(ctx, key).Scan(&apiKeyPayload)
	if err != nil {
		return nil, fmt.Errorf("get API key from redis: %w", err)
	}

	if apiKeyPayload.KeyID == "" {
		return nil, domain.ErrKeyNotFound
	}

	return &apiKeyPayload, nil
}
