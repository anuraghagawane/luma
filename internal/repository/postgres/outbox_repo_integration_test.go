//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/anuraghagawane/luma/internal/domain"
	"github.com/anuraghagawane/luma/internal/security/apikey"
	"github.com/google/uuid"
)

func TestClaimBatch(t *testing.T) {
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

	workerID := uuid.NewString()
	events, err := db.ClaimBatch(ctx, workerID)
	if err != nil {
		t.Fatalf("error in ClaimBatch(): %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}

	event := events[0]
	if event.AggregateID != apiKeyObj.KeyID {
		t.Fatalf("aggregate ID = %q, want %q", event.AggregateID, apiKeyObj.KeyID)
	}
	if event.AggregateVersion != apiKeyObj.Version {
		t.Fatalf("aggregate version = %d, want %d", event.AggregateVersion, apiKeyObj.Version)
	}
	if event.ClaimToken == "" {
		t.Fatal("ClaimBatch() returned an empty claim token")
	}
	if event.Attempts != 1 {
		t.Fatalf("attempts = %d, want 1", event.Attempts)
	}

	var status, lockedBy, claimToken string
	var hasLease bool
	err = db.pool.QueryRow(ctx, `
		SELECT status, locked_by, claim_token, locked_until IS NOT NULL
		FROM outbox_events
		WHERE id = $1
	`, event.ID).Scan(&status, &lockedBy, &claimToken, &hasLease)
	if err != nil {
		t.Fatalf("failed to fetch claimed event: %v", err)
	}
	if status != "PROCESSING" {
		t.Fatalf("status = %q, want %q", status, "PROCESSING")
	}
	if lockedBy != workerID {
		t.Fatalf("locked_by = %q, want %q", lockedBy, workerID)
	}
	if claimToken != event.ClaimToken {
		t.Fatalf("stored claim token = %q, want %q", claimToken, event.ClaimToken)
	}
	if !hasLease {
		t.Fatal("locked_until is NULL, want an active lease")
	}
}

func TestMarkProcessed(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	tests := []struct {
		name              string
		invalidClaimToken bool
	}{
		{
			name:              "success",
			invalidClaimToken: false,
		},
		{
			name:              "invalid claim",
			invalidClaimToken: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
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

			workerID := uuid.NewString()
			events, err := db.ClaimBatch(ctx, workerID)
			if err != nil {
				t.Fatalf("error in ClaimBatch(): %v", err)
			}
			if len(events) != 1 {
				t.Fatalf("len(events) = %d, want 1", len(events))
			}

			event := events[0]

			id := event.ID
			claimToken := event.ClaimToken

			if test.invalidClaimToken {
				claimToken = uuid.NewString()
			}

			err = db.MarkProcessed(ctx, id, claimToken)
			if err != nil {
				if test.invalidClaimToken && errors.Is(err, domain.ErrLostClaim) {
				} else {
					t.Fatalf("error MarkProcessed(): %v", err)
				}
			}

			var status, lockedBy, claimTokenDB *string
			var processedAt, lockedUntil *time.Time
			err = db.pool.QueryRow(ctx, "SELECT status, processed_at, locked_by, locked_until, claim_token from outbox_events where id = $1", id).Scan(&status, &processedAt, &lockedBy, &lockedUntil, &claimTokenDB)
			if err != nil {
				t.Fatalf("error while fetching after updates state: %v", err)
			}

			if !test.invalidClaimToken {
				if *status != "PROCESSED" {
					t.Fatalf("status: %v, want PROCESSED", status)
				}

				if lockedBy != nil {
					t.Fatalf("lockedBy: %v, want nil", lockedBy)
				}

				if claimTokenDB != nil {
					t.Fatalf("claimToken after update: %v, want nil", claimTokenDB)
				}

				if processedAt.After(time.Now().Add(5*time.Second)) || processedAt.Before(time.Now().Add(-5*time.Second)) {
					t.Fatalf("processedAt not marked: %v", processedAt)
				}

				if lockedUntil != nil {
					t.Fatalf("lockedUntil: %v, want nil", lockedUntil)
				}
			} else {
				if *status != "PROCESSING" {
					t.Fatalf("status: %v, want PROCESSING", status)
				}

				if lockedBy == nil {
					t.Fatalf("lockedBy should be present")
				}

				if claimTokenDB == nil {
					t.Fatalf("claimToken should not be removed")
				}

				if processedAt != nil {
					t.Fatalf("processedAt should not be marked: %v", processedAt)
				}

				if lockedUntil == nil {
					t.Fatalf("lockedUntil should not be nil")
				}
			}
		})
	}
}

func TestScheduleRetry(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	tests := []struct {
		name              string
		invalidClaimToken bool
	}{
		{name: "success", invalidClaimToken: false},
		{name: "invalid claim", invalidClaimToken: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := setupTestDB(t)
			ctx := context.Background()

			tenantID, userID := createUserAndTenant(t, db)
			apiKeyObj, err := apikey.GenerateAPIKey("apikey", tenantID, userID)
			if err != nil {
				t.Fatalf("error in GenerateAPIKey(): %v", err)
			}
			if err = db.CreateAPIKeyAndEvent(ctx, *apiKeyObj); err != nil {
				t.Fatalf("error in CreateAPIKeyAndEvent(): %v", err)
			}

			events, err := db.ClaimBatch(ctx, uuid.NewString())
			if err != nil {
				t.Fatalf("error in ClaimBatch(): %v", err)
			}
			if len(events) != 1 {
				t.Fatalf("len(events) = %d, want 1", len(events))
			}

			event := events[0]
			cause := errors.New("temporary failure")
			retryDelay := 2 * time.Second

			claimToken := event.ClaimToken
			if test.invalidClaimToken {
				claimToken = uuid.NewString()
			}

			err = db.ScheduleRetry(ctx, event.ID, cause, claimToken, retryDelay)
			if err != nil {
				if test.invalidClaimToken && errors.Is(err, domain.ErrLostClaim) {
					return
				}
				t.Fatalf("error in ScheduleRetry(): %v", err)
			}

			if test.invalidClaimToken {
				t.Fatal("ScheduleRetry() should have failed with ErrLostClaim")
			}

			var status string
			var lastError, claimTokenDB *string
			var lockedBy *string
			var availableAt time.Time
			err = db.pool.QueryRow(ctx, `
				SELECT status, last_error, claim_token, locked_by, available_at
				FROM outbox_events
				WHERE id = $1
			`, event.ID).Scan(&status, &lastError, &claimTokenDB, &lockedBy, &availableAt)
			if err != nil {
				t.Fatalf("error while fetching retried row: %v", err)
			}

			if status != "PENDING" {
				t.Fatalf("status = %q, want %q", status, "PENDING")
			}
			if lastError == nil || *lastError != cause.Error() {
				t.Fatalf("last_error = %v, want %q", lastError, cause.Error())
			}
			if claimTokenDB != nil {
				t.Fatalf("claim_token = %v, want nil", *claimTokenDB)
			}
			if lockedBy != nil {
				t.Fatalf("locked_by = %v, want nil", *lockedBy)
			}
			if !availableAt.After(time.Now()) {
				t.Fatalf("available_at = %v, want a future retry time", availableAt)
			}
		})
	}
}

