package publicationoutbox

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository/gen"
	"github.com/Will413028/hawkr/go-pkg/database"
)

// InviteSentMarker 是「把一張 invite 從 QUEUED CAS 成 SENT」這件事的既有 owner
// （*repository.Repo.MarkPublicationInviteSent，Task 3 就為這條路徑寫好了，含
// 「影響 0 列不是錯誤」的理由與它自己的 integration test）。這裡宣告成窄 port 而
// 不是在本 package 再對同一句 SQL 包第二層，是本 repo §15 的規則：找到既有 owner
// 就沿用，不為單一 caller 建平行實作。綁定在 procurement/module.go。
type InviteSentMarker interface {
	MarkPublicationInviteSent(ctx context.Context, inviteID int64, now time.Time) error
}

// Repository 實作 pkg/outbox.Repository[*Row]（五個方法）與 pkg/outbox.MonitorRepo
// （三個方法），包 procurement_rfq_publication_outbox 的 sqlc 查詢。
//
// q(ctx) 走 database.GetDBTX(ctx, pool)：ctx 上有交易就用交易、沒有就用連線池，
// 與 repository 層完全同一個慣例。MarkFailed 靠這個性質做到跨兩張表的原子寫入。
type Repository struct {
	pool    *pgxpool.Pool
	txm     database.TxManager
	invites InviteSentMarker
}

func NewRepository(pool *pgxpool.Pool, txm database.TxManager, invites InviteSentMarker) *Repository {
	return &Repository{pool: pool, txm: txm, invites: invites}
}

func (r *Repository) q(ctx context.Context) *gen.Queries {
	return gen.New(database.GetDBTX(ctx, r.pool))
}

