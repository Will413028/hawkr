package usecase

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestPayuniUC_SyncPending_NoPending(t *testing.T) {
	payuniRepo := &mockPayuniRepo{
		GetPendingFn: func(ctx context.Context, limit int, staleBefore time.Time) ([]*domain.PayuniRegistration, error) {
			return []*domain.PayuniRegistration{}, nil
		},
	}

	uc := &PayuniUseCase{payuniRepo: payuniRepo}
	if err := uc.SyncPendingStatuses(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPayuniUC_SyncPending_UpdatesActive(t *testing.T) {
	var updatedPayuni bool

	payuniRepo := &mockPayuniRepo{
		GetPendingFn: func(ctx context.Context, limit int, staleBefore time.Time) ([]*domain.PayuniRegistration, error) {
			return []*domain.PayuniRegistration{
				{ID: 1, AccountID: 100, PayuniMerID: "HAWK2612345678", Status: domain.PayuniStatusPending},
			}, nil
		},
		UpdateFn: func(ctx context.Context, reg *domain.PayuniRegistration) error {
			updatedPayuni = true
			if reg.Status != domain.PayuniStatusActive {
				t.Errorf("payuni Status = %v, want Active", reg.Status)
			}
			return nil
		},
	}

	pc := &mockPayuniClient{
		GetSellerStatusFn: func(ctx context.Context, merID string) (*SellerStatusResponse, error) {
			return &SellerStatusResponse{
				MerStatus: "1", MemberStatus: "1", CreditStatus: "0",
				ApplePayStatus: "0", GooglePayStatus: "0", ATMStatus: "0",
				RawResponse: json.RawMessage(`{}`),
			}, nil
		},
	}

	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID}, nil
		},
	}

	uc := &PayuniUseCase{payuniRepo: payuniRepo, payuniClient: pc, sellerRepo: sellerRepo}
	if err := uc.SyncPendingStatuses(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updatedPayuni {
		t.Error("payuni registration was not updated")
	}
}

func TestPayuniUC_SyncPending_ContradictorySnapshot_DoesNotActivate(t *testing.T) {
	var updatedPayuni bool

	payuniRepo := &mockPayuniRepo{
		GetPendingFn: func(ctx context.Context, limit int, staleBefore time.Time) ([]*domain.PayuniRegistration, error) {
			return []*domain.PayuniRegistration{
				{ID: 1, AccountID: 100, PayuniMerID: "HAWK2612345678", Status: domain.PayuniStatusPending},
			}, nil
		},
		UpdateFn: func(ctx context.Context, reg *domain.PayuniRegistration) error {
			updatedPayuni = true
			if reg.Status == domain.PayuniStatusActive {
				t.Errorf("payuni Status = Active, want it to stay non-Active: MerStatus=1 with MemberStatus=9 is not a fully-approved snapshot")
			}
			if reg.PaymentEnabled {
				t.Error("PaymentEnabled = true, want false: member axis is not confirmed")
			}
			return nil
		},
	}

	pc := &mockPayuniClient{
		GetSellerStatusFn: func(ctx context.Context, merID string) (*SellerStatusResponse, error) {
			return &SellerStatusResponse{
				MerStatus: "1", MemberStatus: "9", CreditStatus: "0",
				ApplePayStatus: "0", GooglePayStatus: "0", ATMStatus: "0",
				RawResponse: json.RawMessage(`{}`),
			}, nil
		},
	}

	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID}, nil
		},
	}

	uc := &PayuniUseCase{payuniRepo: payuniRepo, payuniClient: pc, sellerRepo: sellerRepo}
	if err := uc.SyncPendingStatuses(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updatedPayuni {
		t.Fatal("expected payuniRepo.Update to be called")
	}
}

func TestPayuniUC_SyncPending_SkipsEmptyMerID(t *testing.T) {
	apiCalled := false

	payuniRepo := &mockPayuniRepo{
		GetPendingFn: func(ctx context.Context, limit int, staleBefore time.Time) ([]*domain.PayuniRegistration, error) {
			return []*domain.PayuniRegistration{
				{ID: 1, AccountID: 100, PayuniMerID: "", Status: domain.PayuniStatusPending},
			}, nil
		},
		UpdateFn: func(ctx context.Context, reg *domain.PayuniRegistration) error {
			return nil
		},
	}

	pc := &mockPayuniClient{
		GetSellerStatusFn: func(ctx context.Context, merID string) (*SellerStatusResponse, error) {
			apiCalled = true
			return nil, nil
		},
	}

	uc := &PayuniUseCase{payuniRepo: payuniRepo, payuniClient: pc}
	if err := uc.SyncPendingStatuses(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if apiCalled {
		t.Error("PAYUNi API should not be called for empty MerID")
	}
}

func TestPayuniUC_RefreshStatus_Success(t *testing.T) {
	var updated bool
	payuniRepo := &mockPayuniRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.PayuniRegistration, error) {
			return &domain.PayuniRegistration{
				AccountID: accountID, PayuniMerID: "HAWK2612345678", Status: domain.PayuniStatusPending,
			}, nil
		},
		UpdateFn: func(ctx context.Context, reg *domain.PayuniRegistration) error {
			updated = true
			if reg.MerStatus != "1" {
				t.Errorf("MerStatus = %q, want %q", reg.MerStatus, "1")
			}
			return nil
		},
	}

	pc := &mockPayuniClient{
		GetSellerStatusFn: func(ctx context.Context, merID string) (*SellerStatusResponse, error) {
			return &SellerStatusResponse{
				MerStatus: "1", MemberStatus: "1", CreditStatus: "1",
				ApplePayStatus: "0", GooglePayStatus: "0", ATMStatus: "0",
				RawResponse: json.RawMessage(`{}`),
			}, nil
		},
	}

	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID}, nil
		},
	}

	uc := &PayuniUseCase{payuniRepo: payuniRepo, payuniClient: pc, sellerRepo: sellerRepo}
	if err := uc.RefreshStatus(context.Background(), 100); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updated {
		t.Error("payuni registration was not updated")
	}
}

func TestPayuniUC_RefreshStatus_ActiveAdvancesCertViaFunnel(t *testing.T) {
	outboxRepo := &mockMerchantVerificationOutboxRepo{}
	var certWritten bool
	var savedReg *domain.PayuniRegistration
	payuniRepo := &mockPayuniRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.PayuniRegistration, error) {
			return &domain.PayuniRegistration{AccountID: accountID, ApplicationID: 10, PayuniMerID: "HAWK100", Status: domain.PayuniStatusPending}, nil
		},
		UpdateFn: func(ctx context.Context, reg *domain.PayuniRegistration) error {
			savedReg = reg
			return nil
		},
	}
	pc := &mockPayuniClient{
		GetSellerStatusFn: func(ctx context.Context, merID string) (*SellerStatusResponse, error) {
			return &SellerStatusResponse{MerStatus: "1", MemberStatus: "1", RawResponse: json.RawMessage(`{}`)}, nil
		},
	}
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{ID: id, AccountID: 100, Status: domain.CertStatusForwarded}
			app.InitFSM()
			return app, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error {
			certWritten = true
			return nil
		},
	}

	uc := &PayuniUseCase{payuniRepo: payuniRepo, payuniClient: pc, certRepo: certRepo, txManager: &mockTxManager{}, outboxRepo: outboxRepo, tsid: func() int64 { return 2290 }}
	if err := uc.RefreshStatus(context.Background(), 100); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if certWritten {
		t.Error("cert 被寫入了；PayUni 的審核結果不進 cert 工作流，顯示狀態由 deriveOverallStatus 推導")
	}
	if len(outboxRepo.Inserted) != 1 || outboxRepo.Inserted[0].EventType != "approved" {
		t.Errorf("outbox rows inserted = %+v, want one with EventType=approved", outboxRepo.Inserted)
	}
	if savedReg == nil || savedReg.Status != domain.PayuniStatusActive {
		t.Errorf("registration persisted = %+v, want status Active", savedReg)
	}
}

func TestPayuniUC_RefreshStatus_NoMerID(t *testing.T) {
	payuniRepo := &mockPayuniRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.PayuniRegistration, error) {
			return &domain.PayuniRegistration{AccountID: accountID, PayuniMerID: ""}, nil
		},
	}

	uc := &PayuniUseCase{payuniRepo: payuniRepo}
	err := uc.RefreshStatus(context.Background(), 100)
	if err == nil {
		t.Fatal("expected error for empty MerID")
	}
}

func TestPayuniUC_RefreshStatus_RateLimited_NoAPICall(t *testing.T) {
	var apiCalled bool
	var updateCalled bool
	payuniRepo := &mockPayuniRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.PayuniRegistration, error) {
			return &domain.PayuniRegistration{
				AccountID: accountID, PayuniMerID: "HAWK2612345678", Status: domain.PayuniStatusPending,
			}, nil
		},
		ClaimForSyncFn: func(ctx context.Context, id int64, claimedAt, staleBefore time.Time) (bool, error) {
			return false, nil
		},
		UpdateFn: func(ctx context.Context, reg *domain.PayuniRegistration) error {
			updateCalled = true
			return nil
		},
	}
	pc := &mockPayuniClient{
		GetSellerStatusFn: func(ctx context.Context, merID string) (*SellerStatusResponse, error) {
			apiCalled = true
			return &SellerStatusResponse{MerStatus: "1", MemberStatus: "1", RawResponse: json.RawMessage(`{}`)}, nil
		},
	}

	uc := &PayuniUseCase{payuniRepo: payuniRepo, payuniClient: pc}
	if err := uc.RefreshStatus(context.Background(), 100); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if apiCalled {
		t.Error("payuniClient.GetSellerStatus was called, want no call when rate-limited")
	}
	if updateCalled {
		t.Error("payuniRepo.Update was called, want no write when rate-limited")
	}
}

func TestPayuniUC_RefreshStatus_ClaimSucceeds_StillCallsAPI(t *testing.T) {
	var apiCalled bool
	payuniRepo := &mockPayuniRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.PayuniRegistration, error) {
			return &domain.PayuniRegistration{
				AccountID: accountID, PayuniMerID: "HAWK2612345678", Status: domain.PayuniStatusPending,
			}, nil
		},
		UpdateFn: func(ctx context.Context, reg *domain.PayuniRegistration) error { return nil },
	}
	pc := &mockPayuniClient{
		GetSellerStatusFn: func(ctx context.Context, merID string) (*SellerStatusResponse, error) {
			apiCalled = true
			return &SellerStatusResponse{MerStatus: "1", MemberStatus: "1", RawResponse: json.RawMessage(`{}`)}, nil
		},
	}

	uc := &PayuniUseCase{payuniRepo: payuniRepo, payuniClient: pc}
	if err := uc.RefreshStatus(context.Background(), 100); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !apiCalled {
		t.Error("payuniClient.GetSellerStatus was not called, want a call when the claim succeeds")
	}
}

