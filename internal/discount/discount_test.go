package discount_test

import (
	"testing"

	"github.com/hasanozgan/kata/discount-checkout/internal/discount"
	"github.com/hasanozgan/kata/discount-checkout/internal/domain"
	"github.com/hasanozgan/kata/discount-checkout/pkg/money"
)

func newTestOrder() *domain.Order {
	return domain.GBPOrder(domain.Guest("g-1"))
}

func TestPercentOff(t *testing.T) {
	order := newTestOrder().MustAddLine("ITEM", "Item", money.GBP("50.00"), 1)
	disc := discount.NewPercentOff(10)

	got := disc.Apply(order)
	expected := money.GBP("5.00")

	if !got.Equals(expected) {
		t.Errorf("PercentOff(10) on £50 = %v, want %v", got, expected)
	}
}

func TestPercentOffSKU(t *testing.T) {
	order := newTestOrder().
		MustAddLine("BREAD", "Bread", money.GBP("2.00"), 3). // £6.00
		MustAddLine("MILK", "Milk", money.GBP("1.00"), 4)    // £4.00

	disc := discount.NewPercentOffSKU("BREAD", 20) // 20% of 6.00 = 1.20

	got := disc.Apply(order)
	expected := money.GBP("1.20")

	if !got.Equals(expected) {
		t.Errorf("PercentOffSKU(BREAD, 20) = %v, want %v", got, expected)
	}
}

func TestFixedAmountOff(t *testing.T) {
	t.Run("normal fixed discount", func(t *testing.T) {
		order := newTestOrder().MustAddLine("ITEM", "Item", money.GBP("50.00"), 1)
		disc := discount.NewFixedAmountOff(money.GBP("10.00"))

		got := disc.Apply(order)
		expected := money.GBP("10.00")
		if !got.Equals(expected) {
			t.Errorf("got %v, want %v", got, expected)
		}
	})

	t.Run("capped at order subtotal", func(t *testing.T) {
		order := newTestOrder().MustAddLine("ITEM", "Item", money.GBP("15.00"), 1)
		disc := discount.NewFixedAmountOff(money.GBP("100.00"))

		got := disc.Apply(order)
		expected := money.GBP("15.00") // Capped at £15 subtotal
		if !got.Equals(expected) {
			t.Errorf("Fixed discount exceeding subtotal should cap: got %v, want %v", got, expected)
		}
	})
}

func TestBOGOF(t *testing.T) {
	tests := []struct {
		qty      int
		expected string
	}{
		{qty: 1, expected: "0.00"},
		{qty: 2, expected: "0.00"},
		{qty: 3, expected: "1.50"}, // 1 free
		{qty: 4, expected: "1.50"},
		{qty: 5, expected: "1.50"},
		{qty: 6, expected: "3.00"}, // 2 free
		{qty: 9, expected: "4.50"}, // 3 free
	}

	bogof := discount.BuyTwoGetOneFree("PASTE")

	for _, tt := range tests {
		order := newTestOrder().MustAddLine("PASTE", "Toothpaste", money.GBP("1.50"), tt.qty)
		got := bogof.Apply(order)
		expected := money.GBP(tt.expected)

		if !got.Equals(expected) {
			t.Errorf("BOGOF for qty %d = %v, want %v", tt.qty, got, expected)
		}
	}
}

func TestStackedDiscount(t *testing.T) {
	// 5% off + £2 fixed
	stacked := discount.Stack(
		discount.NewPercentOff(5),
		discount.NewFixedAmountOff(money.GBP("2.00")),
	)

	order := newTestOrder().MustAddLine("ITEM", "Item", money.GBP("20.00"), 1)
	// 5% of 20 = 1.00; + 2.00 = 3.00
	got := stacked.Apply(order)
	expected := money.GBP("3.00")

	if !got.Equals(expected) {
		t.Errorf("StackedDiscount = %v, want %v", got, expected)
	}
}
