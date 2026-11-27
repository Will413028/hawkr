//go:build integration

package social_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

type readable struct {
	post     bool
	comments int
	forum    int
	feed     bool
}

func (f *fixture) readable(t *testing.T, postID uuidT, storeID int64) readable {
	t.Helper()
	ctx := context.Background()
	var r readable
	if _, err := f.module.PublicPost(ctx, social.Caller{}, postID); err == nil {
		r.post = true
	} else if kindOf(err) != apperr.KindNotFound {
		t.Fatal(err)
	}
	batch, err := f.module.PubliclyReadablePosts(ctx, []uuidT{postID, {}})
	must(t, err)
	if batch[postID] != r.post || batch[uuidT{}] {
		t.Fatalf("PubliclyReadablePosts = %v，與 PublicPost 讀得到 = %v 不一致", batch, r.post)
	}
	if page, err := f.module.ListComments(ctx, social.Caller{}, postID, nil, 50); err == nil {
		r.comments = len(page.Comments)
	} else if kindOf(err) != apperr.KindNotFound {
		t.Fatal(err)
	}
	if own, err := f.module.OwnPost(ctx, storeID, postID); err == nil && own.Readable != r.post {
		t.Fatalf("OwnPost.Readable = %v，與 PublicPost 讀得到 = %v 不一致", own.Readable, r.post)
	} else if err != nil && kindOf(err) != apperr.KindNotFound {
		t.Fatal(err)
	}
	if page, err := f.module.ListStoreComments(ctx, f.store(t, storeID), nil, false, nil, 50); err == nil {
		for _, c := range page.Comments {
			if c.PostID == postID && c.PostReadable != r.post {
				t.Fatalf("ListStoreComments 的 PostReadable = %v，與 PublicPost 讀得到 = %v 不一致", c.PostReadable, r.post)
			}
		}
	} else {
		t.Fatal(err)
	}
	forum, err := f.module.ListForumEntries(ctx, social.Caller{}, storeID, nil, 50)
	must(t, err)
	r.forum = len(forum.Entries)
	if r.forum > 0 && !forum.Readable {
		t.Fatalf("討論區有 %d 則可讀留言，ForumPage.Readable 卻是 false", r.forum)
	}
	feed, err := f.module.ListFeed(ctx, social.Caller{}, nil, 50)
	must(t, err)
	for _, p := range feed.Posts {
		if p.ID == postID {
			r.feed = true
		}
	}
	return r
}

