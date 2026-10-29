//go:build integration

package social_test

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

// SD4：reply_to 只接受同篇、沒有刪除、沒有隱藏的留言；回覆的父一律壓成頂層；回覆只限貼文所屬的店家（舊 social 的規則）。
func TestSD4ReplyTargetMustBeReadableAndOnTheSamePost(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	b := f.newBuyer(t)
	p1 := f.publishedPost(t, storeID, "一")
	p2 := f.publishedPost(t, storeID, "二")
	store := f.asStore(t, storeID)
	top := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p1.ID, Body: "頂層"}))
	other := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p2.ID, Body: "別篇"}))
	r1 := mustV[social.Comment](t)(f.module.CreateComment(ctx, store, social.CommentInput{PostID: p1.ID, Body: "回覆", ParentID: &top.ID}))
	if r1.ParentID == nil || *r1.ParentID != top.ID || r1.ReplyToID == nil || *r1.ReplyToID != top.ID {
		t.Fatalf("沒給 reply_to 時是父留言：%+v", r1)
	}
	// 回覆的父是回覆：壓成頂層。
	r2 := mustV[social.Comment](t)(f.module.CreateComment(ctx, store, social.CommentInput{PostID: p1.ID, Body: "回回覆", ParentID: &r1.ID, ReplyToID: &r1.ID}))
	if *r2.ParentID != top.ID || *r2.ReplyToID != r1.ID {
		t.Fatalf("父壓成頂層、reply_to 保留：%+v", r2)
	}
	// 別篇的留言。
	if _, err := f.module.CreateComment(ctx, store, social.CommentInput{PostID: p1.ID, Body: "x", ParentID: &top.ID, ReplyToID: &other.ID}); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("reply_to 指向別篇：%v", err)
	}
	if _, err := f.module.CreateComment(ctx, store, social.CommentInput{PostID: p1.ID, Body: "x", ParentID: &other.ID}); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("父是別篇的留言：%v", err)
	}
	// 已刪除與被隱藏的目標。
	must(t, f.module.DeleteComment(ctx, store, r2.ID))
	if _, err := f.module.CreateComment(ctx, store, social.CommentInput{PostID: p1.ID, Body: "x", ParentID: &top.ID, ReplyToID: &r2.ID}); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("reply_to 已刪除：%v", err)
	}
	_, err := f.module.Hide(ctx, social.HideInput{Target: social.Target{Kind: social.TargetComment, ID: top.ID}, Category: "SPAM", AdminID: 1})
	must(t, err)
	if _, err := f.module.CreateComment(ctx, store, social.CommentInput{PostID: p1.ID, Body: "x", ParentID: &top.ID}); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("父留言被隱藏（reply_to 預設是父）：%v", err)
	}
	// 回覆只限貼文所屬的店；reply_to 沒有父。
	other2, _ := f.exposableStore(t)
	if _, err := f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p1.ID, Body: "x", ParentID: &r1.ID}); kindOf(err) != apperr.KindPermissionDenied {
		t.Fatalf("買家不能回覆：%v", err)
	}
	if _, err := f.module.CreateComment(ctx, f.asStore(t, other2), social.CommentInput{PostID: p1.ID, Body: "x", ParentID: &r1.ID}); kindOf(err) != apperr.KindPermissionDenied {
		t.Fatalf("別家店也不是貼文所屬的店（沒有買家身分）：%v", err)
	}
	if _, err := f.module.CreateComment(ctx, store, social.CommentInput{PostID: p1.ID, Body: "x", ReplyToID: &r1.ID}); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("reply_to 要帶父留言：%v", err)
	}
}

