package audit_test

import (
	"context"
	"log/slog"
	"sync"
	"testing"

	"github.com/netologist/go-discount-checkout/internal/audit"
)

func TestInMemoryLogger(t *testing.T) {
	logger := audit.NewInMemoryLogger()
	ctx := context.Background()

	logger.Log(ctx, slog.LevelInfo, "promotion evaluated", "promo", "10% Off", "saving", "6.00")
	logger.Log(ctx, slog.LevelWarn, "threshold missed", "promo", "Gold £20", "reason", "spend below £50")

	if logger.Count() != 2 {
		t.Fatalf("expected 2 events, got %d", logger.Count())
	}

	events := logger.Events()
	if events[0].Message != "promotion evaluated" {
		t.Errorf("got message %q, want 'promotion evaluated'", events[0].Message)
	}
	if events[0].Attributes["promo"] != "10% Off" {
		t.Errorf("expected promo attribute '10%% Off', got %v", events[0].Attributes["promo"])
	}
	if events[0].Attributes["saving"] != "6.00" {
		t.Errorf("expected saving attribute '6.00', got %v", events[0].Attributes["saving"])
	}

	if !logger.ContainsMessage("threshold missed") {
		t.Errorf("expected ContainsMessage('threshold missed') to be true")
	}

	// Filter by level
	warns := logger.FindByLevel(slog.LevelWarn)
	if len(warns) != 1 || warns[0].Message != "threshold missed" {
		t.Errorf("expected 1 warn event, got %+v", warns)
	}

	// Iterators
	iterCount := 0
	for e := range logger.EventsSeq() {
		if e.Message == "" {
			t.Errorf("expected non-empty message in iterator")
		}
		iterCount++
	}
	if iterCount != 2 {
		t.Errorf("expected 2 iterated events, got %d", iterCount)
	}

	logger.Clear()
	if logger.Count() != 0 {
		t.Errorf("expected 0 events after clear, got %d", logger.Count())
	}
}

func TestInMemoryLoggerConcurrent(t *testing.T) {
	logger := audit.NewInMemoryLogger()
	ctx := context.Background()

	var wg sync.WaitGroup
	workers := 50
	iterations := 100

	for i := range workers {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := range iterations {
				logger.Log(ctx, slog.LevelInfo, "concurrent log", "worker", workerID, "iter", j)
			}
		}(i)
	}

	// Concurrent readers reading Events and iterating
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = logger.Events()
			_ = logger.Count()
			_ = logger.ContainsMessage("concurrent log")
			for _ = range logger.EventsSeq() {
			}
		}()
	}

	wg.Wait()

	expectedTotal := workers * iterations
	if logger.Count() != expectedTotal {
		t.Errorf("expected %d total events, got %d", expectedTotal, logger.Count())
	}
}

func TestNoOpLogger(t *testing.T) {
	noop := audit.NoOp()
	// Should not panic
	noop.Log(context.Background(), slog.LevelError, "ignored", "key", "val")
}

func TestSlogLogger(t *testing.T) {
	slogL := audit.NewSlogLogger(nil)
	// Should not panic
	slogL.Log(context.Background(), slog.LevelInfo, "test message", "k", "v")
}
