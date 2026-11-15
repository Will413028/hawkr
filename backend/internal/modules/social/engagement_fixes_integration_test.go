//go:build integration

package social_test

import (
	"context"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func TestCommentUpdateAndDeleteUseTheVersion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	b := f.newBuyer(t)
	p := f.publishedPost(t, storeID, "貼文")
	c := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: "原文"}))
	if c.Version != 1 || c.StoreID != storeID {
		t.Fatalf("新留言 version=%d store=%d", c.Version, c.StoreID)
	}
	liker := f.newBuyer(t)
	mustV[bool](t)(f.module.LikeComment(ctx, f.asBuyer(liker), c.ID))

	u := mustV[social.Comment](t)(f.module.UpdateComment(ctx, f.asBuyer(b), c.ID, 1, "改一次"))
	if u.Version != 2 || u.Body != "改一次" || u.Likes != 1 || u.State != social.CommentVisible {
		t.Fatalf("修改後 = %+v", u)
	}
	if _, err := f.module.UpdateComment(ctx, f.asBuyer(b), c.ID, 1, "拿舊版本改"); kindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("舊版本修改：%v", err)
	}
	if got := f.scalar(t, `SELECT count(*) FROM social.comments WHERE id = $1 AND body = '改一次' AND version = 2`, c.ID); got != 1 {
		t.Fatal("版本衝突不該寫入")
	}
	stale := int64(1)
	if err := f.module.DeleteComment(ctx, f.asBuyer(b), c.ID, stale); kindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("舊版本刪除：%v", err)
	}
	current := int64(2)
	must(t, f.module.DeleteComment(ctx, f.asBuyer(b), c.ID, current))
	must(t, f.module.DeleteComment(ctx, f.asBuyer(b), c.ID, stale))
}

func TestForumEntryUpdateAndDeleteUseTheVersion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	b := f.newBuyer(t)
	e := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(b), social.ForumInput{StoreID: storeID, Body: "原文"}))
	mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asStore(t, storeID), social.ForumInput{StoreID: storeID, Body: "店家回", ParentID: &e.ID}))

	u := mustV[social.ForumEntry](t)(f.module.UpdateForumEntry(ctx, f.asBuyer(b), e.ID, 1, "改一次"))
	if u.Version != 2 || u.Body != "改一次" || u.ReplyCount != 1 {
		t.Fatalf("修改後 = %+v", u)
	}
	if _, err := f.module.UpdateForumEntry(ctx, f.asBuyer(b), e.ID, 1, "拿舊版本改"); kindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("舊版本修改：%v", err)
	}
	stale := int64(1)
	if err := f.module.DeleteForumEntry(ctx, f.asBuyer(b), e.ID, stale); kindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("舊版本刪除：%v", err)
	}
	current := int64(2)
	must(t, f.module.DeleteForumEntry(ctx, f.asBuyer(b), e.ID, current))
}

func TestReplayReturnsTheCurrentStateOfTheComment(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	b := f.newBuyer(t)
	p := f.publishedPost(t, storeID, "貼文")
	rid := uuid.New()
	in := social.CommentInput{PostID: p.ID, Body: "留言", RequestID: &rid}
	first := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), in))
	mustV[bool](t)(f.module.LikeComment(ctx, f.asBuyer(f.newBuyer(t)), first.ID))
	mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asStore(t, storeID), social.CommentInput{PostID: p.ID, Body: "店家回", ParentID: &first.ID}))

	live := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), in))
	if live.State != social.CommentVisible || live.Likes != 1 || live.ReplyCount != 1 || live.StoreID != storeID {
		t.Fatalf("重送應回目前的計數：%+v", live)
	}
	mustV[social.Comment](t)(f.module.UpdateComment(ctx, f.asBuyer(b), first.ID, 1, "改過"))
	if got := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), in)); got.Body != "改過" || got.Version != 2 {
		t.Fatalf("重送應回目前的內文與版本：%+v", got)
	}

	must(t, f.module.DeleteComment(ctx, f.asBuyer(b), first.ID, 0))
	gone := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), in))
	if gone.ID != first.ID || gone.State != social.CommentDeleted || gone.Body != "" || gone.Actor != (social.Actor{}) {
		t.Fatalf("已刪除的留言重送應回 DELETED 佔位：%+v", gone)
	}
	if n := f.scalar(t, `SELECT count(*) FROM social.comments WHERE post_id = $1 AND parent_id IS NULL`, p.ID); n != 1 {
		t.Fatalf("重送不該另建留言：%d", n)
	}
}

