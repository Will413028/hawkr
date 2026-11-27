package worker

import (
	"context"
	"fmt"
	"sort"
	"time"

	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/go-pkg/outbox"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"go.uber.org/zap"
)


type inboxRow struct{ ev domain.InboxEvent }

func (r inboxRow) OutboxID() int64         { return r.ev.ID }
func (r inboxRow) OutboxAttemptCount() int { return int(r.ev.AttemptCount) }

// InboxOrderPaidReprocessor 是 INV.dl 需要的業務面：拿一列已落庫的 ORDER_PAID 事件重跑
// §7.1.2 步驟 5–9。
//
// 用窄介面而不是直接吃 *invoicemq.OrderPaidConsumer，理由與 RawPublisher／
// SellerAccountResolver 同源：worker 的測試不需要架 inbox repository、兩支 peer client
// 與整個 usecase。（此處 internal/worker → internal/mq 的 import 方向本來就成立，
// 反向才會 cycle。）
type InboxOrderPaidReprocessor interface {
	ReprocessFromInbox(ctx context.Context, ev domain.InboxEvent) error
}

// InboxRefundReprocessor 是退款事件的重跑入口（#3073）。
//
// 型別與 InboxOrderPaidReprocessor 相同但**刻意分成兩個**：分派錨在 event_type，
// 而兩者吃的 payload 形狀完全不同（orderPaidPayload vs orderRefundFinalizedPayload）。
// 合成一個介面的話，分派寫反不會編譯錯——只會在 unmarshal 時拿到一堆零值，
// 而零值的退款金額是 0，它會安靜地跑完。
type InboxRefundReprocessor interface {
	ReprocessFromInbox(ctx context.Context, ev domain.InboxEvent) error
}

type deadlineInboxRepo struct {
	repo domain.InboxRepository
	parkInterval time.Duration
	log          *zap.Logger
}

var _ outbox.Repository[inboxRow] = (*deadlineInboxRepo)(nil)

// FetchPendingDue 撈到期的 ORDER_PAID inbox 列。
//
// 🔴 型別過濾在 **SQL** 不在這裡：在 Go 端撈全部再 skip 掉 refund，那些列每一輪都會重新
// 佔掉 batch 名額而永遠不推進 ⇒ refund 的積壓會靜默吃光 ORDER_PAID 的處理量。
func (r *deadlineInboxRepo) FetchPendingDue(ctx context.Context, now time.Time, limit int) ([]inboxRow, error) {
	paid, err := r.repo.FetchDueByType(ctx, string(pkgmq.EventTypeOrderPaid), now, int32(limit)) //nolint:gosec // limit 是常數
	if err != nil {
		return nil, err
	}
	refunds, err := r.repo.FetchDueActionableByType(ctx, string(pkgmq.EventTypeOrderRefundFinalized), now, int32(limit)) //nolint:gosec // limit 是常數
	if err != nil {
		return nil, err
	}

	evs := make([]domain.InboxEvent, 0, len(paid)+len(refunds))
	evs = append(evs, paid...)
	evs = append(evs, refunds...)
	sort.SliceStable(evs, func(i, j int) bool { return evs[i].NextAttemptAt.Before(evs[j].NextAttemptAt) })
	if len(evs) > limit {
		evs = evs[:limit]
	}

	rows := make([]inboxRow, 0, len(evs))
	for _, ev := range evs {
		rows = append(rows, inboxRow{ev: ev})
	}
	return rows, nil
}

// Claim 是搶佔閘門，走既有的 ClaimInboxEvent（租約欄 ＝ next_attempt_at）。
//
// 它的 CAS 述詞（status IN (RECEIVED,DEFERRED,FAILED) AND next_attempt_at <= claimableBefore）
// 逐字鏡射候選述詞，這正是《多副本排程搶佔規範》規則①／③要的形狀：租約到期後該列會**自己
// 回到候選集合**，所以持有者中途崩潰不會讓它永久卡死（那是永久旗標的失效形狀）。
//
// 🔴 leaseUntil 由 pkg/outbox.Worker **逐筆**在 claim 當下現算（worker.go 的 processRow），
// 不是在批次迴圈外算一次：算一次會讓不變式從「單筆最壞耗時 < 租約」退化成「批次總耗時 <
// 租約」，隨 FetchBatch 一起破。
func (r *deadlineInboxRepo) Claim(ctx context.Context, id int64, leaseUntil, claimableBefore time.Time) (int64, error) {
	ok, err := r.repo.Claim(ctx, id, leaseUntil, claimableBefore)
	if err != nil || !ok {
		return 0, err
	}
	return 1, nil
}

