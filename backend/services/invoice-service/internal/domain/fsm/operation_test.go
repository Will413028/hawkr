package fsm

import (
	"testing"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
)

// §7.3.2 的 J01–J34，逐名獨立抄第二次：Code + allowed-from 集合 + 目標狀態。
// 對帳用逐名 diff，不是數個數——數量相同但兩條互換來源狀態，數個數看不出來。
var wantOperationTransitions = map[string]struct {
	from []string
	to   string
}{
	"J01": {nil, "CREATED"},
	"J02": {[]string{"CREATED"}, "READY"},
	"J03": {[]string{"CREATED"}, "AWAITING_TRIGGER"},
	"J04": {[]string{"CREATED"}, "NOT_APPLICABLE"},
	"J05": {[]string{"AWAITING_TRIGGER"}, "READY"},
	// CREATED／RETRY_SCHEDULED 兩處於 2026-09-04 由 TestOperationFSMMatchesSQL
	// 對帳補上：實作（SQL）早就放寬了，設計書 §7.3.2 與本抄本都沒回灌（#3089 T3）。
	"J06": {[]string{"CREATED", "AWAITING_TRIGGER", "READY", "RETRY_SCHEDULED", "HELD"}, "CANCELLED_BEFORE_ISSUE"},
	"J07": {[]string{"READY", "RETRY_SCHEDULED", "SENDING"}, "SENDING"},
	"J08": {[]string{"READY", "RETRY_SCHEDULED"}, "HELD"},
	"J09": {[]string{"HELD"}, "READY"},
	"J10": {[]string{"SENDING"}, "SUCCEEDED"},
	"J11": {[]string{"SENDING", "RETRY_SCHEDULED"}, "FAILED_PERMANENT"},
	"J12": {[]string{"SENDING"}, "RETRY_SCHEDULED"},
	"J13": {[]string{"SENDING"}, "WAITING_CONFIRMATION"},
	"J14": {[]string{"RETRY_SCHEDULED"}, "FAILED_RETRY_EXHAUSTED"},
	"J15": {[]string{"WAITING_CONFIRMATION"}, "PROVIDER_STATUS_RECHECK"},
	"J16": {[]string{"PROVIDER_STATUS_RECHECK"}, "PROVIDER_STATUS_RECHECK"},
	"J17": {[]string{"PROVIDER_STATUS_RECHECK"}, "FAILED_UNKNOWN"},
	"J18": {[]string{"FAILED_PERMANENT"}, "READY"},
	"J19": {[]string{"FAILED_RETRY_EXHAUSTED"}, "READY"},
	"J20": {[]string{"FAILED_UNKNOWN"}, "PROVIDER_STATUS_RECHECK"},
	"J21": {[]string{"READY", "RETRY_SCHEDULED", "FAILED_PERMANENT", "FAILED_RETRY_EXHAUSTED", "SELLER_ACTION_REQUIRED"}, "CANCEL_REQUESTED"},
	"J22": {[]string{"CANCEL_REQUESTED", "FAILED_UNKNOWN", "CANCEL_BLOCKED_UNKNOWN"}, "PROVIDER_STATUS_RECHECK"},
	"J23": {[]string{"PROVIDER_STATUS_RECHECK"}, "SUCCEEDED"},
	"J24": {[]string{"PROVIDER_STATUS_RECHECK"}, "CANCELLED_BY_ADMIN"},
	"J25": {[]string{"PROVIDER_STATUS_RECHECK"}, "RETRY_SCHEDULED"},
	"J26": {[]string{"PROVIDER_STATUS_RECHECK"}, "CANCEL_BLOCKED_UNKNOWN"},
	"J27": {[]string{"CANCEL_BLOCKED_UNKNOWN"}, "PROVIDER_STATUS_RECHECK"},
	"J28": {[]string{"CANCEL_BLOCKED_UNKNOWN"}, "CANCELLED_BY_ADMIN"},
	"J29": {[]string{"CANCELLED_BY_ADMIN"}, "MANUAL_PROCESS_ALLOWED"},
	"J30": {[]string{"MANUAL_PROCESS_ALLOWED"}, "RESOLVED_EXTERNALLY"},
	"J31": {[]string{"PROVIDER_STATUS_RECHECK"}, "WAITING_CONFIRMATION"},
	"J32": {[]string{"SENDING"}, "SELLER_ACTION_REQUIRED"},
	"J33": {[]string{"SELLER_ACTION_REQUIRED"}, "READY"},
	"J34": {[]string{"SELLER_ACTION_REQUIRED"}, "READY"},
}

