package model

import "testing"

func TestIsDeliverableEmail(t *testing.T) {
	for _, s := range []string{"a@b.co", "vendor.tw@example.com.tw", " ok@example.com "} {
		if !IsDeliverableEmail(s) {
			t.Errorf("IsDeliverableEmail(%q) = false，應該可寄", s)
		}
	}
	for _, s := range []string{"", "  ", "待確認", "未提供", "N/A", "0912345678", "example.com", "a@b", "@b.co", "a@@b.co", "名稱 <a@b.co>"} {
		if IsDeliverableEmail(s) {
			t.Errorf("IsDeliverableEmail(%q) = true，但這個位址收不到信，發邀請等於靜默失敗", s)
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
