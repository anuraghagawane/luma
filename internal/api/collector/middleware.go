package collector

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/anuraghagawane/luma/internal/api/response"
	"github.com/anuraghagawane/luma/internal/domain"
	"github.com/anuraghagawane/luma/internal/security/apikey"
)

type APIKeyAuthenticator struct {
	cacheClient domain.CacheClient
	apiKeyRepo  domain.CollectorAPIKeyRepository
}

func NewAPIKeyAuthenticator(cacheClient domain.CacheClient, apiKeyRepo domain.CollectorAPIKeyRepository) *APIKeyAuthenticator {
	return &APIKeyAuthenticator{cacheClient: cacheClient, apiKeyRepo: apiKeyRepo}
}

type contextKey string

var tenantContextKey contextKey = "tenant_id"

func tenantIDFromContext(ctx context.Context) (string, bool) {
	ctxValue := ctx.Value(tenantContextKey)
	id, ok := ctxValue.(string)
	return id, ok
}

func APIKeyMiddleware(m *APIKeyAuthenticator) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key, err := apiKeyFromRequest(r)
			if err != nil {
				response.Unauthorized(w, "invalid api key")
				return
			}

			keyObj, err := apikey.ValidateAndParseAPIKey(key)
			if err != nil {
				response.Unauthorized(w, "invalid api key")
				return
			}

			apiKeyCachePayload, err := apiKeyPayloadFromKeyID(r.Context(), m, keyObj.ID)
			if err != nil {
				response.InternalServerError(w, "internal error")
				return
			}

			if apiKeyCachePayload.Hash != keyObj.Hash {
				response.Unauthorized(w, "invalid api key")
				return
			}

			if apiKeyCachePayload.Status != domain.APIKeyActive {
				response.Unauthorized(w, "invalid api key")
				return
			}

			ctx := context.WithValue(r.Context(), tenantContextKey, apiKeyCachePayload.TenantID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func apiKeyFromRequest(r *http.Request) (string, error) {
	key := r.Header.Get("Authorization")
	if key == "" {
		return "", errors.New("no api key")
	}

	return key, nil
}

func apiKeyPayloadFromKeyID(ctx context.Context, m *APIKeyAuthenticator, keyID string) (*domain.APIKeyRedisPayload, error) {
	cacheKey := "api_key:" + keyID
	apiKeyCachePayload, err := m.cacheClient.GetAPIKey(ctx, cacheKey)
	if err != nil {
		if errors.Is(err, domain.ErrKeyNotFound) {
			apiKeyCachePayload, err = m.apiKeyRepo.GetAPIKey(ctx, keyID)
			if err != nil {
				if errors.Is(err, domain.ErrKeyNotFound) {
					// ignoring the error here as the key is present in db but caching failed which will be automatically cached on future request
					apiKeyCachePayload = &domain.APIKeyRedisPayload{KeyID: keyID, Status: domain.APIKeyInvalid}
					_, _ = m.cacheClient.Apply(ctx, cacheKey, *apiKeyCachePayload, 30*time.Second)
					return apiKeyCachePayload, nil
				}

				return nil, fmt.Errorf("apiKeyPayloadFromKeyID: %w", err)
			}

			// cache the key
			// ignoring the error here as the key is present in db but caching failed which will be automatically cached on future request
			_, _ = m.cacheClient.Apply(ctx, cacheKey, *apiKeyCachePayload, 0)
			return apiKeyCachePayload, nil
		}
		return nil, fmt.Errorf("apiKeyPayloadFromKeyID: %w", err)
	}
	return apiKeyCachePayload, nil
}