func TestHandleAgentNotify_Active(t *testing.T) {
	var updatedPayuni bool

	payuniRepo := &mockPayuniRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.PayuniRegistration, error) {
			if accountID != 123 {
				t.Errorf("accountID = %d, want 123", accountID)
			}
			return &domain.PayuniRegistration{
				AccountID:     123,
				ApplicationID: 10,
				PayuniMerID:   "HAWK123",
				Status:        domain.PayuniStatusPending,
			}, nil
		},
		UpdateFn: func(ctx context.Context, reg *domain.PayuniRegistration) error {
			updatedPayuni = true
			if reg.Status != domain.PayuniStatusActive {
				t.Errorf("Status = %v, want Active", reg.Status)
			}
			return nil
		},
	}

	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID}, nil
		},
	}

	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{ID: id, Status: domain.CertStatusSubmitted}
			app.InitFSM()
			return app, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error {
			return nil
		},
	}

	uc := &PayuniUseCase{
		payuniRepo: payuniRepo, sellerRepo: sellerRepo, certRepo: certRepo,
		txManager: &mockTxManager{}, outboxRepo: &mockMerchantVerificationOutboxRepo{},
		tsid: func() int64 { return 1 },
	}
	payload := map[string]string{
		"MerStatus":       "1",
		"MemberStatus":    "1",
		"CreditStatus":    "0",
		"ApplePayStatus":  "0",
		"GooglePayStatus": "0",
		"ATMStatus":       "0",
	}
	if err := uc.HandleAgentNotify(context.Background(), "HAWK123", "1", payload); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updatedPayuni {
		t.Error("payuni registration was not updated")
	}
}

func TestHandleAgentNotify_Rejected(t *testing.T) {
	var updatedPayuni bool

	payuniRepo := &mockPayuniRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.PayuniRegistration, error) {
			return &domain.PayuniRegistration{
				AccountID:     123,
				ApplicationID: 10,
				Status:        domain.PayuniStatusPending,
			}, nil
		},
		UpdateFn: func(ctx context.Context, reg *domain.PayuniRegistration) error {
			updatedPayuni = true
			if reg.Status != domain.PayuniStatusFailed {
				t.Errorf("Status = %v, want Failed", reg.Status)
			}
			return nil
		},
	}

	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID}, nil
		},
	}

	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{ID: id, Status: domain.CertStatusForwarded}
			app.InitFSM()
			return app, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error {
			return nil
		},
	}

	uc := &PayuniUseCase{payuniRepo: payuniRepo, sellerRepo: sellerRepo, certRepo: certRepo, txManager: &mockTxManager{}, outboxRepo: &mockMerchantVerificationOutboxRepo{}, tsid: func() int64 { return 3530 }}
	if err := uc.HandleAgentNotify(context.Background(), "HAWK123", "92", map[string]string{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updatedPayuni {
		t.Error("payuni registration was not updated")
	}
}

func TestHandleAgentNotify_PendingInfo(t *testing.T) {
	var fsmTransitioned bool

	payuniRepo := &mockPayuniRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.PayuniRegistration, error) {
			return &domain.PayuniRegistration{AccountID: 123, ApplicationID: 10, Status: domain.PayuniStatusPending}, nil
		},
		UpdateFn: func(ctx context.Context, reg *domain.PayuniRegistration) error { return nil },
	}

	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID}, nil
		},
	}

	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{ID: id, AccountID: 123, Status: domain.CertStatusForwarded}
			app.InitFSM()
			return app, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error {
			fsmTransitioned = true
			return nil
		},
	}

	outboxRepo := &mockMerchantVerificationOutboxRepo{}
	uc := &PayuniUseCase{payuniRepo: payuniRepo, sellerRepo: sellerRepo, certRepo: certRepo, txManager: &mockTxManager{}, outboxRepo: outboxRepo, tsid: func() int64 { return 3930 }}
	if err := uc.HandleAgentNotify(context.Background(), "HAWK123", "91", map[string]string{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fsmTransitioned {
		t.Error("cert 不該被寫（PayUni 的審核結果不進工作流）")
	}
	if len(outboxRepo.Inserted) != 1 || outboxRepo.Inserted[0].EventType != "pending_info" {
		t.Errorf("outbox = %+v, want one pending_info event", outboxRepo.Inserted)
	}
}

func TestHandleAgentNotify_InvalidMerID(t *testing.T) {
	uc := &PayuniUseCase{}

	if err := uc.HandleAgentNotify(context.Background(), "INVALID123", "1", map[string]string{}); err == nil {
		t.Fatal("expected error for invalid merId format")
	}
	if err := uc.HandleAgentNotify(context.Background(), "HAWKabc", "1", map[string]string{}); err == nil {
		t.Fatal("expected error for non-numeric account_id")
	}
}

func TestHandleAgentNotify_UnknownMerStatus(t *testing.T) {
	payuniRepo := &mockPayuniRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.PayuniRegistration, error) {
			return &domain.PayuniRegistration{AccountID: 123, Status: domain.PayuniStatusPending}, nil
		},
	}

	uc := &PayuniUseCase{payuniRepo: payuniRepo}
	if err := uc.HandleAgentNotify(context.Background(), "HAWK123", "99999", map[string]string{}); err != nil {
		t.Fatalf("unexpected error for unknown MerStatus: %v", err)
	}
}

func TestDerivePayuniOutcome_TerminalMerchantStatusWinsRegardlessOfMember(t *testing.T) {
	memberStatuses := []string{"1", "5", "8", "9", "92", "99", ""}
	for _, merStatus := range []string{"2", "8", "92", "99"} {
		for _, memberStatus := range memberStatuses {
			got := derivePayuniOutcome(merStatus, memberStatus)
			if got != outcomeRejected {
				t.Errorf("derivePayuniOutcome(%q, %q) = %q, want %q (terminal MerStatus must win regardless of MemberStatus)",
					merStatus, memberStatus, got, outcomeRejected)
			}
		}
	}
}

func TestDerivePayuniOutcome_MerchantPendingInfoWinsRegardlessOfMember(t *testing.T) {
	memberStatuses := []string{"1", "5", "8", "9", "92", "99", ""}
	for _, memberStatus := range memberStatuses {
		got := derivePayuniOutcome("91", memberStatus)
		if got != outcomePendingInfo {
			t.Errorf("derivePayuniOutcome(\"91\", %q) = %q, want %q (merchant-level pending must win regardless of MemberStatus)",
				memberStatus, got, outcomePendingInfo)
		}
	}
}

func TestDerivePayuniOutcome_AmbiguousMatrix_Exhaustive(t *testing.T) {
	cases := []struct {
		merStatus    string
		memberStatus string
		want         payuniOutcome
		note         string
	}{
		{"1", "1", outcomeApproved, "both active → approved"},
		{"1", "5", outcomePendingInfo, "merchant active but member pending → promote to pending_info"},
		{"1", "8", outcomeRejected, "member denied is member-terminal → rejected (2026-09-10 fix, was under review)"},
		{"1", "9", outcomeUnderReview, "merchant active but member still under review → fail closed, not approved"},
		{"1", "92", outcomeRejected, "member review-failed is member-terminal → rejected (2026-09-10 fix, was under review)"},
		{"1", "99", outcomeRejected, "member suspended is member-terminal → rejected (2026-09-10 fix, was under review)"},
		{"1", "", outcomeUnderReview, "merchant active but member status empty → fail closed, not approved"},
		{"9", "1", outcomeUnderReview, "merchant under review, member active → still under review"},
		{"9", "5", outcomePendingInfo, "merchant under review but member pending → promote"},
		{"9", "8", outcomeRejected, "member denied is member-terminal → rejected (2026-09-10 fix, was under review)"},
		{"9", "9", outcomeUnderReview, "both under review → noop"},
		{"9", "92", outcomeRejected, "member review-failed is member-terminal → rejected (2026-09-10 fix, was under review)"},
		{"9", "99", outcomeRejected, "member suspended is member-terminal → rejected (2026-09-10 fix, was under review)"},
		{"9", "", outcomeUnderReview, "merchant under review, member status empty → under review"},
	}
	for _, tc := range cases {
		got := derivePayuniOutcome(tc.merStatus, tc.memberStatus)
		if got != tc.want {
			t.Errorf("%s: derivePayuniOutcome(%q, %q) = %q, want %q",
				tc.note, tc.merStatus, tc.memberStatus, got, tc.want)
		}
	}
	for _, merStatus := range []string{"1", "9"} {
		for _, memberStatus := range []string{"1", "5", "9", ""} {
			if _, ok := payuniAmbiguousOutcomes[[2]string{merStatus, memberStatus}]; !ok {
				t.Errorf("payuniAmbiguousOutcomes missing explicit entry for (%q, %q)", merStatus, memberStatus)
			}
		}
	}
	if len(payuniAmbiguousOutcomes) != 8 {
		t.Errorf("payuniAmbiguousOutcomes has %d entries, want exactly 8", len(payuniAmbiguousOutcomes))
	}
}

func TestDerivePayuniOutcome_MemberTerminalStates(t *testing.T) {
	cases := []struct {
		merStatus, memberStatus string
		want                    payuniOutcome
	}{
		{"1", "8", outcomeRejected},
		{"1", "92", outcomeRejected},
		{"1", "99", outcomeRejected},
		{"9", "8", outcomeRejected},
		{"9", "92", outcomeRejected},
		{"9", "99", outcomeRejected},
		{"2", "8", outcomeRejected},
		{"91", "8", outcomePendingInfo},
		{"1", "1", outcomeApproved},
		{"1", "5", outcomePendingInfo},
		{"9", "5", outcomePendingInfo},
		{"9", "9", outcomeUnderReview},
	}
	for _, tc := range cases {
		if got := derivePayuniOutcome(tc.merStatus, tc.memberStatus); got != tc.want {
			t.Errorf("derivePayuniOutcome(%q, %q) = %v, want %v", tc.merStatus, tc.memberStatus, got, tc.want)
		}
	}
}

func TestDerivePayuniOutcome_UnrecognizedMerStatus_FailsClosed(t *testing.T) {
	cases := []struct{ merStatus, memberStatus string }{
		{"", ""},
		{"unknown", "1"},
	}
	for _, tc := range cases {
		got := derivePayuniOutcome(tc.merStatus, tc.memberStatus)
		if got != payuniOutcome("") {
			t.Errorf("derivePayuniOutcome(%q, %q) = %q, want the zero value (unrecognized)", tc.merStatus, tc.memberStatus, got)
		}
	}
}

