package registry

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

const (
	contentEvent = "0190a000-0000-7000-8000-0000000000c1"
	contentObj   = "0190a000-0000-7000-8000-0000000000c2"
	contentPost  = "0190a000-0000-7000-8000-0000000000c3"
)

// 內容側每個型別在每個語系的固定輸出（含深連結與聚合鍵）：改文案、路徑或前綴都會紅，要改就連預期一起改。
func TestGoldenContentRendering(t *testing.T) {
	const at = `"occurred_at":"2026-11-23T04:30:00Z"`
	posts := "/sellers/20260001/posts"
	cases := []struct {
		typ, params                      string
		zhTitle, zhBody, enTitle, enBody string
		link, agg                        string
	}{
		{"comment_created", `{"event_id":"` + contentEvent + `",` + at + `,"comment_id":"` + contentObj + `","post_id":"` + contentPost + `","store_id":7,"actor":{"side":"BUYER","id":9}}`,
			"收到新留言", "有人在你的貼文下留言。", "New comment", "Someone commented on your post.", posts, ""},
		{"comment_replied_buyer", `{"event_id":"` + contentEvent + `",` + at + `,"comment_id":"` + contentObj + `","post_id":"` + contentPost + `","store_id":7,"reply_to":{"side":"BUYER","id":9},"actor":{"side":"STORE","id":7}}`,
			"收到新回覆", "有人回覆了你的留言。", "New reply", "Someone replied to your comment.", posts, ""},
		{"comment_replied_store", `{"event_id":"` + contentEvent + `",` + at + `,"comment_id":"` + contentObj + `","post_id":"` + contentPost + `","store_id":7,"reply_to":{"side":"STORE","id":7},"actor":{"side":"BUYER","id":9}}`,
			"收到新回覆", "有人回覆了貴店的留言。", "New reply", "Someone replied to your store's comment.", posts, ""},
		{"comment_liked_buyer", `{"event_id":"` + contentEvent + `",` + at + `,"comment_id":"` + contentObj + `","post_id":"` + contentPost + `","store_id":7,"author":{"side":"BUYER","id":9},"actor":{"side":"STORE","id":7}}`,
			"留言收到讚", "有人對你的留言按讚。", "Your comment got a like", "Someone liked your comment.", posts, "comment_liked_buyer:comment_like:" + contentObj},
		{"comment_liked_store", `{"event_id":"` + contentEvent + `",` + at + `,"comment_id":"` + contentObj + `","post_id":"` + contentPost + `","store_id":7,"author":{"side":"STORE","id":7},"actor":{"side":"BUYER","id":9}}`,
			"留言收到讚", "有人對貴店的留言按讚。", "Your comment got a like", "Someone liked your store's comment.", posts, "comment_liked_store:comment_like:" + contentObj},
		{"post_liked", `{"event_id":"` + contentEvent + `",` + at + `,"post_id":"` + contentPost + `","store_id":7,"buyer_id":9}`,
			"貼文收到讚", "有人對你的貼文按讚。", "Your post got a like", "Someone liked your post.", "/sellers/20260001/posts", "post_liked:post_like:" + contentPost},
		{"store_followed", `{"event_id":"` + contentEvent + `",` + at + `,"store_id":7,"buyer_id":9}`,
			"新的追蹤者", "有買家開始追蹤你的店家。", "New follower", "A buyer started following your store.", "", "store_followed:follow:7"},
		{"review_created", `{"event_id":"` + contentEvent + `",` + at + `,"review_id":"` + contentObj + `","buyer_id":9,"store_id":7,"product_id":"` + contentObj + `"}`,
			"收到新評價", "有買家評價了你的商品。", "New review", "A buyer reviewed your product.", "", ""},
		{"post_published_item", `{"post_id":"` + contentPost + `","store_id":7,"buyer_id":9}`,
			"追蹤的店家發佈了新貼文", "你追蹤的店家剛發佈了一則貼文。", "A store you follow posted", "A store you follow just published a new post.", "/sellers/20260001/posts", ""},
		{"chat_message_buyer", `{"event_id":"` + contentEvent + `",` + at + `,"conversation_id":"` + contentObj + `","buyer_id":9,"store_id":7}`,
			"新訊息", "店家傳了一則訊息給你。", "New message", "A store sent you a message.", "/account/messages/7", ""},
		{"chat_message_store", `{"event_id":"` + contentEvent + `",` + at + `,"conversation_id":"` + contentObj + `","buyer_id":9,"store_id":7}`,
			"新訊息", "買家傳了一則訊息給你。", "New message", "A buyer sent you a message.", "/seller-center/messages/9", ""},
		{"product_restocked", `{"event_id":"` + contentEvent + `",` + at + `,"buyer_id":9,"product_id":"` + contentObj + `","product_name":"Apple","image_key":"public/apple"}`,
			"商品補貨通知", "你關注的「Apple」已補貨。", "Back in stock", "\"Apple\", which you were watching, is back in stock.", "/products/" + contentObj, ""},
	}
	ld := Loaded{StoreCode: 20260001}
	for _, c := range cases {
		e, ok := Lookup(c.typ)
		if !ok {
			t.Fatalf("沒有型別 %s", c.typ)
		}
		for _, l := range []uitext.Locale{uitext.ZhTW, uitext.EN} {
			got, err := e.RenderLoaded(l, []byte(c.params), ld)
			if err != nil {
				t.Errorf("%s/%s：%v", c.typ, l, err)
				continue
			}
			title, body, link := c.zhTitle, c.zhBody, c.link
			if l == uitext.EN {
				title, body = c.enTitle, c.enBody
				if link != "" {
					link = "/en" + link
				}
			}
			if got.Title != title || got.Body != body || got.DeepLink != link || got.AggregationKey != c.agg || got.Locale != l {
				t.Errorf("%s/%s 的輸出\n got  %+v\n want title=%q body=%q link=%q agg=%q", c.typ, l, got, title, body, link, c.agg)
			}
		}
	}
}

