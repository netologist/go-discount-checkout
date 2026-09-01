package spec

import (
	"strings"

	"github.com/hasanozgan/kata/discount-checkout/internal/domain"
)

// HasCoupon is an activation Specification that requires the customer to have
// applied a specific coupon or promotion code to the basket.
//
// Go Pattern: Specification Composition — coupons are not a separate discount
// algorithm; they are pure Specification rules that check whether the code was
// entered, composed with the actual discount formula via Promotion.
type HasCoupon struct {
	Code string
}

// NewHasCoupon creates a HasCoupon specification for the given code.
func NewHasCoupon(code string) HasCoupon {
	return HasCoupon{Code: strings.ToUpper(strings.TrimSpace(code))}
}

// IsSatisfiedBy checks whether the coupon code is redeemed on the order.
func (h HasCoupon) IsSatisfiedBy(order *domain.Order) bool {
	if order == nil {
		return false
	}
	return order.HasCoupon(h.Code)
}
