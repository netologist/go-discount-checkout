package money_test

import (
	"encoding/json"
	"errors"
	"iter"
	"sync"
	"testing"

	"github.com/hasanozgan/kata/discount-checkout/pkg/money"
)

// Go Pattern: Table-Driven Tests (Tablo Güdümlü Testler)

func TestZeroValue(t *testing.T) {
	var m money.Money

	if !m.IsZero() {
		t.Errorf("expected zero money to report IsZero=true, got cents=%d", m.Cents())
	}
	if m.Currency() != money.DefaultCurrency {
		t.Errorf("expected default currency %q, got %q", money.DefaultCurrency, m.Currency())
	}
	if m.String() != "GBP 0.00" {
		t.Errorf("expected 'GBP 0.00', got %q", m.String())
	}
}

func TestMoneyAddition(t *testing.T) {
	tests := []struct {
		name      string
		a         money.Money
		b         money.Money
		expected  money.Money
		expectErr error
	}{
		{
			name:      "zero plus zero",
			a:         money.ZeroGBP(),
			b:         money.ZeroGBP(),
			expected:  money.ZeroGBP(),
			expectErr: nil,
		},
		{
			name:      "positive addition",
			a:         money.GBP("12.50"),
			b:         money.GBP("7.50"),
			expected:  money.GBP("20.00"),
			expectErr: nil,
		},
		{
			name:      "penny addition",
			a:         money.GBP("0.01"),
			b:         money.GBP("0.02"),
			expected:  money.GBP("0.03"),
			expectErr: nil,
		},
		{
			name:      "currency mismatch error",
			a:         money.GBP("10.00"),
			b:         money.FromDecimal(10.00, "EUR"),
			expected:  money.Money{},
			expectErr: money.ErrCurrencyMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.a.Add(tt.b)
			if tt.expectErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.expectErr)
				}
				if !errors.Is(err, tt.expectErr) {
					t.Fatalf("expected error wrapping %v, got %v", tt.expectErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !got.Equals(tt.expected) {
				t.Errorf("got %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMoneySubtraction(t *testing.T) {
	tests := []struct {
		name      string
		a         money.Money
		b         money.Money
		expected  money.Money
		expectErr error
	}{
		{
			name:      "exact subtraction",
			a:         money.GBP("15.00"),
			b:         money.GBP("15.00"),
			expected:  money.GBP("0.00"),
			expectErr: nil,
		},
		{
			name:      "partial subtraction",
			a:         money.GBP("50.00"),
			b:         money.GBP("12.35"),
			expected:  money.GBP("37.65"),
			expectErr: nil,
		},
		{
			name:      "negative result returns ErrNegativeAmount",
			a:         money.GBP("10.00"),
			b:         money.GBP("20.00"),
			expected:  money.Money{},
			expectErr: money.ErrNegativeAmount,
		},
		{
			name:      "currency mismatch error",
			a:         money.GBP("10.00"),
			b:         money.FromDecimal(5.00, "USD"),
			expected:  money.Money{},
			expectErr: money.ErrCurrencyMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.a.Sub(tt.b)
			if tt.expectErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.expectErr)
				}
				if !errors.Is(err, tt.expectErr) {
					t.Fatalf("expected error wrapping %v, got %v", tt.expectErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !got.Equals(tt.expected) {
				t.Errorf("got %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMoneyMultiplicationAndPercentage(t *testing.T) {
	t.Run("multiplication", func(t *testing.T) {
		tests := []struct {
			unit     money.Money
			qty      int
			expected money.Money
		}{
			{unit: money.GBP("10.00"), qty: 0, expected: money.GBP("0.00")},
			{unit: money.GBP("12.50"), qty: 1, expected: money.GBP("12.50")},
			{unit: money.GBP("2.35"), qty: 3, expected: money.GBP("7.05")},
			{unit: money.GBP("0.99"), qty: 100, expected: money.GBP("99.00")},
			{unit: money.GBP("10.00"), qty: -5, expected: money.GBP("0.00")},
		}

		for _, tt := range tests {
			got := tt.unit.Mul(tt.qty)
			if !got.Equals(tt.expected) {
				t.Errorf("Mul(%d) on %v = %v, want %v", tt.qty, tt.unit, got, tt.expected)
			}
		}
	})

	t.Run("percentage with retail rounding", func(t *testing.T) {
		tests := []struct {
			base     money.Money
			percent  int
			expected money.Money
		}{
			{base: money.GBP("100.00"), percent: 10, expected: money.GBP("10.00")},
			{base: money.GBP("50.00"), percent: 20, expected: money.GBP("10.00")},
			{base: money.GBP("14.50"), percent: 5, expected: money.GBP("0.73")}, // 14.50 * 5% = 0.725 -> 0.73
			{base: money.GBP("69.00"), percent: 10, expected: money.GBP("6.90")},
			{base: money.GBP("100.00"), percent: 0, expected: money.GBP("0.00")},
			{base: money.GBP("100.00"), percent: 100, expected: money.GBP("100.00")},
			{base: money.GBP("100.00"), percent: -10, expected: money.GBP("0.00")},
		}

		for _, tt := range tests {
			got := tt.base.Percent(tt.percent)
			if !got.Equals(tt.expected) {
				t.Errorf("Percent(%d) on %v = %v, want %v", tt.percent, tt.base, got, tt.expected)
			}
		}
	})
}

func TestMoneyComparisonsAndClamping(t *testing.T) {
	ten := money.GBP("10.00")
	twenty := money.GBP("20.00")

	if !twenty.AtLeast(ten) {
		t.Errorf("expected 20.00 to be AtLeast 10.00")
	}
	if ten.AtLeast(twenty) {
		t.Errorf("expected 10.00 to NOT be AtLeast 20.00")
	}
	if !ten.AtMost(twenty) {
		t.Errorf("expected 10.00 to be AtMost 20.00")
	}

	if got := ten.Min(twenty); !got.Equals(ten) {
		t.Errorf("Min(20.00) on 10.00 = %v, want %v", got, ten)
	}
	if got := twenty.Min(ten); !got.Equals(ten) {
		t.Errorf("Min(10.00) on 20.00 = %v, want %v", got, ten)
	}

	if got := ten.Max(twenty); !got.Equals(twenty) {
		t.Errorf("Max(20.00) on 10.00 = %v, want %v", got, twenty)
	}

	cmpResult, err := ten.Compare(twenty)
	if err != nil || cmpResult != -1 {
		t.Errorf("Compare 10 vs 20: got (%d, %v), want (-1, nil)", cmpResult, err)
	}
}

func TestMoneySumAndSumSeq(t *testing.T) {
	m1 := money.GBP("10.00")
	m2 := money.GBP("20.50")
	m3 := money.GBP("4.50")

	total, err := money.Sum("GBP", m1, m2, m3)
	if err != nil {
		t.Fatalf("unexpected error in Sum: %v", err)
	}
	if !total.Equals(money.GBP("35.00")) {
		t.Errorf("expected £35.00, got %v", total)
	}

	// SumSeq with Go 1.23+ iter.Seq
	seq := func(yield func(money.Money) bool) {
		for _, m := range []money.Money{m1, m2, m3} {
			if !yield(m) {
				return
			}
		}
	}

	totalSeq, err := money.SumSeq("GBP", iter.Seq[money.Money](seq))
	if err != nil {
		t.Fatalf("unexpected error in SumSeq: %v", err)
	}
	if !totalSeq.Equals(money.GBP("35.00")) {
		t.Errorf("expected £35.00 from SumSeq, got %v", totalSeq)
	}

	// Currency mismatch in Sum
	_, err = money.Sum("GBP", m1, money.FromDecimal(5.00, "EUR"))
	if !errors.Is(err, money.ErrCurrencyMismatch) {
		t.Errorf("expected ErrCurrencyMismatch, got %v", err)
	}
}

func TestMoneyJSONAndTextMarshaling(t *testing.T) {
	m := money.GBP("42.50")

	// Text marshaling
	text, err := m.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText error: %v", err)
	}
	if string(text) != "GBP 42.50" {
		t.Errorf("expected 'GBP 42.50', got %q", string(text))
	}

	var unmarshaledText money.Money
	if err := unmarshaledText.UnmarshalText(text); err != nil {
		t.Fatalf("UnmarshalText error: %v", err)
	}
	if !unmarshaledText.Equals(m) {
		t.Errorf("unmarshaled text = %v, want %v", unmarshaledText, m)
	}

	// JSON marshaling
	type Container struct {
		Price money.Money `json:"price"`
	}

	c := Container{Price: m}
	jsonData, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("JSON marshal error: %v", err)
	}

	var decoded Container
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("JSON unmarshal error: %v", err)
	}
	if !decoded.Price.Equals(m) {
		t.Errorf("decoded JSON = %v, want %v", decoded.Price, m)
	}
}

func TestMoneyConcurrentReadSafety(t *testing.T) {
	m := money.GBP("100.00")
	var wg sync.WaitGroup

	for i := range 100 {
		wg.Add(1)
		go func(pct int) {
			defer wg.Done()
			_ = m.Percent(pct % 50)
			_ = m.Mul(pct % 5)
			_ = m.String()
			_ = m.Cents()
			_ = m.Amount()
		}(i)
	}

	wg.Wait()
}

// Go Benchmarks — Go 1.24+ b.Loop() idiom
func BenchmarkMoney_Add(b *testing.B) {
	m1 := money.GBP("12.50")
	m2 := money.GBP("7.50")

	for b.Loop() {
		_, _ = m1.Add(m2)
	}
}

func BenchmarkMoney_Percent(b *testing.B) {
	m := money.GBP("69.00")

	for b.Loop() {
		_ = m.Percent(10)
	}
}
