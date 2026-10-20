// Package outbox consist of worker and utilities for outbox worker
package outbox

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/anuraghagawane/luma/internal/domain"
	"github.com/google/uuid"
)

type worker struct {
	workerID     string
	cache        domain.CacheClient
	outboxRepo   domain.OutboxEventRepository
	pollInterval time.Duration
	retryPolicy  RetryPolicy
}

func NewWorker(cache domain.CacheClient, outboxRepo domain.OutboxEventRepository, pollInterval time.Duration, retryPolicy RetryPolicy) *worker {
	id := uuid.NewString()
	return &worker{workerID: id, cache: cache, outboxRepo: outboxRepo, pollInterval: pollInterval, retryPolicy: retryPolicy}
}

func (w *worker) Run(ctx context.Context) error {
	ticker := time.NewTicker(w.pollInterval)

	for {
		if err := w.processBatch(ctx); err != nil {
			log.Printf("process outbox error: %v", err)
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (w *worker) processBatch(ctx context.Context) error {
	events, err := w.outboxRepo.ClaimBatch(ctx, w.workerID)
	if err != nil {
		return fmt.Errorf("failed to claim batch: %w", err)
	}
	var wg sync.WaitGroup
	for _, event := range events {
		wg.Add(1)
		go func(event domain.OutBoxEvents) {
			defer wg.Done()
			result, err := w.cache.Apply(ctx, "api_key:"+event.AggregateID, event.Payload, 0)
			if err != nil {
				if event.Attempts >= w.retryPolicy.MaxAttempts {
					repoerr := w.outboxRepo.MarkDead(ctx, event.ID, err, event.ClaimToken)
					if repoerr != nil {
						if errors.Is(repoerr, domain.ErrLostClaim) {
							fmt.Printf("outbox event transition skipped: claim is no longer owned by this worker, event_id: %v, worker_id: %v, transition: %v\n", event.ID, w.workerID, "mark_dead")
							return
						}
						fmt.Printf("failed to mark dead: %v\n", repoerr)
					}
				} else {
					delay := w.retryPolicy.Delay(event.Attempts)
					repoerr := w.outboxRepo.ScheduleRetry(ctx, event.ID, err, event.ClaimToken, delay)
					if errors.Is(repoerr, domain.ErrLostClaim) {
						fmt.Printf("outbox event transition skipped: claim is no longer owned by this worker, event_id: %v, worker_id: %v, transition: %v\n", event.ID, w.workerID, "schedule_retry")
						return
					}
					if repoerr != nil {
						fmt.Printf("failed to schedule retry: %v\n", repoerr)
						return
					}
				}
				return
			}

			switch result {
			case domain.CacheApplyUpdated:
				fmt.Printf("cache updates for keyID: %v\n", event.AggregateID)
			case domain.CacheApplyStale:
				fmt.Printf("cache stale, update skipped for keyID: %v\n", event.AggregateID)
			case domain.CacheApplyUnchanged:
				fmt.Printf("cache at same version, update skipped for keyID: %v\n", event.AggregateID)
			}

			err = w.outboxRepo.MarkProcessed(ctx, event.ID, event.ClaimToken)
			if err != nil {
				if errors.Is(err, domain.ErrLostClaim) {
					fmt.Printf("outbox event transition skipped: claim is no longer owned by this worker, event_id: %v, worker_id: %v, transition: %v\n", event.ID, w.workerID, "mark_processed")
					return
				}
				fmt.Printf("failed to mark event as processed: %v\n", err)
			}
		}(event)
	}
	wg.Wait()
	return nil
}
