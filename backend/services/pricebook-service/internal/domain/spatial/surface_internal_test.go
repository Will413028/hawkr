package spatial

import "testing"

func TestBand_EmptyBreaks_ReturnsZeroBandWithoutPanic(t *testing.T) {
	index, lower, upper := band(42.0, nil)
	if index != 0 {
		t.Fatalf("index = %d, want 0", index)
	}
	if lower != nil || upper != nil {
		t.Fatalf("lower/upper = %v/%v, want nil/nil", lower, upper)
	}
}
