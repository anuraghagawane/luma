package apikey

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anuraghagawane/luma/internal/api/auth"
	"github.com/anuraghagawane/luma/internal/domain"
	"github.com/google/uuid"
)

type apiKeyRepo struct {
	called   bool
	err      error
	tenantID string
	keys     []domain.APIKey
}

func (r *apiKeyRepo) AllAPIKey(ctx context.Context, tenantID string) ([]domain.APIKey, error) {
	r.called = true
	r.tenantID = tenantID
	return r.keys, r.err
}

func (r *apiKeyRepo) CreateAPIKeyAndEvent(ctx context.Context, apiKeyObj domain.APIKey) error {
	return nil
}

func (r *apiKeyRepo) DeleteAPIKeyAndCreateEvent(ctx context.Context, keyID string, tenantID string, userID string) error {
	return nil
}

func TestHandleAPIKeyGet(t *testing.T) {
	tests := []struct {
		name       string
		tenantID   string
		status     int
		repoCalled bool
		err        error
		keys       []domain.APIKey
		withAuth   bool
	}{
		{
			name:       "success",
			tenantID:   uuid.NewString(),
			withAuth:   true,
			status:     http.StatusOK,
			repoCalled: true,
			err:        nil,
			keys: []domain.APIKey{
				{
					KeyID:    uuid.NewString(),
					TenantID: uuid.NewString(),
					Name:     "key",
					Status:   domain.APIKeyActive,
					Version:  1,
				},
			},
		},
		{
			name:       "no keys",
			tenantID:   uuid.NewString(),
			withAuth:   true,
			status:     http.StatusOK,
			repoCalled: true,
			err:        nil,
			keys:       []domain.APIKey{},
		},
		{
			name:       "invalid tenantID",
			tenantID:   "not-a-uuid",
			withAuth:   true,
			status:     http.StatusBadRequest,
			repoCalled: false,
			err:        nil,
			keys:       nil,
		},
		{
			name:       "missing context",
			tenantID:   "t1",
			status:     http.StatusUnauthorized,
			repoCalled: false,
			err:        nil,
			keys:       nil,
			withAuth:   false,
		},
		{
			name:       "repo error",
			tenantID:   uuid.NewString(),
			withAuth:   true,
			status:     http.StatusInternalServerError,
			repoCalled: true,
			err:        errors.New("repo failure"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			contextKey := auth.ContextKey("tenantid")
			req := httptest.NewRequest(http.MethodGet, "/v1/api-key", nil)

			if test.withAuth {
				ctx := context.WithValue(req.Context(), contextKey, test.tenantID)
				req = req.WithContext(ctx)
			}
			res := httptest.NewRecorder()

			repo := apiKeyRepo{
				keys: test.keys,
				err:  test.err,
			}
			handler := NewAPIKeyHandler(&repo)

			handler.HandleAPIKeyGet(res, req)

			if res.Code != test.status {
				t.Fatalf("status: %v, want %v", res.Code, test.status)
			}

			if repo.called != test.repoCalled {
				t.Fatalf("repoCalled: %v, want %v", repo.called, test.repoCalled)
			}

			if test.repoCalled && repo.tenantID != test.tenantID {
				t.Fatalf("repo tenantID: %v, want %v", repo.tenantID, test.tenantID)
			}

			if repo.err != nil && res.Code != http.StatusInternalServerError {
				t.Fatalf("repo error status: %v, want %v", res.Code, http.StatusInternalServerError)
			}

			if test.status == http.StatusInternalServerError {
				var resBody struct {
					Status string                 `json:"status,omitempty"`
					Data   []APIKeyGetResponseObj `json:"data,omitempty"`
				}

				err := json.Unmarshal(res.Body.Bytes(), &resBody)
				if err != nil {
					t.Fatalf("failed to unmarshal the response")
				}

				if resBody.Status != "failure" {
					t.Fatalf(`Status: %v, want "failure"`, resBody.Status)
				}
			}

			if test.status == http.StatusOK {
				var resBody struct {
					Status string                 `json:"status,omitempty"`
					Data   []APIKeyGetResponseObj `json:"data,omitempty"`
				}

				err := json.Unmarshal(res.Body.Bytes(), &resBody)
				if err != nil {
					t.Fatalf("failed to unmarshal the response")
				}

				if resBody.Status != "success" {
					t.Fatalf(`Status: %v, want "success"`, resBody.Status)
				}

				keys := resBody.Data
				if len(keys) != len(test.keys) {
					t.Fatalf("keys length: %v, want %v", len(keys), len(test.keys))
				}

				if len(keys) > 0 && keys[0].Name != test.keys[0].Name {
					t.Fatalf("key name = %s, want %s", keys[0].Name, test.keys[0].Name)
				}
			}
		})
	}
}
