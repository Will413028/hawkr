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

// ---------------------------------------------------------------------------
// INV.dl —— 期限與兜底掃描（§4.3.3 第 7 組、§7.2.5「到期重試」）
//
// 🔴 **本波只跑 inbox 的 ORDER_PAID 兜底掃描一件事。** 設計書指派給 INV.dl 的其餘工作
// （K04／K05／K06 申訴 SLA、I8 REISSUE_FAILED 唯讀掃描）落地時**必須併進同一個 tick**：
// 另開一支排程器會在 `outbox.NewMetrics("invoice_deadline")` 同名時 boot 期直接 panic
// （promauto 重複註冊），而那個 panic 出現在部署當下、不是編譯期。
//
// 這支存在的理由（不是「多一層保險」，是補上一個**從來沒有執行者**的機制）：
// order.paid 的 handler 先 Insert（→RECEIVED）再讀兩支 gRPC、再 Prepare。中間任一步失敗
// 就回 error，而 pkg/mq 的重投會在 Insert 的去重那一步 return nil（ACK）什麼都不重做
// ⇒ 該列**永遠**停在 RECEIVED、發票永遠不會開、DLQ 是空的、沒有任何告警。
// 程式註解與 SQL 都寫著「由兜底掃描重試」，而 FetchDueInboxEvents 的 Go 呼叫端是零。
// dev 上因此卡了 4 筆 ORDER_PAID（對應訂單 invoice_documents 各 0 列）。
//
// 掃兩種 event_type，走**不同的 SQL**（#3073 起）：
//
//	ORDER_PAID              → FetchDueByType           （RECEIVED／DEFERRED／FAILED）
//	ORDER_REFUND_FINALIZED  → FetchDueActionableByType （RECEIVED／FAILED，**排除 DEFERRED**）
//
// 🔴 **退款那一邊排除 DEFERRED 不是最佳化，是正確性**：DEFERRED 的歸宿是反向認領
// （order.paid 到達時 reclaimDeferred 撿走）。兜底掃描碰它就會在失敗時翻成 FAILED
// ⇒ `WHERE status='DEFERRED'` 的述詞永久不成立 ⇒ 該筆退款再也不會被認領，
// 而且完全無聲（症狀是「某一筆退款沒有折讓單」，要對帳才看得出來）。
// 守它的是 TestDeadlineWorker_RefundScanNeverTouchesDeferred。
//
// ⚠️ 「先補租約／CAS 再打開退款掃描」這個前置條件，實際處置與原本設想的不同：
// **沒有補租約，因為唯一鍵比租約更強**。兩道機制取代它——
// (a) 兩條路徑的候選集合不相交（上面那格）；
// (b) 就算相交，`uq_invadj_allowance_refund`／`uq_invadj_void_once` 會讓第二次
//     Insert 走 ON CONFLICT DO NOTHING ⇒ RefundAdjustment.Apply 讀到 inserted=false
//     就 return nil。租約會過期（崩潰、時鐘漂移、GC 停頓），唯一鍵不會。
// 全文在 internal/mq/di_test.go 的 TestAdjustmentApplierIsWired。
//
// 仍然「不掃」的一格：
//   - **不自己標狀態**：MarkProcessed／MarkFailed／UpdateAttempt 全部由 pkg/outbox.Worker
//     依 Process 的回傳值決定，ProcessFunc 內動狀態會讓 Worker 的簿記打在空集合上。
// ---------------------------------------------------------------------------

// inboxRow 把 domain.InboxEvent 套進 pkg/outbox.Row。
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

