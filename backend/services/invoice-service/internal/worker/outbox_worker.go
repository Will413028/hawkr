package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/go-pkg/outbox"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/notifyrouting"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// INV.ob —— outbox 派送（§4.3.3 第 3 組、§7.1.2 步驟 15）
//
// 🔴 **順序是 publish → MarkDone，不可對調。** 反過來（先標完成再發）的失效形狀是
// 「事件蒸發」：MarkDone 成功、publish 失敗 ⇒ 那一列變成 done、永遠不會再被撈到，
// 而買家該收到的發票通知從此不存在，沒有任何錯誤、沒有任何告警。
// 本順序由 pkg/outbox.Worker 保證（processRow：Process 回 nil 才 MarkDone），
// 而「Process 就是 publish 本身」是本檔唯一要守住的事。
//
// 相反方向的代價是重送（publish 成功但 MarkDone 失敗 ⇒ 下一輪再發一次），由
// notification-service 的冪等鍵 `(event_type, reference_id, user_id, user_type)`
// 承接 —— 這正是每則 payload 都自帶 `notification_ref_id` 的理由（§6.3.3.4）。
// ---------------------------------------------------------------------------

// outboxRow 把 domain.OutboxEvent 套進 pkg/outbox.Row。
type outboxRow struct{ ev domain.OutboxEvent }

func (r outboxRow) OutboxID() int64         { return r.ev.ID }
func (r outboxRow) OutboxAttemptCount() int { return int(r.ev.AttemptCount) }

// RawPublisher 是 INV.ob 需要的發布面。
type RawPublisher interface {
	PublishRaw(ctx context.Context, exchange, routingKey string, body []byte) error
}

// SellerAccountResolver 把賣家業務鍵（store_id）解析成登入 principal（account_id）。
//
// §6.3.3.2：`notify_*_account_id` 的解析在 **outbox 派送時**做、不在業務交易內做
// —— 業務交易內做的話，一次 gRPC 逾時會把整筆開票交易一起帶走。
//
// 解析失敗帶 `"0"` 讓下游 skip、**不阻斷派送**：買家向的通知靠 `buyer_id` 路由，
// 不依賴這個值；賣家向的那一則少送一次，比整批事件卡住好。
type SellerAccountResolver interface {
	ResolveSellerAccountID(ctx context.Context, storeID int64) (int64, error)
}

// UnresolvedSellerAccountResolver 在 SEL peer client 落地前佔位（§6.2）。
//
// ⚠️ 它回 error ⇒ 每一則事件的 `notify_seller_account_id` 都會是 `"0"` ⇒ **賣家向的
// 站內通知與 FCM 全部不會送達**（買家向不受影響）。這不是「暫時降級」而是一個必須被
// 記錄的缺口，所以 dispatch 端會逐次記 Warn。
type UnresolvedSellerAccountResolver struct{}

// ResolveSellerAccountID 一律回 0 與一個說明性的 error。
func (UnresolvedSellerAccountResolver) ResolveSellerAccountID(context.Context, int64) (int64, error) {
	return 0, fmt.Errorf("invoice: seller account resolver is not wired yet (see 58 §6.2)")
}

// outboxRepo 是 pkg/outbox.Repository[outboxRow] 的實作。
type outboxRepo struct {
	repo domain.OutboxRepository
	// parkInterval 是「打滿 MaxAttempts 之後多久再試一次」。見 MarkFailed 的說明：
	// 本表的終態 'failed' 是永久旗標且會讓告警反向，所以耗盡重試走停泊不走終結，
	// 而停泊必須自帶退避，否則會變成每 tick 重撈的熱迴圈。
	parkInterval time.Duration
	log          *zap.Logger
}

var _ outbox.Repository[outboxRow] = (*outboxRepo)(nil)

func (r *outboxRepo) FetchPendingDue(ctx context.Context, now time.Time, limit int) ([]outboxRow, error) {
	evs, err := r.repo.FetchPending(ctx, now, int32(limit)) //nolint:gosec // limit 來自 config
	if err != nil {
		return nil, err
	}
	rows := make([]outboxRow, 0, len(evs))
	for _, ev := range evs {
		rows = append(rows, outboxRow{ev: ev})
	}
	return rows, nil
}

func (r *outboxRepo) Claim(ctx context.Context, id int64, leaseUntil, claimableBefore time.Time) (int64, error) {
	ok, err := r.repo.Claim(ctx, id, leaseUntil, claimableBefore)
	if err != nil || !ok {
		return 0, err
	}
	return 1, nil
}