// 店家解析不到 store_code（沒有公開頁或已不存在）時，需要 store_code 的連結是空的，通知本身照常組出。
func TestStoreCodeLinkIsEmptyWhenTheStoreDoesNotResolve(t *testing.T) {
	for _, name := range []string{"comment_created", "post_liked", "post_published_item"} {
		e, _ := Lookup(name)
		raw, _, _ := e.ExampleParams()
		got, err := e.RenderLoaded(uitext.EN, raw, Loaded{})
		if err != nil || got.DeepLink != "" || got.Title == "" {
			t.Errorf("%s：%+v（%v）", name, got, err)
		}
	}
}

// Load 一批參數只問一次 store_code，重複的店只問一次，查不到的是 0。
func TestContentLoadResolvesStoreCodesInOneCall(t *testing.T) {
	e, _ := Lookup("comment_created")
	src := &announcementSource{codes: map[int64]int64{7: 20260001}}
	mk := func(store int64) json.RawMessage {
		raw, _ := json.Marshal(CommentCreatedParams{StoreID: store, Actor: exampleBuyerParty})
		return raw
	}
	ld, errs, err := e.Load(context.Background(), src, []json.RawMessage{mk(7), mk(8), mk(7)})
	if err != nil || errs[0] != nil || src.calls != 1 {
		t.Fatalf("Load = %+v %v（%v），呼叫 %d 次", ld, errs, err, src.calls)
	}
	if ld[0].StoreCode != 20260001 || ld[1].StoreCode != 0 || ld[2].StoreCode != 20260001 {
		t.Errorf("store_code = %+v", ld)
	}
}