// FetchPendingDue 回傳至多 limit 列 status='pending' 且 next_attempt_at <= now 的
// 候選列。它**只是候選查詢**，不提供任何互斥（刻意沒有 FOR UPDATE SKIP LOCKED：
// 這條路徑在 autocommit 下跑，SELECT 一回傳鎖就釋放）；跨副本互斥來自 Claim。
func (r *Repository) FetchPendingDue(ctx context.Context, now time.Time, limit int) ([]*Row, error) {
	rows, err := r.q(ctx).FetchPendingProcurementRfqPublicationOutbox(ctx, gen.FetchPendingProcurementRfqPublicationOutboxParams{
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

// Claim 是跨副本搶佔閘門（《多副本排程搶佔規範》）：把 next_attempt_at 推到
// leaseUntil，述詞鏡射候選查詢（status='pending' AND next_attempt_at <=
// claimableBefore），回傳受影響列數——1 代表本副本取得這一列，0 代表別的副本先搶到
// 或這一列已離開候選集合。
//
// 它是 LEASE 不是 done flag：持有者在 claim 與 publish 之間消失，lease 到期後這一
// 列自己回到候選集合，不需要任何對帳 job。
func (r *Repository) Claim(ctx context.Context, id int64, leaseUntil, claimableBefore time.Time) (int64, error) {
	return r.q(ctx).ClaimProcurementRfqPublicationOutbox(ctx, gen.ClaimProcurementRfqPublicationOutboxParams{
		LeaseUntil:      repository.TimestamptzParam(leaseUntil),
		ID:              id,
		ClaimableBefore: repository.TimestamptzParam(claimableBefore),
	})
}

// UpdateAttempt 記錄一次非終態重試：累加 attempt_count、依呼叫端算好的 backoff 排
// next_attempt_at、存下 lastError（空字串寫入 SQL NULL）。
func (r *Repository) UpdateAttempt(ctx context.Context, id int64, attemptCount int, nextAttemptAt time.Time, lastError string) error {
	return r.q(ctx).UpdateProcurementRfqPublicationOutboxAttempt(ctx, gen.UpdateProcurementRfqPublicationOutboxAttemptParams{
		ID:            id,
		AttemptCount:  int32(attemptCount),
		NextAttemptAt: repository.TimestamptzParam(nextAttemptAt),
		LastError:     nullableText(lastError),
	})
}

// MarkDone 把 outbox 列轉 status='done' 並蓋 completed_at。
//
// ⛔ 這裡刻意**不**碰 invite：invite 的 SENT 是 ProcessFunc 在 publish 成功之後自己
// 轉的（見 worker.go 的順序說明）。MarkDone 失敗會讓這一列重跑，那時 invite 已經是
// SENT，MarkInviteSent 的 CAS 影響 0 列——那是重試路徑的正常狀態，不是錯誤。
func (r *Repository) MarkDone(ctx context.Context, id int64, now time.Time) error {
	completedAt := now
	return r.q(ctx).MarkProcurementRfqPublicationOutboxDone(ctx, gen.MarkProcurementRfqPublicationOutboxDoneParams{
		ID:          id,
		CompletedAt: &completedAt,
	})
}

// MarkFailed 把 outbox 列轉終態 failed，並在**同一個交易內**把對應 invite 轉 FAILED。
//
// ⛔ 不可改用 pkg/outbox.Hooks 達成，兩個 hook 都不行：
//   - OnAttemptFailed 對**每一次**嘗試失敗都觸發，第一次暫時性失敗就會把 invite 打成
//     FAILED，而它其實還會被重試——對外狀態會說謊。
//   - OnTerminalFailed 雖然只在終態觸發，但它的簽名帶 markFailedErr，代表它是在
//     MarkFailed 回來**之後**才跑，不可能與那次寫入原子；中間留下「outbox 已 failed、
//     invite 仍 QUEUED」的窗口。設計書 §4.4。
//
// 這個原子性是「ProcessFunc 看得到的 invite 狀態只有 QUEUED 與 SENT 兩種」（§4.1）
// 那句話成立的前提，不是風格偏好——MarkPublicationInviteSent 的 CAS「影響 0 列必然是
// 已經 SENT」正是靠它。
//
// 兩句都走 r.q(txCtx)，而 q 用 database.GetDBTX 取 ctx 上的交易，所以兩次寫入確實在
// 同一個 pgx.Tx 上；第二句失敗會讓第一句一起 ROLLBACK，worker 下一輪重試整段。
func (r *Repository) MarkFailed(ctx context.Context, id int64, lastErr string, now time.Time) error {
	return r.txm.WithTx(ctx, func(txCtx context.Context) error {
		completedAt := now
		if err := r.q(txCtx).MarkProcurementRfqPublicationOutboxFailed(txCtx, gen.MarkProcurementRfqPublicationOutboxFailedParams{
			ID:          id,
			LastError:   nullableText(lastErr),
			CompletedAt: &completedAt,
		}); err != nil {
			return err
		}
		return r.q(txCtx).MarkProcurementRfqPublicationInviteFailedByOutboxID(txCtx, gen.MarkProcurementRfqPublicationInviteFailedByOutboxIDParams{
			ID:              id,
			DeliveryStateAt: repository.TimestamptzParam(now),
		})
	})
}

// MarkInviteSent 把 outbox 列對應的 invite 從 QUEUED CAS 成 SENT，委派給既有 owner。
// 影響 0 列不是錯誤（重試路徑，見 InviteSentMarker 與該方法自身的註解）。
func (r *Repository) MarkInviteSent(ctx context.Context, inviteID int64, now time.Time) error {
	return r.invites.MarkPublicationInviteSent(ctx, inviteID, now)
}

// CountPending 是 pkg/outbox.MonitorRepo：backlog gauge 的取樣來源。
func (r *Repository) CountPending(ctx context.Context) (int, error) {
	count, err := r.q(ctx).CountPendingProcurementRfqPublicationOutbox(ctx)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// OldestPendingAge 回傳最舊一列 pending 相對 now 的年齡；沒有 pending 列時 ok=false。
func (r *Repository) OldestPendingAge(ctx context.Context, now time.Time) (time.Duration, bool, error) {
	createdAt, err := r.q(ctx).OldestPendingProcurementRfqPublicationOutboxCreatedAt(ctx)
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
	deleted, err := r.q(ctx).DeleteDoneProcurementRfqPublicationOutboxBefore(ctx, gen.DeleteDoneProcurementRfqPublicationOutboxBeforeParams{
		CompletedAt: &completedAt,
		Limit:       int32(limit),
	})
	if err != nil {
		return 0, err
	}
	return int(deleted), nil
}

func toRow(row gen.ProcurementRfqPublicationOutbox) *Row {
	out := &Row{
		ID:            row.ID,
		InviteID:      row.InviteID,
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
