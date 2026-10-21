package collector

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anuraghagawane/luma/internal/domain"
)

type publishedLog struct {
	topic string
	key   []byte
	value []byte
}

type recordingLogProducer struct {
	messages []publishedLog
	errAt    int
	err      error
}

func (p *recordingLogProducer) Publish(_ context.Context, topic string, key []byte, value []byte) error {
	p.messages = append(p.messages, publishedLog{topic: topic, key: key, value: value})
	if p.errAt > 0 && len(p.messages) == p.errAt {
		return p.err
	}
	return nil
}

func newCollectorHandlerRequest(method, path, body, tenantID string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if tenantID != "" {
		req = req.WithContext(context.WithValue(req.Context(), tenantContextKey, tenantID))
	}
	return req
}

func TestHandleLog(t *testing.T) {
	const trustedTenant = "trusted-tenant"
	const payload = `{"eventid":"event-1","tenant":"client-tenant","service":"api","host":"host-1","loglevel":"ERROR","message":"request failed","timestamp":123}`

	tests := []struct {
		name           string
		method         string
		body           string
		tenantID       string
		producerErrAt  int
		wantStatus     int
		wantPublishNum int
	}{
		{
			name:           "success overwrites client tenant",
			method:         http.MethodPost,
			body:           payload,
			tenantID:       trustedTenant,
			wantStatus:     http.StatusOK,
			wantPublishNum: 1,
		},
		{
			name:       "invalid JSON",
			method:     http.MethodPost,
			body:       "{",
			tenantID:   trustedTenant,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing tenant context",
			method:     http.MethodPost,
			body:       payload,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:           "producer failure",
			method:         http.MethodPost,
			body:           payload,
			tenantID:       trustedTenant,
			producerErrAt:  1,
			wantStatus:     http.StatusInternalServerError,
			wantPublishNum: 1,
		},
		{
			name:       "unsupported method",
			method:     http.MethodGet,
			body:       payload,
			tenantID:   trustedTenant,
			wantStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			producer := &recordingLogProducer{errAt: test.producerErrAt, err: errors.New("publish failed")}
			handler := NewHandler(producer)
			req := newCollectorHandlerRequest(test.method, "/v1/log", test.body, test.tenantID)
			rec := httptest.NewRecorder()

			handler.HandleLog(rec, req)

			if rec.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", rec.Code, test.wantStatus, rec.Body.String())
			}
			if len(producer.messages) != test.wantPublishNum {
				t.Fatalf("publish calls = %d, want %d", len(producer.messages), test.wantPublishNum)
			}

			if test.name == "success overwrites client tenant" {
				assertPublishedLog(t, producer.messages[0], "event-1", trustedTenant)
			}
		})
	}
}

func TestHandleBulkLog(t *testing.T) {
	const trustedTenant = "trusted-tenant"
	const payload = `[{"eventid":"event-1","tenant":"client-tenant","service":"api","host":"host-1","loglevel":"ERROR","message":"first","timestamp":123},{"eventid":"event-2","tenant":"another-client-tenant","service":"worker","host":"host-2","loglevel":"DEBUG","message":"second","timestamp":456}]`

	tests := []struct {
		name           string
		method         string
		body           string
		tenantID       string
		producerErrAt  int
		wantStatus     int
		wantPublishNum int
		wantCount      int
	}{
		{
			name:           "success overwrites client tenants",
			method:         http.MethodPost,
			body:           payload,
			tenantID:       trustedTenant,
			wantStatus:     http.StatusOK,
			wantPublishNum: 2,
			wantCount:      2,
		},
		{
			name:       "empty batch",
			method:     http.MethodPost,
			body:       "[]",
			tenantID:   trustedTenant,
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid JSON",
			method:     http.MethodPost,
			body:       "{",
			tenantID:   trustedTenant,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing tenant context",
			method:     http.MethodPost,
			body:       payload,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:           "stops on producer failure",
			method:         http.MethodPost,
			body:           payload,
			tenantID:       trustedTenant,
			producerErrAt:  2,
			wantStatus:     http.StatusInternalServerError,
			wantPublishNum: 2,
		},
		{
			name:       "unsupported method",
			method:     http.MethodGet,
			body:       payload,
			tenantID:   trustedTenant,
			wantStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			producer := &recordingLogProducer{errAt: test.producerErrAt, err: errors.New("publish failed")}
			handler := NewHandler(producer)
			req := newCollectorHandlerRequest(test.method, "/v1/bulklog", test.body, test.tenantID)
			rec := httptest.NewRecorder()

			handler.HandleBulkLog(rec, req)

			if rec.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", rec.Code, test.wantStatus, rec.Body.String())
			}
			if len(producer.messages) != test.wantPublishNum {
				t.Fatalf("publish calls = %d, want %d", len(producer.messages), test.wantPublishNum)
			}

			if test.wantStatus == http.StatusOK {
				var response struct {
					Status string `json:"status"`
					Data   struct {
						Count int `json:"count"`
					} `json:"data"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				if response.Status != "success" {
					t.Errorf("response status = %q, want %q", response.Status, "success")
				}
				if response.Data.Count != test.wantCount {
					t.Errorf("response count = %d, want %d", response.Data.Count, test.wantCount)
				}
			}

			if test.name == "success overwrites client tenants" {
				assertPublishedLog(t, producer.messages[0], "event-1", trustedTenant)
				assertPublishedLog(t, producer.messages[1], "event-2", trustedTenant)
			}
		})
	}
}

func assertPublishedLog(t *testing.T, message publishedLog, wantEventID, wantTenant string) {
	t.Helper()

	if message.topic != "log" {
		t.Errorf("topic = %q, want %q", message.topic, "log")
	}
	if string(message.key) != wantEventID {
		t.Errorf("message key = %q, want %q", message.key, wantEventID)
	}

	var got domain.Log
	if err := json.Unmarshal(message.value, &got); err != nil {
		t.Fatalf("decode published log: %v", err)
	}
	if got.EventID != wantEventID {
		t.Errorf("event ID = %q, want %q", got.EventID, wantEventID)
	}
	if got.Tenant != wantTenant {
		t.Errorf("tenant = %q, want %q", got.Tenant, wantTenant)
	}
}