func TestDerivePayuniOutcome_UndocumentedMemberStatusOnAmbiguousMerStatus_FallsBackToUnderReview(t *testing.T) {
	cases := []struct{ merStatus, memberStatus string }{
		{"1", "77"},
		{"9", "77"},
	}
	for _, tc := range cases {
		got := derivePayuniOutcome(tc.merStatus, tc.memberStatus)
		if got != outcomeUnderReview {
			t.Errorf("derivePayuniOutcome(%q, %q) = %q, want %q (undocumented MemberStatus on ambiguous MerStatus must fail closed to under review, not to fully unrecognized)",
				tc.merStatus, tc.memberStatus, got, outcomeUnderReview)
		}
	}
}

func TestDerivePayuniOutcome_UndocumentedMerStatusWithMemberPending_ResolvesToPendingInfo(t *testing.T) {
	cases := []struct {
		merStatus    string
		memberStatus string
		want         payuniOutcome
		note         string
	}{
		{"77", "5", outcomePendingInfo, "undocumented MerStatus + member pending → pending_info"},
		{"unknown", "5", outcomePendingInfo, "undocumented MerStatus + member pending → pending_info"},
		{"77", "1", payuniOutcome(""), "undocumented MerStatus + non-'5' MemberStatus → fully unrecognized"},
		{"unknown", "9", payuniOutcome(""), "undocumented MerStatus + non-'5' MemberStatus → fully unrecognized"},
	}
	for _, tc := range cases {
		got := derivePayuniOutcome(tc.merStatus, tc.memberStatus)
		if got != tc.want {
			t.Errorf("%s: derivePayuniOutcome(%q, %q) = %q, want %q",
				tc.note, tc.merStatus, tc.memberStatus, got, tc.want)
		}
	}
}

func obsOf(merStatus, memberStatus string) domain.PayuniStatusObservation {
	return domain.PayuniStatusObservation{
		ObservedAt: time.Now(), Source: "test",
		MerStatus: merStatus, MemberStatus: memberStatus,
	}
}

func TestApplyOutcome_ContradictorySnapshot_LogsWithheldApproval(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	restore := zap.ReplaceGlobals(zap.New(core))
	defer restore()

	payuniRepo := &mockPayuniRepo{
		UpdateFn: func(ctx context.Context, reg *domain.PayuniRegistration) error { return nil },
	}
	uc := &PayuniUseCase{payuniRepo: payuniRepo}
	info := &domain.PayuniRegistration{
		AccountID: 100, PayuniMerID: "HAWK100", MerStatus: "1", MemberStatus: "9",
	}
	uc.ReapplyOutcome(context.Background(), info, time.Now(), "test")

	found := false
	for _, entry := range logs.All() {
		if entry.Message == "PAYUNi approval withheld: merchant approved but member status unconfirmed" {
			found = true
		}
	}
	if !found {
		t.Error("expected a withheld-approval warning log, got none")
	}
}

func TestApplyOutcome_OrdinaryUnderReview_DoesNotLog(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	restore := zap.ReplaceGlobals(zap.New(core))
	defer restore()

	payuniRepo := &mockPayuniRepo{
		UpdateFn: func(ctx context.Context, reg *domain.PayuniRegistration) error { return nil },
	}
	uc := &PayuniUseCase{payuniRepo: payuniRepo}
	info := &domain.PayuniRegistration{
		AccountID: 100, PayuniMerID: "HAWK100", MerStatus: "9", MemberStatus: "9",
	}
	uc.ReapplyOutcome(context.Background(), info, time.Now(), "test")

	if len(logs.All()) != 0 {
		t.Errorf("expected no logs for ordinary under-review, got %d: %v", len(logs.All()), logs.All())
	}
}

func decodeVerificationEvent(t *testing.T, row *domain.MerchantVerificationOutbox) domain.MerchantVerificationCompletedEvent {
	t.Helper()
	var event domain.MerchantVerificationCompletedEvent
	if err := json.Unmarshal(row.Payload, &event); err != nil {
		t.Fatalf("outbox payload did not unmarshal: %v", err)
	}
	return event
}

func TestApplyOutcome_UnderReview_PendingInfoCertAndChanged_InsertsHistoryAndOutbox(t *testing.T) {
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			return &domain.CertificationApplication{ID: id, AccountID: 100, Status: domain.CertStatusForwarded}, nil
		},
	}
	var historyCalls int
	payuniRepo := &mockPayuniRepo{
		InsertStatusHistoryFn: func(ctx context.Context, h *domain.PayuniStatusHistory) error {
			historyCalls++
			return nil
		},
	}
	outboxRepo := &mockMerchantVerificationOutboxRepo{}
	uc := &PayuniUseCase{
		certRepo: certRepo, payuniRepo: payuniRepo, outboxRepo: outboxRepo,
		txManager: &mockTxManager{}, tsid: func() int64 { return 555 },
	}
	const wantStoreID int64 = 7300000000000000201
	info := &domain.PayuniRegistration{AccountID: 100, StoreID: wantStoreID, ApplicationID: 10, PayuniMerID: "HAWK100", MerStatus: "91"}

	ok := uc.ApplyObservation(context.Background(), info, obsOf("9", "9"))

	if !ok {
		t.Fatal("ApplyObservation must return true for outcomeUnderReview")
	}
	if historyCalls != 1 {
		t.Errorf("InsertStatusHistory calls = %d, want 1", historyCalls)
	}
	if len(outboxRepo.Inserted) != 1 {
		t.Fatalf("outbox rows inserted = %d, want 1", len(outboxRepo.Inserted))
	}
	if outboxRepo.Inserted[0].EventType != "under_review" {
		t.Errorf("EventType = %q, want under_review", outboxRepo.Inserted[0].EventType)
	}
	if event := decodeVerificationEvent(t, outboxRepo.Inserted[0]); event.StoreID != wantStoreID {
		t.Errorf("outbox payload StoreID = %d, want %d (info.StoreID)", event.StoreID, wantStoreID)
	}
}

func TestApplyOutcome_UnderReview_ChangedButNotKYC_NoInsert(t *testing.T) {
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			t.Fatal("GetApplicationByID must not be called when kycChanged=false")
			return nil, nil
		},
	}
	var historyCalls int
	payuniRepo := &mockPayuniRepo{
		InsertStatusHistoryFn: func(ctx context.Context, h *domain.PayuniStatusHistory) error {
			historyCalls++
			return nil
		},
	}
	outboxRepo := &mockMerchantVerificationOutboxRepo{}
	uc := &PayuniUseCase{
		certRepo: certRepo, payuniRepo: payuniRepo, outboxRepo: outboxRepo,
		txManager: &mockTxManager{}, tsid: func() int64 { return 1 },
	}
	info := &domain.PayuniRegistration{AccountID: 100, ApplicationID: 10, PayuniMerID: "HAWK100", MerStatus: "9", MemberStatus: "9", ApplePayStatus: "0"}

	obs := obsOf("9", "9")
	obs.ApplePayStatus = "1"
	ok := uc.ApplyObservation(context.Background(), info, obs)

	if !ok {
		t.Fatal("ApplyObservation must return true for outcomeUnderReview")
	}
	if historyCalls != 1 {
		t.Errorf("InsertStatusHistory calls = %d, want 1 (changed=true still records the audit trail)", historyCalls)
	}
	if len(outboxRepo.Inserted) != 0 {
		t.Fatalf("outbox rows inserted = %d, want 0 (kycChanged=false, payment-channel-only change must not notify)", len(outboxRepo.Inserted))
	}
}

func TestApplyOutcome_UnderReview_NotChanged_NoInsert(t *testing.T) {
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			t.Fatal("GetApplicationByID must not be called when changed=false")
			return nil, nil
		},
	}
	var historyCalls int
	payuniRepo := &mockPayuniRepo{
		InsertStatusHistoryFn: func(ctx context.Context, h *domain.PayuniStatusHistory) error {
			historyCalls++
			return nil
		},
	}
	outboxRepo := &mockMerchantVerificationOutboxRepo{}
	uc := &PayuniUseCase{
		certRepo: certRepo, payuniRepo: payuniRepo, outboxRepo: outboxRepo,
		txManager: &mockTxManager{}, tsid: func() int64 { return 1 },
	}
	info := &domain.PayuniRegistration{AccountID: 100, ApplicationID: 10, PayuniMerID: "HAWK100", MerStatus: "9", MemberStatus: "9"}

	uc.ApplyObservation(context.Background(), info, obsOf("9", "9"))

	if historyCalls != 0 {
		t.Errorf("InsertStatusHistory calls = %d, want 0 (changed=false, no new observation)", historyCalls)
	}
	if len(outboxRepo.Inserted) != 0 {
		t.Fatalf("outbox rows inserted = %d, want 0 (changed=false, no new observation)", len(outboxRepo.Inserted))
	}
}

func TestApplyOutcome_UnderReview_CertNotPendingInfo_NoInsert(t *testing.T) {
	for _, status := range []domain.CertificationStatus{domain.CertStatusForwarded, domain.CertStatusRejected} {
		t.Run(status.String(), func(t *testing.T) {
			certRepo := &mockCertificationRepo{
				GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
					return &domain.CertificationApplication{ID: id, AccountID: 100, Status: status}, nil
				},
			}
			var historyCalls int
			payuniRepo := &mockPayuniRepo{
				InsertStatusHistoryFn: func(ctx context.Context, h *domain.PayuniStatusHistory) error {
					historyCalls++
					return nil
				},
			}
			outboxRepo := &mockMerchantVerificationOutboxRepo{}
			uc := &PayuniUseCase{
				certRepo: certRepo, payuniRepo: payuniRepo, outboxRepo: outboxRepo,
				txManager: &mockTxManager{}, tsid: func() int64 { return 1 },
			}
			info := &domain.PayuniRegistration{AccountID: 100, ApplicationID: 10, PayuniMerID: "HAWK100"}

			uc.ApplyObservation(context.Background(), info, obsOf("9", "9"))

			if historyCalls != 1 {
				t.Errorf("cert status=%v: InsertStatusHistory calls = %d, want 1 (audit trail records every changed observation)", status, historyCalls)
			}
			if len(outboxRepo.Inserted) != 0 {
				t.Fatalf("cert status=%v: outbox rows inserted = %d, want 0 (not pending_info)", status, len(outboxRepo.Inserted))
			}
		})
	}
}

