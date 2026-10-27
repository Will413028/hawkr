package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// 56 號代行營運的兩支 counter（§8.2.3 OBS8／OBS10）。
//
// 生產者必須是 worker 而不是 sql_exporter：sql_exporter 現行只連 account DB，而這兩件
// 事發生在 social DB；且「被取消／被逾期的數量」是事件計數，用 gauge 查現存列數表達
// 不出來，也分不出各自的來源。
//
// label 值域刻意封閉且小。store_id／admin_id／session_id 一類的值一律不得成為 label
// （基數無上界）——「是哪一家店的哪一則貼文」是稽核問題與 log 問題，不是 metric 問題。

// DelegationScheduledPostExpiredTotal 計「排定了卻始終沒發出去」的貼文。
//
// 兩個來源都必須累加：window_exceeded 隨每個 tick 的第三條收斂語句的 RowsAffected
// 累加，retry_exhausted 在重試達上限那一次轉態時累加 1。只記其中一條，另一條就變成
// 「貼文永遠不發布、賣家不知情、零告警」。
var DelegationScheduledPostExpiredTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "delegation_scheduled_post_expired_total",
	Help: "scheduled delegated posts that were never published (window exceeded or retries exhausted)",
}, []string{"reason"})

// DelegationScheduledPostCancelledTotal 計被取消的排程。
//
// 三處都要累加：revocation 在 RecordAuditIntent 被拒那一次，deletion／governance 各自
// 隨前兩條收斂語句的 RowsAffected（這正是那兩條刻意拆成兩句而不是一句 CASE 的理由）。
// 取消一律不通知賣家、不寫代行稽核，所以「取消至少對平台自己可見」完全靠這一支。
var DelegationScheduledPostCancelledTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "delegation_scheduled_post_cancelled_total",
	Help: "scheduled delegated posts cancelled before execution",
}, []string{"reason"})

// 兩支 counter 的 label 值域（封閉集合）。
const (
	ScheduleExpiredReasonWindowExceeded = "window_exceeded"
	ScheduleExpiredReasonRetryExhausted = "retry_exhausted"

	ScheduleCancelReasonRevocation = "revocation"
	ScheduleCancelReasonDeletion   = "deletion"
	ScheduleCancelReasonGovernance = "governance"
)