// SD1、SD2：一則內容公開可讀，若且唯若店家可曝光、貼文已發布、沒有刪除、沒有未還原的隱藏、作者沒有停用。草稿、下架、刪除、隱藏、店家不可
// 曝光、買家刪除中與已關閉，各自讓貼文、留言、討論區不可見；還原後可見。
func TestSD1SD2PublicReadabilityPredicate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	type scenario struct {
		name   string
		apply  func(t *testing.T, f *fixture, st social.Post, c social.Comment, e social.ForumEntry, b buyerP)
		revert func(t *testing.T, f *fixture, st social.Post, c social.Comment, e social.ForumEntry, b buyerP)
		want   readable
	}
	all := readable{post: true, comments: 1, forum: 1, feed: true}
	scenarios := []scenario{
		{name: "下架", want: readable{forum: 1},
			apply: func(t *testing.T, f *fixture, p social.Post, _ social.Comment, _ social.ForumEntry, _ buyerP) {
				_, err := f.module.UnpublishPost(ctx, f.store(t, p.StoreID), p.ID, p.Version, nil)
				must(t, err)
			},
			revert: func(t *testing.T, f *fixture, p social.Post, _ social.Comment, _ social.ForumEntry, _ buyerP) {
				cur := mustV[social.Post](t)(f.module.OwnPost(ctx, p.StoreID, p.ID))
				_, err := f.module.PublishPost(ctx, f.store(t, p.StoreID), p.ID, cur.Version, nil)
				must(t, err)
			}},
		{name: "貼文刪除", want: readable{forum: 1},
			apply: func(t *testing.T, f *fixture, p social.Post, _ social.Comment, _ social.ForumEntry, _ buyerP) {
				must(t, f.module.DeleteOwnPost(ctx, f.store(t, p.StoreID), p.ID, 0))
			}},
		{name: "留言被隱藏", want: readable{post: true, forum: 1, feed: true},
			apply: func(t *testing.T, f *fixture, _ social.Post, c social.Comment, _ social.ForumEntry, _ buyerP) {
				_, err := f.module.Hide(ctx, social.HideInput{Target: social.Target{Kind: social.TargetComment, ID: c.ID}, Category: "SPAM", AdminID: 1})
				must(t, err)
			},
			revert: func(t *testing.T, f *fixture, _ social.Post, c social.Comment, _ social.ForumEntry, _ buyerP) {
				_, err := f.module.Restore(ctx, social.RestoreInput{Target: social.Target{Kind: social.TargetComment, ID: c.ID}, AdminID: 1, Reason: "誤判"})
				must(t, err)
			}},
		{name: "討論區留言被隱藏", want: readable{post: true, comments: 1, feed: true},
			apply: func(t *testing.T, f *fixture, _ social.Post, _ social.Comment, e social.ForumEntry, _ buyerP) {
				_, err := f.module.Hide(ctx, social.HideInput{Target: social.Target{Kind: social.TargetForumEntry, ID: e.ID}, Category: "HARASSMENT", AdminID: 1})
				must(t, err)
			},
			revert: func(t *testing.T, f *fixture, _ social.Post, _ social.Comment, e social.ForumEntry, _ buyerP) {
				_, err := f.module.Restore(ctx, social.RestoreInput{Target: social.Target{Kind: social.TargetForumEntry, ID: e.ID}, AdminID: 1, Reason: "誤判"})
				must(t, err)
			}},
		{name: "店家不可曝光", want: readable{},
			apply: func(t *testing.T, f *fixture, p social.Post, _ social.Comment, _ social.ForumEntry, _ buyerP) {
				f.setExposable(t, p.StoreID, false)
			},
			revert: func(t *testing.T, f *fixture, p social.Post, _ social.Comment, _ social.ForumEntry, _ buyerP) {
				f.setExposable(t, p.StoreID, true)
			}},
		{name: "買家刪除中", want: readable{post: true, feed: true},
			apply: func(t *testing.T, f *fixture, _ social.Post, _ social.Comment, _ social.ForumEntry, b buyerP) {
				must(t, f.buyers.BeginDeletion(ctx, b.BuyerID))
			}},
		{name: "買家已關閉", want: readable{post: true, feed: true},
			apply: func(t *testing.T, f *fixture, _ social.Post, _ social.Comment, _ social.ForumEntry, b buyerP) {
				must(t, f.buyers.CloseForDeletion(ctx, b.BuyerID))
			}},
	}
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			t.Parallel()
			f := setup(t)
			storeID, _ := f.exposableStore(t)
			b := f.newBuyer(t)
			p := f.publishedPost(t, storeID, "貼文")
			c := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: "留言"}))
			e := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(b), social.ForumInput{StoreID: storeID, Body: "討論"}))
			if got := f.readable(t, p.ID, storeID); got != all {
				t.Fatalf("套用前 %+v", got)
			}
			sc.apply(t, f, p, c, e, b)
			if got := f.readable(t, p.ID, storeID); got != sc.want {
				t.Fatalf("套用後 %+v，預期 %+v", got, sc.want)
			}
			if sc.revert != nil {
				sc.revert(t, f, p, c, e, b)
				if got := f.readable(t, p.ID, storeID); got != all {
					t.Fatalf("還原後 %+v，預期全部可見", got)
				}
			}
		})
	}
}

// SD1：草稿貼文公開讀不到，而且它的留言也讀不到（SD2）；店主讀得到草稿與草稿底下的留言（C8，這一步沒有留言入口，讀取走店主的查詢）。
func TestSD1DraftIsNotPublic(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	p := mustV[social.Post](t)(f.module.CreatePost(ctx, f.store(t, storeID), social.CreatePostInput{Body: "草稿"}, nil))
	if p.Status != social.StatusDraft || p.PublishedAt != nil {
		t.Fatalf("%+v", p)
	}
	if _, err := f.module.PublicPost(ctx, social.Caller{}, p.ID); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("匿名讀草稿：%v", err)
	}
	if _, err := f.module.ListComments(ctx, social.Caller{}, p.ID, nil, 10); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("匿名讀草稿的留言：%v", err)
	}
	b := f.newBuyer(t)
	if _, err := f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: "x"}); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("草稿不能留言：%v", err)
	}
	if _, err := f.module.ListComments(ctx, f.asStore(t, storeID), p.ID, nil, 10); err != nil {
		t.Fatalf("店主讀自己的草稿的留言：%v", err)
	}
	if feed := mustV[social.PostPage](t)(f.module.ListFeed(ctx, social.Caller{}, nil, 10)); len(feed.Posts) != 0 {
		t.Fatalf("動態牆不含草稿：%+v", feed)
	}
}

