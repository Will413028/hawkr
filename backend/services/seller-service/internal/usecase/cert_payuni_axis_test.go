package usecase

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
)

func overallOf(app *domain.CertificationApplication, info *domain.PayuniRegistration) string {
	return deriveOverallStatus(app.Status.String(), derivePayuniOutcome(info.MerStatus, info.MemberStatus))
}

func newReversalFixture(t *testing.T, certStatus domain.CertificationStatus) (*PayuniUseCase, *domain.CertificationApplication, *domain.PayuniRegistration) {
	t.Helper()
	app := &domain.CertificationApplication{ID: 10, AccountID: 100, Status: certStatus}
	app.InitFSM()

	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			return app, nil
		},
		UpdateApplicationFn: func(ctx context.Context, a *domain.CertificationApplication) error { return nil },
	}
	uc := &PayuniUseCase{
		certRepo:   certRepo,
		payuniRepo: &mockPayuniRepo{},
		outboxRepo: &mockMerchantVerificationOutboxRepo{},
		txManager:  &mockTxManager{},
		tsid:       func() int64 { return 3447 },
	}
	info := &domain.PayuniRegistration{
		AccountID: 100, StoreID: 7300000000000003447, ApplicationID: 10, PayuniMerID: "HAWK100",
		Status: domain.PayuniStatusPending,
	}
	return uc, app, info
}

func TestPayuniReversesRejectionToApproval(t *testing.T) {
	uc, app, info := newReversalFixture(t, domain.CertStatusForwarded)
	ctx := context.Background()

	uc.ApplyObservation(ctx, info, obsOf("2", "1"))
	if got := overallOf(app, info); got != overallStatusRejected {
		t.Fatalf("PayUni 駁回後 overallStatus = %q, want %q", got, overallStatusRejected)
	}

	uc.ApplyObservation(ctx, info, obsOf("1", "1"))

	if got := overallOf(app, info); got != overallStatusApproved {
		t.Errorf("PayUni 改判核准後 overallStatus = %q, want %q。"+
			"賣家畫面會一直停在「申請未通過」，點重新整理也無效，只能等客服人工處理", got, overallStatusApproved)
	}
	if app.Status != domain.CertStatusForwarded {
		t.Errorf("cert = %v, want Forwarded（PayUni 的審核結果不寫 cert，工作流狀態不該被動）", app.Status)
	}
	if info.Status != domain.PayuniStatusActive {
		t.Errorf("PayuniRegistration = %v, want Active", info.Status)
	}
}

func TestPayuniAndCertAxesNeverDiverge(t *testing.T) {
	tests := []struct {
		name        string
		merStatus   string
		wantOverall string
		wantReg     domain.PayuniStatus
	}{
		{"核准", "1", overallStatusApproved, domain.PayuniStatusActive},
		{"駁回", "2", overallStatusRejected, domain.PayuniStatusFailed},
		{"要求補件", "91", overallStatusPendingInfo, domain.PayuniStatusPending},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc, app, info := newReversalFixture(t, domain.CertStatusForwarded)
			uc.ApplyObservation(context.Background(), info, obsOf(tt.merStatus, "1"))
			if got := overallOf(app, info); got != tt.wantOverall {
				t.Errorf("overallStatus = %q, want %q", got, tt.wantOverall)
			}
			if info.Status != tt.wantReg {
				t.Errorf("registration = %v, want %v", info.Status, tt.wantReg)
			}
		})
	}
}

func TestPayuniOutcomeDoesNotOverwriteWithdrawnApplication(t *testing.T) {
	uc, app, info := newReversalFixture(t, domain.CertStatusDraft)

	uc.ApplyObservation(context.Background(), info, obsOf("1", "1"))

	if got := overallOf(app, info); got != domain.CertStatusDraft.String() {
		t.Errorf("overallStatus = %q, want draft（賣家正在重新編輯，PayUni 的結果不得蓋過工作流）", got)
	}
	if info.Status != domain.PayuniStatusActive {
		t.Errorf("registration = %v, want Active（payuni 軸仍應如實反映 PayUni）", info.Status)
	}
}

func TestPayuniRepeatedSameOutcomeIsIdempotent(t *testing.T) {
	app := &domain.CertificationApplication{ID: 10, AccountID: 100, Status: domain.CertStatusForwarded}
	app.InitFSM()
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			return app, nil
		},
		UpdateApplicationFn: func(ctx context.Context, a *domain.CertificationApplication) error { return nil },
	}
	outboxRepo := &mockMerchantVerificationOutboxRepo{}
	uc := &PayuniUseCase{
		certRepo: certRepo, payuniRepo: &mockPayuniRepo{}, outboxRepo: outboxRepo,
		txManager: &mockTxManager{}, tsid: func() int64 { return 3447 },
	}
	info := &domain.PayuniRegistration{AccountID: 100, StoreID: 7300000000000003447, ApplicationID: 10, PayuniMerID: "HAWK100"}

	uc.ApplyObservation(context.Background(), info, obsOf("1", "1"))
	uc.ApplyObservation(context.Background(), info, obsOf("1", "1"))
	uc.ApplyObservation(context.Background(), info, obsOf("1", "1"))

	if got := len(outboxRepo.Inserted); got != 1 {
		t.Errorf("outbox 寫入 %d 次, want 1。直接賦值沒有 FSM 的 Can() 擋重複，冪等要自己顧", got)
	}
}

func TestPayuniMemberPendingDowngradesApprovedCertOnceThenIdempotent(t *testing.T) {
	app := &domain.CertificationApplication{ID: 10, AccountID: 100, Status: domain.CertStatusForwarded}
	app.InitFSM()
	certRepo := &mockCertificationRepo{
		GetApplicationByIDFn: func(ctx context.Context, id int64) (*domain.CertificationApplication, error) {
			return app, nil
		},
		UpdateApplicationFn: func(ctx context.Context, a *domain.CertificationApplication) error { return nil },
	}
	outboxRepo := &mockMerchantVerificationOutboxRepo{}
	uc := &PayuniUseCase{
		certRepo: certRepo, payuniRepo: &mockPayuniRepo{}, outboxRepo: outboxRepo,
		txManager: &mockTxManager{}, tsid: func() int64 { return 844 },
	}
	info := &domain.PayuniRegistration{
		AccountID: 100, StoreID: 7300000000000000844, ApplicationID: 10, PayuniMerID: "HAWK100",
		Status: domain.PayuniStatusPending,
	}

	for i := 0; i < 3; i++ {
		uc.ApplyObservation(context.Background(), info, obsOf("1", "5"))
	}

	if got := overallOf(app, info); got != overallStatusPendingInfo {
		t.Errorf("overallStatus = %q, want %q（會員層未完成，賣家實際收不到款）", got, overallStatusPendingInfo)
	}
	if got := len(outboxRepo.Inserted); got != 1 {
		t.Errorf("outbox 寫入 %d 次, want 1（issue #844：不得每輪重發推播）", got)
	}
}
