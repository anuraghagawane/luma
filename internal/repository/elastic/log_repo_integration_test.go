//go:build integration

package elastic_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/anuraghagawane/luma/internal/domain"
	"github.com/anuraghagawane/luma/internal/repository/elastic"
)

func TestLogRepoQueryByTenantAndTimeRange(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	ctx := context.Background()

	indexName := fmt.Sprintf("log_it_%d", time.Now().UnixNano())

	repo, err := elastic.NewLogRepo([]string{"http://localhost:9200"}, indexName)
	if err != nil {
		t.Fatalf("NewLogRepo() error = %v", err)
	}

	defer cleanupIndex(t, repo, indexName)

	now := time.Now().Unix()

	logA1 := domain.Log{
		EventID:   "event-1",
		Tenant:    "tenant-a",
		Service:   "api",
		Host:      "host-1",
		LogLevel:  domain.ERROR,
		Message:   "first error",
		Timestamp: now,
	}

	logA2 := domain.Log{
		EventID:   "event-2",
		Tenant:    "tenant-a",
		Service:   "worker",
		Host:      "host-2",
		LogLevel:  domain.DEBUG,
		Message:   "second debug",
		Timestamp: now + 5,
	}

	logB1 := domain.Log{
		EventID:   "event-3",
		Tenant:    "tenant-b",
		Service:   "api",
		Host:      "host-3",
		LogLevel:  domain.ERROR,
		Message:   "other tenant error",
		Timestamp: now + 10,
	}

	for _, doc := range []domain.Log{logA1, logA2, logB1} {
		if err := repo.Index(ctx, doc.EventID, doc); err != nil {
			t.Fatalf("Index() error for %s: %v", doc.EventID, err)
		}
	}

	query := domain.LogQuery{
		Tenant:   "tenant-a",
		From:     now - 60,
		To:       now + 100,
		Limit:    10,
		LogLevel: domain.ERROR,
	}

	result := waitForLogs(t, repo, query, 1)

	if len(result.Logs) != 1 {
		t.Fatalf("len(result.Logs) = %d, want 1", len(result.Logs))
	}

	if result.Logs[0].Tenant != "tenant-a" {
		t.Fatalf("tenant = %q, want %q", result.Logs[0].Tenant, "tenant-a")
	}

	if result.Logs[0].EventID != "event-1" {
		t.Fatalf("eventID = %q, want %q", result.Logs[0].EventID, "event-1")
	}
}

func TestLogRepoQueryByService(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	ctx := context.Background()

	indexName := fmt.Sprintf("log_it_%d", time.Now().UnixNano())

	repo, err := elastic.NewLogRepo([]string{"http://localhost:9200"}, indexName)
	if err != nil {
		t.Fatalf("NewLogRepo() error = %v", err)
	}

	defer cleanupIndex(t, repo, indexName)

	now := time.Now().Unix()

	log1 := domain.Log{
		EventID:   "event-1",
		Tenant:    "tenant-a",
		Service:   "api",
		Host:      "host-1",
		LogLevel:  domain.ERROR,
		Message:   "first error",
		Timestamp: now,
	}

	log2 := domain.Log{
		EventID:   "event-2",
		Tenant:    "tenant-a",
		Service:   "worker",
		Host:      "host-2",
		LogLevel:  domain.DEBUG,
		Message:   "second debug",
		Timestamp: now + 5,
	}

	log3 := domain.Log{
		EventID:   "event-3",
		Tenant:    "tenant-a",
		Service:   "api",
		Host:      "host-3",
		LogLevel:  domain.ERROR,
		Message:   "other tenant error",
		Timestamp: now + 10,
	}

	for _, doc := range []domain.Log{log1, log2, log3} {
		if err := repo.Index(ctx, doc.EventID, doc); err != nil {
			t.Fatalf("Index() error for %s: %v", doc.EventID, err)
		}
	}

	query := domain.LogQuery{
		Tenant:  "tenant-a",
		From:    now - 60,
		To:      now + 100,
		Limit:   10,
		Service: "api",
	}

	result := waitForLogs(t, repo, query, 2)

	if len(result.Logs) != 2 {
		t.Fatalf("len(result.Logs) = %d, want 2", len(result.Logs))
	}

	for _, log := range result.Logs {
		if log.Tenant != "tenant-a" {
			t.Fatalf("tenant = %q, want %q", log.Tenant, "tenant-a")
		}

		if log.Service != "api" {
			t.Fatalf("service = %q, want %q", log.Service, "api")
		}
	}

	if result.Logs[0].EventID != "event-1" {
		t.Fatalf("eventID = %q, want %q", result.Logs[0].EventID, "event-1")
	}

	if result.Logs[1].EventID != "event-3" {
		t.Fatalf("eventID = %q, want %q", result.Logs[1].EventID, "event-3")
	}
}

