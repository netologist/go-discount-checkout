package domain

import (
	"cmp"
	"errors"
	"fmt"
	"iter"
	"maps"
	"slices"
	"strings"
	"sync"

	"github.com/hasanozgan/kata/discount-checkout/pkg/money"
)

var (
	// ErrEmptySKU is returned when a product SKU is blank.
	ErrEmptySKU = errors.New("order: sku cannot be empty")

	// ErrInvalidQuantity is returned when quantity is less than 1.
	ErrInvalidQuantity = errors.New("order: quantity must be at least 1")

	// ErrEmptyCoupon is returned when a coupon code is blank.
	ErrEmptyCoupon = errors.New("order: coupon code cannot be empty")
)

// OrderLine represents a single product in the basket with its unit price and quantity.
type OrderLine struct {
	SKU       string
	Name      string
	UnitPrice money.Money
	Quantity  int
}

// LineTotal returns the total amount for this line (UnitPrice × Quantity).
func (l OrderLine) LineTotal() money.Money {
	return l.UnitPrice.Mul(l.Quantity)
}

// Order is the DDD Aggregate Root that manages the customer's shopping basket,
// line items, and redeemed coupon codes.
//
// Go Patterns applied:
//   - Encapsulation via unexported fields and defensively copied slices/maps.
//   - Thread-safety via sync.RWMutex (concurrent read/write safe).
//   - Modern Go 1.23+ iterators and standard-library slices/maps integration.
type Order struct {
	mu              sync.RWMutex
	customer        Customer
	currency        string
	lines           []OrderLine
	redeemedCoupons map[string]struct{}
}

// NewOrder creates a new, empty order for the given customer and currency.
func NewOrder(customer Customer, currency string) *Order {
	normalizedCurr := cmp.Or(strings.ToUpper(strings.TrimSpace(currency)), money.DefaultCurrency)
	return &Order{
		customer:        customer,
		currency:        normalizedCurr,
		lines:           make([]OrderLine, 0),
		redeemedCoupons: make(map[string]struct{}),
	}
}

// GBPOrder is a convenience constructor for GBP-denominated orders.
func GBPOrder(customer Customer) *Order {
	return NewOrder(customer, money.DefaultCurrency)
}

// Customer returns the order's owning customer.
func (o *Order) Customer() Customer {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.customer
}

// Currency returns the order's currency code.
func (o *Order) Currency() string {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.currency
}

// AddLine adds a new product line to the basket.
// Go Pattern: Thread-safe, explicit error return for boundary validation.
func (o *Order) AddLine(sku, name string, unitPrice money.Money, quantity int) (*Order, error) {
	sku = strings.TrimSpace(sku)
	if sku == "" {
		return nil, ErrEmptySKU
	}
	name = cmp.Or(strings.TrimSpace(name), sku)
	if quantity < 1 {
		return nil, fmt.Errorf("%w: got %d", ErrInvalidQuantity, quantity)
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	if unitPrice.Currency() != o.currency {
		return nil, fmt.Errorf("%w: line currency %s does not match order currency %s",
			money.ErrCurrencyMismatch, unitPrice.Currency(), o.currency)
	}

	o.lines = append(o.lines, OrderLine{
		SKU:       sku,
		Name:      name,
		UnitPrice: unitPrice,
		Quantity:  quantity,
	})
	return o, nil
}

// MustAddLine calls AddLine and panics on error.
// Go Pattern: Must* functions enable fluent chaining in tests and demos.
func (o *Order) MustAddLine(sku, name string, unitPrice money.Money, quantity int) *Order {
	_, err := o.AddLine(sku, name, unitPrice, quantity)
	if err != nil {
		panic(err)
	}
	return o
}

// RedeemCoupon records a promotion or discount coupon code on the basket.
// Codes are normalised to upper-case and trimmed of whitespace (case-insensitive).
func (o *Order) RedeemCoupon(code string) (*Order, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return nil, ErrEmptyCoupon
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	o.redeemedCoupons[code] = struct{}{}
	return o, nil
}

// MustRedeemCoupon calls RedeemCoupon and panics on error.
func (o *Order) MustRedeemCoupon(code string) *Order {
	_, err := o.RedeemCoupon(code)
	if err != nil {
		panic(err)
	}
	return o
}

// HasCoupon reports whether the given coupon code has been applied to this basket.
func (o *Order) HasCoupon(code string) bool {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return false
	}

	o.mu.RLock()
	defer o.mu.RUnlock()

	_, exists := o.redeemedCoupons[code]
	return exists
}

