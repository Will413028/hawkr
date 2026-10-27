//go:build integration

package social_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/revalidate"
)

// 買家刪帳：進入刪除中與關閉各在同一個 transaction 記複本、重新求值並排清除 job；可見性在 job 跑之前已由旗標擋住；job 軟刪留言與討論區留言、
// 刪讚與分享、重算計數、排快取失效；重跑結果相同。
func TestBuyerDeletionEnqueuesAnIdempotentPurge(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	leaving, staying := f.newBuyer(t), f.newBuyer(t)
	p := f.publishedPost(t, storeID, "貼文")
	cl := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(leaving), social.CommentInput{PostID: p.ID, Body: "要離開的人的留言"}))
	cs := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(staying), social.CommentInput{PostID: p.ID, Body: "留下的人的留言"}))
	mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(leaving), social.ForumInput{StoreID: storeID, Body: "要離開的人的討論"}))
	top := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(staying), social.ForumInput{StoreID: storeID, Body: "留下的人的討論"}))
	mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(leaving), social.ForumInput{StoreID: storeID, Body: "回覆", ParentID: &top.ID}))
	mustV[bool](t)(f.module.LikePost(ctx, leaving.BuyerID, p.ID))
	mustV[bool](t)(f.module.RecordShare(ctx, leaving.BuyerID, p.ID))
	mustV[bool](t)(f.module.LikeComment(ctx, f.asBuyer(leaving), cs.ID))
	if got := mustV[social.Post](t)(f.module.PublicPost(ctx, social.Caller{}, p.ID)); got.Stats.Comments != 2 {
		t.Fatalf("清除前 %+v", got.Stats)
	}
	f.jobs.reset()

	must(t, f.buyers.BeginDeletion(ctx, leaving.BuyerID))
	if !slices.Equal(f.jobs.kinds(), []string{"social.purge_buyer_content"}) {
		t.Fatalf("進入刪除中同 transaction 排清除 job：%v", f.jobs.kinds())
	}
	// job 跑之前，可見性已經改好：訂閱者在同一個 transaction 記下複本並讓旗標由真轉假，留言與討論區讀不到。
	page := mustV[social.CommentPage](t)(f.module.ListComments(ctx, social.Caller{}, p.ID, nil, 10))
	if len(page.Comments) != 1 || page.Comments[0].ID != cs.ID {
		t.Fatalf("刪除中的買家的留言讀不到：%+v", page.Comments)
	}
	if n := f.scalar(t, "SELECT count(*) FROM social.comments WHERE deleted_at IS NOT NULL"); n != 0 {
		t.Fatal("訂閱者只改旗標與複本，不軟刪內容（軟刪是清除 job 的事）")
	}

	must(t, f.module.PurgeBuyerContent(ctx, leaving.BuyerID))
	snapshot := func() []int64 {
		return []int64{
			f.scalar(t, "SELECT count(*) FROM social.comments WHERE actor_kind = 'BUYER' AND actor_id = $1 AND deleted_at IS NULL", leaving.BuyerID),
			f.scalar(t, "SELECT count(*) FROM social.forum_entries WHERE actor_kind = 'BUYER' AND actor_id = $1 AND deleted_at IS NULL", leaving.BuyerID),
			f.scalar(t, "SELECT count(*) FROM social.post_likes WHERE buyer_id = $1", leaving.BuyerID),
			f.scalar(t, "SELECT count(*) FROM social.post_shares WHERE buyer_id = $1", leaving.BuyerID),
			f.scalar(t, "SELECT count(*) FROM social.comment_likes WHERE actor_kind = 'BUYER' AND actor_id = $1", leaving.BuyerID),
			f.scalar(t, "SELECT count(*) FROM social.comments WHERE actor_kind = 'BUYER' AND actor_id = $1 AND deleted_at IS NOT NULL", staying.BuyerID),
			f.scalar(t, "SELECT comment_count FROM social.post_stats WHERE post_id = $1", p.ID),
			f.scalar(t, "SELECT like_count + share_count FROM social.post_stats WHERE post_id = $1", p.ID),
			f.scalar(t, "SELECT like_count FROM social.comment_stats WHERE comment_id = $1", cs.ID),
			f.scalar(t, "SELECT reply_count FROM social.forum_entry_stats WHERE forum_entry_id = $1", top.ID),
		}
	}
	want := []int64{0, 0, 0, 0, 0, 0, 1, 0, 0, 0}
	if got := snapshot(); !slices.Equal(got, want) {
		t.Fatalf("清除之後 %v，預期 %v", got, want)
	}
	if len(f.jobs.revalidations()) == 0 {
		t.Fatal("受影響的貼文要排快取失效")
	}
	f.assertStatsMatchTheDataset(t, "清除之後")
	_ = cl
	// 重跑、以及關閉（BuyerDeleted）再排一次：結果相同。
	f.jobs.reset()
	must(t, f.buyers.CloseForDeletion(ctx, leaving.BuyerID))
	if n := strings.Count(strings.Join(f.jobs.kinds(), ","), "social.purge_buyer_content"); n != 1 {
		t.Fatalf("關閉也排同一個 job（冪等）：%v", f.jobs.kinds())
	}
	must(t, f.module.PurgeBuyerContent(ctx, leaving.BuyerID))
	must(t, f.module.PurgeBuyerContent(ctx, leaving.BuyerID))
	if got := snapshot(); !slices.Equal(got, want) {
		t.Fatalf("重跑之後 %v，預期 %v", got, want)
	}
}

