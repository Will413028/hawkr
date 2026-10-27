package commerce

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
)

// buyerNote 的長度上限以 rune 計，不是 byte。⛔ 這條分得出兩種實作：中文一字三
// byte，byte 計長會在第 67 個字就拒絕一段完全合法的補充說明，而錯誤訊息還寫著
// 「最多 200 個字」。
func TestValidateBuyerNote_CountsRunesNotBytes(t *testing.T) {
	full := strings.Repeat("蕉", domain.MaxBuyerNoteLen)
	got, err := domain.ValidateBuyerNote(full)
	if err != nil {
		t.Fatalf("%d 個中文字應該合法（byte 長度 %d），err = %v",
			domain.MaxBuyerNoteLen, len(full), err)
	}
	if got != full {
		t.Fatalf("合法輸入不得被改寫")
	}
	if _, err := domain.ValidateBuyerNote(full + "蕉"); err == nil {
		t.Fatalf("超過上限必須拒絕，不得靜默截斷——買家會以為送出的跟畫面上一樣")
	}
}

// 沒填、只打了空白，與真的填了東西，是三種不同的輸入但只有兩種結果。
func TestNormalizedBuyerNote_EmptyBecomesNilSoThePreimageIsUnchanged(t *testing.T) {
	for _, raw := range []string{"", "   ", "\t\n "} {
		validated, err := domain.ValidateBuyerNote(raw)
		if err != nil {
			t.Fatalf("空白輸入應視為沒填，err = %v", err)
		}
		if got := normalizedBuyerNote(validated); got != nil {
			t.Fatalf("normalizedBuyerNote(%q) = %q, want nil —— 指向空字串的指標會讓 "+
				"omitempty 失效，preimage 多出一個鍵，既有 payload_hash 全部改變", raw, *got)
		}
	}

	validated, err := domain.ValidateBuyerNote("  中大果，不要有壓傷  ")
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	got := normalizedBuyerNote(validated)
	if got == nil || *got != "中大果，不要有壓傷" {
		t.Fatalf("normalizedBuyerNote = %v，頭尾空白要去掉、內容不得改動", got)
	}
}

// ⛔ buyerNote 不得走 slots：2026-08-17 移除的 spec 就是被 LLM 抽取的結構化欄位
// （backend #2476／website #498），本欄刻意只從 confirm 的入參進來。
func TestBuyerNoteIsNotASlot(t *testing.T) {
	slotsSource, err := os.ReadFile("../domain/slots.go")
	if err != nil {
		t.Fatalf("read slots.go: %v", err)
	}
	for _, forbidden := range []string{"buyer_note", "buyerNote", "BuyerNote"} {
		if strings.Contains(string(slotsSource), forbidden) {
			t.Fatalf("domain/slots.go 出現 %q —— 做成 slot 就會經過 LLM 抽取與候選搜尋"+
				"關鍵字，那正是 spec 在 2026-08-17 被移除的那條路", forbidden)
		}
	}
}

func TestErrBuyerNoteTooLong_WrapsTheDomainReason(t *testing.T) {
	uc := &ConfirmRfqSnapshotUseCase{}
	_, err := uc.ConfirmRfqSnapshot(t.Context(), ConfirmRfqSnapshotInput{
		BuyerNote: strings.Repeat("蕉", domain.MaxBuyerNoteLen+1),
	})
	if !errors.Is(err, ErrBuyerNoteTooLong) {
		t.Fatalf("err = %v, want ErrBuyerNoteTooLong —— 長度檢查要在任何 DB 往返之前擋下", err)
	}
}
