package session

import (
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/model"
)

func TestValidReturnPath(t *testing.T) {
	t.Parallel()
	for p, want := range map[string]bool{
		"/":                                      true,
		"/account/settings":                      true,
		"/search?q=a&b=c#top":                    true,
		"":                                       false,
		"//evil.example":                         false,
		"/\\evil.example":                        false,
		"https://evil.example":                   false,
		"http://evil.example/":                   false,
		"evil":                                   false,
		"javascript:alert(1)":                    false,
		"/ok\r\nSet-Cookie: x":                   false,
		"/a\x00b":                                false,
		"/" + strings.Repeat("a", maxReturnPath): false,
	} {
		if got := ValidReturnPath(p); got != want {
			t.Errorf("ValidReturnPath(%q) = %v，預期 %v", p, got, want)
		}
	}
}

func TestSecretsAreRandomAndHashedWithSHA256(t *testing.T) {
	t.Parallel()
	a, err := newSecret()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := newSecret()
	if a == b || len(a) != tokenLen {
		t.Fatalf("token 要隨機且長度 %d：%q %q", tokenLen, a, b)
	}
	if h := Hash(a); len(h) != 32 || string(h) == a {
		t.Fatalf("雜湊 = %x", h)
	}
}

func TestIdleDependsOnTheClient(t *testing.T) {
	t.Parallel()
	if Idle(model.ClientWeb) != 7*24*time.Hour || Idle(model.ClientApp) != 14*24*time.Hour {
		t.Fatalf("web %v、App %v", Idle(model.ClientWeb), Idle(model.ClientApp))
	}
}
