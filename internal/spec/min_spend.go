package spec

import (
	"github.com/hasanozgan/kata/discount-checkout/internal/domain"
	"github.com/hasanozgan/kata/discount-checkout/pkg/money"
)

// MinimumSpend is a Specification that requires the gross basket subtotal to be
// at or above a given threshold.
// Example: "10% off orders of £50 or more" uses MinimumSpend(GBP("50.00")).
type MinimumSpend struct {
	Threshold money.Money
}

// NewMinimumSpend creates a MinimumSpend specification.
func NewMinimumSpend(threshold money.Money) MinimumSpend {
	return MinimumSpend{Threshold: threshold}
}

// IsSatisfiedBy checks whether the order subtotal meets the threshold.
func (s MinimumSpend) IsSatisfiedBy(order *domain.Order) bool {
	if order == nil {
		return false
	}
	return order.Subtotal().AtLeast(s.Threshold)
}
