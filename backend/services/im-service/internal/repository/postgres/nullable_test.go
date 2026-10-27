package postgres

import "testing"

func TestNullableText(t *testing.T) {
	if got := nullableText(""); got != nil {
		t.Fatalf("nullableText(\"\") = %q, want nil", *got)
	}

	const value = "moderation reason"
	got := nullableText(value)
	if got == nil || *got != value {
		t.Fatalf("nullableText(%q) = %v, want pointer to original value", value, got)
	}
}
