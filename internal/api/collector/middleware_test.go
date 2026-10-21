package collector

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/anuraghagawane/luma/internal/domain"
	"github.com/anuraghagawane/luma/internal/security/apikey"
)

type cacheClient struct {
	applyCalled  bool
	getCalled    bool
	payload      *domain.APIKeyRedisPayload
	getErr       error
	applyErr     error
	getKey       string
	applyKey     string
	applyPayload domain.APIKeyRedisPayload
	applyTTL     time.Duration
}

func (c *cacheClient) Apply(ctx context.Context, key string, payload domain.APIKeyRedisPayload, ttl time.Duration) (domain.CacheApplyOutcome, error) {
	c.applyCalled = true
	c.applyKey = key
	c.applyPayload = payload
	c.applyTTL = ttl
	return domain.CacheApplyUpdated, c.applyErr
}

func (c *cacheClient) GetAPIKey(ctx context.Context, key string) (*domain.APIKeyRedisPayload, error) {
	c.getCalled = true
	c.getKey = key
	return c.payload, c.getErr
}

type apiKeyRepository struct {
	called  bool
	payload *domain.APIKeyRedisPayload
	err     error
	keyID   string
}

func (r *apiKeyRepository) GetAPIKey(ctx context.Context, keyID string) (*domain.APIKeyRedisPayload, error) {
	r.called = true
	r.keyID = keyID
	return r.payload, r.err
}

func TestAPIKeyMiddleware(t *testing.T) {
	tests := []struct {
		name                string
		omitAuthorization   bool
		malformedKey        bool
		cacheErr            error
		repoErr             error
		cacheStatus         domain.APIKeyStatus
		cacheHashMismatch   bool
		applyErr            error
		wantStatus          int
		wantCacheGet        bool
		wantCacheApply      bool
		wantRepo            bool
		wantNext            bool
		wantTenant          string
		wantApplyTTL        time.Duration
		wantInvalidCacheKey bool
	}{
		{
			name:         "cache hit",
			wantStatus:   http.StatusNoContent,
			wantCacheGet: true,
			wantNext:     true,
			wantTenant:   "tenant-1",
		},
		{
			name:           "database fallback",
			cacheErr:       domain.ErrKeyNotFound,
			applyErr:       errors.New("redis write failed"),
			wantStatus:     http.StatusNoContent,
			wantCacheGet:   true,
			wantCacheApply: true,
			wantRepo:       true,
			wantNext:       true,
			wantTenant:     "tenant-1",
		},
		{
			name:                "unknown key is negatively cached",
			cacheErr:            domain.ErrKeyNotFound,
			repoErr:             domain.ErrKeyNotFound,
			wantStatus:          http.StatusUnauthorized,
			wantCacheGet:        true,
			wantCacheApply:      true,
			wantRepo:            true,
			wantApplyTTL:        30 * time.Second,
			wantInvalidCacheKey: true,
		},
		{
			name:              "hash mismatch",
			cacheHashMismatch: true,
			wantStatus:        http.StatusUnauthorized,
			wantCacheGet:      true,
		},
		{
			name:         "revoked key",
			cacheStatus:  domain.APIKeyRevoked,
			wantStatus:   http.StatusUnauthorized,
			wantCacheGet: true,
		},
		{
			name:         "cache backend error",
			cacheErr:     errors.New("redis unavailable"),
			wantStatus:   http.StatusInternalServerError,
			wantCacheGet: true,
		},
		{
			name:         "database backend error",
			cacheErr:     domain.ErrKeyNotFound,
			repoErr:      errors.New("postgres unavailable"),
			wantStatus:   http.StatusInternalServerError,
			wantCacheGet: true,
			wantRepo:     true,
		},
		{
			name:              "missing authorization",
			omitAuthorization: true,
			wantStatus:        http.StatusUnauthorized,
		},
		{
			name:         "malformed key",
			malformedKey: true,
			wantStatus:   http.StatusUnauthorized,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			key, err := apikey.GenerateAPIKey("test", "tenant-1", "user-1")
			if err != nil {
				t.Fatal(err)
			}
			payload := domain.GenerateKeyRedisPayload(*key)
			if test.cacheStatus != "" {
				payload.Status = test.cacheStatus
			}
			if test.cacheHashMismatch {
				payload.Hash = "different-hash"
			}
			cache := &cacheClient{payload: &payload, getErr: test.cacheErr, applyErr: test.applyErr}
			repo := &apiKeyRepository{
				payload: new(domain.GenerateKeyRedisPayload(*key)),
				err:     test.repoErr,
			}

			authenticator := NewAPIKeyAuthenticator(cache, repo)

			handlerCalled := false
			gotTenant := ""
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
				gotTenant, _ = tenantIDFromContext(r.Context())
				w.WriteHeader(http.StatusNoContent)
			})

			middleware := APIKeyMiddleware(authenticator)(handler)

			req := httptest.NewRequest(http.MethodPost, "/v1/log", nil)
			if !test.omitAuthorization {
				authorization := key.Key
				if test.malformedKey {
					authorization = "not-a-luma-key"
				}
				req.Header.Set("Authorization", authorization)
			}
			res := httptest.NewRecorder()

			middleware.ServeHTTP(res, req)
			if res.Code != test.wantStatus {
				t.Errorf("status = %d, want %d", res.Code, test.wantStatus)
			}
			if handlerCalled != test.wantNext {
				t.Errorf("next called = %v, want %v", handlerCalled, test.wantNext)
			}
			if gotTenant != test.wantTenant {
				t.Errorf("tenant = %q, want %q", gotTenant, test.wantTenant)
			}
			if cache.getCalled != test.wantCacheGet {
				t.Errorf("cache get called = %v, want %v", cache.getCalled, test.wantCacheGet)
			}
			if cache.applyCalled != test.wantCacheApply {
				t.Errorf("cache apply called = %v, want %v", cache.applyCalled, test.wantCacheApply)
			}
			if repo.called != test.wantRepo {
				t.Errorf("repository called = %v, want %v", repo.called, test.wantRepo)
			}
			if cache.getCalled && cache.getKey != "api_key:"+key.KeyID {
				t.Errorf("cache key = %q, want %q", cache.getKey, "api_key:"+key.KeyID)
			}
			if repo.called && repo.keyID != key.KeyID {
				t.Errorf("repository key ID = %q, want %q", repo.keyID, key.KeyID)
			}
			if cache.applyCalled {
				if cache.applyTTL != test.wantApplyTTL {
					t.Errorf("cache apply TTL = %s, want %s", cache.applyTTL, test.wantApplyTTL)
				}
				if cache.applyKey != "api_key:"+key.KeyID {
					t.Errorf("cache apply key = %q, want %q", cache.applyKey, "api_key:"+key.KeyID)
				}
				if test.wantInvalidCacheKey && cache.applyPayload.Status != domain.APIKeyInvalid {
					t.Errorf("negative cache status = %q, want %q", cache.applyPayload.Status, domain.APIKeyInvalid)
				}
			}
		})
	}
}
