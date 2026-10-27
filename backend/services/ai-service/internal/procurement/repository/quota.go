package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository/gen"
	"github.com/Will413028/hawkr/go-pkg/identity"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"go.uber.org/zap"
)

// procurementReservationTTL 必須大於 gateway 一次 turn 的 deadline
// （proxy.streamTurnDeadline，120s）留餘裕——比照 internal/usecase/transform_usecase.go
// 的 reservationTTL：doc 57 §7.6 明講過期是唯一的清理機制，不另建 reaper job。
const procurementReservationTTL = 5 * time.Minute

// ReserveQuota atomically reserves one slot in buyerID's periodStart bucket
// iff confirmed+pending usage is still below limit. 比照
// transform_usecase.go 的 reserveSlot——同一套 reserve/confirm/release 三態 +
// TTL + per-買家 advisory lock 的形狀，只是把「store+week」換成「buyer+period」。
// 併發下同一買家的多個 reserve 靠交易內 advisory lock 序列化（commit/rollback
// 時自動釋放，process 中途被殺也不會卡死鎖）。
//
// confirmedBefore 是這次預約前、已確認（durable）的用量——Task 12 的
// TurnResult.Quota.Remaining 需要這個數字，這裡本來就要查一次，回傳它不需要
// 呼叫端再多打一次 query。
func (r *Repo) ReserveQuota(ctx context.Context, buyerID identity.BuyerID, periodStart time.Time, limit int) (reservationID int64, reserved bool, confirmedBefore int, err error) {
	pgPeriod := timestamptzParam(periodStart)

	err = r.txManager.WithTx(ctx, func(txCtx context.Context) error {
		q := r.q(txCtx)

		if err := q.AcquireProcurementUsageLock(txCtx, int64(buyerID)); err != nil {
			return err
		}

		usage, err := q.GetProcurementUsageByBuyerAndPeriod(txCtx, gen.GetProcurementUsageByBuyerAndPeriodParams{
			BuyerID: int64(buyerID), PeriodStart: pgPeriod,
		})
		if err != nil {
			if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		} else {
			confirmedBefore = int(usage.UsageCount)
		}

		pending, err := q.CountPendingProcurementUsageReservations(txCtx, gen.CountPendingProcurementUsageReservationsParams{
			BuyerID: int64(buyerID), PeriodStart: pgPeriod,
		})
		if err != nil {
			return err
		}
		if confirmedBefore+int(pending) >= limit {
			return nil // reserved stays false
		}

		// Opportunistic cleanup of long-expired rows — bounded, indexed, essentially free.
		if err := q.DeleteExpiredProcurementUsageReservations(txCtx); err != nil {
			return err
		}

		reservationID = r.newID()
		if err := q.CreateProcurementUsageReservation(txCtx, gen.CreateProcurementUsageReservationParams{
			ID: reservationID, BuyerID: int64(buyerID), PeriodStart: pgPeriod,
			ExpiresAt: timestamptzParam(time.Now().Add(procurementReservationTTL)),
		}); err != nil {
			return err
		}
		reserved = true
		return nil
	})
	if err != nil {
		return 0, false, 0, err
	}
	return reservationID, reserved, confirmedBefore, nil
}

// confirmQuotaInTx converts a reservation into a durable usage increment.
// 唯一呼叫點是 AppendRevision（revision.go）已經開好的交易的最後一步——它刻意
// 不自己開交易：呼叫端的 txCtx 與 q 已經屬於那一個交易，用量確認因此跟終態結果
// 的持久化一起 commit 或一起 rollback（doc 57 §7.5 point 3）。
//
// q 必須是呼叫端那個交易自己的 *gen.Queries。傳一個另外從連線池取得的 q 會讓
// 下面的 advisory lock 在該敘述結束時就釋放、DELETE／INCREMENT 也落在交易外，
// 但編譯與測試都不會有任何徵兆——見 AppendRevision 呼叫點的完整說明。
//
// IncrementProcurementUsage 是無條件 +1，所以呼叫點必須確定自己在「真的寫入了
// 一個新版本」的路徑上，而不是冪等重播路徑，否則同一輪會被計費兩次。
//
// Takes the same per-買家 advisory lock as ReserveQuota/ReleaseQuota——沒有它，
// 一個併發中的 ReserveQuota 可能讀到尚未 +1 的舊 confirmed count，而這裡的
// DELETE 已經把該筆 reservation 從 pending count 中拿掉，兩邊各少算一筆，
// 讓多一個請求擠進本該已滿的額度。
func (r *Repo) confirmQuotaInTx(txCtx context.Context, q *gen.Queries, reservationID int64, buyerID identity.BuyerID, periodStart time.Time) error {
	if err := q.AcquireProcurementUsageLock(txCtx, int64(buyerID)); err != nil {
		return err
	}
	if err := q.DeleteProcurementUsageReservationByID(txCtx, reservationID); err != nil {
		return err
	}
	return q.IncrementProcurementUsage(txCtx, gen.IncrementProcurementUsageParams{
		BuyerID: int64(buyerID), PeriodStart: timestamptzParam(periodStart),
	})
}

// ReleaseQuota drops a reservation without confirming it (the turn failed
// before AppendRevision ran). Best-effort — logs on failure instead of
// propagating an error, mirroring transform_usecase.go's releaseSlot: the
// caller already has its own outcome to return, and an abandoned reservation
// self-heals via its TTL even if this also fails.
func (r *Repo) ReleaseQuota(ctx context.Context, reservationID int64, buyerID identity.BuyerID) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()

	err := r.txManager.WithTx(ctx, func(txCtx context.Context) error {
		q := r.q(txCtx)
		if err := q.AcquireProcurementUsageLock(txCtx, int64(buyerID)); err != nil {
			return err
		}
		return q.DeleteProcurementUsageReservationByID(txCtx, reservationID)
	})
	if err != nil {
		pkglogger.Error(ctx, "release procurement_usage reservation failed",
			zap.Int64("reservation_id", reservationID), zap.Error(err))
	}
}