func (r *outboxRepo) MarkDone(ctx context.Context, id int64, now time.Time) error {
	return r.repo.MarkDone(ctx, id, now)
}

// MarkFailed 是 pkg/outbox.Worker 對「重試次數用盡」的呼叫點，本服務把它實作成
// **停泊**（§29 的 dlParkInterval 同一形狀），不是終態旗標。
//
// 🔴 為什麼不能照字面 `SET status='failed'`：
//   - 候選述詞（FetchPending／Claim）與 idx_invoutbox_pending 都是 status='pending'，
//     而本表沒有任何時間欄能讓 'failed' 回到候選集合 ⇒ 永久旗標（規則⑧禁止）
//   - CountPending／OldestPendingCreatedAt 的述詞同樣是 'pending' ⇒ 那一列一落到
//     'failed' 就退出 outbox_oldest_pending_age_seconds ⇒ **告警 InvoiceOutboxLag
//     會在事件真正卡死的那一刻由 firing 轉 resolved**（唯一在響的燈自己熄掉）
//   - DeleteDoneBefore 只刪 'done' ⇒ 那一列永久留在表上而永無執行者
//
// 停泊之後那一列仍是 'pending'（積壓指標看得到、告警持續亮著），next_attempt_at 被推到
// now+parkInterval（下一輪不會重撈）。真實的失敗成因幾乎都是可修的——broker 不可用、
// exchange 尚未 apply、DB 短暫不可用——修好之後停泊到期那一輪自己撿回來，不需要任何
// backfill CLI（§22 禁止）。重送由 notification-service 的冪等鍵承接（見檔頭）。
//
// 誠實標註的代價：修不好的那一列會永遠留在 pending，outbox_oldest_pending_age_seconds
// 不會回到 0。那正是要的——它需要一個真的會去看的人，而偵測面是
// invoice_outbox_stranded_rows（M31）＋ 告警 InvoiceOutboxStranded。
func (r *outboxRepo) MarkFailed(ctx context.Context, id int64, lastErr string, now time.Time) error {
	parked, err := r.repo.Park(ctx, id, now.Add(r.parkInterval), lastErr)
	if err != nil {
		return err
	}
	if !parked {
		// rows=0 ⇒ 那一列已不在 'pending'（唯一來源是維運 SQL 的隔離）。不是錯誤，
		// 但必須留痕：它從此沒有任何自動執行者，靠 M31 讓人看見。
		r.log.Warn("invoice.ob.park_skipped_row_not_pending", zap.Int64("outbox_id", id))
	}
	return nil
}

func (r *outboxRepo) UpdateAttempt(ctx context.Context, id int64, attemptCount int, nextAttemptAt time.Time, lastError string) error {
	var le *string
	if lastError != "" {
		le = &lastError
	}
	return r.repo.UpdateAttempt(ctx, id, int32(attemptCount), nextAttemptAt, le) //nolint:gosec // attemptCount 上界是 MaxAttempts
}

// OutboxWorker 是 INV.ob 的整支。
type OutboxWorker struct {
	w        *outbox.Worker[outboxRow]
	pub      RawPublisher
	sellers  SellerAccountResolver
	log      *zap.Logger
	obs      *outbox.Metrics
	monitorR outbox.MonitorRepo
}

// NewOutboxWorker 組出 INV.ob。
//
// parkInterval 的推導在 di.go；本函式只負責接線。
func NewOutboxWorker(
	repo domain.OutboxRepository,
	pub RawPublisher,
	sellers SellerAccountResolver,
	cfg outbox.Config,
	parkInterval time.Duration,
	obs *outbox.Metrics,
	log *zap.Logger,
) *OutboxWorker {
	l := orNopLogger(log)
	ow := &OutboxWorker{pub: pub, sellers: sellers, log: l, obs: obs, monitorR: &outboxMonitor{repo: repo}}
	ow.w = outbox.New[outboxRow](
		&outboxRepo{repo: repo, parkInterval: parkInterval, log: l},
		ow.process, cfg, outbox.Hooks[outboxRow]{
			OnAttemptFailed: func(_ context.Context, row outboxRow, cause string, err error) {
				obs.AttemptFailedTotal(cause).Inc()
				l.Error("invoice.ob.attempt_failed",
					zap.Int64("outbox_id", row.ev.ID), zap.String("routing_key", row.ev.RoutingKey),
					zap.String("cause", cause), zap.Error(err))
			},
			OnTerminalFailed: func(_ context.Context, row outboxRow, attempt int, lastErr string, markErr error) {
				if markErr == nil {
					obs.FailedTotal().Inc()
				}
				// 一則發票事件停泊 ＝ 有買家在停泊期間收不到「發票已開立」的通知。
				// 它會在 parkInterval 之後自己回到候選集合（見 outboxRepo.MarkFailed），
				// 但在那之前需要有人看見——outbox_oldest_pending_age_seconds 會持續
				// 上升（告警 InvoiceOutboxLag），久留不去的則由 M31／InvoiceOutboxStranded 接。
				l.Error("invoice.ob.parked",
					zap.Int64("outbox_id", row.ev.ID), zap.String("routing_key", row.ev.RoutingKey),
					zap.Int("attempt", attempt), zap.String("last_error", lastErr), zap.Error(markErr))
			},
		})
	return ow
}

