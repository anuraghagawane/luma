package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

type contextKey string

const (
	tenantIDKey contextKey = "tenantid"
	userIDKey   contextKey = "id"
)

func TenantIDFromContext(ctx context.Context) (string, bool) {
	ctxValue := ctx.Value(tenantIDKey)
	id, ok := ctxValue.(string)
	return id, ok
}

func AuthMiddleware(m *TokenManager) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, err := bearerToken(r)
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			claims, err := m.ParseToken(token)
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), tenantIDKey, claims.TenantID)
			ctx = context.WithValue(ctx, userIDKey, claims.ID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func bearerToken(r *http.Request) (string, error) {
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", errors.New("invalid authorization header")
	}
	return parts[1], nil
}