// MarkDone 把這一列推進 PROCESSED。
//
// rows=0 **不是錯誤**：allowed-from 是 RECEIVED／DEFERRED／FAILED，走到 0 代表有別的路徑
// （consumer 自己的 markInboxProcessed）先標過了 —— 目標狀態已經達成。回 error 會讓
// Worker 走 bumpAttempt，把一列已經成功的事件重排進候選集合。
func (r *deadlineInboxRepo) MarkDone(ctx context.Context, id int64, now time.Time) error {
	ok, err := r.repo.MarkProcessed(ctx, id, now)
	if err != nil {
		return err
	}
	if !ok {
		r.log.Info("invoice.dl.inbox_already_marked", zap.Int64("inbox_id", id))
	}
	return nil
}

// MarkFailed 是「打滿 MaxAttempts」的歸宿：status='FAILED' ＋ **停泊而非終結**。
//
// 🔴 next_attempt_at 寫 `now + parkInterval`，不是 Worker 傳進來的 now：
// **本表沒有真正的終態** —— idx_invinbox_due 與候選述詞都含 'FAILED'，所以一列被標成
// FAILED 之後仍然是「到期可撈」的。直接透傳 now ⇒ 下一個 tick（5 分鐘）立刻重撈、
// 重打兩支 gRPC，變成一條沒有退避的熱迴圈。
//
// 為什麼不乾脆讓它離開候選集合：發票是法定義務，「永遠不再試」沒有合法終點。賣家補完
// 稅務設定之後，停泊到期的那一輪會自己把它撿回來，不需要任何 backfill CLI（§22 禁止）。
// 代價寫在 di.go 的 parkInterval：outbox_failed_total 不會回到 0，需要真的有人去看。
func (r *deadlineInboxRepo) MarkFailed(ctx context.Context, id int64, lastErr string, now time.Time) error {
	_, err := r.repo.MarkFailed(ctx, id, lastErr, now.Add(r.parkInterval))
	return err
}

// UpdateAttempt 記一次「這輪失敗、待會再試」：不動 status，只寫次數、退避與錯誤。
func (r *deadlineInboxRepo) UpdateAttempt(ctx context.Context, id int64, attemptCount int, nextAttemptAt time.Time, lastError string) error {
	var le *string
	if lastError != "" {
		le = &lastError
	}
	_, err := r.repo.UpdateAttempt(ctx, id, int32(attemptCount), nextAttemptAt, le) //nolint:gosec // attemptCount 上界是 MaxAttempts
	return err
}

// DeadlineWorker 是 INV.dl 的整支。
type DeadlineWorker struct {
	w      *outbox.Worker[inboxRow]
	uc     InboxOrderPaidReprocessor
	refund InboxRefundReprocessor
	log    *zap.Logger
	obs    *outbox.Metrics
	overdue OverdueComplaintSweeper
	statutory StatutoryDeadlineSweeper
}

// WithStatutorySweeper 掛上逾期分類。
//
// 用 setter：NewDeadlineWorker 已有 8 個位置參數，而這一支是可選的（nil 即跳過，
// 與 overdue 同慣例）。
func (w *DeadlineWorker) WithStatutorySweeper(s StatutoryDeadlineSweeper) *DeadlineWorker {
	w.statutory = s
	return w
}

// OverdueComplaintSweeper 是 K05 需要的業務面：把逾 SLA 的申訴標成 OVERDUE 並通知賣家。
//
// 用窄介面而不是直接吃 usecase：與 InboxOrderPaidReprocessor 同源理由，
// worker 的測試不需要架整個 complaint usecase。
type OverdueComplaintSweeper interface {
	SweepOverdueComplaints(ctx context.Context, now time.Time) (int, error)
	// EscalateOverdueComplaints 是 K05：OVERDUE 滿 24 小時 → ESCALATED_TO_ADM。
	// 與 K04 同一個介面而不是另開一支：兩者共用同一個 tick 與同一個 usecase，
	// 拆開只會多一個要記得接線的地方——而「忘了接線」正是 K05 在 2026-09-08
	// 之前零呼叫端的原因。
	EscalateOverdueComplaints(ctx context.Context, now time.Time) (int, error)
	// ResolveExpiredBuyerReviews 是 K06：SELLER_RESPONDED 且買家異議期屆滿 → RESOLVED。
	// 與 K04／K05 同一個 tick，理由同上。
	ResolveExpiredBuyerReviews(ctx context.Context, now time.Time) (int, error)
}

// StatutoryDeadlineSweeper 是 §5.2.1 的逾期分類：已過法定上傳期限的文件轉
// AWAITING_MANUAL ＋ DEADLINE_MISSED。
//
// 🔴 與上面三支分成兩個介面而不是塞進同一個：它們是**不同的軸**（申訴 vs 發票文件），
// 而且由不同的 usecase 提供（Complaint vs Issuance）。合成一個介面會逼出一個沒有人
// 是它自然 owner 的門面型別。
type StatutoryDeadlineSweeper interface {
	SweepOverdueStatutoryDeadlines(ctx context.Context, now time.Time) (int, error)
}