// SD5：重複刪除冪等，只扣一次、不做副作用。
func TestSD5DeleteIsIdempotent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	b := f.newBuyer(t)
	p := f.publishedPost(t, storeID, "貼文")
	c := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: "留言"}))
	e := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(b), social.ForumInput{StoreID: storeID, Body: "討論"}))
	for range 2 {
		must(t, f.module.DeleteComment(ctx, f.asBuyer(b), c.ID))
		must(t, f.module.DeleteForumEntry(ctx, f.asBuyer(b), e.ID))
		must(t, f.module.DeletePost(ctx, f.store(t, storeID), mustV[social.Post](t)(f.module.CreatePost(ctx, f.store(t, storeID), social.CreatePostInput{Body: "草稿"})).ID, 0))
	}
	if got := mustV[social.Post](t)(f.module.PublicPost(ctx, social.Caller{}, p.ID)); got.Stats.Comments != 0 {
		t.Fatalf("連刪兩次只扣一次，不會變負數：%d", got.Stats.Comments)
	}
	// 貼文重複刪除。
	must(t, f.module.DeletePost(ctx, f.store(t, storeID), p.ID, 0))
	f.jobs.reset()
	must(t, f.module.DeletePost(ctx, f.store(t, storeID), p.ID, 0))
	if len(f.jobs.revalidations()) != 0 {
		t.Fatal("重複刪除貼文不做副作用")
	}
	// 別人的留言不能刪；店家只能刪自己貼文底下的。
	p2 := f.publishedPost(t, storeID, "二")
	c2 := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p2.ID, Body: "x"}))
	if err := f.module.DeleteComment(ctx, f.asBuyer(f.newBuyer(t)), c2.ID); kindOf(err) != apperr.KindPermissionDenied {
		t.Fatalf("別的買家：%v", err)
	}
	must(t, f.module.DeleteComment(ctx, f.asStore(t, storeID), c2.ID))
}

// SD6：髒字詞典在所有寫入路徑（建立與編輯貼文、留言、討論區）都擋；長度與空白一併檢查。
func TestSD6ProfanityFilterCoversEveryWritePath(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	st := f.store(t, storeID)
	b := f.newBuyer(t)
	bad := "你這個白痴"
	if _, err := f.module.CreatePost(ctx, st, social.CreatePostInput{Body: bad}); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("建立貼文：%v", err)
	}
	p := f.publishedPost(t, storeID, "乾淨的貼文")
	if _, err := f.module.UpdatePost(ctx, st, social.UpdatePostInput{PostID: p.ID, Version: p.Version, Body: bad}); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("編輯貼文：%v", err)
	}
	if _, err := f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: bad}); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("建立留言：%v", err)
	}
	c := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: "好吃"}))
	if _, err := f.module.UpdateComment(ctx, f.asBuyer(b), c.ID, bad); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("編輯留言：%v", err)
	}
	if _, err := f.module.CreateForumEntry(ctx, f.asBuyer(b), social.ForumInput{StoreID: storeID, Body: bad}); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("建立討論區留言：%v", err)
	}
	e := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(b), social.ForumInput{StoreID: storeID, Body: "問題"}))
	if _, err := f.module.UpdateForumEntry(ctx, f.asBuyer(b), e.ID, bad); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("編輯討論區留言：%v", err)
	}
	// 空白與長度。
	for _, body := range []string{"", "  \t　 ", strings.Repeat("字", 2201)} {
		if _, err := f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: body}); kindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("留言 %q：%v", body, err)
		}
	}
	if _, err := f.module.CreatePost(ctx, st, social.CreatePostInput{Body: strings.Repeat("字", 2201)}); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("貼文太長：%v", err)
	}
	// 上限（Will 2026-10-23）：貼文、留言、討論區留言各 2200 個字元，剛好 2200 通過。
	full := strings.Repeat("字", 2200)
	mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: full}))
	mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(b), social.ForumInput{StoreID: storeID, Body: full}))
	mustV[social.Post](t)(f.module.CreatePost(ctx, st, social.CreatePostInput{Body: full}))
	if _, err := f.module.CreateForumEntry(ctx, f.asBuyer(b), social.ForumInput{StoreID: storeID, Body: full + "字"}); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("討論區留言太長：%v", err)
	}
	if got := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: "  頭尾有空白  "})); got.Body != "頭尾有空白" {
		t.Fatalf("寫入前去頭尾空白：%q", got.Body)
	}
}