func TestMarkDead(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	tests := []struct {
		name              string
		invalidClaimToken bool
	}{
		{name: "success", invalidClaimToken: false},
		{name: "invalid claim", invalidClaimToken: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := setupTestDB(t)
			ctx := context.Background()

			tenantID, userID := createUserAndTenant(t, db)
			apiKeyObj, err := apikey.GenerateAPIKey("apikey", tenantID, userID)
			if err != nil {
				t.Fatalf("error in GenerateAPIKey(): %v", err)
			}
			if err = db.CreateAPIKeyAndEvent(ctx, *apiKeyObj); err != nil {
				t.Fatalf("error in CreateAPIKeyAndEvent(): %v", err)
			}

			events, err := db.ClaimBatch(ctx, uuid.NewString())
			if err != nil {
				t.Fatalf("error in ClaimBatch(): %v", err)
			}
			if len(events) != 1 {
				t.Fatalf("len(events) = %d, want 1", len(events))
			}

			event := events[0]
			cause := errors.New("permanent failure")
			claimToken := event.ClaimToken
			if test.invalidClaimToken {
				claimToken = uuid.NewString()
			}

			err = db.MarkDead(ctx, event.ID, cause, claimToken)
			if err != nil {
				if test.invalidClaimToken && errors.Is(err, domain.ErrLostClaim) {
					return
				}
				t.Fatalf("error in MarkDead(): %v", err)
			}

			if test.invalidClaimToken {
				t.Fatal("MarkDead() should have failed with ErrLostClaim")
			}

			var status string
			var lastError, claimTokenDB *string
			var lockedBy *string
			var lockedUntil *time.Time
			err = db.pool.QueryRow(ctx, `
				SELECT status, last_error, claim_token, locked_by, locked_until
				FROM outbox_events
				WHERE id = $1
			`, event.ID).Scan(&status, &lastError, &claimTokenDB, &lockedBy, &lockedUntil)
			if err != nil {
				t.Fatalf("error while fetching dead row: %v", err)
			}

			if status != "DEAD" {
				t.Fatalf("status = %q, want %q", status, "DEAD")
			}
			if lastError == nil || *lastError != cause.Error() {
				t.Fatalf("last_error = %v, want %q", lastError, cause.Error())
			}
			if claimTokenDB != nil {
				t.Fatalf("claim_token = %v, want nil", *claimTokenDB)
			}
			if lockedBy != nil {
				t.Fatalf("locked_by = %v, want nil", *lockedBy)
			}
			if lockedUntil != nil {
				t.Fatalf("locked_until = %v, want nil", *lockedUntil)
			}
		})
	}
}

