package promotion_test

import (
	"testing"

	"github.com/hasanozgan/kata/discount-checkout/internal/domain"
	"github.com/hasanozgan/kata/discount-checkout/internal/promotion"
	"github.com/hasanozgan/kata/discount-checkout/pkg/money"
)

func TestPromotionEvaluate(t *testing.T) {
	guest := domain.Guest("g-1")
	member := domain.Member("m-gold", domain.TierGold)

	t.Run("PercentOffOver eligible", func(t *testing.T) {
		promo := promotion.PercentOffOver("10% over £50", 10, money.GBP("50.00"))

		orderEligible := domain.GBPOrder(guest).MustAddLine("STEAK", "Ribeye", money.GBP("12.00"), 5) // £60
		applied, ok := promo.Evaluate(orderEligible)

		if !ok {
			t.Fatalf("expected promotion to apply")
		}
		if applied.PromotionName != "10% over £50" {
			t.Errorf("got name %q, want '10%% over £50'", applied.PromotionName)
		}
		expectedSaving := money.GBP("6.00") // 10% of £60
		if !applied.Saving.Equals(expectedSaving) {
			t.Errorf("got saving %v, want %v", applied.Saving, expectedSaving)
		}
	})

	t.Run("PercentOffOver ineligible below threshold", func(t *testing.T) {
		promo := promotion.PercentOffOver("10% over £50", 10, money.GBP("50.00"))

		orderSmall := domain.GBPOrder(guest).MustAddLine("STEAK", "Ribeye", money.GBP("12.00"), 4) // £48
		_, ok := promo.Evaluate(orderSmall)

		if ok {
			t.Errorf("promotion should NOT apply for £48 order")
		}
	})

	t.Run("GoldFixedOff for Gold member", func(t *testing.T) {
		goldPromo := promotion.GoldFixedOff("Gold £20 Off", money.GBP("20.00"))

		orderGold := domain.GBPOrder(member).MustAddLine("TV", "Television", money.GBP("200.00"), 1)
		orderGuest := domain.GBPOrder(guest).MustAddLine("TV", "Television", money.GBP("200.00"), 1)

		applied, ok := goldPromo.Evaluate(orderGold)
		if !ok || !applied.Saving.Equals(money.GBP("20.00")) {
			t.Errorf("gold promo failed for gold member: ok=%v, applied=%+v", ok, applied)
		}

		_, guestOk := goldPromo.Evaluate(orderGuest)
		if guestOk {
			t.Errorf("gold promo should NOT apply for guest")
		}
	})

	t.Run("CouponFixedOff with spend threshold", func(t *testing.T) {
		couponPromo := promotion.CouponFixedOff("Voucher £5 off over £30", "SAVE5", money.GBP("5.00"), money.GBP("30.00"))

		orderWithCouponEligible := domain.GBPOrder(guest).
			MustAddLine("GROCERIES", "Groceries", money.GBP("35.00"), 1).
			MustRedeemCoupon("SAVE5")

		orderNoCoupon := domain.GBPOrder(guest).
			MustAddLine("GROCERIES", "Groceries", money.GBP("35.00"), 1)

		orderBelowSpend := domain.GBPOrder(guest).
			MustAddLine("GROCERIES", "Groceries", money.GBP("25.00"), 1).
			MustRedeemCoupon("SAVE5")

		if _, ok := couponPromo.Evaluate(orderWithCouponEligible); !ok {
			t.Errorf("expected coupon promo to apply when code redeemed and spend >= 30")
		}
		if _, ok := couponPromo.Evaluate(orderNoCoupon); ok {
			t.Errorf("coupon promo should not apply without coupon")
		}
		if _, ok := couponPromo.Evaluate(orderBelowSpend); ok {
			t.Errorf("coupon promo should not apply when spend < 30")
		}
	})
}