// NewDeadlineWorker 組出 INV.dl。
//
// parkInterval 與 cfg 的推導都在 di.go；本函式只負責接線。
func NewDeadlineWorker(
	repo domain.InboxRepository,
	uc InboxOrderPaidReprocessor,
	refund InboxRefundReprocessor,
	cfg outbox.Config,
	parkInterval time.Duration,
	overdue OverdueComplaintSweeper,
	obs *outbox.Metrics,
	log *zap.Logger,
) *DeadlineWorker {
	l := orNopLogger(log)
	dw := &DeadlineWorker{uc: uc, refund: refund, log: l, obs: obs, overdue: overdue}
	dw.w = outbox.New[inboxRow](
		&deadlineInboxRepo{repo: repo, parkInterval: parkInterval, log: l},
		dw.process, cfg, outbox.Hooks[inboxRow]{
			OnAttemptFailed: func(_ context.Context, row inboxRow, cause string, err error) {
				obs.AttemptFailedTotal(cause).Inc()
				l.Error("invoice.dl.attempt_failed",
					zap.Int64("inbox_id", row.ev.ID),
					zap.String("event_type", row.ev.EventType),
					zap.Stringp("order_no", row.ev.AggregateRef),
					zap.String("cause", cause), zap.Error(err))
			},
			OnTerminalFailed: func(_ context.Context, row inboxRow, attempt int, lastErr string, markErr error) {
				if markErr == nil {
					obs.FailedTotal().Inc()
				}
				l.Error("invoice.dl.parked",
					zap.Int64("inbox_id", row.ev.ID),
					zap.Stringp("order_no", row.ev.AggregateRef),
					zap.Int("attempt", attempt),
					zap.String("last_error", lastErr), zap.Error(markErr))
			},
		})
	return dw
}

// RunOnce 跑一輪兜底掃描 ＋ K05 申訴逾期掃描。
//
// 🔴 兩件事在**同一個 tick**（見 DeadlineWorker.overdue 的註解），而且**inbox 那一輪
// 失敗不得吃掉 K05**：兩者的候選集合完全無關，讓其中一個的錯誤阻斷另一個，等於把
// 「發票沒開」的問題升級成「申訴逾期也沒人掃」。錯誤合併回報，兩邊都會跑到。
func (w *DeadlineWorker) RunOnce(ctx context.Context) error {
	inboxErr := w.w.RunOnce(ctx)

	var sweepErr error
	if w.overdue != nil {
		n, err := w.overdue.SweepOverdueComplaints(ctx, time.Now().UTC())
		switch {
		case err != nil:
			w.log.Error("invoice.dl.overdue_sweep_failed", zap.Error(err))
			sweepErr = err
		case n > 0:
			w.log.Info("invoice.dl.overdue_swept", zap.Int("count", n))
		}

		m, eerr := w.overdue.EscalateOverdueComplaints(ctx, time.Now().UTC())
		switch {
		case eerr != nil:
			w.log.Error("invoice.dl.overdue_escalate_failed", zap.Error(eerr))
			if sweepErr == nil {
				sweepErr = eerr
			}
		case m > 0:
			w.log.Info("invoice.dl.overdue_escalated", zap.Int("count", m))
		}

		k, rerr := w.overdue.ResolveExpiredBuyerReviews(ctx, time.Now().UTC())
		switch {
		case rerr != nil:
			w.log.Error("invoice.dl.buyer_review_resolve_failed", zap.Error(rerr))
			if sweepErr == nil {
				sweepErr = rerr
			}
		case k > 0:
			w.log.Info("invoice.dl.buyer_reviews_resolved", zap.Int("count", k))
		}
	}

	if w.statutory != nil {
		n, serr := w.statutory.SweepOverdueStatutoryDeadlines(ctx, time.Now().UTC())
		switch {
		case serr != nil:
			w.log.Error("invoice.dl.statutory_sweep_failed", zap.Error(serr))
			if sweepErr == nil {
				sweepErr = serr
			}
		case n > 0:
			w.log.Info("invoice.dl.statutory_overdue_swept", zap.Int("count", n))
		}
	}

	if inboxErr != nil {
		return inboxErr
	}
	return sweepErr
}

func (w *DeadlineWorker) process(ctx context.Context, row inboxRow) (any, error) {
	switch row.ev.EventType {
	case string(pkgmq.EventTypeOrderPaid):
		if err := w.uc.ReprocessFromInbox(ctx, row.ev); err != nil {
			return nil, err
		}
	case string(pkgmq.EventTypeOrderRefundFinalized):
		if err := w.refund.ReprocessFromInbox(ctx, row.ev); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("invoice: dl has no reprocessor for event_type=%s (inbox_id=%d)",
			row.ev.EventType, row.ev.ID)
	}
	return row.ev.EventType, nil
}
