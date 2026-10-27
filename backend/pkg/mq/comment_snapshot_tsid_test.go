package mq

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

// CommentSnapshot 由 social-service 產生、經 RabbitMQ 推給 gateway，再由 gateway 原樣
// 轉成 SSE comment.created / comment.replied 事件送到 web 前端。留言主鍵是 64-bit TSID，
// 遠超 JS Number.MAX_SAFE_INTEGER (2^53-1)。若 id 以 JSON number 送出：
//   - 瀏覽器 JSON.parse 會精度丟失（gateway 以 interface{} unmarshal 時也會先變 float64）；
//   - 與 REST commentData（已用 ,string 送字串）型別/值不一致，使 web 端 SSE 推送與 REST
//     列表的同一則留言去重對不上，導致重複渲染。
//
// 同通道的 comment.updated/deleted/like_changed 早已用 fmt.Sprintf 送字串；本測試鎖住
// comment.created/replied 的 snapshot id 也一律序列化成字串、且 TSID > 2^53 不丟精度。
func TestCommentSnapshot_IDsSerializedAsString(t *testing.T) {
	const tsid int64 = 7234567890123456789 // > 2^53，float64 無法精確表示

	snap := CommentSnapshot{
		CommentID: tsid,
		PostID:    tsid,
		UserID:    tsid,
		ParentID:  tsid,
		ReplyToID: tsid,
	}

	raw, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("marshal CommentSnapshot: %v", err)
	}
	got := string(raw)

	want := strconv.FormatInt(tsid, 10)
	for _, key := range []string{"commentId", "postId", "userId", "parentId", "replyToId"} {
		fragment := `"` + key + `":"` + want + `"`
		if !strings.Contains(got, fragment) {
			t.Fatalf("%s 必須序列化成 JSON 字串以避免 web 精度丟失/與 REST 不一致。\n want substring: %s\n got: %s", key, fragment, got)
		}
	}

	// 模擬 gateway 端的 MQ round-trip：JSON number 在 unmarshal 成 interface{} 時會變
	// float64，64-bit TSID 精度即丟失；字串則完整保留。
	var roundTrip map[string]interface{}
	if err := json.Unmarshal(raw, &roundTrip); err != nil {
		t.Fatalf("unmarshal round-trip: %v", err)
	}
	idStr, ok := roundTrip["commentId"].(string)
	if !ok || idStr != want {
		t.Fatalf("round-trip 後 commentId 應為精確字串 %q，got %#v", want, roundTrip["commentId"])
	}
}
