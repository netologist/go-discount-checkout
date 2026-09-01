package promotion

import (
	"fmt"

	"github.com/netologist/go-discount-checkout/internal/discount"
	"github.com/netologist/go-discount-checkout/internal/domain"
	"github.com/netologist/go-discount-checkout/internal/spec"
	"github.com/netologist/go-discount-checkout/pkg/money"
)

// PercentOffOver applies a percentage discount to the whole order when the basket
// subtotal meets or exceeds a threshold.
// Example: "10% off orders over £50".
func PercentOffOver(name string, percent int, threshold money.Money) Promotion {
	return New(
		name,
		spec.NewMinimumSpend(threshold),
		discount.NewPercentOff(percent),
	)
}

// MemberPercentOff applies a percentage discount exclusively for loyalty-card members.
// Example: "Clubcard 5% — 5% off the whole basket for members".
func MemberPercentOff(name string, percent int) Promotion {
	return New(
		name,
		spec.NewMemberOnly(),
		discount.NewPercentOff(percent),
	)
}

// GoldFixedOff applies a fixed cash discount exclusively for Gold-tier VIP members.
// Example: "Gold £20 Off — £20 off for Gold members".
func GoldFixedOff(name string, amount money.Money) Promotion {
	return New(
		name,
		spec.And[*domain.Order](
			spec.NewMemberOnly(),
			spec.NewMinimumTier(domain.TierGold),
		),
		discount.NewFixedAmountOff(amount),
	)
}

// BOGOF creates a Buy 2 Get 1 Free (3-for-2) promotion for a specific SKU.
func BOGOF(name string, sku string) Promotion {
	return New(
		name,
		spec.Contains(sku),
		discount.BuyTwoGetOneFree(sku),
	)
}

// CouponFixedOff creates a fixed cash discount triggered by a coupon code, with an
// optional minimum spend requirement.
func CouponFixedOff(name string, couponCode string, amount money.Money, minSpend money.Money) Promotion {
	var when spec.Specification[*domain.Order]
	if !minSpend.IsZero() {
		when = spec.And[*domain.Order](
			spec.NewHasCoupon(couponCode),
			spec.NewMinimumSpend(minSpend),
		)
	} else {
		when = spec.NewHasCoupon(couponCode)
	}

	return New(name, when, discount.NewFixedAmountOff(amount))
}

// CouponPercentOff creates a percentage discount triggered by a coupon code, with an
// optional minimum spend requirement.
func CouponPercentOff(name string, couponCode string, percent int, minSpend money.Money) Promotion {
	var when spec.Specification[*domain.Order]
	if !minSpend.IsZero() {
		when = spec.And[*domain.Order](
			spec.NewHasCoupon(couponCode),
			spec.NewMinimumSpend(minSpend),
		)
	} else {
		when = spec.NewHasCoupon(couponCode)
	}

	return New(name, when, discount.NewPercentOff(percent))
}

// Always creates a promotion that applies unconditionally to every basket.
func Always(name string, disc discount.Discount) Promotion {
	return New(name, spec.Always[*domain.Order](), disc)
}

// Exclusive creates a promotion that belongs to an exclusive group; within the
// group, only the best saving wins (resolved by the Checkout ConflictPolicy).
func Exclusive(
	name string,
	when spec.Specification[*domain.Order],
	what discount.Discount,
	exclusiveGroup string,
	priority int,
) Promotion {
	return NewWithPolicy(name, when, what, priority, exclusiveGroup)
}

// CategoryExclusive is a helper for "best deal wins within a product category" promotions.
func CategoryExclusive(name string, category string, when spec.Specification[*domain.Order], what discount.Discount) Promotion {
	group := fmt.Sprintf("category:%s", category)
	return Exclusive(name, when, what, group, 0)
}
