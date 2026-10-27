package rpc

import "testing"

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
