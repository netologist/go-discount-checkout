package discount

import (
	"github.com/hasanozgan/kata/discount-checkout/internal/domain"
	"github.com/hasanozgan/kata/discount-checkout/pkg/money"
)

// PercentOff applies a percentage discount to the gross subtotal of the entire order.
type PercentOff struct {
	Percent int
}

// NewPercentOff creates a PercentOff strategy (clamped to 0–100).
func NewPercentOff(percent int) PercentOff {
	return PercentOff{Percent: min(max(percent, 0), 100)}
}

// Apply returns the given percentage of the order subtotal with retail rounding.
func (p PercentOff) Apply(order *domain.Order) money.Money {
	if order == nil || order.IsEmpty() || p.Percent <= 0 {
		return money.ZeroGBP()
	}
	return order.Subtotal().Percent(p.Percent)
}
