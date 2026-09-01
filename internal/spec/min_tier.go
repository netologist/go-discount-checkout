package spec

import (
	"github.com/netologist/go-discount-checkout/internal/domain"
)

// MinimumTier is a Specification that requires the customer to hold at least the
// specified loyalty tier (Silver, Gold, etc.).
type MinimumTier struct {
	Required domain.CustomerTier
}

// NewMinimumTier creates a MinimumTier specification.
func NewMinimumTier(required domain.CustomerTier) MinimumTier {
	return MinimumTier{Required: required}
}

// IsSatisfiedBy checks whether the customer's tier meets the required level.
func (t MinimumTier) IsSatisfiedBy(order *domain.Order) bool {
	if order == nil {
		return false
	}
	return order.Customer().Tier.IsAtLeast(t.Required)
}