// SD8：PostPublished 在轉為公開時發，建立草稿不發；已經公開時再發布不再發。
func TestSD8PostPublishedFiresOnPublishNotOnDraft(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	st := f.store(t, storeID)
	var published []social.PostPublished
	events.Subscribe(f.bus, func(_ context.Context, e social.PostPublished) error {
		published = append(published, e)
		return nil
	})
	draft := mustV[social.Post](t)(f.module.CreatePost(ctx, st, social.CreatePostInput{Body: "草稿", ImageToken: "t1"}))
	if len(published) != 0 {
		t.Fatal("建立草稿不發 PostPublished")
	}
	pub := mustV[social.Post](t)(f.module.PublishPost(ctx, st, draft.ID, draft.Version))
	if len(published) != 1 || published[0].PostID != draft.ID || published[0].StoreID != storeID || published[0].Preview != "草稿" {
		t.Fatalf("發布才發：%+v", published)
	}
	// DR-A2：已經公開時再發布是 no-op：不寫入（version、updated_at 不變）、不發事件、published_at 不變。
	f.advance(3600e9)
	again := mustV[social.Post](t)(f.module.PublishPost(ctx, st, pub.ID, pub.Version))
	if len(published) != 1 || again.Version != pub.Version || !again.PublishedAt.Equal(*pub.PublishedAt) || !again.UpdatedAt.Equal(pub.UpdatedAt) {
		t.Fatalf("已經公開再發布是 no-op：%d %+v %+v", len(published), pub, again)
	}
	// 下架不清 published_at；下架後再發布是新的公開（再發事件），但 published_at 沿用第一次（動態牆不能靠下架再發布來洗）。
	f.advance(3600e9)
	down := mustV[social.Post](t)(f.module.UnpublishPost(ctx, st, again.ID, again.Version))
	if down.Status != social.StatusUnpublished || down.PublishedAt == nil || !down.PublishedAt.Equal(*pub.PublishedAt) {
		t.Fatalf("下架不清 published_at：%+v", down)
	}
	f.advance(3600e9)
	re := mustV[social.Post](t)(f.module.PublishPost(ctx, st, down.ID, down.Version))
	if len(published) != 2 {
		t.Fatalf("下架後再發布要再發：%d", len(published))
	}
	if re.Status != social.StatusPublished || re.PublishedAt == nil || !re.PublishedAt.Equal(*pub.PublishedAt) || re.Version != down.Version+1 {
		t.Fatalf("再發布沿用第一次的 published_at：first %v, now %+v", pub.PublishedAt, re)
	}
	// 直接以發布建立也發。
	f.publishedPost(t, storeID, "直接發布")
	if len(published) != 3 {
		t.Fatalf("直接發布建立：%d", len(published))
	}
	// 草稿不能下架；公開要件。
	d2 := mustV[social.Post](t)(f.module.CreatePost(ctx, st, social.CreatePostInput{Body: "草稿二"}))
	if _, err := f.module.UnpublishPost(ctx, st, d2.ID, d2.Version); kindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("草稿不能下架：%v", err)
	}
	if _, err := f.module.PublishPost(ctx, st, d2.ID, d2.Version); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("沒有圖不能發布：%v", err)
	}
	if _, err := f.module.CreatePost(ctx, st, social.CreatePostInput{Body: "", ImageToken: "t2", Publish: true}); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("內文必填：%v", err)
	}
}

// SD9：推播預覽以 rune 截斷，100 個漢字的預覽是合法的 UTF-8。
func TestSD9PreviewIsTruncatedByRunes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	var preview string
	events.Subscribe(f.bus, func(_ context.Context, e social.CommentCreated) error {
		preview = e.Preview
		return nil
	})
	p := f.publishedPost(t, storeID, "貼文")
	b := f.newBuyer(t)
	mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: strings.Repeat("漢", 150)}))
	if !utf8.ValidString(preview) || utf8.RuneCountInString(preview) != 100 {
		t.Fatalf("預覽 %d 個字元、合法 UTF-8 = %v", utf8.RuneCountInString(preview), utf8.ValidString(preview))
	}
}

