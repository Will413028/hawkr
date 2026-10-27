package usecase

import "testing"

func TestStringOrNil(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  *string
	}{
		{name: "empty becomes SQL NULL", input: "", want: nil},
		{name: "value remains present", input: "N04", want: testStringPointer("N04")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stringOrNil(tt.input)
			if tt.want == nil {
				if got != nil {
					t.Fatalf("stringOrNil(%q) = %q, want nil", tt.input, *got)
				}
				return
			}
			if got == nil || *got != *tt.want {
				t.Fatalf("stringOrNil(%q) = %v, want %q", tt.input, got, *tt.want)
			}
		})
	}
}

func testStringPointer(value string) *string {
	return &value
}
