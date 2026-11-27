package snapshot

import (
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/model"
)

func comment(body string, refs ...string) Content {
	return Content{Kind: model.TargetComment, Social: &Social{AuthorKind: "BUYER", AuthorID: 7, StoreID: 3, Body: body, MediaRefs: refs}}
}

func mutated(c Content, f func(*Social)) Content {
	s := *c.Social
	f(&s)
	c.Social = &s
	return c
}

func TestEquivalentRepresentationsHashTheSame(t *testing.T) {
	_, want := Encode(comment("第一行\n第二行", "b", "a"))
	for name, c := range map[string]Content{
		"CRLF":         comment("第一行\r\n第二行", "a", "b"),
		"頭尾空白":         comment("　第一行\n第二行 \t", "a", "b"),
		"媒體參照重複":       comment("第一行\n第二行", "a", "b", "a"),
		"舊式 CR 換行":     comment("第一行\r第二行", "b", "a"),
		"NBSP 與全形空白包住": comment(" 第一行\n第二行　", "a", "b"),
	} {
		if _, got := Encode(c); got != want {
			t.Errorf("%s：雜湊應與正規形式相同", name)
		}
	}
}

func TestAnyChangeOfContentChangesTheHash(t *testing.T) {
	_, base := Encode(comment("本文", "a"))
	for name, c := range map[string]Content{
		"本文":   comment("本文改了", "a"),
		"媒體":   comment("本文", "b"),
		"沒有媒體": comment("本文"),
		"作者":   mutated(comment("本文", "a"), func(s *Social) { s.AuthorID = 8 }),
		"種類":   func() Content { x := comment("本文", "a"); x.Kind = model.TargetForumEntry; return x }(),
		"店家":   mutated(comment("本文", "a"), func(s *Social) { s.StoreID = 4 }),
		"中間換行": comment("本\n文", "a"),
	} {
		if _, got := Encode(c); got == base {
			t.Errorf("%s 不同，雜湊不能相同", name)
		}
	}
}

func TestEncodeDoesNotMutateTheInput(t *testing.T) {
	c := comment(" 本文 ", "b", "a")
	Encode(c)
	if c.Social.Body != " 本文 " || !slices.Equal(c.Social.MediaRefs, []string{"b", "a"}) {
		t.Fatalf("輸入被改動：%+v", c.Social)
	}
}

func TestBuyerProfileFieldsAreHashed(t *testing.T) {
	a := Content{Kind: model.TargetBuyer, Buyer: &Buyer{DisplayName: "小明", AvatarAssetID: "a1"}}
	b := Content{Kind: model.TargetBuyer, Buyer: &Buyer{DisplayName: "小明", AvatarAssetID: "a2"}}
	c := Content{Kind: model.TargetBuyer, Buyer: &Buyer{DisplayName: "小華", AvatarAssetID: "a1"}}
	_, ha := Encode(a)
	for name, x := range map[string]Content{"頭像資產": b, "名稱": c} {
		if _, h := Encode(x); h == ha {
			t.Errorf("%s 不同，雜湊不能相同", name)
		}
	}
	if text, _ := Encode(Content{Kind: model.TargetBuyer, Buyer: &Buyer{}}); len(text) < 2 {
		t.Fatal("空內容也要是合法的 JSON 物件")
	}
}

func TestNilAndEmptyMediaAreTheSame(t *testing.T) {
	x, hx := Encode(comment("本文"))
	y, hy := Encode(comment("本文", []string{}...))
	if x != y || hx != hy {
		t.Fatalf("nil 與空切片要相同：%s %s", x, y)
	}
}

func TestDecodeReadsBackWhatEncodeWrote(t *testing.T) {
	text, _ := Encode(comment("本文", "asset-b", "asset-a"))
	got, err := Decode(text)
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != model.TargetComment || got.Social == nil || got.Social.AuthorKind != "BUYER" || got.Social.AuthorID != 7 || got.Social.Body != "本文" || len(got.Social.MediaRefs) != 2 {
		t.Fatalf("%+v", got)
	}
}

