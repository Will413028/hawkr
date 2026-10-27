package testkit

import "context"

// FakeResolver is a generic single-canned-response test double for a
// Resolve(ctx, itemText) (T, error) capability such as cropidentity.Resolver
// (instantiated as FakeResolver[cropidentity.DisambiguationResult]).
// Generic rather than importing cropidentity directly: cropidentity's own
// in-package tests need this same double too, and cropidentity importing
// testkit importing cropidentity would be an import cycle.
type FakeResolver[T any] struct {
	ResolveResult T
	ResolveErr    error
}

func (f *FakeResolver[T]) Resolve(_ context.Context, _ string) (T, error) {
	return f.ResolveResult, f.ResolveErr
}
