package postgres

import "testing"

func TestNullableText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  *string
	}{
		{name: "empty maps to SQL NULL", input: "", want: nil},
		{name: "non-empty is preserved", input: "reason", want: stringPtrForTest("reason")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nullableText(tt.input)
			if !sameStringPointer(got, tt.want) {
				t.Fatalf("nullableText(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestTextPointerPreservesEmptyString(t *testing.T) {
	got := textPointer("")
	if got == nil || *got != "" {
		t.Fatalf("textPointer(\"\") = %v, want pointer to empty string", got)
	}
}

func TestNullableInt64(t *testing.T) {
	if got := nullableInt64(0); got != nil {
		t.Fatalf("nullableInt64(0) = %v, want nil", got)
	}

	got := nullableInt64(42)
	if got == nil || *got != 42 {
		t.Fatalf("nullableInt64(42) = %v, want pointer to 42", got)
	}
}

func stringPtrForTest(value string) *string { return &value }

func sameStringPointer(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
