//go:build integration

package social_test

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// 封鎖的讀寫語意（A7-1b，block-brief.md）：個人封鎖依觀看者過濾讀取（買家↔買家雙向，買家→店家），店家封鎖買家只擋寫入；
// 計數不依封鎖；匿名與店主以店家身分的讀取不受影響。

func (f *fixture) blockBuyer(t *testing.T, blocker, target buyerP) {
	t.Helper()
	changed, err := f.buyers.BlockBuyer(context.Background(), blocker.BuyerID, target.BuyerID)
	if err != nil || !changed {
		t.Fatalf("封鎖買家：%v %v", changed, err)
	}
}

func (f *fixture) blockStore(t *testing.T, blocker buyerP, storeID int64) {
	t.Helper()
	changed, err := f.buyers.BlockStore(context.Background(), blocker.BuyerID, storeID)
	if err != nil || !changed {
		t.Fatalf("封鎖店家：%v %v", changed, err)
	}
}

func (f *fixture) unblockStore(t *testing.T, blocker buyerP, storeID int64) {
	t.Helper()
	if _, err := f.buyers.UnblockStore(context.Background(), blocker.BuyerID, storeID); err != nil {
		t.Fatal(err)
	}
}

// storeBlocksBuyer 讓店家封鎖買家（直接寫 seller 的表，最小的列；seller 自己的規則在 seller 的測試）。
func (f *fixture) storeBlocksBuyer(t *testing.T, storeID int64, b buyerP) {
	t.Helper()
	f.exec(t, "INSERT INTO seller.store_blocks (store_id, buyer_id, created_at) VALUES ($1, $2, $3)", storeID, b.BuyerID, f.clock())
}

func commentIDs(p social.CommentPage) []uuid.UUID {
	out := make([]uuid.UUID, len(p.Comments))
	for i, c := range p.Comments {
		out[i] = c.ID
	}
	return out
}

func forumIDs(p social.ForumPage) []uuid.UUID {
	out := make([]uuid.UUID, len(p.Entries))
	for i, e := range p.Entries {
		out[i] = e.ID
	}
	return out
}

func postIDs(p social.PostPage) []uuid.UUID {
	out := make([]uuid.UUID, len(p.Posts))
	for i, x := range p.Posts {
		out[i] = x.ID
	}
	return out
}

func sameIDs(got []uuid.UUID, want ...uuid.UUID) bool { return slices.Equal(got, want) }

func (f *fixture) topLevel(t *testing.T, c social.Caller, postID uuid.UUID) social.CommentPage {
	t.Helper()
	return mustV[social.CommentPage](t)(f.module.ListComments(context.Background(), c, postID, nil, 50))
}

func (f *fixture) replies(t *testing.T, c social.Caller, parentID uuid.UUID) social.CommentPage {
	t.Helper()
	return mustV[social.CommentPage](t)(f.module.ListReplies(context.Background(), c, parentID, nil, 50))
}

func (f *fixture) forumTop(t *testing.T, c social.Caller, storeID int64) social.ForumPage {
	t.Helper()
	return mustV[social.ForumPage](t)(f.module.ListForumEntries(context.Background(), c, storeID, nil, 50))
}

func (f *fixture) forumReplies(t *testing.T, c social.Caller, parentID uuid.UUID) social.ForumPage {
	t.Helper()
	return mustV[social.ForumPage](t)(f.module.ListForumReplies(context.Background(), c, parentID, nil, 50))
}

