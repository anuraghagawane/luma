//go:build integration

package cache_test

import (
	"context"
	"errors"
	"log"
	"testing"
	"time"

	"github.com/anuraghagawane/luma/internal/config"
	"github.com/anuraghagawane/luma/internal/domain"
	"github.com/anuraghagawane/luma/internal/infra/cache"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func getRedisCacheClient(t *testing.T) (domain.CacheClient, *redis.Client) {
	t.Helper()
	cfg, err := config.LoadEnv()
	if err != nil {
		log.Fatalf("Error while parsing env: %v", err)
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisHost,
		Password: cfg.RedisPassword, // no password
		DB:       0,                 // use default DB
		Protocol: 2,
	})

	t.Cleanup(func() { _ = rdb.Close() })
	return cache.NewCacheClient(rdb), rdb
}

func generateKeyID() string {
	return "test:apikey:" + uuid.NewString()
}

func TestCacheClientApply_StoreNewKey(t *testing.T) {
	ctx := context.Background()
	cacheClient, rdb := getRedisCacheClient(t)

	apiKeyID := generateKeyID()

	apiKey := domain.APIKeyRedisPayload{
		KeyID:    apiKeyID,
		TenantID: uuid.NewString(),
		Hash:     "afafafgasgagasfasfasdfad",
		Status:   domain.APIKeyActive,
		Version:  1,
	}

	outcome, err := cacheClient.Apply(ctx, apiKeyID, apiKey, 0)
	if err != nil {
		t.Fatalf("Error Apply(): %v", err)
	}

	t.Cleanup(func() {
		_, err := rdb.Del(ctx, apiKeyID).Result()
		if err != nil {
			t.Fatalf("failed to delete the key: %v", err)
		}
	})

	if outcome != domain.CacheApplyUpdated {
		t.Errorf("cache apply outcome: %v, want %v", outcome, domain.CacheApplyUpdated)
		return
	}

	cacheRes, err := cacheClient.GetAPIKey(ctx, apiKeyID)
	if err != nil {
		t.Fatalf("Error GetAPIKey(): %v", err)
	}

	if cacheRes.KeyID != apiKey.KeyID {
		t.Errorf("keyID: %v, want %v", cacheRes.KeyID, apiKey.KeyID)
		return
	}

	if cacheRes.TenantID != apiKey.TenantID {
		t.Errorf("tenantID: %v, want %v", cacheRes.TenantID, apiKey.TenantID)
		return
	}

	if cacheRes.Hash != apiKey.Hash {
		t.Errorf("Hash: %v, want %v", cacheRes.Hash, apiKey.Hash)
		return
	}

	if cacheRes.Status != apiKey.Status {
		t.Errorf("Status: %v, want %v", cacheRes.Status, apiKey.Status)
		return
	}

	if cacheRes.Version != apiKey.Version {
		t.Errorf("Version: %v, want %v", cacheRes.Version, apiKey.Version)
		return
	}
}

func TestCacheClientApply_SecondSameVersionIsUnchanged(t *testing.T) {
	ctx := context.Background()
	cacheClient, rdb := getRedisCacheClient(t)

	apiKeyID := generateKeyID()

	apiKey := domain.APIKeyRedisPayload{
		KeyID:    apiKeyID,
		TenantID: uuid.NewString(),
		Hash:     "afafafgasgagasfasfasdfad",
		Status:   domain.APIKeyActive,
		Version:  1,
	}

	outcome, err := cacheClient.Apply(ctx, apiKeyID, apiKey, 0)
	if err != nil {
		t.Fatalf("Error Apply(): %v", err)
	}

	t.Cleanup(func() {
		_, err := rdb.Del(ctx, apiKeyID).Result()
		if err != nil {
			t.Fatalf("failed to delete the key: %v", err)
		}
	})

	if outcome != domain.CacheApplyUpdated {
		t.Errorf("cache apply outcome: %v, want %v", outcome, domain.CacheApplyUpdated)
		return
	}

	outcome, err = cacheClient.Apply(ctx, apiKeyID, apiKey, 0)
	if err != nil {
		t.Fatalf("Error Apply(): %v", err)
	}

	if outcome != domain.CacheApplyUnchanged {
		t.Errorf("cache apply outcome: %v, want %v", outcome, domain.CacheApplyUnchanged)
		return
	}

	cacheRes, err := cacheClient.GetAPIKey(ctx, apiKeyID)
	if err != nil {
		t.Fatalf("Error GetAPIKey(): %v", err)
	}

	if cacheRes.KeyID != apiKey.KeyID {
		t.Errorf("keyID: %v, want %v", cacheRes.KeyID, apiKey.KeyID)
		return
	}

	if cacheRes.TenantID != apiKey.TenantID {
		t.Errorf("tenantID: %v, want %v", cacheRes.TenantID, apiKey.TenantID)
		return
	}

	if cacheRes.Hash != apiKey.Hash {
		t.Errorf("Hash: %v, want %v", cacheRes.Hash, apiKey.Hash)
		return
	}

	if cacheRes.Status != apiKey.Status {
		t.Errorf("Status: %v, want %v", cacheRes.Status, apiKey.Status)
		return
	}

	if cacheRes.Version != apiKey.Version {
		t.Errorf("Version: %v, want %v", cacheRes.Version, apiKey.Version)
		return
	}
}

