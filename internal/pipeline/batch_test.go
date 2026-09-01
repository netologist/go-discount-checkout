package pipeline_test

import (
	"context"
	"fmt"
	"iter"
	"sync"
	"testing"

	"github.com/hasanozgan/kata/discount-checkout/internal/checkout"
	"github.com/hasanozgan/kata/discount-checkout/internal/domain"
	"github.com/hasanozgan/kata/discount-checkout/internal/pipeline"
	"github.com/hasanozgan/kata/discount-checkout/internal/promotion"
	"github.com/hasanozgan/kata/discount-checkout/pkg/money"
)

func TestBatchProcessor_ProcessBatch(t *testing.T) {
	engine := checkout.With(
		promotion.PercentOffOver("10% over £50", 10, money.GBP("50.00")),
		promotion.MemberPercentOff("Clubcard 5%", 5),
	)

	processor := pipeline.NewBatchProcessor(engine, 4)

	// Create 50 sample orders
	count := 50
	orders := make([]*domain.Order, count)
	for i := range count {
		customer := domain.Guest(fmt.Sprintf("guest-%d", i))
		if i%2 == 0 {
			customer = domain.Member(fmt.Sprintf("member-%d", i), domain.TierSilver)
		}

		order := domain.GBPOrder(customer)
		order.MustAddLine("STEAK", "Ribeye", money.GBP("15.00"), i%5+1) // 15, 30, 45, 60, 75
		orders[i] = order
	}

	ctx := context.Background()
	results, err := processor.ProcessBatch(ctx, orders)

	if err != nil {
		t.Fatalf("unexpected error during batch processing: %v", err)
	}

	if len(results) != count {
		t.Fatalf("expected %d results, got %d", count, len(results))
	}

	for i, res := range results {
		if res.Index != i {
			t.Errorf("result index mismatch: got %d, want %d", res.Index, i)
		}
		if res.Err != nil {
			t.Errorf("order %d failed with error: %v", i, res.Err)
		}
		if res.Receipt.Subtotal.IsZero() {
			t.Errorf("order %d subtotal should not be zero", i)
		}
	}

	if processor.ProcessedCount() != int64(count) {
		t.Errorf("expected ProcessedCount = %d, got %d", count, processor.ProcessedCount())
	}
}

func TestBatchProcessor_ProcessStream(t *testing.T) {
	engine := checkout.With(
		promotion.PercentOffOver("10% over £50", 10, money.GBP("50.00")),
	)
	processor := pipeline.NewBatchProcessor(engine, 4)

	orderList := []*domain.Order{
		domain.GBPOrder(domain.Guest("g-1")).MustAddLine("A", "Item A", money.GBP("10.00"), 1),
		domain.GBPOrder(domain.Guest("g-2")).MustAddLine("B", "Item B", money.GBP("20.00"), 2),
		domain.GBPOrder(domain.Guest("g-3")).MustAddLine("C", "Item C", money.GBP("30.00"), 3),
	}

	orderSeq := func(yield func(*domain.Order) bool) {
		for _, o := range orderList {
			if !yield(o) {
				return
			}
		}
	}

	ctx := context.Background()
	streamResults := make([]pipeline.JobResult, 0)
	for res := range processor.ProcessStream(ctx, iter.Seq[*domain.Order](orderSeq)) {
		streamResults = append(streamResults, res)
	}

	if len(streamResults) != 3 {
		t.Fatalf("expected 3 stream results, got %d", len(streamResults))
	}
}

func TestBatchProcessor_ContextCancellation(t *testing.T) {
	engine := checkout.NoPromotions()
	processor := pipeline.NewBatchProcessor(engine, 2)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	orders := []*domain.Order{
		domain.GBPOrder(domain.Guest("g-1")).MustAddLine("X", "Item", money.GBP("10.00"), 1),
	}

	_, err := processor.ProcessBatch(ctx, orders)
	if err == nil {
		t.Errorf("expected cancellation error, got nil")
	}
}

func TestBatchProcessor_EmptyBatch(t *testing.T) {
	engine := checkout.NoPromotions()
	processor := pipeline.NewBatchProcessor(engine, 4)

	results, err := processor.ProcessBatch(context.Background(), []*domain.Order{})
	if err != nil {
		t.Fatalf("unexpected error on empty batch: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestBatchProcessor_ConcurrentProcessBatch(t *testing.T) {
	engine := checkout.With(
		promotion.PercentOffOver("10% over £50", 10, money.GBP("50.00")),
	)
	processor := pipeline.NewBatchProcessor(engine, 4)

	var wg sync.WaitGroup
	batches := 10
	ordersPerBatch := 10

	for b := range batches {
		wg.Add(1)
		go func(batchID int) {
			defer wg.Done()
			orders := make([]*domain.Order, ordersPerBatch)
			for i := range ordersPerBatch {
				orders[i] = domain.GBPOrder(domain.Guest(fmt.Sprintf("guest-%d-%d", batchID, i))).
					MustAddLine("X", "Item", money.GBP("15.00"), (i%3)+1)
			}
			res, err := processor.ProcessBatch(context.Background(), orders)
			if err != nil {
				t.Errorf("batch %d failed: %v", batchID, err)
			}
			if len(res) != ordersPerBatch {
				t.Errorf("batch %d expected %d results, got %d", batchID, ordersPerBatch, len(res))
			}
		}(b)
	}

	wg.Wait()
}

func BenchmarkBatchProcessor_ProcessBatch(b *testing.B) {
	engine := checkout.With(
		promotion.PercentOffOver("10% over £50", 10, money.GBP("50.00")),
	)
	processor := pipeline.NewBatchProcessor(engine, 8)

	orders := make([]*domain.Order, 100)
	for i := range 100 {
		orders[i] = domain.GBPOrder(domain.Guest("g")).
			MustAddLine("STEAK", "Steak", money.GBP("12.00"), 5)
	}

	ctx := context.Background()
	for b.Loop() {
		_, _ = processor.ProcessBatch(ctx, orders)
	}
}