// 店家退場（StoreRetired）：訂閱者只排清除 job；可見性在 job 之前由曝光規則擋住；job 軟刪貼文並釋放圖片引用、軟刪討論區留言與店家身分的留言；
// 標籤的數量在讀取時算，退場之後不再算它的貼文（SD16）；重跑結果相同。
func TestStoreRetirementEnqueuesAnIdempotentPurge(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	otherID, _ := f.exposableStore(t)
	b := f.newBuyer(t)
	p := f.publishedPost(t, storeID, "貼文", "水果")
	keep := f.publishedPost(t, otherID, "別家的貼文", "水果")
	ref := media.Ref{Type: media.RefTypePost, ID: p.ID.String()}
	mustV[media.SyncResult](t)(f.module.SyncMedia(ctx, ref))
	if len(f.media.Referenced(ref)) != 1 {
		t.Fatal("同步後有圖片引用")
	}
	mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(b), social.ForumInput{StoreID: storeID, Body: "討論"}))
	mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asStore(t, storeID), social.ForumInput{StoreID: storeID, Body: "店家的公告"}))
	c := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: keep.ID, Body: "別家貼文的留言"}))
	reply := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asStore(t, otherID), social.CommentInput{PostID: keep.ID, Body: "x", ParentID: &c.ID}))
	_ = reply
	// 店家身分的留言只存在於它自己的貼文與討論區（actor.ForWrite 只在資源屬於呼叫者的店時推導出 STORE）。
	storeComment := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asStore(t, storeID), social.CommentInput{PostID: p.ID, Body: "自己貼文下的"}))
	_ = storeComment
	if got := mustV[[]social.HashtagCount](t)(f.module.SuggestHashtags(ctx, "水", false, 10)); len(got) != 1 || got[0].Posts != 2 {
		t.Fatalf("退場前 %+v", got)
	}
	f.jobs.reset()
	must(t, f.db.InTx(ctx, func(ctx context.Context) error {
		return f.module.OnStoreRetired(ctx, seller.StoreRetired{StoreID: storeID})
	}))
	if !slices.Equal(f.jobs.kinds(), []string{"social.purge_store_content"}) {
		t.Fatalf("只排清除 job：%v", f.jobs.kinds())
	}
	// 店家不再可曝光（退場前 seller 已經擋住）：讀取時不算它的標籤。
	f.setExposable(t, storeID, false)
	if got := mustV[[]social.HashtagCount](t)(f.module.SuggestHashtags(ctx, "水", false, 10)); len(got) != 1 || got[0].Posts != 1 {
		t.Fatalf("標籤數量在讀取時算，退場的店不算（SD16）：%+v", got)
	}
	if feed := mustV[social.PostPage](t)(f.module.ListFeed(ctx, social.Caller{}, nil, 10)); len(feed.Posts) != 1 || feed.Posts[0].ID != keep.ID {
		t.Fatalf("動態牆：%+v", feed.Posts)
	}
	for range 2 {
		must(t, f.module.PurgeStoreContent(ctx, storeID))
		if n := f.scalar(t, "SELECT count(*) FROM social.posts WHERE store_id = $1 AND (deleted_at IS NULL OR image_asset_id IS NOT NULL)", storeID); n != 0 {
			t.Fatalf("貼文已軟刪、圖片欄位已清：%d", n)
		}
		if n := f.scalar(t, "SELECT count(*) FROM social.forum_entries WHERE store_id = $1 AND deleted_at IS NULL", storeID); n != 0 {
			t.Fatalf("討論區（含買家寫的與店家公告）已清：%d", n)
		}
		if n := f.scalar(t, "SELECT count(*) FROM social.comments WHERE actor_kind = 'STORE' AND actor_id = $1 AND deleted_at IS NULL", storeID); n != 0 {
			t.Fatalf("店家身分的留言已清：%d", n)
		}
		if n := f.scalar(t, "SELECT count(*) FROM social.comments WHERE id = $1 AND deleted_at IS NULL", c.ID); n != 1 {
			t.Fatal("別人的留言不動")
		}
	}
	mustV[media.SyncResult](t)(f.module.SyncMedia(ctx, ref))
	if len(f.media.Referenced(ref)) != 0 {
		t.Fatalf("圖片引用已釋放：%v", f.media.Referenced(ref))
	}
	f.assertStatsMatchTheDataset(t, "店家清除之後")
	if _, err := f.module.PublicPost(ctx, social.Caller{}, p.ID); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("%v", err)
	}
}