func TestCacheClientApply_StaleVersionIsRejected(t *testing.T) {
	ctx := context.Background()
	cacheClient, rdb := getRedisCacheClient(t)

	apiKeyID := generateKeyID()

	apiKeyLatest := domain.APIKeyRedisPayload{
		KeyID:    apiKeyID,
		TenantID: uuid.NewString(),
		Hash:     "afafafgasgagasfasfasdfad",
		Status:   domain.APIKeyActive,
		Version:  2,
	}

	apiKeyStale := domain.APIKeyRedisPayload{
		KeyID:    apiKeyID,
		TenantID: uuid.NewString(),
		Hash:     "afafafgasgagasfasfasdfad",
		Status:   domain.APIKeyActive,
		Version:  1,
	}
	outcome, err := cacheClient.Apply(ctx, apiKeyID, apiKeyLatest, 0)
	if err != nil {
		t.Fatalf("Error Apply(): %v", err)
	}

	t.Cleanup(func() {
		_, err := rdb.Del(ctx, apiKeyID).Result()
		if err != nil {
			t.Fatalf("failed to delete the key: %v", err)
		}
	})

	if outcome != domain.CacheApplyUpdated {
		t.Errorf("cache apply outcome: %v, want %v", outcome, domain.CacheApplyUpdated)
		return
	}

	outcome, err = cacheClient.Apply(ctx, apiKeyID, apiKeyStale, 0)
	if err != nil {
		t.Fatalf("Error Apply(): %v", err)
	}

	if outcome != domain.CacheApplyStale {
		t.Errorf("cache apply outcome: %v, want %v", outcome, domain.CacheApplyStale)
		return
	}

	cacheRes, err := cacheClient.GetAPIKey(ctx, apiKeyID)
	if err != nil {
		t.Fatalf("Error GetAPIKey(): %v", err)
	}

	if cacheRes.KeyID != apiKeyLatest.KeyID {
		t.Errorf("keyID: %v, want %v", cacheRes.KeyID, apiKeyLatest.KeyID)
		return
	}

	if cacheRes.TenantID != apiKeyLatest.TenantID {
		t.Errorf("tenantID: %v, want %v", cacheRes.TenantID, apiKeyLatest.TenantID)
		return
	}

	if cacheRes.Hash != apiKeyLatest.Hash {
		t.Errorf("Hash: %v, want %v", cacheRes.Hash, apiKeyLatest.Hash)
		return
	}

	if cacheRes.Status != apiKeyLatest.Status {
		t.Errorf("Status: %v, want %v", cacheRes.Status, apiKeyLatest.Status)
		return
	}

	if cacheRes.Version != apiKeyLatest.Version {
		t.Errorf("Version: %v, want %v", cacheRes.Version, apiKeyLatest.Version)
		return
	}
}

func TestCacheClientGetAPIKey_NotFound(t *testing.T) {
	ctx := context.Background()
	cacheClient, _ := getRedisCacheClient(t)

	apiKeyID := generateKeyID()
	_, err := cacheClient.GetAPIKey(ctx, apiKeyID)
	if err == nil {
		t.Fatalf("GetAPIKey() should error")
	}

	if !errors.Is(err, domain.ErrKeyNotFound) {
		t.Fatalf("Error: %v, want %v", err, domain.ErrKeyNotFound)
	}
}

func TestCacheClientApply_ExpiresAfterTTL(t *testing.T) {
	ctx := context.Background()
	cacheClient, rdb := getRedisCacheClient(t)

	apiKeyID := generateKeyID()

	apiKey := domain.APIKeyRedisPayload{
		KeyID:    apiKeyID,
		TenantID: uuid.NewString(),
		Hash:     "afafafgasgagasfasfasdfad",
		Status:   domain.APIKeyActive,
		Version:  1,
	}

	outcome, err := cacheClient.Apply(ctx, apiKeyID, apiKey, 1*time.Second)
	if err != nil {
		t.Fatalf("Error Apply(): %v", err)
	}

	t.Cleanup(func() {
		_, err := rdb.Del(ctx, apiKeyID).Result()
		if err != nil {
			t.Fatalf("failed to delete the key: %v", err)
		}
	})

	if outcome != domain.CacheApplyUpdated {
		t.Errorf("cache apply outcome: %v, want %v", outcome, domain.CacheApplyUpdated)
		return
	}

	time.Sleep(1 * time.Second)

	_, err = cacheClient.GetAPIKey(ctx, apiKeyID)
	if err == nil {
		t.Fatalf("GetAPIKey() should error")
	}

	if !errors.Is(err, domain.ErrKeyNotFound) {
		t.Fatalf("Error: %v, want %v", err, domain.ErrKeyNotFound)
	}
}
