package usecase

import (
	"strconv"

	"github.com/Will413028/hawkr/ai-service/internal/agent"
)

// ProcurementAppName 是這個 procurement graph 在 ADK session.Service 裡的邏輯
// 隔離命名——重用既有 internal/agent/di.go 已經在跑的同一個 Postgres-backed
// session.Service（同 DSN），不同 AppName 讓事件表（session 的 events 表）
// 邏輯上分開，不需要新的 DB 連線或 migration（見設計書 §1.1）。
// 型別與值都來自 internal/agent.AppNameProcurement（agent.ADKAppName），不是
// 獨立字面值：避免這裡與 assistant 側各自維護一份分區鍵、悄悄漂移。
const ProcurementAppName agent.ADKAppName = agent.AppNameProcurement

// ProcurementSessionID 把 procurement 的 requestId（既有跨輪次穩定的 threadId）
// 映射成 ADK 的 SessionID。刻意不用 runId（OTel trace ID，
// 每個 HTTP request 各自不同）——CropGate 消歧的掛起與買家回答分屬兩個獨立 HTTP
// request，session 必須跨越兩者存活，只有 requestId 滿足這個條件。
func ProcurementSessionID(requestID int64) string {
	return strconv.FormatInt(requestID, 10)
}
