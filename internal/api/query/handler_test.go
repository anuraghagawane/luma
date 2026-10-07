package query

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/anuraghagawane/luma/internal/api/auth"
	"github.com/anuraghagawane/luma/internal/domain"
)

type mockRepo struct {
	shouldErr bool
}

func (r *mockRepo) Query(ctx context.Context, logQuery domain.LogQuery) (*domain.QueryResponse, error) {
	if r.shouldErr {
		return nil, errors.New("Error: failed to query")
	}

	return nil, nil
}

type recordingRepo struct {
	response *domain.QueryResponse
	err      error
	called   bool
	query    domain.LogQuery
	ctx      context.Context
}

func (r *recordingRepo) Query(ctx context.Context, logQuery domain.LogQuery) (*domain.QueryResponse, error) {
	r.called = true
	r.ctx = ctx
	r.query = logQuery
	return r.response, r.err
}

func TestHandleLogQuerySuccess(t *testing.T) {
	cursor := "next-cursor"

	repo := &recordingRepo{
		response: &domain.QueryResponse{
			Cursor: &cursor,
			Logs: []domain.Log{
				{
					EventID:  "event-1",
					Tenant:   "tenant_abc",
					Service:  "api",
					Host:     "server-1",
					LogLevel: domain.DEBUG,
					Message:  "success",
				},
			},
		},
	}

	handler := NewHandler(repo, time.Minute)

	query := domain.LogQuery{
		From:  time.Now().Add(-time.Hour).Unix(),
		To:    time.Now().Unix(),
		Limit: 500,
	}

	body := bytes.NewBuffer(getJSONForQuery(query))

	req := httptest.NewRequest(http.MethodGet, "/v1/logs", body)

	ctx := context.WithValue(req.Context(), auth.ContextKey("tenantid"), "tenant_abc")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.HandleLogQuery(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf(
			"Content-Type = %q, want application/json",
			rec.Header().Get("Content-Type"),
		)
	}

	if !repo.called {
		t.Fatal("repository was not called")
	}

	if repo.query.Tenant != "tenant_abc" {
		t.Errorf(
			"tenant = %q, want %q",
			repo.query.Tenant,
			"tenant_abc",
		)
	}

	var response domain.QueryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(response.Logs) != 1 {
		t.Fatalf("logs = %d, want 1", len(response.Logs))
	}

	if response.Logs[0].EventID != "event-1" {
		t.Errorf(
			"event ID = %q, want %q",
			response.Logs[0].EventID,
			"event-1",
		)
	}

	if *response.Cursor != cursor {
		t.Errorf("cursor = %q, want %q", *response.Cursor, cursor)
	}
}

func TestHandleLogQueryWithoutTenantContext(t *testing.T) {
	logRepo := &mockRepo{}
	queryHandler := NewHandler(logRepo, 1)

	req := httptest.NewRequest(http.MethodGet, "/v1/logs", nil)
	rec := httptest.NewRecorder()

	queryHandler.HandleLogQuery(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Response code: %v, expected: %v", rec.Code, http.StatusUnauthorized)
	}
}