func TestPayuniStatusCode(t *testing.T) {
	cases := []struct {
		merStatus, memberStatus string
		want                    string
	}{
		{"92", "1", "MER_92"},
		{"2", "1", "MER_2"},
		{"99", "1", "MER_99"},
		{"8", "1", "MER_8"},
		{"91", "1", "MER_91"},
		{"1", "8", "MEMBER_8"},
		{"1", "92", "MEMBER_92"},
		{"1", "99", "MEMBER_99"},
		{"1", "5", "MEMBER_5"},
		{"9", "5", "MEMBER_5"},
		{"1", "1", "MER_1"}, // merStatus 非空但兩個 switch 都吃不到
		{"", "1", ""},       // merStatus 與 memberStatus 都吃不到——完全無法辨識
	}
	for _, tc := range cases {
		if got := payuniStatusCode(tc.merStatus, tc.memberStatus); got != tc.want {
			t.Errorf("payuniStatusCode(%q, %q) = %q, want %q", tc.merStatus, tc.memberStatus, got, tc.want)
		}
	}
}

func TestHandleAgentNotify_MemberPendingInfoPromotesToPendingInfo(t *testing.T) {
	var certWritten bool

	payuniRepo := &mockPayuniRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.PayuniRegistration, error) {
			return &domain.PayuniRegistration{AccountID: 123, ApplicationID: 10, Status: domain.PayuniStatusPending}, nil
		},
		UpdateFn: func(ctx context.Context, reg *domain.PayuniRegistration) error { return nil },
	}
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID}, nil
		},
	}
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{ID: id, AccountID: 123, Status: domain.CertStatusForwarded}
			app.InitFSM()
			return app, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error {
			certWritten = true
			return nil
		},
	}

	outboxRepo := &mockMerchantVerificationOutboxRepo{}
	uc := &PayuniUseCase{payuniRepo: payuniRepo, sellerRepo: sellerRepo, certRepo: certRepo, txManager: &mockTxManager{}, outboxRepo: outboxRepo, tsid: func() int64 { return 7460 }}
	err := uc.HandleAgentNotify(context.Background(), "HAWK123", "9", map[string]string{
		"MemberStatus": "5",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if certWritten {
		t.Error("cert 不該被寫")
	}
	if len(outboxRepo.Inserted) != 1 || outboxRepo.Inserted[0].EventType != "pending_info" {
		t.Errorf("outbox = %+v, want one pending_info event", outboxRepo.Inserted)
	}
}

func TestHandleAgentNotify_UnderReviewNoTransition(t *testing.T) {
	var fsmTransitioned bool

	payuniRepo := &mockPayuniRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.PayuniRegistration, error) {
			return &domain.PayuniRegistration{AccountID: 123, ApplicationID: 10, Status: domain.PayuniStatusPending}, nil
		},
		UpdateFn: func(ctx context.Context, reg *domain.PayuniRegistration) error { return nil },
	}
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID}, nil
		},
	}
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			return &domain.CertificationApplication{ID: id, AccountID: 123, Status: domain.CertStatusForwarded}, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error {
			fsmTransitioned = true
			return nil
		},
	}

	uc := &PayuniUseCase{payuniRepo: payuniRepo, sellerRepo: sellerRepo, certRepo: certRepo}
	if err := uc.HandleAgentNotify(context.Background(), "HAWK123", "9", map[string]string{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fsmTransitioned {
		t.Error("cert FSM should NOT transition while merchant is still under review")
	}
}

func TestSyncPending_AppendsStatusHistoryWithReason(t *testing.T) {
	var historyEntries []*domain.PayuniStatusHistory
	var registrationReason string

	payuniRepo := &mockPayuniRepo{
		GetPendingFn: func(ctx context.Context, limit int, staleBefore time.Time) ([]*domain.PayuniRegistration, error) {
			return []*domain.PayuniRegistration{
				{ID: 1, AccountID: 100, PayuniMerID: "HAWK100", Status: domain.PayuniStatusPending},
			}, nil
		},
		UpdateFn: func(ctx context.Context, reg *domain.PayuniRegistration) error {
			registrationReason = reg.Reason
			return nil
		},
		InsertStatusHistoryFn: func(ctx context.Context, h *domain.PayuniStatusHistory) error {
			historyEntries = append(historyEntries, h)
			return nil
		},
	}

	pc := &mockPayuniClient{
		GetSellerStatusFn: func(ctx context.Context, merID string) (*SellerStatusResponse, error) {
			return &SellerStatusResponse{
				MerStatus:    "91",
				MemberStatus: "1",
				Reason:       "請補上負責人身分證背面",
				RawResponse:  json.RawMessage(`{"MerStatus":"91","Reason":"請補上負責人身分證背面"}`),
			}, nil
		},
	}

	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID}, nil
		},
	}

	uc := &PayuniUseCase{
		payuniRepo:   payuniRepo,
		payuniClient: pc,
		sellerRepo:   sellerRepo,
		tsid:         func() int64 { return 7777 },
	}
	if err := uc.SyncPendingStatuses(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(historyEntries) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(historyEntries))
	}
	h := historyEntries[0]
	if h.Source != "sync" {
		t.Errorf("history.Source = %q, want sync", h.Source)
	}
	if h.AccountID != 100 {
		t.Errorf("history.AccountID = %d, want 100", h.AccountID)
	}
	if h.MerStatus != "91" {
		t.Errorf("history.MerStatus = %q, want 91", h.MerStatus)
	}
	if h.Reason != "請補上負責人身分證背面" {
		t.Errorf("history.Reason = %q, want '請補上負責人身分證背面'", h.Reason)
	}
	if registrationReason != "請補上負責人身分證背面" {
		t.Errorf("registration.Reason = %q, want '請補上負責人身分證背面'", registrationReason)
	}
}

func TestHandleAgentNotify_DeniedAsRejected(t *testing.T) {
	var certWritten bool

	payuniRepo := &mockPayuniRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.PayuniRegistration, error) {
			return &domain.PayuniRegistration{AccountID: 123, ApplicationID: 10, Status: domain.PayuniStatusPending}, nil
		},
		UpdateFn: func(ctx context.Context, reg *domain.PayuniRegistration) error { return nil },
	}
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID}, nil
		},
	}
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{ID: id, AccountID: 123, Status: domain.CertStatusForwarded}
			app.InitFSM()
			return app, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error {
			certWritten = true
			return nil
		},
	}

	outboxRepo := &mockMerchantVerificationOutboxRepo{}
	uc := &PayuniUseCase{payuniRepo: payuniRepo, sellerRepo: sellerRepo, certRepo: certRepo, txManager: &mockTxManager{}, outboxRepo: outboxRepo, tsid: func() int64 { return 8940 }}
	if err := uc.HandleAgentNotify(context.Background(), "HAWK123", "8", map[string]string{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if certWritten {
		t.Error("cert 不該被寫")
	}
	if len(outboxRepo.Inserted) != 1 || outboxRepo.Inserted[0].EventType != "rejected" {
		t.Errorf("outbox = %+v, want one rejected event", outboxRepo.Inserted)
	}
}

func TestSyncPending_MemberPendingOnReconciledCert_NoReNotify(t *testing.T) {
	for _, certState := range []domain.CertificationStatus{domain.CertStatusForwarded} {
		t.Run(certState.String(), func(t *testing.T) {
			outboxRepo := &mockMerchantVerificationOutboxRepo{}
			payuniRepo := &mockPayuniRepo{
				GetPendingFn: func(ctx context.Context, limit int, staleBefore time.Time) ([]*domain.PayuniRegistration, error) {
					return []*domain.PayuniRegistration{
						{ID: 1, AccountID: 100, ApplicationID: 10, PayuniMerID: "HAWK100", Status: domain.PayuniStatusPending, MerStatus: "9", MemberStatus: "5"},
					}, nil
				},
				UpdateFn: func(ctx context.Context, reg *domain.PayuniRegistration) error { return nil },
			}
			pc := &mockPayuniClient{
				GetSellerStatusFn: func(ctx context.Context, merID string) (*SellerStatusResponse, error) {
					return &SellerStatusResponse{MerStatus: "9", MemberStatus: "5", RawResponse: json.RawMessage(`{}`)}, nil
				},
			}
			sellerRepo := &mockSellerRepo{
				GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
					return &domain.Seller{AccountID: accountID}, nil
				},
			}
			certRepo := &mockCertificationRepo{
				GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
					app := &domain.CertificationApplication{ID: id, AccountID: 100, Status: certState}
					app.InitFSM()
					return app, nil
				},
				UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error {
					t.Errorf("UpdateApplication must not be called when cert already %v", certState)
					return nil
				},
			}

			uc := &PayuniUseCase{payuniRepo: payuniRepo, payuniClient: pc, sellerRepo: sellerRepo, certRepo: certRepo, txManager: &mockTxManager{}, outboxRepo: outboxRepo}
			if err := uc.SyncPendingStatuses(context.Background()); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(outboxRepo.Inserted) != 0 {
				t.Errorf("verification events = %v, want none (already-reconciled cert must not re-notify)", outboxRepo.Inserted)
			}
		})
	}
}

func TestSyncPending_MemberPendingOnForwardedCert_TransitionsAndNotifiesOnce(t *testing.T) {
	outboxRepo := &mockMerchantVerificationOutboxRepo{}
	var certWritten bool
	const wantStoreID int64 = 7300000000000000301
	payuniRepo := &mockPayuniRepo{
		GetPendingFn: func(ctx context.Context, limit int, staleBefore time.Time) ([]*domain.PayuniRegistration, error) {
			return []*domain.PayuniRegistration{
				{ID: 1, AccountID: 100, StoreID: wantStoreID, ApplicationID: 10, PayuniMerID: "HAWK100", Status: domain.PayuniStatusPending},
			}, nil
		},
		UpdateFn: func(ctx context.Context, reg *domain.PayuniRegistration) error { return nil },
	}
	pc := &mockPayuniClient{
		GetSellerStatusFn: func(ctx context.Context, merID string) (*SellerStatusResponse, error) {
			return &SellerStatusResponse{MerStatus: "9", MemberStatus: "5", RawResponse: json.RawMessage(`{}`)}, nil
		},
	}
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID}, nil
		},
	}
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{ID: id, AccountID: 100, Status: domain.CertStatusForwarded}
			app.InitFSM()
			return app, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error {
			certWritten = true
			return nil
		},
	}

	uc := &PayuniUseCase{payuniRepo: payuniRepo, payuniClient: pc, sellerRepo: sellerRepo, certRepo: certRepo, txManager: &mockTxManager{}, outboxRepo: outboxRepo, tsid: func() int64 { return 9900 }}
	if err := uc.SyncPendingStatuses(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if certWritten {
		t.Error("cert 不該被寫（PayUni 的審核結果不進工作流，顯示狀態由 deriveOverallStatus 推導）")
	}
	if len(outboxRepo.Inserted) != 1 || outboxRepo.Inserted[0].EventType != "pending_info" {
		t.Errorf("outbox rows inserted = %+v, want one with EventType=pending_info", outboxRepo.Inserted)
	}
	if len(outboxRepo.Inserted) == 1 {
		event := decodeVerificationEvent(t, outboxRepo.Inserted[0])
		if event.StoreID != wantStoreID {
			t.Errorf("outbox payload StoreID = %d, want %d (info.StoreID)", event.StoreID, wantStoreID)
		}
		if want := codemeta.PayuniSellerFacingErrorLabel("MEMBER_5"); event.Reason != want {
			t.Errorf("outbox payload Reason = %q, want catalog label %q (empty PayUni Reason must not produce a blank notification)", event.Reason, want)
		}
	}
}

