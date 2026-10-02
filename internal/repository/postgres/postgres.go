// Package postgres
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, connString string) (*pgxpool.Pool, error) {
	dbPool, _ := pgxpool.New(ctx, connString)

	err := dbPool.Ping(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %w", err)
	}

	return dbPool, err
}
