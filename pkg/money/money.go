// Package money represents monetary amounts as a DDD Value Object, following
// Go's "make the zero value useful" principle.
package money

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"math"
	"strconv"
	"strings"
)

// Sentinel errors — compatible with errors.Is and errors.As.
var (
	// ErrCurrencyMismatch is returned when an operation is attempted between two different currencies.
	ErrCurrencyMismatch = errors.New("money: currency mismatch")

	// ErrNegativeAmount is returned when the result would be negative but that is disallowed.
	ErrNegativeAmount = errors.New("money: negative amount not allowed")

	// ErrInvalidAmount is returned when a monetary string cannot be parsed.
	ErrInvalidAmount = errors.New("money: invalid amount format")
)

const (
	// DefaultCurrency is the default currency code used when none is specified.
	DefaultCurrency = "GBP"

	// Scale is the number of minor units per major unit (100 pence per pound).
	Scale = 100
)

// Money is an immutable value object that stores monetary amounts as integer
// minor units (pence / cents) to avoid floating-point precision loss.
//
// Go Pattern: Value Receiver & Immutability.
// Money is carried by value (not pointer) and every operation returns a new copy.
// Immutable value semantics make it inherently thread-safe in concurrent contexts.
type Money struct {
	cents    int64
	currency string
}

// New creates a Money from a minor-unit amount.
// Example: New(1250, "GBP") → £12.50
func New(cents int64, currency string) Money {
	normalizedCurr := cmp.Or(strings.ToUpper(strings.TrimSpace(currency)), DefaultCurrency)
	return Money{
		cents:    cents,
		currency: normalizedCurr,
	}
}

// Zero returns a Money with a zero amount in the given currency.
// Go Pattern: "Make the zero value useful".
func Zero(currency string) Money {
	return New(0, currency)
}

// ZeroGBP returns a ready-made £0.00 value.
func ZeroGBP() Money {
	return New(0, DefaultCurrency)
}

// FromDecimal converts a decimal float to a Money using HALF_UP retail rounding.
func FromDecimal(amount float64, currency string) Money {
	cents := int64(math.Round(amount * float64(Scale)))
	return New(cents, currency)
}

// FromString parses a decimal string such as "12.50" or "0.65" into a Money.
func FromString(amountStr string, currency string) (Money, error) {
	amountStr = strings.TrimSpace(amountStr)
	if amountStr == "" {
		return Money{}, fmt.Errorf("%w: empty string", ErrInvalidAmount)
	}

	val, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		return Money{}, fmt.Errorf("%w: %s (%v)", ErrInvalidAmount, amountStr, err)
	}

	return FromDecimal(val, currency), nil
}

// MustFromString is the panic-on-error variant of FromString.
// Go Pattern: Must* functions are safe to use in tests or package-level init.
func MustFromString(amountStr string, currency string) Money {
	m, err := FromString(amountStr, currency)
	if err != nil {
		panic(fmt.Sprintf("MustFromString failed for %q (%s): %v", amountStr, currency, err))
	}
	return m
}

// GBP is a convenience constructor for British Sterling amounts.
func GBP(amountStr string) Money {
	return MustFromString(amountStr, DefaultCurrency)
}

// Cents returns the amount in minor units (pence / cents).
func (m Money) Cents() int64 {
	return m.cents
}

// Currency returns the currency code; defaults to GBP if empty.
func (m Money) Currency() string {
	return cmp.Or(m.currency, DefaultCurrency)
}

// Amount returns the amount as a decimal float.
func (m Money) Amount() float64 {
	return float64(m.cents) / float64(Scale)
}

// IsZero reports whether the amount is exactly zero.
func (m Money) IsZero() bool {
	return m.cents == 0
}

// IsPositive reports whether the amount is greater than zero.
func (m Money) IsPositive() bool {
	return m.cents > 0
}

// IsNegative reports whether the amount is less than zero.
func (m Money) IsNegative() bool {
	return m.cents < 0
}

// Add sums two monetary values. Returns an error if currencies differ.
// Go Pattern: Explicit error return (T, error).
func (m Money) Add(other Money) (Money, error) {
	if err := m.checkCurrency(other); err != nil {
		return Money{}, err
	}
	return New(m.cents+other.cents, m.Currency()), nil
}

// MustAdd adds two amounts and panics on a currency mismatch.
func (m Money) MustAdd(other Money) Money {
	res, err := m.Add(other)
	if err != nil {
		panic(err)
	}
	return res
}

// Sub subtracts other from m. Returns an error if the result is negative or
// if the currencies differ.
func (m Money) Sub(other Money) (Money, error) {
	if err := m.checkCurrency(other); err != nil {
		return Money{}, err
	}
	diff := m.cents - other.cents
	if diff < 0 {
		return Money{}, fmt.Errorf("%w: %s - %s resulting in negative %d cents", ErrNegativeAmount, m, other, diff)
	}
	return New(diff, m.Currency()), nil
}

// MustSub subtracts and panics on error.
func (m Money) MustSub(other Money) Money {
	res, err := m.Sub(other)
	if err != nil {
		panic(err)
	}
	return res
}

// Mul multiplies the amount by a non-negative integer quantity.
func (m Money) Mul(quantity int) Money {
	if quantity < 0 {
		return Zero(m.Currency())
	}
	return New(m.cents*int64(quantity), m.Currency())
}

