package usecase

import (
	"bufio"
	"os"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
)

func loadPayuniErrorCodeSnapshot(t *testing.T) []string {
	t.Helper()
	f, err := os.Open("testdata/payuni_error_codes.txt")
	if err != nil {
		t.Fatalf("open snapshot: %v", err)
	}
	defer f.Close()
	var codes []string
	s := bufio.NewScanner(f)
	for s.Scan() {
		if line := strings.TrimSpace(s.Text()); line != "" {
			codes = append(codes, line)
		}
	}
	if err := s.Err(); err != nil {
		t.Fatalf("scan snapshot: %v", err)
	}
	return codes
}

// payuniNonErrorCodes 是兩軸的正常態，收進目錄只為值域完整，不會下發給賣家。
var payuniNonErrorCodes = map[string]bool{
	"MER_1": true, "MER_9": true, "MEMBER_1": true, "MEMBER_9": true,
}

// 每個代碼都必須有文案與合法 remedy——「每個錯誤代碼都要接上」的機械保證。
func TestResolvePayuniError_CoversEverySnapshotCode(t *testing.T) {
	for _, code := range loadPayuniErrorCodeSnapshot(t) {
		got := ResolvePayuniError(code, "")
		if got.Message == "" || got.Message == code {
			t.Errorf("code %s: message = %q, want a human-readable label", code, got.Message)
		}
		if payuniNonErrorCodes[code] {
			// 正常態不下發，沒有處置分類可言；有 Label 就夠了。
			if got.Remedy != "" {
				t.Errorf("code %s: remedy = %q, want empty for a non-error state", code, got.Remedy)
			}
			continue
		}
		switch got.Remedy {
		case RemedySellerFix, RemedyContactSupport, RemedyRetryLater:
		default:
			t.Errorf("code %s: remedy = %q, want one of the three known values", code, got.Remedy)
		}
		if got.Remedy == RemedySellerFix && got.RetryStep == "" {
			t.Errorf("code %s: SELLER_FIX must carry a retry step", code)
		}
		if got.Remedy != RemedySellerFix && got.RetryStep != "" {
			t.Errorf("code %s: remedy %s must not carry retry step %q", code, got.Remedy, got.RetryStep)
		}
	}
}

// 未知代碼 fail-closed：不猜測、不回退成 PayUni 原文。
func TestResolvePayuniError_UnknownCodeFailsClosed(t *testing.T) {
	got := ResolvePayuniError("AGENT99999", "")
	if got.Remedy != RemedyContactSupport {
		t.Errorf("remedy = %q, want CONTACT_SUPPORT", got.Remedy)
	}
	if got.Message != payuniUnknownErrorMessage() {
		t.Errorf("message = %q, want the generic fallback", got.Message)
	}
	if got.RetryStep != "" {
		t.Errorf("retry step = %q, want empty", got.RetryStep)
	}
}

// final-review fix (Minor)：代碼不在目錄裡，但 PayUni 同時給了具體審核意見時，
// reason 優先序（spec §5.2）不能被「代碼未知」的 fail-closed 判斷插隊蓋掉——
// 訊息要顯示 reason，remedy 仍固定 CONTACT_SUPPORT（未知代碼的分類不因為有
// reason 就改變）。
func TestResolvePayuniError_UnknownCodeWithReason_ReasonWinsButRemedyStaysContactSupport(t *testing.T) {
	got := ResolvePayuniError("AGENT99999", "審核人員手動標註：文件字跡無法辨識")
	if got.Message != "審核人員手動標註：文件字跡無法辨識" {
		t.Errorf("message = %q, want the PayUni reason", got.Message)
	}
	if got.Remedy != RemedyContactSupport {
		t.Errorf("remedy = %q, want CONTACT_SUPPORT even with a reason present", got.Remedy)
	}
	if got.RetryStep != "" {
		t.Errorf("retry step = %q, want empty", got.RetryStep)
	}
	if got.Code != "AGENT99999" {
		t.Errorf("code = %q, want the original code echoed back", got.Code)
	}
}

// PayUni 針對個店寫的 Reason 優先於目錄文案（spec §5.2）。
func TestResolvePayuniError_ReasonWins(t *testing.T) {
	got := ResolvePayuniError("MER_91", "請補上營業登記證影本")
	if got.Message != "請補上營業登記證影本" {
		t.Errorf("message = %q, want the PayUni reason", got.Message)
	}
	if got.Remedy != RemedySellerFix {
		t.Errorf("remedy = %q, want SELLER_FIX", got.Remedy)
	}
}

// 群組表不得有重複代碼——同一碼落在兩組時分類會取決於宣告順序。
func TestPayuniErrorGroups_NoDuplicateCodes(t *testing.T) {
	seen := map[string]bool{}
	for _, g := range payuniErrorGroups {
		for _, code := range g.codes {
			if seen[code] {
				t.Errorf("code %s appears in more than one group", code)
			}
			seen[code] = true
		}
	}
}

