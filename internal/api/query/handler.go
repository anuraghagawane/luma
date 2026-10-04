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
	"github.com/anuraghagawane/luma/internal/domain"
	"github.com/anuraghagawane/luma/internal/repository/elastic"
)

type QueryHandler struct {
	logRepo      *elastic.LogRepo
	queryTimeout time.Duration
}

func NewHandler(logRepo *elastic.LogRepo) *QueryHandler {
	queryTimeout, err := time.ParseDuration("1m")
	if err != nil {
		log.Fatalf("failed to initialize QueryHandler: %v", err)
		return nil
	}
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
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		data, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}
		var logQuery domain.LogQuery
		err = json.Unmarshal(data, &logQuery)
		if err != nil {
			http.Error(w, "Error: Invalid Input", http.StatusBadRequest)
			return
		}

		logQuery.Tenant = tenantID
		if err := logQuery.ValidateAndDefault(); err != nil {
			http.Error(w, "Error: "+err.Error(), http.StatusBadRequest)
			return
		}

		queryCtx, cancel := context.WithTimeout(r.Context(), h.queryTimeout)
		defer cancel()

		logs, err := h.logRepo.Query(queryCtx, logQuery)
		if err != nil {
			if errors.Is(queryCtx.Err(), context.DeadlineExceeded) {
				http.Error(w, "Error: Query timed out", http.StatusGatewayTimeout)
				return
			}
			log.Printf("Failed to Query: %v", err)
			http.Error(w, "Error: Query Failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		marshalledLog, err := json.Marshal(logs)
		if err != nil {
			log.Printf("Failed to marshall log: %v", err)
			http.Error(w, "Error: failed to build response", http.StatusInternalServerError)
			return
		}
		_, err = w.Write(marshalledLog)
		if err != nil {
			log.Printf("Failed write response: %v", err)
			http.Error(w, "Error: failed to respond", http.StatusInternalServerError)
			return
		}
	default:
		http.Error(w, "method not suported", http.StatusMethodNotAllowed)
		return
	}
}
