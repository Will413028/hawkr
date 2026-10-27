package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/repository/postgres/gen"
)

// ManualUnlockRepository 是 invoice_manual_unlocks（§5.1.16）的唯一存取面。
//
// 本表承載 A4 人工出口的雙人解鎖（端點 54／55）。三條規則與 queries 檔逐字相同：
// 雙人守衛在 SQL 的 WHERE、逾期不由排程器收斂、每一句寫入都是 CAS 且 rows=0 一律
// 由呼叫端回明確業務碼。
type ManualUnlockRepository struct {
	baseRepo
}

// NewManualUnlockRepository 建立 repository。
func NewManualUnlockRepository(pool *pgxpool.Pool) *ManualUnlockRepository {
	return &ManualUnlockRepository{baseRepo: baseRepo{pool: pool}}
}

var _ domain.ManualUnlockRepository = (*ManualUnlockRepository)(nil)

// ExpireStale 把該 job 底下已過期而仍掛著的那一列就地轉 EXPIRED。
//
// 呼叫端必須在 Create 之前、同一交易內呼叫它——理由見 domain 介面的紅字（名額釋放）。
func (r *ManualUnlockRepository) ExpireStale(ctx context.Context, operationID int64, now time.Time) (int64, error) {
	return r.q(ctx).ExpireStaleManualUnlocks(ctx, gen.ExpireStaleManualUnlocksParams{
		OperationID: operationID,
		Now:         ts(now),
	})
}

// Create 建立一列解鎖請求。
//
// 🔴 taken=true（撞 uq_invmu_open_per_operation）之後**這個交易已經 aborted**
// （PostgreSQL 25P02）：呼叫端只能 rollback 並回業務碼，不得再下任何一句 SQL。
// 這裡沒有改寫成 ON CONFLICT DO NOTHING 的路：衝突的那一列與新列不同鍵（id 是新的
// TSID），DO NOTHING 會靜默不插入而呼叫端拿不到任何列可回。
func (r *ManualUnlockRepository) Create(ctx context.Context, in domain.NewManualUnlock) (domain.ManualUnlock, bool, error) {
	row, err := r.q(ctx).InsertManualUnlock(ctx, gen.InsertManualUnlockParams{
		ID:                    in.ID,
		OperationID:           in.OperationID,
		RequestedByAccountID:  in.RequestedByAccountID,
		RequestReason:         in.RequestReason,
		ProviderCheckEvidence: in.ProviderCheckEvidence,
		ProviderCheckedAt:     ts(in.ProviderCheckedAt),
		Now:                   ts(in.Now),
		ExpiresAt:             ts(in.ExpiresAt),
		// J28 證據：只有從 CANCEL_BLOCKED_UNKNOWN 發起的解鎖有值。
		CancelEvidenceKind:      in.CancelEvidenceKind,
		CancelEvidenceObjectKey: in.CancelEvidenceObjectKey,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ManualUnlock{}, true, nil
		}
		return domain.ManualUnlock{}, false, err
	}
	return toManualUnlock(row), false, nil
}

// GetByID 讀一列；查無回 9340（與 job 查無同碼：unlock_id 是 job 底下的子資源，
// 對 ADM 而言「這個 job 沒有這個解鎖請求」與「沒有這個 job」的下一步相同）。
func (r *ManualUnlockRepository) GetByID(ctx context.Context, id int64) (domain.ManualUnlock, error) {
	row, err := r.q(ctx).GetManualUnlockByID(ctx, id)
	if err != nil {
		return domain.ManualUnlock{}, notFound(err, bizcode.InvoiceJobNotFound)
	}
	return toManualUnlock(row), nil
}

// GetOpen 回「尚未決策且尚未逾期」的那一列。
//
// 🔴 查無走 found=false 而**不轉業務碼**：呼叫端要用它分辨「有沒有開著的請求」，
// 把 ErrNoRows 換成 9340 會讓「這個 job 目前沒有待覆核的解鎖」變成「這個 job 不存在」。
func (r *ManualUnlockRepository) GetOpen(ctx context.Context, operationID int64, now time.Time) (domain.ManualUnlock, bool, error) {
	row, err := r.q(ctx).GetOpenManualUnlockByOperation(ctx, gen.GetOpenManualUnlockByOperationParams{
		OperationID: operationID,
		Now:         ts(now),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ManualUnlock{}, false, nil
	}
	if err != nil {
		return domain.ManualUnlock{}, false, err
	}
	return toManualUnlock(row), true, nil
}

// GetLatest 回最近一列（不論狀態）。found=false ＝ 從未發起過，端點 51 的
// `manualUnlock` 回 null——那不是錯誤。
func (r *ManualUnlockRepository) GetLatest(ctx context.Context, operationID int64) (domain.ManualUnlock, bool, error) {
	row, err := r.q(ctx).GetLatestManualUnlockByOperation(ctx, operationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ManualUnlock{}, false, nil
	}
	if err != nil {
		return domain.ManualUnlock{}, false, err
	}
	return toManualUnlock(row), true, nil
}

// Decide 是端點 55 的 CAS，回 rows>0。
func (r *ManualUnlockRepository) Decide(ctx context.Context, in domain.DecideManualUnlock) (bool, error) {
	approver := in.ApprovedByAccountID
	now := in.Now
	rows, err := r.q(ctx).DecideManualUnlock(ctx, gen.DecideManualUnlockParams{
		NextStatus:          in.NextStatus,
		ApprovedByAccountID: &approver,
		DecisionReason:      nilIfBlank(in.DecisionReason),
		Now:                 &now,
		ID:                  in.ID,
		OperationID:         in.OperationID,
		ExpectedVersion:     in.ExpectedVersion,
	})
	return rows > 0, err
}

// toManualUnlock 把生成列映成 domain 型別。
func toManualUnlock(row gen.InvoiceManualUnlock) domain.ManualUnlock {
	return domain.ManualUnlock{
		ID:                      row.ID,
		OperationID:             row.OperationID,
		Status:                  row.Status,
		RequestedByAccountID:    row.RequestedByAccountID,
		RequestReason:           row.RequestReason,
		ProviderCheckEvidence:   row.ProviderCheckEvidence,
		ProviderCheckedAt:       row.ProviderCheckedAt.Time,
		RequestedAt:             row.RequestedAt.Time,
		ExpiresAt:               row.ExpiresAt.Time,
		CancelEvidenceKind:      row.CancelEvidenceKind,
		CancelEvidenceObjectKey: row.CancelEvidenceObjectKey,
		ApprovedByAccountID:     row.ApprovedByAccountID,
		DecisionReason:          row.DecisionReason,
		DecidedAt:               row.DecidedAt,
		Version:                 row.Version,
	}
}