// SD24：貼文的商品在寫入時驗證存在、上架中且屬於該店（經 port），讀取時不重驗；查詢失敗不當成商品不存在。
func TestSD24ProductIsVerifiedOnWrite(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	otherStore, _ := f.exposableStore(t)
	f.products.owner[7] = storeID
	f.products.owner[8] = otherStore
	st := f.store(t, storeID)
	id := func(v int64) *int64 { return &v }
	p := mustV[social.Post](t)(f.module.CreatePost(ctx, st, social.CreatePostInput{Body: "x", ProductID: id(7)}))
	if p.ProductID == nil || *p.ProductID != 7 {
		t.Fatalf("%+v", p)
	}
	for _, bad := range []int64{9, 8, -1} {
		if _, err := f.module.CreatePost(ctx, st, social.CreatePostInput{Body: "x", ProductID: id(bad)}); kindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("商品 %d：%v", bad, err)
		}
		if _, err := f.module.UpdatePost(ctx, st, social.UpdatePostInput{PostID: p.ID, Version: p.Version, Body: "x", ProductID: id(bad)}); kindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("更新商品 %d：%v", bad, err)
		}
	}
	// 之後商品下架：讀取不重驗。
	delete(f.products.owner, 7)
	if got := mustV[social.Post](t)(f.module.OwnPost(ctx, storeID, p.ID)); got.ProductID == nil {
		t.Fatal("讀取時不重驗商品")
	}
	f.products.err = apperr.Unavailable("商品服務停擺")
	if _, err := f.module.CreatePost(ctx, st, social.CreatePostInput{Body: "x", ProductID: id(7)}); kindOf(err) != apperr.KindUnavailable {
		t.Fatalf("查詢失敗要回錯誤：%v", err)
	}
}

// SD19：標籤搜尋在查詢前驗證字元集（LIKE 的萬用字元不是標籤字元）；前綴與包含兩種。
func TestSD19HashtagSearchValidatesCharacters(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	f.publishedPost(t, storeID, "a", "有機蔬菜", "Organic")
	f.publishedPost(t, storeID, "b", "有機水果")
	for _, bad := range []string{"%", "_", "有%", "a_b", "有 機", "😀"} {
		if _, err := f.module.SuggestHashtags(ctx, bad, false, 10); kindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("%q：%v", bad, err)
		}
	}
	got := mustV[[]social.HashtagCount](t)(f.module.SuggestHashtags(ctx, "#有機", false, 10))
	if len(got) != 2 {
		t.Fatalf("前綴：%+v", got)
	}
	got = mustV[[]social.HashtagCount](t)(f.module.SuggestHashtags(ctx, "機蔬", true, 10))
	if len(got) != 1 || got[0].Name != "有機蔬菜" || got[0].Posts != 1 {
		t.Fatalf("包含：%+v", got)
	}
	if got := mustV[[]social.HashtagCount](t)(f.module.SuggestHashtags(ctx, "ORGAN", false, 10)); len(got) != 1 || got[0].Name != "organic" {
		t.Fatalf("名稱是 ASCII 小寫：%+v", got)
	}
	if got := mustV[[]social.HashtagCount](t)(f.module.SuggestHashtags(ctx, "###", false, 10)); len(got) != 0 {
		t.Fatalf("正規化後是空的沒有建議：%+v", got)
	}
}

// 標籤：整批替換、正規化與去重、數量與長度。
func TestHashtagsAreAReplaceableValueSet(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	st := f.store(t, storeID)
	p := f.publishedPost(t, storeID, "貼文", "#Fruit", "fruit", "水果")
	if len(p.Hashtags) != 2 || p.Hashtags[0].Name != "fruit" || p.Hashtags[0].Display != "Fruit" || p.Hashtags[1].Name != "水果" {
		t.Fatalf("正規化與去重（保留第一次出現的樣子與順序）：%+v", p.Hashtags)
	}
	// 不帶 Hashtags 保留；帶空集合清空；帶新集合取代。
	p = mustV[social.Post](t)(f.module.UpdatePost(ctx, st, social.UpdatePostInput{PostID: p.ID, Version: p.Version, Body: "改"}))
	if len(p.Hashtags) != 2 {
		t.Fatalf("nil 保留：%+v", p.Hashtags)
	}
	tags := []string{"蔬菜"}
	p = mustV[social.Post](t)(f.module.UpdatePost(ctx, st, social.UpdatePostInput{PostID: p.ID, Version: p.Version, Body: "改", Hashtags: &tags}))
	if len(p.Hashtags) != 1 || p.Hashtags[0].Name != "蔬菜" {
		t.Fatalf("取代：%+v", p.Hashtags)
	}
	none := []string{}
	p = mustV[social.Post](t)(f.module.UpdatePost(ctx, st, social.UpdatePostInput{PostID: p.ID, Version: p.Version, Body: "改", Hashtags: &none}))
	if len(p.Hashtags) != 0 {
		t.Fatalf("空集合清空：%+v", p.Hashtags)
	}
	var eleven []string
	for i := range 11 {
		eleven = append(eleven, string(rune('a'+i)))
	}
	for _, bad := range [][]string{eleven, {"a b"}, {"a#b"}, {""}, {"#"}, {strings.Repeat("字", 41)}} {
		if _, err := f.module.CreatePost(ctx, st, social.CreatePostInput{Body: "x", Hashtags: bad}); kindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("%v：%v", bad, err)
		}
	}
	if _, err := f.module.CreatePost(ctx, st, social.CreatePostInput{Body: "x", Hashtags: []string{strings.Repeat("字", 40)}}); err != nil {
		t.Fatalf("40 個字元可以：%v", err)
	}
}

