package domain

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCertification_Submit_FromDraft(t *testing.T) {
	now := time.Now()
	app := &CertificationApplication{Status: CertStatusDraft}
	app.InitFSM()

	if err := app.Submit(now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if app.Status != CertStatusSubmitted {
		t.Errorf("Status = %v, want Submitted", app.Status)
	}
	if app.SubmittedAt == nil || !app.SubmittedAt.Equal(now) {
		t.Errorf("SubmittedAt = %v, want %v", app.SubmittedAt, now)
	}
}

func TestCertification_Submit_FromSubmitted_Fails(t *testing.T) {
	app := &CertificationApplication{Status: CertStatusSubmitted}
	app.InitFSM()

	err := app.Submit(time.Now())
	if err == nil {
		t.Fatal("expected ErrInvalidTransition")
	}
	var target *ErrInvalidTransition
	if !errors.As(err, &target) {
		t.Errorf("error type = %T, want *ErrInvalidTransition", err)
	}
}

func TestCertification_Forward_FromForwarded_Fails(t *testing.T) {
	app := &CertificationApplication{Status: CertStatusForwarded}
	app.InitFSM()

	err := app.Forward(time.Now())
	if err == nil {
		t.Fatal("expected ErrInvalidTransition")
	}
	if app.Status != CertStatusForwarded {
		t.Errorf("Status = %v, want Forwarded (unchanged)", app.Status)
	}
}

func TestCertification_Forward_FromSubmitted(t *testing.T) {
	now := time.Now()
	app := &CertificationApplication{Status: CertStatusSubmitted}
	app.InitFSM()

	if err := app.Forward(now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if app.Status != CertStatusForwarded {
		t.Errorf("Status = %v, want Forwarded", app.Status)
	}
}

func TestCertification_Forward_FromDraft_Fails(t *testing.T) {
	now := time.Now()
	app := &CertificationApplication{Status: CertStatusDraft}
	app.InitFSM()

	if err := app.Forward(now); err == nil {
		t.Fatal("expected error approving a draft, got nil")
	}
	if app.Status != CertStatusDraft {
		t.Errorf("Status = %v, want Draft (unchanged)", app.Status)
	}
}

func TestCertification_Forward_FromRejected_Fails(t *testing.T) {
	now := time.Now()
	app := &CertificationApplication{Status: CertStatusRejected}
	app.InitFSM()

	err := app.Forward(now)
	if err == nil {
		t.Fatal("expected ErrInvalidTransition")
	}
	if app.Status != CertStatusRejected {
		t.Errorf("Status = %v, want Rejected (unchanged)", app.Status)
	}
}

func TestCertification_Reject_FromForwardFailed(t *testing.T) {
	now := time.Now()
	app := &CertificationApplication{Status: CertStatusForwardFailed}
	app.InitFSM()

	if err := app.Reject(now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if app.Status != CertStatusRejected {
		t.Errorf("Status = %v, want Rejected", app.Status)
	}
	if app.RejectedAt == nil || !app.RejectedAt.Equal(now) {
		t.Errorf("RejectedAt = %v, want %v", app.RejectedAt, now)
	}
}

func TestCertification_Reject_FromForwarded_Fails(t *testing.T) {
	now := time.Now()
	app := &CertificationApplication{Status: CertStatusForwarded}
	app.InitFSM()

	err := app.Reject(now)
	if err == nil {
		t.Fatal("expected ErrInvalidTransition")
	}
	if app.Status != CertStatusForwarded {
		t.Errorf("Status = %v, want Forwarded (unchanged)", app.Status)
	}
}

func TestCertification_Reject_FromDraft_Fails(t *testing.T) {
	now := time.Now()
	app := &CertificationApplication{Status: CertStatusDraft}
	app.InitFSM()

	if err := app.Reject(now); err == nil {
		t.Fatal("expected error rejecting a draft, got nil")
	}
	if app.Status != CertStatusDraft {
		t.Errorf("Status = %v, want Draft (unchanged)", app.Status)
	}
}

func TestCertification_Reject_FromSubmitted(t *testing.T) {
	now := time.Now()
	app := &CertificationApplication{Status: CertStatusSubmitted}
	app.InitFSM()

	if err := app.Reject(now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if app.Status != CertStatusRejected {
		t.Errorf("Status = %v, want Rejected", app.Status)
	}
}

func TestCertification_Cancel_FromDraft(t *testing.T) {
	app := &CertificationApplication{Status: CertStatusDraft}
	app.InitFSM()

	if err := app.Cancel(time.Now(), ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if app.Status != CertStatusCancelled {
		t.Errorf("Status = %v, want Cancelled", app.Status)
	}
}

func TestCertification_Cancel_FromSubmitted(t *testing.T) {
	app := &CertificationApplication{Status: CertStatusSubmitted}
	app.InitFSM()

	if err := app.Cancel(time.Now(), ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if app.Status != CertStatusCancelled {
		t.Errorf("Status = %v, want Cancelled", app.Status)
	}
}

func TestCertification_Cancel_FromForwardFailed(t *testing.T) {
	app := &CertificationApplication{Status: CertStatusForwardFailed}
	app.InitFSM()

	if err := app.Cancel(time.Now(), ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if app.Status != CertStatusCancelled {
		t.Errorf("Status = %v, want Cancelled", app.Status)
	}
}

func TestCertification_Cancel_FromForwarded_Fails(t *testing.T) {
	app := &CertificationApplication{Status: CertStatusForwarded}
	app.InitFSM()

	err := app.Cancel(time.Now(), "")
	if err == nil {
		t.Fatal("expected ErrInvalidTransition")
	}
	if app.Status != CertStatusForwarded {
		t.Errorf("Status = %v, want Forwarded (unchanged)", app.Status)
	}
}

func TestCertification_Cancel_FromRejected_Fails(t *testing.T) {
	app := &CertificationApplication{Status: CertStatusRejected}
	app.InitFSM()

	err := app.Cancel(time.Now(), "")
	if err == nil {
		t.Fatal("expected ErrInvalidTransition")
	}
}

func TestCertification_ResetToDraft_FromRejected(t *testing.T) {
	app := &CertificationApplication{Status: CertStatusRejected}
	app.InitFSM()

	if err := app.ResetToDraft(time.Now(), false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if app.Status != CertStatusDraft {
		t.Errorf("Status = %v, want Draft", app.Status)
	}
}

func TestCertification_ResetToDraft_FromSubmitted_Fails(t *testing.T) {
	app := &CertificationApplication{Status: CertStatusSubmitted}
	app.InitFSM()

	err := app.ResetToDraft(time.Now(), false)
	if err == nil {
		t.Fatal("expected ErrInvalidTransition")
	}
	if app.Status != CertStatusSubmitted {
		t.Errorf("Status = %v, want Submitted (unchanged)", app.Status)
	}
}

func TestCertification_UpdateProgress(t *testing.T) {
	now := time.Now()
	app := &CertificationApplication{}
	stepData := json.RawMessage(`{"step1":"done"}`)

	app.UpdateProgress(3, stepData, now)

	if app.CurrentStep != 3 {
		t.Errorf("CurrentStep = %d, want 3", app.CurrentStep)
	}
	if string(app.StepData) != `{"step1":"done"}` {
		t.Errorf("StepData = %s, want %s", app.StepData, `{"step1":"done"}`)
	}
	if !app.UpdatedAt.Equal(now) {
		t.Errorf("UpdatedAt = %v, want %v", app.UpdatedAt, now)
	}
}

func TestCertification_IsDraft(t *testing.T) {
	draft := &CertificationApplication{Status: CertStatusDraft}
	if !draft.IsDraft() {
		t.Error("IsDraft() = false for draft, want true")
	}

	submitted := &CertificationApplication{Status: CertStatusSubmitted}
	if submitted.IsDraft() {
		t.Error("IsDraft() = true for submitted, want false")
	}
}

func TestCertification_IsRejected(t *testing.T) {
	rejected := &CertificationApplication{Status: CertStatusRejected}
	if !rejected.IsRejected() {
		t.Error("IsRejected() = false for rejected, want true")
	}

	draft := &CertificationApplication{Status: CertStatusDraft}
	if draft.IsRejected() {
		t.Error("IsRejected() = true for draft, want false")
	}
}

func TestCertification_FullLifecycle(t *testing.T) {
	now := time.Now()
	app := &CertificationApplication{Status: CertStatusDraft}
	app.InitFSM()

	if err := app.Submit(now); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if err := app.Forward(now); err != nil {
		t.Fatalf("Forward: %v", err)
	}
	if app.Status != CertStatusForwarded {
		t.Errorf("final Status = %v, want Forwarded (PayUni outcome is not a cert state)", app.Status)
	}
}

func TestCertification_ResubmitAfterAdminReject(t *testing.T) {
	now := time.Now()
	app := &CertificationApplication{Status: CertStatusDraft}
	app.InitFSM()

	if err := app.Submit(now); err != nil {
		t.Fatalf("Submit(1): %v", err)
	}
	if err := app.Reject(now); err != nil {
		t.Fatalf("Reject: %v", err)
	}
	if err := app.ResetToDraft(now, false); err != nil {
		t.Fatalf("ResetToDraft: %v", err)
	}
	if err := app.Submit(now); err != nil {
		t.Fatalf("Submit(2): %v", err)
	}
	if app.Status != CertStatusSubmitted {
		t.Errorf("final Status = %v, want Submitted", app.Status)
	}
}

func TestCertification_ResubmitAfterPayuniReject(t *testing.T) {
	now := time.Now()
	app := &CertificationApplication{Status: CertStatusDraft}
	app.InitFSM()

	if err := app.Submit(now); err != nil {
		t.Fatalf("Submit(1): %v", err)
	}
	if err := app.Forward(now); err != nil {
		t.Fatalf("Forward: %v", err)
	}
	app.Status = CertStatusRejected
	app.InitFSM()
	if err := app.ResetToDraft(now, false); err != nil {
		t.Fatalf("ResetToDraft: %v", err)
	}
	if err := app.Submit(now); err != nil {
		t.Fatalf("Submit(2): %v", err)
	}
	if app.Status != CertStatusSubmitted {
		t.Errorf("final Status = %v, want Submitted", app.Status)
	}
}

func TestCertification_UpdateProgress_Boundary(t *testing.T) {
	now := time.Now()
	app := &CertificationApplication{}
	stepData := json.RawMessage(`{}`)

	app.UpdateProgress(-1, stepData, now)
	if app.CurrentStep != -1 {
		t.Errorf("CurrentStep = %d, want -1", app.CurrentStep)
	}

	app.UpdateProgress(0, stepData, now)
	if app.CurrentStep != 0 {
		t.Errorf("CurrentStep = %d, want 0", app.CurrentStep)
	}
}

func TestCertification_StatusHelpers_Exclusive(t *testing.T) {
	draft := &CertificationApplication{Status: CertStatusDraft}
	if !draft.IsDraft() {
		t.Error("Draft: IsDraft() = false")
	}
	if draft.IsSubmitted() {
		t.Error("Draft: IsSubmitted() = true")
	}
	if draft.IsRejected() {
		t.Error("Draft: IsRejected() = true")
	}

	submitted := &CertificationApplication{Status: CertStatusSubmitted}
	if submitted.IsDraft() {
		t.Error("Submitted: IsDraft() = true")
	}
	if !submitted.IsSubmitted() {
		t.Error("Submitted: IsSubmitted() = false")
	}
	if submitted.IsRejected() {
		t.Error("Submitted: IsRejected() = true")
	}

	rejected := &CertificationApplication{Status: CertStatusRejected}
	if rejected.IsDraft() {
		t.Error("Rejected: IsDraft() = true")
	}
	if rejected.IsSubmitted() {
		t.Error("Rejected: IsSubmitted() = true")
	}
	if !rejected.IsRejected() {
		t.Error("Rejected: IsRejected() = false")
	}

	forwarded := &CertificationApplication{Status: CertStatusForwarded}
	if forwarded.IsDraft() {
		t.Error("Forwarded: IsDraft() = true")
	}
	if forwarded.IsSubmitted() {
		t.Error("Forwarded: IsSubmitted() = true")
	}
	if forwarded.IsRejected() {
		t.Error("Forwarded: IsRejected() = true")
	}
}

func TestCertification_ForwardFailed_FromSubmitted(t *testing.T) {
	now := time.Now()
	app := &CertificationApplication{Status: CertStatusSubmitted}
	app.InitFSM()

	if err := app.ForwardFailed(now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if app.Status != CertStatusForwardFailed {
		t.Errorf("Status = %v, want ForwardFailed", app.Status)
	}
}

func TestCertification_ForwardFailed_FromForwardFailed_SameState(t *testing.T) {
	now := time.Now()
	app := &CertificationApplication{Status: CertStatusForwardFailed, UpdatedAt: now}
	app.InitFSM()

	later := now.Add(time.Minute)
	if err := app.ForwardFailed(later); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if app.Status != CertStatusForwardFailed {
		t.Errorf("Status = %v, want ForwardFailed", app.Status)
	}
	if !app.UpdatedAt.Equal(later) {
		t.Errorf("UpdatedAt = %v, want %v", app.UpdatedAt, later)
	}
}

func TestCertification_Forward_FromForwardFailed(t *testing.T) {
	now := time.Now()
	app := &CertificationApplication{Status: CertStatusSubmitted}
	app.InitFSM()

	if err := app.ForwardFailed(now); err != nil {
		t.Fatalf("ForwardFailed: %v", err)
	}
	if err := app.Forward(now); err != nil {
		t.Fatalf("Forward (retry): %v", err)
	}
	if app.Status != CertStatusForwarded {
		t.Errorf("Status = %v, want Forwarded", app.Status)
	}
}

func TestCertification_ForwardedAt_StampedOnForward(t *testing.T) {
	now := time.Now()
	app := &CertificationApplication{Status: CertStatusSubmitted}
	app.InitFSM()

	if err := app.Forward(now); err != nil {
		t.Fatalf("Forward: %v", err)
	}
	if app.ForwardedAt == nil || !app.ForwardedAt.Equal(now) {
		t.Errorf("ForwardedAt = %v, want %v", app.ForwardedAt, now)
	}
}

func TestCertification_Cancel_PersistsReason(t *testing.T) {
	app := &CertificationApplication{Status: CertStatusDraft}
	app.InitFSM()
	if err := app.Cancel(time.Now(), "商品類別填錯，重新申請"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if app.CancelReason != "商品類別填錯，重新申請" {
		t.Errorf("CancelReason = %q, want %q", app.CancelReason, "商品類別填錯，重新申請")
	}
}

func TestCertification_Cancel_TruncatesOverlongReason(t *testing.T) {
	app := &CertificationApplication{Status: CertStatusDraft}
	app.InitFSM()
	long := strings.Repeat("很", 600) // 600 個字元（1800 bytes），遠超 varchar(500)
	if err := app.Cancel(time.Now(), long); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if got := len([]rune(app.CancelReason)); got != 500 {
		t.Errorf("CancelReason 長度 = %d 字元, want 500", got)
	}
	if len([]rune(long)) != 600 {
		t.Fatal("測試前提壞了：輸入應為 600 字元")
	}
}

func TestCertification_Cancel_FSMFailureLeavesNoReason(t *testing.T) {
	app := &CertificationApplication{Status: CertStatusForwarded}
	app.InitFSM()
	if err := app.Cancel(time.Now(), "不該被寫進去"); err == nil {
		t.Fatal("從 forwarded 撤回應該失敗")
	}
	if app.CancelReason != "" {
		t.Errorf("FSM 失敗後 CancelReason 應為空, got %q", app.CancelReason)
	}
}

func TestCertification_ResetToDraft_FromForwarded_RequiresPayuniRejected(t *testing.T) {
	now := time.Now()
	app := &CertificationApplication{Status: CertStatusForwarded}
	app.InitFSM()

	if err := app.ResetToDraft(now, false); err == nil {
		t.Fatal("PayUni 未駁回時不得從 forwarded 回 draft")
	}
	if app.Status != CertStatusForwarded {
		t.Errorf("Status = %v, want Forwarded (unchanged)", app.Status)
	}

	if err := app.ResetToDraft(now, true); err != nil {
		t.Fatalf("PayUni 駁回後必須能回 draft 重送: %v", err)
	}
	if app.Status != CertStatusDraft {
		t.Errorf("Status = %v, want Draft", app.Status)
	}
}
