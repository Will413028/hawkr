package usecase

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/notification-service/internal/domain"
)

// inboxItemToMap 的輸出會經 RabbitMQ 推給 gateway，再以 SSE notification.new
// 事件送到 web 前端。其中 id 與 userId 都是 64-bit TSID，遠超 JS
// Number.MAX_SAFE_INTEGER (2^53)。若以 JSON number 送出：
//   - 瀏覽器 JSON.parse 會精度丟失（gateway 以 map[string]interface{} unmarshal
//     時也會先變成 float64，精度在那步即丟失）；
//   - id 還會與 REST inbox（已用 strconv.FormatInt 送字串）型別/值不一致，使
//     web 前端 NotificationToastBridge 的 seenIdsRef / dedupeKey 對不上，導致
//     同一則通知 toast 兩次。
//
// 所有 64-bit id 必須序列化成字串。
func TestInboxItemToMap_BigIntIDsSerializedAsString(t *testing.T) {
	const (
		tsid   int64 = 7234567890123456789 // > 2^53，float64 無法精確表示
		userID int64 = 6123456789012345678 // 同為 64-bit TSID
	)

	item := &domain.InboxItem{
		ID:          tsid,
		UserID:      userID,
		UserType:    "buyer",
		EventType:   "order.paid",
		ReferenceID: "O748344852",
		Title:       "訂單成立通知",
		Body:        "訂單成立，感謝您的訂購，我們正快馬加鞭為您處理中",
		IsRead:      false,
		CreatedAt:   "2026-06-14T13:45:00Z",
	}

	raw, err := json.Marshal(inboxItemToMap(item))
	if err != nil {
		t.Fatalf("marshal inboxItemToMap: %v", err)
	}
	got := string(raw)

	// 兩個 64-bit id 都必須是精確的 JSON 字串。
	for field, value := range map[string]int64{"id": tsid, "userId": userID} {
		want := `"` + field + `":"` + strconv.FormatInt(value, 10) + `"`
		if !strings.Contains(got, want) {
			t.Fatalf("%s 必須序列化成字串以避免 JS 精度丟失/與 REST inbox 不一致。\n want substring: %s\n got: %s", field, want, got)
		}
	}

	// 模擬 gateway 端的 MQ round-trip：JSON number 在 unmarshal 成 interface{} 時
	// 會變 float64，64-bit TSID 精度即丟失；字串則完整保留。
	var roundTrip map[string]interface{}
	if err := json.Unmarshal(raw, &roundTrip); err != nil {
		t.Fatalf("unmarshal round-trip: %v", err)
	}
	for field, value := range map[string]int64{"id": tsid, "userId": userID} {
		s, ok := roundTrip[field].(string)
		if !ok || s != strconv.FormatInt(value, 10) {
			t.Fatalf("round-trip 後 %s 應為精確字串 %q，got %#v", field, strconv.FormatInt(value, 10), roundTrip[field])
		}
	}
}
