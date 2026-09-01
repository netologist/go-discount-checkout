package spec_test

import (
	"testing"

	"github.com/netologist/go-discount-checkout/internal/domain"
	"github.com/netologist/go-discount-checkout/internal/spec"
	"github.com/netologist/go-discount-checkout/pkg/money"
)

func orderWith(customer domain.Customer, sku string, price string, qty int) *domain.Order {
	o := domain.GBPOrder(customer)
	o.MustAddLine(sku, sku, money.GBP(price), qty)
	return o
}

func TestSpecifications(t *testing.T) {
	guest := domain.Guest("g-1")
	silver := domain.Member("m-silver", domain.TierSilver)
	gold := domain.Member("m-gold", domain.TierGold)

	t.Run("MinimumSpend", func(t *testing.T) {
		min50 := spec.NewMinimumSpend(money.GBP("50.00"))

		order49 := orderWith(guest, "X", "49.99", 1)
		order50 := orderWith(guest, "X", "50.00", 1)
		order60 := orderWith(guest, "X", "60.00", 1)

		if min50.IsSatisfiedBy(order49) {
			t.Errorf("49.99 should not meet 50.00 minimum spend")
		}
		if !min50.IsSatisfiedBy(order50) {
			t.Errorf("50.00 should meet 50.00 minimum spend")
		}
		if !min50.IsSatisfiedBy(order60) {
			t.Errorf("60.00 should meet 50.00 minimum spend")
		}
	})

	t.Run("MemberOnly", func(t *testing.T) {
		memberOnly := spec.NewMemberOnly()

		guestOrder := orderWith(guest, "X", "10.00", 1)
		memberOrder := orderWith(silver, "X", "10.00", 1)

		if memberOnly.IsSatisfiedBy(guestOrder) {
			t.Errorf("guest order should not satisfy MemberOnly")
		}
		if !memberOnly.IsSatisfiedBy(memberOrder) {
			t.Errorf("member order should satisfy MemberOnly")
		}
	})

	t.Run("MinimumTier", func(t *testing.T) {
		goldReq := spec.NewMinimumTier(domain.TierGold)

		guestOrder := orderWith(guest, "X", "10.00", 1)
		silverOrder := orderWith(silver, "X", "10.00", 1)
		goldOrder := orderWith(gold, "X", "10.00", 1)

		if goldReq.IsSatisfiedBy(guestOrder) {
			t.Errorf("guest should not satisfy Gold requirement")
		}
		if goldReq.IsSatisfiedBy(silverOrder) {
			t.Errorf("silver should not satisfy Gold requirement")
		}
		if !goldReq.IsSatisfiedBy(goldOrder) {
			t.Errorf("gold should satisfy Gold requirement")
		}
	})

	t.Run("ContainsSKU", func(t *testing.T) {
		specToothpaste := spec.NewContainsSKU("PASTE", 3)

		order1 := orderWith(guest, "PASTE", "1.50", 1)
		order3 := orderWith(guest, "PASTE", "1.50", 3)
		orderOther := orderWith(guest, "SOAP", "1.00", 5)

		if specToothpaste.IsSatisfiedBy(order1) {
			t.Errorf("qty=1 should not satisfy minQuantity=3")
		}
		if !specToothpaste.IsSatisfiedBy(order3) {
			t.Errorf("qty=3 should satisfy minQuantity=3")
		}
		if specToothpaste.IsSatisfiedBy(orderOther) {
			t.Errorf("other SKU should not satisfy PASTE spec")
		}
	})

	t.Run("HasCoupon", func(t *testing.T) {
		specCoupon := spec.NewHasCoupon("SAVE10")

		orderNoCoupon := orderWith(guest, "X", "20.00", 1)
		orderWithCoupon := orderWith(guest, "X", "20.00", 1)
		orderWithCoupon.MustRedeemCoupon("save10")

		if specCoupon.IsSatisfiedBy(orderNoCoupon) {
			t.Errorf("order without coupon should not satisfy HasCoupon")
		}
		if !specCoupon.IsSatisfiedBy(orderWithCoupon) {
			t.Errorf("order with coupon should satisfy HasCoupon")
		}
	})
}

func TestSpecificationCombinators(t *testing.T) {
	guest := domain.Guest("g-1")
	member := domain.Member("m-1", domain.TierStandard)

	min30 := spec.NewMinimumSpend(money.GBP("30.00"))
	memberOnly := spec.NewMemberOnly()

	t.Run("And combinator", func(t *testing.T) {
		both := spec.And[*domain.Order](memberOnly, min30)

		eligible := domain.GBPOrder(member).MustAddLine("X", "X", money.GBP("10.00"), 4) // £40, member
		tooSmall := domain.GBPOrder(member).MustAddLine("X", "X", money.GBP("10.00"), 2) // £20, member
		guestBig := domain.GBPOrder(guest).MustAddLine("X", "X", money.GBP("10.00"), 4)  // £40, guest

		if !both.IsSatisfiedBy(eligible) {
			t.Errorf("member + £40 should satisfy memberOnly AND min30")
		}
		if both.IsSatisfiedBy(tooSmall) {
			t.Errorf("member + £20 should NOT satisfy min30")
		}
		if both.IsSatisfiedBy(guestBig) {
			t.Errorf("guest + £40 should NOT satisfy memberOnly")
		}
	})

	t.Run("Or combinator", func(t *testing.T) {
		either := spec.Or[*domain.Order](memberOnly, min30)

		guestBig := domain.GBPOrder(guest).MustAddLine("X", "X", money.GBP("40.00"), 1)
		memberSmall := domain.GBPOrder(member).MustAddLine("X", "X", money.GBP("10.00"), 1)
		guestSmall := domain.GBPOrder(guest).MustAddLine("X", "X", money.GBP("10.00"), 1)

		if !either.IsSatisfiedBy(guestBig) {
			t.Errorf("guest + £40 satisfies min30, so OR should be true")
		}
		if !either.IsSatisfiedBy(memberSmall) {
			t.Errorf("member + £10 satisfies memberOnly, so OR should be true")
		}
		if either.IsSatisfiedBy(guestSmall) {
			t.Errorf("guest + £10 satisfies neither, so OR should be false")
		}
	})

	t.Run("Not combinator", func(t *testing.T) {
		notMember := spec.Not[*domain.Order](memberOnly)

		guestOrder := domain.GBPOrder(guest)
		memberOrder := domain.GBPOrder(member)

		if !notMember.IsSatisfiedBy(guestOrder) {
			t.Errorf("guest order should satisfy NOT memberOnly")
		}
		if notMember.IsSatisfiedBy(memberOrder) {
			t.Errorf("member order should NOT satisfy NOT memberOnly")
		}
	})

	t.Run("Always and Never", func(t *testing.T) {
		order := domain.GBPOrder(guest)
		if !spec.Always[*domain.Order]().IsSatisfiedBy(order) {
			t.Errorf("Always should always be true")
		}
		if spec.Never[*domain.Order]().IsSatisfiedBy(order) {
			t.Errorf("Never should always be false")
		}
	})
}
