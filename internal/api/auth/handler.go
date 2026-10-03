// Package auth provides api for user management
package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"unicode"

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

func validateEmail(email string) error {
	_, err := mail.ParseAddress(email)
	return err
}

func validatePassword(password string) error {
	var (
		hasMinLen  = len(password) >= 8
		hasMaxLen  = len(password) <= 64
		hasUpper   bool
		hasLower   bool
		hasNumber  bool
		hasSpecial bool
	)

	if !hasMinLen || !hasMaxLen {
		return errors.New("password must be between 8 and 64 characters long")
	}

	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsNumber(char):
			hasNumber = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	if !hasUpper {
		return errors.New("password must contain atleast one upper case character")
	}
	if !hasLower {
		return errors.New("password must contain atleast one lower case character")
	}
	if !hasNumber {
		return errors.New("password must contain atleast one digit character")
	}
	if !hasSpecial {
		return errors.New("password must contain atleast one special character(punctuation or symbol")
	}

	return nil
}

func validateTenantName(tenantName string) error {
	var (
		hasMinLen               = len(tenantName) >= 5
		hasMaxLen               = len(tenantName) <= 20
		hasNumber               bool
		hasSpecialNotUnderScore bool
	)

	if !hasMinLen || !hasMaxLen {
		return errors.New("tenant name must be between 5 and 20 characters long")
	}

	for _, char := range tenantName {
		switch {
		case unicode.IsNumber(char):
			hasNumber = true
		case (unicode.IsPunct(char) || unicode.IsSymbol(char)) && char != '_':
			hasSpecialNotUnderScore = true
		}
	}

	if hasNumber {
		return errors.New("tenant name should not contain the digit/number")
	}

	if hasSpecialNotUnderScore {
		return errors.New("tenant name should not contain special characters apart from underscore \"_\" ")
	}

	return nil
}

func (b AccountCreateRequestBody) Validate() error {
	if err := validateEmail(b.Email); err != nil {
		return fmt.Errorf("invalid email")
	}

	if err := validatePassword(b.Password); err != nil {
		return fmt.Errorf("invalid password: %w", err)
	}

	if err := validateTenantName(b.TenantName); err != nil {
		return fmt.Errorf("invalid tenant name: %w", err)
	}

	return nil
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
