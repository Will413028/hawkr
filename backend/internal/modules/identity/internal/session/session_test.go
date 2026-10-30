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

func TestExpiredFollowsTheIdleAndAbsoluteLimits(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	created := now.Add(-100 * 24 * time.Hour)
	for name, tc := range map[string]struct {
		client   model.ClientKind
		lastUsed time.Time
		absolute time.Time
		want     bool
	}{
		"web 閒置未滿 7 天":  {model.ClientWeb, now.Add(WebIdle - time.Second).Add(-WebIdle), now.Add(time.Hour), false},
		"web 閒置剛好 7 天":  {model.ClientWeb, now.Add(-WebIdle), now.Add(time.Hour), true},
		"App 閒置 8 天仍有效": {model.ClientApp, now.Add(-8 * 24 * time.Hour), now.Add(time.Hour), false},
		"App 閒置剛好 14 天": {model.ClientApp, now.Add(-AppIdle), now.Add(time.Hour), true},
		"絕對期限前一秒":       {model.ClientApp, now, now.Add(time.Second), false},
		"絕對期限剛好到":       {model.ClientApp, now, now, true},
		"絕對期限已過但剛使用過":   {model.ClientWeb, now, created.Add(Absolute), true},
	} {
		if got := Expired(tc.client, tc.lastUsed, tc.absolute, now); got != tc.want {
			t.Errorf("%s：Expired = %v，預期 %v", name, got, tc.want)
		}
	}
}
