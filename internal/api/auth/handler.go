// Package auth provides api for user management
package auth

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/anuraghagawane/luma/internal/domain"
)

type AuthHandler struct {
	userRepo     domain.UserRepository
	tokenManager *TokenManager
}

func NewHandler(userRepo domain.UserRepository, tokenManager *TokenManager) *AuthHandler {
	return &AuthHandler{userRepo, tokenManager}
}

func (h *AuthHandler) HandleCreateAccount(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		data, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}

		var body AccountCreateRequestBody
		err = json.Unmarshal(data, &body)
		if err != nil {
			http.Error(w, "Error: Invalid Input", http.StatusBadRequest)
			return
		}

		if err := body.Validate(); err != nil {
			http.Error(w, "Error: "+err.Error(), http.StatusBadRequest)
			return
		}

		passwordHash, err := hashPassword(body.Password)
		if err != nil {
			http.Error(w, "Error: Internal error", http.StatusInternalServerError)
			return
		}
		user := domain.User{Email: body.Email, PasswordHash: passwordHash, Status: domain.ACTIVE, Role: domain.ADMIN}
		tenant := domain.Tenant{Name: body.TenantName, Status: domain.TACTIVE}

		err = h.userRepo.CreateTenantAndUser(r.Context(), &user, &tenant)

		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "failure", "message": err.Error()})
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	default:
		http.Error(w, "method not supported", http.StatusMethodNotAllowed)
	}
}

func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		data, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}

		var body LoginRequestBody
		err = json.Unmarshal(data, &body)
		if err != nil {
			http.Error(w, "Error: Invalid Input", http.StatusBadRequest)
			return
		}

		if err := body.Validate(); err != nil {
			http.Error(w, "Error: "+err.Error(), http.StatusBadRequest)
			return
		}

		user, err := h.userRepo.FindUserWithEmail(r.Context(), body.Email)

		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "failure", "message": "Invalid email or password"})
			return
		}

		if err := comparePassword(user.PasswordHash, body.Password); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "failure", "message": "Invalid email or password"})
			return
		}

		token, err := h.tokenManager.IssueToken(user)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "failure", "message": "Internal server error"})
			return
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "success", "token": token})
	default:
		http.Error(w, "method not supported", http.StatusMethodNotAllowed)
	}
}
