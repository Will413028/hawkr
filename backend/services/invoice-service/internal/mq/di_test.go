package mq

import (
	"testing"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/invoice-service/internal/config"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// TestModule_GraphResolves 驗 Layer 3.5 的 fx 圖解得開。
//
// fx 的接線錯誤只有**開機時**才會發現，而那時已經在叢集裡了。編譯期的
// `var _ IssuancePreparer = (*usecase.Issuance)(nil)` 擋得住「沒實作介面」，
// 擋不住「圖裡少一個型別」。
func TestModule_GraphResolves(t *testing.T) {
	cfg := &config.Config{}
	if err := fx.ValidateApp(
		fx.NopLogger,
		fx.Supply(cfg, zap.NewNop()),
		fx.Provide(
			func() domain.InboxRepository { return nil },
			func() domain.DocumentRepository { return nil },
			func() *usecase.Issuance { return nil },
			// G01 usecase（#3073）：AdjustmentApplier 的唯一依賴。
			func() *usecase.RefundAdjustment { return nil },
			// ORD／PAY 的 provider 住在 internal/client（見 di.go 的註解：反向 import
			// 會造成 cycle）⇒ 本層的圖測試必須自己供這兩個型別。
			// ⚠️ 這正是 `internal/app/app_test.go` 的整圖 ValidateApp 存在的理由：
			// 這裡供得出來，不代表真實的 client.Module 供得出來。
			func() OrderFactsClient { return UnavailableOrderFactsClient{} },
			func() PaymentFactsClient { return UnavailablePaymentFactsClient{} },
			// C3（#2904）多要兩個型別：TxManager（它是唯一一個把去重列與業務寫入
			// 放進同一交易的 consumer）與 CancelledOrderRepository。
			// 真實的 provider 分別住在 internal/config（Layer 0）與
			// internal/repository/postgres（Layer 2）。
			func() database.TxManager { return nil },
			func() domain.CancelledOrderRepository { return nil },
		),
		Module,
		fx.Invoke(func(*OrderPaidConsumer, *RefundFinalizedConsumer, *OrderCancelledConsumer, *EventPublisher) {}),
	); err != nil {
		t.Fatalf("fx graph 解不開：%v", err)
	}
}

// TestAdjustmentApplierIsWired 是前一版 TestAdjustmentApplierIsStillUnimplemented 的
// 繼任者（#3073）。前一版釘的是「§7.2.2 還沒落地」這個**前提**，並在前提改變的當下
// 轉紅、把「落地時要順帶做的四件事」印出來。那四件事本波已全部完成：
//
//	① INV.dl 的 FetchPendingDue 現在同時掃 ORDER_REFUND_FINALIZED
//	   —— 走 FetchDueActionableByType（**排除 DEFERRED**），見下面 ③
//	② TestDeadlineWorker_OnlyScansOrderPaid 已改成 TestDeadlineWorker_ScansBothInboxTypes
//	③ 「先補租約／CAS 再打開掃描」這個前置條件的處置與原本設想的不同，寫在下面
//	④ refund_finalized_consumer.go 與 deadline_worker.go 的紅字註解已一起更新
//
// 🔴 ③ 的實際處置：**沒有補租約，因為唯一鍵比租約更強**。
// 原本的擔心是「反向認領 ＋ 兜底掃描同時動同一列 DEFERRED ⇒ 第二張折讓單」。
// 本波用兩道機制取代租約：
//
//	(a) 兜底掃描根本不碰 DEFERRED（FetchDueActionableInboxEventsByType 的述詞是
//	    `status IN ('RECEIVED','FAILED')`）⇒ 兩條路徑的候選集合不相交
//	(b) 就算相交，`uq_invadj_allowance_refund`／`uq_invadj_void_once` 這兩個
//	    partial unique 會讓第二次 Insert 走 ON CONFLICT DO NOTHING ⇒ rows=0 ⇒
//	    RefundAdjustment.Apply 讀到 inserted=false 就 return nil
//
// (b) 比租約強在哪：租約會過期（持有者崩潰、時鐘漂移、GC 停頓），唯一鍵不會。
// 折讓單是法定憑證、開錯不可回收，所以這裡要的是**不可能**而不是**很少發生**。
//
// 本測試現在守的是「applier 真的接上了」——退化回 Unimplemented 就當場轉紅。
func TestAdjustmentApplierIsWired(t *testing.T) {
	got := newAdjustmentApplier(&usecase.RefundAdjustment{})
	if _, unwired := got.(UnimplementedAdjustmentApplier); unwired {
		t.Fatalf(`AdjustmentApplier 退回 UnimplementedAdjustmentApplier 了。

那個狀態下每一則退款事件都會落 inbox FAILED 而**不進 DLQ**（pkg/mq 的 retry 會被
inbox 去重擋掉 ⇒ retry counter 停在 1、mq_consume_total{result="dlq"} 恆為 0），
也就是說：折讓永遠不會開，而 MQ 那一側完全看不見。`)
	}
	if _, ok := got.(refundAdjustmentApplier); !ok {
		t.Fatalf("applier 型別 = %T，want refundAdjustmentApplier —— "+
			"換了實作要連同本測試一起想清楚：它守的是「退款有執行者」這件事", got)
	}
}