func TestSyncPending_ActiveOnForwardedCert_TransitionsToApprovedAndNotifies(t *testing.T) {
	outboxRepo := &mockMerchantVerificationOutboxRepo{}
	var certWritten bool
	const wantStoreID int64 = 7300000000000000302
	payuniRepo := &mockPayuniRepo{
		GetPendingFn: func(ctx context.Context, limit int, staleBefore time.Time) ([]*domain.PayuniRegistration, error) {
			return []*domain.PayuniRegistration{
				{ID: 1, AccountID: 100, StoreID: wantStoreID, ApplicationID: 10, PayuniMerID: "HAWK100", Status: domain.PayuniStatusPending},
			}, nil
		},
		UpdateFn: func(ctx context.Context, reg *domain.PayuniRegistration) error { return nil },
	}
	pc := &mockPayuniClient{
		GetSellerStatusFn: func(ctx context.Context, merID string) (*SellerStatusResponse, error) {
			return &SellerStatusResponse{MerStatus: "1", MemberStatus: "1", RawResponse: json.RawMessage(`{}`)}, nil
		},
	}
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID}, nil
		},
	}
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{ID: id, AccountID: 100, Status: domain.CertStatusForwarded}
			app.InitFSM()
			return app, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error {
			certWritten = true
			return nil
		},
	}

	uc := &PayuniUseCase{payuniRepo: payuniRepo, payuniClient: pc, sellerRepo: sellerRepo, certRepo: certRepo, txManager: &mockTxManager{}, outboxRepo: outboxRepo, tsid: func() int64 { return 10380 }}
	if err := uc.SyncPendingStatuses(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if certWritten {
		t.Error("cert 不該被寫（PayUni 的審核結果不進工作流，顯示狀態由 deriveOverallStatus 推導）")
	}
	if len(outboxRepo.Inserted) != 1 || outboxRepo.Inserted[0].EventType != "approved" {
		t.Errorf("outbox rows inserted = %+v, want one with EventType=approved", outboxRepo.Inserted)
	}
	if len(outboxRepo.Inserted) == 1 {
		if event := decodeVerificationEvent(t, outboxRepo.Inserted[0]); event.StoreID != wantStoreID {
			t.Errorf("outbox payload StoreID = %d, want %d (info.StoreID)", event.StoreID, wantStoreID)
		}
	}
}

func TestSyncPending_RejectedOnForwardedCert_TransitionsToRejectedAndNotifies(t *testing.T) {
	outboxRepo := &mockMerchantVerificationOutboxRepo{}
	var certWritten bool
	var savedReg *domain.PayuniRegistration
	const wantStoreID int64 = 7300000000000000303
	payuniRepo := &mockPayuniRepo{
		GetPendingFn: func(ctx context.Context, limit int, staleBefore time.Time) ([]*domain.PayuniRegistration, error) {
			return []*domain.PayuniRegistration{
				{ID: 1, AccountID: 100, StoreID: wantStoreID, ApplicationID: 10, PayuniMerID: "HAWK100", Status: domain.PayuniStatusPending},
			}, nil
		},
		UpdateFn: func(ctx context.Context, reg *domain.PayuniRegistration) error {
			savedReg = reg
			return nil
		},
	}
	pc := &mockPayuniClient{
		GetSellerStatusFn: func(ctx context.Context, merID string) (*SellerStatusResponse, error) {
			return &SellerStatusResponse{MerStatus: "92", MemberStatus: "1", RawResponse: json.RawMessage(`{}`)}, nil
		},
	}
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID}, nil
		},
	}
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{ID: id, AccountID: 100, Status: domain.CertStatusForwarded}
			app.InitFSM()
			return app, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error {
			certWritten = true
			return nil
		},
	}

	uc := &PayuniUseCase{payuniRepo: payuniRepo, payuniClient: pc, sellerRepo: sellerRepo, certRepo: certRepo, txManager: &mockTxManager{}, outboxRepo: outboxRepo, tsid: func() int64 { return 10910 }}
	if err := uc.SyncPendingStatuses(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if certWritten {
		t.Error("cert 不該被寫（PayUni 的審核結果不進工作流，顯示狀態由 deriveOverallStatus 推導）")
	}
	if len(outboxRepo.Inserted) != 1 || outboxRepo.Inserted[0].EventType != "rejected" {
		t.Errorf("outbox rows inserted = %+v, want one with EventType=rejected", outboxRepo.Inserted)
	}
	if savedReg == nil || savedReg.ErrorCode != "MER_92" {
		t.Errorf("registration ErrorCode = %q, want 'MER_92'", savedReg.ErrorCode)
	}
	if len(outboxRepo.Inserted) == 1 {
		if event := decodeVerificationEvent(t, outboxRepo.Inserted[0]); event.StoreID != wantStoreID {
			t.Errorf("outbox payload StoreID = %d, want %d (info.StoreID) — this is the PayUni-driven auto-reject path, distinct from admin Reject", event.StoreID, wantStoreID)
		}
	}
}

func TestSyncPending_ClosedOnForwardedCert_TransitionsToRejectedAndNotifies(t *testing.T) {
	outboxRepo := &mockMerchantVerificationOutboxRepo{}
	var certWritten bool
	payuniRepo := &mockPayuniRepo{
		GetPendingFn: func(ctx context.Context, limit int, staleBefore time.Time) ([]*domain.PayuniRegistration, error) {
			return []*domain.PayuniRegistration{
				{ID: 1, AccountID: 100, ApplicationID: 10, PayuniMerID: "HAWK100", Status: domain.PayuniStatusPending},
			}, nil
		},
		UpdateFn: func(ctx context.Context, reg *domain.PayuniRegistration) error { return nil },
	}
	pc := &mockPayuniClient{
		GetSellerStatusFn: func(ctx context.Context, merID string) (*SellerStatusResponse, error) {
			return &SellerStatusResponse{MerStatus: "2", MemberStatus: "1", RawResponse: json.RawMessage(`{}`)}, nil
		},
	}
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID}, nil
		},
	}
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{ID: id, AccountID: 100, Status: domain.CertStatusForwarded}
			app.InitFSM()
			return app, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error {
			certWritten = true
			return nil
		},
	}

	uc := &PayuniUseCase{payuniRepo: payuniRepo, payuniClient: pc, sellerRepo: sellerRepo, certRepo: certRepo, txManager: &mockTxManager{}, outboxRepo: outboxRepo, tsid: func() int64 { return 11440 }}
	if err := uc.SyncPendingStatuses(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if certWritten {
		t.Error("cert 不該被寫（PayUni 的審核結果不進工作流，顯示狀態由 deriveOverallStatus 推導）")
	}
	if len(outboxRepo.Inserted) != 1 || outboxRepo.Inserted[0].EventType != "rejected" {
		t.Errorf("outbox rows inserted = %+v, want one with EventType=rejected", outboxRepo.Inserted)
	}
}

func TestSyncPending_OtherClosedOnForwardedCert_TransitionsToRejectedAndNotifies(t *testing.T) {
	outboxRepo := &mockMerchantVerificationOutboxRepo{}
	var certWritten bool
	payuniRepo := &mockPayuniRepo{
		GetPendingFn: func(ctx context.Context, limit int, staleBefore time.Time) ([]*domain.PayuniRegistration, error) {
			return []*domain.PayuniRegistration{
				{ID: 1, AccountID: 100, ApplicationID: 10, PayuniMerID: "HAWK100", Status: domain.PayuniStatusPending},
			}, nil
		},
		UpdateFn: func(ctx context.Context, reg *domain.PayuniRegistration) error { return nil },
	}
	pc := &mockPayuniClient{
		GetSellerStatusFn: func(ctx context.Context, merID string) (*SellerStatusResponse, error) {
			return &SellerStatusResponse{MerStatus: "99", MemberStatus: "1", RawResponse: json.RawMessage(`{}`)}, nil
		},
	}
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID}, nil
		},
	}
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{ID: id, AccountID: 100, Status: domain.CertStatusForwarded}
			app.InitFSM()
			return app, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error {
			certWritten = true
			return nil
		},
	}

	uc := &PayuniUseCase{payuniRepo: payuniRepo, payuniClient: pc, sellerRepo: sellerRepo, certRepo: certRepo, txManager: &mockTxManager{}, outboxRepo: outboxRepo, tsid: func() int64 { return 11890 }}
	if err := uc.SyncPendingStatuses(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if certWritten {
		t.Error("cert 不該被寫（PayUni 的審核結果不進工作流，顯示狀態由 deriveOverallStatus 推導）")
	}
	if len(outboxRepo.Inserted) != 1 || outboxRepo.Inserted[0].EventType != "rejected" {
		t.Errorf("outbox rows inserted = %+v, want one with EventType=rejected", outboxRepo.Inserted)
	}
}

