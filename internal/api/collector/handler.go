// Package api defines the handler for all apis
package collector

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/anuraghagawane/luma/internal/api/response"
	"github.com/anuraghagawane/luma/internal/domain"
)

type LogHandler struct {
	producer domain.LogProducer
}

func NewHandler(producer domain.LogProducer) *LogHandler {
	return &LogHandler{producer}
}

func (h *LogHandler) HandleLog(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	switch r.Method {
	case http.MethodPost:
		var log domain.Log
		data, err := io.ReadAll(r.Body)
		if err != nil {
			response.BadRequest(w, "Failed to read request boyd")
			return
		}

		err = json.Unmarshal(data, &log)
		if err != nil {
			response.BadRequest(w, "Invalid input")
			return
		}

		err = h.producer.Publish(r.Context(), "log", []byte(log.EventID), data)
		if err != nil {
			fmt.Printf("Error while publishing log: %v", err)
			response.InternalServerError(w, "failed")
		}
		fmt.Printf("%+v\n", log)
		response.OK(w, "", nil)
	default:
		response.MethodNotAllowed(w, "method not supported")
	}
}

func (h *LogHandler) HandleBulkLog(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	switch r.Method {
	case http.MethodPost:
		var logs []domain.Log
		data, err := io.ReadAll(r.Body)
		if err != nil {
			response.BadRequest(w, "Failed to read request boyd")
			return
		}
		err = json.Unmarshal(data, &logs)
		if err != nil {
			response.BadRequest(w, "Invalid input")
			return
		}

		for _, log := range logs {
			data, _ := json.Marshal(log)
			err = h.producer.Publish(r.Context(), "log", []byte(log.EventID), data)
			if err != nil {
				response.InternalServerError(w, "failed")
				return
			}
		}
		response.OK(w, "", map[string]any{"count": len(logs)})
	default:
		response.MethodNotAllowed(w, "method not supported")
	}
}