func TestClaimBatch_ExpiredLeaseRecovery(t *testing.T) {
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
	if err = db.CreateAPIKeyAndEvent(ctx, *apiKeyObj); err != nil {
		t.Fatalf("error in CreateAPIKeyAndEvent(): %v", err)
	}

	worker1 := "worker-1"
	events1, err := db.ClaimBatch(ctx, worker1)
	if err != nil {
		t.Fatalf("ClaimBatch() by worker1 failed: %v", err)
	}
	if len(events1) != 1 {
		t.Fatalf("len(events1) = %d, want 1", len(events1))
	}

	initialEvent := events1[0]

	// Simulate expired lease (worker 1 crashed or timed out)
	_, err = db.pool.Exec(ctx, `
		UPDATE outbox_events
		SET locked_until = NOW() - INTERVAL '10 seconds'
		WHERE id = $1
	`, initialEvent.ID)
	if err != nil {
		t.Fatalf("failed to simulate expired lease: %v", err)
	}

	worker2 := "worker-2"
	events2, err := db.ClaimBatch(ctx, worker2)
	if err != nil {
		t.Fatalf("ClaimBatch() by worker2 failed: %v", err)
	}
	if len(events2) != 1 {
		t.Fatalf("len(events2) = %d, want 1", len(events2))
	}

	reclaimedEvent := events2[0]
	if reclaimedEvent.ID != initialEvent.ID {
		t.Fatalf("reclaimed event ID = %v, want %v", reclaimedEvent.ID, initialEvent.ID)
	}
	if reclaimedEvent.ClaimToken == initialEvent.ClaimToken {
		t.Fatal("reclaimed event should have a new claim token")
	}
	if reclaimedEvent.Attempts != 2 {
		t.Fatalf("reclaimed attempts = %d, want 2", reclaimedEvent.Attempts)
	}

	// Stale worker1 should now fail with ErrLostClaim when trying to complete
	err = db.MarkProcessed(ctx, initialEvent.ID, initialEvent.ClaimToken)
	if !errors.Is(err, domain.ErrLostClaim) {
		t.Fatalf("stale worker MarkProcessed() err = %v, want ErrLostClaim", err)
	}

	// Worker 2 should succeed
	err = db.MarkProcessed(ctx, reclaimedEvent.ID, reclaimedEvent.ClaimToken)
	if err != nil {
		t.Fatalf("active worker MarkProcessed() err = %v, want nil", err)
	}
}

func TestClaimBatch_OrderingAndBatchLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	db := setupTestDB(t)
	ctx := context.Background()

	tenantID, userID := createUserAndTenant(t, db)

	// Create 12 events; 11 available now and 1 scheduled in the future
	for i := 0; i < 11; i++ {
		apiKeyObj, err := apikey.GenerateAPIKey(fmt.Sprintf("apikey-%d", i), tenantID, userID)
		if err != nil {
			t.Fatalf("error generating api key %d: %v", i, err)
		}
		if err = db.CreateAPIKeyAndEvent(ctx, *apiKeyObj); err != nil {
			t.Fatalf("error creating api key %d: %v", i, err)
		}
	}

	futureKey, err := apikey.GenerateAPIKey("apikey-future", tenantID, userID)
	if err != nil {
		t.Fatalf("error generating future api key: %v", err)
	}
	if err = db.CreateAPIKeyAndEvent(ctx, *futureKey); err != nil {
		t.Fatalf("error creating future api key: %v", err)
	}
	// Push available_at for futureKey into the future
	_, err = db.pool.Exec(ctx, `
		UPDATE outbox_events
		SET available_at = NOW() + INTERVAL '1 hour'
		WHERE aggregate_id = $1
	`, futureKey.KeyID)
	if err != nil {
		t.Fatalf("failed to update future event available_at: %v", err)
	}

	// First claim should fetch exactly 10 due to LIMIT 10
	events, err := db.ClaimBatch(ctx, "worker-limit-1")
	if err != nil {
		t.Fatalf("ClaimBatch() failed: %v", err)
	}
	if len(events) != 10 {
		t.Fatalf("len(events) = %d, want 10", len(events))
	}

	// Second claim should fetch the remaining 1 available event (and ignore the future one)
	eventsSecondBatch, err := db.ClaimBatch(ctx, "worker-limit-2")
	if err != nil {
		t.Fatalf("ClaimBatch() second batch failed: %v", err)
	}
	if len(eventsSecondBatch) != 1 {
		t.Fatalf("len(eventsSecondBatch) = %d, want 1", len(eventsSecondBatch))
	}
	if eventsSecondBatch[0].AggregateID == futureKey.KeyID {
		t.Fatal("future event was claimed prematurely")
	}

	// Third claim should find 0 events
	eventsThirdBatch, err := db.ClaimBatch(ctx, "worker-limit-3")
	if err != nil {
		t.Fatalf("ClaimBatch() third batch failed: %v", err)
	}
	if len(eventsThirdBatch) != 0 {
		t.Fatalf("len(eventsThirdBatch) = %d, want 0", len(eventsThirdBatch))
	}
}

