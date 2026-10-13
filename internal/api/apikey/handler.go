// Package apikey consists for api related to api key management
package apikey

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/anuraghagawane/luma/internal/api/auth"
	"github.com/anuraghagawane/luma/internal/api/response"
	"github.com/anuraghagawane/luma/internal/domain"
)

type APIKeyHander struct {
	apiKeyRepo domain.APIKeyRepository
}

func NewAPIKeyHandler(apikeyRepo domain.APIKeyRepository) *APIKeyHander {
	return &APIKeyHander{apiKeyRepo: apikeyRepo}
}

func (h *APIKeyHander) HandleAPIKey(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.handleAPIKeyPost(w, r)
		return
	case http.MethodDelete:
		h.handleAPIKeyDelete(w, r)
		return
	default:
		response.MethodNotAllowed(w, "method not allowed")
	}
}

func (h *APIKeyHander) handleAPIKeyPost(w http.ResponseWriter, r *http.Request) {
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

func (h *APIKeyHander) handleAPIKeyDelete(w http.ResponseWriter, r *http.Request) {
	// TODO:
	fmt.Println("handling delete")
	response.InternalServerError(w, "Not implemented")
}