func TestReplayOfAnInactiveAuthorsContentMatchesTheListState(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	b := f.newBuyer(t)
	p := f.publishedPost(t, storeID, "貼文")
	rid := uuid.New()
	in := social.CommentInput{PostID: p.ID, Body: "留言", RequestID: &rid}
	first := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), in))
	mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asStore(t, storeID), social.CommentInput{PostID: p.ID, Body: "店家回", ParentID: &first.ID}))
	f.exec(t, `INSERT INTO social.inactive_authors (buyer_id, since) VALUES ($1, now())`, b.BuyerID)
	f.exec(t, `SELECT social.reevaluate_comments(ARRAY(SELECT id FROM social.comments WHERE actor_id = $1))`, b.BuyerID)

	listed := f.topLevel(t, social.Caller{}, p.ID).Comments
	if len(listed) != 1 || listed[0].ID != first.ID || listed[0].State != social.CommentDeleted {
		t.Fatalf("前提：列表應把作者停用的留言顯示成 DELETED 佔位：%+v", listed)
	}
	got := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), in))
	if got.ID != first.ID || got.State != listed[0].State || got.Body != "" || got.Actor != (social.Actor{}) {
		t.Fatalf("重送的狀態應與列表同值（%s）：%+v", listed[0].State, got)
	}
}

func TestReplayReturnsTheCurrentStateOfTheForumEntry(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	b := f.newBuyer(t)
	rid := uuid.New()
	in := social.ForumInput{StoreID: storeID, Body: "討論", RequestID: &rid}
	first := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(b), in))
	mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asStore(t, storeID), social.ForumInput{StoreID: storeID, Body: "店家回", ParentID: &first.ID}))

	live := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(b), in))
	if live.State != social.CommentVisible || live.ReplyCount != 1 {
		t.Fatalf("重送應回目前的回覆數：%+v", live)
	}
	must(t, f.module.DeleteForumEntry(ctx, f.asBuyer(b), first.ID, 0))
	gone := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(b), in))
	if gone.ID != first.ID || gone.State != social.CommentDeleted || gone.Body != "" || gone.Actor != (social.Actor{}) {
		t.Fatalf("已刪除的項目重送應回 DELETED 佔位：%+v", gone)
	}
}

func TestBuyerPurgeClearsTheRequestIDAndItsHash(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	leaving := f.newBuyer(t)
	p := f.publishedPost(t, storeID, "貼文")
	rc, rf := uuid.New(), uuid.New()
	c := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(leaving), social.CommentInput{PostID: p.ID, Body: "SECRET留言", RequestID: &rc}))
	e := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(leaving), social.ForumInput{StoreID: storeID, Body: "SECRET討論", RequestID: &rf}))
	must(t, f.module.DeleteComment(ctx, f.asBuyer(leaving), c.ID, 0))
	must(t, f.module.DeleteForumEntry(ctx, f.asBuyer(leaving), e.ID, 0))
	if n := f.scalar(t, `SELECT count(*) FROM social.comments WHERE id = $1 AND request_hash IS NOT NULL`, c.ID); n != 1 {
		t.Fatal("清除前留言應有 request_hash")
	}

	must(t, f.buyers.BeginDeletion(ctx, leaving.BuyerID))
	must(t, f.module.PurgeBuyerContent(ctx, leaving.BuyerID))

	for _, q := range []struct {
		sql string
		id  uuid.UUID
	}{
		{`SELECT count(*) FROM social.comments WHERE id = $1 AND body IS NULL AND request_id IS NULL AND request_hash IS NULL`, c.ID},
		{`SELECT count(*) FROM social.forum_entries WHERE id = $1 AND body IS NULL AND request_id IS NULL AND request_hash IS NULL`, e.ID},
	} {
		if n := f.scalar(t, q.sql, q.id); n != 1 {
			t.Fatalf("清除後內文、request_id、request_hash 都該是 NULL：%s", q.sql)
		}
	}
}

