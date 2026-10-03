package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/anuraghagawane/luma/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepo struct {
	client *pgxpool.Pool
}

func NewUserRepo(client *pgxpool.Pool) *UserRepo {
	return &UserRepo{client}
}

func (r *UserRepo) Create(ctx context.Context, user *domain.User) error { return nil }
func (r *UserRepo) CreateTenantAndUser(ctx context.Context, user *domain.User, tenant *domain.Tenant) error {
	tx, err := r.client.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer tx.Rollback(ctx)

	var tenantID string
	err = tx.QueryRow(ctx, "Insert into tenants (name, status) values ($1, $2) returning id", tenant.Name, tenant.Status).Scan(&tenantID)
	if err != nil {
		return fmt.Errorf("failed to create tenant: %w", err)
	}

	res, err := tx.Exec(ctx, "Insert into users (tenant_id, email, password_hash, role, status) values ($1, $2, $3, $4, $5);", tenantID, user.Email, user.PasswordHash, user.Role, user.Status)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("user with this email already exists")
		}
		return fmt.Errorf("failed to create user: %w", err)
	}

	if res.RowsAffected() != 1 {
		return fmt.Errorf("failed to create row, RowsAffected %d", res.RowsAffected())
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed ot commit transaction: %w", err)
	}

	tenant.ID = tenantID
	user.TenantID = tenantID

	return nil
}

func (r *UserRepo) FindUserWithEmail(ctx context.Context, email string) (*domain.User, error) {
	var user domain.User

	err := r.client.QueryRow(ctx, "select id, tenant_id, email, password_hash, role, status from users where email = $1 and status = 'ACTIVE';", email).Scan(&user.ID, &user.TenantID, &user.Email, &user.PasswordHash, &user.Role, &user.Status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("email not registered")
		}
		return nil, err
	}

	return &user, nil
}