func TestDecodeRejectsKindAndFieldMismatch(t *testing.T) {
	for name, text := range map[string]string{
		"種類與欄位不同": `{"kind":"REVIEW","social":{"author_kind":"BUYER"}}`,
		"沒有欄位":    `{"kind":"COMMENT"}`,
		"兩個欄位":    `{"kind":"BUYER","buyer":{},"review":{}}`,
		"壞掉":      `not json`,
	} {
		if _, err := Decode(text); err == nil {
			t.Errorf("%s 應該讀不回", name)
		}
	}
}

func TestEncodePanicsOnAMismatchedUnion(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("種類與欄位不一致是接線錯誤")
		}
	}()
	Encode(Content{Kind: model.TargetReview, Social: &Social{}})
}

func TestStoreOfReadsTheFrozenStoreAuthorOnly(t *testing.T) {
	store, _ := Encode(Content{Kind: model.TargetPost, Social: &Social{AuthorKind: "STORE", AuthorID: 7, StoreID: 7}})
	buyer, _ := Encode(comment("x"))
	if got := StoreOf(store); got == nil || *got != 7 {
		t.Fatalf("店家作者 = %v", got)
	}
	if StoreOf(buyer) != nil || StoreOf("not json") != nil {
		t.Fatal("買家作者與壞掉的快照不回店家")
	}
}

func TestStoreOfRequiresTheStoreToBeTheAuthor(t *testing.T) {
	other, _ := Encode(Content{Kind: model.TargetComment, Social: &Social{AuthorKind: "STORE", AuthorID: 8, StoreID: 7}})
	if StoreOf(other) != nil {
		t.Fatal("別家店的留言下的店家作者不算快照店家")
	}
}

// 處分依據的發文時間與連結目標一併凍結：寫出再讀回得到同一份（時間以 UTC 微秒為準），改發文時間或連結雜湊就不同。
func TestEncodeFreezesAuthoredTimeAndLinkTarget(t *testing.T) {
	at := time.Date(2026, 11, 1, 3, 4, 5, 123456789, time.FixedZone("TST", 8*3600))
	c := mutated(comment("本文"), func(s *Social) { s.AuthoredAt, s.PostID = at, "0190a000-0000-7000-8000-000000000001" })
	text, sum := Encode(c)
	got, err := Decode(text)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Social.AuthoredAt.Equal(at.Truncate(time.Microsecond)) || got.Social.PostID != c.Social.PostID || got.Social.ThreadID != "" {
		t.Fatalf("讀回 %+v", got.Social)
	}
	for name, mutate := range map[string]func(*Social){
		"發文時間": func(s *Social) { s.AuthoredAt = s.AuthoredAt.Add(time.Second) },
		"貼文連結": func(s *Social) { s.PostID = "0190a000-0000-7000-8000-000000000002" },
		"討論串":  func(s *Social) { s.ThreadID = "0190a000-0000-7000-8000-000000000003" },
	} {
		if _, other := Encode(mutated(c, mutate)); other == sum {
			t.Errorf("%s 不同時雜湊要不同", name)
		}
	}
}

func review() Content {
	return Content{Kind: model.TargetReview, Review: &Review{
		AuthorID: 9, SubjectStoreID: 4, ProductID: uuid.UUID{15: 5}, Rating: 2, Body: "不好吃", MediaRefs: []string{"b", "a"}, ReplyBody: "抱歉",
		AuthoredAt: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
	}}
}

func TestReviewSnapshotHasNoStoreAuthor(t *testing.T) {
	text, _ := Encode(review())
	if StoreOf(text) != nil {
		t.Fatal("評價的作者是買家，評價所屬的店家不是作者，StoreOf 不回店家")
	}
	c := review()
	if kind, id, ok := c.Author(); !ok || kind != "BUYER" || id != 9 || c.StoreID() != 0 {
		t.Fatalf("author = %s %d %v store = %d", kind, id, ok, c.StoreID())
	}
}

