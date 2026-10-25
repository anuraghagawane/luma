//go:build integration

package postgres_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"

	"github.com/anuraghagawane/luma/internal/repository/postgres"
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