// Subtotal returns the gross total of all line items before discounts.
func (o *Order) Subtotal() money.Money {
	o.mu.RLock()
	defer o.mu.RUnlock()

	total := money.Zero(o.currency)
	for _, line := range o.lines {
		total, _ = total.Add(line.LineTotal())
	}
	return total
}

// QuantityOf returns the total number of units of the given SKU in the basket.
func (o *Order) QuantityOf(sku string) int {
	sku = strings.TrimSpace(sku)
	o.mu.RLock()
	defer o.mu.RUnlock()

	count := 0
	for _, line := range o.lines {
		if strings.EqualFold(line.SKU, sku) {
			count += line.Quantity
		}
	}
	return count
}

// ContainsSKU reports whether at least one unit of the given SKU is in the basket.
func (o *Order) ContainsSKU(sku string) bool {
	return o.QuantityOf(sku) > 0
}

// FindBySKU returns the first OrderLine whose SKU matches, and a boolean found flag.
func (o *Order) FindBySKU(sku string) (OrderLine, bool) {
	sku = strings.TrimSpace(sku)
	o.mu.RLock()
	defer o.mu.RUnlock()

	idx := slices.IndexFunc(o.lines, func(l OrderLine) bool {
		return strings.EqualFold(l.SKU, sku)
	})
	if idx >= 0 {
		return o.lines[idx], true
	}
	return OrderLine{}, false
}

// Lines returns a defensive copy of the basket's line items (via slices.Clone).
func (o *Order) Lines() []OrderLine {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return slices.Clone(o.lines)
}

// RedeemedCoupons returns a defensive copy of the applied coupon codes.
func (o *Order) RedeemedCoupons() []string {
	o.mu.RLock()
	defer o.mu.RUnlock()

	coupons := make([]string, 0, len(o.redeemedCoupons))
	for c := range o.redeemedCoupons {
		coupons = append(coupons, c)
	}
	return coupons
}

// IsEmpty reports whether the basket contains no items.
func (o *Order) IsEmpty() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return len(o.lines) == 0
}

// ItemCount returns the total number of product units in the basket.
func (o *Order) ItemCount() int {
	o.mu.RLock()
	defer o.mu.RUnlock()

	total := 0
	for _, line := range o.lines {
		total += line.Quantity
	}
	return total
}

// AllLines returns a Go 1.23+ iter.Seq2 that ranges over lines with index and value.
func (o *Order) AllLines() iter.Seq2[int, OrderLine] {
	return func(yield func(int, OrderLine) bool) {
		for i, line := range o.Lines() {
			if !yield(i, line) {
				return
			}
		}
	}
}

// LinesSeq returns a Go 1.23+ iter.Seq that ranges over the basket's lines in order.
func (o *Order) LinesSeq() iter.Seq[OrderLine] {
	return func(yield func(OrderLine) bool) {
		for _, line := range o.Lines() {
			if !yield(line) {
				return
			}
		}
	}
}

// CouponsSeq returns a Go 1.23+ iter.Seq that ranges over the redeemed coupon codes.
func (o *Order) CouponsSeq() iter.Seq[string] {
	return func(yield func(string) bool) {
		for _, c := range o.RedeemedCoupons() {
			if !yield(c) {
				return
			}
		}
	}
}

// Clone creates a deep, thread-safe copy of the order.
func (o *Order) Clone() *Order {
	o.mu.RLock()
	defer o.mu.RUnlock()

	return &Order{
		customer:        o.customer,
		currency:        o.currency,
		lines:           slices.Clone(o.lines),
		redeemedCoupons: maps.Clone(o.redeemedCoupons),
	}
}
