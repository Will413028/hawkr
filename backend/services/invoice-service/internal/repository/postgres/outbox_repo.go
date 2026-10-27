package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/repository/postgres/gen"
)

// OutboxRepository 是 invoice_outbox_events 的 thin adapter（INV.ob，租約 30s）。
//
// Insert **必須由業務交易內呼叫**：這張表的存在理由就是「業務寫入與事件發布同一個交易」，
// 事後才 enqueue 等於把 outbox 的保證丟掉。baseRepo.q 會在 ctx 帶交易時回那個交易，
// 所以呼叫端只要包在 TxManager.WithTx 裡就自動成立——本層不自己 Begin。
type OutboxRepository struct {
	baseRepo
}

// NewOutboxRepository 建立 repository。
func NewOutboxRepository(pool *pgxpool.Pool) *OutboxRepository {
	return &OutboxRepository{baseRepo: baseRepo{pool: pool}}
}

var _ domain.OutboxRepository = (*OutboxRepository)(nil)

// Insert 把一則出向事件排進 outbox。payload 必須在此刻就快照完成。
func (r *OutboxRepository) Insert(ctx context.Context, in domain.NewOutboxEvent) (domain.OutboxEvent, error) {
	row, err := r.q(ctx).InsertInvoiceOutboxEvent(ctx, gen.InsertInvoiceOutboxEventParams{
		AggregateType: in.AggregateType,
		AggregateID:   in.AggregateID,
		EventType:     in.EventType,
		Exchange:      in.Exchange,
		RoutingKey:    in.RoutingKey,
		Payload:       in.Payload,
		NextAttemptAt: ts(in.NextAttemptAt),
		Now:           ts(in.Now),
	})
	if err != nil {
		return domain.OutboxEvent{}, err
	}
	return toOutboxEvent(row), nil
}

// FetchPending 是 INV.ob 的候選。
func (r *OutboxRepository) FetchPending(ctx context.Context, now time.Time, limit int32) ([]domain.OutboxEvent, error) {
	rows, err := r.q(ctx).FetchPendingInvoiceOutboxEvents(ctx, gen.FetchPendingInvoiceOutboxEventsParams{
		Now:      ts(now),
		RowLimit: limit,
	})
	if err != nil {
		return nil, err
	}
	return toOutboxEvents(rows), nil
}

// Claim 搶佔一則事件。claimableBefore 是 CAS 上界（tickNow），leaseUntil 逐筆現算。
func (r *OutboxRepository) Claim(ctx context.Context, id int64, leaseUntil, claimableBefore time.Time) (bool, error) {
	rows, err := r.q(ctx).ClaimInvoiceOutboxEvents(ctx, gen.ClaimInvoiceOutboxEventsParams{
		LeaseUntil:      ts(leaseUntil),
		ID:              id,
		ClaimableBefore: ts(claimableBefore),
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// MarkDone 在 publisher confirm 之後把事件標成完成。
func (r *OutboxRepository) MarkDone(ctx context.Context, id int64, now time.Time) error {
	return r.q(ctx).MarkDoneInvoiceOutboxEvents(ctx, gen.MarkDoneInvoiceOutboxEventsParams{
		Now: tp(now),
		ID:  id,
	})
}

// Park 把耗盡重試的事件**停泊**：status 維持 'pending'、next_attempt_at 推到 parkUntil。
//
// 🔴 不是 `SET status='failed'`。理由寫在 domain.OutboxRepository.Park 與
// db/queries/invoice_outbox_events.sql 的 ParkInvoiceOutboxEvents：'failed' 在本表是
// 永久旗標，而且會讓積壓告警在卡死那一刻熄滅。
//
// rows=0 不是錯誤，是「那一列已離開 pending」（維運 SQL 隔離）；呼叫端據此記 log。
func (r *OutboxRepository) Park(ctx context.Context, id int64, parkUntil time.Time, lastError string) (bool, error) {
	rows, err := r.q(ctx).ParkInvoiceOutboxEvents(ctx, gen.ParkInvoiceOutboxEventsParams{
		ParkUntil: ts(parkUntil),
		LastError: sp(lastError),
		ID:        id,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// UpdateAttempt 記一次失敗並重排下次派送。
func (r *OutboxRepository) UpdateAttempt(ctx context.Context, id int64, attemptCount int32, nextAttemptAt time.Time, lastError *string) error {
	return r.q(ctx).UpdateAttemptInvoiceOutboxEvents(ctx, gen.UpdateAttemptInvoiceOutboxEventsParams{
		AttemptCount:  attemptCount,
		NextAttemptAt: ts(nextAttemptAt),
		LastError:     lastError,
		ID:            id,
	})
}

// CountPending 給 outbox metrics 的 Refresh 用。
func (r *OutboxRepository) CountPending(ctx context.Context) (int64, error) {
	return r.q(ctx).CountPendingInvoiceOutboxEvents(ctx)
}

// OldestPendingCreatedAt 回最舊一則未派送事件的建立時間；沒有未派送事件時回 nil。
//
// 回 nil 而不是零值 time.Time：呼叫端要算的是「積壓多久」，而零值會被算成 55 年，
// 那個數字會把告警打亮，而真相是「一則都沒積壓」。
func (r *OutboxRepository) OldestPendingCreatedAt(ctx context.Context) (*time.Time, error) {
	v, err := r.q(ctx).OldestPendingInvoiceOutboxEventsCreatedAt(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil // 一則都沒積壓，不是錯誤
	}
	if err != nil {
		return nil, err
	}
	if !v.Valid {
		return nil, nil
	}
	t := v.Time
	return &t, nil
}

// DeleteDoneBefore 清理已完成的舊事件（分批，避免長交易）。
//
// ⚠️ 只刪 **done** 的列。invoice_inbox_events 那張表則是**永不刪列**（去重鍵是唯一防線），
// 兩張表的保存策略相反，不要照抄。
func (r *OutboxRepository) DeleteDoneBefore(ctx context.Context, cutoff time.Time, limit int32) (int64, error) {
	return r.q(ctx).DeleteDoneInvoiceOutboxEventsBefore(ctx, gen.DeleteDoneInvoiceOutboxEventsBeforeParams{
		Cutoff:   tp(cutoff),
		RowLimit: limit,
	})
}
