// Package cache implementes wrapper over redis lib
package cache

import (
	"context"
	"fmt"

	"github.com/anuraghagawane/luma/internal/domain"
	"github.com/redis/go-redis/v9"
)

type CacheClient struct {
	rdb *redis.Client
}

func NewCacheClient(rdb *redis.Client) *CacheClient {
	return &CacheClient{rdb: rdb}
}

var applyAPIKeyScript = redis.NewScript(`
		local api_key = KEYS[1]
		local key_id = ARGV[1]
		local tenant_id = ARGV[2]
		local hash = ARGV[3]
		local status = ARGV[4]

		local versionRaw = ARGV[5]
		local version = tonumber(versionRaw)
		if versionRaw and not version then
			return redis.error_reply("invalid version")
		end

		local currentVersionRaw = redis.call("HGET", api_key, "version")
		local currentVersion = tonumber(currentVersionRaw)
		if currentVersionRaw and not currentVersion then
				return redis.error_reply("invalid_cached_version")
		end

		if currentVersion then
			if currentVersion > version then return -1 end
			if currentVersion == version then return 0 end
		end

		redis.call("HSET", api_key, "key_id", key_id, "tenant_id", tenant_id, "hash", hash, "status", status, "version", version)
		return 1
	`)

func (c *CacheClient) Apply(ctx context.Context, key string, payload domain.APIKeyRedisPayload) (domain.CacheApplyOutcome, error) {
	result, err := applyAPIKeyScript.Run(ctx, c.rdb, []string{key}, payload.KeyID, payload.TenantID, payload.Hash, string(payload.Status), payload.Version).Int()
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
