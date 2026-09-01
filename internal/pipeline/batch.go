// Package pipeline processes multiple baskets concurrently using a Go Concurrency
// Worker Pool (bounded goroutine pool) and Fan-Out / Fan-In patterns.
package pipeline

import (
	"context"
	"fmt"
	"iter"
	"sync"
	"sync/atomic"

	"github.com/hasanozgan/kata/discount-checkout/internal/checkout"
	"github.com/hasanozgan/kata/discount-checkout/internal/domain"
)

const (
	// DefaultWorkerCount is the default number of concurrent worker goroutines.
	DefaultWorkerCount = 4
)

// Job represents a single order to be processed along with its original index.
type Job struct {
	Index int
	Order *domain.Order
}

// JobResult carries the receipt or error for a processed order.
type JobResult struct {
	Index   int
	Receipt domain.Receipt
	Err     error
}

// BatchProcessor distributes baskets across a pool of worker goroutines for
// concurrent processing.
//
// Go Patterns applied:
//   - Bounded Worker Pool (goroutine pool with a fixed upper limit).
//   - Channel-based Fan-Out / Fan-In with goroutine leak prevention.
//   - Go 1.23+ Streaming Iterator (ProcessStream) for memory-efficient pipelines.
//   - sync/atomic for lock-free observability metrics (ProcessedCount).
type BatchProcessor struct {
	engine          *checkout.Checkout
	workerCount     int
	processedOrders atomic.Int64
}

// NewBatchProcessor creates a BatchProcessor with the given Checkout engine and
// worker count. Defaults to DefaultWorkerCount if workers <= 0.
func NewBatchProcessor(engine *checkout.Checkout, workers int) *BatchProcessor {
	if workers <= 0 {
		workers = DefaultWorkerCount
	}
	return &BatchProcessor{
		engine:      engine,
		workerCount: workers,
	}
}

// ProcessedCount returns the total number of successfully processed orders
// (thread-safe via atomic.Int64).
func (b *BatchProcessor) ProcessedCount() int64 {
	return b.processedOrders.Load()
}

// ProcessBatch processes a slice of orders concurrently in the worker pool and
// returns results in the original order index.
//
// Go Patterns applied:
//  1. Context cancellation and timeout protection (select ctx.Done()).
//  2. sync.WaitGroup + defer to prevent goroutine leaks.
//  3. Deterministic result ordering (results placed by original index — Fan-In).
func (b *BatchProcessor) ProcessBatch(ctx context.Context, orders []*domain.Order) ([]JobResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("batch processing aborted: %w", err)
	}

	n := len(orders)
	if n == 0 {
		return make([]JobResult, 0), nil
	}

	jobsChan := make(chan Job, n)
	resultsChan := make(chan JobResult, n)

	// 1. Fan-Out: launch the worker pool.
	var wg sync.WaitGroup
	actualWorkers := min(b.workerCount, n)

	for range actualWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobsChan {
				if err := ctx.Err(); err != nil {
					resultsChan <- JobResult{
						Index: job.Index,
						Err:   err,
					}
					continue
				}

				receipt, err := b.engine.Total(ctx, job.Order)
				if err == nil {
					b.processedOrders.Add(1)
				}
				resultsChan <- JobResult{
					Index:   job.Index,
					Receipt: receipt,
					Err:     err,
				}
			}
		}()
	}

	// 2. Feed orders into the job queue.
	for i, order := range orders {
		jobsChan <- Job{Index: i, Order: order}
	}
	close(jobsChan)

	// 3. Close the results channel once all workers finish.
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// 4. Fan-In: collect results and place them at their original index.
	results := make([]JobResult, n)
	for res := range resultsChan {
		results[res.Index] = res
	}

	return results, nil
}

// ProcessStream processes orders from a Go 1.23+ iter.Seq concurrently and
// returns results as a streaming iter.Seq[JobResult].
func (b *BatchProcessor) ProcessStream(ctx context.Context, orders iter.Seq[*domain.Order]) iter.Seq[JobResult] {
	return func(yield func(JobResult) bool) {
		if ctx == nil {
			ctx = context.Background()
		}

		jobsChan := make(chan Job, b.workerCount*2)
		resultsChan := make(chan JobResult, b.workerCount*2)

		var wg sync.WaitGroup
		for range b.workerCount {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for job := range jobsChan {
					if err := ctx.Err(); err != nil {
						resultsChan <- JobResult{Index: job.Index, Err: err}
						continue
					}
					receipt, err := b.engine.Total(ctx, job.Order)
					if err == nil {
						b.processedOrders.Add(1)
					}
					resultsChan <- JobResult{Index: job.Index, Receipt: receipt, Err: err}
				}
			}()
		}

		// Feeder goroutine: range over the input iterator and send jobs.
		go func() {
			idx := 0
			for order := range orders {
				if ctx.Err() != nil {
					break
				}
				jobsChan <- Job{Index: idx, Order: order}
				idx++
			}
			close(jobsChan)
			wg.Wait()
			close(resultsChan)
		}()

		for res := range resultsChan {
			if !yield(res) {
				return
			}
		}
	}
}
