package contentkind

import "testing"

func TestValidIsExhaustiveOverValues(t *testing.T) {
	t.Parallel()
	for _, k := range Values() {
		if !k.Valid() {
			t.Errorf("%q 在 Values 裡卻不是 Valid", k)
		}
	}
	for _, k := range []Kind{"", "post", "BOGUS"} {
		if k.Valid() {
			t.Errorf("%q 不該是 Valid", k)
		}
	}
}
