package domain

import "testing"

func TestIsDeliverableEmail(t *testing.T) {
	deliverable := []string{"a@b.co", "vendor.tw@example.com.tw", " ok@example.com "}
	for _, s := range deliverable {
		if !IsDeliverableEmail(s) {
			t.Errorf("IsDeliverableEmail(%q) = false, 應該可寄", s)
		}
	}

	undeliverable := []string{"", "  ", "待確認", "未提供", "N/A", "0912345678", "example.com", "a@b", "@b.co", "a@@b.co"}
	for _, s := range undeliverable {
		if IsDeliverableEmail(s) {
			t.Errorf("IsDeliverableEmail(%q) = true，但這個位址收不到信，發邀請等於靜默失敗", s)
		}
	}
}
