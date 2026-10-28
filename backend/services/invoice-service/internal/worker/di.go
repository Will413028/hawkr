// Package worker 是 Layer 4.5：排程與搶佔。
//
// claim → 呼叫 usecase → 標記結果；把 ErrorClass 翻成 outbox 的重試／終結行為。
// 不含業務規則。
package worker

import (
	"context"
	"time"

	"github.com/Will413028/hawkr/go-pkg/outbox"
	"github.com/Will413028/hawkr/invoice-service/internal/client/seller"
	"github.com/Will413028/hawkr/invoice-service/internal/config"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	invoicemq "github.com/Will413028/hawkr/invoice-service/internal/mq"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// outbox_name 的值域。
//
// 🔴 `outbox.NewMetrics(name)` 走 promauto ⇒ **同一個 process 內同名呼叫兩次直接 panic**
// （§6.3.15 末段）。所以每張 outbox 各自 NewMetrics 一次、且各自只 provide 一次。
// 其餘六個名字（invoice_recheck／invoice_upload_recheck／invoice_pdf／invoice_governance／
// invoice_track／invoice_health_check）隨各自的排程器落地。
const (
	outboxNameIssue  = "invoice_issue"
	outboxNameOutbox = "invoice_outbox"
	// 🔴 用設計書指定的 invoice_deadline，**不新增第 10 個 outbox_name**：INV.dl 的其餘
	// 工作（K04–K06 申訴 SLA、I8 REISSUE_FAILED 掃描）落地時併進同一個 tick 即可。
	// 另開一個名字＋另開一支排程器，會在第二支 NewMetrics 時 boot 期 panic。
	outboxNameDeadline = "invoice_deadline"
)

// obDispatchInterval 是 INV.ob 的 tick 間隔。
//
// 沒有為它開 env：§0.2 沒給這個參數，而 outbox 派送的節奏由「事件多快要送到買家手上」
// 決定，不是可調的容量參數。5 秒是「使用者感受不到」與「空轉成本可忽略」的交會點。
const obDispatchInterval = 5 * time.Second

// obMonitorInterval 是 outbox 監控指標（M11 的 pending / oldest age）的取樣間隔。
// 低頻即可：它是告警的輸入不是控制迴圈。
const obMonitorInterval = 30 * time.Second

// obParkInterval 是 INV.ob「打滿 MaxAttempts 之後多久再試一次」。
//
// 🔴 它必須存在，理由與 dlParkInterval 同源但成因相反：invoice_inbox_events 是**沒有
// 終態**所以停泊才不會熱迴圈；invoice_outbox_events 則是**終態太終**——'failed' 沒有任何
// 時間欄能讓它回到候選集合（永久旗標），而且 CountPending／OldestPending 的述詞是
// 'pending' ⇒ 那一列一落到 'failed' 就把告警 InvoiceOutboxLag 從 firing 打成 resolved。
// 兩張表的正解都是「停泊」，只是它取代的東西不同。
//
// 為什麼是 1 小時：MaxAttempts=8、BaseBackoff=5s、MaxBackoff=10min 的完整退避序列
// （10+20+40+80+160+320+600 秒）≈ 20.5 分鐘。停泊間隔必須明顯大於它（否則只是把梯子
// 接長，看不出「這一列已經不正常」），又要小到讓修好之後的排空在一個小時內發生。
//
// 代價（誠實標註）：修不好的那一列會永遠留在 pending，outbox_oldest_pending_age_seconds
// 不會回到 0、告警持續亮著。那是刻意的——偵測面是 invoice_outbox_stranded_rows（M31）
// 與告警 InvoiceOutboxStranded；若最後的處置是把告警調靜音，就等於回到「靜默卡死」的原點。
const obParkInterval = 1 * time.Hour

// ---------------------------------------------------------------------------
// INV.dl 參數（§7.5.3／§9.4.4）。與 obDispatchInterval 同理**不開 env**：
// 這些是由「單筆最壞耗時」與「tick 預算」推導出來的不變式，不是可調的容量旋鈕。
// 兩個各自可調的數字必然漂移，而漂移的方向若是「租約 < 單筆耗時」，症狀是同一列被兩個
// 副本同時處理（本波的副作用是 Prepare，冪等由 uq_invdoc_order_identity 兜著，但
// attempt 計數與 log 會失真）。
// ---------------------------------------------------------------------------
const (
	// dlScanInterval：§7.5.3 給 INV.dl 的 tick 間隔。
	dlScanInterval = 5 * time.Minute

	// dlInitialDelay：§9 line 9992 的質數秒錯相位。多支排程器同時 Start 時，若首 tick
	// 全部對齊，DB 會週期性地吃到一個尖峰；質數秒讓它們長期不重合。
	dlInitialDelay = 41 * time.Second

	// dlFetchBatch = 20，**不是 §7.5.3 表上的 200**。
	//
	// 推導：單筆最壞耗時 ≈ 6.6s
	//   claim 0.05s ＋ ORD GetOrderInvoiceSnapshot 3s（client/order/client.go 的 deadline）
	//   ＋ PAY GetPaymentFacts 3s（client/payment/client.go）＋ Prepare ~0.2s（三次本地
	//   DB 讀 ＋ 一個純寫入交易，零外部呼叫）＋ 標記 ~0.3s
	// 20 × 6.6s = 132s，在 §9.4.4 給 INV.dl 的 240s tick 預算內。
	// 200 × 6.6s = 22 分鐘 ⇒ 會直接吃掉 §7.3.7 給 REISSUE_FAILED 的 15 分鐘 SLA。
	//
	// ⚠️ 這是**算出來的不是量出來的**：日後有人調大 peer deadline 或在 Prepare 裡加外部
	// 呼叫，240s 預算會被吃掉而沒有任何東西轉紅。落地後用 M25 的直方圖實測一輪回灌。
	dlFetchBatch = 20

	// dlClaimLease：§7.5.3。
	// 下界 ＝ 單筆最壞耗時 6.6s（餘裕 45 倍）；上界不存在——候選窗口（status ∈
	// RECEIVED/DEFERRED/FAILED）不會關閉，租約再長也只是延後下一次兜底。
	dlClaimLease = 5 * time.Minute

	// dlMaxAttempts／dlBaseBackoff／dlMaxBackoff：§7.5.3。
	dlMaxAttempts = 3
	dlBaseBackoff = 60 * time.Second
	dlMaxBackoff  = 10 * time.Minute

	// dlParkInterval 是「打滿 dlMaxAttempts 之後多久再試一次」。
	//
	// 🔴 它必須存在，因為 **invoice_inbox_events 沒有真正的終態**：候選述詞與
	// idx_invinbox_due 都含 'FAILED'。把 Worker 傳進來的 now 直接寫回 next_attempt_at
	// ⇒ 下一個 tick 立刻重撈、重打兩支 gRPC ⇒ 一條沒有退避的熱迴圈。
	//
	// 為什麼是「停泊」而不是「終結」：發票是法定義務，「永遠不再試」沒有合法終點。
	// 賣家補完稅務設定之後，停泊到期那一輪會自己把它撿回來——不需要 backfill CLI
	// （§22 本來就禁止）。
	// 代價（誠實標註）：outbox_failed_total{outbox_name="invoice_deadline"} 不會回到 0，
	// 一列修不好的事件每 6 小時把它加一。這需要一個真的會去看的人；若最後的處置是把
	// 告警調靜音，就等於回到「靜默卡住」的原點。
	dlParkInterval = 6 * time.Hour
)

// Module 是 Layer 4.5 的組裝。
//
// 兩條硬規則（§4.2.1）：
//   - 每個 outbox_name 的 NewMetrics 只能 provide 一次，重名即 boot 期 panic
//   - 排程一律走 lease claim，不用永久旗標（《多副本搶佔規範》規則⑧）
var Module = fx.Module("worker",
	fx.Provide(NewMetrics),
	fx.Provide(fx.Annotate(newIssueOutboxMetrics, fx.ResultTags(`name:"obs_issue"`))),
	fx.Provide(fx.Annotate(newOutboxOutboxMetrics, fx.ResultTags(`name:"obs_outbox"`))),
	fx.Provide(fx.Annotate(newDeadlineOutboxMetrics, fx.ResultTags(`name:"obs_deadline"`))),
	fx.Provide(fx.Annotate(newSellerAccountResolver, fx.As(new(SellerAccountResolver)))),
	fx.Provide(newIssueWorker),
	fx.Provide(newOutboxWorker),
	fx.Provide(fx.Annotate(newInboxReprocessor, fx.As(new(InboxOrderPaidReprocessor)))),
	fx.Provide(fx.Annotate(newRefundReprocessor, fx.As(new(InboxRefundReprocessor)))),
	// K05 的業務面（#3055 的橫向稽查）。與上一行同一個形狀：窄介面在這裡綁到
	// 具體 usecase，worker 那側只看得到介面。
	fx.Provide(fx.Annotate(newOverdueComplaintSweeper, fx.As(new(OverdueComplaintSweeper)))),
	fx.Provide(fx.Annotate(newStatutoryDeadlineSweeper, fx.As(new(StatutoryDeadlineSweeper)))),
	fx.Provide(newDeadlineWorker),
	// INV.trk 的兩半：observer 把四支 gauge 交給 usecase（usecase 不 import promauto），
	// worker 把「現在是不是掃描時點」的閘門包在 usecase 之外。
	fx.Provide(fx.Annotate(newTrackObserver, fx.As(new(usecase.TrackObserver)))),
	fx.Provide(newTrackWorker),
	fx.Provide(newHealthCheckWorker),
	fx.Provide(newRecheckWorker),
	fx.Provide(fx.Annotate(newGovernanceObserver, fx.As(new(usecase.GovernanceObserver)))),
	fx.Provide(newGovernanceWorker),
	fx.Provide(newPDFWorker),
	fx.Invoke(startSchedulers),
)

func newIssueOutboxMetrics() *outbox.Metrics    { return outbox.NewMetrics(outboxNameIssue) }
func newOutboxOutboxMetrics() *outbox.Metrics   { return outbox.NewMetrics(outboxNameOutbox) }
func newDeadlineOutboxMetrics() *outbox.Metrics { return outbox.NewMetrics(outboxNameDeadline) }

// newInboxReprocessor 把 C1 的 handler 當成 INV.dl 的業務面。
//
// 🔴 兜底掃描要重跑的**就是** order.paid handler 的步驟 5–9，所以它必須是同一段程式碼。
// 在 worker 這一側另寫一份「重跑邏輯」，是本 repo 反覆踩過的形狀：兩份會漂移，而漂移的
// 那一天不會有任何東西轉紅（兩邊各自的測試都綠）。
func newInboxReprocessor(c *invoicemq.OrderPaidConsumer) InboxOrderPaidReprocessor { return c }

// newRefundReprocessor 把退款 consumer 的重跑入口接給 INV.dl（#3073）。
func newRefundReprocessor(c *invoicemq.RefundFinalizedConsumer) InboxRefundReprocessor { return c }

// newOverdueComplaintSweeper 把 K05 綁到 Complaint usecase（它已持有 tx／complaints／
// issuers／outbox 四個依賴，正好是這件事需要的全部）。
func newOverdueComplaintSweeper(c *usecase.Complaint) OverdueComplaintSweeper { return c }

// newStatutoryDeadlineSweeper 把 §5.2.1 的逾期分類綁到 Issuance usecase（它已持有
// docs 與 audit，逾期分類只需要那兩樣）。
func newStatutoryDeadlineSweeper(i *usecase.Issuance) StatutoryDeadlineSweeper { return i }

// newSellerAccountResolver 接 §6.2 的 SEL peer client。
//
// `*seller.Client` 逐字滿足 SellerAccountResolver（`internal/client/seller/client_test.go`
// 有編譯期斷言守住），所以這裡只是把型別接上去，沒有轉接層。
func newSellerAccountResolver(c *seller.Client) SellerAccountResolver { return c }

type issueWorkerDeps struct {
	fx.In

	Ops  domain.OperationRepository
	Docs domain.DocumentRepository
	UC   *usecase.Issuance
	Void *usecase.Voiding
	// Allowance 是第三條軸（#3073）。三者共用同一組候選與租約策略，
	// 差別只在送出的東西與落點。
	Allowance *usecase.Allowance
	Cfg       *config.Config
	Metrics   *Metrics
	Obs       *outbox.Metrics `name:"obs_issue"`
	Log       *zap.Logger
}

// newIssueWorker 用 §0.2／§7.5.4 的參數組出 INV.op。
//
// 🔴 ClaimLease = INVOICE_ISSUE_LEASE_SECONDS（120s）。下界推導在 §7.5.4，**不是**
// 「看起來還有餘裕」就能調小的數字：把 code 51 那條 20s 的 invoice_list 從下界表拿掉，
// 下界會從 60.4s 掉到 40.3s，於是 60s 看起來安全——而真實最壞耗時仍是 60.4s ⇒ 持有者
// 還在翻 invoice_list 就被另一個副本合法搶走 ⇒ 重複開票，編譯／測試／metrics 全部正常。
// Config.Validate 已擋住「租約 <= AMEGO issue timeout」這一條下界。
func newIssueWorker(d issueWorkerDeps) *IssueWorker {
	return NewIssueWorker(d.Ops, d.Docs, d.UC, d.Void, d.Allowance, outbox.Config{
		MaxAttempts: d.Cfg.Schedule.IssueMaxRetry,
		BaseBackoff: time.Duration(d.Cfg.Schedule.BackoffBaseSeconds) * time.Second,
		MaxBackoff:  time.Duration(d.Cfg.Schedule.BackoffMaxSeconds) * time.Second,
		FetchBatch:  d.Cfg.Schedule.IssueBatchSize,
		ClaimLease:  time.Duration(d.Cfg.Schedule.IssueLeaseSeconds) * time.Second,
	}, d.Metrics, d.Obs, d.Log)
}

type outboxWorkerDeps struct {
	fx.In

	Repo    domain.OutboxRepository
	Pub     *invoicemq.EventPublisher
	Sellers SellerAccountResolver
	Cfg     *config.Config
	Obs     *outbox.Metrics `name:"obs_outbox"`
	Log     *zap.Logger
}

// newOutboxWorker 組出 INV.ob。
//
// ClaimLease = 30s（§4.3.3 第 3 組；下界 10.2s ＝ publisher confirm 10s ＋ stamp 0.2s）。
// 那個 10s 就是 INVOICE_MQ_PUBLISH_TIMEOUT_SECONDS ⇒ 租約直接由它推導，不另開 env：
// 兩個各自可調的數字必然漂移，而漂移的方向若是「租約 < confirm timeout」，症狀是
// 同一則事件被兩個副本同時發出去。
func newOutboxWorker(d outboxWorkerDeps) *OutboxWorker {
	return NewOutboxWorker(d.Repo, d.Pub, d.Sellers, outbox.Config{
		// 派送重試次數與開票重試次數是兩件事：發不出去通常是 broker 短暫不可用，
		// 沿用開票的 13 次會讓一則事件在表上躺兩天才終結。
		MaxAttempts: 8,
		BaseBackoff: 5 * time.Second,
		MaxBackoff:  10 * time.Minute,
		FetchBatch:  100,
		ClaimLease:  outboxLeaseFor(d.Cfg),
	}, obParkInterval, d.Obs, d.Log)
}

// outboxLeaseFor 由 publisher confirm timeout 推導 INV.ob 的租約。
//
// 抽成具名函式而不是內聯：這條推導是一個**不變式**（租約必須大於 confirm timeout，
// 否則持有者還在等 broker ack 時就被搶走 ⇒ 同一則事件發兩次），而不變式要有測試守著。
// 下限 30s 是 §4.3.3 第 3 組的定值。
func outboxLeaseFor(cfg *config.Config) time.Duration {
	confirm := time.Duration(cfg.MQ.PublishTimeoutSecond) * time.Second
	lease := confirm*2 + 10*time.Second
	if lease < 30*time.Second {
		return 30 * time.Second
	}
	return lease
}

type deadlineWorkerDeps struct {
	fx.In

	Inbox domain.InboxRepository
	UC    InboxOrderPaidReprocessor
	// Refund 是退款事件的重跑入口（#3073）。與 UC 分成兩個欄位而不是一個介面：
	// 兩者的 payload 形狀完全不同，分派寫反不會編譯錯，只會靜默拿到零值。
	Refund InboxRefundReprocessor
	// Overdue 是 K05（申訴逾 3 工作天未回覆）。它併進 INV.dl 的同一個 tick，
	// 不另開排程器 —— 理由見 deadline_worker.go 檔頭（同名 metrics 會 boot 期 panic）。
	Overdue OverdueComplaintSweeper
	// Statutory 是 §5.2.1 的逾期分類（已過法定上傳期限 → AWAITING_MANUAL ＋
	// DEADLINE_MISSED）。與 Overdue 同一個 tick、不同的軸。
	//
	// 🔴 **不用 `optional:"true"`**：那會在「忘了 provide」時靜默注入 nil，而 RunOnce
	// 對 nil 是安靜跳過 ⇒ 整段掃描零錯誤零 log 地不存在。2026-09-08 才因為同一個形狀
	// 踩過（AMEGO_PLATFORM_APP_KEY 缺 optional secret ⇒ 載具驗證整段跳過）。
	// 列成必要依賴，忘了接線就是 boot 期 fx 錯誤——那是看得見的失敗。
	Statutory StatutoryDeadlineSweeper
	Obs       *outbox.Metrics `name:"obs_deadline"`
	Log       *zap.Logger
}

// deadlineCfg 是 INV.dl 的執行參數。
//
// 抽成函式而不是在 newDeadlineWorker 內就地組：deadline_worker_test.go 的租約下界檢查
// （§三 第 5 點）必須量到**生產值**。在測試裡複製一份 outbox.Config 的話，日後有人調大
// dlClaimLease 或 dlFetchBatch，那支測試會拿著舊數字繼續綠。
func deadlineCfg() outbox.Config {
	return outbox.Config{
		MaxAttempts: dlMaxAttempts,
		BaseBackoff: dlBaseBackoff,
		MaxBackoff:  dlMaxBackoff,
		FetchBatch:  dlFetchBatch,
		ClaimLease:  dlClaimLease,
	}
}

// newDeadlineWorker 用上面那組常數組出 INV.dl。
func newDeadlineWorker(d deadlineWorkerDeps) *DeadlineWorker {
	return NewDeadlineWorker(d.Inbox, d.UC, d.Refund, deadlineCfg(), dlParkInterval, d.Overdue, d.Obs, d.Log).
		WithStatutorySweeper(d.Statutory)
}

// startSchedulers 把三支 worker 掛上 pkg/outbox.Scheduler 與 fx.Lifecycle。
//
// 🔴 走 Scheduler 而不是自己寫 `for { <-ticker.C }`：panic recovery 在 scheduler.go 的
// tick（逐 tick recover），裸迴圈少了它 ⇒ 一次 panic 讓 goroutine 就地死掉，之後所有
// 排程永不執行，而服務健康、/metrics 照常、告警要等候選窗口過完才響。
func startSchedulers(
	lc fx.Lifecycle,
	cfg *config.Config,
	issue *IssueWorker,
	ob *OutboxWorker,
	dl *DeadlineWorker,
	trk *trackWorker,
	hc *healthCheckWorker,
	rc *recheckWorker,
	gov *governanceWorker,
	pdf *pdfWorker,
	metrics *Metrics,
	log *zap.Logger,
) {
	l := orNopLogger(log)

	// M5：《硬上限依賴接入治理規範》三件套的「上限」那一半（啟動時設一次）。
	metrics.AmegoRateLimitQPS.Set(float64(cfg.Amego.RateLimitQPS))

	issueInterval := time.Duration(cfg.Schedule.IssuePollIntervalSeconds) * time.Second
	// M26：告警 ⑲ 的分母。只有耗時沒有 interval 就算不出飽和度。
	metrics.WorkerTickInterval.WithLabelValues(workerNameIssue).Set(issueInterval.Seconds())
	metrics.WorkerTickInterval.WithLabelValues(workerNameOutbox).Set(obDispatchInterval.Seconds())
	metrics.WorkerTickInterval.WithLabelValues(workerNameDeadline).Set(dlScanInterval.Seconds())
	// INV.trk 的 interval 是輪詢間隔（1h），不是掃描頻率——真正的頻率由
	// isTrackScanSlot 決定。M26 要的是「排程器多久 tick 一次」，所以這裡填 trackTick。
	metrics.WorkerTickInterval.WithLabelValues(workerNameTrack).Set(trackTick.Seconds())
	// INV.hc 的 interval 是「多久回頭看有沒有帳戶到期」（60s），**不是**巡檢週期
	// （後者是 usecase 寫回的 health_check_next_at = now+15min）。混淆會讓呼叫量變 15 倍。
	metrics.WorkerTickInterval.WithLabelValues(workerNameHealth).Set(hcInterval.Seconds())
	metrics.WorkerTickInterval.WithLabelValues(workerNameRecheck).Set(rcInterval.Seconds())
	// INV.gov 是每日一次（86400s）。M26 的分母要的是「排程週期」，填實際週期。
	metrics.WorkerTickInterval.WithLabelValues(workerNameGov).Set((24 * time.Hour).Seconds())
	metrics.WorkerTickInterval.WithLabelValues(workerNamePDF).Set(pdfInterval.Seconds())
	// M20 的三個 window 預先註冊成 0（見 governance_worker.go）。
	registerGovernanceMetrics()
	// 四支字軌 gauge 預先註冊成 0，讓序列在第一次掃描之前就存在（見 track_metrics.go）。
	registerTrackMetrics()

	schedulers := []*outbox.Scheduler{
		outbox.NewScheduler(issueInterval,
			timedTick(metrics, workerNameIssue, issue.RunOnce),
			panicHook(l, "INV.op"), errorHook(l, "INV.op")),
		outbox.NewScheduler(obDispatchInterval,
			timedTick(metrics, workerNameOutbox, ob.RunOnce),
			panicHook(l, "INV.ob"), errorHook(l, "INV.ob")),
		outbox.NewScheduler(obMonitorInterval,
			ob.RefreshMetrics,
			panicHook(l, "INV.ob.monitor"), errorHook(l, "INV.ob.monitor")),
		// INV.dl：inbox 兜底掃描（§7.2.5 第二列）。錯相位見 dlInitialDelay。
		outbox.NewScheduler(dlScanInterval,
			timedTick(metrics, workerNameDeadline, dl.RunOnce),
			panicHook(l, "INV.dl"), errorHook(l, "INV.dl"),
			outbox.WithInitialDelay(dlInitialDelay)),
		// INV.trk：每小時 tick、由 isTrackScanSlot 決定這一輪跑不跑（§7.5.3）。
		outbox.NewScheduler(trackTick,
			timedTick(metrics, workerNameTrack, trk.RunOnce),
			panicHook(l, "INV.trk"), errorHook(l, "INV.trk")),
		// INV.hc：每 60 秒看有沒有到期的 provider 帳戶要巡檢（§7.5.3）。
		outbox.NewScheduler(hcInterval,
			timedTick(metrics, workerNameHealth, hc.RunOnce),
			panicHook(l, "INV.hc"), errorHook(l, "INV.hc")),
		// INV.rc：結果未明對帳 ＋ 上傳狀態覆查（§7.5.3）。
		outbox.NewScheduler(rcInterval,
			timedTick(metrics, workerNameRecheck, rc.RunOnce),
			panicHook(l, "INV.rc"), errorHook(l, "INV.rc")),
		// INV.pdf：發票 PDF 抓檔（§7.5.3）。刻意不併進 INV.op——併進去下界逼近 110s、
		// 租約得放大到 4 分鐘，而開票是法定期限最緊的路徑。
		outbox.NewScheduler(pdfInterval,
			timedTick(metrics, workerNamePDF, pdf.RunOnce),
			panicHook(l, "INV.pdf"), errorHook(l, "INV.pdf")),
	}

	// INV.gov 走 DailyScheduler（每日 UTC 16:00 ＝ 台灣次日 00:00，§7.5.3）。
	// 型別與 outbox.Scheduler 不同，所以獨立掛 lifecycle。
	govSched := outbox.NewDailyScheduler(govDailyHourUTC, govDailyMinuteUTC, "INV.gov", gov.RunDaily, l)

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			// 🔴 同樣傳 context.Background()：OnStart 收到的那個是 fx 的 start-timeout
			// context，一返回就被取消 ⇒ 每日迴圈立刻結束、無錯誤、無 log。
			// 🔴 **啟動時補跑一次**，不等下一個 UTC 16:00。
			//
			// 理由不是「方便驗證」，是正確性：行事曆鏡像每日整段覆蓋，而 pod 在
			// UTC 16:00 之後才第一次啟動的話，鏡像要空到隔天——首次上線那一天尤其。
			// 而空鏡像 = 零涵蓋 = 那段期間每一筆申訴的 SLA 都退到 14 自然日的硬上限，
			// **完全靜默**。catchUp 要在 Start 之前完成。
			//
			// 放在自己的 goroutine：RunOnce 會打一次跨服務 RPC（8 s deadline），
			// 而 OnStart 阻塞超過 fx 的 start timeout 會讓整個服務起不來。
			//
			// 🔴 **帶重試**，理由不是「網路偶爾會抖」：**pod 剛啟動的那幾秒打不出去**。
			//
			// 2026-09-04 dev 實測的時間軸：shipping 的 gRPC 06:18:26 就 ready，
			// invoice 新 pod 06:21:0x 起、06:21:10 補跑 ⇒ 仍然 `connection refused`；
			// 而同一時間從 node 測 shipping 的 ClusterIP 與 pod IP 兩個埠**都通**。
			// ⇒ 不是對端沒起來，是**本 pod 自己的網路策略還沒就位**
			// （NetworkPolicy 對新 pod 的規則套用有幾秒延遲，期間被 REJECT ⇒ refused）。
			//
			// 補跑一次就放棄的話鏡像要空到隔天的 UTC 16:00。這個窗口每次部署、
			// 每次重啟都會出現 —— **是常態不是例外**。
			//
			// RunOnce 整體冪等（C4 是集合式 UPDATE、M20 唯讀、行事曆整段覆蓋）⇒ 重跑安全。
			go func() {
				ctx := context.Background()
				for attempt := 1; attempt <= govCatchUpAttempts; attempt++ {
					if err := gov.RunOnce(ctx); err == nil {
						return
					} else if attempt == govCatchUpAttempts {
						l.Error("invoice.gov.catchup_gave_up",
							zap.Int("attempts", attempt), zap.Error(err))
						return
					}
					select {
					case <-time.After(govCatchUpInterval):
					case <-ctx.Done():
						return
					}
				}
			}()
			govSched.Start(context.Background())
			for _, s := range schedulers {
				// 🔴 傳 context.Background()，不是 OnStart 收到的那一個：後者是 fx 自己的
				// start-timeout context，OnStart 一返回就被取消 ⇒ 迴圈立刻結束、無錯誤、無 log。
				s.Start(context.Background())
			}
			l.Info("invoice.worker.started",
				zap.Duration("issue_interval", issueInterval),
				zap.Duration("outbox_interval", obDispatchInterval),
				zap.Duration("deadline_interval", dlScanInterval))
			return nil
		},
		OnStop: func(context.Context) error {
			for _, s := range schedulers {
				s.Stop()
			}
			return nil
		},
	})
}

