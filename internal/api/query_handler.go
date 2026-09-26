package api

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/anuraghagawane/luma/internal/domain"
	"github.com/anuraghagawane/luma/internal/repository/elastic"
)

type QueryHandler struct {
	logRepo *elastic.LogRepo
}

func NewQueryHandler(logRepo *elastic.LogRepo) *QueryHandler {
	return &QueryHandler{
		logRepo: logRepo,
	}
}

func (h *QueryHandler) HandleLogQuery(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	switch r.Method {
	case http.MethodGet:
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

		logs, err := h.logRepo.Query(r.Context(), logQuery)
		if err != nil {
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
