package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

type fakeHistoryAudit struct {
	domain.AuditRepository
	rows           []domain.StatusHistoryEntry
	err            error
	gotSubjectType string
	gotSubjectID   int64
}

func (f *fakeHistoryAudit) ListStatusHistory(_ context.Context, subjectType string, subjectID int64) ([]domain.StatusHistoryEntry, error) {
	f.gotSubjectType, f.gotSubjectID = subjectType, subjectID
	if f.err != nil {
		return nil, f.err
	}
	if subjectType != taxSubjectType {
		return nil, errors.New("fakeHistoryAudit: 只接受 TAX_PROFILE，收到 " + subjectType)
	}
	return f.rows, nil
}

func historyRow(reason, detail string, at time.Time) domain.StatusHistoryEntry {
	return domain.StatusHistoryEntry{
		SubjectType: taxSubjectType, ToStatus: "SELF_ISSUE_PENDING",
		ReasonCode: &reason, ReasonDetail: &detail, OccurredAt: at,
	}
}

func TestPendingTaxExtensionRequest(t *testing.T) {
	base := time.Date(2026, 9, 3, 0, 41, 47, 0, time.UTC)
	other := "SELLER_TAX_SELF_ISSUE_ATTESTED"
	rows := []domain.StatusHistoryEntry{
		historyRow(taxReasonExtensionRequested, "會計師還在準備文件", base),
		historyRow(other, "", base.Add(time.Hour)),
		historyRow(taxReasonExtensionRequested, "字軌配號作業中", base.Add(2*time.Hour)),
	}

	t.Run("有待裁決的申請：回最新那一筆的時間與理由", func(t *testing.T) {
		f := &fakeHistoryAudit{rows: rows}
		q := &Query{audit: f}
		got := q.PendingTaxExtensionRequest(context.Background(),
			domain.SellerTaxProfile{ID: 7501076744221229056, ExtensionCount: 0})
		if got == nil {
			t.Fatal("有一筆待裁決的申請卻回 nil —— A03 又會看不見它")
		}
		if got.Reason != "字軌配號作業中" {
			t.Errorf("理由 = %q，want 最新那一筆；取到舊的等於給 ADM 過期的判斷依據", got.Reason)
		}
		if !got.RequestedAt.Equal(base.Add(2 * time.Hour)) {
			t.Errorf("requestedAt = %v，want %v", got.RequestedAt, base.Add(2*time.Hour))
		}
		if f.gotSubjectType != taxSubjectType || f.gotSubjectID != 7501076744221229056 {
			t.Errorf("查的是 (%s,%d)，want (TAX_PROFILE,7501076744221229056)", f.gotSubjectType, f.gotSubjectID)
		}
	})

	t.Run("已核准過（extension_count=1）：不得回", func(t *testing.T) {
		q := &Query{audit: &fakeHistoryAudit{rows: rows}}
		if got := q.PendingTaxExtensionRequest(context.Background(),
			domain.SellerTaxProfile{ID: 1, ExtensionCount: 1}); got != nil {
			t.Errorf("已經准過還回 %+v —— A03 會一直顯示「有人在等」而其實沒有", got)
		}
	})

	t.Run("從來沒申請過：回 nil", func(t *testing.T) {
		q := &Query{audit: &fakeHistoryAudit{rows: []domain.StatusHistoryEntry{
			historyRow(other, "", base)}}}
		if got := q.PendingTaxExtensionRequest(context.Background(),
			domain.SellerTaxProfile{ID: 1}); got != nil {
			t.Errorf("沒申請過卻回 %+v", got)
		}
	})

	t.Run("稽核表讀失敗：降級成 nil，不把整頁打爆", func(t *testing.T) {
		q := &Query{audit: &fakeHistoryAudit{err: errors.New("db down")}}
		if got := q.PendingTaxExtensionRequest(context.Background(),
			domain.SellerTaxProfile{ID: 1}); got != nil {
			t.Errorf("讀失敗卻回 %+v", got)
		}
	})

	t.Run("profile 還沒落庫（ID=0）：不得發出查詢", func(t *testing.T) {
		f := &fakeHistoryAudit{rows: rows}
		q := &Query{audit: f}
		q.PendingTaxExtensionRequest(context.Background(), domain.SellerTaxProfile{ID: 0})
		if f.gotSubjectID != 0 || f.gotSubjectType != "" {
			t.Error("ID=0 時仍然查了歷程 —— subject_id=0 會掃到別人的列")
		}
	})
}
