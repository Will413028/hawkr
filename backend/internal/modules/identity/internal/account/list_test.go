package account

import "testing"

func TestEscapeLike(t *testing.T) {
	if got := escapeLike(`a%b_c\d`); got != `a\%b\_c\\d` {
		t.Fatalf("escaped pattern = %q", got)
	}
}
