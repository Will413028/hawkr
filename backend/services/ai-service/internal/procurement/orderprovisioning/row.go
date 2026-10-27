// Package orderprovisioning 排空 procurement_order_provisioning_outbox：把每一列
// 建單命令送進 order-service 的 CreateRfqOrder，並把對應的 buyer quote acceptance
// 從 CREATING_ORDER 推進到 ORDER_CREATED（成功）或 ORDER_FAILED（重試耗盡）。
// #2538 Wave B，D2 的最後一哩。
//
// 形狀照 publicationoutbox：同樣走 pkg/outbox（ADR D2），claim 用 lease CAS。
// ⛔ 不要照 internal/procurement/outbox——那一份是遷移前手抄的，claim/backoff/terminal
// 是第二份手寫實作，依 ADR D3 不遷移、隨 #2543 退場。
package orderprovisioning

import (
	"encoding/json"
	"time"
)

// Row 是一列 procurement_order_provisioning_outbox，翻成 worker 看得懂的原生型別
// （gen 的 pgtype.Timestamptz／*string 已在 repository.go 的 toRow 收斂掉）。
//
// ⚠️ 表上的 product_sku_id 刻意**不映射進來**：RFQ 訂單明細是 non-catalog line
// （ADR 2026-09-15），該欄自那時起是死欄，Wave C 才 DROP COLUMN。映射進來會讓
// 後續維護者以為它還有用途。
type Row struct {
	ID            int64
	AcceptanceID  int64
	EventType     string
	Payload       json.RawMessage
	Status        string
	AttemptCount  int
	NextAttemptAt time.Time
	LastError     string
	CreatedAt     time.Time
	CompletedAt   *time.Time
}

// OutboxID 與 OutboxAttemptCount 滿足 github.com/Will413028/hawkr/go-pkg/outbox.Row。
func (r *Row) OutboxID() int64         { return r.ID }
func (r *Row) OutboxAttemptCount() int { return r.AttemptCount }
