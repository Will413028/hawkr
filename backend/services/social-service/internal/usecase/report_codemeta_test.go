package usecase

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
)

// #2551 的病灶不是「某一張表抄錯了」，而是同一個值域被抄成三份互不相干的清單：
// DB CHECK、social.proto 的註解、ADM 前端的下拉。三份都是純字串，抄漏一個值
// 不會編譯錯、不會測試紅——proto 註解就這樣掛著 HATE / ADULT 兩個 CHECK 從未
// 接受的值，同時漏掉 SEXUAL / FOOD_SAFETY / POLITICAL / FAKE_NEWS。
//
// 收斂之後 Go 端的述詞已經直接讀 CodeMeta，但 **DB CHECK 仍然是手寫的字面值**，
// 而它才是權威值域。沒有這支測試的話，「CodeMeta 加了一個新的 reason_code、
// schema.sql 忘了加」會一路綠燈到執行期才炸成 23514 —— 而且只在真的有人送出
// 那個新值時才炸。所以這裡把兩邊並排比對：CodeMeta 是清單，schema.sql 的 CHECK
// 是清單，兩者必須逐值相等。
//
// 新增 enum 時把它補進 checkConstraintCases，不要另開一支平行的測試。
const reportsSchemaPath = "../../db/schema/schema.sql"

// checkInLiteral 抓 `CONSTRAINT <name> CHECK (... IN ('A','B',...))` 裡的值集合。
// 刻意不解析整段 SQL：只要求「約束名稱」與「它後面第一組 IN (...)」，多餘的
// IS NULL 之類的前綴述詞不影響取值。
func checkConstraintValues(t *testing.T, schema, constraintName string) []string {
	t.Helper()

	idx := strings.Index(schema, "CONSTRAINT "+constraintName+" ")
	if idx < 0 {
		t.Fatalf("schema.sql 找不到約束 %s——它是本 enum 的權威值域，被刪掉等於值域無人看守", constraintName)
	}
	rest := schema[idx:]

	inClause := regexp.MustCompile(`IN\s*\(([^)]*)\)`).FindStringSubmatch(rest)
	if inClause == nil {
		t.Fatalf("約束 %s 找不到 IN (...) 值集合；若改寫成別的述詞形式，本測試要跟著改", constraintName)
	}

	matches := regexp.MustCompile(`'([^']*)'`).FindAllStringSubmatch(inClause[1], -1)
	values := make([]string, 0, len(matches))
	for _, m := range matches {
		values = append(values, m[1])
	}
	if len(values) == 0 {
		t.Fatalf("約束 %s 的 IN (...) 解析出 0 個值——解析器壞了，不是約束空了", constraintName)
	}
	sort.Strings(values)
	return values
}

func TestReportCheckConstraintsMatchCodeMeta(t *testing.T) {
	raw, err := os.ReadFile(reportsSchemaPath)
	if err != nil {
		t.Fatalf("讀取 %s 失敗: %v", reportsSchemaPath, err)
	}
	schema := string(raw)

	targetTypes := make([]string, 0, len(codemeta.AllReportTargetTypes))
	for _, v := range codemeta.AllReportTargetTypes {
		targetTypes = append(targetTypes, v.Value)
	}
	reasonCodes := make([]string, 0, len(codemeta.AllReportReasonCodes))
	for _, v := range codemeta.AllReportReasonCodes {
		reasonCodes = append(reasonCodes, v.Value)
	}
	statuses := make([]string, 0, len(codemeta.AllReportStatuss))
	for _, v := range codemeta.AllReportStatuss {
		statuses = append(statuses, v.Value)
	}
	// chk_reports_resolved_action 的值域是 ReportResolveAction，**不是**
	// ModerationActionType。後者是前者的嚴格超集（多 AUTO_REJECT / REOPEN），
	// 而那兩個值不對應任何 ReportStatus 終態——若哪天有人「兩個 enum 長得很像就
	// 合併吧」，這一列會先亮紅燈。
	resolveActions := make([]string, 0, len(codemeta.AllReportResolveActions))
	for _, v := range codemeta.AllReportResolveActions {
		resolveActions = append(resolveActions, v.Value)
	}

	checkConstraintCases := []struct {
		name       string
		constraint string
		codeMeta   []string
	}{
		{"target_type", "chk_reports_target_type", targetTypes},
		{"reason_code", "chk_reports_reason_code", reasonCodes},
		{"status", "chk_reports_status", statuses},
		{"resolved_action", "chk_reports_resolved_action", resolveActions},
	}

	for _, tc := range checkConstraintCases {
		t.Run(tc.name, func(t *testing.T) {
			want := append([]string(nil), tc.codeMeta...)
			sort.Strings(want)
			got := checkConstraintValues(t, schema, tc.constraint)

			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Errorf("%s 的 DB CHECK 與 CodeMeta 值域不一致：\n  CHECK    = %v\n  CodeMeta = %v\n"+
					"兩邊都是手寫清單，任一邊單獨改動都不會被編譯或既有測試擋下——"+
					"新增值請同時改 docs/代碼管理（CodeMeta）/*.md（再 generate.sh）與 schema.sql 的 CHECK",
					tc.constraint, got, want)
			}
		})
	}
}

