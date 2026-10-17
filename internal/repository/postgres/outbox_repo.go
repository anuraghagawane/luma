package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/anuraghagawane/luma/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OutboxEventRepo struct {
	client *pgxpool.Pool
}

func NewOutboxEventRepo(client *pgxpool.Pool) *OutboxEventRepo {
	return &OutboxEventRepo{client: client}
}

func (r *OutboxEventRepo) ClaimBatch(ctx context.Context, workerID string) ([]domain.OutBoxEvents, error) {
	db, err := r.client.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}
	defer db.Rollback(ctx)
	claimQuery := `
		WITH picked AS (
			SELECT id
			FROM outbox_events
			WHERE (
				status = 'PENDING'
				AND available_at <= NOW()
			) OR (
				status = 'PROCESSING'
				AND locked_until < NOW()
			)
			ORDER BY available_at, created_at, id
			FOR UPDATE SKIP LOCKED
			LIMIT 10
		)
		UPDATE outbox_events AS e
		SET status = 'PROCESSING',
			locked_by = $1,
			claim_token = gen_random_uuid(),
			locked_until = NOW() + INTERVAL '30 seconds',
			attempts = attempts + 1
		FROM picked
		where e.id = picked.id
		RETURNING e.id, e.aggregate_id, e.aggregate_version, e.payload, e.claim_token, e.attempts;
	`
	rows, err := db.Query(ctx, claimQuery, workerID)
	if err != nil {
		return nil, fmt.Errorf("failed to claim batch: %w", err)
	}

	var events []domain.OutBoxEvents

	for rows.Next() {
		var event domain.OutBoxEvents
		err := rows.Scan(&event.ID, &event.AggregateID, &event.AggregateVersion, &event.Payload, &event.ClaimToken, &event.Attempts)
		if err != nil {
			return nil, fmt.Errorf("failed scan row: %w", err)
		}
		events = append(events, event)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error in rows: %w", err)
	}

	err = db.Commit(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to commit the transaction: %w", err)
	}
	return events, nil
}

func (r *OutboxEventRepo) MarkProcessed(ctx context.Context, id string, claimToken string) error {
	query := `
		UPDATE outbox_events SET status = 'PROCESSED', processed_at = NOW(), locked_by = NULL, locked_until = NULL, claim_token = NULL, last_error = NULL where id = $1 and claim_token = $2 and status = 'PROCESSING';
	`

	tag, err := r.client.Exec(ctx, query, id, claimToken)
	if err != nil {
		return fmt.Errorf("failed to mark processed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrLostClaim
	}
	return nil
}

func (r *OutboxEventRepo) ScheduleRetry(ctx context.Context, id string, cause error, claimToken string, delay time.Duration) error {
	query := `
		UPDATE outbox_events SET status = 'PENDING', available_at = NOW() + $1 * INTERVAL '1 microsecond', last_error = $2, claim_token = NULL, locked_by = NULL, locked_until = NULL where id = $3 and claim_token = $4 and status = 'PROCESSING';
	`

	tag, err := r.client.Exec(ctx, query, delay.Microseconds(), cause.Error(), id, claimToken)
	if err != nil {
		return fmt.Errorf("failed to mark processed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrLostClaim
	}

	return nil
}

func (r *OutboxEventRepo) MarkDead(ctx context.Context, id string, cause error, claimToken string) error {
	query := `
		UPDATE outbox_events SET status = 'DEAD', last_error = $1, claim_token = NULL, locked_by = NULL, locked_until = NULL where id = $2 and claim_token = $3 and status = 'PROCESSING';
	`

	tag, err := r.client.Exec(ctx, query, cause.Error(), id, claimToken)
	if err != nil {
		return fmt.Errorf("failed to mark processed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrLostClaim
	}

	return nil
}
