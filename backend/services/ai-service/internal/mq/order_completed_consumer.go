package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"

	"github.com/Will413028/hawkr/go-pkg/identity"
	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
)

// completedAtShape 只驗「到秒為止的 ISO-8601 本地時間」形狀，刻意不含結尾錨點：
// completed_at 由 Java 的 LocalDateTime.toString() 產生，尾端小數秒位數浮動
// （"…:34.567"／"…:34.5"／整秒時完全沒有小數點），而且**沒有 Z 或時區偏移**。
//
// 因此這裡不用 time.Parse：time.RFC3339 會直接因缺少偏移量而失敗，換成固定
// layout 也擋不住浮動的小數位數。期間只需要前 7 碼（YYYY-MM），驗完形狀直接
// 取前綴，既避開小數秒解析陷阱，又能對真正畸形的輸入 fail loud。
// 時鐘本身已是 UTC（Java 端 LocalDateTime.now(ZoneOffset.UTC)），無需轉換。
var completedAtShape = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}`)

// periodFromCompletedAt 由事件的 completed_at 取出 seller_ranking_facts 的
// count_period（YYYY-MM）。空值或形狀不符一律回錯，讓 handler fail loud —— 絕不
// 退回零值／當月預設，否則會寫進一個永遠對不上的期間而讓月度歸零靜默失效。
func periodFromCompletedAt(completedAt string) (string, error) {
	if completedAt == "" {
		return "", fmt.Errorf("missing completed_at")
	}
	if !completedAtShape.MatchString(completedAt) {
		return "", fmt.Errorf("malformed completed_at %q: want YYYY-MM-DDTHH:MM:SS[.fffffffff]", completedAt)
	}
	return completedAt[:7], nil
}

// orderCompletedRepo is the narrow, handler-owned port onto
// repository.RankingFactsRepository (Task 11) — only the one method this
// handler calls, following the same small consumer-owned interface pattern
// usecase.SellerClient already demonstrates elsewhere in this codebase.
type orderCompletedRepo interface {
	UpsertOrderCompleted(ctx context.Context, storeID identity.StoreID, period, eventID string) error
}

// orderCompletedHandler returns a pkgmq.HandlerFunc that maintains
// seller_ranking_facts.monthly_completed_orders from order.completed events.
// A parse failure (missing store_id, missing/malformed completed_at, bad JSON)
// returns an error so ResilientConsumer retries/DLQs it — never silently drops
// it, unlike the abandoned SellerOrderUpdatedEvent approach this replaces
// (spec Corrections).
//
// 期間（count_period）取自 event.CompletedAt，不是 BaseEvent.Timestamp：Java
// producer 的 wire payload 沒有 timestamp 這個 key，用它推導出來的期間在真實流量
// 下恆為 "0001-01"，UpsertSellerRankingOrderCount 的
// `CASE WHEN count_period = $2` 月度歸零就永遠不會觸發，計數只增不減，賣家一旦
// 撞到上限就永久從 AI 候選消失。詳見 pkgmq.OrderCompletedEvent.CompletedAt。
func orderCompletedHandler(repo orderCompletedRepo) pkgmq.HandlerFunc {
	return func(ctx context.Context, routingKey string, body []byte) error {
		var event pkgmq.OrderCompletedEvent
		if err := json.Unmarshal(body, &event); err != nil {
			return fmt.Errorf("unmarshal order.completed: %w", err)
		}
		if event.StoreID == "" {
			return fmt.Errorf("order.completed event %s: missing store_id", event.EventID)
		}
		storeIDInt, err := strconv.ParseInt(event.StoreID, 10, 64)
		if err != nil {
			return fmt.Errorf("order.completed event %s: invalid store_id %q: %w", event.EventID, event.StoreID, err)
		}
		period, err := periodFromCompletedAt(event.CompletedAt)
		if err != nil {
			return fmt.Errorf("order.completed event %s: %w", event.EventID, err)
		}
		return repo.UpsertOrderCompleted(ctx, identity.StoreID(storeIDInt), period, event.EventID)
	}
}