// DR-A4（計畫 P2：實體刪除由 job 執行）：買家刪帳的清除 job 不只軟刪，還把他全部已刪除的留言與討論區留言的內文設為 NULL（包含他在申請之前自己刪掉、
// 內文還留著的），列保留讓討論串結構與佔位項目還在。清除之後 social 的任何文字欄位都找不到他寫過的字；別人的內容不受影響；
// 作者自己刪除時仍然保留內文（不改一般刪除存的東西）。
func TestBuyerPurgeScrubsBodiesAndKeepsTombstones(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	leaving, staying := f.newBuyer(t), f.newBuyer(t)
	p := f.publishedPost(t, storeID, "貼文")
	top := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(leaving), social.CommentInput{PostID: p.ID, Body: "SECRET頂層留言"}))
	mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asStore(t, storeID), social.CommentInput{PostID: p.ID, ParentID: &top.ID, Body: "店家的回覆"}))
	own := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(leaving), social.CommentInput{PostID: p.ID, Body: "SECRET自己刪掉的留言"}))
	other := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(staying), social.CommentInput{PostID: p.ID, Body: "留下的人的留言"}))
	_ = other
	fstay := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(staying), social.ForumInput{StoreID: storeID, Body: "留下的人的討論"}))
	mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(leaving), social.ForumInput{StoreID: storeID, Body: "SECRET回覆別人的討論", ParentID: &fstay.ID}))
	ftop := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(leaving), social.ForumInput{StoreID: storeID, Body: "SECRET討論"}))
	mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asStore(t, storeID), social.ForumInput{StoreID: storeID, Body: "店家回討論", ParentID: &ftop.ID}))
	fown := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(leaving), social.ForumInput{StoreID: storeID, Body: "SECRET自己刪掉的討論"}))
	must(t, f.module.DeleteComment(ctx, f.asBuyer(leaving), own.ID))
	must(t, f.module.DeleteForumEntry(ctx, f.asBuyer(leaving), fown.ID))
	if n := f.scalar(t, "SELECT count(*) FROM social.comments WHERE id = $1 AND deleted_at IS NOT NULL AND body = 'SECRET自己刪掉的留言'", own.ID); n != 1 {
		t.Fatal("作者自己刪除時內文仍然保留（一般刪除存的東西不變）")
	}

	// CHECK：內文 NULL 只能出現在已刪除的列（未刪除的留言與討論區留言不能清空內文）。
	for _, q := range []string{
		"UPDATE social.comments SET body = NULL WHERE id = $1",
		"UPDATE social.forum_entries SET body = NULL WHERE id = $1",
	} {
		id := top.ID
		if strings.Contains(q, "forum") {
			id = ftop.ID
		}
		if _, err := f.db.Conn(ctx).Exec(ctx, q, id); err == nil || !strings.Contains(err.Error(), "scrubbed_check") {
			t.Fatalf("未刪除的列不能清空內文：%v", err)
		}
	}

	must(t, f.buyers.BeginDeletion(ctx, leaving.BuyerID))
	must(t, f.module.PurgeBuyerContent(ctx, leaving.BuyerID))
	must(t, f.module.PurgeBuyerContent(ctx, leaving.BuyerID)) // 重跑冪等

	// social 的每一個文字欄位都找不到他寫過的字（內文，也包含留言事件、隱藏紀錄等別處）。
	rows, err := f.db.Conn(ctx).Query(ctx, `SELECT table_name, column_name FROM information_schema.columns
		WHERE table_schema = 'social' AND data_type IN ('text', 'character varying', 'jsonb', 'json') ORDER BY 1, 2`)
	must(t, err)
	type col struct{ table, column string }
	var cols []col
	for rows.Next() {
		var c col
		must(t, rows.Scan(&c.table, &c.column))
		cols = append(cols, c)
	}
	must(t, rows.Err())
	if len(cols) < 10 {
		t.Fatalf("掃描的欄位太少：%d", len(cols))
	}
	for _, c := range cols {
		if n := f.scalar(t, `SELECT count(*) FROM social.`+c.table+` WHERE `+c.column+`::text LIKE '%SECRET%'`); n != 0 {
			t.Errorf("social.%s.%s 還留著買家寫過的字：%d 列", c.table, c.column, n)
		}
	}
	// 列還在，內文是 NULL、deleted_at 已設；別人的內容完整。
	if n := f.scalar(t, "SELECT count(*) FROM social.comments WHERE actor_kind = 'BUYER' AND actor_id = $1 AND body IS NULL AND deleted_at IS NOT NULL", leaving.BuyerID); n != 2 {
		t.Fatalf("買家的 2 則留言要留下 NULL 內文的墓碑：%d", n)
	}
	if n := f.scalar(t, "SELECT count(*) FROM social.forum_entries WHERE actor_kind = 'BUYER' AND actor_id = $1 AND body IS NULL AND deleted_at IS NOT NULL", leaving.BuyerID); n != 3 {
		t.Fatalf("買家的 3 則討論區留言要留下 NULL 內文的墓碑：%d", n)
	}
	if n := f.scalar(t, "SELECT count(*) FROM social.comments WHERE body IN ('店家的回覆', '留下的人的留言') ") + f.scalar(t, "SELECT count(*) FROM social.forum_entries WHERE body IN ('店家回討論', '留下的人的討論')"); n != 4 {
		t.Fatalf("別人的內容不受影響：%d", n)
	}
	// 討論串的結構還在：有店家回覆的頂層留言與討論區留言仍以佔位項目出現，看不到內文與作者。
	page := mustV[social.CommentPage](t)(f.module.ListComments(ctx, social.Caller{}, p.ID, nil, 10))
	var placeholder *social.Comment
	for i, c := range page.Comments {
		if c.ID == top.ID {
			placeholder = &page.Comments[i]
		}
		if c.ID == own.ID {
			t.Fatal("沒有回覆的已刪除留言不出現")
		}
	}
	if placeholder == nil || placeholder.State != social.CommentDeleted || placeholder.Body != "" || placeholder.Actor.ID != 0 {
		t.Fatalf("有店家回覆的頂層留言是佔位項目：%+v", placeholder)
	}
	replies := mustV[social.CommentPage](t)(f.module.ListReplies(ctx, social.Caller{}, top.ID, nil, 10))
	if len(replies.Comments) != 1 || replies.Comments[0].Body != "店家的回覆" {
		t.Fatalf("佔位項目底下的回覆還在：%+v", replies.Comments)
	}
	fpage := mustV[social.ForumPage](t)(f.module.ListForumEntries(ctx, social.Caller{}, storeID, nil, 10))
	var fplaceholder *social.ForumEntry
	for i, e := range fpage.Entries {
		if e.ID == ftop.ID {
			fplaceholder = &fpage.Entries[i]
		}
		if e.ID == fown.ID {
			t.Fatal("沒有回覆的已刪除討論區留言不出現")
		}
	}
	if fplaceholder == nil || fplaceholder.State != social.CommentDeleted || fplaceholder.Body != "" {
		t.Fatalf("討論區的佔位項目：%+v", fplaceholder)
	}
	f.assertStatsMatchTheDataset(t, "內文清除之後")
}

