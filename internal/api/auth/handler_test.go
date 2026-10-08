package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
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

func TestHandleCreateAccountWithInvalidMethod(t *testing.T) {
	authRepo := &authRepo{
		err: errors.New("auth repo err"),
	}
	handler := NewHandler(authRepo, nil)

	tests := []struct {
		name         string
		reqMethod    string
		expectedCode int
	}{
		{
			name:         "get",
			reqMethod:    http.MethodGet,
			expectedCode: http.StatusMethodNotAllowed,
		},
		{
			name:         "put",
			reqMethod:    http.MethodPut,
			expectedCode: http.StatusMethodNotAllowed,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(test.reqMethod, "/v1/createaccount", nil)
			rec := httptest.NewRecorder()

			handler.HandleCreateAccount(rec, req)

			if rec.Code != test.expectedCode {
				t.Fatalf("status: %d, want %d", rec.Code, test.expectedCode)
			}
		})
	}
}

// Login tests
type loginAuthRepo struct {
	called bool
	email  string
	user   *domain.User
	err    error
}

func (r *loginAuthRepo) CreateTenantAndUser(ctx context.Context, user *domain.User, tenant *domain.Tenant) error {
	return nil
}

func (r *loginAuthRepo) FindUserWithEmail(ctx context.Context, email string) (*domain.User, error) {
	r.called = true
	r.email = email
	return r.user, r.err
}

type LoginResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Token   string `json:"token"`
}

func TestHandleLoginWithValidUser(t *testing.T) {
	tokenManager, err := NewTokenManager("afadfadfadaiouyqrioyholiufhaskljfhaofyqpoyrqpoweyufpoiqwdufoashjflakjdflk", 1)
	if err != nil {
		t.Fatalf("error initiating token manager: %v", err)
	}

	email := "abc@abc.abc"
	password := "affafad1A@"
	passwordHash, err := hashPassword(password)
	if err != nil {
		t.Fatal("failed to hash password")
	}

	user := &domain.User{
		Email:        email,
		PasswordHash: passwordHash,
		ID:           "123",
		TenantID:     "t123",
		Status:       domain.ACTIVE,
		Role:         domain.ADMIN,
	}

	authRepo := loginAuthRepo{
		user: user,
	}

	handler := NewHandler(&authRepo, tokenManager)

	requestBody := LoginRequestBody{
		Email:    email,
		Password: password,
	}
	requestBodyJSON, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatal("failed to marshal request body")
	}
	body := bytes.NewBuffer(requestBodyJSON)

	req := httptest.NewRequest(http.MethodPost, "/v1/login", body)
	rec := httptest.NewRecorder()

	handler.HandleLogin(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d, want %d", rec.Code, http.StatusOK)
	}

	if !authRepo.called {
		t.Fatal("Auth repo never called")
	}

	if authRepo.email != email {
		t.Fatalf("email: %v, want: %v", authRepo.email, email)
	}

	if rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("Content-Type: %v, want %v", rec.Header().Get("Content-Type"), "application/json")
	}

	resBody, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatal("failed to read response body")
	}

	var response LoginResponse

	err = json.Unmarshal(resBody, &response)
	if err != nil {
		t.Fatal("failed to unmarshal the response")
	}

	if response.Status != "success" {
		t.Fatalf("response status: %v, want %v", response.Status, "success")
	}

	claims, err := tokenManager.ParseToken(response.Token)
	if err != nil {
		t.Fatal("Failed to parse the issued token")
	}

	if claims.Email != email {
		t.Fatalf("email: %v, want: %v", claims.Email, email)
	}

	if claims.ID != user.ID {
		t.Fatalf("ID: %v, want: %v", claims.ID, user.ID)
	}

	if claims.TenantID != user.TenantID {
		t.Fatalf("TenantID: %v, want: %v", claims.TenantID, user.TenantID)
	}

	if claims.Role != string(user.Role) {
		t.Fatalf("Role: %v, want: %v", claims.Role, user.Role)
	}
}