// D2：買家↔買家雙向互不可見。留言：被封鎖者的頂層連同底下的回覆，對封鎖者整則不出現、不留佔位；被封鎖者看不到封鎖者的留言；
// 第三人與匿名都看得到全部。
func TestBlockedBuyersCommentsAreHiddenInBothDirections(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	a, b, c := f.newBuyer(t), f.newBuyer(t), f.newBuyer(t)
	p := f.publishedPost(t, storeID, "貼文")
	store := f.asStore(t, storeID)
	cA := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(a), social.CommentInput{PostID: p.ID, Body: "A"}))
	cB := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: "B"}))
	cC := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(c), social.CommentInput{PostID: p.ID, Body: "C"}))
	rB := mustV[social.Comment](t)(f.module.CreateComment(ctx, store, social.CommentInput{PostID: p.ID, Body: "回 B", ParentID: &cB.ID}))
	rC := mustV[social.Comment](t)(f.module.CreateComment(ctx, store, social.CommentInput{PostID: p.ID, Body: "回 C", ParentID: &cC.ID}))

	f.blockBuyer(t, a, b) // A 封鎖 B（單向發起，效果雙向）

	// 封鎖者 A：B 的頂層連同回覆整則不出現，沒有佔位項目。
	got := f.topLevel(t, f.asBuyer(a), p.ID)
	if !sameIDs(commentIDs(got), cA.ID, cC.ID) {
		t.Fatalf("被封鎖者的頂層連同回覆整則不出現、不留佔位：%v", commentIDs(got))
	}
	for _, x := range got.Comments {
		if x.State != social.CommentVisible {
			t.Fatalf("不該有佔位項目：%+v", x)
		}
	}
	// 帶被封鎖者的頂層 ID 讀不到回覆；別串的回覆照常。
	if got := f.replies(t, f.asBuyer(a), cB.ID); len(got.Comments) != 0 {
		t.Fatalf("帶被封鎖者的頂層 ID 讀不到回覆：%v", commentIDs(got))
	}
	if got := f.replies(t, f.asBuyer(a), cC.ID); !sameIDs(commentIDs(got), rC.ID) {
		t.Fatalf("別串的回覆照常：%v", commentIDs(got))
	}
	// 被封鎖者 B：看不到 A 的留言（雙向互不可見）。
	if got := f.topLevel(t, f.asBuyer(b), p.ID); !sameIDs(commentIDs(got), cB.ID, cC.ID) {
		t.Fatalf("被封鎖者看不到封鎖者的留言：%v", commentIDs(got))
	}
	// 第三人與匿名看得到全部。
	for name, viewer := range map[string]social.Caller{"第三人": f.asBuyer(c), "匿名": {}} {
		if got := f.topLevel(t, viewer, p.ID); !sameIDs(commentIDs(got), cA.ID, cB.ID, cC.ID) {
			t.Fatalf("%s看得到全部：%v", name, commentIDs(got))
		}
		if got := f.replies(t, viewer, cB.ID); !sameIDs(commentIDs(got), rB.ID) {
			t.Fatalf("%s看得到被封鎖者那串的回覆：%v", name, commentIDs(got))
		}
	}
	// 封鎖者不能對看不到的留言按讚；看得到的可以。
	if _, err := f.module.LikeComment(ctx, f.asBuyer(a), cB.ID); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("讀不到的留言不能按讚：%v", err)
	}
	if added, err := f.module.LikeComment(ctx, f.asBuyer(a), cC.ID); err != nil || !added {
		t.Fatalf("看得到的留言可以按讚：%v %v", added, err)
	}
}

// D2：討論區同樣雙向互不可見（含買家的回覆）。已刪除的頂層只剩被封鎖者的回覆時整則不出現；其他人仍看到佔位項目加回覆。
func TestBlockedBuyersForumEntriesAreHiddenInBothDirections(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	a, b, c := f.newBuyer(t), f.newBuyer(t), f.newBuyer(t)
	enter := func(who buyerP, in social.ForumInput) social.ForumEntry {
		in.StoreID = storeID
		return mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(who), in))
	}
	eA := enter(a, social.ForumInput{Body: "A"})
	eB := enter(b, social.ForumInput{Body: "B"})
	eC := enter(c, social.ForumInput{Body: "C"})
	storeReply := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asStore(t, storeID), social.ForumInput{StoreID: storeID, Body: "店家回 B", ParentID: &eB.ID}))
	bReplyToC := enter(b, social.ForumInput{Body: "B 回 C", ParentID: &eC.ID})
	// 已刪除的頂層 D，底下只剩 B 的回覆。
	eD := enter(c, social.ForumInput{Body: "D"})
	bReplyToD := enter(b, social.ForumInput{Body: "B 回 D", ParentID: &eD.ID})
	must(t, f.module.DeleteForumEntry(ctx, f.asBuyer(c), eD.ID))

	f.blockBuyer(t, a, b)

	got := f.forumTop(t, f.asBuyer(a), storeID)
	if !sameIDs(forumIDs(got), eA.ID, eC.ID) {
		t.Fatalf("被封鎖者的頂層連同回覆整則不出現；已刪除的頂層只剩被封鎖者的回覆時也不出現：%v", forumIDs(got))
	}
	if got := f.forumReplies(t, f.asBuyer(a), eB.ID); len(got.Entries) != 0 {
		t.Fatalf("帶被封鎖者的頂層 ID 讀不到回覆：%v", forumIDs(got))
	}
	if got := f.forumReplies(t, f.asBuyer(a), eC.ID); len(got.Entries) != 0 {
		t.Fatalf("同一串裡被封鎖者的回覆不出現：%v", forumIDs(got))
	}
	// 被封鎖者 B：看不到 A 的留言，其餘照常。
	if got := f.forumTop(t, f.asBuyer(b), storeID); !sameIDs(forumIDs(got), eB.ID, eC.ID, eD.ID) {
		t.Fatalf("被封鎖者看不到封鎖者的留言：%v", forumIDs(got))
	}
	// 第三人與匿名：全部，含已刪除頂層的佔位項目與底下 B 的回覆。
	for name, viewer := range map[string]social.Caller{"第三人": f.asBuyer(c), "匿名": {}} {
		got := f.forumTop(t, viewer, storeID)
		if !sameIDs(forumIDs(got), eA.ID, eB.ID, eC.ID, eD.ID) {
			t.Fatalf("%s看得到全部（含佔位）：%v", name, forumIDs(got))
		}
		if got.Entries[3].State != social.CommentDeleted {
			t.Fatalf("%s：已刪除的頂層是佔位項目：%+v", name, got.Entries[3])
		}
		if got := f.forumReplies(t, viewer, eD.ID); !sameIDs(forumIDs(got), bReplyToD.ID) {
			t.Fatalf("%s看得到佔位底下的回覆：%v", name, forumIDs(got))
		}
		if got := f.forumReplies(t, viewer, eB.ID); !sameIDs(forumIDs(got), storeReply.ID) {
			t.Fatalf("%s看得到被封鎖者那串的回覆：%v", name, forumIDs(got))
		}
	}
	// 讀不到的留言不能回覆（讀寫對「看得到」的定義一致）。
	if _, err := f.module.CreateForumEntry(ctx, f.asBuyer(a), social.ForumInput{StoreID: storeID, Body: "x", ParentID: &eB.ID}); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("讀不到的留言不能回覆：%v", err)
	}
	if _, err := f.module.CreateForumEntry(ctx, f.asBuyer(a), social.ForumInput{StoreID: storeID, Body: "x", ParentID: &eC.ID, ReplyToID: &bReplyToC.ID}); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("reply_to 是被封鎖者的回覆時，比照目標已刪除：%v", err)
	}
	if _, err := f.module.CreateForumEntry(ctx, f.asBuyer(a), social.ForumInput{StoreID: storeID, Body: "回 C", ParentID: &eC.ID}); err != nil {
		t.Fatalf("看得到的串可以回覆：%v", err)
	}
}

