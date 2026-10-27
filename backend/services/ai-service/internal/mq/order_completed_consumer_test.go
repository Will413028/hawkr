package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/identity"
	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
)

// stubOrderCompletedRepo satisfies only the narrow orderCompletedRepo
// interface the handler actually depends on (see order_completed_consumer.go).
type stubOrderCompletedRepo struct {
	UpsertOrderCompletedFn func(ctx context.Context, storeID identity.StoreID, period, eventID string) error
}

func (s *stubOrderCompletedRepo) UpsertOrderCompleted(ctx context.Context, storeID identity.StoreID, period, eventID string) error {
	return s.UpsertOrderCompletedFn(ctx, storeID, period, eventID)
}

func mustParseTime(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, s)
	require.NoError(t, err)
	return ts
}

func TestOrderCompletedHandler_ParsesStoreIDAndDerivesPeriodFromCompletedAt(t *testing.T) {
	var gotStoreID identity.StoreID
	var gotPeriod, gotEventID string
	repo := &stubOrderCompletedRepo{
		UpsertOrderCompletedFn: func(ctx context.Context, storeID identity.StoreID, period, eventID string) error {
			gotStoreID, gotPeriod, gotEventID = storeID, period, eventID
			return nil
		},
	}
	handler := orderCompletedHandler(repo)

	// Timestamp 刻意設成與 CompletedAt 不同的月份：期間必須來自 completed_at。
	// （Timestamp 在真實流量下根本不存在於 wire 上，見下面的 literal-JSON 測試。）
	event := pkgmq.OrderCompletedEvent{
		BaseEvent: pkgmq.BaseEvent{EventID: "evt-123", Timestamp: mustParseTime(t, "2026-01-15T10:00:00Z")},
		OrderNo:   "O123", StoreID: "700001",
		CompletedAt: "2026-08-29T10:00:00.123",
	}
	body, _ := json.Marshal(event)

	err := handler(context.Background(), pkgmq.RoutingKeyOrderCompleted, body)
	require.NoError(t, err)
	require.Equal(t, identity.StoreID(700001), gotStoreID)
	require.Equal(t, "2026-08", gotPeriod,
		"期間必須取自 completed_at（2026-08），不是 BaseEvent.Timestamp（2026-01）")
	require.Equal(t, "evt-123", gotEventID)
}

func TestOrderCompletedHandler_MissingStoreID_ReturnsErrorForRetryNotSilentDrop(t *testing.T) {
	repo := &stubOrderCompletedRepo{}
	handler := orderCompletedHandler(repo)

	event := pkgmq.OrderCompletedEvent{BaseEvent: pkgmq.BaseEvent{EventID: "evt-456"}, OrderNo: "O456", StoreID: ""}
	body, _ := json.Marshal(event)

	err := handler(context.Background(), pkgmq.RoutingKeyOrderCompleted, body)
	require.Error(t, err, "an order.completed event with no store_id must fail loud (fail-closed on parse), not silently no-op")
}

// javaWireBody 產生 Java producer 真正送上 wire 的 body：直接序列化 record
// io.github.will413028.hawkr.order.mq.dto.OrderCompletedPayload（@JsonNaming(SnakeCaseStrategy)）
// 的 9 個欄位，**沒有 timestamp**，也沒有任何 BaseEvent 信封欄位。
//
// ⚠️ 這裡必須是手寫的字面 JSON，不能用 json.Marshal(pkgmq.OrderCompletedEvent{...})
// 產生。Go struct marshal 再 unmarshal 回來永遠對得起來，不管欄位名稱跟 producer
// 差多遠——原本的 struct-based 測試就是這樣讓「期間取自 wire 上不存在的
// timestamp」這個 bug 全綠通過的。只有比對真實 producer 的字面 payload 才擋得住
// 這一類欄位契約漂移。
func javaWireBody(completedAt string) []byte {
	return []byte(fmt.Sprintf(`{
  "event_id": "0f8b6c1e-2a4d-4f9b-9a1e-7c3d5e6f8a90",
  "order_no": "SO20260829000123",
  "buyer_id": "500042",
  "notify_buyer_account_id": "900042",
  "store_id": "700001",
  "notify_seller_account_id": "900777",
  "total_actual_amount": "1250",
  "confirm_type": "AUTO",
  "completed_at": %q
}`, completedAt))
}

func TestOrderCompletedHandler_DerivesPeriodFromLiteralJavaWireJSON(t *testing.T) {
	// Java 端是 DateTimeUtils.nowUtc().toString()，nowUtc() 回傳 LocalDateTime，
	// 所以三種小數秒形狀都是真實可能出現的值，而且一律沒有 Z／時區偏移。
	// time.Parse(time.RFC3339, …) 會對這三個全部失敗。
	cases := []struct {
		name        string
		completedAt string
	}{
		{"nanos", "2026-08-29T10:12:34.567"},
		{"single fractional digit (Java 省略尾端 0)", "2026-08-29T10:12:34.5"},
		{"exactly on the second (完全沒有小數點)", "2026-08-29T10:12:34"},
		{"full nanosecond precision", "2026-08-29T10:12:34.123456789"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotStoreID identity.StoreID
			var gotPeriod, gotEventID string
			called := 0
			repo := &stubOrderCompletedRepo{
				UpsertOrderCompletedFn: func(ctx context.Context, storeID identity.StoreID, period, eventID string) error {
					called++
					gotStoreID, gotPeriod, gotEventID = storeID, period, eventID
					return nil
				},
			}
			handler := orderCompletedHandler(repo)

			err := handler(context.Background(), pkgmq.RoutingKeyOrderCompleted, javaWireBody(tc.completedAt))

			require.NoError(t, err)
			require.Equal(t, 1, called)
			require.Equal(t, identity.StoreID(700001), gotStoreID)
			require.Equal(t, "0f8b6c1e-2a4d-4f9b-9a1e-7c3d5e6f8a90", gotEventID)
			require.Equal(t, "2026-08", gotPeriod,
				"期間必須由 wire 上真實存在的 completed_at 推導；"+
					"若退回 BaseEvent.Timestamp，這裡會拿到零值月份 \"0001-01\"，"+
					"count_period 永遠對不上、月度歸零永不觸發")
			require.NotEqual(t, "0001-01", gotPeriod,
				"\"0001-01\" 代表期間來自 wire 上不存在的 timestamp 欄位（C1 迴歸）")
		})
	}
}

func TestOrderCompletedHandler_BadCompletedAt_ReturnsErrorNeverDefaultsToZeroPeriod(t *testing.T) {
	cases := []struct {
		name        string
		completedAt string
	}{
		{"empty", ""},
		{"date only, no time", "2026-08-29"},
		{"space instead of T separator", "2026-08-29 10:12:34"},
		{"not a timestamp at all", "not-a-timestamp"},
		{"two-digit year", "26-08-29T10:12:34"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &stubOrderCompletedRepo{
				UpsertOrderCompletedFn: func(context.Context, identity.StoreID, string, string) error {
					t.Fatal("repo must not be called when completed_at is unusable — " +
						"寫入一個推導不出的期間比 retry/DLQ 更糟，會靜默污染 count_period")
					return nil
				},
			}
			handler := orderCompletedHandler(repo)

			err := handler(context.Background(), pkgmq.RoutingKeyOrderCompleted, javaWireBody(tc.completedAt))
			require.Error(t, err,
				"completed_at 缺失或畸形必須 fail loud 交給 ResilientConsumer retry/DLQ，不得退回預設期間")
		})
	}
}
