package discount

import (
	"github.com/netologist/go-discount-checkout/internal/domain"
	"github.com/netologist/go-discount-checkout/pkg/money"
)

// FixedAmountOff applies a fixed cash discount to the basket (e.g. £5 off or £20 off).
//
// The saving is capped at the order subtotal so the customer's bill can never
// go negative.
type FixedAmountOff struct {
	Amount money.Money
}

// NewFixedAmountOff creates a FixedAmountOff strategy.
func NewFixedAmountOff(amount money.Money) FixedAmountOff {
	return FixedAmountOff{Amount: amount}
}

// Apply returns the fixed discount amount, capped (min) at the order subtotal.
func (f FixedAmountOff) Apply(order *domain.Order) money.Money {
	if order == nil || order.IsEmpty() || f.Amount.IsZero() || f.Amount.IsNegative() {
		return money.ZeroGBP()
	}
	return f.Amount.Min(order.Subtotal())
}