// 已刪除的頂層留言只剩被封鎖者的回覆時，對封鎖者整則不出現（留言的回覆原本只限店家，這裡直接寫入一則買家的回覆來量判斷）。
func TestDeletedTopLevelCommentWithOnlyBlockedRepliesDisappears(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	a, b, c := f.newBuyer(t), f.newBuyer(t), f.newBuyer(t)
	p := f.publishedPost(t, storeID, "貼文")
	top := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(c), social.CommentInput{PostID: p.ID, Body: "C"}))
	f.exec(t, `INSERT INTO social.comments (id, post_id, parent_id, reply_to_id, actor_kind, actor_id, body, created_at, updated_at)
		VALUES ($1, $2, $3, $3, 'BUYER', $4, 'B 的回覆', $5, $5)`, uuidNew(), p.ID, top.ID, b.BuyerID, f.clock())
	must(t, f.module.DeleteComment(ctx, f.asBuyer(c), top.ID))

	if got := f.topLevel(t, f.asBuyer(a), p.ID); len(got.Comments) != 1 || got.Comments[0].State != social.CommentDeleted {
		t.Fatalf("沒有封鎖時，已刪除的頂層因為有可見的回覆而是佔位項目：%+v", got.Comments)
	}
	f.blockBuyer(t, a, b)
	if got := f.topLevel(t, f.asBuyer(a), p.ID); len(got.Comments) != 0 {
		t.Fatalf("只剩被封鎖者的回覆時整則不出現：%+v", got.Comments)
	}
}