// 貼文以 version CAS 寫入：版本不符回 VersionConflict、不寫入；別家店的貼文是 NotFound。
func TestPostWritesUseVersionCAS(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	otherID, _ := f.exposableStore(t)
	st := f.store(t, storeID)
	p := f.publishedPost(t, storeID, "原文")
	up := mustV[social.Post](t)(f.module.UpdatePost(ctx, st, social.UpdatePostInput{PostID: p.ID, Version: p.Version, Body: "新文"}))
	if up.Version != p.Version+1 {
		t.Fatalf("版本加一：%d → %d", p.Version, up.Version)
	}
	if _, err := f.module.UpdatePost(ctx, st, social.UpdatePostInput{PostID: p.ID, Version: p.Version, Body: "過期"}); kindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("過期的版本：%v", err)
	}
	if _, err := f.module.UnpublishPost(ctx, st, p.ID, p.Version); kindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("過期的版本下架：%v", err)
	}
	if got := mustV[social.Post](t)(f.module.OwnPost(ctx, storeID, p.ID)); got.Body != "新文" {
		t.Fatalf("衝突不寫入：%q", got.Body)
	}
	if _, err := f.module.UpdatePost(ctx, f.store(t, otherID), social.UpdatePostInput{PostID: p.ID, Version: up.Version, Body: "偷改"}); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("別家店：%v", err)
	}
	if _, err := f.module.UpdatePost(ctx, st, social.UpdatePostInput{PostID: p.ID, Body: "沒帶版本"}); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("沒帶版本：%v", err)
	}
	// 已發布的貼文被編輯後仍要滿足公開要件。
	if _, err := f.module.UpdatePost(ctx, st, social.UpdatePostInput{PostID: p.ID, Version: up.Version, Body: ""}); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("已發布的貼文內文不能清空：%v", err)
	}
	if _, err := f.module.UpdatePost(ctx, st, social.UpdatePostInput{PostID: p.ID, Version: up.Version, Body: "x", ClearImage: true}); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("已發布的貼文不能清圖：%v", err)
	}
}

