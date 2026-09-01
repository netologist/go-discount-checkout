package domain_test

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/hasanozgan/kata/discount-checkout/internal/domain"
	"github.com/hasanozgan/kata/discount-checkout/pkg/money"
)

func TestOrderCreationAndSubtotal(t *testing.T) {
	guest := domain.Guest("g-1")
	order := domain.GBPOrder(guest)

	if !order.IsEmpty() {
		t.Errorf("new order should be empty")
	}
	if !order.Subtotal().Equals(money.ZeroGBP()) {
		t.Errorf("empty order subtotal should be 0, got %v", order.Subtotal())
	}
	if order.ItemCount() != 0 {
		t.Errorf("empty order item count should be 0, got %d", order.ItemCount())
	}

	// Add lines fluently
	order.MustAddLine("BEANS", "Baked Beans", money.GBP("0.65"), 2) // 1.30
	order.MustAddLine("STEAK", "Ribeye", money.GBP("12.00"), 5)    // 60.00

	if order.IsEmpty() {
		t.Errorf("order should not be empty after adding lines")
	}
	if order.ItemCount() != 7 {
		t.Errorf("expected 7 items, got %d", order.ItemCount())
	}
	expectedSubtotal := money.GBP("61.30")
	if !order.Subtotal().Equals(expectedSubtotal) {
		t.Errorf("subtotal = %v, want %v", order.Subtotal(), expectedSubtotal)
	}

	if order.QuantityOf("BEANS") != 2 {
		t.Errorf("expected 2 BEANS, got %d", order.QuantityOf("BEANS"))
	}
	if order.QuantityOf("STEAK") != 5 {
		t.Errorf("expected 5 STEAK, got %d", order.QuantityOf("STEAK"))
	}
	if !order.ContainsSKU("BEANS") || !order.ContainsSKU("STEAK") {
		t.Errorf("order should contain BEANS and STEAK")
	}
	if order.ContainsSKU("NON_EXISTENT") {
		t.Errorf("order should not contain NON_EXISTENT")
	}

	line, found := order.FindBySKU("BEANS")
	if !found || line.Quantity != 2 || line.Name != "Baked Beans" {
		t.Errorf("FindBySKU BEANS failed: found=%v, line=%+v", found, line)
	}

	_, notFound := order.FindBySKU("DOES_NOT_EXIST")
	if notFound {
		t.Errorf("FindBySKU DOES_NOT_EXIST should return false")
	}
}

func TestOrderLineValidation(t *testing.T) {
	guest := domain.Guest("g-1")
	order := domain.GBPOrder(guest)

	t.Run("empty sku returns ErrEmptySKU", func(t *testing.T) {
		_, err := order.AddLine("", "Name", money.GBP("1.00"), 1)
		if !errors.Is(err, domain.ErrEmptySKU) {
			t.Errorf("expected ErrEmptySKU, got %v", err)
		}
	})

	t.Run("zero or negative quantity returns ErrInvalidQuantity", func(t *testing.T) {
		_, err := order.AddLine("SKU", "Name", money.GBP("1.00"), 0)
		if !errors.Is(err, domain.ErrInvalidQuantity) {
			t.Errorf("expected ErrInvalidQuantity, got %v", err)
		}
	})

	t.Run("mismatched currency returns ErrCurrencyMismatch", func(t *testing.T) {
		eurPrice := money.FromDecimal(5.00, "EUR")
		_, err := order.AddLine("SKU", "Name", eurPrice, 1)
		if !errors.Is(err, money.ErrCurrencyMismatch) {
			t.Errorf("expected ErrCurrencyMismatch, got %v", err)
		}
	})
}