// D2：買家封鎖店家，看不到該店的貼文、留言與討論區（單向：店家那邊不受影響）；也不能對它寫入。解除後恢復。
func TestBuyerBlockingAStoreHidesItsContentAndStopsWrites(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	blockedStore, _ := f.exposableStore(t)
	otherStore, _ := f.exposableStore(t)
	a, c := f.newBuyer(t), f.newBuyer(t)
	pBlocked := f.publishedPost(t, blockedStore, "被封鎖的店", "organic")
	pOther := f.publishedPost(t, otherStore, "別家店", "organic")
	cmt := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(c), social.CommentInput{PostID: pBlocked.ID, Body: "留言"}))
	entry := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(c), social.ForumInput{StoreID: blockedStore, Body: "討論"}))
	reply := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asStore(t, blockedStore), social.ForumInput{StoreID: blockedStore, Body: "店家回", ParentID: &entry.ID}))
	mustV[bool](t)(f.module.LikePost(ctx, a.BuyerID, pBlocked.ID)) // 封鎖之前按的讚

	f.blockStore(t, a, blockedStore)
	viewer := f.asBuyer(a)

	if _, err := f.module.PublicPost(ctx, viewer, pBlocked.ID); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("單篇讀不到：%v", err)
	}
	if got := mustV[social.Post](t)(f.module.PublicPost(ctx, viewer, pOther.ID)); got.ID != pOther.ID {
		t.Fatal("別家店的貼文照常")
	}
	if feed := mustV[social.PostPage](t)(f.module.ListFeed(ctx, viewer, nil, 50)); !slices.Contains(postIDs(feed), pOther.ID) || slices.Contains(postIDs(feed), pBlocked.ID) {
		t.Fatalf("動態牆不含被封鎖的店：%v", postIDs(feed))
	}
	if page := mustV[social.PostPage](t)(f.module.ListStorePosts(ctx, viewer, blockedStore, nil, 50)); len(page.Posts) != 0 {
		t.Fatalf("店家頁是空的：%v", postIDs(page))
	}
	if page := mustV[social.PostPage](t)(f.module.ListHashtagPosts(ctx, viewer, "organic", nil, 50)); !sameIDs(postIDs(page), pOther.ID) {
		t.Fatalf("標籤的貼文不含被封鎖的店：%v", postIDs(page))
	}
	if top := mustV[[]social.Post](t)(f.module.TopPosts(ctx, viewer, 30)); len(top) != 1 || top[0].ID != pOther.ID {
		t.Fatalf("本週主題不含被封鎖的店：%v", top)
	}
	if _, err := f.module.ListComments(ctx, viewer, pBlocked.ID, nil, 50); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("被封鎖的店的貼文的留言讀不到：%v", err)
	}
	if got := f.forumTop(t, viewer, blockedStore); len(got.Entries) != 0 {
		t.Fatalf("討論區是空的：%v", forumIDs(got))
	}
	if got := f.forumReplies(t, viewer, entry.ID); len(got.Entries) != 0 {
		t.Fatalf("討論區的回覆也讀不到：%v", forumIDs(got))
	}
	// 寫入：讀不到就不能寫。
	if _, err := f.module.CreateComment(ctx, viewer, social.CommentInput{PostID: pBlocked.ID, Body: "x"}); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("不能留言：%v", err)
	}
	if _, err := f.module.CreateForumEntry(ctx, viewer, social.ForumInput{StoreID: blockedStore, Body: "x"}); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("不能在討論區發言：%v", err)
	}
	if _, err := f.module.LikeComment(ctx, viewer, cmt.ID); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("不能對留言按讚：%v", err)
	}
	if _, err := f.module.RecordShare(ctx, a.BuyerID, pBlocked.ID); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("不能分享讀不到的貼文：%v", err)
	}
	// 收回自己的讚不需要讀得到。
	if removed, err := f.module.UnlikePost(ctx, a.BuyerID, pBlocked.ID); err != nil || !removed {
		t.Fatalf("取消讚：%v %v", removed, err)
	}
	// 第三人與匿名不受影響。
	for name, who := range map[string]social.Caller{"第三人": f.asBuyer(c), "匿名": {}} {
		if feed := mustV[social.PostPage](t)(f.module.ListFeed(ctx, who, nil, 50)); len(feed.Posts) != 2 || !slices.Contains(postIDs(feed), pBlocked.ID) || !slices.Contains(postIDs(feed), pOther.ID) {
			t.Fatalf("%s的動態牆含兩家店：%v", name, postIDs(feed))
		}
		if got := f.forumReplies(t, who, entry.ID); !sameIDs(forumIDs(got), reply.ID) {
			t.Fatalf("%s看得到討論區的回覆：%v", name, forumIDs(got))
		}
	}
	// 解除後恢復。
	f.unblockStore(t, a, blockedStore)
	if _, err := f.module.PublicPost(ctx, viewer, pBlocked.ID); err != nil {
		t.Fatalf("解除後讀得到：%v", err)
	}
	if got := f.forumTop(t, viewer, blockedStore); len(got.Entries) != 1 {
		t.Fatalf("解除後討論區恢復：%v", forumIDs(got))
	}
}

