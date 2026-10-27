package testkit

// Ptr is a generic pointer-of helper shared across test files that build
// SlotPatch/ItemResolution literals with pointer-typed fields.
func Ptr[T any](v T) *T { return &v }
