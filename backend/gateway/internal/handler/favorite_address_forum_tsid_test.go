package handler

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

// ForumRecord 是 gateway 賣家論壇 REST 端點的回應 DTO。其 id 類欄位是
// 64-bit TSID，超過 JS Number.MAX_SAFE_INTEGER (2^53-1)。先前以
// JSON number 送出 → web JSON.parse 精度丟失、比較/路由失準。本測試鎖住這些 TSID id
// 一律序列化成 JSON 字串、且 > 2^53 不丟精度；timestamp/金額/count 維持 number。
const tsidAFF int64 = 7234567890123456789 // > 2^53

func assertStringIDs(t *testing.T, name string, raw []byte, keys []string) {
	t.Helper()
	got := string(raw)
	want := strconv.FormatInt(tsidAFF, 10)
	for _, key := range keys {
		fragment := `"` + key + `":"` + want + `"`
		if !strings.Contains(got, fragment) {
			t.Fatalf("%s.%s 必須序列化成 JSON 字串。\n want substring: %s\n got: %s", name, key, fragment, got)
		}
	}
}

func TestForumRecord_IDsSerializedAsString(t *testing.T) {
	tsidStr := strconv.FormatInt(tsidAFF, 10)
	raw, err := json.Marshal(ForumRecord{
		ID:        tsidStr,
		SellerID:  tsidStr,
		ParentID:  tsidStr,
		ReplyToID: tsidStr,
		CreatedAt: 1_750_000_000_000,
	})
	if err != nil {
		t.Fatalf("marshal ForumRecord: %v", err)
	}
	assertStringIDs(t, "ForumRecord", raw, []string{"id", "sellerId", "parentId", "replyToId"})
	if !strings.Contains(string(raw), `"createdAt":1750000000000`) {
		t.Fatalf("created_at 應維持 number，got: %s", raw)
	}
}
