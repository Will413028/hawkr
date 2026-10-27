package handler

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	pb "github.com/Will413028/hawkr/go-pkg/proto/social"
)

// 賣家自己的貼文清單必須帶 publishStatus。少了它，賣家看不出哪幾則是代行者留下的
// 草稿或已下架貼文——而畫面不會報錯：前端在缺值時刻意什麼都不畫（把缺值讀成「已
// 發布」會是一個沒有根據的宣稱），所以漏掉這個欄位的畫面與正常畫面像素相同。
// 上游（social-service ListMyPosts）恆帶值，缺口只在 Gateway 這一層。
func TestMyPostListItem_CarriesPublishStatus(t *testing.T) {
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

	for _, tc := range []struct {
		name string
		code int
	}{
		{"draft", codemeta.PostPublishStatusDraft},
		{"published", codemeta.PostPublishStatusPublished},
		{"unpublished", codemeta.PostPublishStatusUnpublished},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := strconv.Itoa(tc.code)
			items := toMyPostListItems("https://cdn.test", []*pb.PostListItem{
				{Id: "1", PublishStatus: want},
			})
			got := marshalFirst(t, items[0])
			if got["publishStatus"] != want {
				t.Fatalf("publishStatus 應原樣透傳上游碼值 %q, got %v（整包：%v）", want, got["publishStatus"], got)
			}
		})
	}

	// 缺值與「已發布」在 wire 上必須分得開：前端把缺值讀成「這條路徑的上游沒給」
	// 而不畫任何標記，若這裡序列化成空字串或 "4402"，那個判斷就失效了。
	t.Run("upstream gave nothing", func(t *testing.T) {
		items := toMyPostListItems("https://cdn.test", []*pb.PostListItem{{Id: "1"}})
		got := marshalFirst(t, items[0])
		if _, present := got["publishStatus"]; present {
			t.Fatalf("上游沒給時不得吐出 publishStatus（前端靠缺席判斷「這條路徑沒有狀態」）, got %v", got)
		}
	})
}

// 排程尚未執行的草稿要把排程時間帶給店主；沒有排程時整個鍵不出現（缺席＝沒有排程）。
func TestMyPostListItem_CarriesScheduledPublishAt(t *testing.T) {
	items := toMyPostListItems("https://cdn.test", []*pb.PostListItem{
		{Id: "1", PublishStatus: "4401", ScheduledPublishAt: "2026-10-20T03:30:00Z"},
		{Id: "2", PublishStatus: "4401"},
	})
	raw, err := json.Marshal(items)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got []map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got[0]["scheduledPublishAt"] != "2026-10-20T03:30:00Z" {
		t.Errorf("scheduledPublishAt = %v, want 2026-10-20T03:30:00Z", got[0]["scheduledPublishAt"])
	}
	if _, present := got[1]["scheduledPublishAt"]; present {
		t.Errorf("沒有排程時不得輸出 scheduledPublishAt, got %v", got[1])
	}
}