func TestOrderCouponRedemption(t *testing.T) {
	order := domain.GBPOrder(domain.Guest("g-1"))

	t.Run("redeem uppercase stripped coupon", func(t *testing.T) {
		order.MustRedeemCoupon("  save10  ")
		order.MustRedeemCoupon("FREESHIP")

		if !order.HasCoupon("SAVE10") {
			t.Errorf("expected HasCoupon('SAVE10') = true")
		}
		if !order.HasCoupon("save10") {
			t.Errorf("expected HasCoupon('save10') = true (case-insensitive)")
		}
		if !order.HasCoupon("FREESHIP") {
			t.Errorf("expected HasCoupon('FREESHIP') = true")
		}
		if order.HasCoupon("NON_EXISTENT") {
			t.Errorf("expected HasCoupon('NON_EXISTENT') = false")
		}

		coupons := order.RedeemedCoupons()
		if len(coupons) != 2 {
			t.Errorf("expected 2 coupons, got %d", len(coupons))
		}
	})

	t.Run("empty coupon code returns ErrEmptyCoupon", func(t *testing.T) {
		_, err := order.RedeemCoupon("   ")
		if !errors.Is(err, domain.ErrEmptyCoupon) {
			t.Errorf("expected ErrEmptyCoupon, got %v", err)
		}
	})
}

func TestOrderIteratorsAndClone(t *testing.T) {
	order := domain.GBPOrder(domain.Guest("g-iter"))
	order.MustAddLine("SKU1", "Item 1", money.GBP("10.00"), 2)
	order.MustAddLine("SKU2", "Item 2", money.GBP("5.00"), 1)
	order.MustRedeemCoupon("CODE1")

	// AllLines iterator
	count := 0
	for i, l := range order.AllLines() {
		if l.Quantity < 1 {
			t.Errorf("line %d quantity invalid", i)
		}
		count++
	}
	if count != 2 {
		t.Errorf("AllLines expected 2 lines, got %d", count)
	}

	// LinesSeq iterator
	count = 0
	for l := range order.LinesSeq() {
		if l.SKU == "" {
			t.Errorf("expected non-empty SKU")
		}
		count++
	}
	if count != 2 {
		t.Errorf("LinesSeq expected 2 lines, got %d", count)
	}

	// CouponsSeq iterator
	cCount := 0
	for c := range order.CouponsSeq() {
		if c != "CODE1" {
			t.Errorf("unexpected coupon: %q", c)
		}
		cCount++
	}
	if cCount != 1 {
		t.Errorf("CouponsSeq expected 1 coupon, got %d", cCount)
	}

	// Clone
	cloned := order.Clone()
	if cloned.ItemCount() != order.ItemCount() {
		t.Errorf("cloned item count = %d, want %d", cloned.ItemCount(), order.ItemCount())
	}
	if !cloned.Subtotal().Equals(order.Subtotal()) {
		t.Errorf("cloned subtotal = %v, want %v", cloned.Subtotal(), order.Subtotal())
	}
}

func TestOrderConcurrentReadWriteSafety(t *testing.T) {
	order := domain.GBPOrder(domain.Guest("concurrent-shopper"))
	var wg sync.WaitGroup

	// Writers: concurrently add lines and redeem coupons
	for i := range 20 {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			sku := fmt.Sprintf("SKU-%d", workerID)
			_, _ = order.AddLine(sku, "Product", money.GBP("2.50"), (workerID%3)+1)
			_, _ = order.RedeemCoupon(fmt.Sprintf("VOUCHER-%d", workerID))
		}(i)
	}

	// Readers: concurrently query subtotal, lines, coupons, quantity
	for i := range 30 {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			_ = order.Subtotal()
			_ = order.ItemCount()
			_ = order.Lines()
			_ = order.RedeemedCoupons()
			_ = order.ContainsSKU(fmt.Sprintf("SKU-%d", workerID%20))
			_ = order.QuantityOf(fmt.Sprintf("SKU-%d", workerID%20))
			_ = order.HasCoupon(fmt.Sprintf("VOUCHER-%d", workerID%20))
			_ = order.IsEmpty()
			_ = order.Customer()
			_ = order.Currency()

			for _, _ = range order.AllLines() {
			}
			for _ = range order.LinesSeq() {
			}
			for _ = range order.CouponsSeq() {
			}
		}(i)
	}

	wg.Wait()

	if order.IsEmpty() {
		t.Errorf("order should not be empty after concurrent writes")
	}
	if len(order.Lines()) != 20 {
		t.Errorf("expected 20 lines, got %d", len(order.Lines()))
	}
}
