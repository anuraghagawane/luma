// Package auth provides api for user management
package auth

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/anuraghagawane/luma/internal/api/response"
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
			response.BadRequest(w, "Failed to read request body")
			return
		}

		var body AccountCreateRequestBody
		err = json.Unmarshal(data, &body)
		if err != nil {
			response.BadRequest(w, "Invalid Input")
			return
		}

		if err := body.Validate(); err != nil {
			response.BadRequest(w, err.Error())
			return
		}

		passwordHash, err := hashPassword(body.Password)
		if err != nil {
			response.InternalServerError(w, "Internal error")
			return
		}
		user := domain.User{Email: body.Email, PasswordHash: passwordHash, Status: domain.ACTIVE, Role: domain.ADMIN}
		tenant := domain.Tenant{Name: body.TenantName, Status: domain.TACTIVE}

		err = h.userRepo.CreateTenantAndUser(r.Context(), &user, &tenant)
		if err != nil {
			response.BadRequest(w, err.Error())
			return
		}
		response.Created(w)
	default:
		response.MethodNotAllowed(w, "method not supported")
	}
}

func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		data, err := io.ReadAll(r.Body)
		if err != nil {
			response.BadRequest(w, "Failed to read request body")
			return
		}

		var body LoginRequestBody
		err = json.Unmarshal(data, &body)
		if err != nil {
			response.BadRequest(w, "Invalid Input")
			return
		}

		if err := body.Validate(); err != nil {
			response.BadRequest(w, err.Error())
			return
		}

		user, err := h.userRepo.FindUserWithEmail(r.Context(), body.Email)
		if err != nil {
			response.Unauthorized(w, "Invalid email or password")
			return
		}

		if err := comparePassword(user.PasswordHash, body.Password); err != nil {
			response.Unauthorized(w, "Invalid email or password")
			return
		}

		token, err := h.tokenManager.IssueToken(user)
		if err != nil {
			response.InternalServerError(w, "Internal server error")
			return
		}

		response.OK(w, "", LoginData{token})
	default:
		response.MethodNotAllowed(w, "method not supported")
	}
}
