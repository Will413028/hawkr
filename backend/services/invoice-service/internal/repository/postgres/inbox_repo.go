package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/repository/postgres/gen"
)

// InboxRepository 是 invoice_inbox_events 的 thin adapter。
//
// 這張表**沒有自己的排程器**（§4.1.3／§4.3.3 第 7 組）：DEFERRED 列由開票成功後的反向
// 認領推進（主路徑）；到期重試（兜底）由 INV.dl 那一輪順帶跑
// （internal/worker/deadline_worker.go，走 FetchDueByType／Claim／UpdateAttempt）。
// 看到「inbox worker」出現在 PR 裡，先問它為什麼不是反向認領——掛在 INV.dl 上的兜底
// 掃描是設計指定的機制，不在此列。
//
// 🔴 本檔沒有、也不得新增任何 Delete：去重鍵是「同一筆退款不會被開第二張折讓單」的唯一
// 防線，而上游重發沒有時間上界（每次換一個新的 UUID event_id）。鍵被刪掉的那一刻，
// 下一輪重發就是一個「全新的事件」（I-49）。payload 的瘦身走 PurgeProcessedPayloads——
// 它清的是 payload 欄位，不是列。
type InboxRepository struct {
	baseRepo
}

// NewInboxRepository 建立 repository。
func NewInboxRepository(pool *pgxpool.Pool) *InboxRepository {
	return &InboxRepository{baseRepo: baseRepo{pool: pool}}
}

var _ domain.InboxRepository = (*InboxRepository)(nil)

// Insert 收下一則入向事件，走 ON CONFLICT DO NOTHING（不指名 conflict target：本表有
// 兩個唯一約束，指名其一 ⇒ 另一個衝突時拋 23505 ⇒ 整個交易 aborted 25P02）。
//
// inserted=false ⇒ 已處理過，consumer 直接 ACK。這是**正常的去重結果不是錯誤**，所以
// 不回業務碼——回錯誤會讓 consumer 走 retry/DLQ，而那則訊息本來就該被丟掉。
//
// Status 在 INSERT 當下就要決定（不是先寫 RECEIVED 再更新）：handler 在同一個交易內已經
// 知道前置條件到了沒有——查得到該 aggregate_ref 的 document ⇒ RECEIVED；查不到 ⇒ DEFERRED。
func (r *InboxRepository) Insert(ctx context.Context, in domain.NewInboxEvent) (int64, bool, error) {
	rows, err := r.q(ctx).InsertInboxEvent(ctx, gen.InsertInboxEventParams{
		EventID:          in.EventID,
		EventType:        in.EventType,
		RoutingKey:       in.RoutingKey,
		Exchange:         in.Exchange,
		BusinessDedupKey: in.BusinessDedupKey,
		AggregateRef:     in.AggregateRef,
		Payload:          in.Payload,
		Status:           in.Status,
		DeferredUntil:    in.DeferredUntil,
		NextAttemptAt:    ts(in.NextAttemptAt),
		OccurredAt:       ts(in.OccurredAt),
		Now:              ts(in.Now),
	})
	if err != nil {
		return 0, false, err
	}
	// ON CONFLICT DO NOTHING RETURNING id：插入回 1 列、衝突回 0 列。
	// 回 0 列是正常的去重結果，不是錯誤。
	if len(rows) == 0 {
		return 0, false, nil
	}
	return rows[0], true, nil
}

// FetchDue 是兜底掃描的候選（由 INV.dl 那一輪順帶跑，不新增排程器）。
func (r *InboxRepository) FetchDue(ctx context.Context, now time.Time, limit int32) ([]domain.InboxEvent, error) {
	rows, err := r.q(ctx).FetchDueInboxEvents(ctx, gen.FetchDueInboxEventsParams{
		Now:      ts(now),
		RowLimit: limit,
	})
	if err != nil {
		return nil, err
	}
	return toInboxEvents(rows), nil
}

// FetchDueByType 是兜底掃描的候選，限定單一 event_type（INV.dl 用）。
//
// 型別過濾在 SQL 不在 Go：在 Go 端 skip 掉不處理的型別，那些列每一輪都會重新佔掉
// batch 名額而永遠不推進 ⇒ 一種事件的積壓會靜默吃光另一種的處理量。
func (r *InboxRepository) FetchDueByType(ctx context.Context, eventType string, now time.Time, limit int32) ([]domain.InboxEvent, error) {
	rows, err := r.q(ctx).FetchDueInboxEventsByType(ctx, gen.FetchDueInboxEventsByTypeParams{
		EventType: eventType,
		Now:       ts(now),
		RowLimit:  limit,
	})
	if err != nil {
		return nil, err
	}
	return toInboxEvents(rows), nil
}

// FetchDueActionableByType 同上但**排除 DEFERRED**（#3073）。
//
// 🔴 給退款路徑用：DEFERRED 的歸宿是反向認領（order.paid 到達時 reclaimDeferred 撿走），
// 兜底掃描碰它就會在失敗時把它翻成 FAILED ⇒ 反向認領的述詞永久不成立、完全無聲。
func (r *InboxRepository) FetchDueActionableByType(
	ctx context.Context, eventType string, now time.Time, limit int32,
) ([]domain.InboxEvent, error) {
	rows, err := r.q(ctx).FetchDueActionableInboxEventsByType(ctx, gen.FetchDueActionableInboxEventsByTypeParams{
		EventType: eventType,
		Now:       ts(now),
		RowLimit:  limit,
	})
	if err != nil {
		return nil, err
	}
	return toInboxEvents(rows), nil
}