// D2：店家封鎖買家只擋寫入，被封鎖的買家仍看得到店家的公開內容（單向，不是雙向）。
func TestStoreBlockingABuyerLeavesPublicContentReadable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	b, c := f.newBuyer(t), f.newBuyer(t)
	p := f.publishedPost(t, storeID, "貼文", "organic")
	store := f.asStore(t, storeID)
	mine := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: "封鎖之前的留言"}))
	reply := mustV[social.Comment](t)(f.module.CreateComment(ctx, store, social.CommentInput{PostID: p.ID, Body: "店家回覆", ParentID: &mine.ID}))
	entry := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(b), social.ForumInput{StoreID: storeID, Body: "封鎖之前的發言"}))
	f.storeBlocksBuyer(t, storeID, b)
	viewer := f.asBuyer(b)

	// 仍看得到店家的公開內容。
	if _, err := f.module.PublicPost(ctx, viewer, p.ID); err != nil {
		t.Fatalf("被封鎖的買家仍讀得到貼文：%v", err)
	}
	if feed := mustV[social.PostPage](t)(f.module.ListFeed(ctx, viewer, nil, 50)); !sameIDs(postIDs(feed), p.ID) {
		t.Fatalf("動態牆仍有這篇：%v", postIDs(feed))
	}
	if got := f.topLevel(t, viewer, p.ID); !sameIDs(commentIDs(got), mine.ID) {
		t.Fatalf("被封鎖的買家仍看得到留言：%v", commentIDs(got))
	}
	if got := f.replies(t, viewer, mine.ID); !sameIDs(commentIDs(got), reply.ID) {
		t.Fatalf("被封鎖的買家仍看得到店家的回覆：%v", commentIDs(got))
	}
	if got := f.forumTop(t, viewer, storeID); !sameIDs(forumIDs(got), entry.ID) {
		t.Fatalf("被封鎖的買家仍看得到討論區：%v", forumIDs(got))
	}
	// 店家那邊也一樣看得到他的內容（店家不受個人封鎖）。
	if got := f.topLevel(t, f.asStore(t, storeID), p.ID); !sameIDs(commentIDs(got), mine.ID) {
		t.Fatalf("店主看得到：%v", commentIDs(got))
	}

	// 寫入：每個入口一個子測試。
	denied := map[string]func() error{
		"留言": func() error {
			_, err := f.module.CreateComment(ctx, viewer, social.CommentInput{PostID: p.ID, Body: "x"})
			return err
		},
		"留言的編輯": func() error {
			_, err := f.module.UpdateComment(ctx, viewer, mine.ID, "改")
			return err
		},
		"討論區的發言": func() error {
			_, err := f.module.CreateForumEntry(ctx, viewer, social.ForumInput{StoreID: storeID, Body: "x"})
			return err
		},
		"討論區的回覆": func() error {
			_, err := f.module.CreateForumEntry(ctx, viewer, social.ForumInput{StoreID: storeID, Body: "x", ParentID: &entry.ID})
			return err
		},
		"討論區的編輯": func() error {
			_, err := f.module.UpdateForumEntry(ctx, viewer, entry.ID, "改")
			return err
		},
		"貼文讚": func() error {
			_, err := f.module.LikePost(ctx, b.BuyerID, p.ID)
			return err
		},
		"留言讚": func() error {
			_, err := f.module.LikeComment(ctx, viewer, mine.ID)
			return err
		},
	}
	for name, do := range denied {
		t.Run("擋"+name, func(t *testing.T) {
			err := do()
			if kindOf(err) != apperr.KindPermissionDenied {
				t.Fatalf("要 PermissionDenied：%v", err)
			}
		})
	}
	// 沒被擋的買家（第三人）同樣的動作成功。
	if _, err := f.module.CreateComment(ctx, f.asBuyer(c), social.CommentInput{PostID: p.ID, Body: "第三人"}); err != nil {
		t.Fatalf("第三人可以留言：%v", err)
	}
	// 店家自己照常寫入。
	if _, err := f.module.CreateComment(ctx, store, social.CommentInput{PostID: p.ID, Body: "店家", ParentID: &mine.ID}); err != nil {
		t.Fatalf("店家可以回覆：%v", err)
	}
	// 不擋：取消讚、分享、瀏覽、刪除自己的內容。
	t.Run("不擋取消讚", func(t *testing.T) {
		other := f.publishedPost(t, storeID, "另一篇")
		f.exec(t, "INSERT INTO social.post_likes (post_id, buyer_id, created_at) VALUES ($1, $2, $3)", other.ID, b.BuyerID, f.clock())
		if removed, err := f.module.UnlikePost(ctx, b.BuyerID, other.ID); err != nil || !removed {
			t.Fatalf("%v %v", removed, err)
		}
	})
	t.Run("不擋分享與瀏覽", func(t *testing.T) {
		if added, err := f.module.RecordShare(ctx, b.BuyerID, p.ID); err != nil || !added {
			t.Fatalf("分享：%v %v", added, err)
		}
		if counted, err := f.module.RecordView(ctx, p.ID, social.Viewer{AccountID: 1}); err != nil || !counted {
			t.Fatalf("瀏覽：%v %v", counted, err)
		}
	})
	t.Run("不擋刪除自己的內容", func(t *testing.T) {
		must(t, f.module.DeleteComment(ctx, viewer, mine.ID))
		must(t, f.module.DeleteForumEntry(ctx, viewer, entry.ID))
	})
}

// 店主以店家身分讀自己的店，不套個人封鎖：店主（同時是買家）封鎖了一位買家，或被一位買家封鎖，他以店家身分仍看得到那位買家的留言與討論區留言。
func TestStoreOwnerReadsOwnStoreIgnoringPersonalBlocks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	owner, blockedByOwner, blocksOwner := f.newBuyer(t), f.newBuyer(t), f.newBuyer(t)
	p := f.publishedPost(t, storeID, "貼文")
	s := f.store(t, storeID)
	ownerCaller := social.Caller{BuyerID: owner.BuyerID, Store: &s}
	c1 := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(blockedByOwner), social.CommentInput{PostID: p.ID, Body: "被店主封鎖的買家"}))
	c2 := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(blocksOwner), social.CommentInput{PostID: p.ID, Body: "封鎖店主的買家"}))
	r1 := mustV[social.Comment](t)(f.module.CreateComment(ctx, ownerCaller, social.CommentInput{PostID: p.ID, Body: "店家回覆", ParentID: &c1.ID}))
	e1 := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(blockedByOwner), social.ForumInput{StoreID: storeID, Body: "討論一"}))
	e2 := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(blocksOwner), social.ForumInput{StoreID: storeID, Body: "討論二"}))
	f.blockBuyer(t, owner, blockedByOwner)
	f.blockBuyer(t, blocksOwner, owner)

	if got := f.topLevel(t, ownerCaller, p.ID); !sameIDs(commentIDs(got), c1.ID, c2.ID) {
		t.Fatalf("店主仍看得到封鎖自己的買家與被自己封鎖的買家的留言：%v", commentIDs(got))
	}
	if got := f.replies(t, ownerCaller, c1.ID); !sameIDs(commentIDs(got), r1.ID) {
		t.Fatalf("店主仍讀得到回覆：%v", commentIDs(got))
	}
	if got := f.forumTop(t, ownerCaller, storeID); !sameIDs(forumIDs(got), e1.ID, e2.ID) {
		t.Fatalf("店主仍看得到討論區：%v", forumIDs(got))
	}
	// 同一個人以買家身分讀（沒有以店家身分）才套個人封鎖。
	if got := f.topLevel(t, f.asBuyer(owner), p.ID); len(got.Comments) != 0 {
		// 作為買家的店主封鎖了 blockedByOwner、被 blocksOwner 封鎖，兩則留言都看不到。
		t.Fatalf("以買家身分讀才套個人封鎖：%v", commentIDs(got))
	}
	// 店家身分回覆被個人封鎖的買家的留言：寫入前置檢查不套個人封鎖。
	if _, err := f.module.CreateComment(ctx, ownerCaller, social.CommentInput{PostID: p.ID, Body: "再回", ParentID: &c1.ID, ReplyToID: &c1.ID}); err != nil {
		t.Fatalf("店家仍可回覆被個人封鎖的買家：%v", err)
	}
	if _, err := f.module.CreateForumEntry(ctx, ownerCaller, social.ForumInput{StoreID: storeID, Body: "店家回", ParentID: &e1.ID}); err != nil {
		t.Fatalf("店家仍可在討論區回覆：%v", err)
	}
}

