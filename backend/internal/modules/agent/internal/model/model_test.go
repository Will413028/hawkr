package model

import (
	"strings"
	"testing"
)

func TestDeliverableEmail(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"a@b.co", "a@b.co"},
		{"vendor.tw@example.com.tw", "vendor.tw@example.com.tw"},
		{" ok@example.com ", "ok@example.com"},
		{" Owner@Example.COM ", "owner@example.com"},
		{`"ab"@c.com`, "ab@c.com"},
		{"<a@b.co>", "a@b.co"},
		{"a+b@c.co", "a+b@c.co"},
		{"用戶@例子.台灣", "用戶@例子.台灣"},
	} {
		if got, ok := DeliverableEmail(c.in); !ok || got != c.want {
			t.Errorf("DeliverableEmail(%q) = %q, %v，預期 %q", c.in, got, ok, c.want)
		}
	}
	long := strings.Repeat("a", 250) + "@b.co"
	for _, s := range []string{"", "  ", "待確認", "未提供", "N/A", "0912345678", "example.com", "a@b", "@b.co", "a@@b.co", "名稱 <a@b.co>", "Name <a@b.co>", "a@b.", long} {
		if got, ok := DeliverableEmail(s); ok {
			t.Errorf("DeliverableEmail(%q) = %q, true，但這個位址收不到信，發邀請等於靜默失敗", s, got)
		}
	}
}

func TestMaskEmail(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"vendor@example.com", "v***@example.com"},
		{"a@example.com", "***@example.com"},
		{"ab@example.com", "a***@example.com"},
		{"@example.com", "***"},
		{"not-an-email", "***"},
		{"", "***"},
	} {
		if got := MaskEmail(c.in); got != c.want {
			t.Errorf("MaskEmail(%q) = %q，預期 %q", c.in, got, c.want)
		}
	}
}
