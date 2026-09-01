package domain_test

import (
	"testing"

	"github.com/hasanozgan/kata/discount-checkout/internal/domain"
)

func TestCustomerZeroValueAndConstructors(t *testing.T) {
	t.Run("zero value customer", func(t *testing.T) {
		var c domain.Customer
		if c.IsMember {
			t.Errorf("zero value customer should not be member")
		}
		if c.Tier != domain.TierStandard {
			t.Errorf("zero value customer tier should be TierStandard, got %v", c.Tier)
		}
	})

	t.Run("anonymous factory", func(t *testing.T) {
		c := domain.Anonymous()
		if c.ID != "anonymous-guest" {
			t.Errorf("expected ID 'anonymous-guest', got %q", c.ID)
		}
		if c.IsMember {
			t.Errorf("expected member false")
		}
		if c.Tier != domain.TierStandard {
			t.Errorf("expected standard tier")
		}
	})

	t.Run("guest factory", func(t *testing.T) {
		c := domain.Guest("g-123")
		if c.ID != "g-123" || c.IsMember || c.Tier != domain.TierStandard {
			t.Errorf("unexpected guest values: %+v", c)
		}
	})

	t.Run("member factory with tiers", func(t *testing.T) {
		c := domain.Member("m-gold", domain.TierGold)
		if c.ID != "m-gold" || !c.IsMember || c.Tier != domain.TierGold {
			t.Errorf("unexpected member values: %+v", c)
		}
	})
}

func TestCustomerTierComparisons(t *testing.T) {
	tests := []struct {
		name     string
		tier     domain.CustomerTier
		required domain.CustomerTier
		expected bool
	}{
		{"gold meets gold", domain.TierGold, domain.TierGold, true},
		{"gold meets silver", domain.TierGold, domain.TierSilver, true},
		{"gold meets standard", domain.TierGold, domain.TierStandard, true},
		{"silver meets silver", domain.TierSilver, domain.TierSilver, true},
		{"silver meets standard", domain.TierSilver, domain.TierStandard, true},
		{"silver does not meet gold", domain.TierSilver, domain.TierGold, false},
		{"standard meets standard", domain.TierStandard, domain.TierStandard, true},
		{"standard does not meet silver", domain.TierStandard, domain.TierSilver, false},
		{"standard does not meet gold", domain.TierStandard, domain.TierGold, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.tier.IsAtLeast(tt.required)
			if got != tt.expected {
				t.Errorf("%v.IsAtLeast(%v) = %v, want %v", tt.tier, tt.required, got, tt.expected)
			}
		})
	}
}
