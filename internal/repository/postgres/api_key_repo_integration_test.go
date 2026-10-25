//go:build integration

package postgres_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"

	"github.com/anuraghagawane/luma/internal/domain"
	"github.com/anuraghagawane/luma/internal/repository/postgres"
	"github.com/anuraghagawane/luma/internal/security/apikey"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DBRepo struct {
	pool *pgxpool.Pool
	*postgres.APIKeyRepo
	*postgres.UserRepo
}

func setupTestDB(t *testing.T) *DBRepo {
	t.Helper()
	ctx := context.Background()
	dsn := os.Getenv("LUMA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("LUMA_TEST_DATABASE_URL must point to a dedicated test database")
	}
	adminPool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}

	var id [8]byte
	if _, err := rand.Read(id[:]); err != nil {
		adminPool.Close()
		t.Fatalf("generate test schema name: %v", err)
	}

	schema := "it_" + hex.EncodeToString(id[:])
	schemaSQL := pgx.Identifier{schema}.Sanitize()

	if _, err := adminPool.Exec(ctx, "CREATE SCHEMA "+schemaSQL); err != nil {
		adminPool.Close()
		t.Fatalf("create test schema: %v", err)
	}

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		_, _ = adminPool.Exec(ctx, "DROP SCHEMA "+schemaSQL+" CASCADE")
		adminPool.Close()
		t.Fatalf("parse test database config: %v", err)
	}

	config.ConnConfig.RuntimeParams["search_path"] = schema
	testPool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		_, _ = adminPool.Exec(ctx, "DROP SCHEMA "+schemaSQL+" CASCADE")
		adminPool.Close()
		t.Fatalf("create test pool: %v", err)
	}

	t.Cleanup(func() {
		testPool.Close()
		if _, err = adminPool.Exec(ctx, "DROP SCHEMA "+schemaSQL+" CASCADE"); err != nil {
			t.Errorf("drop test schema: %v", err)
		}
		adminPool.Close()
	})

	migrator := postgres.NewMigrator(testPool, "../../../migrations/", ctx)
	if err := migrator.RunMigration(); err != nil {
		t.Fatalf("RunMigration(): %v", err)
	}

	return &DBRepo{pool: testPool, APIKeyRepo: postgres.NewAPIKeyRepo(testPool), UserRepo: postgres.NewUserRepo(testPool)}
}

func createUserAndTenant(t *testing.T, db *DBRepo) (tenantID string, userID string) {
	t.Helper()

	ctx := context.Background()
	user := domain.User{Email: "abc@abc.abc", PasswordHash: "abcdef", Status: domain.ACTIVE, Role: domain.ADMIN}
	tenant := domain.Tenant{Name: "tenant-1", Status: domain.TACTIVE}

	err := db.CreateTenantAndUser(ctx, &user, &tenant)
	if err != nil {
		t.Fatalf("error in CreateTenantAndUser(): %v", err)
	}

	return tenant.ID, user.ID
}

