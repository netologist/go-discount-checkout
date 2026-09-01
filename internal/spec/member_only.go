package spec

import (
	"github.com/hasanozgan/kata/discount-checkout/internal/domain"
)

// MemberOnly is a Specification that restricts a promotion to loyalty-card or
// membership-programme members only.
//
// Go Pattern: Zero-sized struct — implements an interface with zero memory overhead.
type MemberOnly struct{}

// NewMemberOnly returns a MemberOnly specification.
func NewMemberOnly() MemberOnly {
	return MemberOnly{}
}

// IsSatisfiedBy checks whether the order's customer holds an active membership.
func (m MemberOnly) IsSatisfiedBy(order *domain.Order) bool {
	if order == nil {
		return false
	}
	return order.Customer().IsMember
}