// SD15：封鎖過濾在分頁之前。被排除的列不佔名額：每頁筆數是 size，keyset 跨頁不重不漏。
func TestBlockFilterRunsBeforePagination(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	blockedStoreID, _ := f.exposableStore(t)
	a, b, c := f.newBuyer(t), f.newBuyer(t), f.newBuyer(t)
	p := f.publishedPost(t, storeID, "貼文")
	var wantComments, wantForum []uuid.UUID
	var wantPosts []uuid.UUID
	for i := range 6 {
		author := c
		if i%2 == 0 {
			author = b // 被封鎖者的留言夾在中間
		}
		f.advance(time.Second)
		cm := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(author), social.CommentInput{PostID: p.ID, Body: fmt.Sprintf("留言%d", i)}))
		en := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(author), social.ForumInput{StoreID: storeID, Body: fmt.Sprintf("討論%d", i)}))
		if author == c {
			wantComments, wantForum = append(wantComments, cm.ID), append(wantForum, en.ID)
		}
	}
	for i := range 6 {
		sid := storeID
		if i%2 == 0 {
			sid = blockedStoreID
		}
		f.advance(time.Second)
		post := f.publishedPost(t, sid, fmt.Sprintf("貼文%d", i))
		if sid == storeID {
			wantPosts = append([]uuid.UUID{post.ID}, wantPosts...) // 最新的在前
		}
	}
	f.blockBuyer(t, a, b)
	f.blockStore(t, a, blockedStoreID)
	viewer := f.asBuyer(a)

	var gotComments, gotForum, gotPosts []uuid.UUID
	var cur *social.CommentCursor
	for pages := 0; ; pages++ {
		pg := mustV[social.CommentPage](t)(f.module.ListComments(ctx, viewer, p.ID, cur, 2))
		if pages == 0 && len(pg.Comments) != 2 {
			t.Fatalf("第一頁要有 2 筆（被排除的不佔名額）：%v", commentIDs(pg))
		}
		gotComments = append(gotComments, commentIDs(pg)...)
		if cur = pg.Next; cur == nil {
			break
		}
	}
	var fcur *social.CommentCursor
	for pages := 0; ; pages++ {
		pg := mustV[social.ForumPage](t)(f.module.ListForumEntries(ctx, viewer, storeID, fcur, 2))
		if pages == 0 && len(pg.Entries) != 2 {
			t.Fatalf("討論區第一頁要有 2 筆：%v", forumIDs(pg))
		}
		gotForum = append(gotForum, forumIDs(pg)...)
		if fcur = pg.Next; fcur == nil {
			break
		}
	}
	var pcur *social.PostCursor
	for pages := 0; ; pages++ {
		pg := mustV[social.PostPage](t)(f.module.ListFeed(ctx, viewer, pcur, 2))
		if pages == 0 && len(pg.Posts) != 2 {
			t.Fatalf("動態牆第一頁要有 2 筆：%v", postIDs(pg))
		}
		gotPosts = append(gotPosts, slices.DeleteFunc(postIDs(pg), func(id uuid.UUID) bool { return id == p.ID })...)
		if pcur = pg.Next; pcur == nil {
			break
		}
	}
	if !sameIDs(gotComments, wantComments...) || !sameIDs(gotForum, wantForum...) {
		t.Fatalf("keyset 跨頁不重不漏：\n留言 %v\n要   %v\n討論 %v\n要   %v", gotComments, wantComments, gotForum, wantForum)
	}
	// 動態牆還有 storeID 的第一篇貼文 p（最舊）；扣掉後只剩沒被封鎖的店的貼文。
	if !sameIDs(gotPosts, wantPosts...) {
		t.Fatalf("動態牆跨頁不重不漏：%v 要 %v", gotPosts, wantPosts)
	}
}

