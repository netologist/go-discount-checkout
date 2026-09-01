package discount

import (
	"cmp"
	"strings"

	"github.com/hasanozgan/kata/discount-checkout/internal/domain"
	"github.com/hasanozgan/kata/discount-checkout/pkg/money"
)

// BOGOF computes Buy X Get Y Free (multi-buy) discounts.
//
// Example — Buy 2 Get 1 Free (pay for 2, get 3): PayFor: 2, FreePerGroup: 1.
// With 3 items: 1 free. With 6 items: 2 free.
type BOGOF struct {
	SKU          string
	PayFor       int
	FreePerGroup int
}

// NewBOGOF creates a generic Buy X Get Y Free strategy.
func NewBOGOF(sku string, payFor int, freePerGroup int) BOGOF {
	return BOGOF{
		SKU:          cmp.Or(strings.TrimSpace(sku), ""),
		PayFor:       max(payFor, 1),
		FreePerGroup: max(freePerGroup, 1),
	}
}

// BuyTwoGetOneFree creates the classic 3-for-2 (pay for 2, get 3) strategy.
func BuyTwoGetOneFree(sku string) BOGOF {
	return NewBOGOF(sku, 2, 1)
}

// Apply computes the number of free units based on the basket quantity and returns the saving.
func (b BOGOF) Apply(order *domain.Order) money.Money {
	if order == nil || order.IsEmpty() {
		return money.ZeroGBP()
	}

	qty := order.QuantityOf(b.SKU)
	groupSize := b.PayFor + b.FreePerGroup
	if groupSize <= 0 || qty < groupSize {
		return money.Zero(order.Currency())
	}

	freeUnits := (qty / groupSize) * b.FreePerGroup
	if freeUnits <= 0 {
		return money.Zero(order.Currency())
	}

	line, found := order.FindBySKU(b.SKU)
	if !found {
		return money.Zero(order.Currency())
	}

	return line.UnitPrice.Mul(freeUnits)
}
