package orderprovisioning

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository/gen"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

// AcceptanceStore 是本 worker 對 acceptance 的**全部**需求，窄到只有三個方法。
//
// 它由 *repository.Repo 實作（那三支查詢在 #3418 就隨 schema 一起生成好了），綁定寫在
// procurement/module.go——介面是本層宣告的，在 repository 那邊去綁它依賴方向就反了
// （同 publicationoutbox.InviteSentMarker 的理由）。
type AcceptanceStore interface {
	FindBuyerQuoteAcceptanceByIDAndBuyer(ctx context.Context, id int64, buyerID identity.BuyerID) (repository.BuyerQuoteAcceptance, error)
	FindSellerResponseVersion(ctx context.Context, sellerResponseID int64, version int32) (repository.SellerResponseVersion, error)
	FindConfirmedRfqSnapshotByIDAndBuyer(ctx context.Context, id int64, buyerID identity.BuyerID) (repository.ConfirmedRfqSnapshot, error)
	AttachOrderToAcceptance(ctx context.Context, id int64, expectedVersion int32, orderNo string, paymentDeadline time.Time) (repository.BuyerQuoteAcceptance, error)
	MarkAcceptanceOrderFailed(ctx context.Context, id int64, expectedVersion int32, reason string) (repository.BuyerQuoteAcceptance, error)
	MarkAcceptanceOrderFailedByOutboxID(ctx context.Context, outboxID int64, reason string) error
}

// Repository 實作 pkg/outbox.Repository[*Row]（五個方法）與 pkg/outbox.MonitorRepo
// （三個方法），包 procurement_order_provisioning_outbox 的八支查詢。
//
// q(ctx) 走 database.GetDBTX(ctx, pool)：ctx 上有交易就用交易、沒有就用連線池。
// MarkFailed 靠這個性質做到跨兩張表的原子寫入。形狀與 publicationoutbox.Repository
// 逐項對齊——⛔ 不要照 internal/procurement/outbox，那份是遷移前手抄的（ADR D3）。
type Repository struct {
	pool        *pgxpool.Pool
	txm         database.TxManager
	acceptances AcceptanceStore
}

func NewRepository(pool *pgxpool.Pool, txm database.TxManager, acceptances AcceptanceStore) *Repository {
	return &Repository{pool: pool, txm: txm, acceptances: acceptances}
}

func (r *Repository) q(ctx context.Context) *gen.Queries {
	return gen.New(database.GetDBTX(ctx, r.pool))
}

// FetchPendingDue 回傳至多 limit 列 status='pending' 且 next_attempt_at <= now 的候選列。
// 它**只是候選查詢**，不提供任何互斥（刻意沒有 FOR UPDATE SKIP LOCKED：這條路徑在
// autocommit 下跑，SELECT 一回傳鎖就釋放）；跨副本互斥來自 Claim。
func (r *Repository) FetchPendingDue(ctx context.Context, now time.Time, limit int) ([]*Row, error) {
	rows, err := r.q(ctx).FetchPendingProcurementOrderProvisioningOutbox(ctx, gen.FetchPendingProcurementOrderProvisioningOutboxParams{
		NextAttemptAt: repository.TimestamptzParam(now),
		Limit:         int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]*Row, 0, len(rows))
	for _, row := range rows {
		out = append(out, toRow(row))
	}
	return out, nil
}

// Claim 是跨副本搶佔閘門（《多副本排程搶佔規範》）：把 next_attempt_at 推到 leaseUntil，
// 述詞鏡射候選查詢，回傳受影響列數——1 代表本副本取得這一列，0 代表別的副本先搶到。
//
// 它是 LEASE 不是 done flag：持有者在 claim 與建單之間消失，lease 到期後這一列自己回到
// 候選集合，不需要任何對帳 job。
func (r *Repository) Claim(ctx context.Context, id int64, leaseUntil, claimableBefore time.Time) (int64, error) {
	return r.q(ctx).ClaimProcurementOrderProvisioningOutbox(ctx, gen.ClaimProcurementOrderProvisioningOutboxParams{
		LeaseUntil:      repository.TimestamptzParam(leaseUntil),
		ID:              id,
		ClaimableBefore: repository.TimestamptzParam(claimableBefore),
	})
}

// UpdateAttempt 記錄一次非終態重試：累加 attempt_count、依呼叫端算好的 backoff 排
// next_attempt_at、存下 lastError（空字串寫入 SQL NULL）。
func (r *Repository) UpdateAttempt(ctx context.Context, id int64, attemptCount int, nextAttemptAt time.Time, lastError string) error {
	return r.q(ctx).UpdateProcurementOrderProvisioningOutboxAttempt(ctx, gen.UpdateProcurementOrderProvisioningOutboxAttemptParams{
		ID:            id,
		AttemptCount:  int32(attemptCount),
		NextAttemptAt: repository.TimestamptzParam(nextAttemptAt),
		LastError:     nullableText(lastError),
	})
}