func TestHandleLoginWithInvalidPassword(t *testing.T) {
	tokenManager, err := NewTokenManager("afadfadfadaiouyqrioyholiufhaskljfhaofyqpoyrqpoweyufpoiqwdufoashjflakjdflk", 1)
	if err != nil {
		t.Fatalf("error initiating token manager: %v", err)
	}

	email := "abc@abc.abc"
	realPassword := "affafad1A@ffafas"
	password := "affafad1A@"
	passwordHash, err := hashPassword(password)
	if err != nil {
		t.Fatal("failed to hash password")
	}

	user := &domain.User{
		Email:        email,
		PasswordHash: passwordHash,
		ID:           "123",
		TenantID:     "t123",
		Status:       domain.ACTIVE,
		Role:         domain.ADMIN,
	}

	authRepo := loginAuthRepo{
		user: user,
	}

	handler := NewHandler(&authRepo, tokenManager)

	requestBody := LoginRequestBody{
		Email:    email,
		Password: realPassword,
	}
	requestBodyJSON, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatal("failed to marshal request body")
	}
	body := bytes.NewBuffer(requestBodyJSON)

	req := httptest.NewRequest(http.MethodPost, "/v1/login", body)
	rec := httptest.NewRecorder()

	handler.HandleLogin(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status: %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	if !authRepo.called {
		t.Fatal("Auth repo never called")
	}

	if authRepo.email != email {
		t.Fatalf("email: %v, want: %v", authRepo.email, email)
	}

	if rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("Content-Type: %v, want %v", rec.Header().Get("Content-Type"), "application/json")
	}

	resBody, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatal("failed to read response body")
	}

	var response LoginResponse

	err = json.Unmarshal(resBody, &response)
	if err != nil {
		t.Fatal("failed to unmarshal the response")
	}

	if response.Status != "failure" {
		t.Fatalf("response status: %v, want %v", response.Status, "failure")
	}

	if response.Token != "" {
		t.Fatal("issued token")
	}
}

func TestHandleLoginWithInvalidUser(t *testing.T) {
	tokenManager, err := NewTokenManager("afadfadfadaiouyqrioyholiufhaskljfhaofyqpoyrqpoweyufpoiqwdufoashjflakjdflk", 1)
	if err != nil {
		t.Fatalf("error initiating token manager: %v", err)
	}

	email := "abc@abc.abc"
	password := "affafad1A@ffafas"

	authRepo := loginAuthRepo{
		err: errors.New("Invalid email or password"),
	}

	handler := NewHandler(&authRepo, tokenManager)

	requestBody := LoginRequestBody{
		Email:    email,
		Password: password,
	}
	requestBodyJSON, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatal("failed to marshal request body")
	}
	body := bytes.NewBuffer(requestBodyJSON)

	req := httptest.NewRequest(http.MethodPost, "/v1/login", body)
	rec := httptest.NewRecorder()

	handler.HandleLogin(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status: %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	if !authRepo.called {
		t.Fatal("Auth repo never called")
	}

	if authRepo.email != email {
		t.Fatalf("email: %v, want: %v", authRepo.email, email)
	}

	if rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("Content-Type: %v, want %v", rec.Header().Get("Content-Type"), "application/json")
	}

	resBody, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatal("failed to read response body")
	}

	var response LoginResponse

	err = json.Unmarshal(resBody, &response)
	if err != nil {
		t.Fatal("failed to unmarshal the response")
	}

	if response.Status != "failure" {
		t.Fatalf("response status: %v, want %v", response.Status, "failure")
	}

	if response.Token != "" {
		t.Fatal("issued token")
	}
}

func TestHandleLoginWithInvalidJSON(t *testing.T) {
	authRepo := loginAuthRepo{}

	handler := NewHandler(&authRepo, nil)

	body := bytes.NewBufferString("{")

	req := httptest.NewRequest(http.MethodPost, "/v1/login", body)
	rec := httptest.NewRecorder()

	handler.HandleLogin(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: %d, want %d", rec.Code, http.StatusBadRequest)
	}

	if authRepo.called {
		t.Fatal("Auth repo called")
	}
}

func TestHandleLoginWithInvalidMethod(t *testing.T) {
	authRepo := loginAuthRepo{}

	handler := NewHandler(&authRepo, nil)
	tests := []struct {
		name         string
		reqMethod    string
		expectedCode int
	}{
		{
			name:         "get",
			reqMethod:    http.MethodGet,
			expectedCode: http.StatusMethodNotAllowed,
		},
		{
			name:         "put",
			reqMethod:    http.MethodPut,
			expectedCode: http.StatusMethodNotAllowed,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(test.reqMethod, "/v1/login", nil)
			rec := httptest.NewRecorder()

			handler.HandleLogin(rec, req)

			if rec.Code != test.expectedCode {
				t.Fatalf("status: %d, want %d", rec.Code, test.expectedCode)
			}
		})
	}
}