func TestOperationTransitions_MatchDesignDoc(t *testing.T) {
	if len(OperationTransitions) != 34 {
		t.Errorf("M2 轉移列數 = %d, want 34（J01–J34）", len(OperationTransitions))
	}
	got := map[string]OperationTransition{}
	for _, tr := range OperationTransitions {
		if _, dup := got[tr.Code]; dup {
			t.Errorf("duplicate transition code %s", tr.Code)
		}
		got[tr.Code] = tr
	}
	for code, want := range wantOperationTransitions {
		tr, ok := got[code]
		if !ok {
			t.Errorf("%s: 設計書有、實作沒有", code)
			continue
		}
		if tr.To != want.to {
			t.Errorf("%s: To = %s, want %s", code, tr.To, want.to)
		}
		diffSets(t, code+".From", tr.From, want.from)
	}
	for code := range got {
		if _, ok := wantOperationTransitions[code]; !ok {
			t.Errorf("%s: 實作有、設計書沒有", code)
		}
	}
}

// 值域 20 值，每個都必須有路徑抵達。
func TestOperationTransitions_StatesAreCodeMetaValues(t *testing.T) {
	seen := map[string]struct{}{}
	for _, tr := range OperationTransitions {
		for _, s := range append(append([]string{}, tr.From...), tr.To) {
			if _, ok := codemeta.InvoiceOperationStatusFromValue(s); !ok {
				t.Errorf("%q is not an InvoiceOperationStatus value", s)
			}
			seen[s] = struct{}{}
		}
	}
	if len(codemeta.AllInvoiceOperationStatuss) != 20 {
		t.Fatalf("InvoiceOperationStatus 值域 = %d, want 20", len(codemeta.AllInvoiceOperationStatuss))
	}
	for _, v := range codemeta.AllInvoiceOperationStatuss {
		if _, ok := seen[v.Value]; !ok {
			t.Errorf("狀態 %s 未出現在任何一條轉移上——它在 CHECK 裡、在 DTO 值域裡，唯獨沒有人走得到", v.Value)
		}
	}
}

// 租約回收：J07 的候選述詞必須含 SENDING、J16 必須含 PROVIDER_STATUS_RECHECK，
// 否則崩潰的租約永遠回不來（無錯誤、無告警）。
func TestOperationLeaseReclaimSelfLoops(t *testing.T) {
	j07, ok := OperationTransitionByCode("J07")
	if !ok {
		t.Fatal("J07 missing")
	}
	if !contains(j07.From, "SENDING") {
		t.Error("J07 的 allowed-from 必須含 SENDING（租約回收）")
	}
	j16, ok := OperationTransitionByCode("J16")
	if !ok {
		t.Fatal("J16 missing")
	}
	if !contains(j16.From, "PROVIDER_STATUS_RECHECK") || j16.To != "PROVIDER_STATUS_RECHECK" {
		t.Error("J16 必須是 PROVIDER_STATUS_RECHECK 的自環（租約回收）")
	}
}

