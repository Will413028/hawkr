package suppress

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/resend"
)

var testKeys = Keys{Current: []byte("current-key-0123456789-0123456789"), Previous: []byte("previous-key-0123456789-012345678")}

// 雜湊以正規化位址為輸入：大小寫與空白不影響；目前與舊密鑰各一個結果，Hash 只用目前密鑰；沒有密鑰時回錯誤。
func TestHasherNormalizesAndLooksUpBothKeys(t *testing.T) {
	h := NewHasher(testKeys)
	a, ok, err := h.Hash("  Gone@Example.COM ")
	if err != nil || !ok {
		t.Fatalf("Hash：%v %v", ok, err)
	}
	b, _, _ := h.Hash("gone@example.com")
	if !bytes.Equal(a, b) || len(a) != 32 {
		t.Fatal("正規化前後的雜湊不同")
	}
	look, err := h.Lookup("gone@example.com")
	if err != nil || len(look) != 2 || !bytes.Equal(look[0], a) || bytes.Equal(look[0], look[1]) {
		t.Fatalf("Lookup = %x %v", look, err)
	}
	if one, _ := NewHasher(Keys{Current: testKeys.Current}).Lookup("gone@example.com"); len(one) != 1 {
		t.Fatalf("沒有舊密鑰時只查一個：%d", len(one))
	}
	if _, ok, err := h.Hash("not-an-address"); ok || err != nil {
		t.Fatalf("格式不合：ok=%v err=%v", ok, err)
	}
	if _, _, err := NewHasher(Keys{}).Hash("gone@example.com"); err == nil {
		t.Fatal("沒有密鑰要回錯誤")
	}
	if _, err := NewHasher(Keys{}).Lookup("gone@example.com"); err == nil {
		t.Fatal("沒有密鑰要回錯誤")
	}
}

// 縮減後的 payload 沒有位址；格式不合的位址略過，都不合格時沒有 payload。
func TestRedactDropsTheAddress(t *testing.T) {
	s := &Service{d: Deps{Hasher: NewHasher(testKeys)}}
	p, err := s.redact(resend.Event{Type: resend.EventBounced, BounceType: "Permanent", To: []string{"Gone@Example.com", "nope"}})
	if err != nil || p == nil {
		t.Fatalf("redact：%v %v", p, err)
	}
	if strings.Contains(strings.ToLower(string(p)), "example") || strings.Contains(string(p), "@") || !strings.Contains(string(p), ReasonHardBounce) {
		t.Fatalf("payload = %s", p)
	}
	if p, err := s.redact(resend.Event{Type: resend.EventComplained, To: []string{"nope"}}); p != nil || err != nil {
		t.Fatalf("沒有合格位址：%s %v", p, err)
	}
}