// 語系前綴（D20）由註冊表統一加：en 文字加 /en，zh-TW 不加，空連結維持空；對任何型別一樣，不是各型別自己記得加。
func TestDeepLinkGetsTheLocalePrefixCentrally(t *testing.T) {
	type linkParams struct {
		Link string `json:"link"`
	}
	e := Define(Definition[linkParams]{
		Name: "link_probe", Audiences: []model.Audience{model.AudienceBuyer}, Category: model.CategoryAccount, Presentation: model.PresentationAccount,
		Channels: []model.Channel{model.ChannelInbox}, IdempotencySource: "t", IdempotencyKey: func(p linkParams) string { return p.Link },
		OccurredAt: func(linkParams) time.Time { return exampleAt }, Recipient: func(linkParams) Recipient { return Recipient{AccountID: 1} },
		Text: func(l uitext.Locale, _ linkParams) (Content, error) {
			return Content{Title: uitext.Message{Text: "t", Locale: l}, Body: uitext.Message{Text: "b", Locale: l}}, nil
		},
		DeepLink: func(p linkParams) string { return p.Link },
		Example:  linkParams{Link: "/account/orders/1"},
	})
	for _, c := range []struct {
		l          uitext.Locale
		link, want string
	}{
		{uitext.EN, "/account/orders/1", "/en/account/orders/1"},
		{uitext.ZhTW, "/account/orders/1", "/account/orders/1"},
		{uitext.EN, "", ""},
	} {
		got, err := e.Render(c.l, []byte(`{"link":"`+c.link+`"}`))
		if err != nil || got.DeepLink != c.want {
			t.Errorf("%s %q → %q（%v），預期 %q", c.l, c.link, got.DeepLink, err, c.want)
		}
	}
	prefixed := false
	for _, err := range Validate(Define(Definition[linkParams]{Name: "bad_link", Example: linkParams{Link: "/en/x"}, DeepLink: func(p linkParams) string { return p.Link }})) {
		prefixed = prefixed || strings.Contains(err.Error(), "語系前綴")
	}
	if !prefixed {
		t.Error("範例深連結已帶語系前綴應該被自檢擋下")
	}
}

// 店家號碼不合法的列（store_id <= 0）不進 StoreCodes 查詢，也不讓整批失敗：該列沒有連結，其餘照常解析。
func TestContentLoadDropsInvalidStoreIDsBeforeTheLookup(t *testing.T) {
	e, _ := Lookup("comment_created")
	src := &announcementSource{codes: map[int64]int64{7: 20260001}}
	mk := func(store int64) json.RawMessage {
		raw, _ := json.Marshal(CommentCreatedParams{StoreID: store, Actor: exampleBuyerParty})
		return raw
	}
	ld, _, err := e.Load(context.Background(), src, []json.RawMessage{mk(0), mk(7), mk(-3)})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range src.asked {
		if id <= 0 {
			t.Errorf("StoreCodes 收到不合法的店家號碼 %d", id)
		}
	}
	if ld[0].StoreCode != 0 || ld[1].StoreCode != 20260001 || ld[2].StoreCode != 0 {
		t.Errorf("store_code = %+v", ld)
	}
}

// 補貨通知的推播圖片取自參數的 image_key（D14）。
func TestProductRestockedCarriesThePushImageKey(t *testing.T) {
	e, _ := Lookup("product_restocked")
	raw, _, err := e.ExampleParams()
	if err != nil {
		t.Fatal(err)
	}
	got, err := e.Render(uitext.ZhTW, raw)
	if err != nil || got.ImageKey != "public/apple" {
		t.Fatalf("%+v（%v）", got, err)
	}
}

// 同一則貼文重新發布（公開、取消、再公開）的事件不同、貼文相同：母意圖的冪等鍵只看貼文。
func TestPostPublishedIdempotencyKeyIgnoresTheEventID(t *testing.T) {
	e, _ := Lookup("post_published")
	key := func(event string) string {
		k, err := e.IdempotencyKey([]byte(`{"event_id":"` + event + `","occurred_at":"2026-11-23T04:30:00Z","post_id":"` + contentPost + `","store_id":7}`))
		if err != nil {
			t.Fatal(err)
		}
		return k
	}
	if a, b := key(contentEvent), key(contentObj); a != b {
		t.Errorf("兩次發布的冪等鍵不同：%q、%q", a, b)
	}
}