func TestLogRepoQueryByLogLevel(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	ctx := context.Background()

	indexName := fmt.Sprintf("log_it_%d", time.Now().UnixNano())

	repo, err := elastic.NewLogRepo([]string{"http://localhost:9200"}, indexName)
	if err != nil {
		t.Fatalf("NewLogRepo() error = %v", err)
	}

	defer cleanupIndex(t, repo, indexName)

	now := time.Now().Unix()

	log1 := domain.Log{
		EventID:   "event-1",
		Tenant:    "tenant-a",
		Service:   "api",
		Host:      "host-1",
		LogLevel:  domain.ERROR,
		Message:   "first error",
		Timestamp: now,
	}

	log2 := domain.Log{
		EventID:   "event-2",
		Tenant:    "tenant-a",
		Service:   "worker",
		Host:      "host-2",
		LogLevel:  domain.DEBUG,
		Message:   "second debug",
		Timestamp: now + 5,
	}

	log3 := domain.Log{
		EventID:   "event-3",
		Tenant:    "tenant-a",
		Service:   "api",
		Host:      "host-3",
		LogLevel:  domain.ERROR,
		Message:   "other tenant error",
		Timestamp: now + 10,
	}

	for _, doc := range []domain.Log{log1, log2, log3} {
		if err := repo.Index(ctx, doc.EventID, doc); err != nil {
			t.Fatalf("Index() error for %s: %v", doc.EventID, err)
		}
	}

	query := domain.LogQuery{
		Tenant:   "tenant-a",
		From:     now - 60,
		To:       now + 100,
		Limit:    10,
		LogLevel: domain.ERROR,
	}

	result := waitForLogs(t, repo, query, 2)

	if len(result.Logs) != 2 {
		t.Fatalf("len(result.Logs) = %d, want 2", len(result.Logs))
	}

	for _, log := range result.Logs {
		if log.Tenant != "tenant-a" {
			t.Fatalf("tenant = %q, want %q", log.Tenant, "tenant-a")
		}

		if log.LogLevel != domain.ERROR {
			t.Fatalf("loglevel = %q, want %q", log.LogLevel, domain.ERROR)
		}
	}

	if result.Logs[0].EventID != "event-1" {
		t.Fatalf("eventID = %q, want %q", result.Logs[0].EventID, "event-1")
	}

	if result.Logs[1].EventID != "event-3" {
		t.Fatalf("eventID = %q, want %q", result.Logs[1].EventID, "event-3")
	}
}

func TestLogRepoQueryByKeywordMatch(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	ctx := context.Background()

	indexName := fmt.Sprintf("log_it_%d", time.Now().UnixNano())

	repo, err := elastic.NewLogRepo([]string{"http://localhost:9200"}, indexName)
	if err != nil {
		t.Fatalf("NewLogRepo() error = %v", err)
	}

	defer cleanupIndex(t, repo, indexName)

	now := time.Now().Unix()

	logA1 := domain.Log{
		EventID:   "event-1",
		Tenant:    "tenant-a",
		Service:   "api",
		Host:      "host-1",
		LogLevel:  domain.ERROR,
		Message:   "first error",
		Timestamp: now,
	}

	logA2 := domain.Log{
		EventID:   "event-2",
		Tenant:    "tenant-a",
		Service:   "worker",
		Host:      "host-2",
		LogLevel:  domain.DEBUG,
		Message:   "second debug",
		Timestamp: now + 5,
	}

	logB1 := domain.Log{
		EventID:   "event-3",
		Tenant:    "tenant-b",
		Service:   "api",
		Host:      "host-3",
		LogLevel:  domain.ERROR,
		Message:   "other tenant error",
		Timestamp: now + 10,
	}

	for _, doc := range []domain.Log{logA1, logA2, logB1} {
		if err := repo.Index(ctx, doc.EventID, doc); err != nil {
			t.Fatalf("Index() error for %s: %v", doc.EventID, err)
		}
	}

	query := domain.LogQuery{
		Tenant:  "tenant-a",
		From:    now - 60,
		To:      now + 100,
		Limit:   10,
		Keyword: "debug",
	}

	result := waitForLogs(t, repo, query, 1)

	if len(result.Logs) != 1 {
		t.Fatalf("len(result.Logs) = %d, want 1", len(result.Logs))
	}

	if result.Logs[0].Tenant != "tenant-a" {
		t.Fatalf("tenant = %q, want %q", result.Logs[0].Tenant, "tenant-a")
	}

	if result.Logs[0].EventID != "event-2" {
		t.Fatalf("eventID = %q, want %q", result.Logs[0].EventID, "event-1")
	}

	if !strings.Contains(result.Logs[0].Message, query.Keyword) {
		t.Fatalf("message = %q, want keyword %q", result.Logs[0].Message, query.Keyword)
	}
}

func waitForLogs(t *testing.T, repo *elastic.LogRepo, q domain.LogQuery, wantCount int) *domain.QueryResponse {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)

	for {
		res, err := repo.Query(context.Background(), q)
		if err == nil && len(res.Logs) == wantCount {
			return res
		}

		if time.Now().After(deadline) {
			if err != nil {
				t.Fatalf("query failed before deadline: %v", err)
			}
			t.Fatalf("query returned %d logs, want %d", len(res.Logs), wantCount)
		}

		time.Sleep(200 * time.Millisecond)
	}
}

func cleanupIndex(t *testing.T, repo *elastic.LogRepo, index string) {
	t.Helper()
	ctx := context.Background()
	err := repo.DeleteIndex(ctx, index)
	if err != nil {
		t.Logf("cleanup index warning: %v", err)
	}
}