func TestRecordViewSkipsTheOwnerOfThePostsStore(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	p := f.publishedPost(t, storeID, "貼文")
	owner := f.asStore(t, storeID)
	if counted := mustV[bool](t)(f.module.RecordView(ctx, owner, p.ID, social.Viewer{AccountID: 5001})); counted {
		t.Fatal("店主看自己店的貼文不該計")
	}
	if n := f.scalar(t, `SELECT count(*) FROM social.post_views WHERE post_id = $1`, p.ID); n != 0 {
		t.Fatalf("post_views = %d，預期 0", n)
	}
	otherStore, _ := f.exposableStore(t)
	if counted := mustV[bool](t)(f.module.RecordView(ctx, f.asStore(t, otherStore), p.ID, social.Viewer{AccountID: 5002})); !counted {
		t.Fatal("別家店的店主看這篇貼文照計")
	}
}

func TestPurgeBumpsTheVersionOfWhatItChanges(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	leaving := f.newBuyer(t)
	p := f.publishedPost(t, storeID, "貼文")
	c := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(leaving), social.CommentInput{PostID: p.ID, Body: "留言"}))
	e := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(leaving), social.ForumInput{StoreID: storeID, Body: "討論"}))
	keepC := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(leaving), social.CommentInput{PostID: p.ID, Body: "留言二"}))
	keepE := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(leaving), social.ForumInput{StoreID: storeID, Body: "討論二"}))
	must(t, f.module.DeleteComment(ctx, f.asBuyer(leaving), keepC.ID, 0))
	must(t, f.module.DeleteForumEntry(ctx, f.asBuyer(leaving), keepE.ID, 0))

	must(t, f.buyers.BeginDeletion(ctx, leaving.BuyerID))
	must(t, f.module.PurgeBuyerContent(ctx, leaving.BuyerID))

	for _, q := range []struct {
		sql string
		id  uuid.UUID
	}{
		{`SELECT count(*) FROM social.comments WHERE id = $1 AND deleted_at IS NOT NULL AND version = 2`, c.ID},
		{`SELECT count(*) FROM social.forum_entries WHERE id = $1 AND deleted_at IS NOT NULL AND version = 2`, e.ID},
		{`SELECT count(*) FROM social.comments WHERE id = $1 AND body IS NULL AND version = 3`, keepC.ID},
		{`SELECT count(*) FROM social.forum_entries WHERE id = $1 AND body IS NULL AND version = 3`, keepE.ID},
	} {
		if n := f.scalar(t, q.sql, q.id); n != 1 {
			t.Fatalf("清除後版本應各加一：%s", q.sql)
		}
	}
}

func TestUpdateOfAHiddenCommentIsFailedPreconditionNotVersionConflict(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	b := f.newBuyer(t)
	p := f.publishedPost(t, storeID, "貼文")
	c := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: "留言"}))
	mustV[social.Hide](t)(f.module.Hide(ctx, social.HideInput{Target: social.Target{Kind: social.TargetComment, ID: c.ID}, Category: "SPAM", AdminID: 1}))
	if _, err := f.module.UpdateComment(ctx, f.asBuyer(b), c.ID, 99, "改"); kindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("被隱藏的留言以舊版本修改：%v", err)
	}
}
