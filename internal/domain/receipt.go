package domain

import (
	"fmt"
	"iter"
	"slices"
	"strings"

	"github.com/netologist/go-discount-checkout/pkg/money"
)

// AppliedDiscount is the audit trail entry for a promotion successfully applied to a basket.
type AppliedDiscount struct {
	PromotionName string
	Saving        money.Money
	Group         string
}

// Receipt is the final shopping receipt produced by the Checkout process.
//
// Go Pattern: Value Semantics & Pure DTO with Go 1.23+ Iterators.
type Receipt struct {
	Lines        []OrderLine
	Subtotal     money.Money
	Discounts    []AppliedDiscount
	TotalSavings money.Money
	Payable      money.Money
}

// EmptyReceipt returns a safe, zero-valued receipt in the given currency.
func EmptyReceipt(currency string) Receipt {
	zero := money.Zero(currency)
	return Receipt{
		Lines:        make([]OrderLine, 0),
		Subtotal:     zero,
		Discounts:    make([]AppliedDiscount, 0),
		TotalSavings: zero,
		Payable:      zero,
	}
}

// HasSavings reports whether the receipt contains any discount savings.
func (r Receipt) HasSavings() bool {
	return r.TotalSavings.IsPositive()
}

// DiscountCount returns the number of applied promotions.
func (r Receipt) DiscountCount() int {
	return len(r.Discounts)
}

// DiscountsSeq returns a Go 1.23+ iter.Seq that ranges over the applied discounts.
func (r Receipt) DiscountsSeq() iter.Seq[AppliedDiscount] {
	return func(yield func(AppliedDiscount) bool) {
		for _, d := range r.Discounts {
			if !yield(d) {
				return
			}
		}
	}
}

// LinesSeq returns a Go 1.23+ iter.Seq that ranges over the receipt's product lines.
func (r Receipt) LinesSeq() iter.Seq[OrderLine] {
	return func(yield func(OrderLine) bool) {
		for _, l := range r.Lines {
			if !yield(l) {
				return
			}
		}
	}
}

// Clone returns a defensive copy of the Receipt.
func (r Receipt) Clone() Receipt {
	return Receipt{
		Lines:        slices.Clone(r.Lines),
		Subtotal:     r.Subtotal,
		Discounts:    slices.Clone(r.Discounts),
		TotalSavings: r.TotalSavings,
		Payable:      r.Payable,
	}
}

// Summary returns a one-line human-readable summary of the receipt.
func (r Receipt) Summary() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Subtotal: %s", r.Subtotal))
	if r.HasSavings() {
		sb.WriteString(fmt.Sprintf(" | Savings: -%s (%d promos)", r.TotalSavings, len(r.Discounts)))
	}
	sb.WriteString(fmt.Sprintf(" | Payable: %s", r.Payable))
	return sb.String()
}
