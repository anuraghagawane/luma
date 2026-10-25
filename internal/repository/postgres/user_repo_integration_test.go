//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/anuraghagawane/luma/internal/domain"
)

func TestCreateAndFindUserAndTenant(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)

	user := domain.User{Email: "abc@abc.com", PasswordHash: "abcdef", Status: domain.ACTIVE, Role: domain.ADMIN}
	tenant := domain.Tenant{Name: "abc", Status: domain.TACTIVE}

	err := db.CreateTenantAndUser(ctx, &user, &tenant)
	if err != nil {
		t.Fatalf("error in CreateTenantAndUser(): %v", err)
	}

	foundUser, err := db.FindUserWithEmail(ctx, user.Email)
	if err != nil {
		t.Fatalf("error in FindUserWithEmail(): %v", err)
	}

	var foundTenant domain.Tenant
	err = db.pool.QueryRow(ctx, "select name, status from tenants where id = $1", foundUser.TenantID).Scan(&foundTenant.Name, &foundTenant.Status)
	if err != nil {
		t.Fatalf("error in finding tenant: %v", err)
	}
	if foundUser.Email != user.Email {
		t.Fatalf("email: %v, want %v", foundUser.Email, user.Email)
	}

	if foundUser.PasswordHash != user.PasswordHash {
		t.Fatalf("passwordHash: %v, want %v", foundUser.PasswordHash, user.PasswordHash)
	}

	if foundUser.Status != user.Status {
		t.Fatalf("status: %v, want %v", foundUser.Status, user.Status)
	}

	if foundUser.Role != user.Role {
		t.Fatalf("status: %v, want %v", foundUser.Role, user.Role)
	}

	if foundTenant.Name != tenant.Name {
		t.Fatalf("name: %v, want %v", foundTenant.Name, tenant.Name)
	}

	if foundTenant.Status != tenant.Status {
		t.Fatalf("tenant status: %v, want %v", foundTenant.Status, tenant.Status)
	}
}

func TestCreateDuplicateUserWithSameEmail(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)

	user := domain.User{Email: "abc@abc.com", PasswordHash: "abcdef", Status: domain.ACTIVE, Role: domain.ADMIN}
	tenant := domain.Tenant{Name: "abc", Status: domain.TACTIVE}

	err := db.CreateTenantAndUser(ctx, &user, &tenant)
	if err != nil {
		t.Fatalf("error in CreateTenantAndUser(): %v", err)
	}

	var tenantCountBefore int
	err = db.pool.QueryRow(ctx, "select count(*) from tenants where name = $1;", tenant.Name).Scan(&tenantCountBefore)
	if err != nil {
		t.Fatalf("error fetching tenantCountBefore: %v", err)
	}

	if tenantCountBefore == 0 {
		t.Fatalf("tenantCountBefore: %v, want 1", tenantCountBefore)
	}

	// should fail for dupliate email address
	err = db.CreateTenantAndUser(ctx, &user, &tenant)
	if err == nil {
		t.Fatalf("expected error found none")
	}

	var tenantCountAfter int
	err = db.pool.QueryRow(ctx, "select count(*) from tenants where name = $1;", tenant.Name).Scan(&tenantCountAfter)
	if err != nil {
		t.Fatalf("error fetching tenantCountBefore: %v", err)
	}

	if tenantCountAfter != tenantCountBefore {
		t.Fatalf("tenantCountAfter: %v, want %v", tenantCountAfter, tenantCountBefore)
	}
}