func TestSyncPending_MemberTerminalRejectedOnForwardedCert_TransitionsToRejectedAndNotifies(t *testing.T) {
	outboxRepo := &mockMerchantVerificationOutboxRepo{}
	var certWritten bool
	var savedReg *domain.PayuniRegistration
	const wantStoreID int64 = 7300000000000000404
	payuniRepo := &mockPayuniRepo{
		GetPendingFn: func(ctx context.Context, limit int, staleBefore time.Time) ([]*domain.PayuniRegistration, error) {
			return []*domain.PayuniRegistration{
				{ID: 1, AccountID: 100, StoreID: wantStoreID, ApplicationID: 10, PayuniMerID: "HAWK100", Status: domain.PayuniStatusPending},
			}, nil
		},
		UpdateFn: func(ctx context.Context, reg *domain.PayuniRegistration) error {
			savedReg = reg
			return nil
		},
	}
	pc := &mockPayuniClient{
		GetSellerStatusFn: func(ctx context.Context, merID string) (*SellerStatusResponse, error) {
			return &SellerStatusResponse{MerStatus: "1", MemberStatus: "8", RawResponse: json.RawMessage(`{}`)}, nil
		},
	}
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID}, nil
		},
	}
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{ID: id, AccountID: 100, Status: domain.CertStatusForwarded}
			app.InitFSM()
			return app, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error {
			certWritten = true
			return nil
		},
	}

	uc := &PayuniUseCase{payuniRepo: payuniRepo, payuniClient: pc, sellerRepo: sellerRepo, certRepo: certRepo, txManager: &mockTxManager{}, outboxRepo: outboxRepo, tsid: func() int64 { return 12200 }}
	if err := uc.SyncPendingStatuses(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if certWritten {
		t.Error("cert 不該被寫（member-terminal 仍要發 rejected 通知，但不進工作流）")
	}
	if len(outboxRepo.Inserted) != 1 || outboxRepo.Inserted[0].EventType != "rejected" {
		t.Errorf("outbox rows inserted = %+v, want one with EventType=rejected", outboxRepo.Inserted)
	}
	if savedReg == nil {
		t.Fatal("registration was not saved")
	}
	if savedReg.Status != domain.PayuniStatusFailed {
		t.Errorf("registration Status = %v, want PayuniStatusFailed", savedReg.Status)
	}
	if savedReg.ErrorCode != "MEMBER_8" {
		t.Errorf("registration ErrorCode = %q, want 'MEMBER_8' (member-level terminal code, not a MerStatus=1 carryover)", savedReg.ErrorCode)
	}
	if len(outboxRepo.Inserted) == 1 {
		event := decodeVerificationEvent(t, outboxRepo.Inserted[0])
		if want := codemeta.PayuniSellerFacingErrorLabel("MEMBER_8"); event.Reason != want {
			t.Errorf("outbox payload Reason = %q, want catalog label %q", event.Reason, want)
		}
	}
	if len(outboxRepo.Inserted) == 1 {
		if event := decodeVerificationEvent(t, outboxRepo.Inserted[0]); event.StoreID != wantStoreID {
			t.Errorf("outbox payload StoreID = %d, want %d (info.StoreID)", event.StoreID, wantStoreID)
		}
	}
}

func TestHandleAgentNotify_MemberPendingOnApprovedCert_DowngradesOnce(t *testing.T) {
	outboxRepo := &mockMerchantVerificationOutboxRepo{}
	reg := &domain.PayuniRegistration{AccountID: 123, ApplicationID: 10, Status: domain.PayuniStatusPending}
	payuniRepo := &mockPayuniRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.PayuniRegistration, error) {
			return reg, nil
		},
		UpdateFn: func(ctx context.Context, reg *domain.PayuniRegistration) error { return nil },
	}
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID}, nil
		},
	}
	app := &domain.CertificationApplication{ID: 10, AccountID: 123, Status: domain.CertStatusForwarded}
	app.InitFSM()
	var certWritten bool
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			return app, nil
		},
		UpdateApplicationFn: func(ctx context.Context, a *domain.CertificationApplication) error {
			certWritten = true
			return nil
		},
	}

	uc := &PayuniUseCase{payuniRepo: payuniRepo, sellerRepo: sellerRepo, certRepo: certRepo, txManager: &mockTxManager{}, outboxRepo: outboxRepo, tsid: func() int64 { return 844 }}
	for i := 0; i < 3; i++ {
		if err := uc.HandleAgentNotify(context.Background(), "HAWK123", "9", map[string]string{"MemberStatus": "5"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if certWritten {
		t.Error("cert 不該被寫")
	}
	if got := deriveOverallStatus(app.Status.String(), derivePayuniOutcome(reg.MerStatus, reg.MemberStatus)); got != overallStatusPendingInfo {
		t.Errorf("overallStatus = %q, want %q（會員層未完成，賣家實際收不到款）", got, overallStatusPendingInfo)
	}
	if len(outboxRepo.Inserted) != 1 {
		t.Errorf("verification events = %d, want 1（#844：不得每次 webhook 都重發）", len(outboxRepo.Inserted))
	}
}

func TestSyncPending_NotifyOutboxFails_KeepsRegistrationPending(t *testing.T) {
	var insertAttempts int
	outboxRepo := &mockMerchantVerificationOutboxRepo{
		InsertFn: func(ctx context.Context, row *domain.MerchantVerificationOutbox) error {
			insertAttempts++
			return pkgerrors.NewConflict("outbox insert failed")
		},
	}
	reg := &domain.PayuniRegistration{ID: 1, AccountID: 100, ApplicationID: 10, PayuniMerID: "HAWK100", Status: domain.PayuniStatusPending}
	payuniRepo := &mockPayuniRepo{
		GetPendingFn: func(ctx context.Context, limit int, staleBefore time.Time) ([]*domain.PayuniRegistration, error) {
			return []*domain.PayuniRegistration{reg}, nil
		},
		UpdateFn: func(ctx context.Context, r *domain.PayuniRegistration) error {
			t.Error("registration must NOT be persisted when the verification notification could not be recorded")
			return nil
		},
	}
	pc := &mockPayuniClient{
		GetSellerStatusFn: func(ctx context.Context, merID string) (*SellerStatusResponse, error) {
			return &SellerStatusResponse{MerStatus: "1", MemberStatus: "1", RawResponse: json.RawMessage(`{}`)}, nil
		},
	}

	uc := &PayuniUseCase{payuniRepo: payuniRepo, payuniClient: pc, certRepo: &mockCertificationRepo{}, txManager: &mockTxManager{}, outboxRepo: outboxRepo, tsid: func() int64 { return 12710 }}
	if err := uc.SyncPendingStatuses(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reg.Status != domain.PayuniStatusPending {
		t.Errorf("registration status = %v, want Pending（通知寫不進 outbox 就不得推進，否則下一輪 kycChanged=false、事件永久丟失）", reg.Status)
	}
	if insertAttempts != 1 {
		t.Errorf("outbox insert attempts = %d, want 1", insertAttempts)
	}
}

func TestApplyPayuniOutcome_SameStatusReobserved_ActivatesWithoutNotify(t *testing.T) {
	outboxRepo := &mockMerchantVerificationOutboxRepo{}
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{ID: id, AccountID: 100, Status: domain.CertStatusForwarded}
			app.InitFSM()
			return app, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error {
			t.Error("cert 不該被寫")
			return nil
		},
	}
	uc := &PayuniUseCase{certRepo: certRepo, txManager: &mockTxManager{}, outboxRepo: outboxRepo, tsid: func() int64 { return 1 }}
	info := &domain.PayuniRegistration{AccountID: 100, ApplicationID: 10, PayuniMerID: "HAWK100", Status: domain.PayuniStatusPending, MerStatus: "1", MemberStatus: "1"}

	obs := obsOf("1", "1")
	obs.Source = "sync"
	if !uc.ApplyObservation(context.Background(), info, obs) {
		t.Fatal("ApplyObservation = false, want true（同狀態重觀測仍可持久化）")
	}
	if info.Status != domain.PayuniStatusActive {
		t.Errorf("registration status = %v, want Active", info.Status)
	}
	if len(outboxRepo.Inserted) != 0 {
		t.Errorf("published = %v, want none（PayUni 狀態沒變，不得重發）", outboxRepo.Inserted)
	}
}

func TestPayuniPendingInfoReason(t *testing.T) {
	memberGuidance := codemeta.PayuniSellerFacingErrorLabel("MEMBER_5")
	if memberGuidance == "MEMBER_5" {
		t.Fatalf("CodeMeta has no MEMBER_5 label registered — got the key back verbatim")
	}
	if got := payuniPendingInfoReason("請補上負責人身分證背面"); got != "請補上負責人身分證背面" {
		t.Errorf("non-empty reason should pass through, got %q", got)
	}
	if got := payuniPendingInfoReason(""); got != memberGuidance {
		t.Errorf("empty reason should fall back to member guidance, got %q, want %q", got, memberGuidance)
	}
	if got := payuniPendingInfoReason("   "); got != memberGuidance {
		t.Errorf("whitespace-only reason should fall back to member guidance, got %q, want %q", got, memberGuidance)
	}
}

func TestSyncPending_MemberPendingEmptyReason_PublishesGuidance(t *testing.T) {
	outboxRepo := &mockMerchantVerificationOutboxRepo{}
	payuniRepo := &mockPayuniRepo{
		GetPendingFn: func(ctx context.Context, limit int, staleBefore time.Time) ([]*domain.PayuniRegistration, error) {
			return []*domain.PayuniRegistration{
				{ID: 1, AccountID: 100, ApplicationID: 10, PayuniMerID: "HAWK100", Status: domain.PayuniStatusPending},
			}, nil
		},
		UpdateFn: func(ctx context.Context, reg *domain.PayuniRegistration) error { return nil },
	}
	pc := &mockPayuniClient{
		GetSellerStatusFn: func(ctx context.Context, merID string) (*SellerStatusResponse, error) {
			return &SellerStatusResponse{MerStatus: "9", MemberStatus: "5", Reason: "", RawResponse: json.RawMessage(`{}`)}, nil
		},
	}
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID}, nil
		},
	}
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{ID: id, AccountID: 100, Status: domain.CertStatusForwarded}
			app.InitFSM()
			return app, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error { return nil },
	}

	uc := &PayuniUseCase{payuniRepo: payuniRepo, payuniClient: pc, sellerRepo: sellerRepo, certRepo: certRepo, txManager: &mockTxManager{}, outboxRepo: outboxRepo, tsid: func() int64 { return 13610 }}
	if err := uc.SyncPendingStatuses(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(outboxRepo.Inserted) != 1 || outboxRepo.Inserted[0].Reason != codemeta.PayuniSellerFacingErrorLabel("MEMBER_5") {
		t.Errorf("published reasons = %v, want exactly [member guidance]", outboxRepo.Inserted)
	}
}