// ==================== PayuniErrorDisplayFor: outcome 無法辨識時的兩條分支 ====================
//
// derivePayuniOutcome 對一組未記載的 MerStatus/MemberStatus 回傳零值 payuniOutcome("")，
// PayuniErrorDisplayFor 的 switch 落不到 outcomeApproved/outcomeUnderReview 也落不到
// outcomeRejected/outcomePendingInfo，往下走到這兩條分支。這裡直接呼叫函式測試，
// 不透過 grpc handler：outcomeRejected/outcomePendingInfo 分支已被 handler_test.go 的
// TestGetPayuniStatus_* 系列間接釘住，但這兩條分支目前完全沒有測試覆蓋。

// bind 階段（或 PayUni 回了未記載狀態值）殘留的 AGENT 碼仍要能查到目錄文案，
// 不能因為 outcome 推導不出來就整個丟棄。
func TestPayuniErrorDisplayFor_UnrecognizedOutcome_FallsBackToStoredErrorCode(t *testing.T) {
	reg := &domain.PayuniRegistration{
		MerStatus: "77", MemberStatus: "1", // 未記載組合 -> derivePayuniOutcome 回零值
		ErrorCode: "AGENT03019",
		Status:    domain.PayuniStatusFailed,
	}
	got := PayuniErrorDisplayFor(reg)
	if got.Code != "AGENT03019" {
		t.Errorf("Code = %q, want %q", got.Code, "AGENT03019")
	}
	want := "這組電子信箱或手機號碼已被其他 PayUni 帳號使用，請聯繫客服協助確認"
	if got.Message != want {
		t.Errorf("Message = %q, want %q", got.Message, want)
	}
	if got.Remedy != RemedyContactSupport {
		t.Errorf("Remedy = %q, want %q", got.Remedy, RemedyContactSupport)
	}
	if got.RetryStep != "" {
		t.Errorf("RetryStep = %q, want empty", got.RetryStep)
	}
}

// 失敗了卻沒有業務代碼（transport／加解密失敗，或一次連線失敗的 upsert 把先前的
// AGENT 碼覆蓋成空）：fail-closed 到通用文案，不能讓賣家看到一片空白。這是這次改動
// 要消滅的問題本身，所以必須釘住。
func TestPayuniErrorDisplayFor_FailedWithNoErrorCode_FailsClosed(t *testing.T) {
	reg := &domain.PayuniRegistration{
		MerStatus: "77", MemberStatus: "1", // 未記載組合 -> derivePayuniOutcome 回零值
		ErrorCode: "",
		Status:    domain.PayuniStatusFailed,
	}
	got := PayuniErrorDisplayFor(reg)
	if got.Code != "" {
		t.Errorf("Code = %q, want empty", got.Code)
	}
	if got.Message != payuniUnknownErrorMessage() {
		t.Errorf("Message = %q, want the generic fail-closed fallback %q", got.Message, payuniUnknownErrorMessage())
	}
	if got.Remedy != RemedyContactSupport {
		t.Errorf("Remedy = %q, want %q", got.Remedy, RemedyContactSupport)
	}
	if got.RetryStep != "" {
		t.Errorf("RetryStep = %q, want empty", got.RetryStep)
	}
}

// outcome 無法辨識、沒有殘留代碼、狀態也不是 Failed（例如仍在 pending 中途收到一個
// 未記載的狀態值）：四個欄位都該是空——沒有理由對非失敗狀態顯示錯誤區塊。
func TestPayuniErrorDisplayFor_UnrecognizedOutcome_NotFailed_ReturnsZeroValue(t *testing.T) {
	reg := &domain.PayuniRegistration{
		MerStatus: "77", MemberStatus: "1", // 未記載組合 -> derivePayuniOutcome 回零值
		ErrorCode: "",
		Status:    domain.PayuniStatusPending,
	}
	got := PayuniErrorDisplayFor(reg)
	if got != (PayuniErrorDisplay{}) {
		t.Errorf("PayuniErrorDisplayFor = %+v, want the zero value", got)
	}
}

// nil registration（理論上不應該發生，但 GetPayuniStatus 的 NotFound 分支不會走到
// 這裡）也要 fail-closed 成零值，不能 panic。
func TestPayuniErrorDisplayFor_NilRegistration_ReturnsZeroValue(t *testing.T) {
	got := PayuniErrorDisplayFor(nil)
	if got != (PayuniErrorDisplay{}) {
		t.Errorf("PayuniErrorDisplayFor(nil) = %+v, want the zero value", got)
	}
}

// fail-closed 文案的 SSOT 必須真的在 CodeMeta，不能只靠 Go 裡那份最後防線。
// 這個測試會在有人從 業務類型碼.md 拿掉 PAYUNI_UNKNOWN 時轉紅——否則那次移除
// 會靜默退回硬編碼，而兩邊字串一樣，沒有任何現象看得出來。
func TestPayuniUnknownErrorMessage_ComesFromCodeMeta(t *testing.T) {
	label := codemeta.PayuniSellerFacingErrorLabel(payuniUnknownErrorCode)
	if label == payuniUnknownErrorCode {
		t.Fatalf("PAYUNI_UNKNOWN 未登錄在 CodeMeta，fail-closed 文案退回 Go 的硬編碼最後防線")
	}
	if got := payuniUnknownErrorMessage(); got != label {
		t.Errorf("payuniUnknownErrorMessage() = %q, want CodeMeta label %q", got, label)
	}
}