// 作者讀得到自己被隱藏的內容與理由，別人讀不到（A7-D9）；作者不能編輯被隱藏的內容（SD7）；還原之後可以編輯。SD13：還原後公開列表含該留言。
func TestSD7SD13HiddenContentIsVisibleToItsAuthorOnly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	author, other := f.newBuyer(t), f.newBuyer(t)
	p := f.publishedPost(t, storeID, "貼文")
	c := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(author), social.CommentInput{PostID: p.ID, Body: "原本的留言"}))
	e := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(author), social.ForumInput{StoreID: storeID, Body: "原本的討論"}))
	for _, target := range []social.Target{{Kind: social.TargetComment, ID: c.ID}, {Kind: social.TargetForumEntry, ID: e.ID}} {
		h, err := f.module.Hide(ctx, social.HideInput{Target: target, Category: "SPAM", Text: "廣告洗版", SourceRef: "case-1", AdminID: 9})
		must(t, err)
		if !h.Created {
			t.Fatal("第一次隱藏要是新增")
		}
	}
	mine := mustV[social.HiddenPage](t)(f.module.MyHiddenContent(ctx, author.BuyerID, nil, 10))
	if len(mine.Items) != 2 {
		t.Fatalf("作者讀得到兩則被隱藏的內容：%+v", mine)
	}
	for _, it := range mine.Items {
		if it.Category != "SPAM" || it.Text != "廣告洗版" || (it.Body != "原本的留言" && it.Body != "原本的討論") {
			t.Fatalf("作者讀得到內容與理由：%+v", it)
		}
	}
	if theirs := mustV[social.HiddenPage](t)(f.module.MyHiddenContent(ctx, other.BuyerID, nil, 10)); len(theirs.Items) != 0 {
		t.Fatalf("別人讀不到：%+v", theirs)
	}
	if _, err := f.module.UpdateComment(ctx, f.asBuyer(author), c.ID, 1, "想偷改"); kindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("被隱藏的留言不能編輯：%v", err)
	}
	if _, err := f.module.UpdateForumEntry(ctx, f.asBuyer(author), e.ID, 1, "想偷改"); kindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("被隱藏的討論區留言不能編輯：%v", err)
	}
	again, err := f.module.Hide(ctx, social.HideInput{Target: social.Target{Kind: social.TargetComment, ID: c.ID}, Category: "HATE", AdminID: 9})
	must(t, err)
	if again.Created || again.Category != "SPAM" {
		t.Fatalf("重複隱藏要回既有的：%+v", again)
	}
	if n := f.scalar(t, "SELECT count(*) FROM social.hides WHERE comment_id = $1", c.ID); n != 1 {
		t.Fatalf("同一則內容未還原的隱藏最多一筆：%d", n)
	}

	_, err = f.module.Restore(ctx, social.RestoreInput{Target: social.Target{Kind: social.TargetComment, ID: c.ID}, AdminID: 9, Reason: "誤判"})
	must(t, err)
	page := mustV[social.CommentPage](t)(f.module.ListComments(ctx, social.Caller{}, p.ID, nil, 10))
	if len(page.Comments) != 1 || page.Comments[0].Body != "原本的留言" {
		t.Fatalf("還原後公開列表含該留言：%+v", page)
	}
	if _, err := f.module.UpdateComment(ctx, f.asBuyer(author), c.ID, 1, "改好了"); err != nil {
		t.Fatalf("還原後可以編輯：%v", err)
	}
	if n := f.scalar(t, "SELECT count(*) FROM social.hides WHERE comment_id = $1 AND restored_at IS NOT NULL", c.ID); n != 1 {
		t.Fatal("還原留列不刪")
	}
	if _, err := f.module.Restore(ctx, social.RestoreInput{Target: social.Target{Kind: social.TargetComment, ID: c.ID}, AdminID: 9, Reason: "再還原"}); kindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("沒有未還原的隱藏時還原：%v", err)
	}
	if h, err := f.module.Hide(ctx, social.HideInput{Target: social.Target{Kind: social.TargetComment, ID: c.ID}, Category: "HATE", AdminID: 9}); err != nil || !h.Created {
		t.Fatalf("還原後可以再隱藏：%+v %v", h, err)
	}
}

