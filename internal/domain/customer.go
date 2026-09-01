// Package domain contains the core business entities and value objects of the
// e-commerce checkout system: Customer, Order, OrderLine, and Receipt.
package domain

import (
	"cmp"
	"fmt"
	"strings"
)

// CustomerTier represents the loyalty level of a customer.
// Go Pattern: Typed Constants & iota Enums.
type CustomerTier int

const (
	// TierStandard is the default (non-member or entry-level) tier.
	// Go Pattern: iota starts at zero, so the zero value is a safe, useful default.
	TierStandard CustomerTier = iota

	// TierSilver is the silver loyalty tier.
	TierSilver

	// TierGold is the gold (VIP) loyalty tier.
	TierGold
)

// String returns the human-readable name of a CustomerTier.
func (t CustomerTier) String() string {
	switch t {
	case TierGold:
		return "GOLD"
	case TierSilver:
		return "SILVER"
	default:
		return "STANDARD"
	}
}

// IsAtLeast reports whether the customer's tier is at or above the required level.
func (t CustomerTier) IsAtLeast(required CustomerTier) bool {
	return t >= required
}

// Customer is a value object representing the owner of a shopping basket and their
// loyalty status.
//
// Go Pattern: "Make the zero value useful".
// Customer{} (zero value) safely behaves as an anonymous guest with no membership.
type Customer struct {
	ID       string
	IsMember bool
	Tier     CustomerTier
}

// Anonymous returns the zero-value default anonymous guest customer.
func Anonymous() Customer {
	return Customer{
		ID:       "anonymous-guest",
		IsMember: false,
		Tier:     TierStandard,
	}
}

// Guest returns a non-member customer with the given ID.
func Guest(id string) Customer {
	return Customer{
		ID:       cmp.Or(strings.TrimSpace(id), "guest"),
		IsMember: false,
		Tier:     TierStandard,
	}
}

// Member returns a loyalty-card member customer with the given ID and tier.
func Member(id string, tier CustomerTier) Customer {
	return Customer{
		ID:       cmp.Or(strings.TrimSpace(id), "member"),
		IsMember: true,
		Tier:     tier,
	}
}

func (c Customer) String() string {
	if c.IsMember {
		return fmt.Sprintf("Customer(ID=%s, Member=true, Tier=%s)", c.ID, c.Tier)
	}
	return fmt.Sprintf("Customer(ID=%s, Member=false)", c.ID)
}
