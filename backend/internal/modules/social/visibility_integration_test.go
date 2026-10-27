//go:build integration

package social_test

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// readable 是匿名讀者此刻看得到的：貼文、貼文底下的頂層留言數、討論區的頂層留言數、動態牆有沒有這篇。
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
	if page, err := f.module.ListComments(ctx, social.Caller{}, postID, nil, 50); err == nil {
		r.comments = len(page.Comments)
	} else if kindOf(err) != apperr.KindNotFound {
		t.Fatal(err)
	}
	forum, err := f.module.ListForumEntries(ctx, social.Caller{}, storeID, nil, 50)
	must(t, err)
	r.forum = len(forum.Entries)
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
		want   readable // 套用之後
	}
	all := readable{post: true, comments: 1, forum: 1, feed: true}
	scenarios := []scenario{
		{name: "下架", want: readable{forum: 1},
			apply: func(t *testing.T, f *fixture, p social.Post, _ social.Comment, _ social.ForumEntry, _ buyerP) {
				_, err := f.module.UnpublishPost(ctx, f.store(t, p.StoreID), p.ID, p.Version)
				must(t, err)
			},
			revert: func(t *testing.T, f *fixture, p social.Post, _ social.Comment, _ social.ForumEntry, _ buyerP) {
				cur := mustV[social.Post](t)(f.module.OwnPost(ctx, p.StoreID, p.ID))
				_, err := f.module.PublishPost(ctx, f.store(t, p.StoreID), p.ID, cur.Version)
				must(t, err)
			}},
		{name: "貼文刪除", want: readable{forum: 1},
			apply: func(t *testing.T, f *fixture, p social.Post, _ social.Comment, _ social.ForumEntry, _ buyerP) {
				must(t, f.module.DeletePost(ctx, f.store(t, p.StoreID), p.ID, 0))
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
	p := mustV[social.Post](t)(f.module.CreatePost(ctx, f.store(t, storeID), social.CreatePostInput{Body: "草稿"}))
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
	if _, err := f.module.UpdateComment(ctx, f.asBuyer(author), c.ID, "想偷改"); kindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("被隱藏的留言不能編輯：%v", err)
	}
	if _, err := f.module.UpdateForumEntry(ctx, f.asBuyer(author), e.ID, "想偷改"); kindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("被隱藏的討論區留言不能編輯：%v", err)
	}
	// 冪等：同一則再隱藏回既有的那一筆。
	again, err := f.module.Hide(ctx, social.HideInput{Target: social.Target{Kind: social.TargetComment, ID: c.ID}, Category: "HATE", AdminID: 9})
	must(t, err)
	if again.Created || again.Category != "SPAM" {
		t.Fatalf("重複隱藏要回既有的：%+v", again)
	}
	if n := f.scalar(t, "SELECT count(*) FROM social.hides WHERE comment_id = $1", c.ID); n != 1 {
		t.Fatalf("同一則內容未還原的隱藏最多一筆：%d", n)
	}

	// 管理員更正：還原後公開列表含該留言，作者可以編輯，隱藏紀錄留著；之後可以再隱藏。
	_, err = f.module.Restore(ctx, social.RestoreInput{Target: social.Target{Kind: social.TargetComment, ID: c.ID}, AdminID: 9, Reason: "誤判"})
	must(t, err)
	page := mustV[social.CommentPage](t)(f.module.ListComments(ctx, social.Caller{}, p.ID, nil, 10))
	if len(page.Comments) != 1 || page.Comments[0].Body != "原本的留言" {
		t.Fatalf("還原後公開列表含該留言：%+v", page)
	}
	if _, err := f.module.UpdateComment(ctx, f.asBuyer(author), c.ID, "改好了"); err != nil {
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
	must(t, f.module.DeleteComment(ctx, f.asBuyer(buyerP), deleted.ID))
	must(t, f.module.DeleteComment(ctx, f.asBuyer(buyerP), alone.ID))
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
		replies := mustV[social.CommentPage](t)(f.module.ListReplies(ctx, social.Caller{}, c.ID, nil, 10))
		if len(replies.Comments) != 1 || replies.Comments[0].Body != "回覆" {
			t.Fatalf("回覆照常顯示：%+v", replies)
		}
	}
	// 回覆照常計入：3 則頂層（兩則回覆是計入的，被刪與被隱藏的頂層不計入）。
	got := mustV[social.Post](t)(f.module.PublicPost(ctx, social.Caller{}, p.ID))
	if got.Stats.Comments != 2 {
		t.Fatalf("留言數含回覆、不含不計入的頂層：%d", got.Stats.Comments)
	}
	// 回覆也被刪掉之後，整則（含佔位項目）不再出現。
	for _, c := range page.Comments {
		replies := mustV[social.CommentPage](t)(f.module.ListReplies(ctx, social.Caller{}, c.ID, nil, 10))
		must(t, f.module.DeleteComment(ctx, store, replies.Comments[0].ID))
	}
	if page := mustV[social.CommentPage](t)(f.module.ListComments(ctx, social.Caller{}, p.ID, nil, 10)); len(page.Comments) != 0 {
		t.Fatalf("沒有可見的回覆時整則不出現：%+v", page.Comments)
	}
}