// 隱藏只收買家寫的內容：店家身分的留言與貼文走內容下架措施；理由的類別要是社群內容可用的類別。
func TestHideRejectsStoreContentAndBadCategories(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	buyerP := f.newBuyer(t)
	p := f.publishedPost(t, storeID, "貼文")
	top := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(buyerP), social.CommentInput{PostID: p.ID, Body: "買家留言"}))
	reply := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asStore(t, storeID), social.CommentInput{PostID: p.ID, Body: "店家回覆", ParentID: &top.ID}))
	if _, err := f.module.Hide(ctx, social.HideInput{Target: social.Target{Kind: social.TargetComment, ID: reply.ID}, Category: "SPAM", AdminID: 1}); kindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("店家身分的留言不能隱藏：%v", err)
	}
	if _, err := f.module.Hide(ctx, social.HideInput{Target: social.Target{Kind: social.TargetPost, ID: p.ID}, Category: "SPAM", AdminID: 1}); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("貼文不能隱藏：%v", err)
	}
	for _, cat := range []string{"", "OTHER", "NOPE", "spam", "FOOD_SAFETY", "PROHIBITED_GOODS", "UNLICENSED_SALE", "FAKE_CERTIFICATION", "COUNTERFEIT"} {
		if _, err := f.module.Hide(ctx, social.HideInput{Target: social.Target{Kind: social.TargetComment, ID: top.ID}, Category: cat, AdminID: 1}); kindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("類別 %q：%v", cat, err)
		}
	}
	if _, err := f.module.Hide(ctx, social.HideInput{Target: social.Target{Kind: social.TargetComment, ID: top.ID}, Category: "PRIVACY", AdminID: 1}); err != nil {
		t.Fatalf("社群子集含對買家言論成立的類別：%v", err)
	}
}

// 頂層留言刪除或被隱藏而底下有可見的回覆時留佔位項目（只有 ID、時間、狀態），回覆照常顯示並計入；沒有可見回覆時整則不出現。
func TestPlaceholderForTopLevelCommentWithVisibleReplies(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	buyerP := f.newBuyer(t)
	p := f.publishedPost(t, storeID, "貼文")
	store := f.asStore(t, storeID)
	deleted := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(buyerP), social.CommentInput{PostID: p.ID, Body: "要被刪的"}))
	hidden := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(buyerP), social.CommentInput{PostID: p.ID, Body: "要被隱藏的"}))
	alone := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(buyerP), social.CommentInput{PostID: p.ID, Body: "沒人回覆又被刪"}))
	for _, parent := range []social.Comment{deleted, hidden} {
		mustV[social.Comment](t)(f.module.CreateComment(ctx, store, social.CommentInput{PostID: p.ID, Body: "回覆", ParentID: &parent.ID}))
	}
	must(t, f.module.DeleteComment(ctx, f.asBuyer(buyerP), deleted.ID, 0))
	must(t, f.module.DeleteComment(ctx, f.asBuyer(buyerP), alone.ID, 0))
	_, err := f.module.Hide(ctx, social.HideInput{Target: social.Target{Kind: social.TargetComment, ID: hidden.ID}, Category: "SPAM", AdminID: 1})
	must(t, err)

	page := mustV[social.CommentPage](t)(f.module.ListComments(ctx, social.Caller{}, p.ID, nil, 10))
	if len(page.Comments) != 2 {
		t.Fatalf("兩個佔位項目，沒有回覆的那則不出現：%+v", page.Comments)
	}
	states := map[string]social.CommentState{}
	for _, c := range page.Comments {
		if c.Body != "" || c.Actor != (social.Actor{}) {
			t.Fatalf("佔位項目沒有內文與作者：%+v", c)
		}
		states[c.ID.String()] = c.State
		if c.ReplyCount != 1 {
			t.Fatalf("佔位項目帶回覆數：%+v", c)
		}
	}
	if states[deleted.ID.String()] != social.CommentDeleted || states[hidden.ID.String()] != social.CommentHidden {
		t.Fatalf("狀態：%v", states)
	}
	for _, c := range page.Comments {
		replies := mustV[social.CommentPage](t)(f.module.ListReplies(ctx, social.Caller{}, c.PostID, c.ID, nil, 10))
		if len(replies.Comments) != 1 || replies.Comments[0].Body != "回覆" {
			t.Fatalf("回覆照常顯示：%+v", replies)
		}
	}
	got := mustV[social.Post](t)(f.module.PublicPost(ctx, social.Caller{}, p.ID))
	if got.Stats.Comments != 2 {
		t.Fatalf("留言數含回覆、不含不計入的頂層：%d", got.Stats.Comments)
	}
	for _, c := range page.Comments {
		replies := mustV[social.CommentPage](t)(f.module.ListReplies(ctx, social.Caller{}, c.PostID, c.ID, nil, 10))
		must(t, f.module.DeleteComment(ctx, store, replies.Comments[0].ID, 0))
	}
	if page := mustV[social.CommentPage](t)(f.module.ListComments(ctx, social.Caller{}, p.ID, nil, 10)); len(page.Comments) != 0 {
		t.Fatalf("沒有可見的回覆時整則不出現：%+v", page.Comments)
	}
}