func TestEveryReviewFieldIsHashed(t *testing.T) {
	_, base := Encode(review())
	for name, f := range map[string]func(*Review){
		"評分":   func(r *Review) { r.Rating = 3 },
		"商品":   func(r *Review) { r.ProductID = uuid.UUID{15: 6} },
		"店家":   func(r *Review) { r.SubjectStoreID = 5 },
		"店家回覆": func(r *Review) { r.ReplyBody = "" },
		"本文":   func(r *Review) { r.Body = "好吃" },
		"圖片":   func(r *Review) { r.MediaRefs = []string{"a"} },
		"作者":   func(r *Review) { r.AuthorID = 10 },
	} {
		c := review()
		r := *c.Review
		f(&r)
		c.Review = &r
		if _, h := Encode(c); h == base {
			t.Errorf("%s 不同，雜湊不能相同", name)
		}
	}
}

func chat() Content {
	t0 := time.Date(2026, 11, 20, 1, 0, 0, 0, time.UTC)
	return Content{Kind: model.TargetChatMessage, Chat: &Chat{
		AuthorKind: "STORE", AuthorID: 4, StoreID: 4, Body: "你好", MediaRefs: []string{"m1"}, AuthoredAt: t0, ConversationID: "c1", Seq: 6,
		Context: []ContextMessage{
			{Seq: 7, SenderKind: "BUYER", SenderID: 9, Type: "TEXT", Body: "後", SentAt: t0},
			{Seq: 5, SenderKind: "BUYER", SenderID: 9, Type: "IMAGE", MediaRefs: []string{"c2", "c1"}, SentAt: t0},
		},
	}}
}

func TestChatContextIsOrderedBySeqAndHashesTheSame(t *testing.T) {
	a := chat()
	b := chat()
	ctx := slices.Clone(b.Chat.Context)
	slices.Reverse(ctx)
	ch := *b.Chat
	ch.Context = ctx
	b.Chat = &ch
	ta, ha := Encode(a)
	tb, hb := Encode(b)
	if ta != tb || ha != hb {
		t.Fatalf("前後文的輸入順序不影響雜湊：\n%s\n%s", ta, tb)
	}
	got, err := Decode(ta)
	if err != nil {
		t.Fatal(err)
	}
	if got.Chat.Context[0].Seq != 5 || got.Chat.Context[1].Seq != 7 {
		t.Fatalf("依 seq 由小到大：%+v", got.Chat.Context)
	}
}

func TestChatTombstonesCarryNoContent(t *testing.T) {
	c := chat()
	ch := *c.Chat
	ch.Context = []ContextMessage{
		{Seq: 5, SenderKind: "BUYER", SenderID: 9, Type: "TEXT", Body: "不可寫入", Hidden: true},
		{Seq: 7, SenderKind: "BUYER", SenderID: 9, Type: "IMAGE", MediaRefs: []string{"secret"}, Erased: true},
	}
	c.Chat = &ch
	n := Normalize(c)
	for _, m := range n.Chat.Context {
		if m.Body != "" || len(m.MediaRefs) != 0 {
			t.Fatalf("墓碑帶了內容：%+v", m)
		}
	}
	if got := c.AssetIDs(); !slices.Equal(got, []string{"m1"}) {
		t.Fatalf("證據保全不含墓碑的圖片：%v", got)
	}
}

func TestChatAssetIDsUnionTargetAndContext(t *testing.T) {
	if got := chat().AssetIDs(); !slices.Equal(got, []string{"c1", "c2", "m1"}) {
		t.Fatalf("目標與前後文圖片的聯集：%v", got)
	}
}

func TestChatStoreIsTheAuthorOnlyWhenTheSenderIsTheStore(t *testing.T) {
	text, _ := Encode(chat())
	if got := StoreOf(text); got == nil || *got != 4 {
		t.Fatalf("店家寄件：%v", got)
	}
	c := chat()
	ch := *c.Chat
	ch.AuthorKind, ch.AuthorID, ch.StoreID = "BUYER", 9, 0
	c.Chat = &ch
	text, _ = Encode(c)
	if StoreOf(text) != nil {
		t.Fatal("買家寄件不回店家")
	}
}