// A7-D11：封鎖不影響計數，所有人看到同一個數字。
func TestBlocksDoNotChangeCounts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	a, b := f.newBuyer(t), f.newBuyer(t)
	p := f.publishedPost(t, storeID, "貼文", "organic")
	cB := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: "B"}))
	mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asStore(t, storeID), social.CommentInput{PostID: p.ID, Body: "店家回", ParentID: &cB.ID}))
	mustV[bool](t)(f.module.LikeComment(ctx, f.asBuyer(a), cB.ID))
	mustV[bool](t)(f.module.LikePost(ctx, b.BuyerID, p.ID))
	snapshot := func() string {
		return fmt.Sprintf("post=%d/%d/%d comment=%d/%d tags=%v",
			f.scalar(t, "SELECT like_count FROM social.post_stats WHERE post_id = $1", p.ID),
			f.scalar(t, "SELECT comment_count FROM social.post_stats WHERE post_id = $1", p.ID),
			f.scalar(t, "SELECT share_count FROM social.post_stats WHERE post_id = $1", p.ID),
			f.scalar(t, "SELECT like_count FROM social.comment_stats WHERE comment_id = $1", cB.ID),
			f.scalar(t, "SELECT reply_count FROM social.comment_stats WHERE comment_id = $1", cB.ID),
			mustV[[]social.HashtagCount](t)(f.module.SuggestHashtags(ctx, "org", false, 10)))
	}
	before := snapshot()
	f.blockBuyer(t, a, b)
	f.blockStore(t, a, storeID)
	if after := snapshot(); after != before {
		t.Fatalf("封鎖不改計數：\n前 %s\n後 %s", before, after)
	}
	if got := mustV[social.Post](t)(f.module.PublicPost(ctx, social.Caller{}, p.ID)); got.Stats.Comments != 2 {
		t.Fatalf("匿名看到的留言數是全域的數字：%+v", got.Stats)
	}
	// 熱門標籤與建議是全域彙總，封鎖者看到的與匿名相同。
	trend := func() []social.HashtagCount {
		return mustV[[]social.HashtagCount](t)(f.module.TrendingHashtags(ctx, f.clock().AddDate(0, 0, -7), f.clock().Add(time.Second), 10))
	}
	if got := trend(); len(got) != 1 || got[0].Posts != 1 {
		t.Fatalf("熱門標籤不依觀看者：%v", got)
	}
}

// 匿名讀取不受任何封鎖影響。
func TestAnonymousReadsIgnoreEveryBlock(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	a, b := f.newBuyer(t), f.newBuyer(t)
	p := f.publishedPost(t, storeID, "貼文", "organic")
	cB := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: "B"}))
	eB := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(b), social.ForumInput{StoreID: storeID, Body: "B"}))
	f.blockBuyer(t, a, b)
	f.blockBuyer(t, b, a)
	f.blockStore(t, a, storeID)
	f.blockStore(t, b, storeID)
	f.storeBlocksBuyer(t, storeID, a)

	anon := social.Caller{}
	if _, err := f.module.PublicPost(ctx, anon, p.ID); err != nil {
		t.Fatal(err)
	}
	if feed := mustV[social.PostPage](t)(f.module.ListFeed(ctx, anon, nil, 10)); !sameIDs(postIDs(feed), p.ID) {
		t.Fatalf("%v", postIDs(feed))
	}
	if got := f.topLevel(t, anon, p.ID); !sameIDs(commentIDs(got), cB.ID) {
		t.Fatalf("%v", commentIDs(got))
	}
	if got := f.forumTop(t, anon, storeID); !sameIDs(forumIDs(got), eB.ID) {
		t.Fatalf("%v", forumIDs(got))
	}
	if page := mustV[social.PostPage](t)(f.module.ListHashtagPosts(ctx, anon, "organic", nil, 10)); !sameIDs(postIDs(page), p.ID) {
		t.Fatalf("%v", postIDs(page))
	}
}

// 被排除的頂層底下的回覆：寫入與讚的前置檢查比照讀取要求頂層（父列）沒被觀看者的封鎖排除；即使 reply_to 或被按讚的那一列本身（店家的回覆）看得到。
func TestRepliesUnderAnExcludedTopLevelCannotBeTargetedByWrites(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	a, b := f.newBuyer(t), f.newBuyer(t)
	p := f.publishedPost(t, storeID, "貼文")
	store := f.asStore(t, storeID)
	cB := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: "B"}))
	rB := mustV[social.Comment](t)(f.module.CreateComment(ctx, store, social.CommentInput{PostID: p.ID, Body: "店家回 B", ParentID: &cB.ID}))
	eB := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(b), social.ForumInput{StoreID: storeID, Body: "B"}))
	sB := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, store, social.ForumInput{StoreID: storeID, Body: "店家回 B", ParentID: &eB.ID}))
	f.blockBuyer(t, a, b)

	t.Run("討論區：reply_to 是店家在被排除頂層底下的回覆", func(t *testing.T) {
		in := social.ForumInput{StoreID: storeID, Body: "x", ParentID: &eB.ID, ReplyToID: &sB.ID}
		if _, err := f.module.CreateForumEntry(ctx, f.asBuyer(a), in); kindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("頂層被排除，底下的回覆也不能當回覆目標：%v", err)
		}
	})
	t.Run("討論區：父留言是店家的回覆", func(t *testing.T) {
		in := social.ForumInput{StoreID: storeID, Body: "x", ParentID: &sB.ID}
		if _, err := f.module.CreateForumEntry(ctx, f.asBuyer(a), in); kindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("回覆的串頂層被排除：%v", err)
		}
	})
	t.Run("討論區：只帶 ParentID 的頂層", func(t *testing.T) {
		in := social.ForumInput{StoreID: storeID, Body: "x", ParentID: &eB.ID}
		if _, err := f.module.CreateForumEntry(ctx, f.asBuyer(a), in); kindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("頂層被排除：%v", err)
		}
	})
	t.Run("留言讚：按店家在被排除頂層底下的回覆", func(t *testing.T) {
		if _, err := f.module.LikeComment(ctx, f.asBuyer(a), rB.ID); kindOf(err) != apperr.KindNotFound {
			t.Fatalf("父留言被排除，底下的回覆不能按讚：%v", err)
		}
	})
	t.Run("對照：沒被封鎖的觀看者都可以", func(t *testing.T) {
		c := f.newBuyer(t)
		in := social.ForumInput{StoreID: storeID, Body: "y", ParentID: &eB.ID, ReplyToID: &sB.ID}
		if _, err := f.module.CreateForumEntry(ctx, f.asBuyer(c), in); err != nil {
			t.Fatalf("第三人可以回覆：%v", err)
		}
		if added, err := f.module.LikeComment(ctx, f.asBuyer(c), rB.ID); err != nil || !added {
			t.Fatalf("第三人可以按讚：%v %v", added, err)
		}
	})
}

