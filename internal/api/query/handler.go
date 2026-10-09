// Package query provides api handlers for quering logs
package query

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/anuraghagawane/luma/internal/api/auth"
	"github.com/anuraghagawane/luma/internal/api/response"
	"github.com/anuraghagawane/luma/internal/domain"
)

type QueryHandler struct {
	logRepo      domain.QueryRepository
	queryTimeout time.Duration
}

func NewHandler(logRepo domain.QueryRepository, queryTimeout time.Duration) *QueryHandler {
	return &QueryHandler{
		logRepo:      logRepo,
		queryTimeout: queryTimeout,
	}
}

func (h *QueryHandler) HandleLogQuery(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	switch r.Method {
	case http.MethodGet:
		tenantID, ok := auth.TenantIDFromContext(r.Context())
		if !ok {
			response.Unauthorized(w, "Unauthorized")
			return
		}
		data, err := io.ReadAll(r.Body)
		if err != nil {
			response.BadRequest(w, "Failed to read request body")
			return
		}
		var logQuery domain.LogQuery
		err = json.Unmarshal(data, &logQuery)
		if err != nil {
			response.BadRequest(w, "Invalid Input")
			return
		}

		logQuery.Tenant = tenantID
		if err := logQuery.ValidateAndDefault(); err != nil {
			response.BadRequest(w, err.Error())
			return
		}

		queryCtx, cancel := context.WithTimeout(r.Context(), h.queryTimeout)
		defer cancel()

		logs, err := h.logRepo.Query(queryCtx, logQuery)
		if err != nil {
			if errors.Is(queryCtx.Err(), context.DeadlineExceeded) {
				response.GatewayTimeout(w, "Query timed out")
				return
			}
			log.Printf("Failed to Query: %v", err)
			response.InternalServerError(w, "Query failed")
			return
		}

		response.OK(w, "", logs)
	default:
		response.MethodNotAllowed(w, "method not supported")
		return
	}
}