func TestEveryChatFieldIsHashed(t *testing.T) {
	_, base := Encode(chat())
	for name, f := range map[string]func(*Chat){
		"對話":     func(c *Chat) { c.ConversationID = "c2" },
		"seq":    func(c *Chat) { c.Seq = 8 },
		"本文":     func(c *Chat) { c.Body = "別的" },
		"前後文本文":  func(c *Chat) { c.Context = slices.Clone(c.Context); c.Context[0].Body = "改" },
		"少一則前後文": func(c *Chat) { c.Context = c.Context[:1] },
		"墓碑旗標":   func(c *Chat) { c.Context = slices.Clone(c.Context); c.Context[0].Hidden = true },
	} {
		c := chat()
		ch := *c.Chat
		f(&ch)
		c.Chat = &ch
		if _, h := Encode(c); h == base {
			t.Errorf("%s 不同，雜湊不能相同", name)
		}
	}
}

func TestSocialAssetIDsAreSortedAndDeduplicated(t *testing.T) {
	if got := comment("x", "b", "a", "b").AssetIDs(); !slices.Equal(got, []string{"a", "b"}) {
		t.Fatalf("%v", got)
	}
	if got := (Content{Kind: model.TargetBuyer, Buyer: &Buyer{}}).AssetIDs(); got == nil || len(got) != 0 {
		t.Fatalf("買家對象沒有圖片資產：%v", got)
	}
}

func TestReplaceAssetsSwapsEveryReferenceWithoutTouchingTheOriginal(t *testing.T) {
	to := map[string]string{"c1": "n1", "c2": "n2", "m1": "n3"}
	orig := chat()
	got := orig.ReplaceAssets(to)
	if want := []string{"n1", "n2", "n3"}; !slices.Equal(got.AssetIDs(), want) {
		t.Fatalf("換掉之後的圖片：%v，預期 %v", got.AssetIDs(), want)
	}
	if !slices.Equal(orig.AssetIDs(), []string{"c1", "c2", "m1"}) {
		t.Fatalf("原內容被改動：%v", orig.AssetIDs())
	}
	if got.Chat.Body != orig.Chat.Body || len(got.Chat.Context) != len(orig.Chat.Context) {
		t.Fatalf("只換圖片，其餘欄位不變：%+v", got.Chat)
	}
}

func TestReplaceAssetsLeavesUnmappedIDsAndOtherKindsAlone(t *testing.T) {
	if got := comment("x", "a", "b").ReplaceAssets(map[string]string{"a": "z"}).AssetIDs(); !slices.Equal(got, []string{"b", "z"}) {
		t.Fatalf("沒對應的 ID 維持原樣：%v", got)
	}
	buyer := Content{Kind: model.TargetBuyer, Buyer: &Buyer{DisplayName: "a"}}
	if got := buyer.ReplaceAssets(map[string]string{"a": "z"}); got.Buyer == nil || got.Buyer.DisplayName != "a" {
		t.Fatalf("買家內容不變：%+v", got)
	}
}

func TestBuyerSnapshotStoresOnlyTheAvatarAssetID(t *testing.T) {
	text, _ := Encode(Content{Kind: model.TargetBuyer, Buyer: &Buyer{DisplayName: "a", AvatarAssetID: "id"}})
	if strings.Contains(text, `"avatar"`) || !strings.Contains(text, `"avatar_asset_id":"id"`) {
		t.Fatalf("快照只存頭像資產 ID：%s", text)
	}
}

func TestBuyerAvatarIsAnEvidenceAssetThatIsSwappedForTheCopy(t *testing.T) {
	orig := Content{Kind: model.TargetBuyer, Buyer: &Buyer{DisplayName: "a", AvatarAssetID: "src"}}
	if got := orig.AssetIDs(); !slices.Equal(got, []string{"src"}) {
		t.Fatalf("頭像是證據圖片：%v", got)
	}
	got := orig.ReplaceAssets(map[string]string{"src": "copy"})
	if got.Buyer.AvatarAssetID != "copy" || got.Buyer.DisplayName != "a" {
		t.Fatalf("頭像要換成複本的資產：%+v", got.Buyer)
	}
	if orig.Buyer.AvatarAssetID != "src" {
		t.Fatalf("原內容被改動：%+v", orig.Buyer)
	}
	if none := (Content{Kind: model.TargetBuyer, Buyer: &Buyer{DisplayName: "a"}}); len(none.AssetIDs()) != 0 {
		t.Fatalf("沒有頭像就沒有證據圖片：%v", none.AssetIDs())
	}
}