// Percent calculates a percentage discount (0–100) with HALF_UP retail rounding.
// Example: £14.50 × 5% = 0.725 → rounded to £0.73.
func (m Money) Percent(pct int) Money {
	if pct <= 0 {
		return Zero(m.Currency())
	}
	if pct >= 100 {
		return m
	}
	// Retail rounding formula: (cents * pct + 50) / 100
	discountCents := (m.cents*int64(pct) + 50) / 100
	return New(discountCents, m.Currency())
}

// Min returns the smaller of the two amounts (used for capping savings).
func (m Money) Min(other Money) Money {
	if err := m.checkCurrency(other); err != nil {
		return m
	}
	if m.cents <= other.cents {
		return m
	}
	return other
}

// Max returns the larger of the two amounts.
func (m Money) Max(other Money) Money {
	if err := m.checkCurrency(other); err != nil {
		return m
	}
	if m.cents >= other.cents {
		return m
	}
	return other
}

// AtLeast reports whether m >= other (same currency required).
func (m Money) AtLeast(other Money) bool {
	if err := m.checkCurrency(other); err != nil {
		return false
	}
	return m.cents >= other.cents
}

// AtMost reports whether m <= other (same currency required).
func (m Money) AtMost(other Money) bool {
	if err := m.checkCurrency(other); err != nil {
		return false
	}
	return m.cents <= other.cents
}

// Equals reports whether both the currency and amount are identical.
func (m Money) Equals(other Money) bool {
	return m.cents == other.cents && m.Currency() == other.Currency()
}

// Compare returns -1 if m < other, 0 if equal, 1 if m > other.
func (m Money) Compare(other Money) (int, error) {
	if err := m.checkCurrency(other); err != nil {
		return 0, err
	}
	return cmp.Compare(m.cents, other.cents), nil
}

// Sum adds all given amounts in the same currency.
// Go Pattern: Variadic function with domain error propagation.
func Sum(currency string, amounts ...Money) (Money, error) {
	total := Zero(currency)
	for _, a := range amounts {
		var err error
		total, err = total.Add(a)
		if err != nil {
			return Money{}, err
		}
	}
	return total, nil
}

// SumSeq sums all Money values produced by a Go 1.23+ iter.Seq iterator.
// Go Pattern: Go 1.23+ Range-over-func Iterator Integration.
func SumSeq(currency string, seq iter.Seq[Money]) (Money, error) {
	total := Zero(currency)
	for a := range seq {
		var err error
		total, err = total.Add(a)
		if err != nil {
			return Money{}, err
		}
	}
	return total, nil
}

// String implements fmt.Stringer, producing output such as "GBP 12.50".
func (m Money) String() string {
	curr := m.Currency()
	units := m.cents / Scale
	fraction := m.cents % Scale
	if fraction < 0 {
		fraction = -fraction
	}
	return fmt.Sprintf("%s %d.%02d", curr, units, fraction)
}

// MarshalText implements encoding.TextMarshaler ("GBP 12.50").
func (m Money) MarshalText() ([]byte, error) {
	return []byte(m.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler ("GBP 12.50" or "12.50").
func (m *Money) UnmarshalText(data []byte) error {
	str := strings.TrimSpace(string(data))
	if str == "" {
		*m = ZeroGBP()
		return nil
	}

	parts := strings.Fields(str)
	if len(parts) == 2 {
		curr, amt := parts[0], parts[1]
		parsed, err := FromString(amt, curr)
		if err != nil {
			return err
		}
		*m = parsed
		return nil
	}

	if len(parts) == 1 {
		parsed, err := FromString(parts[0], DefaultCurrency)
		if err != nil {
			return err
		}
		*m = parsed
		return nil
	}

	return fmt.Errorf("%w: cannot parse text %q into Money", ErrInvalidAmount, str)
}

// MarshalJSON implements json.Marshaler.
func (m Money) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{
		"cents":    m.cents,
		"currency": m.Currency(),
		"amount":   m.Amount(),
		"display":  m.String(),
	})
}

// UnmarshalJSON implements json.Unmarshaler.
func (m *Money) UnmarshalJSON(data []byte) error {
	// String format: "GBP 12.50"
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		return m.UnmarshalText([]byte(str))
	}

	// Number format: decimal value
	var num float64
	if err := json.Unmarshal(data, &num); err == nil {
		*m = FromDecimal(num, DefaultCurrency)
		return nil
	}

	// Object format: { cents, currency } or { amount, currency }
	var obj struct {
		Cents    int64   `json:"cents"`
		Currency string  `json:"currency"`
		Amount   float64 `json:"amount"`
	}
	if err := json.Unmarshal(data, &obj); err == nil {
		if obj.Cents != 0 || obj.Currency != "" {
			*m = New(obj.Cents, obj.Currency)
			return nil
		}
		if obj.Amount != 0 {
			*m = FromDecimal(obj.Amount, cmp.Or(obj.Currency, DefaultCurrency))
			return nil
		}
		*m = ZeroGBP()
		return nil
	}

	return fmt.Errorf("%w: cannot unmarshal JSON into Money", ErrInvalidAmount)
}

func (m Money) checkCurrency(other Money) error {
	if m.Currency() != other.Currency() {
		return fmt.Errorf("%w: %s vs %s", ErrCurrencyMismatch, m.Currency(), other.Currency())
	}
	return nil
}
