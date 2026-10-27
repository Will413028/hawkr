package domain

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

// domain.Message 經 json.Marshal 直接送 WebSocket frame 給 web/Flutter 端
// （chat / chat_ack / sync 的 chat / typing / presence 皆用同一 struct）。
// from/to/messageId/seqId 是 64-bit TSID，遠超 JS Number.MAX_SAFE_INTEGER (2^53-1)。
// 若以 JSON number 送出：
//   - web JSON.parse 精度丟失，去重 key `${conversationId}_${seqId}` 失準 → 訊息
//     重複或漏去重；
//   - 與 REST 聊天歷史（gRPC proto，id 已是字串）型別不一致。
//
// 本測試鎖住這四個 id 一律序列化成 JSON 字串、且 TSID > 2^53 round-trip 不丟精度。
func TestMessage_IDsSerializedAsString(t *testing.T) {
	const tsid int64 = 7234567890123456789 // > 2^53

	msg := Message{
		Type:      "chat",
		RoomID:    "room-1",
		From:      tsid,
		To:        tsid,
		MessageID: tsid,
		SeqID:     tsid,
		Timestamp: 1_750_000_000_000, // 毫秒 timestamp，< 2^53，維持 number
	}

	raw, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal Message: %v", err)
	}
	got := string(raw)

	want := strconv.FormatInt(tsid, 10)
	for _, key := range []string{"from", "to", "messageId", "seqId"} {
		fragment := `"` + key + `":"` + want + `"`
		if !strings.Contains(got, fragment) {
			t.Fatalf("%s 必須序列化成 JSON 字串以避免 web 精度丟失/與 REST 不一致。\n want substring: %s\n got: %s", key, fragment, got)
		}
	}

	// timestamp 應維持 JSON number（非字串）。
	if !strings.Contains(got, `"timestamp":1750000000000`) {
		t.Fatalf("timestamp 應維持 number，got: %s", got)
	}

	// 模擬 gateway/瀏覽器以 interface{} unmarshal：number 會變 float64 丟精度，字串完整保留。
	var roundTrip map[string]interface{}
	if err := json.Unmarshal(raw, &roundTrip); err != nil {
		t.Fatalf("unmarshal round-trip: %v", err)
	}
	if s, ok := roundTrip["seqId"].(string); !ok || s != want {
		t.Fatalf("round-trip 後 seqId 應為精確字串 %q，got %#v", want, roundTrip["seqId"])
	}
}

// 0 值的 id（如 typing frame 沒有 messageId/seqId）應被 omitempty 省略，不送 "0"。
func TestMessage_ZeroIDsOmitted(t *testing.T) {
	msg := Message{Type: "typing", From: 7234567890123456789, To: 8234567890123456789}
	raw, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), "messageId") || strings.Contains(string(raw), "seqId") {
		t.Fatalf("messageId/seqId 為 0 時應 omitempty 省略，got: %s", raw)
	}
}
