package fsm

import (
	"sort"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
)

var wantDocumentEdges = []string{
	"[*]->PENDING",
	"[*]->NOT_APPLICABLE",
	"PENDING->ISSUING",
	"PENDING->CANCELLED_BEFORE_ISSUE",
	"ISSUING->ISSUED",
	"ISSUING->ISSUE_FAILED",
	"ISSUING->PENDING",
	"ISSUE_FAILED->ISSUING",
	"PENDING->AWAITING_MANUAL",
	"ISSUE_FAILED->AWAITING_MANUAL",
	"AWAITING_MANUAL->PENDING",
	"AWAITING_MANUAL->RESOLVED_EXTERNALLY",
	"AWAITING_MANUAL->CANCELLED_BEFORE_ISSUE",
	"ISSUE_FAILED->RESOLVED_EXTERNALLY",
	"ISSUED->VOIDING",
	"VOIDING->VOIDED",
	"VOIDING->ISSUED",
	"VOIDED->SUPERSEDED",
	"ISSUED->SUPERSEDED",
}

func documentEdges() []string {
	var out []string
	for _, t := range DocumentTransitions {
		if len(t.From) == 0 {
			out = append(out, "[*]->"+t.To)
			continue
		}
		for _, f := range t.From {
			out = append(out, f+"->"+t.To)
		}
	}
	return out
}

func diffSets(t *testing.T, label string, got, want []string) {
	t.Helper()
	g, w := map[string]int{}, map[string]int{}
	for _, s := range got {
		g[s]++
	}
	for _, s := range want {
		w[s]++
	}
	var missing, extra []string
	for s, n := range w {
		if g[s] < n {
			missing = append(missing, s)
		}
	}
	for s, n := range g {
		if w[s] < n {
			extra = append(extra, s)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) > 0 {
		t.Errorf("%s: 設計書有、實作沒有: %s", label, strings.Join(missing, ", "))
	}
	if len(extra) > 0 {
		t.Errorf("%s: 實作有、設計書沒有: %s", label, strings.Join(extra, ", "))
	}
}

func TestDocumentTransitions_MatchDesignDoc(t *testing.T) {
	if len(DocumentTransitions) != 17 {
		t.Errorf("M1 轉移列數 = %d, want 17（§7.3.0 轉移總數 123 的組成）", len(DocumentTransitions))
	}
	diffSets(t, "M1", documentEdges(), wantDocumentEdges)
}

// 每個狀態值都必須查得回生成的 enum；值域大小必須是 11。
func TestDocumentTransitions_StatesAreCodeMetaValues(t *testing.T) {
	seen := map[string]struct{}{}
	for _, tr := range DocumentTransitions {
		for _, s := range append(append([]string{}, tr.From...), tr.To) {
			if _, ok := codemeta.InvoiceDocumentStatusFromValue(s); !ok {
				t.Errorf("%q is not an InvoiceDocumentStatus value", s)
			}
			seen[s] = struct{}{}
		}
	}
	if len(codemeta.AllInvoiceDocumentStatuss) != 11 {
		t.Fatalf("InvoiceDocumentStatus 值域 = %d, want 11", len(codemeta.AllInvoiceDocumentStatuss))
	}
	for _, v := range codemeta.AllInvoiceDocumentStatuss {
		if _, ok := seen[v.Value]; !ok {
			t.Errorf("狀態 %s 未出現在任何一條轉移上", v.Value)
		}
	}
}

func TestDocumentAllowedFrom(t *testing.T) {
	got, ok := DocumentAllowedFrom(codemeta.InvoiceDocumentStatusAwaitingManual.Value)
	if !ok {
		t.Fatal("AWAITING_MANUAL should have transitions")
	}
	want := []string{
		codemeta.InvoiceDocumentStatusPending.Value,
		codemeta.InvoiceDocumentStatusIssueFailed.Value,
	}
	diffSets(t, "AllowedFrom(AWAITING_MANUAL)", got, want)

	got, _ = DocumentAllowedFrom(codemeta.InvoiceDocumentStatusSuperseded.Value)
	diffSets(t, "AllowedFrom(SUPERSEDED)", got, []string{
		codemeta.InvoiceDocumentStatusVoided.Value,
		codemeta.InvoiceDocumentStatusIssued.Value,
	})

	if _, ok := DocumentAllowedFrom("NOT_A_STATUS"); ok {
		t.Fatal("unknown target must return ok=false")
	}
	got, ok = DocumentAllowedFrom(codemeta.InvoiceDocumentStatusPending.Value)
	if !ok {
		t.Fatal("PENDING should have transitions")
	}
	if len(got) == 0 {
		t.Fatal("PENDING 除了 [*] 之外還有 ISSUING / AWAITING_MANUAL 兩個起點")
	}
}

func TestCanTransitionDocument(t *testing.T) {
	ok := func(from, to string) {
		t.Helper()
		if !CanTransitionDocument(from, to) {
			t.Errorf("CanTransitionDocument(%s, %s) = false, want true", from, to)
		}
	}
	no := func(from, to string) {
		t.Helper()
		if CanTransitionDocument(from, to) {
			t.Errorf("CanTransitionDocument(%s, %s) = true, want false", from, to)
		}
	}
	ok(codemeta.InvoiceDocumentStatusIssuing.Value, codemeta.InvoiceDocumentStatusIssued.Value)
	ok(codemeta.InvoiceDocumentStatusVoiding.Value, codemeta.InvoiceDocumentStatusIssued.Value)
	no(codemeta.InvoiceDocumentStatusVoided.Value, codemeta.InvoiceDocumentStatusIssuing.Value)
	no(codemeta.InvoiceDocumentStatusNotApplicable.Value, codemeta.InvoiceDocumentStatusPending.Value)
	no(codemeta.InvoiceDocumentStatusPending.Value, codemeta.InvoiceDocumentStatusIssued.Value)
}

// ISSUED 是靜止態而非封閉終態——它隨時可以再走 VOIDING、SUPERSEDED。
func TestDocumentTerminalStates(t *testing.T) {
	want := []string{"CANCELLED_BEFORE_ISSUE", "NOT_APPLICABLE", "RESOLVED_EXTERNALLY", "SUPERSEDED", "VOIDED"}
	diffSets(t, "M1 終態", DocumentTerminalStates, want)
	if IsDocumentTerminal(codemeta.InvoiceDocumentStatusIssued.Value) {
		t.Fatal("ISSUED 不是終態：法定保存 5 年，第 5 年仍可能有折讓")
	}
	for _, s := range DocumentTerminalStates {
		if !IsDocumentTerminal(s) {
			t.Errorf("IsDocumentTerminal(%s) = false", s)
		}
		if s == codemeta.InvoiceDocumentStatusVoided.Value {
			continue
		}
		for _, tr := range DocumentTransitions {
			for _, f := range tr.From {
				if f == s {
					t.Errorf("終態 %s 不得是任何轉移的起點（→ %s）", s, tr.To)
				}
			}
		}
	}
}