func TestSyncPending_MerchantPendingWithReason_PassesThrough(t *testing.T) {
	outboxRepo := &mockMerchantVerificationOutboxRepo{}
	payuniRepo := &mockPayuniRepo{
		GetPendingFn: func(ctx context.Context, limit int, staleBefore time.Time) ([]*domain.PayuniRegistration, error) {
			return []*domain.PayuniRegistration{
				{ID: 1, AccountID: 100, ApplicationID: 10, PayuniMerID: "HAWK100", Status: domain.PayuniStatusPending},
			}, nil
		},
		UpdateFn: func(ctx context.Context, reg *domain.PayuniRegistration) error { return nil },
	}
	pc := &mockPayuniClient{
		GetSellerStatusFn: func(ctx context.Context, merID string) (*SellerStatusResponse, error) {
			return &SellerStatusResponse{MerStatus: "91", MemberStatus: "1", Reason: "請補上負責人身分證背面", RawResponse: json.RawMessage(`{}`)}, nil
		},
	}
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID}, nil
		},
	}
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{ID: id, AccountID: 100, Status: domain.CertStatusForwarded}
			app.InitFSM()
			return app, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error { return nil },
	}

	uc := &PayuniUseCase{payuniRepo: payuniRepo, payuniClient: pc, sellerRepo: sellerRepo, certRepo: certRepo, txManager: &mockTxManager{}, outboxRepo: outboxRepo, tsid: func() int64 { return 14010 }}
	if err := uc.SyncPendingStatuses(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(outboxRepo.Inserted) != 1 || outboxRepo.Inserted[0].Reason != "請補上負責人身分證背面" {
		t.Errorf("published reasons = %v, want exactly [請補上負責人身分證背面]", outboxRepo.Inserted)
	}
}

