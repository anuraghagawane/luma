// Package response provides centralized response generator for http apis
package response

import (
	"encoding/json"
	"net/http"
)

type JSONResponse struct {
	Status  string `json:"status,omitempty"`
	Message string `json:"message,omitempty"`
	Token   string `json:"token,omitempty"`
	Data    any    `json:"data,omitempty"`
}

func WriteResponse(w http.ResponseWriter, statusCode int, payload JSONResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

func OK(w http.ResponseWriter, message string, data any) {
	WriteResponse(w, http.StatusOK, JSONResponse{
		Status:  "success",
		Message: message,
		Data:    data,
	})
}

func Created(w http.ResponseWriter) {
	WriteResponse(w, http.StatusCreated, JSONResponse{
		Status: "success",
	})
}

func Unauthorized(w http.ResponseWriter, message string) {
	WriteResponse(w, http.StatusUnauthorized, JSONResponse{
		Status:  "failure",
		Message: message,
	})
}

func InternalServerError(w http.ResponseWriter, message string) {
	WriteResponse(w, http.StatusInternalServerError, JSONResponse{
		Status:  "failure",
		Message: message,
	})
}

func BadRequest(w http.ResponseWriter, message string) {
	WriteResponse(w, http.StatusBadRequest, JSONResponse{
		Status:  "failure",
		Message: message,
	})
}

func MethodNotAllowed(w http.ResponseWriter, message string) {
	WriteResponse(w, http.StatusMethodNotAllowed, JSONResponse{
		Status:  "failure",
		Message: message,
	})
}

func GatewayTimeout(w http.ResponseWriter, message string) {
	WriteResponse(w, http.StatusGatewayTimeout, JSONResponse{
		Status:  "failure",
		Message: message,
	})
}
