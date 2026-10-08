package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anuraghagawane/luma/internal/domain"
)

type authRepo struct {
	called bool
	user   *domain.User
	tenant *domain.Tenant
	err    error
}

func (r *authRepo) CreateTenantAndUser(ctx context.Context, user *domain.User, tenant *domain.Tenant) error {
	r.called = true
	r.user = user
	r.tenant = tenant

	return r.err
}

func (r *authRepo) FindUserWithEmail(ctx context.Context, email string) (*domain.User, error) {
	return nil, nil
}

func TestHandleCreateAccountWithInvalidJSON(t *testing.T) {
	handler := AuthHandler{nil, nil}

	body := bytes.NewBufferString("{")
	req := httptest.NewRequest(http.MethodPost, "/v1/createaccount", body)
	rec := httptest.NewRecorder()

	handler.HandleCreateAccount(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateAccountWithInvalidInputs(t *testing.T) {
	handler := AuthHandler{nil, nil}

	tests := []struct {
		name           string
		body           AccountCreateRequestBody
		expectedStatus int
	}{
		{
			name:           "invalid email",
			body:           AccountCreateRequestBody{Email: "aafdfafa@fsadfaafafa", Password: "fadfadfadfaf!A2", TenantName: "afdfafadsfasfa"},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid password",
			body:           AccountCreateRequestBody{Email: "aafdfafa@fsadf.aafafa", Password: "fadfadfadfaf!A", TenantName: "afdfafadsfasfa"},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid tenant",
			body:           AccountCreateRequestBody{Email: "aafdfafa@fsadf.aafafa", Password: "fadfadfadfaf!A1", TenantName: "afdfafadsfasfa&"},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			jsonBody, err := json.Marshal(test.body)
			if err != nil {
				t.Fatal("Invalid test input")
			}
			body := bytes.NewBuffer(jsonBody)
			req := httptest.NewRequest(http.MethodPost, "/v1/createaccount", body)
			rec := httptest.NewRecorder()

			handler.HandleCreateAccount(rec, req)

			if rec.Code != test.expectedStatus {
				t.Errorf("status = %d, want %d, response: %v", rec.Code, test.expectedStatus, rec.Body.String())
			}
		})
	}
}

func TestHandleCreateAccountWithValidInput(t *testing.T) {
	authRepo := &authRepo{}
	handler := NewHandler(authRepo, nil)
	loginRequest := AccountCreateRequestBody{Email: "aafdfafa@fsadfaa.fafa", Password: "fadfadfadfaf!A2", TenantName: "afdfafadsfasfa"}
	jsonBody, err := json.Marshal(loginRequest)
	if err != nil {
		t.Fatal("Invalid test input")
	}
	body := bytes.NewBuffer(jsonBody)
	req := httptest.NewRequest(http.MethodPost, "/v1/createaccount", body)
	rec := httptest.NewRecorder()

	handler.HandleCreateAccount(rec, req)

	if !authRepo.called {
		t.Fatal("Auth repo not called")
	}

	if rec.Code != http.StatusCreated {
		t.Fatalf("status: %d, want %d", rec.Code, http.StatusCreated)
	}

	if authRepo.user.Email != loginRequest.Email {
		t.Fatalf("user email: %s, want %s", authRepo.user.Email, loginRequest.Email)
	}

	if authRepo.tenant.Name != loginRequest.TenantName {
		t.Fatalf("tenant name: %s, want %s", authRepo.tenant.Name, loginRequest.TenantName)
	}

	if authRepo.user.PasswordHash == loginRequest.Password {
		t.Fatal("password hash and provided password are same")
	}

	if comparePassword(authRepo.user.PasswordHash, loginRequest.Password) != nil {
		t.Fatal("password hash and provided password are not comparable")
	}

	if authRepo.user.Status != domain.ACTIVE {
		t.Fatalf("status: %v, want %v", authRepo.user.Status, domain.ACTIVE)
	}

	if authRepo.user.Role != domain.ADMIN {
		t.Fatalf("role: %v, want %v", authRepo.user.Role, domain.ADMIN)
	}
}

func TestHandleCreateAccountForRepositoryError(t *testing.T) {
	authRepo := &authRepo{
		err: errors.New("auth repo err"),
	}
	handler := NewHandler(authRepo, nil)

	loginRequest := AccountCreateRequestBody{Email: "aafdfafa@fsadfaa.fafa", Password: "fadfadfadfaf!A2", TenantName: "afdfafadsfasfa"}
	jsonBody, err := json.Marshal(loginRequest)
	if err != nil {
		t.Fatal("Invalid test input")
	}
	body := bytes.NewBuffer(jsonBody)
	req := httptest.NewRequest(http.MethodPost, "/v1/createaccount", body)
	rec := httptest.NewRecorder()

	handler.HandleCreateAccount(rec, req)

	if !authRepo.called {
		t.Fatal("Auth repo not called")
	}

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