func TestHandleLogQueryWithInvalidJSON(t *testing.T) {
	logRepo := &mockRepo{}
	queryHandler := NewHandler(logRepo, 1)
	body := bytes.NewBufferString("afasfad")
	req := httptest.NewRequest(http.MethodGet, "/v1/logs", body)
	ctx := context.WithValue(req.Context(), auth.ContextKey("tenantid"), "tenant_abc")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()

	queryHandler.HandleLogQuery(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Response code: %v, expected: %v", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleLogQueryWithInvalidQueryValue(t *testing.T) {
	logRepo := &mockRepo{}
	queryHandler := NewHandler(logRepo, 1)

	tests := []struct {
		name         string
		query        domain.LogQuery
		expectedCode int
	}{
		{
			name: "valid",
			query: domain.LogQuery{
				From:  time.Now().Add(-1 * time.Hour).Unix(),
				To:    time.Now().Unix() - 10,
				Limit: 500,
			},
			expectedCode: http.StatusOK,
		},
		{
			name: "before 30 days query",
			query: domain.LogQuery{
				From:  time.Now().Add(-40 * 24 * time.Hour).Unix(),
				To:    time.Now().Unix() - 10,
				Limit: 500,
			},
			expectedCode: http.StatusBadRequest,
		},
		{
			name: "fromtime after totime",
			query: domain.LogQuery{
				From:  time.Now().Add(-40 * time.Hour).Unix(),
				To:    time.Now().Add(-60 * time.Hour).Unix(),
				Limit: 500,
			},
			expectedCode: http.StatusBadRequest,
		},
		{
			name: "limit exceed",
			query: domain.LogQuery{
				From:  time.Now().Add(-40 * time.Hour).Unix(),
				To:    time.Now().Unix() - 10,
				Limit: 1500,
			},
			expectedCode: http.StatusBadRequest,
		},
		{
			name: "future totime",
			query: domain.LogQuery{
				From:  time.Now().Add(-40 * time.Hour).Unix(),
				To:    time.Now().Add(1 * time.Hour).Unix(),
				Limit: 500,
			},
			expectedCode: http.StatusBadRequest,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := bytes.NewBuffer(getJSONForQuery(test.query))
			req := httptest.NewRequest(http.MethodGet, "/v1/logs", body)
			ctx := context.WithValue(req.Context(), auth.ContextKey("tenantid"), "tenant_abc")
			req = req.WithContext(ctx)

			rec := httptest.NewRecorder()
			queryHandler.HandleLogQuery(rec, req)

			if rec.Code != test.expectedCode {
				t.Errorf("Response code: %v, expected: %v", rec.Code, test.expectedCode)
			}
		})
	}
}

func getJSONForQuery(query domain.LogQuery) []byte {
	marshalledJSON, _ := json.Marshal(query)
	return marshalledJSON
}

func TestHandleLogQueryWithRepositoryFailure(t *testing.T) {
	logRepo := &mockRepo{shouldErr: true}
	queryHandler := NewHandler(logRepo, 1*time.Minute)
	query := domain.LogQuery{
		From:  time.Now().Add(-1 * time.Hour).Unix(),
		To:    time.Now().Unix() - 10,
		Limit: 500,
	}

	body := bytes.NewBuffer(getJSONForQuery(query))
	req := httptest.NewRequest(http.MethodGet, "/v1/logs", body)
	ctx := context.WithValue(req.Context(), auth.ContextKey("tenantid"), "tenant_abc")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()

	queryHandler.HandleLogQuery(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Response code: %v, expected: %v", rec.Code, http.StatusInternalServerError)
	}
}

type blockingRepo struct{}

func (r *blockingRepo) Query(ctx context.Context, logQuery domain.LogQuery) (*domain.QueryResponse, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestHandleLogQueryForQueryTimeout(t *testing.T) {
	logRepo := &blockingRepo{}
	queryHandler := NewHandler(logRepo, 10*time.Millisecond)
	query := domain.LogQuery{
		From:  time.Now().Add(-1 * time.Hour).Unix(),
		To:    time.Now().Unix() - 10,
		Limit: 500,
	}

	body := bytes.NewBuffer(getJSONForQuery(query))
	req := httptest.NewRequest(http.MethodGet, "/v1/logs", body)
	ctx := context.WithValue(req.Context(), auth.ContextKey("tenantid"), "tenant_abc")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()

	queryHandler.HandleLogQuery(rec, req)

	if rec.Code != http.StatusGatewayTimeout {
		t.Errorf("Response code: %v, expected: %v", rec.Code, http.StatusGatewayTimeout)
	}
}

func TestHandleLogQueryUnmatchedMethods(t *testing.T) {
	logRepo := &mockRepo{}
	queryHandler := NewHandler(logRepo, 10*time.Second)
	query := domain.LogQuery{
		From:  time.Now().Add(-1 * time.Hour).Unix(),
		To:    time.Now().Unix() - 10,
		Limit: 500,
	}
	body := bytes.NewBuffer(getJSONForQuery(query))

	tests := []struct {
		name         string
		reqMethod    string
		expectedCode int
	}{
		{
			name:         "post",
			reqMethod:    http.MethodPost,
			expectedCode: http.StatusMethodNotAllowed,
		},
		{
			name:         "put",
			reqMethod:    http.MethodPut,
			expectedCode: http.StatusMethodNotAllowed,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(test.reqMethod, "/v1/logs", body)
			rec := httptest.NewRecorder()
			queryHandler.HandleLogQuery(rec, req)
			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("Response code: %v, expected: %v", rec.Code, http.StatusMethodNotAllowed)
			}
		})
	}
}
