// Package auth provides api for user management
package auth

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/anuraghagawane/luma/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	dbClient *pgxpool.Pool
	userRepo domain.UserRepository
}

func NewHandler(dbClient *pgxpool.Pool, userRepo domain.UserRepository) *AuthHandler {
	return &AuthHandler{dbClient, userRepo}
}

type AccountCreateRequestBody struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	TenantName string `json:"tenantname"`
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
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "failure"})
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	default:
		http.Error(w, "method not supported", http.StatusMethodNotAllowed)
	}
}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}