// 店家身分的寫入以能力檢查：停權的店建立、編輯、發布貼文與留言都被擋（CapMarketing），下架與刪除自己的內容仍可（CapContentContract，停權時仍授予）。
func TestStoreWritesAreGatedByCapabilities(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	live := f.store(t, storeID)
	p := f.publishedPost(t, storeID, "貼文")
	b := f.newBuyer(t)
	c := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: "問"}))

	suspended := live
	suspended.Sanctions = seller.SanctionedBy(seller.EffectiveMeasure{MeasureID: uuid.NewV7()})
	if _, err := f.module.CreatePost(ctx, suspended, social.CreatePostInput{Body: "x"}); kindOf(err) != apperr.KindPermissionDenied {
		t.Fatalf("停權建立貼文：%v", err)
	}
	if _, err := f.module.UpdatePost(ctx, suspended, social.UpdatePostInput{PostID: p.ID, Version: p.Version, Body: "x"}); kindOf(err) != apperr.KindPermissionDenied {
		t.Fatalf("停權編輯貼文：%v", err)
	}
	if _, err := f.module.PublishPost(ctx, suspended, p.ID, p.Version); kindOf(err) != apperr.KindPermissionDenied {
		t.Fatalf("停權發布：%v", err)
	}
	sc := social.Caller{Store: &suspended}
	if _, err := f.module.CreateComment(ctx, sc, social.CommentInput{PostID: p.ID, Body: "回", ParentID: &c.ID}); kindOf(err) != apperr.KindPermissionDenied {
		t.Fatalf("停權的店在自己的貼文上留言（不退回買家身分）：%v", err)
	}
	if _, err := f.module.CreateForumEntry(ctx, sc, social.ForumInput{StoreID: storeID, Body: "公告"}); kindOf(err) != apperr.KindPermissionDenied {
		t.Fatalf("停權的店在自己的討論區留言：%v", err)
	}
	if _, err := f.module.LikeComment(ctx, sc, c.ID); kindOf(err) != apperr.KindPermissionDenied {
		t.Fatalf("停權的店對留言按讚：%v", err)
	}
	// 收縮型的動作停權時仍可：刪除別人的留言（貼文所屬的店）與下架自己的貼文。
	must(t, f.module.DeleteComment(ctx, sc, c.ID))
	if _, err := f.module.UnpublishPost(ctx, suspended, p.ID, p.Version); err != nil {
		t.Fatalf("停權時仍可下架自己的貼文：%v", err)
	}
	// 隨帳號刪除中的店不能做經營類的事（前置條件沒有滿足）。
	f.exec(t, "UPDATE seller.stores SET status = 'PENDING_DELETION' WHERE store_id = $1", storeID)
	if _, err := f.module.CreatePost(ctx, f.store(t, storeID), social.CreatePostInput{Body: "x"}); kindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("等待刪除的店不能建立貼文：%v", err)
	}
	// 沒有推導過條件的店一律拒絕（fail-closed）。
	if _, err := f.module.CreatePost(ctx, seller.Store{Profile: live.Profile, StoreID: live.StoreID, AccountID: live.AccountID}, social.CreatePostInput{Body: "x"}); err == nil {
		t.Fatal("沒有推導過條件的店要拒絕")
	}
}

// 買家身分的寫入要求買家是營運中：刪除中的買家不能留言、按讚、分享。
func TestBuyerWritesRequireAnActiveBuyer(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	p := f.publishedPost(t, storeID, "貼文")
	b := f.newBuyer(t)
	must(t, f.buyers.BeginDeletion(ctx, b.BuyerID))
	if _, err := f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: "x"}); kindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("留言：%v", err)
	}
	if _, err := f.module.CreateForumEntry(ctx, f.asBuyer(b), social.ForumInput{StoreID: storeID, Body: "x"}); kindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("討論區：%v", err)
	}
	if _, err := f.module.LikePost(ctx, b.BuyerID, p.ID); kindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("按讚：%v", err)
	}
	if _, err := f.module.RecordShare(ctx, b.BuyerID, p.ID); kindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("分享：%v", err)
	}
	if _, err := f.module.CreateComment(ctx, social.Caller{}, social.CommentInput{PostID: p.ID, Body: "x"}); kindOf(err) != apperr.KindPermissionDenied {
		t.Fatalf("沒有身分：%v", err)
	}
}

// 換圖在 transaction 內寫業務列並排入引用同步；舊的圖記成退場，由同步 job 精確解除；轉正在 transaction 之外。
func TestPostImageReplacementRetiresTheOldAsset(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	st := f.store(t, storeID)
	p := mustV[social.Post](t)(f.module.CreatePost(ctx, st, social.CreatePostInput{Body: "x", ImageToken: "first", Publish: true}))
	ref := media.Ref{Type: media.RefTypePost, ID: p.ID.String()}
	if r := mustV[media.SyncResult](t)(f.module.SyncMedia(ctx, ref)); r.Again {
		t.Fatal("第一次同步一次就完成")
	}
	if n := len(f.media.Referenced(ref)); n != 1 {
		t.Fatalf("同步後有一個引用：%d", n)
	}
	up := mustV[social.Post](t)(f.module.UpdatePost(ctx, st, social.UpdatePostInput{PostID: p.ID, Version: p.Version, Body: "y", ImageToken: "second"}))
	if up.ImageKey == p.ImageKey {
		t.Fatal("圖換了")
	}
	mustV[media.SyncResult](t)(f.module.SyncMedia(ctx, ref))
	if n := len(f.media.Referenced(ref)); n != 1 {
		t.Fatalf("舊的引用已解除、只剩新的：%d", n)
	}
	retired := f.scalar(t, "SELECT count(*) FROM social.retired_assets WHERE ref_id = $1", p.ID.String())
	if retired != 1 {
		t.Fatalf("舊圖記成退場：%d", retired)
	}
}

