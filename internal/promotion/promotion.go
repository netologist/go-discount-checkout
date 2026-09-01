// Package promotion combines a Specification (when) and a Discount (what) into
// a named promotion, and manages priority and exclusive-group conflict rules.
package promotion

import (
	"cmp"
	"strings"

	"github.com/hasanozgan/kata/discount-checkout/internal/discount"
	"github.com/hasanozgan/kata/discount-checkout/internal/domain"
	"github.com/hasanozgan/kata/discount-checkout/internal/spec"
)

// Promotion binds a name, an eligibility rule (When), a saving formula (What),
// an evaluation priority, and an optional exclusivity group (ExclusiveGroup).
//
// Go Pattern: Composition over Inheritance & Immutability.
type Promotion struct {
	Name           string
	When           spec.Specification[*domain.Order]
	What           discount.Discount
	Priority       int
	ExclusiveGroup string
}

// New creates a stackable (no exclusive group) promotion with default priority 0.
func New(name string, when spec.Specification[*domain.Order], what discount.Discount) Promotion {
	return NewWithPolicy(name, when, what, 0, "")
}

// NewWithPolicy creates a promotion with an explicit priority and exclusivity group.
//
// Go Pattern: Policy as Data — whether a promotion is stackable or exclusive is
// determined by a data field (ExclusiveGroup), not by a code branch.
func NewWithPolicy(
	name string,
	when spec.Specification[*domain.Order],
	what discount.Discount,
	priority int,
	exclusiveGroup string,
) Promotion {
	name = cmp.Or(strings.TrimSpace(name), "Unnamed Promotion")
	if when == nil {
		when = spec.Always[*domain.Order]()
	}
	if what == nil {
		what = discount.NewPercentOff(0)
	}

	return Promotion{
		Name:           name,
		When:           when,
		What:           what,
		Priority:       priority,
		ExclusiveGroup: strings.TrimSpace(exclusiveGroup),
	}
}

// IsStackable reports whether the promotion can freely combine with others.
// A promotion is stackable when its ExclusiveGroup is empty.
func (p Promotion) IsStackable() bool {
	return p.ExclusiveGroup == ""
}

// Evaluate tests the order against the promotion's rule (When) and, if satisfied
// and the saving is positive, returns an AppliedDiscount and true.
//
// Go Pattern: Idiomatic (T, bool) comma-ok pattern.
func (p Promotion) Evaluate(order *domain.Order) (domain.AppliedDiscount, bool) {
	if order == nil || order.IsEmpty() {
		return domain.AppliedDiscount{}, false
	}

	if !p.When.IsSatisfiedBy(order) {
		return domain.AppliedDiscount{}, false
	}

	saving := p.What.Apply(order)
	if saving.IsZero() || saving.IsNegative() {
		return domain.AppliedDiscount{}, false
	}

	return domain.AppliedDiscount{
		PromotionName: p.Name,
		Saving:        saving,
		Group:         p.ExclusiveGroup,
	}, true
}