// FDR4：沒有開啟中的訂單時，買家直接從營運中關閉（BuyerDeleted），沒有 BuyerDeletionRequested。OnBuyerDeleted 因此不是多餘的重複：
// 它要記下複本、讓旗標與計數立即改好，並排清除 job（唯一的入口）。少了它，這條路徑上買家的內容照常公開、計入，也永遠不會被清除。
func TestDirectCloseStillHidesUncountsAndPurges(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	b := f.newBuyer(t)
	p := f.publishedPost(t, storeID, "貼文")
	mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: "留言"}))
	f.jobs.reset()
	must(t, f.buyers.CloseForDeletion(ctx, b.BuyerID))
	purges := 0
	for _, k := range f.jobs.kinds() {
		if k == "social.purge_buyer_content" {
			purges++
		}
	}
	if purges != 1 {
		t.Fatalf("直接關閉只發 BuyerDeleted，清除 job 要由它排一次：%v", f.jobs.kinds())
	}
	if n := f.scalar(t, "SELECT count(*) FROM social.inactive_authors WHERE buyer_id = $1", b.BuyerID); n != 1 {
		t.Fatalf("直接關閉要記下複本：%d", n)
	}
	if n := f.scalar(t, "SELECT comment_count FROM social.post_stats WHERE post_id = $1", p.ID); n != 0 {
		t.Fatalf("直接關閉之後留言數 %d，預期 0", n)
	}
	if page := mustV[social.CommentPage](t)(f.module.ListComments(ctx, social.Caller{}, p.ID, nil, 10)); len(page.Comments) != 0 {
		t.Fatalf("已關閉的買家的留言讀不到：%+v", page.Comments)
	}
	must(t, f.module.PurgeBuyerContent(ctx, b.BuyerID))
	f.assertStatsMatchTheDataset(t, "直接關閉之後")
}