// 買家封鎖店家後，直接呼叫的瀏覽不計入；匿名與沒封鎖的買家照計。
func TestViewsFromABuyerBlockingTheStoreAreNotCounted(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	a, c := f.newBuyer(t), f.newBuyer(t)
	p := f.publishedPost(t, storeID, "貼文")
	f.blockStore(t, a, storeID)

	if counted, err := f.module.RecordView(ctx, p.ID, social.Viewer{AccountID: 9001, BuyerID: a.BuyerID}); err != nil || counted {
		t.Fatalf("封鎖了這家店的買家不計入：%v %v", counted, err)
	}
	if counted, err := f.module.RecordView(ctx, p.ID, social.Viewer{AccountID: 9002, BuyerID: c.BuyerID}); err != nil || !counted {
		t.Fatalf("沒封鎖的買家照計：%v %v", counted, err)
	}
	if counted, err := f.module.RecordView(ctx, p.ID, social.Viewer{IP: "203.0.113.5", UserAgent: "x"}); err != nil || !counted {
		t.Fatalf("匿名照計：%v %v", counted, err)
	}
	f.unblockStore(t, a, storeID)
	if counted, err := f.module.RecordView(ctx, p.ID, social.Viewer{AccountID: 9001, BuyerID: a.BuyerID}); err != nil || !counted {
		t.Fatalf("解除封鎖後計入：%v %v", counted, err)
	}
}

// D2 效果表：買家封鎖店家後，店主以店家身分仍看得到這位買家在自己貼文下的既有留言與討論區留言。
func TestStoreOwnerStillSeesCommentsOfABuyerWhoBlockedTheStore(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	a := f.newBuyer(t)
	p := f.publishedPost(t, storeID, "貼文")
	cA := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(a), social.CommentInput{PostID: p.ID, Body: "封鎖之前的留言"}))
	eA := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(a), social.ForumInput{StoreID: storeID, Body: "封鎖之前的發言"}))
	f.blockStore(t, a, storeID)

	store := f.asStore(t, storeID)
	if got := f.topLevel(t, store, p.ID); !sameIDs(commentIDs(got), cA.ID) {
		t.Fatalf("店主看得到封鎖了店家的買家的留言：%v", commentIDs(got))
	}
	if got := f.forumTop(t, store, storeID); !sameIDs(forumIDs(got), eA.ID) {
		t.Fatalf("店主看得到封鎖了店家的買家的討論區留言：%v", forumIDs(got))
	}
	if _, err := f.module.PublicPost(ctx, f.asBuyer(a), p.ID); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("封鎖者自己讀不到這家店的貼文：%v", err)
	}
}

// D2 效果表：店家封鎖買家後，店主可以刪除這位買家的既有留言與討論區留言。
func TestStoreCanDeleteExistingContentOfABuyerItBlocked(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	b, c := f.newBuyer(t), f.newBuyer(t)
	p := f.publishedPost(t, storeID, "貼文")
	cB := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: "B"}))
	eB := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(b), social.ForumInput{StoreID: storeID, Body: "B"}))
	f.storeBlocksBuyer(t, storeID, b)

	store := f.asStore(t, storeID)
	must(t, f.module.DeleteComment(ctx, store, cB.ID))
	must(t, f.module.DeleteForumEntry(ctx, store, eB.ID))
	if got := f.topLevel(t, f.asBuyer(c), p.ID); len(got.Comments) != 0 {
		t.Fatalf("留言已刪除：%v", commentIDs(got))
	}
	if got := f.forumTop(t, f.asBuyer(c), storeID); len(got.Entries) != 0 {
		t.Fatalf("討論區留言已刪除：%v", forumIDs(got))
	}
}
