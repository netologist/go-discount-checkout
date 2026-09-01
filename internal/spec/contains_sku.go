package spec

import (
	"cmp"
	"strings"

	"github.com/hasanozgan/kata/discount-checkout/internal/domain"
)

// ContainsSKU is a Specification that requires at least a minimum quantity of a
// given SKU to be present in the basket.
// Example: a "Buy 3 Get 1 Free toothpaste" rule requires ContainsSKU("PASTE", 3).
type ContainsSKU struct {
	SKU         string
	MinQuantity int
}

// NewContainsSKU creates a ContainsSKU specification.
func NewContainsSKU(sku string, minQuantity int) ContainsSKU {
	return ContainsSKU{
		SKU:         cmp.Or(strings.TrimSpace(sku), ""),
		MinQuantity: max(minQuantity, 1),
	}
}

// Contains is a convenience constructor that checks for at least 1 unit of the SKU.
func Contains(sku string) ContainsSKU {
	return NewContainsSKU(sku, 1)
}

// IsSatisfiedBy checks that the basket contains at least MinQuantity of the SKU.
func (c ContainsSKU) IsSatisfiedBy(order *domain.Order) bool {
	if order == nil {
		return false
	}
	return order.QuantityOf(c.SKU) >= c.MinQuantity
}
