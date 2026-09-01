// Package checkout orchestrates promotion evaluation over a basket, resolves
// exclusive-group conflicts, and produces the final Receipt.
package checkout

import (
	"strings"

	"github.com/netologist/go-discount-checkout/internal/audit"
	"github.com/netologist/go-discount-checkout/internal/promotion"
	"github.com/netologist/go-discount-checkout/pkg/money"
)

// ConflictPolicy is the strategy function that decides which of two competing
// promotions in the same exclusive group wins.
type ConflictPolicy func(a, b evaluatedCandidate) evaluatedCandidate

// BestSavingWins selects the promotion with the highest saving in the same group.
// Tie-breaker: lower Priority value (= higher precedence) → then alphabetical name.
func BestSavingWins(a, b evaluatedCandidate) evaluatedCandidate {
	savingCmp, _ := a.applied.Saving.Compare(b.applied.Saving)
	if savingCmp > 0 {
		return a
	}
	if savingCmp < 0 {
		return b
	}

	// Equal savings — lower priority value wins (higher precedence).
	if a.promo.Priority != b.promo.Priority {
		if a.promo.Priority < b.promo.Priority {
			return a
		}
		return b
	}

	// Deterministic final tie-break: alphabetical name.
	if strings.Compare(a.promo.Name, b.promo.Name) <= 0 {
		return a
	}
	return b
}

// Option is the functional option type for configuring a Checkout engine.
//
// Go Pattern: Rob Pike & Dave Cheney Functional Options Pattern.
// Parameters can grow without telescoping constructors or breaking API changes.
type Option func(*Checkout)

// WithPromotion adds a single promotion to the engine.
func WithPromotion(p promotion.Promotion) Option {
	return func(c *Checkout) {
		c.promotions = append(c.promotions, p)
	}
}

// WithPromotions adds multiple promotions to the engine in bulk.
func WithPromotions(promos ...promotion.Promotion) Option {
	return func(c *Checkout) {
		c.promotions = append(c.promotions, promos...)
	}
}

// WithLogger attaches a custom AuditLogger for structured event logging.
func WithLogger(logger audit.Logger) Option {
	return func(c *Checkout) {
		if logger != nil {
			c.logger = logger
		}
	}
}

// WithMaxSavingsCap sets a hard upper limit on the total savings any combination
// of promotions can provide.
func WithMaxSavingsCap(cap money.Money) Option {
	return func(c *Checkout) {
		c.maxSavingsCap = cap
	}
}

// WithConflictPolicy sets a custom conflict-resolution policy for exclusive groups.
func WithConflictPolicy(policy ConflictPolicy) Option {
	return func(c *Checkout) {
		if policy != nil {
			c.conflictPolicy = policy
		}
	}
}
