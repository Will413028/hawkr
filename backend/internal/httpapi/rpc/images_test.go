package rpc

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestNewImagesAcceptsOnlyAnOrigin(t *testing.T) {
	cases := []struct {
		origin string
		ok     bool
	}{
		{"https://media.example", true},
		{"https://media.example/", true},
		{"http://localhost:8787", true},
		{"http://127.0.0.1:8787", true},
		{"http://media.example", false},
		{"https://media.example/i", false},
		{"https://media.example?x=1", false},
		{"https://user@media.example", false},
		{"media.example", false},
		{"", false},
	}
	for _, c := range cases {
		_, err := NewImages(c.origin)
		if (err == nil) != c.ok {
			t.Errorf("NewImages(%q) 的錯誤 = %v，預期可用 = %v", c.origin, err, c.ok)
		}
	}
}

func TestImageIsATemplateWithOnePreset(t *testing.T) {
	im, err := NewImages("https://media.example/")
	if err != nil {
		t.Fatal(err)
	}
	if got := im.Image(""); got != nil {
		t.Fatalf("沒有 key 時 = %v，預期 nil", got)
	}
	want := "https://media.example/i/{preset}/public/banner/b1/a.jpg"
	if got := im.Image("public/banner/b1/a.jpg").GetUriTemplate(); got != want {
		t.Fatalf("uri_template = %q，預期 %q", got, want)
	}
}

// 推播圖片的網址與 API 的 uri_template 同一個 origin 與路徑形狀，preset 取代 {preset}。
func TestImageURLMatchesTheTemplateShape(t *testing.T) {
	im, err := NewImages("https://media.example/")
	if err != nil {
		t.Fatal(err)
	}
	key := "public/product/p1/a.jpg"
	want := strings.Replace(im.Image(key).GetUriTemplate(), "{preset}", PushImagePreset, 1)
	if got := im.URL(key, PushImagePreset); got != want || got != "https://media.example/i/card/"+key {
		t.Fatalf("URL = %q，預期 %q", got, want)
	}
	if got := im.URL("", PushImagePreset); got != "" {
		t.Fatalf("沒有 key 時 = %q，預期空字串", got)
	}
}

// 推播用的 preset 必須是 image gateway 白名單內的名稱（presets.ts 是唯一來源，未知的 preset 在 gateway 回 400）。
func TestPushImagePresetIsAGatewayPreset(t *testing.T) {
	src, err := os.ReadFile("../../../workers/image-gateway/src/presets.ts")
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`(?m)^  ` + PushImagePreset + `: \{`).Match(src) {
		t.Fatalf("presets.ts 的 PRESETS 沒有 %q", PushImagePreset)
	}
}