func TestHandleAgentNotify_MemberPendingEmptyReason_PublishesGuidance(t *testing.T) {
	outboxRepo := &mockMerchantVerificationOutboxRepo{}
	payuniRepo := &mockPayuniRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.PayuniRegistration, error) {
			return &domain.PayuniRegistration{AccountID: 123, ApplicationID: 10, Status: domain.PayuniStatusPending}, nil
		},
		UpdateFn: func(ctx context.Context, reg *domain.PayuniRegistration) error { return nil },
	}
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID}, nil
		},
	}
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{ID: id, AccountID: 123, Status: domain.CertStatusForwarded}
			app.InitFSM()
			return app, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error { return nil },
	}

	uc := &PayuniUseCase{payuniRepo: payuniRepo, sellerRepo: sellerRepo, certRepo: certRepo, txManager: &mockTxManager{}, outboxRepo: outboxRepo, tsid: func() int64 { return 14340 }}
	if err := uc.HandleAgentNotify(context.Background(), "HAWK123", "9", map[string]string{"MemberStatus": "5"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(outboxRepo.Inserted) != 1 || outboxRepo.Inserted[0].Reason != codemeta.PayuniSellerFacingErrorLabel("MEMBER_5") {
		t.Errorf("published reasons = %v, want exactly [member guidance]", outboxRepo.Inserted)
	}
}

func TestSyncPending_UnchangedStatus_SkipsHistory(t *testing.T) {
	var historyEntries []*domain.PayuniStatusHistory

	payuniRepo := &mockPayuniRepo{
		GetPendingFn: func(ctx context.Context, limit int, staleBefore time.Time) ([]*domain.PayuniRegistration, error) {
			return []*domain.PayuniRegistration{
				{
					ID: 1, AccountID: 100, PayuniMerID: "HAWK100", Status: domain.PayuniStatusPending,
					MerStatus: "91", MemberStatus: "1", Reason: "請補上負責人身分證背面",
				},
			}, nil
		},
		UpdateFn: func(ctx context.Context, reg *domain.PayuniRegistration) error { return nil },
		InsertStatusHistoryFn: func(ctx context.Context, h *domain.PayuniStatusHistory) error {
			historyEntries = append(historyEntries, h)
			return nil
		},
	}

	pc := &mockPayuniClient{
		GetSellerStatusFn: func(ctx context.Context, merID string) (*SellerStatusResponse, error) {
			return &SellerStatusResponse{
				MerStatus:    "91",
				MemberStatus: "1",
				Reason:       "請補上負責人身分證背面",
				RawResponse:  json.RawMessage(`{"MerStatus":"91"}`),
			}, nil
		},
	}

	uc := &PayuniUseCase{payuniRepo: payuniRepo, payuniClient: pc, tsid: func() int64 { return 7777 }}
	if err := uc.SyncPendingStatuses(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(historyEntries) != 0 {
		t.Fatalf("expected 0 history entries for an unchanged poll result, got %d", len(historyEntries))
	}
}

func TestSyncPending_ChangedStatus_RecordsHistory(t *testing.T) {
	var historyEntries []*domain.PayuniStatusHistory

	payuniRepo := &mockPayuniRepo{
		GetPendingFn: func(ctx context.Context, limit int, staleBefore time.Time) ([]*domain.PayuniRegistration, error) {
			return []*domain.PayuniRegistration{
				{ID: 1, AccountID: 100, PayuniMerID: "HAWK100", Status: domain.PayuniStatusPending, MerStatus: "9"},
			}, nil
		},
		UpdateFn: func(ctx context.Context, reg *domain.PayuniRegistration) error { return nil },
		InsertStatusHistoryFn: func(ctx context.Context, h *domain.PayuniStatusHistory) error {
			historyEntries = append(historyEntries, h)
			return nil
		},
	}

	pc := &mockPayuniClient{
		GetSellerStatusFn: func(ctx context.Context, merID string) (*SellerStatusResponse, error) {
			return &SellerStatusResponse{MerStatus: "91", MemberStatus: "1", RawResponse: json.RawMessage(`{}`)}, nil
		},
	}

	uc := &PayuniUseCase{payuniRepo: payuniRepo, payuniClient: pc, tsid: func() int64 { return 7778 }}
	if err := uc.SyncPendingStatuses(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(historyEntries) != 1 {
		t.Fatalf("expected 1 history entry for a changed poll result, got %d", len(historyEntries))
	}
}

func TestSyncPending_StalePending_LogsWarning(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	restore := zap.ReplaceGlobals(zap.New(core))
	defer restore()

	staleSince := time.Now().Add(-31 * 24 * time.Hour)
	payuniRepo := &mockPayuniRepo{
		GetPendingFn: func(ctx context.Context, limit int, staleBefore time.Time) ([]*domain.PayuniRegistration, error) {
			return []*domain.PayuniRegistration{
				{
					ID: 1, AccountID: 100, PayuniMerID: "HAWK100", Status: domain.PayuniStatusPending,
					MerStatus: "91", MemberStatus: "1", RegisteredAt: &staleSince,
				},
			}, nil
		},
		UpdateFn:              func(ctx context.Context, reg *domain.PayuniRegistration) error { return nil },
		InsertStatusHistoryFn: func(ctx context.Context, h *domain.PayuniStatusHistory) error { return nil },
	}
	pc := &mockPayuniClient{
		GetSellerStatusFn: func(ctx context.Context, merID string) (*SellerStatusResponse, error) {
			return &SellerStatusResponse{MerStatus: "91", MemberStatus: "1", RawResponse: json.RawMessage(`{}`)}, nil
		},
	}

	uc := &PayuniUseCase{payuniRepo: payuniRepo, payuniClient: pc, tsid: func() int64 { return 7779 }}
	if err := uc.SyncPendingStatuses(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, entry := range logs.All() {
		if entry.Message == "PAYUNi registration stuck in pending past SLA threshold" {
			found = true
		}
	}
	if !found {
		t.Error("expected a stale-pending warning log, got none")
	}
}

func TestSyncPendingStatuses_SkipsRecentlySyncedRegistration(t *testing.T) {
	apiCalled := false
	recentSync := time.Now().Add(-payuniPollMinInterval / 10)

	payuniRepo := &mockPayuniRepo{
		GetPendingFn: func(ctx context.Context, limit int, staleBefore time.Time) ([]*domain.PayuniRegistration, error) {
			return []*domain.PayuniRegistration{
				{
					ID: 1, AccountID: 100, PayuniMerID: "HAWK100", Status: domain.PayuniStatusPending,
					LastSyncedAt: &recentSync,
				},
			}, nil
		},
		ClaimForSyncFn: func(ctx context.Context, id int64, claimedAt, staleBefore time.Time) (bool, error) {
			return !recentSync.After(staleBefore), nil
		},
		UpdateFn: func(ctx context.Context, reg *domain.PayuniRegistration) error { return nil },
	}
	pc := &mockPayuniClient{
		GetSellerStatusFn: func(ctx context.Context, merID string) (*SellerStatusResponse, error) {
			apiCalled = true
			return &SellerStatusResponse{MerStatus: "9", RawResponse: json.RawMessage(`{}`)}, nil
		},
	}

	uc := &PayuniUseCase{payuniRepo: payuniRepo, payuniClient: pc}
	if err := uc.SyncPendingStatuses(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if apiCalled {
		t.Error("PAYUNi API should not be called for a registration synced within the throttle window")
	}
}

func TestSyncPendingStatuses_PollsStaleRegistration(t *testing.T) {
	apiCalled := false
	staleSync := time.Now().Add(-2 * payuniPollMinInterval)

	payuniRepo := &mockPayuniRepo{
		GetPendingFn: func(ctx context.Context, limit int, staleBefore time.Time) ([]*domain.PayuniRegistration, error) {
			return []*domain.PayuniRegistration{
				{
					ID: 1, AccountID: 100, PayuniMerID: "HAWK100", Status: domain.PayuniStatusPending,
					LastSyncedAt: &staleSync,
				},
			}, nil
		},
		ClaimForSyncFn: func(ctx context.Context, id int64, claimedAt, staleBefore time.Time) (bool, error) {
			return !staleSync.After(staleBefore), nil
		},
		UpdateFn: func(ctx context.Context, reg *domain.PayuniRegistration) error { return nil },
	}
	pc := &mockPayuniClient{
		GetSellerStatusFn: func(ctx context.Context, merID string) (*SellerStatusResponse, error) {
			apiCalled = true
			return &SellerStatusResponse{MerStatus: "9", RawResponse: json.RawMessage(`{}`)}, nil
		},
	}

	uc := &PayuniUseCase{payuniRepo: payuniRepo, payuniClient: pc}
	if err := uc.SyncPendingStatuses(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !apiCalled {
		t.Error("PAYUNi API should still be called for a registration synced past the throttle window")
	}
}

func TestSyncPendingStatuses_ClaimWindowMatchesThrottle(t *testing.T) {
	var gotQueryStaleBefore, gotClaimStaleBefore time.Time
	claimCalls := 0
	before := time.Now().UTC()

	payuniRepo := &mockPayuniRepo{
		GetPendingFn: func(ctx context.Context, limit int, staleBefore time.Time) ([]*domain.PayuniRegistration, error) {
			gotQueryStaleBefore = staleBefore
			return []*domain.PayuniRegistration{
				{ID: 1, AccountID: 100, PayuniMerID: "HAWK100", Status: domain.PayuniStatusPending},
			}, nil
		},
		ClaimForSyncFn: func(ctx context.Context, id int64, claimedAt, staleBefore time.Time) (bool, error) {
			claimCalls++
			gotClaimStaleBefore = staleBefore
			return false, nil
		},
	}
	uc := &PayuniUseCase{payuniRepo: payuniRepo, payuniClient: &mockPayuniClient{}}
	if err := uc.SyncPendingStatuses(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	after := time.Now().UTC()

	if claimCalls != 1 {
		t.Fatalf("expected exactly one claim attempt, got %d", claimCalls)
	}
	if !gotQueryStaleBefore.Equal(gotClaimStaleBefore) {
		t.Errorf("staleBefore differs between the work-queue query (%v) and the CAS (%v); "+
			"they must be the same instant or the two filters disagree",
			gotQueryStaleBefore, gotClaimStaleBefore)
	}
	if lo, hi := before.Add(-payuniPollMinInterval), after.Add(-payuniPollMinInterval); gotQueryStaleBefore.Before(lo) || gotQueryStaleBefore.After(hi) {
		t.Errorf("staleBefore = %v, want now - %v (i.e. within [%v, %v])",
			gotQueryStaleBefore, payuniPollMinInterval, lo, hi)
	}
}

func TestSyncPendingStatuses_ClaimInstantIsPerRegistration(t *testing.T) {
	var instants []time.Time

	payuniRepo := &mockPayuniRepo{
		GetPendingFn: func(ctx context.Context, limit int, staleBefore time.Time) ([]*domain.PayuniRegistration, error) {
			return []*domain.PayuniRegistration{
				{ID: 1, AccountID: 100, PayuniMerID: "HAWK100", Status: domain.PayuniStatusPending},
				{ID: 2, AccountID: 200, PayuniMerID: "HAWK200", Status: domain.PayuniStatusPending},
			}, nil
		},
		ClaimForSyncFn: func(ctx context.Context, id int64, claimedAt, staleBefore time.Time) (bool, error) {
			instants = append(instants, claimedAt)
			return true, nil
		},
		UpdateFn: func(ctx context.Context, reg *domain.PayuniRegistration) error { return nil },
	}
	pc := &mockPayuniClient{
		GetSellerStatusFn: func(ctx context.Context, merID string) (*SellerStatusResponse, error) {
			time.Sleep(20 * time.Millisecond)
			return &SellerStatusResponse{MerStatus: "9", RawResponse: json.RawMessage(`{}`)}, nil
		},
	}

	uc := &PayuniUseCase{payuniRepo: payuniRepo, payuniClient: pc}
	if err := uc.SyncPendingStatuses(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(instants) != 2 {
		t.Fatalf("expected 2 claim attempts, got %d", len(instants))
	}
	if !instants[1].After(instants[0]) {
		t.Errorf("the second claim instant (%v) must be strictly later than the first (%v); "+
			"equal instants mean the baseline is taken once per tick, which degrades the "+
			"invariant into whole-batch-within-one-window", instants[1], instants[0])
	}
}

func TestSyncPendingStatuses_PostPollWriteDoesNotTouchLease(t *testing.T) {
	leaseSentinel := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)

	type persisted struct {
		claimedAt     time.Time
		lastSyncedAt  time.Time
		lastClaimedAt *time.Time
	}
	var claims []time.Time
	var writes []persisted

	payuniRepo := &mockPayuniRepo{
		GetPendingFn: func(ctx context.Context, limit int, staleBefore time.Time) ([]*domain.PayuniRegistration, error) {
			return []*domain.PayuniRegistration{
				{ID: 1, AccountID: 100, PayuniMerID: "HAWK100", Status: domain.PayuniStatusPending, LastClaimedAt: &leaseSentinel},
				{ID: 2, AccountID: 200, PayuniMerID: "HAWK200", Status: domain.PayuniStatusPending, LastClaimedAt: &leaseSentinel},
			}, nil
		},
		ClaimForSyncFn: func(ctx context.Context, id int64, claimedAt, staleBefore time.Time) (bool, error) {
			claims = append(claims, claimedAt)
			return true, nil
		},
		UpdateFn: func(ctx context.Context, reg *domain.PayuniRegistration) error {
			if reg.LastSyncedAt == nil {
				t.Fatalf("registration %d persisted a nil last_synced_at", reg.ID)
			}
			writes = append(writes, persisted{
				claimedAt:     claims[len(claims)-1],
				lastSyncedAt:  *reg.LastSyncedAt,
				lastClaimedAt: reg.LastClaimedAt,
			})
			return nil
		},
	}
	pc := &mockPayuniClient{
		GetSellerStatusFn: func(ctx context.Context, merID string) (*SellerStatusResponse, error) {
			time.Sleep(20 * time.Millisecond)
			return &SellerStatusResponse{MerStatus: "9", RawResponse: json.RawMessage(`{}`)}, nil
		},
	}

	uc := &PayuniUseCase{payuniRepo: payuniRepo, payuniClient: pc}
	if err := uc.SyncPendingStatuses(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(writes) != 2 {
		t.Fatalf("expected 2 persisted registrations, got %d", len(writes))
	}
	for i, w := range writes {
		if !w.lastSyncedAt.After(w.claimedAt) {
			t.Errorf("registration #%d persisted last_synced_at = %v, want strictly after the claim "+
				"instant %v. last_synced_at means \"when the observation actually succeeded\"; "+
				"stamping it with the claim instant is what used to make a failed PAYUNi call "+
				"read back as a fresh sync.", i, w.lastSyncedAt, w.claimedAt)
		}
		if w.lastClaimedAt == nil || !w.lastClaimedAt.Equal(leaseSentinel) {
			t.Errorf("registration #%d persisted last_claimed_at = %v, want it untouched at %v. "+
				"The post-poll write must never move the lease; only ClaimForSync's CAS may.",
				i, w.lastClaimedAt, leaseSentinel)
		}
	}
	if !writes[1].lastSyncedAt.After(writes[0].lastSyncedAt) {
		t.Errorf("the second registration's persisted last_synced_at (%v) must be strictly later "+
			"than the first (%v); equal values mean a single tick-wide baseline reached the DB",
			writes[1].lastSyncedAt, writes[0].lastSyncedAt)
	}
}

func TestHandleAgentNotify_HistoryCarriesItsOwnPayload(t *testing.T) {
	staleRaw := json.RawMessage(`{"Status":"SUCCESS","Message":"合作商店資料建立成功"}`)
	var historyEntries []*domain.PayuniStatusHistory

	payuniRepo := &mockPayuniRepo{
		GetByMerIDFn: func(ctx context.Context, merID string) (*domain.PayuniRegistration, error) {
			return &domain.PayuniRegistration{
				AccountID:     123,
				ApplicationID: 10,
				PayuniMerID:   "HAWK123",
				Status:        domain.PayuniStatusPending,
				RawResponse:   staleRaw,
			}, nil
		},
		UpdateFn: func(ctx context.Context, reg *domain.PayuniRegistration) error { return nil },
		InsertStatusHistoryFn: func(ctx context.Context, h *domain.PayuniStatusHistory) error {
			historyEntries = append(historyEntries, h)
			return nil
		},
	}
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID}, nil
		},
	}
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			app := &domain.CertificationApplication{ID: id, Status: domain.CertStatusSubmitted}
			app.InitFSM()
			return app, nil
		},
		UpdateApplicationFn: func(ctx context.Context, app *domain.CertificationApplication) error { return nil },
	}

	uc := &PayuniUseCase{
		payuniRepo: payuniRepo,
		sellerRepo: sellerRepo,
		certRepo:   certRepo,
		tsid:       func() int64 { return 8888 },
		txManager:  &mockTxManager{},
		outboxRepo: &mockMerchantVerificationOutboxRepo{},
	}
	payload := map[string]string{
		"MerStatus":       "1",
		"MemberStatus":    "1",
		"CreditStatus":    "1",
		"ApplePayStatus":  "0",
		"GooglePayStatus": "0",
		"ATMStatus":       "0",
		"Reason":          "",
	}
	if err := uc.HandleAgentNotify(context.Background(), "HAWK123", "1", payload); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(historyEntries) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(historyEntries))
	}
	h := historyEntries[0]
	if h.Source != "webhook" {
		t.Fatalf("history.Source = %q, want webhook", h.Source)
	}

	var got map[string]string
	if err := json.Unmarshal(h.RawResponse, &got); err != nil {
		t.Fatalf("history.RawResponse is not a JSON object: %v (raw=%s)", err, h.RawResponse)
	}
	if got["MemberStatus"] != "1" {
		t.Errorf("history.RawResponse MemberStatus = %q, want 1 (raw=%s)", got["MemberStatus"], h.RawResponse)
	}
	if _, leaked := got["Message"]; leaked {
		t.Errorf("history.RawResponse carries residue from an earlier call: %s", h.RawResponse)
	}
}

func TestRefreshStatus_HistoryCarriesApiResponse(t *testing.T) {
	apiRaw := json.RawMessage(`{"MerStatus":"91","MemberStatus":"5","Reason":"補件"}`)
	var historyEntries []*domain.PayuniStatusHistory

	payuniRepo := &mockPayuniRepo{
		GetByStoreIDFn: func(ctx context.Context, storeID int64) (*domain.PayuniRegistration, error) {
			return &domain.PayuniRegistration{
				ID: 1, AccountID: 100, StoreID: storeID, PayuniMerID: "HAWK100",
				Status: domain.PayuniStatusPending,
			}, nil
		},
		UpdateFn: func(ctx context.Context, reg *domain.PayuniRegistration) error { return nil },
		InsertStatusHistoryFn: func(ctx context.Context, h *domain.PayuniStatusHistory) error {
			historyEntries = append(historyEntries, h)
			return nil
		},
	}
	pc := &mockPayuniClient{
		GetSellerStatusFn: func(ctx context.Context, merID string) (*SellerStatusResponse, error) {
			return &SellerStatusResponse{
				MerStatus: "91", MemberStatus: "5", Reason: "補件", RawResponse: apiRaw,
			}, nil
		},
	}
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID}, nil
		},
	}

	uc := &PayuniUseCase{
		payuniRepo: payuniRepo, payuniClient: pc, sellerRepo: sellerRepo,
		tsid: func() int64 { return 9999 },
	}
	if err := uc.RefreshStatus(context.Background(), 100); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(historyEntries) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(historyEntries))
	}
	if string(historyEntries[0].RawResponse) != string(apiRaw) {
		t.Errorf("history.RawResponse = %s, want %s", historyEntries[0].RawResponse, apiRaw)
	}
	if historyEntries[0].Source != "refresh" {
		t.Errorf("history.Source = %q, want refresh", historyEntries[0].Source)
	}
}