// 驗證的述詞現在直接讀 CodeMeta，但「讀的是哪一個 enum」仍是人選的。這支測試
// 鎖住兩件呼叫端假設：整份 CodeMeta 清單都被接受（漏接＝新值上線即 400），
// 以及不在清單上的值被拒（誤放＝寫進 DB 才被 23514 擋，錯誤面目全非）。
func TestReportValidatorsAcceptExactlyCodeMetaDomain(t *testing.T) {
	for _, v := range codemeta.AllReportTargetTypes {
		if !isValidTargetType(v.Value) {
			t.Errorf("CodeMeta 登記的 target_type %q 被 isValidTargetType 拒絕", v.Value)
		}
	}
	for _, v := range codemeta.AllReportReasonCodes {
		if !isValidReasonCode(v.Value) {
			t.Errorf("CodeMeta 登記的 reason_code %q 被 isValidReasonCode 拒絕", v.Value)
		}
	}

	// HATE / ADULT 是 social.proto 註解曾經宣告、DB CHECK 從未接受的值。它們留在
	// 這裡當回歸樣本：哪天有人「照註解補齊 enum」，這兩個會先亮紅燈。
	for _, dead := range []string{"HATE", "ADULT", "", "spam", "POST_comment"} {
		if isValidReasonCode(dead) {
			t.Errorf("非法 reason_code %q 被 isValidReasonCode 接受", dead)
		}
		if isValidTargetType(dead) {
			t.Errorf("非法 target_type %q 被 isValidTargetType 接受", dead)
		}
	}
}

// ReportResolveAction 與 ModerationActionType 值域幾乎重疊、名字也像，是最容易被
// 「合併成一個 enum 比較乾淨」處理掉的一對。合併之後 AUTO_REJECT（系統自動處置，
// 根本沒有檢舉單）與 REOPEN（把單子打回申訴，不是終態）就有資格被寫進
// reports.resolved_action，而 actionToStatus 找不到它們的映射——單子會停在一個沒有
// 對應 ReportStatus 的狀態。這支測試是那件事發生時唯一會轉紅的東西。
func TestResolveActionDomainExcludesAuditOnlyVerbs(t *testing.T) {
	for _, auditOnly := range []string{
		codemeta.ModerationActionTypeAutoReject.Value,
		codemeta.ModerationActionTypeReopen.Value,
	} {
		if _, ok := codemeta.ReportResolveActionFromValue(auditOnly); ok {
			t.Errorf("%q 是稽核 log 專用動作，不得進入 ReportResolveAction 值域——"+
				"它沒有對應的 ReportStatus 終態，寫進 reports.resolved_action 會讓單子失去出口", auditOnly)
		}
		if _, err := actionToStatus(auditOnly); err == nil {
			t.Errorf("actionToStatus 接受了稽核專用動作 %q，必須拒絕", auditOnly)
		}
	}

	// 反向：ModerationActionType 必須是嚴格超集。少了任何一個裁決動作，
	// AdminResolve 寫出的稽核列就會帶著一個該表不認得的 action。
	for _, v := range codemeta.AllReportResolveActions {
		if _, ok := codemeta.ModerationActionTypeFromValue(v.Value); !ok {
			t.Errorf("裁決動作 %q 不存在於 ModerationActionType——AdminResolve 會把它寫進 moderation_actions.action", v.Value)
		}
	}
	if len(codemeta.AllModerationActionTypes) <= len(codemeta.AllReportResolveActions) {
		t.Errorf("ModerationActionType(%d) 必須嚴格大於 ReportResolveAction(%d)：兩者相等即代表被合併了",
			len(codemeta.AllModerationActionTypes), len(codemeta.AllReportResolveActions))
	}
}

// actionToStatus 是 ReportResolveAction → ReportStatus 的唯一橋樑，兩邊都是手挑的
// 常數。漏接一個裁決動作＝管理員按下該按鈕時拿到 "invalid resolve action"；映射到
// 一個不存在於 CodeMeta 的狀態＝寫進 DB 時 23514。兩者都只在執行期現形。
func TestActionToStatusCoversEveryResolveActionWithARegisteredStatus(t *testing.T) {
	produced := map[string]bool{}
	for _, action := range codemeta.AllReportResolveActions {
		status, err := actionToStatus(action.Value)
		if err != nil {
			t.Errorf("CodeMeta 登記的裁決動作 %q 沒有對應狀態: %v", action.Value, err)
			continue
		}
		if _, ok := codemeta.ReportStatusFromValue(status); !ok {
			t.Errorf("裁決動作 %q 映射到未登記的狀態 %q", action.Value, status)
		}
		if produced[status] {
			t.Errorf("狀態 %q 被兩個裁決動作共用——映射必須一對一，否則從終態反推裁決會有歧義", status)
		}
		produced[status] = true
	}

	// 兩個非終態狀態不得是任何裁決的產物。
	for _, notTerminal := range []string{
		codemeta.ReportStatusPending.Value,
		codemeta.ReportStatusAppealed.Value,
	} {
		if produced[notTerminal] {
			t.Errorf("裁決不得把單子送回非終態 %q", notTerminal)
		}
	}
}
