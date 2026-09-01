package discount

import (
	"slices"

	"github.com/hasanozgan/kata/discount-checkout/internal/domain"
	"github.com/hasanozgan/kata/discount-checkout/pkg/money"
)

// StackedDiscount is a Composite Pattern implementation that combines multiple
// discount strategies into a single Discount.
//
// The savings of all parts are summed and capped at the order subtotal.
type StackedDiscount struct {
	parts []Discount
}

// Stack combines the given discount strategies into a single composite discount.
func Stack(discounts ...Discount) StackedDiscount {
	filtered := make([]Discount, 0, len(discounts))
	for _, d := range discounts {
		if d != nil {
			filtered = append(filtered, d)
		}
	}
	return StackedDiscount{parts: slices.Clone(filtered)}
}

// Apply sums all component savings and caps the result at the order subtotal.
func (s StackedDiscount) Apply(order *domain.Order) money.Money {
	if order == nil || order.IsEmpty() {
		return money.ZeroGBP()
	}

	totalSaving := money.Zero(order.Currency())
	for _, part := range s.parts {
		if part != nil {
			saving := part.Apply(order)
			totalSaving, _ = totalSaving.Add(saving)
		}
	}

	return totalSaving.Min(order.Subtotal())
}
