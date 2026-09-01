// Package discount implements the Strategy and Composite design patterns to answer
// the question "HOW MUCH (what) does a promotion save?"
//
// Every discount type is a pure function — it only computes a saving amount (Money)
// and never mutates the order or any promotion state.
package discount

import (
	"github.com/hasanozgan/kata/discount-checkout/internal/domain"
	"github.com/hasanozgan/kata/discount-checkout/pkg/money"
)

// Discount is the Strategy interface that computes the saving amount for an order.
//
// Go Pattern: Single-Method Strategy Interface ("Accept interfaces, return structs").
type Discount interface {
	// Apply returns the saving amount for the order (returns Zero if not applicable).
	Apply(order *domain.Order) money.Money
}

// Func adapts a plain function to the Discount interface.
type Func func(order *domain.Order) money.Money

// Apply invokes the underlying function.
func (f Func) Apply(order *domain.Order) money.Money {
	return f(order)
}
