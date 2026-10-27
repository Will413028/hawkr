package mq

import (
	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ---------------------------------------------------------------------------
// M30 invoice_event_dropped_total（§8.2.3）
//
// 🔴 這支指標存在的理由是「一則入向事件連 DB 列都沒有建」這個失效形狀**在別處完全看不見**：
//   - invoice_inbox_events 上沒有痕跡（根本沒 INSERT）
//   - mq_consume_total{result="dlq"} 在**當時**也是 0（丟棄走的是 return nil ＝ ACK，不是 NACK）；
//     現在 unmarshal 失敗會進 DLQ，但另外兩個 reason（empty_order_no / empty_refund_ref）
//     仍然是「連列都沒有」的形狀，M30 對它們依舊是唯一訊號
//   - M8 invoice_paid_orders_without_operation 的基準是「inbox 裡有這一列」⇒ 同樣看不到
//
// 實例：`order.refund.finalized` 的 store_id 曾以 string 送出而本服務 payload 宣告 int64
// ⇒ json.Unmarshal 失敗 ⇒ 當時的慣例①把它 ACK 丟棄 ⇒ 40 則退款事件靜默消失、DB 零痕跡。
// （慣例①已於 2026-08-30 改成 return error → DLQ；M30 仍然必要——它守的是「這則事件在
//   invoice_inbox_events 沒有留下任何一列」，DLQ 路徑下那件事依然成立。）
//
// 走 package-level promauto 而不是 worker/metrics.go 那種 struct + fx provide：
//   - internal/worker 已經 import internal/mq，反向 import 會 cycle
//   - package-level var 在 process 內天然只註冊一次，沒有「NewMetrics 呼叫兩次即 panic」
//     那條需要靠 DI 紀律維持的不變式
// ---------------------------------------------------------------------------

// eventDroppedReasons 是 M30 的 reason 值域（封閉三值）。
//
// 三者的共同語意是「這則事件沒有在 invoice_inbox_events 留下任何一列」，不是「處理失敗」
// ——後者留得下痕跡（status/last_error），前者留不下。
var eventDroppedReasons = []string{
	"unmarshal_failed", // payload 解不開（現在走 DLQ，不再 ACK 丟棄）
	"empty_order_no",   // order.paid／order.cancelled 缺 order_no
	"empty_refund_ref", // order.refund.finalized 缺 refund_ref（冪等鍵缺失）
	// order.cancelled 缺 buyer_id。它單獨一個 reason 而不是併進 empty_order_no：
	// buyer_id 是讀路徑的授權述詞（Tier A），它為零代表 producer 的 enrich 出事，
	// 與「payload 根本沒有訂單號」是兩種完全不同的上游故障，混在同一個 label 下
	// 會讓告警指不出該去看哪一段 code。
	"empty_buyer_id",
}

// eventDroppedTypes 是 M30 的 event_type 值域（本服務訂閱的三條 queue）。
//
// 🔴 新增訂閱時必須同步加進來，否則 observeEventDropped 會**靜默忽略**該型別的
// 觀測（它對值域外的組合直接 return，刻意不造新序列）——那正是「看不到就放行」
// 的形狀。TestEventDroppedTypesCoverEverySubscription 拿 Subscriptions() 對帳。
var eventDroppedTypes = []string{
	string(pkgmq.EventTypeOrderPaid),
	string(pkgmq.EventTypeOrderRefundFinalized),
	string(pkgmq.EventTypeOrderCancelled),
}

// eventDropped 是 M30 本體。
var eventDropped = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "invoice_event_dropped_total",
	Help: "Inbound events that produced no invoice_inbox_events row at all (58 M30)",
}, []string{"event_type", "reason"})

// init 預先展開全部 label 組合。
//
// counter 在第一次觀測之前不會出現在 /metrics，而 Prometheus 告警的 selector 對**不存在
// 的序列**永遠不會 fire ⇒ 告警 ㉓ InvoiceEventDropped 會是永久靜默的擺設。
// 驗收判準是「序列存在」，不是「值為 0」——值為 0 才是正確的初始狀態。
func init() {
	for _, t := range eventDroppedTypes {
		for _, r := range eventDroppedReasons {
			eventDropped.WithLabelValues(t, r).Add(0)
		}
	}
}

// observeEventDropped 記一次 M30。值域外的組合直接忽略，不造新序列。
func observeEventDropped(eventType, reason string) {
	if !containsMQString(eventDroppedTypes, eventType) || !containsMQString(eventDroppedReasons, reason) {
		return
	}
	eventDropped.WithLabelValues(eventType, reason).Inc()
}

func containsMQString(vals []string, v string) bool {
	for _, x := range vals {
		if x == v {
			return true
		}
	}
	return false
}