// RunOnce 跑一輪派送。
func (w *OutboxWorker) RunOnce(ctx context.Context) error { return w.w.RunOnce(ctx) }

// RefreshMetrics 更新 outbox_pending_rows / outbox_oldest_pending_age_seconds（M11）。
func (w *OutboxWorker) RefreshMetrics(ctx context.Context) error {
	return w.obs.Refresh(ctx, w.monitorR, time.Now().UTC())
}

// process 是 ProcessFunc：解析通知路由欄位，然後發出去。
//
// 🔴 **這個函式裡不得出現任何 MarkDone。** 標記完成是 pkg/outbox.Worker 在本函式回 nil
// **之後**做的事；把它挪進來（不論放在 publish 前或後）都等於自己決定「發出去了沒有」，
// 而 publish 失敗時該列就永遠留不下來。
func (w *OutboxWorker) process(ctx context.Context, row outboxRow) (any, error) {
	body, err := w.withNotifyRouting(ctx, row.ev)
	if err != nil {
		return nil, err
	}
	if err := w.pub.PublishRaw(ctx, row.ev.Exchange, row.ev.RoutingKey, body); err != nil {
		return nil, fmt.Errorf("publish %s/%s: %w", row.ev.Exchange, row.ev.RoutingKey, err)
	}
	w.log.Info("invoice.ob.published",
		zap.Int64("outbox_id", row.ev.ID),
		zap.String("exchange", row.ev.Exchange),
		zap.String("routing_key", row.ev.RoutingKey),
		zap.String("event_type", row.ev.EventType))
	return row.ev.RoutingKey, nil
}

// withNotifyRouting 在派送當下補上 `notify_seller_account_id`（§6.3.3.2）。
//
// 為什麼在這裡而不是在業務交易內：解析要打 seller-service 的 gRPC，放進開票交易會讓
// 一次逾時把整筆開票一起回滾。
//
// 🔴 規則本體住在 internal/notifyrouting（葉 package），不在這裡：出向事件的生產者
// 在 internal/usecase，而 worker import usecase ⇒ 生產者的測試沒辦法 import 這裡。
// 規則留在 worker 的話，usecase 側只能對著自己抄的一份驗證，而「自己跟自己對必然
// 全綠」正是這批缺陷的結構性成因。搬成葉 package 之後，兩邊打的是同一份。
func (w *OutboxWorker) withNotifyRouting(ctx context.Context, ev domain.OutboxEvent) ([]byte, error) {
	body, err := notifyrouting.Apply(ctx, ev.Payload, w.sellers, w.log.With(zap.Int64("outbox_id", ev.ID)))
	if err != nil {
		return nil, fmt.Errorf("outbox %d: %w", ev.ID, err)
	}
	return body, nil
}

// outboxMonitor 把 domain.OutboxRepository 的三支監控查詢接上 pkg/outbox.MonitorRepo。
type outboxMonitor struct{ repo domain.OutboxRepository }

func (m *outboxMonitor) CountPending(ctx context.Context) (int, error) {
	n, err := m.repo.CountPending(ctx)
	return int(n), err
}

func (m *outboxMonitor) OldestPendingAge(ctx context.Context, now time.Time) (time.Duration, bool, error) {
	oldest, err := m.repo.OldestPendingCreatedAt(ctx)
	if err != nil || oldest == nil {
		return 0, false, err
	}
	return now.Sub(*oldest), true, nil
}

func (m *outboxMonitor) DeleteDoneBefore(ctx context.Context, cutoff time.Time, limit int) (int, error) {
	n, err := m.repo.DeleteDoneBefore(ctx, cutoff, int32(limit)) //nolint:gosec // limit 是常數
	return int(n), err
}
