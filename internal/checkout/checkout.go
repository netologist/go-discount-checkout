package checkout

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"

	"github.com/netologist/go-discount-checkout/internal/audit"
	"github.com/netologist/go-discount-checkout/internal/domain"
	"github.com/netologist/go-discount-checkout/internal/promotion"
	"github.com/netologist/go-discount-checkout/pkg/money"
)

var (
	// ErrNilOrder is returned when a nil order is passed to Total.
	ErrNilOrder = errors.New("checkout: order cannot be nil")
)

type evaluatedCandidate struct {
	promo   promotion.Promotion
	applied domain.AppliedDiscount
}

// Checkout is the main orchestrator that evaluates promotion rules over a basket.
//
// Pattern map:
//   - Dependency Inversion (DIP): depends only on Promotion and AuditLogger abstractions.
//   - Open/Closed (OCP): adding a new promotion = a new Promotion value; zero changes to Checkout.
//   - Single Responsibility (SRP): orchestrates evaluate → filter/stack → sum → receipt only.
//   - Thread-Safety (sync.RWMutex): safe for concurrent basket calculations and dynamic promotion registration.
//   - Modern Go (Go 1.21+): slices.SortStableFunc for type-safe, reflection-free sorting.
type Checkout struct {
	mu             sync.RWMutex
	promotions     []promotion.Promotion
	logger         audit.Logger
	maxSavingsCap  money.Money
	conflictPolicy ConflictPolicy
}

// New initialises a Checkout engine via functional options.
//
// Go Pattern: Functional Options Constructor.
func New(opts ...Option) *Checkout {
	c := &Checkout{
		promotions:     make([]promotion.Promotion, 0),
		logger:         audit.NoOp(),
		maxSavingsCap:  money.Money{},
		conflictPolicy: BestSavingWins,
	}

	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}

	return c
}

// NoPromotions creates an empty Checkout engine (no promotions registered).
func NoPromotions(opts ...Option) *Checkout {
	return New(opts...)
}

// With is a convenience constructor that pre-loads a set of promotions.
func With(promotions ...promotion.Promotion) *Checkout {
	return New(WithPromotions(promotions...))
}

// AddPromotions registers new promotions at runtime in a thread-safe manner.
func (c *Checkout) AddPromotions(promos ...promotion.Promotion) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.promotions = append(c.promotions, promos...)
}

// Promotions returns a defensive copy of the currently registered promotions.
func (c *Checkout) Promotions() []promotion.Promotion {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return slices.Clone(c.promotions)
}

// Total evaluates all promotion rules against the order and returns the final Receipt.
//
// Go Pattern: Context as first parameter & explicit error return (domain.Receipt, error).
func (c *Checkout) Total(ctx context.Context, order *domain.Order) (domain.Receipt, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	// 1. Thread-safe snapshot of engine state.
	c.mu.RLock()
	promos := slices.Clone(c.promotions)
	logger := c.logger
	maxSavingsCap := c.maxSavingsCap
	conflictPolicy := c.conflictPolicy
	c.mu.RUnlock()

	if conflictPolicy == nil {
		conflictPolicy = BestSavingWins
	}
	if logger == nil {
		logger = audit.NoOp()
	}

	// 2. Fail fast on cancelled / timed-out context.
	if err := ctx.Err(); err != nil {
		logger.Log(ctx, slog.LevelError, "checkout cancelled or timed out", "err", err)
		return domain.Receipt{}, fmt.Errorf("checkout aborted: %w", err)
	}

	if order == nil {
		return domain.Receipt{}, ErrNilOrder
	}

	if order.IsEmpty() {
		logger.Log(ctx, slog.LevelInfo, "empty order checked out", "customer", order.Customer().ID)
		return domain.EmptyReceipt(order.Currency()), nil
	}

	subtotal := order.Subtotal()
	logger.Log(ctx, slog.LevelDebug, "starting promotion evaluation",
		"customer", order.Customer().ID,
		"subtotal", subtotal.String(),
		"lines", len(order.Lines()),
	)

	// 3. Evaluate all promotions (eligibility + saving calculation).
	eligible := make([]evaluatedCandidate, 0, len(promos))
	for _, p := range promos {
		if applied, ok := p.Evaluate(order); ok {
			eligible = append(eligible, evaluatedCandidate{promo: p, applied: applied})
		}
	}

	// 4. Separate stackable promotions; within each exclusive group pick the winner.
	survivors := make([]evaluatedCandidate, 0, len(eligible))
	bestPerGroup := make(map[string]evaluatedCandidate)

	for _, cand := range eligible {
		if cand.promo.IsStackable() {
			survivors = append(survivors, cand)
		} else {
			grp := cand.promo.ExclusiveGroup
			if existing, exists := bestPerGroup[grp]; exists {
				bestPerGroup[grp] = conflictPolicy(existing, cand)
			} else {
				bestPerGroup[grp] = cand
			}
		}
	}

	for _, winner := range bestPerGroup {
		survivors = append(survivors, winner)
	}

	// 5. Deterministic audit ordering: Priority (ascending) → Name (alphabetical).
	// Go 1.21+ slices.SortStableFunc: type-safe, reflection-free.
	slices.SortStableFunc(survivors, func(a, b evaluatedCandidate) int {
		if a.promo.Priority != b.promo.Priority {
			return cmp.Compare(a.promo.Priority, b.promo.Priority)
		}
		return strings.Compare(a.promo.Name, b.promo.Name)
	})

	// 6. Sum savings and apply caps (subtotal cap + optional max savings cap).
	appliedDiscounts := make([]domain.AppliedDiscount, 0, len(survivors))
	totalSavings := money.Zero(order.Currency())

	for _, s := range survivors {
		appliedDiscounts = append(appliedDiscounts, s.applied)
		totalSavings, _ = totalSavings.Add(s.applied.Saving)

		logger.Log(ctx, slog.LevelInfo, "applied promotion",
			"promo", s.applied.PromotionName,
			"saving", s.applied.Saving.String(),
			"group", s.applied.Group,
		)
	}

	// Total savings cannot exceed the order subtotal.
	totalSavings = totalSavings.Min(subtotal)

	// Apply the optional global savings cap if configured.
	if !maxSavingsCap.IsZero() && maxSavingsCap.Currency() == order.Currency() {
		totalSavings = totalSavings.Min(maxSavingsCap)
	}

	payable, _ := subtotal.Sub(totalSavings)

	receipt := domain.Receipt{
		Lines:        order.Lines(),
		Subtotal:     subtotal,
		Discounts:    appliedDiscounts,
		TotalSavings: totalSavings,
		Payable:      payable,
	}

	logger.Log(ctx, slog.LevelInfo, "checkout completed",
		"subtotal", subtotal.String(),
		"savings", totalSavings.String(),
		"payable", payable.String(),
		"discounts_count", len(appliedDiscounts),
	)

	return receipt, nil
}

// MustTotal calls Total with context.Background() and panics on error.
// Go Pattern: convenient for tests and synchronous single-call scenarios.
func (c *Checkout) MustTotal(order *domain.Order) domain.Receipt {
	receipt, err := c.Total(context.Background(), order)
	if err != nil {
		panic(err)
	}
	return receipt
}
