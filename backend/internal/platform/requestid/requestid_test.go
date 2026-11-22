package requestid

import (
	"testing"
	"uuid"
)

func TestIsV4(t *testing.T) {
	for _, raw := range []string{"3f0a4c9e-7b21-4d6a-9c1e-5a2b8d7f0e13", "3f0a4c9e-7b21-4d6a-8c1e-5a2b8d7f0e13", "3f0a4c9e-7b21-4d6a-bc1e-5a2b8d7f0e13"} {
		if !IsV4(uuid.MustParse(raw)) {
			t.Errorf("IsV4(%s) = false，預期 true", raw)
		}
	}
	for _, raw := range []string{"00000000-0000-0000-0000-000000000000", "3f0a4c9e-7b21-1d6a-9c1e-5a2b8d7f0e13", "3f0a4c9e-7b21-4d6a-cc1e-5a2b8d7f0e13", "3f0a4c9e-7b21-4d6a-7c1e-5a2b8d7f0e13"} {
		if IsV4(uuid.MustParse(raw)) {
			t.Errorf("IsV4(%s) = true，預期 false", raw)
		}
	}
}