func TestAPIKeyRepoCreateAndFetchByID(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	db := setupTestDB(t)
	ctx := context.Background()

	tenantID, userID := createUserAndTenant(t, db)

	apiKeyObj, err := apikey.GenerateAPIKey("apikey", tenantID, userID)
	if err != nil {
		t.Fatalf("error in GenerateAPIKey(): %v", err)
	}
	err = db.CreateAPIKeyAndEvent(ctx, *apiKeyObj)
	if err != nil {
		t.Fatalf("error in CreateAPIKeyAndEvent(): %v", err)
	}

	key, err := db.GetAPIKey(ctx, apiKeyObj.KeyID)
	if err != nil {
		t.Fatalf("error in GetAPIKey(): %v", err)
	}

	var outboxEvent domain.OutBoxEvents

	err = db.pool.QueryRow(ctx, "select aggregate_id, aggregate_version, status, event_type, aggregate_type from outbox_events where aggregate_id = $1;", apiKeyObj.KeyID).Scan(&outboxEvent.AggregateID, &outboxEvent.AggregateVersion, &outboxEvent.Status, &outboxEvent.EventType, &outboxEvent.AggregateType)
	if err != nil {
		t.Fatalf("error while fetching from outbox_events: %v", err)
	}

	if key == nil {
		t.Fatalf("key not found")
	}

	if key.KeyID != apiKeyObj.KeyID {
		t.Fatalf("keyid: %v, want %v", key.KeyID, apiKeyObj.KeyID)
	}

	if key.TenantID != apiKeyObj.TenantID {
		t.Fatalf("tenantID: %v, want %v", key.TenantID, apiKeyObj.TenantID)
	}

	if key.Hash != apiKeyObj.Hash {
		t.Fatalf("Hash: %v, want %v", key.Hash, apiKeyObj.Hash)
	}

	if key.Status != domain.APIKeyStatus(domain.APIKeyActive) {
		t.Fatalf("status: %v, want %v", key.Status, domain.APIKeyActive)
	}

	if key.Version != apiKeyObj.Version {
		t.Fatalf("version: %v, want %v", key.Version, apiKeyObj.Version)
	}

	if outboxEvent.AggregateID != apiKeyObj.KeyID {
		t.Fatalf("aggregate_id: %v, want %v", outboxEvent.AggregateID, apiKeyObj.KeyID)
	}

	if outboxEvent.AggregateVersion != apiKeyObj.Version {
		t.Fatalf("aggregate_version: %v, want %v", outboxEvent.AggregateVersion, apiKeyObj.Version)
	}

	if outboxEvent.Status != "PENDING" {
		t.Fatalf(`outboxEvent status: %v, want "PENDING"`, outboxEvent.Status)
	}

	if outboxEvent.EventType != "API_KEY_STATE_CHANGED" {
		t.Fatalf(`event_type: %v, want "API_KEY_STATE_CHANGED"`, outboxEvent.EventType)
	}

	if outboxEvent.AggregateType != "API_KEY" {
		t.Fatalf(`aggregate_type: %v, want "API_KEY"`, outboxEvent.AggregateType)
	}
}

func TestDeleteAPIKeyAndFetchByID(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	db := setupTestDB(t)
	ctx := context.Background()

	tenantID, userID := createUserAndTenant(t, db)

	apiKeyObj, err := apikey.GenerateAPIKey("apikey", tenantID, userID)
	if err != nil {
		t.Fatalf("error in GenerateAPIKey(): %v", err)
	}
	err = db.CreateAPIKeyAndEvent(ctx, *apiKeyObj)
	if err != nil {
		t.Fatalf("error in CreateAPIKeyAndEvent(): %v", err)
	}

	err = db.DeleteAPIKeyAndCreateEvent(ctx, apiKeyObj.KeyID, tenantID, userID)
	if err != nil {
		t.Fatalf("error in DeleteAPIKeyAndCreateEvent(): %v", err)
	}

	key, err := db.GetAPIKey(ctx, apiKeyObj.KeyID)
	if err != nil {
		t.Fatalf("error in GetAPIKey(): %v", err)
	}

	var outboxEvent domain.OutBoxEvents

	err = db.pool.QueryRow(ctx, "select aggregate_id, aggregate_version, status, event_type, aggregate_type from outbox_events where aggregate_id = $1 and aggregate_version = $2;", apiKeyObj.KeyID, apiKeyObj.Version+1).Scan(&outboxEvent.AggregateID, &outboxEvent.AggregateVersion, &outboxEvent.Status, &outboxEvent.EventType, &outboxEvent.AggregateType)
	if err != nil {
		t.Fatalf("error while fetching from outbox_events: %v", err)
	}

	if key.Status != domain.APIKeyStatus(domain.APIKeyRevoked) {
		t.Fatalf("status: %v, want %v", key.Status, domain.APIKeyRevoked)
	}

	if key.Version != apiKeyObj.Version+1 {
		t.Fatalf("version: %v, want %v", key.Version, apiKeyObj.Version+1)
	}

	if outboxEvent.AggregateVersion != apiKeyObj.Version+1 {
		t.Fatalf("aggregate_version: %v, want %v", outboxEvent.AggregateVersion, apiKeyObj.Version+1)
	}

	if outboxEvent.Status != "PENDING" {
		t.Fatalf(`outboxEvent status: %v, want "PENDING"`, outboxEvent.Status)
	}

	if outboxEvent.EventType != "API_KEY_STATE_CHANGED" {
		t.Fatalf(`event_type: %v, want "API_KEY_STATE_CHANGED"`, outboxEvent.EventType)
	}

	if outboxEvent.AggregateType != "API_KEY" {
		t.Fatalf(`aggregate_type: %v, want "API_KEY"`, outboxEvent.AggregateType)
	}
}
