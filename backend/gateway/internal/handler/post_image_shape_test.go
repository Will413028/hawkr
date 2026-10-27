package handler

import (
	"encoding/json"
	"testing"

	pb "github.com/Will413028/hawkr/go-pkg/proto/social"
)

// 貼文列表（PostListItemData / MyPostListItemData）與詳情（PostDetailData）過去
// 各自用不同 shape 表示同一張貼文圖：列表回單數 imageKey/imageUrl，詳情回複數
// imageKeys/imageUrls 陣列（PR seller#118 排查 N+1 enrichment 時發現的根因）。
// 列表曾為了對齊詳情而「兩組都吐」——但兩組並存反而是 A0 的直接成因：前端編輯
// 貼文時讀單數欄回送，複數欄那份就被當成「使用者刪掉了」而清空 DB 圖片。
//
// 現在只有一組：imageKeys / imageUrls。本測試釘住單數欄真的消失（只斷言複數欄
// 存在的話，同時吐兩組也會綠）。
func TestPostListDTOs_PluralImageFieldsOnly(t *testing.T) {
	marshalFirst := func(t *testing.T, v any) map[string]any {
		t.Helper()
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		var got map[string]any
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		return got
	}
	assertNoSingular := func(t *testing.T, got map[string]any) {
		t.Helper()
		for _, stale := range []string{"imageKey", "imageUrl"} {
			if _, leaked := got[stale]; leaked {
				t.Errorf("列表不得再吐單數欄 %q（與複數欄並存正是 A0 的成因）; got %v", stale, got)
			}
		}
	}

	t.Run("toPostListItems", func(t *testing.T) {
		items := toPostListItems("https://cdn.test", []*pb.PostListItem{
			{Id: "1", ImageKey: "posts/abc.jpg"},
		})
		got := marshalFirst(t, items[0])
		assertNoSingular(t, got)
		keys, ok := got["imageKeys"].([]any)
		if !ok || len(keys) != 1 || keys[0] != "posts/abc.jpg" {
			t.Fatalf("imageKeys 應為單元素陣列, got: %v", got["imageKeys"])
		}
		urls, ok := got["imageUrls"].([]any)
		if !ok || len(urls) != 1 {
			t.Fatalf("imageUrls 應為單元素陣列, got: %v", got["imageUrls"])
		}
	})

	t.Run("toMyPostListItems", func(t *testing.T) {
		items := toMyPostListItems("https://cdn.test", []*pb.PostListItem{
			{Id: "1", ImageKey: "posts/abc.jpg"},
		})
		got := marshalFirst(t, items[0])
		assertNoSingular(t, got)
		keys, ok := got["imageKeys"].([]any)
		if !ok || len(keys) != 1 || keys[0] != "posts/abc.jpg" {
			t.Fatalf("imageKeys 應為單元素陣列, got: %v", got["imageKeys"])
		}
	})

	// 無圖時欄位仍在、且是 []——不是 null、也不是整個欄位消失。三種空值表示法
	// 讓前端得寫三次判斷；契約只給一種。
	t.Run("empty image key still emits empty arrays", func(t *testing.T) {
		items := toPostListItems("https://cdn.test", []*pb.PostListItem{{Id: "1"}})
		got := marshalFirst(t, items[0])
		assertNoSingular(t, got)
		for _, k := range []string{"imageKeys", "imageUrls"} {
			v, exists := got[k]
			if !exists {
				t.Fatalf("無圖時 %q 仍必須存在（欄位不得 omitempty）; got %v", k, got)
			}
			arr, ok := v.([]any)
			if !ok {
				t.Fatalf("無圖時 %q 必須是 []，不得為 null; got %#v", k, v)
			}
			if len(arr) != 0 {
				t.Fatalf("無圖時 %q 必須是空陣列; got %v", k, arr)
			}
		}
	})

	// 詳情端同形：AdminPostListItemData / PostDetailData 的空集合也是 []。
	t.Run("detail and admin list emit empty arrays too", func(t *testing.T) {
		detail := marshalFirst(t, toPostDetailData("https://cdn.test", &pb.PostDetailInfo{Id: "1"}))
		admin := marshalFirst(t, toAdminPostListItems("https://cdn.test",
			[]*pb.AdminPostListItem{{PostId: "1"}})[0])
		for name, got := range map[string]map[string]any{"PostDetailData": detail, "AdminPostListItemData": admin} {
			for _, k := range []string{"imageKeys", "imageUrls"} {
				arr, ok := got[k].([]any)
				if !ok || len(arr) != 0 {
					t.Errorf("%s.%s 空集合必須是 []; got %#v", name, k, got[k])
				}
			}
		}
	})
}

func TestSingleImageKeys(t *testing.T) {
	// 空字串回空陣列而非 nil：欄位已無 omitempty，nil 會序列化成 null。
	if got := singleImageKeys(""); got == nil || len(got) != 0 {
		t.Fatalf("空字串應回傳空陣列, got: %#v", got)
	}
	got := singleImageKeys("posts/abc.jpg")
	if len(got) != 1 || got[0] != "posts/abc.jpg" {
		t.Fatalf("應回傳單元素陣列, got: %v", got)
	}
}