// deadlineInboxRepo 是 pkg/outbox.Repository[inboxRow] 的實作。
type deadlineInboxRepo struct {
	repo domain.InboxRepository
	// parkInterval 是「打滿 MaxAttempts 之後多久再試一次」。見 NewDeadlineWorker 的說明：
	// 本表沒有真正的終態，MarkFailed 必須自己帶退避，否則會變成每 tick 重撈的熱迴圈。
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
	// 🔴 退款走 **Actionable** 那一支（排除 DEFERRED）。用 FetchDueByType 的話，
	// 亂序落 DEFERRED 的那些列會被撈進來，失敗時翻成 FAILED ⇒
	// ListDeferredByAggregateRef 的反向認領述詞永久不成立、該筆退款再也不會被認領，
	// 而且完全無聲。這一格是本次擴充唯一真正危險的地方。
	refunds, err := r.repo.FetchDueActionableByType(ctx, string(pkgmq.EventTypeOrderRefundFinalized), now, int32(limit)) //nolint:gosec // limit 是常數
	if err != nil {
		return nil, err
	}

	// 合併後依 next_attempt_at 排序再截斷：兩種型別各自有 LIMIT，合併取最早到期的
	// 才不會讓一種的積壓吃光另一種的處理量（那正是 FetchDueInboxEventsByType 檔頭
	// 「型別過濾在 SQL 不在 Go 端」要防的事——差別在於這裡每一列都會被處理，
	// 不是被 skip 掉之後還佔著名額）。
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
	// K05：申訴逾 3 工作天未回覆（#3055 的橫向稽查）。
	//
	// 🔴 併進**本 worker 的同一個 tick**，不另開排程器 —— 檔頭那條約束不是風格問題：
	// 另一支用同名 `outbox.NewMetrics("invoice_deadline")` 會在 promauto 重複註冊時
	// boot 期直接 panic，而那個 panic 出現在部署當下、不是編譯期。
	overdue OverdueComplaintSweeper
	// statutory 是 §5.2.1 的逾期分類（#3261 的延伸，2026-09-09）。可以是 nil ⇒ 跳過。
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
				// 一則 order.paid 事件停泊 ＝ 有一張訂單的發票沒有開、也沒有留檔文件。
				// 它會在 parkInterval 之後自己回到候選集合（本表無終態），但在那之前
				// 需要有人看見——`invoice_inbox_stranded_rows`（M30）＋ 告警
				// InvoiceInboxStranded 就是為此。
				// ⚠️ 這裡曾經寫「M29 invoice_inbox_stuck 與告警 ㉒」：M29 不存在，
				// 而 ㉒ 是 InvoiceTrackExhausted（字軌用盡），與 inbox 無關。
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

	// 🔴 **三件事各自獨立、任何一件的錯誤都不得吃掉後面那件**（2026-09-08 修）。
	//
	// 原本的寫法在 K04 失敗且 inbox 成功時直接 `return err` ⇒ 後面新增的任何一段
	// 都會被靜默跳過。那不是假想的風險：K05 就是接在它後面的，而 K04 最可能失敗的
	// 時候（DB 不穩、通知 outbox 寫不進去）正是申訴最需要被升級的時候。
	//
	// sweeper 沒 wire 時安靜跳過（測試與部分組裝路徑不掛它），而不是 panic。
	var sweepErr error
	if w.overdue != nil {
		// K04：逾 3 工作天未回覆的申訴 → OVERDUE ＋ 通知賣家。
		n, err := w.overdue.SweepOverdueComplaints(ctx, time.Now().UTC())
		switch {
		case err != nil:
			w.log.Error("invoice.dl.overdue_sweep_failed", zap.Error(err))
			sweepErr = err
		case n > 0:
			w.log.Info("invoice.dl.overdue_swept", zap.Int("count", n))
		}

		// K05：OVERDUE 滿 24 小時仍無人回覆 → ESCALATED_TO_ADM，平台接手。
		// 它處理的是**既有**的 OVERDUE，與 K04 這一輪標到幾筆完全無關。
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

		// K06：賣家已回覆、買家異議期屆滿仍未表態 → RESOLVED（逾期視為接受）。
		// 與前兩支各自獨立：它處理的是**已經有賣家回覆**的案件，與逾期未回覆那條軸無關。
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

	// §5.2.1：已逾法定上傳期限的文件轉 AWAITING_MANUAL ＋ DEADLINE_MISSED。
	// 與申訴那三支各自獨立（不同的軸、不同的 usecase），錯誤同樣不互相吃掉。
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

// process 是 ProcessFunc：重跑 §7.1.2 步驟 5–9。
//
// 🔴 **這個函式裡不得出現任何 Mark\***。標記由 pkg/outbox.Worker 在本函式回 nil 之後做；
// 把它挪進來等於自己決定「處理完了沒有」，而 Worker 的 attempt 簿記會打在一列已經離開
// 候選集合的資料上（rows=0 靜默）。
func (w *DeadlineWorker) process(ctx context.Context, row inboxRow) (any, error) {
	// 🔴 分派錨在 event_type。走錯的後果不是錯誤而是**靜默成功**：
	// 兩個 payload struct 的欄名沒有交集 ⇒ unmarshal 全部拿到零值 ⇒
	// 一筆金額 0 的退款會安靜地跑完，或一張沒有 order_no 的發票被 fail-loud 擋掉
	// （後者還算好的）。所以 default 一律回 error，不 fallback。
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