// MarkDone 把 outbox 列轉 status='done' 並蓋 completed_at。
//
// ⛔ 這裡刻意**不**碰 acceptance：acceptance 的終態是 ProcessFunc 在建單成功之後自己
// CAS 的（見 worker.go 的順序說明）。MarkDone 失敗會讓這一列重跑，那時 acceptance 已是
// ORDER_CREATED，ProcessFunc 第 2 步直接回 MarkDone——那是重試路徑的正常狀態，不是錯誤。
func (r *Repository) MarkDone(ctx context.Context, id int64, now time.Time) error {
	completedAt := now
	return r.q(ctx).MarkProcurementOrderProvisioningOutboxDone(ctx, gen.MarkProcurementOrderProvisioningOutboxDoneParams{
		ID:          id,
		CompletedAt: &completedAt,
	})
}

// MarkFailed 把 outbox 列轉終態 failed，並在**同一個交易內**把對應 acceptance 轉
// ORDER_FAILED（reason ＝ RETRY_EXHAUSTED）。
//
// ⛔ 不可改用 pkg/outbox.Hooks 達成，兩個 hook 都不行：
//   - OnAttemptFailed 對**每一次**嘗試失敗都觸發，第一次暫時性失敗就會把 acceptance
//     打成終態，而它其實還會被重試——對買家的可觀察狀態會說謊。
//   - OnTerminalFailed 雖然只在終態觸發，但它的簽名帶 markFailedErr，代表它是在
//     MarkFailed 回來**之後**才跑，不可能與那次寫入原子；中間留下「outbox 已 failed、
//     acceptance 仍 CREATING_ORDER」的窗口，而 CREATING_ORDER 會讓前端一直顯示
//     「訂單建立中」（doc 64 §6.2 的中間態文案），沒有任何東西會把它推出去。
//
// 兩句都走 ctx 上的同一個交易；第二句失敗會讓第一句一起 ROLLBACK，worker 下一輪重試整段。
func (r *Repository) MarkFailed(ctx context.Context, id int64, lastErr string, now time.Time) error {
	return r.txm.WithTx(ctx, func(txCtx context.Context) error {
		completedAt := now
		if err := r.q(txCtx).MarkProcurementOrderProvisioningOutboxFailed(txCtx, gen.MarkProcurementOrderProvisioningOutboxFailedParams{
			ID:          id,
			LastError:   nullableText(lastErr),
			CompletedAt: &completedAt,
		}); err != nil {
			return err
		}
		return r.acceptances.MarkAcceptanceOrderFailedByOutboxID(txCtx, id, FailureReasonRetryExhausted)
	})
}

// CountPending 是 pkg/outbox.MonitorRepo：backlog gauge 的取樣來源。
func (r *Repository) CountPending(ctx context.Context) (int, error) {
	count, err := r.q(ctx).CountPendingProcurementOrderProvisioningOutbox(ctx)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// OldestPendingAge 回傳最舊一列 pending 相對 now 的年齡；沒有 pending 列時 ok=false。
func (r *Repository) OldestPendingAge(ctx context.Context, now time.Time) (time.Duration, bool, error) {
	createdAt, err := r.q(ctx).OldestPendingProcurementOrderProvisioningOutboxCreatedAt(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, err
	}
	if !createdAt.Valid {
		return 0, false, nil
	}
	return now.Sub(createdAt.Time), true, nil
}

// DeleteDoneBefore 刪除至多 limit 列 completed_at < cutoff 的 done 列，回傳刪除筆數。
// 由 pkg/outbox.CleanupDone 呼叫，界住表的成長。
func (r *Repository) DeleteDoneBefore(ctx context.Context, cutoff time.Time, limit int) (int, error) {
	completedAt := cutoff
	deleted, err := r.q(ctx).DeleteDoneProcurementOrderProvisioningOutboxBefore(ctx, gen.DeleteDoneProcurementOrderProvisioningOutboxBeforeParams{
		CompletedAt: &completedAt,
		Limit:       int32(limit),
	})
	if err != nil {
		return 0, err
	}
	return int(deleted), nil
}

func toRow(row gen.ProcurementOrderProvisioningOutbox) *Row {
	out := &Row{
		ID:            row.ID,
		AcceptanceID:  row.AcceptanceID,
		EventType:     row.EventType,
		Payload:       row.Payload,
		Status:        row.Status,
		AttemptCount:  int(row.AttemptCount),
		NextAttemptAt: row.NextAttemptAt.Time,
		CreatedAt:     row.CreatedAt.Time,
		CompletedAt:   row.CompletedAt,
	}
	if row.LastError != nil {
		out.LastError = *row.LastError
	}
	return out
}

// nullableText 讓空字串寫入 SQL NULL——last_error 沒有錯誤時該是 NULL，不是空字串。
func nullableText(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
