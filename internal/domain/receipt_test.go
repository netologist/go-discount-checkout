package domain_test

import (
	"testing"

	"github.com/netologist/go-discount-checkout/internal/domain"
	"github.com/netologist/go-discount-checkout/pkg/money"
)

func TestEmptyReceipt(t *testing.T) {
	r := domain.EmptyReceipt(money.DefaultCurrency)

	if len(r.Lines) != 0 {
		t.Errorf("empty receipt should have 0 lines, got %d", len(r.Lines))
	}
	if !r.Subtotal.IsZero() {
		t.Errorf("empty receipt subtotal should be zero, got %v", r.Subtotal)
	}
	if len(r.Discounts) != 0 {
		t.Errorf("empty receipt should have 0 discounts, got %d", len(r.Discounts))
	}
	if r.HasSavings() {
		t.Errorf("empty receipt should have HasSavings = false")
	}
	if r.DiscountCount() != 0 {
		t.Errorf("expected DiscountCount = 0, got %d", r.DiscountCount())
	}
}