// P1b：post_hashtags.published_at 是貼文 published_at 的複本（trigger 維護）。草稿的標籤列是 NULL；第一次發布寫進所有標籤列；
// 已發布貼文之後才寫入的標籤列帶入貼文的值；下架再發布沿用；貼文的 published_at 發布後不能改。
func TestHashtagPublishedAtIsACopyOfThePost(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	st := f.store(t, storeID)
	mismatch := func() int64 {
		return f.scalar(t, `SELECT count(*) FROM social.post_hashtags h JOIN social.posts p ON p.id = h.post_id WHERE h.published_at IS DISTINCT FROM p.published_at`)
	}
	draft := mustV[social.Post](t)(f.module.CreatePost(ctx, st, social.CreatePostInput{Body: "草稿", ImageToken: "d1", Hashtags: []string{"fruit", "水果"}}))
	if n := f.scalar(t, `SELECT count(*) FROM social.post_hashtags WHERE post_id = $1 AND published_at IS NULL`, draft.ID); n != 2 {
		t.Fatalf("草稿的標籤列沒有 published_at：%d", n)
	}
	f.advance(3600e9)
	pub := mustV[social.Post](t)(f.module.PublishPost(ctx, st, draft.ID, draft.Version))
	if n := f.scalar(t, `SELECT count(*) FROM social.post_hashtags WHERE post_id = $1 AND published_at = $2`, pub.ID, *pub.PublishedAt); n != 2 || mismatch() != 0 {
		t.Fatalf("第一次發布寫進所有標籤列：%d，不一致 %d", n, mismatch())
	}
	// 已發布的貼文換一組標籤：新列帶入貼文的值。
	tags := []string{"蔬菜", "fruit", "organic"}
	f.advance(3600e9)
	upd := mustV[social.Post](t)(f.module.UpdatePost(ctx, st, social.UpdatePostInput{PostID: pub.ID, Version: pub.Version, Body: "改", Hashtags: &tags}))
	if n := f.scalar(t, `SELECT count(*) FROM social.post_hashtags WHERE post_id = $1`, pub.ID); n != 3 || mismatch() != 0 {
		t.Fatalf("已發布貼文的新標籤列帶入貼文的 published_at：%d，不一致 %d", n, mismatch())
	}
	// 下架再發布：貼文與標籤列都沿用第一次。
	f.advance(3600e9)
	down := mustV[social.Post](t)(f.module.UnpublishPost(ctx, st, upd.ID, upd.Version))
	f.advance(3600e9)
	re := mustV[social.Post](t)(f.module.PublishPost(ctx, st, down.ID, down.Version))
	if !re.PublishedAt.Equal(*pub.PublishedAt) || mismatch() != 0 {
		t.Fatalf("再發布沿用第一次：%v／%v，不一致 %d", re.PublishedAt, pub.PublishedAt, mismatch())
	}
	// 標籤列表的順序與貼文的 published_at 一致（先發布的在後）。
	later := f.publishedPost(t, storeID, "較晚", "organic")
	page := mustV[social.PostPage](t)(f.module.ListHashtagPosts(ctx, social.Caller{}, "organic", nil, 10))
	if len(page.Posts) != 2 || page.Posts[0].ID != later.ID || page.Posts[1].ID != pub.ID {
		t.Fatalf("標籤的貼文依發布時間由新到舊：%+v", page.Posts)
	}
	// 貼文的 published_at 發布後不能改（複本才不需要同步）。
	if _, err := f.db.Conn(ctx).Exec(ctx, `UPDATE social.posts SET published_at = published_at + interval '1 hour' WHERE id = $1`, pub.ID); err == nil {
		t.Fatal("published_at 發布後改動要被擋下")
	}
}