// UpdateAttempt 記一次「這輪失敗、待會再試」：不動 status，只寫次數、退避與錯誤。
//
// attemptCount 是 pkg/outbox.Worker 算好的**絕對值**，SQL 也寫絕對值（不是 +1）：
// 兩種寫法今天的數值巧合相等，而巧合不是契約。
func (r *InboxRepository) UpdateAttempt(ctx context.Context, id int64, attemptCount int32, nextAttemptAt time.Time, lastError *string) (bool, error) {
	rows, err := r.q(ctx).UpdateInboxEventAttempt(ctx, gen.UpdateInboxEventAttemptParams{
		AttemptCount:  attemptCount,
		NextAttemptAt: ts(nextAttemptAt),
		LastError:     lastError,
		ID:            id,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// Claim 搶佔一則事件。
func (r *InboxRepository) Claim(ctx context.Context, id int64, leaseUntil, claimableBefore time.Time) (bool, error) {
	rows, err := r.q(ctx).ClaimInboxEvent(ctx, gen.ClaimInboxEventParams{
		LeaseUntil:      ts(leaseUntil),
		ID:              id,
		ClaimableBefore: ts(claimableBefore),
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// MarkProcessed 標記事件已被業務處理完成。
func (r *InboxRepository) MarkProcessed(ctx context.Context, id int64, now time.Time) (bool, error) {
	rows, err := r.q(ctx).MarkInboxEventProcessed(ctx, gen.MarkInboxEventProcessedParams{
		Now: tp(now),
		ID:  id,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// MarkSkipped 記「這則事件刻意不處理」與理由。
//
// disposition ＋ reason 兩欄是處置留證：ADM 必須答得出「這筆退款為什麼沒有開折讓」。
// 不填理由等於把那個問題留給未來的人去猜。
func (r *InboxRepository) MarkSkipped(ctx context.Context, id int64, disposition, reason string, now time.Time) (bool, error) {
	rows, err := r.q(ctx).MarkInboxEventSkipped(ctx, gen.MarkInboxEventSkippedParams{
		Disposition:       sp(disposition),
		DispositionReason: sp(reason),
		Now:               tp(now),
		ID:                id,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// MarkDeferred 把亂序抵達的事件延後（refund 早於 paid 抵達時不得丟棄）。
func (r *InboxRepository) MarkDeferred(ctx context.Context, id int64, deferredUntil time.Time) (bool, error) {
	rows, err := r.q(ctx).MarkInboxEventDeferred(ctx, gen.MarkInboxEventDeferredParams{
		DeferredUntil: tp(deferredUntil),
		ID:            id,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// MarkFailed 記一次處理失敗並重排。
func (r *InboxRepository) MarkFailed(ctx context.Context, id int64, lastError string, nextAttemptAt time.Time) (bool, error) {
	rows, err := r.q(ctx).MarkInboxEventFailed(ctx, gen.MarkInboxEventFailedParams{
		LastError:     sp(lastError),
		NextAttemptAt: ts(nextAttemptAt),
		ID:            id,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// ListDeferredByAggregateRef 是反向認領的入口：開票成功後拿 aggregate_ref 回來撈那些
// 因為前置條件還沒到而被延後的事件。
func (r *InboxRepository) ListDeferredByAggregateRef(ctx context.Context, aggregateRef string) ([]domain.InboxEvent, error) {
	rows, err := r.q(ctx).ListDeferredInboxEventsByAggregateRef(ctx, sp(aggregateRef))
	if err != nil {
		return nil, err
	}
	return toInboxEvents(rows), nil
}

// CountStaleDeferred 回延後太久還沒被認領走的事件數。
//
// ⚠️ **7 天告警的生產路徑不是這裡**：它走 sql_exporter 的
// `invoice_inbox_stranded_rows`（M30，monitoring/sql-exporter/sql_exporter.yml）
// ——監測是 metric ＋ Alertmanager，不是應用層掃描（CLAUDE.md §22）。
// 本方法留著是給 ADM／維運查詢用；註解曾寫「給告警用」而生產側呼叫端為 0，
// 那句話讓兩位審查者相信有一道不存在的防線。
func (r *InboxRepository) CountStaleDeferred(ctx context.Context, cutoff time.Time) (int64, error) {
	return r.q(ctx).CountStaleDeferredInboxEvents(ctx, ts(cutoff))
}

// PurgeProcessedPayloads 把已處理事件的 payload 欄位清空（§5.4.6）。
// 🔴 清的是欄位不是列——列永遠留著當去重鍵。
func (r *InboxRepository) PurgeProcessedPayloads(ctx context.Context, now, cutoff time.Time, limit int32) (int64, error) {
	return r.q(ctx).PurgeProcessedInboxPayloads(ctx, gen.PurgeProcessedInboxPayloadsParams{
		Now:      tp(now),
		Cutoff:   tp(cutoff),
		RowLimit: limit,
	})
}
