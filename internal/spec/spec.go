// Package spec implements the DDD Specification pattern using Go 1.18+ generics
// and the single-method small-interface philosophy.
//
// It answers the question "WHEN does a promotion apply?" independently of the
// discount formula (what) and the Checkout orchestrator.
package spec

// Specification checks whether a candidate object satisfies a particular business rule.
//
// Go Pattern: Single-Method Interface & Generic Composable Rules.
type Specification[T any] interface {
	IsSatisfiedBy(candidate T) bool
}

// Func adapts a plain function to the Specification interface (Adapter / closure).
type Func[T any] func(candidate T) bool

// IsSatisfiedBy invokes the underlying function to evaluate the rule.
func (f Func[T]) IsSatisfiedBy(candidate T) bool {
	return f(candidate)
}

// And returns a Specification that requires ALL given rules to be satisfied (logical AND).
func And[T any](specs ...Specification[T]) Specification[T] {
	return Func[T](func(candidate T) bool {
		for _, s := range specs {
			if s == nil || !s.IsSatisfiedBy(candidate) {
				return false
			}
		}
		return true
	})
}

// AllOf is a semantic alias for And.
func AllOf[T any](specs ...Specification[T]) Specification[T] {
	return And(specs...)
}

// Or returns a Specification that requires AT LEAST ONE of the given rules to be satisfied (logical OR).
func Or[T any](specs ...Specification[T]) Specification[T] {
	return Func[T](func(candidate T) bool {
		for _, s := range specs {
			if s != nil && s.IsSatisfiedBy(candidate) {
				return true
			}
		}
		return false
	})
}

// AnyOf is a semantic alias for Or.
func AnyOf[T any](specs ...Specification[T]) Specification[T] {
	return Or(specs...)
}

// Not returns a Specification that is the logical negation of the given rule.
func Not[T any](s Specification[T]) Specification[T] {
	return Func[T](func(candidate T) bool {
		if s == nil {
			return false
		}
		return !s.IsSatisfiedBy(candidate)
	})
}

// Always returns a Specification that is unconditionally true.
func Always[T any]() Specification[T] {
	return Func[T](func(candidate T) bool {
		return true
	})
}

// Never returns a Specification that is unconditionally false.
func Never[T any]() Specification[T] {
	return Func[T](func(candidate T) bool {
		return false
	})
}
