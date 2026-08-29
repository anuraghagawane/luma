// Package api defines the handler for all apis
package api

import (
	"context"
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
		data, _ := io.ReadAll(r.Body)
		err := json.Unmarshal(data, &log)
		if err != nil {
			http.Error(w, "Error: Invalid input", http.StatusBadRequest)
			return
		}

		ctx := context.Background()

		err = h.producer.Publish(ctx, "log", []byte(log.EventID), data)
		if err != nil {
			fmt.Printf("Error while publishing log: %v", err)
			http.Error(w, "Error: failed", http.StatusInternalServerError)
		}
		fmt.Printf("%+v\n", log)
		_, _ = fmt.Fprintf(w, "received data")
	default:
		http.Error(w, "method not supported", http.StatusMethodNotAllowed)
	}
}

func (h *LogHandler) HandleBulkLog(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	switch r.Method {
	case http.MethodPost:
		var logs []domain.Log
		data, _ := io.ReadAll(r.Body)
		err := json.Unmarshal(data, &logs)
		if err != nil {
			http.Error(w, "Error: Invalid input", http.StatusBadRequest)
			return
		}

		ctx := context.Background()
		for _, log := range logs {
			data, _ := json.Marshal(log)
			err = h.producer.Publish(ctx, "log", []byte(log.EventID), data)
			if err != nil {
				http.Error(w, "Error: failed", http.StatusInternalServerError)
			}

		}
		fmt.Printf("%+v\n", logs)
		_, _ = fmt.Fprintf(w, "received data")
	default:
		http.Error(w, "method not supported", http.StatusMethodNotAllowed)
	}
}
