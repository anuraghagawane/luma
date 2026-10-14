// Package apikey consists for api related to api key management
package apikey

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/anuraghagawane/luma/internal/api/auth"
	"github.com/anuraghagawane/luma/internal/api/response"
	"github.com/anuraghagawane/luma/internal/domain"
	"github.com/google/uuid"
)

type APIKeyHander struct {
	apiKeyRepo domain.APIKeyRepository
}

func NewAPIKeyHandler(apikeyRepo domain.APIKeyRepository) *APIKeyHander {
	return &APIKeyHander{apiKeyRepo: apikeyRepo}
}

func (h *APIKeyHander) HandleAPIKeyGet(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := auth.TenantIDFromContext(r.Context())
	if !ok {
		response.Unauthorized(w, "Unauthorized")
		return
	}

	err := uuid.Validate(tenantID)
	if err != nil {
		response.BadRequest(w, "invalid tenantID")
		return
	}

	keys, err := h.apiKeyRepo.AllAPIKey(r.Context(), tenantID)
	if err != nil {
		response.InternalServerError(w, "failed to fetch api keys")
		return
	}

	response.OK(w, "", ResponseFromAPIKeys(keys))
}

func (h *APIKeyHander) HandleAPIKeyPost(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := auth.TenantIDFromContext(r.Context())
	if !ok {
		response.Unauthorized(w, "Unauthorized")
		return
	}
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		response.Unauthorized(w, "Unauthorized")
		return
	}

	data, err := io.ReadAll(r.Body)
	if err != nil {
		response.BadRequest(w, "Failed to read request body")
		return
	}

	var body APIKeyPostBody
	err = json.Unmarshal(data, &body)
	if err != nil {
		response.BadRequest(w, "Invalid Input")
		return
	}
	err = body.validate()
	if err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	apiKeyObj, err := generateKeyObj(body.Name, tenantID, userID)
	if err != nil {
		response.InternalServerError(w, "failed to create key")
		return
	}

	err = h.apiKeyRepo.CreateAPIKeyAndEvent(r.Context(), *apiKeyObj)
	if err != nil {
		response.InternalServerError(w, "failed to create the api key")
		return
	}

	response.Created(w, "", map[string]any{"name": apiKeyObj.Name, "api_key": apiKeyObj.Key})
}

func (h *APIKeyHander) HandleAPIKeyDelete(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := auth.TenantIDFromContext(r.Context())
	if !ok {
		response.Unauthorized(w, "Unauthorized")
		return
	}
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		response.Unauthorized(w, "Unauthorized")
		return
	}

	keyID := r.PathValue("id")

	err := validateKeyID(keyID)
	if err != nil {
		response.BadRequest(w, err.Error())
		return
	}

	err = h.apiKeyRepo.DeleteAPIKeyAndCreateEvent(r.Context(), keyID, tenantID, userID)
	if err != nil {
		if errors.Is(err, domain.ErrKeyAlreadyRevoked) {
			response.Conflict(w, "key already revoked")
			return
		}
		if errors.Is(err, domain.ErrKeyNotFound) {
			response.NotFound(w, "key not found")
			return
		}
		response.InternalServerError(w, "Failed to delete")
		return
	}

	response.OK(w, "success", "api key deleted")
}
