package discount

import (
	"cmp"
	"strings"

	"github.com/netologist/go-discount-checkout/internal/domain"
	"github.com/netologist/go-discount-checkout/pkg/money"
)

// PercentOffSKU applies a percentage discount to the line total of a specific SKU.
type PercentOffSKU struct {
	SKU     string
	Percent int
}

// NewPercentOffSKU creates a PercentOffSKU strategy (clamped to 0–100).
func NewPercentOffSKU(sku string, percent int) PercentOffSKU {
	return PercentOffSKU{
		SKU:     cmp.Or(strings.TrimSpace(sku), ""),
		Percent: min(max(percent, 0), 100),
	}
}

// Apply computes the discount against the relevant SKU's line total.
func (p PercentOffSKU) Apply(order *domain.Order) money.Money {
	if order == nil || order.IsEmpty() || p.Percent <= 0 {
		return money.ZeroGBP()
	}
	line, found := order.FindBySKU(p.SKU)
	if !found {
		return money.Zero(order.Currency())
	}
	return line.LineTotal().Percent(p.Percent)
}
