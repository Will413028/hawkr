package emailaddr

import (
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
	if got, ok := Normalize(" Owner@Example.COM "); !ok || got != "owner@example.com" {
		t.Errorf("email = %q, %v", got, ok)
	}
	for _, raw := range []string{"", "   ", "user@localhost", "no-at.example.com", "Name <a@b.co>", `"a@b"@c.com`, `"a b"@c.co`, "a@b@c.com"} {
		if _, ok := Normalize(raw); ok {
			t.Errorf("Normalize(%q) 應該被拒絕", raw)
		}
	}
	if _, ok := Normalize(strings.Repeat("a", 250) + "@b.co"); ok {
		t.Errorf("超過 254 字的信箱應該被拒絕")
	}
	if got, ok := Normalize(`"ab"@C.com`); !ok || got != "ab@c.com" {
		t.Errorf("quoted without @ = %q, %v", got, ok)
	}
}
