package collector

import (
	"context"
	"errors"
	"net/http"

	"github.com/anuraghagawane/luma/internal/api/response"
	"github.com/anuraghagawane/luma/internal/domain"
	"github.com/anuraghagawane/luma/internal/security/apikey"
)

type APIKeyAuthenticator struct {
	cacheClient domain.CollectorCacheClient
}

func NewAPIKeyAuthenticator(cacheClient domain.CollectorCacheClient) *APIKeyAuthenticator {
	return &APIKeyAuthenticator{cacheClient: cacheClient}
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

			apiKeyCachePayload, err := m.cacheClient.GetAPIKey(r.Context(), "api_key:"+keyObj.ID)
			if err != nil {
				if errors.Is(err, domain.ErrKeyNotFound) {
					response.Unauthorized(w, "invalid api key")
					return
				}
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
