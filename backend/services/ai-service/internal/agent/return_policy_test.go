package agent

import (
	"strings"
	"testing"
)

// TestReturnPolicyFacts_HasRefundTiming guards refund-timing content
// (credit card vs. ATM) in the converged single source. The original task
// brief for #3519 slice B Task 7 item 5 believed only one of the three
// pre-existing copies carried this detail; checking the actual pre-change
// files (commit 7b0de886c in the private archive) showed all three already had it — so this test
// isn't restoring something that was missing, it's locking in that the
// merge didn't lose it either, since the constant these three consumers now
// share is the only place this detail lives going forward.
func TestReturnPolicyFacts_HasRefundTiming(t *testing.T) {
	for _, want := range []string{"信用卡", "7-14", "ATM", "3-5"} {
		if !strings.Contains(ReturnPolicyFacts, want) {
			t.Errorf("ReturnPolicyFacts missing refund-timing detail %q", want)
		}
	}
}

// TestBuyerInstructions_EmbedSharedReturnPolicyFacts locks in that the buyer
// system prompts reference the single shared ReturnPolicyFacts constant instead
// of hand-copying (and re-drifting) their own paraphrase of it.
//
// buyerChatInstruction 隨 all-tools fallback 於 2026-09-20 一併移除（ADR
// 2026-09-20-coordinator-vs-all-tools-fallback），原本三個消費端剩兩個。
func TestBuyerInstructions_EmbedSharedReturnPolicyFacts(t *testing.T) {
	cases := []struct {
		name  string
		instr string
	}{
		{"buyerCoordinatorInstruction", buyerCoordinatorInstruction},
	}
	for _, c := range cases {
		if !strings.Contains(c.instr, ReturnPolicyFacts) {
			t.Errorf("%s does not embed ReturnPolicyFacts verbatim", c.name)
		}
	}
}
