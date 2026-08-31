// Package api defines the handler for all apis
package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/anuraghagawane/luma/internal/domain"
)

type LogHandler struct {
	producer domain.LogProducer
}

func NewLogHandler(producer domain.LogProducer) *LogHandler {
	return &LogHandler{producer}
}

func (h *LogHandler) HandleLog(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	switch r.Method {
	case http.MethodPost:
		var log domain.Log
		data, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}

		err = json.Unmarshal(data, &log)
		if err != nil {
			http.Error(w, "Error: Invalid input", http.StatusBadRequest)
			return
		}

		err = h.producer.Publish(r.Context(), "log", []byte(log.EventID), data)
		if err != nil {
			fmt.Printf("Error while publishing log: %v", err)
			http.Error(w, "Error: failed", http.StatusInternalServerError)
		}
		fmt.Printf("%+v\n", log)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	default:
		http.Error(w, "method not supported", http.StatusMethodNotAllowed)
	}
}

func (h *LogHandler) HandleBulkLog(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	switch r.Method {
	case http.MethodPost:
		var logs []domain.Log
		data, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}
		err = json.Unmarshal(data, &logs)
		if err != nil {
			http.Error(w, "Error: Invalid input", http.StatusBadRequest)
			return
		}

		for _, log := range logs {
			data, _ := json.Marshal(log)
			err = h.producer.Publish(r.Context(), "log", []byte(log.EventID), data)
			if err != nil {
				http.Error(w, "Error: failed", http.StatusInternalServerError)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"status": "success", "count": len(logs)})
	default:
		http.Error(w, "method not supported", http.StatusMethodNotAllowed)
	}
}
