package charge

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// 建立付款的訂單編號：至少一張、去頭尾空白、長度 1 到 64、不得重複。
// mutation：normalizeOrderNos 不檢查重複 → 重複案例失敗；不 trim → 空白案例的輸出失敗。
func TestNormalizeOrderNosTrimsAndRefusesEmptyDuplicatedAndOverlongNumbers(t *testing.T) {
	t.Parallel()
	got, err := normalizeOrderNos([]string{" A ", "B"})
	if err != nil || len(got) != 2 || got[0] != "A" || got[1] != "B" {
		t.Fatalf("got %v err %v", got, err)
	}
	for name, in := range map[string][]string{
		"沒有訂單":  nil,
		"空白編號":  {"A", "  "},
		"重複":    {"A", " A"},
		"過長的編號": {strings.Repeat("x", 65)},
	} {
		if _, err := normalizeOrderNos(in); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：err=%v，預期 InvalidInput", name, err)
		}
	}
}

// 查回的 created 下限是第一次呼叫的記錄時刻減時鐘誤差（UTC）；沒有記錄時用列的建立時刻。
// mutation：下限不減時鐘誤差 → 這個測試失敗。
func TestTheCreatedLowerBoundIsTheFirstCallMinusTheClockSkew(t *testing.T) {
	t.Parallel()
	called := time.Date(2026, 11, 23, 4, 0, 0, 0, time.FixedZone("TST", 8*3600))
	created := called.Add(-time.Hour)
	if got := lookupLowerBound(&called, created); !got.Equal(called.Add(-ListClockSkew)) || got.Location() != time.UTC {
		t.Fatalf("有呼叫記錄：%v", got)
	}
	if got := lookupLowerBound(nil, created); !got.Equal(created.Add(-ListClockSkew)) {
		t.Fatalf("沒有呼叫記錄：%v", got)
	}
	if ListClockSkew <= 0 || ListClockSkew >= IdempotencyRetention {
		t.Fatalf("時鐘誤差餘裕 %v 不合理", ListClockSkew)
	}
}

// transfer_group 是付款群組的 ID：同一筆付款的所有轉帳共用，不同付款不同。
func TestTransferGroupIsThePaymentID(t *testing.T) {
	t.Parallel()
	id := uuid.NewV7()
	if TransferGroupOf(id) != id.String() || TransferGroupOf(id) == TransferGroupOf(uuid.NewV7()) {
		t.Fatal("transfer_group 不是付款 ID")
	}
}

// 人工結案：原因必須是已知值而且與結果相容；備註去頭尾空白後 1 到上限個字元。
// mutation：validateResolution 不檢查相容性 → 對應案例失敗；不檢查長度 → 過長案例失敗；不 trim → 回傳的備註帶空白，失敗。
func TestValidateResolutionChecksTheReasonAgainstTheOutcomeAndBoundsTheNote(t *testing.T) {
	t.Parallel()
	ok := func(outcome model.AnomalyOutcome, reason model.AnomalyResolutionReason, note string) string {
		t.Helper()
		got, err := validateResolution(outcome, reason, note)
		if err != nil {
			t.Fatalf("%s/%s：%v", outcome, reason, err)
		}
		return got
	}
	if got := ok(model.AnomalySucceeded, model.ResolutionProviderExecutedNotRecorded, "  核對過 "); got != "核對過" {
		t.Fatalf("備註=%q", got)
	}
	ok(model.AnomalyFailed, model.ResolutionProviderNotExecuted, "n")
	for _, outcome := range []model.AnomalyOutcome{model.AnomalySucceeded, model.AnomalyFailed} {
		ok(outcome, model.ResolutionDuplicate, "n")
		ok(outcome, model.ResolutionOther, "n")
	}
	ok(model.AnomalyFailed, model.ResolutionOther, strings.Repeat("字", model.ResolutionNoteMaxRunes))
	for name, c := range map[string]struct {
		outcome model.AnomalyOutcome
		reason  model.AnomalyResolutionReason
		note    string
	}{
		"沒有原因":       {model.AnomalyFailed, "", "n"},
		"不認得的原因":     {model.AnomalyFailed, "X", "n"},
		"已執行只搭配成功":   {model.AnomalyFailed, model.ResolutionProviderExecutedNotRecorded, "n"},
		"沒執行只搭配失敗":   {model.AnomalySucceeded, model.ResolutionProviderNotExecuted, "n"},
		"空白備註":       {model.AnomalyFailed, model.ResolutionOther, " 　\t"},
		"超過上限的備註":    {model.AnomalyFailed, model.ResolutionOther, strings.Repeat("字", model.ResolutionNoteMaxRunes+1)},
		"上限以字計不以位元組": {model.AnomalyFailed, model.ResolutionOther, strings.Repeat("字", model.ResolutionNoteMaxRunes) + "x"},
	} {
		if _, err := validateResolution(c.outcome, c.reason, c.note); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：err=%v，預期 InvalidInput", name, err)
		}
	}
}

// 401／403 仍是「不明」，但每次都記一筆錯誤等級的告警 log：帶操作、provider、HTTP 狀態與 provider 的錯誤代碼，不帶回應內容或金鑰（D-F）。
// mutation：classify 不記 log → 這個測試失敗；classify 把 AuthRejectedError 歸成 callRefused 或 callRetryable → 分類斷言失敗。
func TestAnAuthRejectionIsAlertedAtErrorLevelAndStaysUnknown(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	s := &Service{d: Deps{Logger: slog.New(slog.NewTextHandler(&logs, nil)), Provider: NewFake()}}
	leakCanary := "sk_live_do_not_print"
	err := &AuthRejectedError{Status: 401, Code: "invalid_api_key", Err: fmt.Errorf("stripe：HTTP 401 invalid_api_key：%s", "Invalid API Key provided")}
	if got := s.classify(t.Context(), "create_transfer", err); got != callUnknown {
		t.Fatalf("classify = %v，預期 callUnknown", got)
	}
	out := logs.String()
	for _, want := range []string{"level=ERROR", "op=create_transfer", "http_status=401", "provider_error_code=invalid_api_key", "provider=" + FakeName} {
		if !strings.Contains(out, want) {
			t.Errorf("告警 log 缺少 %q：%s", want, out)
		}
	}
	if strings.Contains(out, leakCanary) || strings.Contains(out, "Invalid API Key provided") {
		t.Errorf("告警 log 帶了回應內容：%s", out)
	}
	logs.Reset()
	if got := s.classify(t.Context(), "create_transfer", errors.New("timeout")); got != callUnknown || logs.Len() != 0 {
		t.Fatalf("一般的不明錯誤不該告警：%v %q", got, logs.String())
	}
}