// INV.rc 的 ClaimInvoiceRecheck 是同一支 CAS 的五個來源（J15/J16/J20/J22/J27），
// 它的 status IN (...) 必須逐字等於這五列來源狀態的聯集。
func TestOperationRecheckClaimAllowedFrom(t *testing.T) {
	got, ok := OperationAllowedFrom("PROVIDER_STATUS_RECHECK")
	if !ok {
		t.Fatal("PROVIDER_STATUS_RECHECK should have transitions")
	}
	diffSets(t, "ClaimInvoiceRecheck allowed-from", got, []string{
		"WAITING_CONFIRMATION",
		"PROVIDER_STATUS_RECHECK",
		"FAILED_UNKNOWN",
		"CANCEL_REQUESTED",
		"CANCEL_BLOCKED_UNKNOWN",
	})
}

// FAILED_UNKNOWN 的唯一出邊是重查——直接重試可能開出第二張發票。
func TestOperationFailedUnknownHasOnlyOneOutEdge(t *testing.T) {
	var targets []string
	for _, tr := range OperationTransitions {
		if contains(tr.From, "FAILED_UNKNOWN") {
			targets = append(targets, tr.Code+"->"+tr.To)
		}
	}
	diffSets(t, "FAILED_UNKNOWN 出邊", targets, []string{
		"J20->PROVIDER_STATUS_RECHECK",
		"J22->PROVIDER_STATUS_RECHECK",
	})
	for _, tr := range OperationTransitions {
		if contains(tr.From, "FAILED_UNKNOWN") && tr.To != "PROVIDER_STATUS_RECHECK" {
			t.Errorf("%s: FAILED_UNKNOWN 不得直接轉往 %s", tr.Code, tr.To)
		}
	}
}

// SELLER_ACTION_REQUIRED 不是永久旗標：J33 是它的自動出邊，刪掉它卡住的票就再也回不來。
func TestOperationSellerActionRequiredHasAutomaticExit(t *testing.T) {
	j33, ok := OperationTransitionByCode("J33")
	if !ok || !contains(j33.From, "SELLER_ACTION_REQUIRED") || j33.To != "READY" {
		t.Fatal("J33 必須存在且是 SELLER_ACTION_REQUIRED → READY 的自動出邊")
	}
	// J34（ADM 手動）不能是唯一出邊。
	auto := 0
	for _, tr := range OperationTransitions {
		if contains(tr.From, "SELLER_ACTION_REQUIRED") && tr.To == "READY" {
			auto++
		}
	}
	if auto < 2 {
		t.Errorf("SELLER_ACTION_REQUIRED → READY 的邊只有 %d 條，want 2（J33 自動 ＋ J34 人工）", auto)
	}
}

func TestOperationTerminalStates(t *testing.T) {
	diffSets(t, "M2 終態", OperationTerminalStates,
		[]string{"SUCCEEDED", "NOT_APPLICABLE", "CANCELLED_BEFORE_ISSUE", "RESOLVED_EXTERNALLY"})
	// CANCEL_BLOCKED_UNKNOWN 與 MANUAL_PROCESS_ALLOWED 各有出邊，不是終態。
	for _, s := range []string{"CANCEL_BLOCKED_UNKNOWN", "MANUAL_PROCESS_ALLOWED"} {
		if IsOperationTerminal(s) {
			t.Errorf("%s 不是終態", s)
		}
	}
	for _, s := range OperationTerminalStates {
		for _, tr := range OperationTransitions {
			if contains(tr.From, s) {
				t.Errorf("終態 %s 不得是任何轉移的起點（%s → %s）", s, tr.Code, tr.To)
			}
		}
	}
}

func TestCanTransitionOperation(t *testing.T) {
	if !CanTransitionOperation("SENDING", "SELLER_ACTION_REQUIRED") {
		t.Error("J32 應該是合法邊")
	}
	if CanTransitionOperation("WAITING_CONFIRMATION", "SUCCEEDED") {
		t.Error("WAITING_CONFIRMATION 刻意沒有直接到 SUCCEEDED 的邊（必須先經 claim）")
	}
	if CanTransitionOperation("SUCCEEDED", "READY") {
		t.Error("終態不得再出發")
	}
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
