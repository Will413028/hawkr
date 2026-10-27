package worker

import (
	"testing"

	"github.com/Will413028/hawkr/invoice-service/internal/client/seller"
	"github.com/Will413028/hawkr/invoice-service/internal/config"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	invoicemq "github.com/Will413028/hawkr/invoice-service/internal/mq"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// TestModule_GraphResolves 驗 Layer 4.5 的 fx 圖解得開。
//
// fx 的接線錯誤只有**開機時**才會發現，而那時已經在叢集裡了。這一支特別重要的地方是
// 三個具名 *outbox.Metrics（`obs_issue` / `obs_outbox` / `obs_deadline`）：少掛一個 name
// tag 的症狀是「重複提供 *outbox.Metrics」而不是「少了一個」，錯誤訊息不會告訴你是哪幾個。
//
// ⚠️ ValidateApp **不呼叫任何建構函式** ⇒ 它驗的是圖的形狀，不是 promauto 的重複註冊。
// 後者由 TestMain 的單次 NewMetrics 保證（同名第二次註冊會 panic）。
func TestModule_GraphResolves(t *testing.T) {
	cfg := &config.Config{}
	cfg.Schedule.IssueMaxRetry = 13
	cfg.Schedule.BackoffBaseSeconds = 30
	cfg.Schedule.BackoffMaxSeconds = 1800
	cfg.Schedule.IssueBatchSize = 50
	cfg.Schedule.IssueLeaseSeconds = 120
	cfg.Schedule.IssuePollIntervalSeconds = 10
	cfg.MQ.PublishTimeoutSecond = 10

	if err := fx.ValidateApp(
		fx.NopLogger,
		fx.Supply(cfg, zap.NewNop()),
		fx.Provide(
			func() domain.OperationRepository { return nil },
			func() domain.DocumentRepository { return nil },
			func() domain.OutboxRepository { return nil },
			func() *usecase.Issuance { return nil },
			func() *usecase.Voiding { return nil },
			func() *invoicemq.EventPublisher { return nil },
			// SEL peer client 由 internal/client.Module 提供；本測試只驗 worker 的圖，
			// 供一個 nil 讓型別對得上即可（ValidateApp 不呼叫建構函式）。
			func() *seller.Client { return nil },
			// INV.dl 的兩個跨層依賴：inbox repository（Layer 2）與 order.paid handler
			// （Layer 3.5，兜底掃描要重跑的就是它的步驟 5–9）。
			func() domain.InboxRepository { return nil },
			func() *invoicemq.OrderPaidConsumer { return nil },
			// 退款重跑入口（#3073）：INV.dl 的第二條分派。
			func() *invoicemq.RefundFinalizedConsumer { return nil },
			// INV.trk 的 usecase（Layer 4）。與上面幾支同理：ValidateApp 不呼叫建構函式，
			// 供一個 nil 讓型別對得上即可。
			func() *usecase.Track { return nil },
			func() *usecase.HealthCheck { return nil },
			func() *usecase.Recheck { return nil },
			func() *usecase.Governance { return nil },
			func() *usecase.PDF { return nil },
			// K05 的 sweeper 綁到它（#3055 的橫向稽查）。
			func() *usecase.Complaint { return nil },
			// 折讓軸（#3073）：IssueWorker 的第三條分派。
			func() *usecase.Allowance { return nil },
		),
		Module,
		// 🔴 每一支 worker 都要列出來：fx.ValidateApp 只驗**被要求到的**那一段子圖，
		// 少列一支等於那支的接線完全沒被驗（DeadlineWorker 就是這樣才需要補進來，
		// trackWorker 於 2026-08-31 補上時同理）。
		fx.Invoke(func(*IssueWorker, *OutboxWorker, *DeadlineWorker, *trackWorker, *healthCheckWorker, *recheckWorker, *governanceWorker) {
		}),
	); err != nil {
		t.Fatalf("fx graph 解不開：%v", err)
	}
}

// TestOutboxLeaseExceedsPublisherConfirmTimeout 守 §4.3.3 第 3 組的租約下界。
//
// 下界 ＝ publisher confirm timeout ＋ stamp ⇒ 租約若小於 confirm timeout，持有者還在
// 等 broker ack 的時候就被另一個副本合法搶走 ⇒ **同一則事件被發兩次**。
// newOutboxWorker 由 INVOICE_MQ_PUBLISH_TIMEOUT_SECONDS 推導租約，就是為了讓這兩個
// 數字不可能各自漂移；這一支把那條推導釘住。
func TestOutboxLeaseExceedsPublisherConfirmTimeout(t *testing.T) {
	for _, confirmSeconds := range []int{1, 5, 10, 30, 60} {
		cfg := &config.Config{}
		cfg.MQ.PublishTimeoutSecond = confirmSeconds
		lease := outboxLeaseFor(cfg)
		confirm := float64(confirmSeconds)
		if lease.Seconds() <= confirm {
			t.Errorf("confirm=%ds 時租約 = %v，必須大於 confirm timeout", confirmSeconds, lease)
		}
		if lease.Seconds() < 30 {
			t.Errorf("confirm=%ds 時租約 = %v，下限 30s（§4.3.3 第 3 組）", confirmSeconds, lease)
		}
	}
}
