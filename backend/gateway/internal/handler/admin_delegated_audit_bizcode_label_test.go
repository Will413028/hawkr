package handler

import (
	"encoding/json"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	accountpb "github.com/Will413028/hawkr/go-pkg/proto/account"
)

// 稽核列只存 biz_code、不存當時的訊息，所以「這個碼是什麼意思」只有後端查得到。
// 少了 bizCodeLabel，面板上就只剩「業務碼 9266」——對操作者與客服等於沒有資訊，而
// 前端自建對照表會變成第二份 CodeMeta 鏡像（Markdown 改文案它不轉紅，只是顯示舊字串）。
func TestDelegatedAuditEntryDTO_FillsBizCodeLabel(t *testing.T) {
	got := delegatedAuditEntryDTO(&accountpb.AuditLogEntry{
		Result:  "FAILED",
		BizCode: bizcode.DelegationResourceVersionConflict,
	})

	want := bizcode.Message(bizcode.DelegationResourceVersionConflict)
	if want == "" {
		t.Fatalf("CodeMeta 未登錄 %d 的訊息，生成表可能是空的",
			bizcode.DelegationResourceVersionConflict)
	}
	if got.BizCodeLabel != want {
		t.Fatalf("bizCodeLabel = %q，預期 %q", got.BizCodeLabel, want)
	}
}

// 反向鑑別力：成功列與未登錄的碼都必須是空字串。
//
// 少了這兩筆，「無條件填一句『操作失敗』」也會讓上面那支綠 —— 而那正是本欄要避免的
// 東西：一句蓋掉原碼的假說明，比只顯示原碼更難追。
func TestDelegatedAuditEntryDTO_BizCodeLabelEmptyWhenUnmapped(t *testing.T) {
	cases := []struct {
		name    string
		result  string
		bizCode int32
	}{
		{"成功列（bizCode 恆為 0）", "SUCCEEDED", 0},
		{"未登錄的碼", "FAILED", 999999},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := delegatedAuditEntryDTO(&accountpb.AuditLogEntry{
				Result:  tc.result,
				BizCode: tc.bizCode,
			})
			if got.BizCodeLabel != "" {
				t.Fatalf("bizCodeLabel = %q，預期空字串", got.BizCodeLabel)
			}
		})
	}
}

// 欄位必須永遠出現在 JSON 裡。加上 omitempty 之後前端拿到的是 undefined，而 ADM 的
// 契約解析對缺欄與空字串的處置不同 —— 前者整頁報錯，後者才是「這個碼沒登記」。
func TestDelegatedAuditEntryDTO_BizCodeLabelKeyAlwaysPresent(t *testing.T) {
	raw, err := json.Marshal(delegatedAuditEntryDTO(&accountpb.AuditLogEntry{
		Result:  "SUCCEEDED",
		BizCode: 0,
	}))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := decoded["bizCodeLabel"]; !ok {
		t.Fatalf("回應缺少 bizCodeLabel（實際欄位：%s）", raw)
	}
}