func TestClaimBatch_ConcurrentWorkersSkipLocked(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	db := setupTestDB(t)
	ctx := context.Background()

	tenantID, userID := createUserAndTenant(t, db)

	// Create 10 events
	totalEvents := 10
	for i := 0; i < totalEvents; i++ {
		apiKeyObj, err := apikey.GenerateAPIKey(fmt.Sprintf("key-%d", i), tenantID, userID)
		if err != nil {
			t.Fatalf("GenerateAPIKey error: %v", err)
		}
		if err = db.CreateAPIKeyAndEvent(ctx, *apiKeyObj); err != nil {
			t.Fatalf("CreateAPIKeyAndEvent error: %v", err)
		}
	}

	numWorkers := 5
	claimedIDs := make(chan string, totalEvents*2)
	errCh := make(chan error, numWorkers)
	var wg sync.WaitGroup

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerIdx int) {
			defer wg.Done()
			workerID := fmt.Sprintf("concurrent-worker-%d", workerIdx)
			batch, err := db.ClaimBatch(ctx, workerID)
			if err != nil {
				errCh <- err
				return
			}
			for _, ev := range batch {
				claimedIDs <- ev.ID
			}
		}(i)
	}

	wg.Wait()
	close(claimedIDs)
	close(errCh)

	for err := range errCh {
		t.Fatalf("error from concurrent worker: %v", err)
	}

	seen := make(map[string]bool)
	for id := range claimedIDs {
		if seen[id] {
			t.Fatalf("event %s was claimed by multiple workers simultaneously!", id)
		}
		seen[id] = true
	}

	if len(seen) != totalEvents {
		t.Fatalf("total claimed events = %d, want %d", len(seen), totalEvents)
	}
}

func TestTransitions_TerminalOrNonExistent(t *testing.T) {
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
	if err = db.CreateAPIKeyAndEvent(ctx, *apiKeyObj); err != nil {
		t.Fatalf("error in CreateAPIKeyAndEvent(): %v", err)
	}

	events, err := db.ClaimBatch(ctx, uuid.NewString())
	if err != nil || len(events) != 1 {
		t.Fatalf("failed to claim event: %v", err)
	}
	event := events[0]

	// Mark processed once
	if err = db.MarkProcessed(ctx, event.ID, event.ClaimToken); err != nil {
		t.Fatalf("initial MarkProcessed failed: %v", err)
	}

	// Repeated transition calls on already PROCESSED event should return ErrLostClaim
	if err = db.MarkProcessed(ctx, event.ID, event.ClaimToken); !errors.Is(err, domain.ErrLostClaim) {
		t.Fatalf("repeated MarkProcessed err = %v, want ErrLostClaim", err)
	}
	if err = db.ScheduleRetry(ctx, event.ID, errors.New("err"), event.ClaimToken, time.Minute); !errors.Is(err, domain.ErrLostClaim) {
		t.Fatalf("ScheduleRetry on PROCESSED event err = %v, want ErrLostClaim", err)
	}
	if err = db.MarkDead(ctx, event.ID, errors.New("err"), event.ClaimToken); !errors.Is(err, domain.ErrLostClaim) {
		t.Fatalf("MarkDead on PROCESSED event err = %v, want ErrLostClaim", err)
	}

	// Calling with non-existent ID
	randomID := uuid.NewString()
	if err = db.MarkProcessed(ctx, randomID, uuid.NewString()); !errors.Is(err, domain.ErrLostClaim) {
		t.Fatalf("MarkProcessed on random ID err = %v, want ErrLostClaim", err)
	}
}
