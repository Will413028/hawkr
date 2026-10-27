package handler

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

// commentData 是 gateway 留言 REST 端點（GET /public/posts/{id}/comments、
// GET /app/posts/comments/{id}/replies）的回應 DTO。留言主鍵是 64-bit TSID，超過 JS
// Number.MAX_SAFE_INTEGER (2^53-1)。先前 int64 id 以 JSON number 送出 → web JSON.parse
// 精度丟失，且與 SSE comment.created（CommentSnapshot 已字串）型別不一致使去重失效。
//
// 本測試鎖住 commentData 的所有 TSID id 一律序列化成 JSON 字串、且 > 2^53 不丟精度；
// created_at 是毫秒 timestamp（< 2^53）維持 number。
func TestCommentData_IDsSerializedAsString(t *testing.T) {
	const tsid int64 = 7234567890123456789 // > 2^53
	tsidStr := strconv.FormatInt(tsid, 10)

	c := commentData{
		ID:        tsidStr,
		PostID:    tsidStr,
		ParentID:  tsidStr,
		ReplyToID: tsidStr,
		CreatedAt: 1_750_000_000_000, // 毫秒 timestamp，< 2^53，維持 number
	}

	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal commentData: %v", err)
	}
	got := string(raw)

	want := strconv.FormatInt(tsid, 10)
	for _, key := range []string{"id", "postId", "parentId", "replyToId"} {
		fragment := `"` + key + `":"` + want + `"`
		if !strings.Contains(got, fragment) {
			t.Fatalf("%s 必須序列化成 JSON 字串。\n want substring: %s\n got: %s", key, fragment, got)
		}
	}

	// created_at 應維持 JSON number（非字串）。
	if !strings.Contains(got, `"createdAt":1750000000000`) {
		t.Fatalf("created_at 應維持 number，got: %s", got)
	}
}
