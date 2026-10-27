// Package publicationoutbox 排空 procurement_rfq_publication_outbox：把每一列
// 投遞命令發到 RabbitMQ，並把它對應的 invite 從 QUEUED 推進到 SENT（成功）或
// FAILED（重試耗盡）。#2450 Slice 2b。
//
// ⛔ 本 package 刻意**不是** internal/procurement/outbox 的複製。那一份是照
// 遷移前的 seller-service 手抄的，沒有 CleanupDone、沒有 backlog 指標、也沒有
// per-outbox 指標（它只 import 了 pkg/outbox.Scheduler，claim/backoff/terminal
// 三段邏輯是第二份手寫實作）。本 package 一律走 pkg/outbox（ADR D2），形狀照
// services/buyer-service/internal/{domain,worker}/buyer_deleted_outbox*.go；
// 那一份 legacy 依 ADR D3 不遷移、隨 #2543 退場。
package publicationoutbox

import (
	"encoding/json"
	"time"
)

// Row 是一列 procurement_rfq_publication_outbox，翻成 worker 看得懂的原生型別
// （gen 的 pgtype.Timestamptz／*string 已在 repository.go 的 toRow 收斂掉）。
type Row struct {
	ID            int64
	InviteID      int64
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
