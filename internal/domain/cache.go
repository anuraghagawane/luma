package domain

import (
	"context"
	"time"
)

type CacheApplyOutcome int

const (
	CacheApplyStale     CacheApplyOutcome = -1
	CacheApplyUnchanged CacheApplyOutcome = 0
	CacheApplyUpdated   CacheApplyOutcome = 1
)

type CacheClient interface {
	Apply(ctx context.Context, key string, payload APIKeyRedisPayload, ttl time.Duration) (CacheApplyOutcome, error)
	GetAPIKey(ctx context.Context, key string) (*APIKeyRedisPayload, error)
}