// 空的討論區頁也要帶這家店的討論區此刻可不可以讀寫（viewer_can_comment 的來源）：可曝光是 true，不可曝光是 false。
func TestForumPageReadableOnAnEmptyPage(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	page, err := f.module.ListForumEntries(ctx, social.Caller{}, storeID, nil, 50)
	must(t, err)
	if len(page.Entries) != 0 || !page.Readable {
		t.Fatalf("可曝光的店的空頁：entries=%d readable=%v，預期 0 與 true", len(page.Entries), page.Readable)
	}
	f.setExposable(t, storeID, false)
	page, err = f.module.ListForumEntries(ctx, social.Caller{}, storeID, nil, 50)
	must(t, err)
	if page.Readable {
		t.Fatal("店家不可曝光的空頁 Readable 應是 false")
	}
}

// Restore 帶來源參照（留言、討論區留言）：沒有未還原的隱藏與隱藏是別案造成的是兩種不同的 FailedPrecondition，後者不動隱藏。
// 殺死的變異：Restore 忽略 SourceRef；兩種情況回同一句。
func TestRestoreSourceMismatchIsDistinctFromNotHidden(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	author := f.newBuyer(t)
	p := f.publishedPost(t, storeID, "貼文")
	c := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(author), social.CommentInput{PostID: p.ID, Body: "留言"}))
	e := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(author), social.ForumInput{StoreID: storeID, Body: "討論"}))
	for _, target := range []social.Target{{Kind: social.TargetComment, ID: c.ID}, {Kind: social.TargetForumEntry, ID: e.ID}} {
		restore := func(ref string) error {
			_, err := f.module.Restore(ctx, social.RestoreInput{Target: target, AdminID: 9, Reason: "誤判", SourceRef: ref})
			return err
		}
		if err := restore("case-1"); kindOf(err) != apperr.KindFailedPrecondition || !strings.Contains(err.Error(), "沒有被隱藏") {
			t.Fatalf("%v 沒有隱藏：%v", target.Kind, err)
		}
		mustV[social.Hide](t)(f.module.Hide(ctx, social.HideInput{Target: target, Category: "SPAM", Text: "廣告", SourceRef: "case-1", AdminID: 9}))
		if err := restore("case-2"); kindOf(err) != apperr.KindFailedPrecondition || !strings.Contains(err.Error(), "不是由這個案件造成") {
			t.Fatalf("%v 來源不符：%v", target.Kind, err)
		}
		if n := f.scalar(t, "SELECT count(*) FROM social.hides WHERE (comment_id = $1 OR forum_entry_id = $1) AND restored_at IS NULL", target.ID); n != 1 {
			t.Fatalf("%v 來源不符時隱藏仍在：%d", target.Kind, n)
		}
		if err := restore("case-1"); err != nil {
			t.Fatalf("%v 來源相符可還原：%v", target.Kind, err)
		}
	}
}
