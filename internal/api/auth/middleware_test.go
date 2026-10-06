package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anuraghagawane/luma/internal/domain"
)

func TestAuthMiddleware(t *testing.T) {
	tokenManager, err := NewTokenManager("fasfadfasfasdfasddfasdfasfasfasfasdfasdfasdfasdfasdfasdf", 2)
	if err != nil {
		t.Errorf("error in NewTokenManager(), error: %v", err)
		return
	}
	user := domain.User{
		ID:           "12345",
		TenantID:     "12345",
		Email:        "abc@abc.abc",
		PasswordHash: "afdsfasdfa",
		Role:         domain.ADMIN,
		Status:       domain.ACTIVE,
	}

	token, err := tokenManager.IssueToken(&user)
	if err != nil {
		t.Errorf("Error in IssueToken(): %v", err)
		return
	}

	var nextCalled bool
	var gotTenantID, gotUserID string
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		gotTenantID, _ = TenantIDFromContext(r.Context())
		gotUserID, _ = r.Context().Value(userIDKey).(string)
	})

	middleware := AuthMiddleware(tokenManager)(nextHandler)

	tests := []struct {
		name             string
		authorization    string
		expectedCode     int
		expectNext       bool
		expectedTenantID string
		expectedUserID   string
	}{
		{
			name:             "valid bearer token",
			authorization:    "Bearer " + token,
			expectedCode:     http.StatusOK,
			expectNext:       true,
			expectedTenantID: user.TenantID,
			expectedUserID:   user.ID,
		},
		{
			name:         "missing authorization",
			expectedCode: http.StatusUnauthorized,
		},
		{
			name:          "basic auth",
			authorization: "Basic abc",
			expectedCode:  http.StatusUnauthorized,
		},
		{
			name:          "invalid format",
			authorization: "Bearer",
			expectedCode:  http.StatusUnauthorized,
		},
		{
			name:          "invalid token",
			authorization: "Bearer abcdef",
			expectedCode:  http.StatusUnauthorized,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			nextCalled = false
			gotTenantID = ""
			gotUserID = ""

			req := httptest.NewRequest(http.MethodGet, "/v1/logs", nil)
			if test.authorization != "" {
				req.Header.Set("Authorization", test.authorization)
			}

			res := httptest.NewRecorder()
			middleware.ServeHTTP(res, req)

			if res.Code != test.expectedCode {
				t.Errorf("AuthMiddleware(), code: %v, expected code: %v", res.Code, test.expectedCode)
			}
			if nextCalled != test.expectNext {
				t.Errorf("next handler called: %v, expected %v", nextCalled, test.expectNext)
			}
			if gotTenantID != test.expectedTenantID {
				t.Errorf("tenant ID: %q, expected %q", gotTenantID, test.expectedTenantID)
			}
			if gotUserID != test.expectedUserID {
				t.Errorf("user ID: %q, expected %q", gotUserID, test.expectedUserID)
			}
		})
	}
}
