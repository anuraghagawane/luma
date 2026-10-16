package apikey

import (
	"bytes"
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
	userID   string
	keyID    string
	apiKey   domain.APIKey
	keys     []domain.APIKey
}

func (r *apiKeyRepo) AllAPIKey(ctx context.Context, tenantID string) ([]domain.APIKey, error) {
	r.called = true
	r.tenantID = tenantID
	return r.keys, r.err
}

func (r *apiKeyRepo) CreateAPIKeyAndEvent(ctx context.Context, apiKeyObj domain.APIKey) error {
	r.called = true
	r.apiKey = apiKeyObj
	return r.err
}

func (r *apiKeyRepo) DeleteAPIKeyAndCreateEvent(ctx context.Context, keyID string, tenantID string, userID string) error {
	r.called = true
	r.tenantID = tenantID
	r.userID = userID
	r.keyID = keyID
	return r.err
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

type KeyCreateRequest struct {
	Name string `json:"name,omitempty"`
}

type APIKeyPostResponse struct {
	Status string `json:"status"`
	Data   struct {
		Name   string `json:"name"`
		APIKey string `json:"api_key"`
	} `json:"data"`
}

func TestHandleAPIKeyPost(t *testing.T) {
	tests := []struct {
		name        string
		tenantID    string
		userID      string
		status      int
		invalidJSON bool
		repoCalled  bool
		err         error
		reqBody     KeyCreateRequest
	}{
		{
			name:       "success",
			tenantID:   uuid.NewString(),
			userID:     uuid.NewString(),
			status:     http.StatusCreated,
			repoCalled: true,
			err:        nil,
			reqBody:    KeyCreateRequest{Name: "prodkey"},
		},
		{
			name:       "missing tenant context",
			tenantID:   "",
			userID:     uuid.NewString(),
			status:     http.StatusUnauthorized,
			repoCalled: false,
			err:        nil,
			reqBody:    KeyCreateRequest{Name: "prodkey"},
		},
		{
			name:       "missing user context",
			tenantID:   uuid.NewString(),
			userID:     "",
			status:     http.StatusUnauthorized,
			repoCalled: false,
			err:        nil,
			reqBody:    KeyCreateRequest{Name: "prodkey"},
		},
		{
			name:       "invalid request body",
			tenantID:   uuid.NewString(),
			userID:     uuid.NewString(),
			status:     http.StatusBadRequest,
			repoCalled: false,
			err:        nil,
			reqBody:    KeyCreateRequest{Name: ""},
		},
		{
			name:       "invalid key name",
			tenantID:   uuid.NewString(),
			userID:     uuid.NewString(),
			status:     http.StatusBadRequest,
			repoCalled: false,
			err:        nil,
			reqBody:    KeyCreateRequest{Name: "hflkashflahl;fhja;lkfja;lsjfa;ljf;alsjfoyhroiuyriouqywrqywprqywporyqwporyqwpoyrqpowyrpoqwyrpoqwyrpoqwyfophsdflkjahslkfjh"},
		},
		{
			name:       "repo error",
			tenantID:   uuid.NewString(),
			userID:     uuid.NewString(),
			status:     http.StatusInternalServerError,
			repoCalled: true,
			err:        errors.New("repo error"),
			reqBody:    KeyCreateRequest{Name: "prodkey"},
		},
		{
			name:        "invalid json",
			tenantID:    uuid.NewString(),
			userID:      uuid.NewString(),
			status:      http.StatusBadRequest,
			repoCalled:  false,
			invalidJSON: true,
			err:         nil,
			reqBody:     KeyCreateRequest{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			contextKeyTenantID := auth.ContextKey("tenantid")
			contextKeyUserID := auth.ContextKey("id")
			reqData, err := json.Marshal(test.reqBody)
			if err != nil {
				t.Fatalf("Error while marshalling test data: %v", err)
			}
			body := bytes.NewBuffer(reqData)
			if test.invalidJSON {
				body = bytes.NewBufferString("{fadf")
			}
			req := httptest.NewRequest(http.MethodPost, "/v1/api-key", body)

			if test.tenantID != "" {
				ctx := context.WithValue(req.Context(), contextKeyTenantID, test.tenantID)
				req = req.WithContext(ctx)
			}
			if test.userID != "" {
				ctx := context.WithValue(req.Context(), contextKeyUserID, test.userID)
				req = req.WithContext(ctx)
			}

			res := httptest.NewRecorder()

			repo := apiKeyRepo{
				err: test.err,
			}
			handler := NewAPIKeyHandler(&repo)

			handler.HandleAPIKeyPost(res, req)
			if res.Code != test.status {
				t.Fatalf("status: %v, want %v", res.Code, test.status)
			}

			if repo.called != test.repoCalled {
				t.Fatalf("repoCalled: %v, want %v", repo.called, test.repoCalled)
			}

			if repo.err != nil && res.Code != http.StatusInternalServerError {
				t.Fatalf("repo error status: %v, want %v", res.Code, http.StatusInternalServerError)
			}

			if test.repoCalled {
				if repo.apiKey.Name != test.reqBody.Name {
					t.Fatalf("Name: %v, want %v", repo.apiKey.Name, test.reqBody.Name)
				}

				if repo.apiKey.TenantID != test.tenantID {
					t.Fatalf("TenantID: %v, want %v", repo.apiKey.TenantID, test.tenantID)
				}

				if repo.apiKey.CreatedBy != test.userID {
					t.Fatalf("CreatedBy: %v, want %v", repo.apiKey.CreatedBy, test.userID)
				}

				if repo.apiKey.UpdatedBy != test.userID {
					t.Fatalf("UpdatedBy: %v, want %v", repo.apiKey.UpdatedBy, test.userID)
				}

			}

			if test.status == http.StatusInternalServerError {
				var resBody APIKeyPostResponse

				err := json.Unmarshal(res.Body.Bytes(), &resBody)
				if err != nil {
					t.Fatalf("failed to unmarshal the response")
				}

				if resBody.Status != "failure" {
					t.Fatalf(`Status: %v, want "failure"`, resBody.Status)
				}
			}

			if test.status == http.StatusCreated {
				var resBody APIKeyPostResponse

				err := json.Unmarshal(res.Body.Bytes(), &resBody)
				if err != nil {
					t.Fatalf("failed to unmarshal the response")
				}

				if resBody.Status != "success" {
					t.Fatalf(`Status: %v, want "success"`, resBody.Status)
				}

				if resBody.Data.Name != test.reqBody.Name {
					t.Fatalf("Name: %v, want %v", resBody.Data.Name, test.reqBody.Name)
				}

				if resBody.Data.APIKey == "" {
					t.Fatalf("API key not returned")
				}

				if repo.apiKey.Hash == "" {
					t.Fatalf("Hash is empty")
				}

				if resBody.Data.APIKey != repo.apiKey.Key {
					t.Fatalf("response apikey: %v, repo apikey: %v", resBody.Data.APIKey, repo.apiKey.Key)
				}
			}
		})
	}
}

func TestHandleAPIKeyDelete(t *testing.T) {
	tests := []struct {
		name       string
		tenantID   string
		userID     string
		keyID      string
		status     int
		repoCalled bool
		err        error
		message    string
	}{
		{
			name:       "success",
			tenantID:   uuid.NewString(),
			userID:     uuid.NewString(),
			keyID:      uuid.NewString(),
			status:     http.StatusOK,
			repoCalled: true,
			err:        nil,
		},
		{
			name:       "missing tenant context",
			tenantID:   "",
			userID:     uuid.NewString(),
			keyID:      uuid.NewString(),
			status:     http.StatusUnauthorized,
			repoCalled: false,
			err:        nil,
		},
		{
			name:       "missing user context",
			tenantID:   uuid.NewString(),
			userID:     "",
			keyID:      uuid.NewString(),
			status:     http.StatusUnauthorized,
			repoCalled: false,
			err:        nil,
		},
		{
			name:       "invalid api key",
			tenantID:   uuid.NewString(),
			userID:     uuid.NewString(),
			keyID:      "not-a-uuid",
			status:     http.StatusBadRequest,
			repoCalled: false,
			err:        nil,
		},
		{
			name:       "no api key",
			tenantID:   uuid.NewString(),
			userID:     uuid.NewString(),
			keyID:      "",
			status:     http.StatusBadRequest,
			repoCalled: false,
			err:        nil,
		},
		{
			name:       "key already revoked",
			tenantID:   uuid.NewString(),
			userID:     uuid.NewString(),
			keyID:      uuid.NewString(),
			status:     http.StatusConflict,
			repoCalled: true,
			err:        domain.ErrKeyAlreadyRevoked,
			message:    "key already revoked",
		},
		{
			name:       "key not found",
			tenantID:   uuid.NewString(),
			userID:     uuid.NewString(),
			keyID:      uuid.NewString(),
			status:     http.StatusNotFound,
			repoCalled: true,
			err:        domain.ErrKeyNotFound,
			message:    "key not found",
		},
		{
			name:       "repo internal error",
			tenantID:   uuid.NewString(),
			userID:     uuid.NewString(),
			keyID:      uuid.NewString(),
			status:     http.StatusInternalServerError,
			repoCalled: true,
			err:        errors.New("Failed to delete"),
			message:    "Failed to delete",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			contextKeyTenantID := auth.ContextKey("tenantid")
			contextKeyUserID := auth.ContextKey("id")
			req := httptest.NewRequest(http.MethodDelete, "/v1/api-key/{id}", nil)
			req.SetPathValue("id", test.keyID)

			if test.tenantID != "" {
				ctx := context.WithValue(req.Context(), contextKeyTenantID, test.tenantID)
				req = req.WithContext(ctx)
			}
			if test.userID != "" {
				ctx := context.WithValue(req.Context(), contextKeyUserID, test.userID)
				req = req.WithContext(ctx)
			}

			res := httptest.NewRecorder()

			repo := apiKeyRepo{
				err: test.err,
			}
			handler := NewAPIKeyHandler(&repo)

			handler.HandleAPIKeyDelete(res, req)
			if res.Code != test.status {
				t.Fatalf("status: %v, want %v", res.Code, test.status)
			}

			if repo.called != test.repoCalled {
				t.Fatalf("repoCalled: %v, want %v", repo.called, test.repoCalled)
			}

			if repo.err != nil {
				var resBody struct {
					Status  string `json:"status"`
					Message string `json:"message"`
				}

				err := json.Unmarshal(res.Body.Bytes(), &resBody)
				if err != nil {
					t.Fatalf("failed to unmarshal the response")
				}

				if resBody.Status != "failure" {
					t.Fatalf("status: %v, want %v", resBody.Message, "failure")
				}

				if errors.Is(test.err, domain.ErrKeyAlreadyRevoked) && resBody.Message != test.message {
					t.Fatalf("message: %v, want: %v", test.err.Error(), test.message)
				}

				if errors.Is(test.err, domain.ErrKeyNotFound) && resBody.Message != test.message {
					t.Fatalf("message: %v, want: %v", test.err.Error(), test.message)
				}

				if test.status == http.StatusInternalServerError && resBody.Message != test.message {
					t.Fatalf("message: %v, want: %v", test.err.Error(), test.message)
				}
			}

			if test.repoCalled {
				if repo.tenantID != test.tenantID {
					t.Fatalf("TenantID: %v, want %v", repo.tenantID, test.tenantID)
				}

				if repo.userID != test.userID {
					t.Fatalf("userID: %v, want %v", repo.userID, test.userID)
				}

				if repo.keyID != test.keyID {
					t.Fatalf("keyID: %v, want %v", repo.keyID, test.keyID)
				}
			}

			if test.status == http.StatusOK {
				var resBody struct {
					Status string `json:"status"`
				}

				err := json.Unmarshal(res.Body.Bytes(), &resBody)
				if err != nil {
					t.Fatalf("failed to unmarshal the response")
				}

				if resBody.Status != "success" {
					t.Fatalf(`Status: %v, want "success"`, resBody.Status)
				}
			}
		})
	}
}
