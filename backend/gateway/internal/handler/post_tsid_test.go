package handler

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

// 貼文相關 REST DTO（PostListItemData / PostDetailData / MyPostListItemData /
// AdminPostListItemData / PostProductData）的 id / seller_id / post_id /
// store_id 皆為 64-bit TSID，超過 JS Number.MAX_SAFE_INTEGER (2^53-1)。
// 先前以 JSON number 送出 → web / admin 原生 JSON.parse 精度丟失（與 comment / report
// 同 bug class，全 repo TSID-as-number 稽核項 cluster ①）。
//
// 本測試鎖住上述 TSID id 一律序列化成 JSON 字串、> 2^53 不丟精度；created_at /
// hidden_at / latest_report_at 是毫秒 timestamp（< 2^53）及各 *_count 維持 number。
func TestPostDTOs_TSIDsSerializedAsString(t *testing.T) {
	const tsid int64 = 7234567890123456789 // > 2^53
	want := strconv.FormatInt(tsid, 10)

	assertStringID := func(t *testing.T, raw []byte, keys ...string) {
		t.Helper()
		got := string(raw)
		for _, key := range keys {
			fragment := `"` + key + `":"` + want + `"`
			if !strings.Contains(got, fragment) {
				t.Fatalf("%s 必須序列化成 JSON 字串。\n want substring: %s\n got: %s", key, fragment, got)
			}
		}
	}

	t.Run("PostListItemData", func(t *testing.T) {
		raw, err := json.Marshal(PostListItemData{ID: want, SellerID: want, CreatedAt: 1_750_000_000_000})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		assertStringID(t, raw, "id", "sellerId")
		if !strings.Contains(string(raw), `"createdAt":1750000000000`) {
			t.Fatalf("created_at 應維持 number，got: %s", raw)
		}
	})

	t.Run("PostDetailData", func(t *testing.T) {
		raw, err := json.Marshal(PostDetailData{ID: want, SellerID: want})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		assertStringID(t, raw, "id", "sellerId")
	})

	t.Run("MyPostListItemData", func(t *testing.T) {
		raw, err := json.Marshal(MyPostListItemData{ID: want, SellerID: want})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		assertStringID(t, raw, "id", "sellerId")
	})

	t.Run("AdminPostListItemData", func(t *testing.T) {
		raw, err := json.Marshal(AdminPostListItemData{PostID: want, StoreID: want, ReportCount: 3})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		assertStringID(t, raw, "postId", "storeId")
		if !strings.Contains(string(raw), `"reportCount":3`) {
			t.Fatalf("report_count 應維持 number，got: %s", raw)
		}
	})

	t.Run("PostProductData", func(t *testing.T) {
		raw, err := json.Marshal(PostProductData{ID: want})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		assertStringID(t, raw, "id")
	})
}