// 每日對帳核對複本與 buyer.pub_inactive_buyers：訂閱者漏了（這裡直接改買家的狀態、不經事件）時補上複本、讓內容重新求值，並計入漂移；
// 補完再對帳不再有漂移。
func TestReconcileAddsTheBuyersTheSubscriberMissed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	b, other := f.newBuyer(t), f.newBuyer(t)
	p := f.publishedPost(t, storeID, "貼文")
	mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: "留言"}))
	mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(other), social.CommentInput{PostID: p.ID, Body: "留下的人的留言"}))
	f.exec(t, "UPDATE buyer.buyers SET status = 'PENDING_DELETION' WHERE buyer_id = $1", b.BuyerID)
	if n := f.scalar(t, "SELECT comment_count FROM social.post_stats WHERE post_id = $1", p.ID); n != 2 {
		t.Fatalf("前提：沒有事件，社群不知道，留言數 %d", n)
	}
	rep, err := f.module.Reconcile(ctx)
	must(t, err)
	if rep.AuthorsAdded != 1 || rep.Drifted < 1 {
		t.Fatalf("對帳要補上 1 位買家並回報漂移：%+v", rep)
	}
	if n := f.scalar(t, "SELECT comment_count FROM social.post_stats WHERE post_id = $1", p.ID); n != 1 {
		t.Fatalf("補上之後留言數 %d，預期 1（只剩留下的人）", n)
	}
	f.assertStatsMatchTheDataset(t, "對帳補複本")
	rep, err = f.module.Reconcile(ctx)
	must(t, err)
	if rep.AuthorsAdded != 0 || rep.Drifted != 0 {
		t.Fatalf("補好之後再對帳不該有漂移：%+v", rep)
	}
}

// R2：買家刪帳的清除 job 軟刪他在各店討論區的留言，要送各店店面頁的快取失效（與隱藏討論區留言相同的 tag），每家店一次，不重複。
func TestBuyerPurgeBustsTheStorePagesOfTheirForumEntries(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeA, _ := f.exposableStore(t)
	storeB, _ := f.exposableStore(t)
	b := f.newBuyer(t)
	for _, in := range []social.ForumInput{{StoreID: storeA, Body: "甲一"}, {StoreID: storeA, Body: "甲二"}, {StoreID: storeB, Body: "乙"}} {
		mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(b), in))
	}
	must(t, f.buyers.BeginDeletion(ctx, b.BuyerID))
	f.jobs.reset()
	must(t, f.module.PurgeBuyerContent(ctx, b.BuyerID))
	var got []string
	for _, tags := range f.jobs.revalidations() {
		got = append(got, tags...)
	}
	for _, id := range []int64{storeA, storeB} {
		st := f.store(t, id)
		for _, tag := range revalidate.StoreTags(st.StoreID, st.StoreCode) {
			if n := slices.Index(got, tag); n < 0 || slices.Index(got[n+1:], tag) >= 0 {
				t.Fatalf("店家 %d 的 tag %q 要送一次：%v", id, tag, got)
			}
		}
	}
}
