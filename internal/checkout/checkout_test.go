package checkout_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/netologist/go-discount-checkout/internal/audit"
	"github.com/netologist/go-discount-checkout/internal/checkout"
	"github.com/netologist/go-discount-checkout/internal/discount"
	"github.com/netologist/go-discount-checkout/internal/domain"
	"github.com/netologist/go-discount-checkout/internal/promotion"
	"github.com/netologist/go-discount-checkout/internal/spec"
	"github.com/netologist/go-discount-checkout/pkg/money"
)

func TestCheckoutScenarios(t *testing.T) {
	guest := domain.Guest("g-1")
	member := domain.Member("m-silver", domain.TierSilver)
	gold := domain.Member("m-gold", domain.TierGold)

	t.Run("should return zero for empty order", func(t *testing.T) {
		engine := checkout.NoPromotions()
		receipt := engine.MustTotal(domain.GBPOrder(guest))

		if !receipt.Payable.Equals(money.ZeroGBP()) {
			t.Errorf("expected payable 0.00, got %v", receipt.Payable)
		}
		if len(receipt.Discounts) != 0 {
			t.Errorf("expected 0 discounts, got %d", len(receipt.Discounts))
		}
	})

	t.Run("should charge full subtotal when no promotions registered", func(t *testing.T) {
		order := domain.GBPOrder(guest).
			MustAddLine("BEANS", "Baked Beans", money.GBP("0.65"), 2) // £1.30

		receipt := checkout.NoPromotions().MustTotal(order)

		if !receipt.Subtotal.Equals(money.GBP("1.30")) {
			t.Errorf("got subtotal %v, want 1.30", receipt.Subtotal)
		}
		if !receipt.Payable.Equals(money.GBP("1.30")) {
			t.Errorf("got payable %v, want 1.30", receipt.Payable)
		}
		if !receipt.TotalSavings.Equals(money.ZeroGBP()) {
			t.Errorf("got savings %v, want 0.00", receipt.TotalSavings)
		}
	})

	t.Run("should apply percent off when minimum spend threshold is met", func(t *testing.T) {
		// 5 x £12 steak = £60 -> 10% off (£6.00) -> pay £54.00
		order := domain.GBPOrder(guest).
			MustAddLine("STEAK", "Ribeye", money.GBP("12.00"), 5)

		engine := checkout.With(
			promotion.PercentOffOver("10% over £50", 10, money.GBP("50.00")),
		)

		receipt := engine.MustTotal(order)

		if !receipt.Subtotal.Equals(money.GBP("60.00")) {
			t.Errorf("got subtotal %v, want 60.00", receipt.Subtotal)
		}
		if !receipt.TotalSavings.Equals(money.GBP("6.00")) {
			t.Errorf("got savings %v, want 6.00", receipt.TotalSavings)
		}
		if !receipt.Payable.Equals(money.GBP("54.00")) {
			t.Errorf("got payable %v, want 54.00", receipt.Payable)
		}
		if len(receipt.Discounts) != 1 || receipt.Discounts[0].PromotionName != "10% over £50" {
			t.Errorf("unexpected discounts: %+v", receipt.Discounts)
		}
	})

	t.Run("should not apply percent off below minimum spend threshold", func(t *testing.T) {
		// 4 x £12 steak = £48 (< £50) -> no discount
		order := domain.GBPOrder(guest).
			MustAddLine("STEAK", "Ribeye", money.GBP("12.00"), 4)

		engine := checkout.With(
			promotion.PercentOffOver("10% over £50", 10, money.GBP("50.00")),
		)

		receipt := engine.MustTotal(order)

		if !receipt.Payable.Equals(money.GBP("48.00")) {
			t.Errorf("got payable %v, want 48.00", receipt.Payable)
		}
		if len(receipt.Discounts) != 0 {
			t.Errorf("expected no discount applied, got %d", len(receipt.Discounts))
		}
	})

	t.Run("should apply discount exactly at threshold", func(t *testing.T) {
		// 2 x £25 = £50 -> exactly £50 -> 10% off = £5.00
		order := domain.GBPOrder(guest).
			MustAddLine("X", "Item", money.GBP("25.00"), 2)

		engine := checkout.With(
			promotion.PercentOffOver("10% over £50", 10, money.GBP("50.00")),
		)

		receipt := engine.MustTotal(order)
		if !receipt.TotalSavings.Equals(money.GBP("5.00")) {
			t.Errorf("got savings %v, want 5.00", receipt.TotalSavings)
		}
	})

	t.Run("should apply member only discount for clubcard members", func(t *testing.T) {
		memberOrder := domain.GBPOrder(member).
			MustAddLine("MILK", "Milk", money.GBP("1.00"), 10) // £10.00 -> pay £9.50
		guestOrder := domain.GBPOrder(guest).
			MustAddLine("MILK", "Milk", money.GBP("1.00"), 10) // £10.00 -> pay £10.00

		engine := checkout.With(
			promotion.MemberPercentOff("Clubcard 5%", 5),
		)

		memberReceipt := engine.MustTotal(memberOrder)
		guestReceipt := engine.MustTotal(guestOrder)

		if !memberReceipt.Payable.Equals(money.GBP("9.50")) {
			t.Errorf("member payable = %v, want 9.50", memberReceipt.Payable)
		}
		if !guestReceipt.Payable.Equals(money.GBP("10.00")) {
			t.Errorf("guest payable = %v, want 10.00", guestReceipt.Payable)
		}
	})

	t.Run("should apply BOGOF on SKU", func(t *testing.T) {
		// 3 toothpaste @ £1.50 = £4.50 -> 1 free (£1.50) -> pay £3.00
		order := domain.GBPOrder(guest).
			MustAddLine("PASTE", "Toothpaste", money.GBP("1.50"), 3)

		engine := checkout.With(
			promotion.BOGOF("BOGOF Toothpaste", "PASTE"),
		)

		receipt := engine.MustTotal(order)

		if !receipt.TotalSavings.Equals(money.GBP("1.50")) {
			t.Errorf("got savings %v, want 1.50", receipt.TotalSavings)
		}
		if !receipt.Payable.Equals(money.GBP("3.00")) {
			t.Errorf("got payable %v, want 3.00", receipt.Payable)
		}
	})

	t.Run("should stack BOGOF and minimum spend percentage discount", func(t *testing.T) {
		// 6 paste @ 1.50 = 9.00 -> BOGOF free 2 (£3.00)
		// 5 steak @ 12.00 = 60.00
		// Subtotal = 69.00
		// 10% of 69.00 = 6.90
		// Total savings = 3.00 + 6.90 = 9.90 -> Payable = 59.10
		order := domain.GBPOrder(guest).
			MustAddLine("PASTE", "Toothpaste", money.GBP("1.50"), 6).
			MustAddLine("STEAK", "Ribeye", money.GBP("12.00"), 5)

		engine := checkout.With(
			promotion.BOGOF("BOGOF Toothpaste", "PASTE"),
			promotion.PercentOffOver("10% over £50", 10, money.GBP("50.00")),
		)

		receipt := engine.MustTotal(order)

		if !receipt.Subtotal.Equals(money.GBP("69.00")) {
			t.Errorf("got subtotal %v, want 69.00", receipt.Subtotal)
		}
		if !receipt.TotalSavings.Equals(money.GBP("9.90")) {
			t.Errorf("got savings %v, want 9.90", receipt.TotalSavings)
		}
		if !receipt.Payable.Equals(money.GBP("59.10")) {
			t.Errorf("got payable %v, want 59.10", receipt.Payable)
		}
		if len(receipt.Discounts) != 2 {
			t.Errorf("expected 2 applied discounts, got %d", len(receipt.Discounts))
		}
	})

	t.Run("should require Gold member tier for fixed off", func(t *testing.T) {
		goldOrder := domain.GBPOrder(gold).
			MustAddLine("TV", "Television", money.GBP("200.00"), 1)
		silverOrder := domain.GBPOrder(member).
			MustAddLine("TV", "Television", money.GBP("200.00"), 1)

		engine := checkout.With(
			promotion.GoldFixedOff("Gold £20 off", money.GBP("20.00")),
		)

		if !engine.MustTotal(goldOrder).Payable.Equals(money.GBP("180.00")) {
			t.Errorf("gold payable should be 180.00")
		}
		if !engine.MustTotal(silverOrder).Payable.Equals(money.GBP("200.00")) {
			t.Errorf("silver payable should be 200.00")
		}
	})

	t.Run("should compose specifications with And", func(t *testing.T) {
		// Member AND MinimumSpend £30 -> 15% off
		promo := promotion.New(
			"Member Weekend",
			spec.And[*domain.Order](
				spec.NewMemberOnly(),
				spec.NewMinimumSpend(money.GBP("30.00")),
			),
			discount.NewPercentOff(15),
		)

		eligible := domain.GBPOrder(member).MustAddLine("X", "Item", money.GBP("10.00"), 4) // £40 -> 15% = £6 -> pay £34
		tooSmall := domain.GBPOrder(member).MustAddLine("X", "Item", money.GBP("10.00"), 2) // £20 -> pay £20
		guestBig := domain.GBPOrder(guest).MustAddLine("X", "Item", money.GBP("10.00"), 4)  // £40 -> pay £40

		engine := checkout.With(promo)

		if !engine.MustTotal(eligible).Payable.Equals(money.GBP("34.00")) {
			t.Errorf("eligible payable should be 34.00")
		}
		if !engine.MustTotal(tooSmall).Payable.Equals(money.GBP("20.00")) {
			t.Errorf("tooSmall payable should be 20.00")
		}
		if !engine.MustTotal(guestBig).Payable.Equals(money.GBP("40.00")) {
			t.Errorf("guestBig payable should be 40.00")
		}
	})

	t.Run("should use composite discount strategy", func(t *testing.T) {
		// Bundle deal: 5% + £2 fixed off
		bundlePromo := promotion.New(
			"Bundle Deal",
			spec.Always[*domain.Order](),
			discount.Stack(
				discount.NewPercentOff(5),
				discount.NewFixedAmountOff(money.GBP("2.00")),
			),
		)

		order := domain.GBPOrder(guest).
			MustAddLine("X", "Item", money.GBP("20.00"), 1) // 5% of 20 = 1.00 + 2.00 = 3.00 -> pay 17.00

		receipt := checkout.With(bundlePromo).MustTotal(order)

		if !receipt.TotalSavings.Equals(money.GBP("3.00")) {
			t.Errorf("got savings %v, want 3.00", receipt.TotalSavings)
		}
		if !receipt.Payable.Equals(money.GBP("17.00")) {
			t.Errorf("got payable %v, want 17.00", receipt.Payable)
		}
	})

	t.Run("should cap savings at subtotal (never negative payable)", func(t *testing.T) {
		// Fixed £100 off on a £30 order -> pay £0, not -£70
		hugeVoucher := promotion.New(
			"Huge Voucher",
			spec.NewMemberOnly(),
			discount.NewFixedAmountOff(money.GBP("100.00")),
		)

		order := domain.GBPOrder(member).
			MustAddLine("X", "Item", money.GBP("30.00"), 1)

		receipt := checkout.With(hugeVoucher).MustTotal(order)

		if !receipt.TotalSavings.Equals(money.GBP("30.00")) {
			t.Errorf("savings should cap at 30.00, got %v", receipt.TotalSavings)
		}
		if !receipt.Payable.Equals(money.ZeroGBP()) {
			t.Errorf("payable should be 0.00, got %v", receipt.Payable)
		}
	})

	t.Run("should handle exclusive groups where best saving wins", func(t *testing.T) {
		// Family deal: 10% off OR £15 off (exclusive group: 'voucher')
		// On £100 order: 10% is £10, £15 fixed is £15 -> £15 wins!
		promoPercent := promotion.Exclusive(
			"10% Family Voucher",
			spec.Always[*domain.Order](),
			discount.NewPercentOff(10),
			"family-deals",
			1,
		)
		promoFixed := promotion.Exclusive(
			"£15 Family Fixed",
			spec.Always[*domain.Order](),
			discount.NewFixedAmountOff(money.GBP("15.00")),
			"family-deals",
			2,
		)

		order := domain.GBPOrder(guest).
			MustAddLine("X", "Item", money.GBP("100.00"), 1)

		engine := checkout.With(promoPercent, promoFixed)
		receipt := engine.MustTotal(order)

		if len(receipt.Discounts) != 1 {
			t.Fatalf("expected exactly 1 discount from exclusive group, got %d", len(receipt.Discounts))
		}
		if receipt.Discounts[0].PromotionName != "£15 Family Fixed" {
			t.Errorf("expected '£15 Family Fixed' to win, got %q", receipt.Discounts[0].PromotionName)
		}
		if !receipt.TotalSavings.Equals(money.GBP("15.00")) {
			t.Errorf("got savings %v, want 15.00", receipt.TotalSavings)
		}
	})

	t.Run("should respect context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // cancel immediately

		engine := checkout.NoPromotions()
		order := domain.GBPOrder(guest).MustAddLine("X", "Item", money.GBP("10.00"), 1)

		_, err := engine.Total(ctx, order)
		if err == nil {
			t.Fatalf("expected context cancellation error, got nil")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected error wrapping context.Canceled, got %v", err)
		}
	})

	t.Run("should log audit events during checkout", func(t *testing.T) {
		logger := audit.NewInMemoryLogger()
		engine := checkout.New(
			checkout.WithPromotion(promotion.PercentOffOver("10% over £50", 10, money.GBP("50.00"))),
			checkout.WithLogger(logger),
		)

		order := domain.GBPOrder(guest).MustAddLine("X", "Item", money.GBP("100.00"), 1)
		receipt, err := engine.Total(context.Background(), order)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !receipt.TotalSavings.Equals(money.GBP("10.00")) {
			t.Errorf("expected 10.00 savings, got %v", receipt.TotalSavings)
		}

		if logger.Count() == 0 {
			t.Errorf("expected audit logs to be recorded")
		}
		if !logger.ContainsMessage("checkout completed") {
			t.Errorf("expected 'checkout completed' audit message")
		}
	})

	t.Run("should be thread-safe for concurrent Total and AddPromotions calls", func(t *testing.T) {
		engine := checkout.With(
			promotion.PercentOffOver("10% over £50", 10, money.GBP("50.00")),
		)

		var wg sync.WaitGroup

		// Writers adding promotions
		for i := range 10 {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				engine.AddPromotions(promotion.MemberPercentOff(fmt.Sprintf("Member Promo %d", id), 5))
			}(i)
		}

		// Readers calculating checkout total
		for i := range 30 {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				o := domain.GBPOrder(domain.Guest(fmt.Sprintf("guest-%d", id)))
				o.MustAddLine("X", "Item", money.GBP("60.00"), 1)
				receipt, err := engine.Total(context.Background(), o)
				if err != nil {
					t.Errorf("unexpected error in concurrent Total: %v", err)
				}
				if receipt.Subtotal.IsZero() {
					t.Errorf("expected non-zero subtotal")
				}
			}(i)
		}

		wg.Wait()
	})
}
