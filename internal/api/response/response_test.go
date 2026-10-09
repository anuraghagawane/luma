package response

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteResponse(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteResponse(rec, http.StatusOK, JSONResponse{
		Status: "success",
	})

	if rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("Content-Type: %v, want: application/json", rec.Header().Get("Content-Type"))
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("Status Code: %d, want: %d", rec.Code, http.StatusOK)
	}

	data, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatal("Failed to read response body")
	}

	var response JSONResponse
	err = json.Unmarshal(data, &response)
	if err != nil {
		t.Fatal("Failed to Unmarshal response")
	}

	if response.Status != "success" {
		t.Fatalf("Status: %v, want: %v", response.Status, "success")
	}
}

func TestOK(t *testing.T) {
	rec := httptest.NewRecorder()
	message := "login success"
	OK(rec, message, "abc")

	if rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("Content-Type: %v, want: application/json", rec.Header().Get("Content-Type"))
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("Status Code: %d, want: %d", rec.Code, http.StatusOK)
	}

	data, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatal("Failed to read response body")
	}

	var response JSONResponse
	err = json.Unmarshal(data, &response)
	if err != nil {
		t.Fatal("Failed to Unmarshal response")
	}

	if response.Status != "success" {
		t.Fatalf("Status: %v, want: %v", response.Status, "success")
	}

	if response.Message != message {
		t.Fatalf("Message: %v, want: %v", response.Message, message)
	}

	if response.Data != "abc" {
		t.Fatalf("Data: %v, want: %v", response.Data, "abc")
	}
}

func TestCreated(t *testing.T) {
	rec := httptest.NewRecorder()
	Created(rec)

	if rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("Content-Type: %v, want: application/json", rec.Header().Get("Content-Type"))
	}

	if rec.Code != http.StatusCreated {
		t.Fatalf("Status Code: %d, want: %d", rec.Code, http.StatusCreated)
	}

	data, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatal("Failed to read response body")
	}

	var response JSONResponse
	err = json.Unmarshal(data, &response)
	if err != nil {
		t.Fatal("Failed to Unmarshal response")
	}

	if response.Status != "success" {
		t.Fatalf("Status: %v, want: %v", response.Status, "success")
	}

	if response.Data != nil {
		t.Fatalf("data: %v, want: nil", response.Data)
	}
}

func TestFailureCase(t *testing.T) {
	tests := []struct {
		name    string
		message string
		fn      func(w http.ResponseWriter, message string)
		status  int
	}{
		{
			name:    "unauthorized",
			message: "abc",
			fn:      Unauthorized,
			status:  http.StatusUnauthorized,
		},
		{
			name:    "InternalServerError",
			message: "abc",
			fn:      InternalServerError,
			status:  http.StatusInternalServerError,
		},
		{
			name:    "BadRequest",
			message: "abc",
			fn:      BadRequest,
			status:  http.StatusBadRequest,
		},
		{
			name:    "MethodNotAllowed",
			message: "abc",
			fn:      MethodNotAllowed,
			status:  http.StatusMethodNotAllowed,
		},
		{
			name:    "GatewayTimeout",
			message: "abc",
			fn:      GatewayTimeout,
			status:  http.StatusGatewayTimeout,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			test.fn(rec, test.message)

			if rec.Header().Get("Content-Type") != "application/json" {
				t.Fatalf("Content-Type: %v, want: application/json", rec.Header().Get("Content-Type"))
			}

			if rec.Code != test.status {
				t.Fatalf("Status Code: %d, want: %d", rec.Code, test.status)
			}

			data, err := io.ReadAll(rec.Body)
			if err != nil {
				t.Fatal("Failed to read response body")
			}

			var response JSONResponse
			err = json.Unmarshal(data, &response)
			if err != nil {
				t.Fatal("Failed to Unmarshal response")
			}

			if response.Status != "failure" {
				t.Fatalf("Status: %v, want: %v", response.Status, "failure")
			}

			if response.Message != test.message {
				t.Fatalf("Message: %v, want: %v", response.Message, test.message)
			}

			if response.Data != nil {
				t.Fatalf("data: %v, want: nil", response.Data)
			}
		})
	}
}