// timedTick 把一次 tick 包上 M25（invoice_worker_tick_duration_seconds）。
//
// 不論成敗都 observe：失敗的 tick 也佔用了時間，把它排除掉會讓飽和度在最需要它的時候
// 低報。
func timedTick(m *Metrics, worker string, run func(context.Context) error) func(context.Context) error {
	obs := m.WorkerTickDuration.WithLabelValues(worker)
	return func(ctx context.Context) error {
		start := time.Now()
		err := run(ctx)
		obs.Observe(time.Since(start).Seconds())
		return err
	}
}

func panicHook(l *zap.Logger, name string) func(context.Context, any) {
	return func(_ context.Context, recovered any) {
		l.Error("invoice.worker.tick_panic", zap.String("worker", name), zap.Any("recovered", recovered))
	}
}

func errorHook(l *zap.Logger, name string) func(context.Context, error) {
	return func(_ context.Context, err error) {
		l.Error("invoice.worker.tick_error", zap.String("worker", name), zap.Error(err))
	}
}

// newTrackObserver 建 INV.trk 的指標出口。
func newTrackObserver() trackObserver { return trackObserver{} }

// newTrackWorker 把掃描時點閘門接上 usecase。
func newTrackWorker(uc *usecase.Track, cfg *config.Config) *trackWorker {
	return &trackWorker{uc: uc, baseHourUTC: cfg.Schedule.TrackBalanceCheckHourUTC, now: time.Now}
}

// newHealthCheckWorker 把 INV.hc 的 usecase 接上排程器。
func newHealthCheckWorker(uc *usecase.HealthCheck) *healthCheckWorker {
	return &healthCheckWorker{uc: uc}
}

// newRecheckWorker 把 INV.rc 的 usecase 接上排程器。
func newRecheckWorker(uc *usecase.Recheck) *recheckWorker { return &recheckWorker{uc: uc} }

// newGovernanceObserver 建 INV.gov 的指標出口。
func newGovernanceObserver() governanceObserver { return governanceObserver{} }

// newGovernanceWorker 把 INV.gov 的 usecase 接上每日排程器。
func newGovernanceWorker(uc *usecase.Governance) *governanceWorker { return &governanceWorker{uc: uc} }

// newPDFWorker 把 INV.pdf 的 usecase 接上排程器。
func newPDFWorker(uc *usecase.PDF) *pdfWorker { return &pdfWorker{uc: uc} }
