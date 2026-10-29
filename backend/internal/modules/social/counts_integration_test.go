//go:build integration

package social_test

import (
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// oracle 是計數的另一份獨立寫法（手寫 SQL，與 internal/visibility、reads 的重算語句和 trigger 都不同）：貼文的留言數含回覆，只算沒有刪除、沒有未還原的隱藏、
// 作者不是已停用的買家的留言。
func (f *fixture) expectedCommentCount(t *testing.T, postID uuidT) int64 {
	t.Helper()
	return f.scalar(t, `SELECT count(*) FROM social.comments c
		WHERE c.post_id = $1 AND c.deleted_at IS NULL
		  AND NOT EXISTS (SELECT 1 FROM social.hides h WHERE h.comment_id = c.id AND h.restored_at IS NULL)
			  AND NOT EXISTS (SELECT 1 FROM social.content_takedowns t WHERE t.comment_id = c.id)
		  AND NOT (c.actor_kind = 'BUYER' AND EXISTS (SELECT 1 FROM buyer.buyers b WHERE b.buyer_id = c.actor_id AND b.status <> 'ACTIVE'))`, postID)
}

// assertStatsMatchTheDataset 對全部的 stats 列，檢查 trigger 維護的值等於從列算出的值（獨立的 SQL），而且重算沒有發現漂移。
func (f *fixture) assertStatsMatchTheDataset(t *testing.T, label string) {
	t.Helper()
	ctx := context.Background()
	for _, q := range []struct{ name, sql string }{
		{"貼文的留言數", `SELECT count(*) FROM social.post_stats s WHERE s.comment_count <> (
			SELECT count(*) FROM social.comments c WHERE c.post_id = s.post_id AND c.deleted_at IS NULL
			  AND NOT EXISTS (SELECT 1 FROM social.hides h WHERE h.comment_id = c.id AND h.restored_at IS NULL)
			  AND NOT EXISTS (SELECT 1 FROM social.content_takedowns t WHERE t.comment_id = c.id)
			  AND NOT (c.actor_kind = 'BUYER' AND EXISTS (SELECT 1 FROM buyer.buyers b WHERE b.buyer_id = c.actor_id AND b.status <> 'ACTIVE')))`},
		{"留言的計入旗標", `SELECT count(*) FROM social.comments c WHERE c.counted <> (c.deleted_at IS NULL
			  AND NOT EXISTS (SELECT 1 FROM social.hides h WHERE h.comment_id = c.id AND h.restored_at IS NULL)
			  AND NOT EXISTS (SELECT 1 FROM social.content_takedowns t WHERE t.comment_id = c.id)
			  AND NOT (c.actor_kind = 'BUYER' AND EXISTS (SELECT 1 FROM buyer.buyers b WHERE b.buyer_id = c.actor_id AND b.status <> 'ACTIVE')))`},
		{"討論區留言的計入旗標", `SELECT count(*) FROM social.forum_entries c WHERE c.counted <> (c.deleted_at IS NULL
			  AND NOT EXISTS (SELECT 1 FROM social.hides h WHERE h.forum_entry_id = c.id AND h.restored_at IS NULL)
			  AND NOT EXISTS (SELECT 1 FROM social.content_takedowns t WHERE t.forum_entry_id = c.id)
			  AND NOT (c.actor_kind = 'BUYER' AND EXISTS (SELECT 1 FROM buyer.buyers b WHERE b.buyer_id = c.actor_id AND b.status <> 'ACTIVE')))`},
		{"貼文的讚數", `SELECT count(*) FROM social.post_stats s WHERE s.like_count <> (SELECT count(*) FROM social.post_likes l WHERE l.post_id = s.post_id)`},
		{"貼文的分享數", `SELECT count(*) FROM social.post_stats s WHERE s.share_count <> (SELECT count(*) FROM social.post_shares l WHERE l.post_id = s.post_id)`},
		{"留言的讚數", `SELECT count(*) FROM social.comment_stats s WHERE s.like_count <> (SELECT count(*) FROM social.comment_likes l WHERE l.comment_id = s.comment_id)`},
		{"留言的回覆數", `SELECT count(*) FROM social.comment_stats s WHERE s.reply_count <> (
			SELECT count(*) FROM social.comments c WHERE c.parent_id = s.comment_id AND c.deleted_at IS NULL
			  AND NOT EXISTS (SELECT 1 FROM social.hides h WHERE h.comment_id = c.id AND h.restored_at IS NULL)
			  AND NOT EXISTS (SELECT 1 FROM social.content_takedowns t WHERE t.comment_id = c.id)
			  AND NOT (c.actor_kind = 'BUYER' AND EXISTS (SELECT 1 FROM buyer.buyers b WHERE b.buyer_id = c.actor_id AND b.status <> 'ACTIVE')))`},
		{"討論區留言的回覆數", `SELECT count(*) FROM social.forum_entry_stats s WHERE s.reply_count <> (
			SELECT count(*) FROM social.forum_entries c WHERE c.parent_id = s.forum_entry_id AND c.deleted_at IS NULL
			  AND NOT EXISTS (SELECT 1 FROM social.hides h WHERE h.forum_entry_id = c.id AND h.restored_at IS NULL)
			  AND NOT EXISTS (SELECT 1 FROM social.content_takedowns t WHERE t.forum_entry_id = c.id)
			  AND NOT (c.actor_kind = 'BUYER' AND EXISTS (SELECT 1 FROM buyer.buyers b WHERE b.buyer_id = c.actor_id AND b.status <> 'ACTIVE')))`},
	} {
		if n := f.scalar(t, q.sql); n != 0 {
			t.Fatalf("%s：%d 列的計數與資料不符", label+" "+q.name, n)
		}
	}
	rep, err := f.module.RecountAll(ctx)
	must(t, err)
	if rep.Drifted != 0 {
		t.Fatalf("%s：重算發現漂移 %+v", label, rep.Drifts)
	}
}

// 計數的資料集測試：任意一串寫入（留言、回覆、刪除、隱藏、還原、讚、取消讚、分享、討論區、清除 job）之後，stats 等於重算函式（與獨立的 SQL）
// 的結果。固定種子，每一步都檢查。
func TestStatsMatchTheDatasetAfterRandomWrites(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	store := f.asStore(t, storeID)
	buyers := []social.Caller{}
	profiles := []buyerP{}
	for range 5 {
		b := f.newBuyer(t)
		profiles = append(profiles, b)
		buyers = append(buyers, f.asBuyer(b))
	}
	posts := []social.Post{f.publishedPost(t, storeID, "一"), f.publishedPost(t, storeID, "二")}
	var comments []social.Comment
	var entries []social.ForumEntry
	rng := rand.New(rand.NewPCG(20261023, 7)) //nolint:gosec // G404：固定種子，隨機操作序列要可重現，不是安全用途
	steps := 0
	step := func(name string, err error) {
		t.Helper()
		steps++
		if err != nil {
			t.Fatalf("步驟 %d %s：%v", steps, name, err)
		}
		f.assertStatsMatchTheDataset(t, fmt.Sprintf("步驟 %d %s 之後", steps, name))
	}
	pick := func() int { return rng.IntN(len(buyers)) }
	hidden := map[uuidT]bool{}
	purged := map[int]bool{}
	for i := 0; i < 120; i++ {
		post := posts[rng.IntN(len(posts))]
		switch op := rng.IntN(12); {
		case op < 3:
			bi := pick()
			if purged[bi] {
				continue
			}
			c, err := f.module.CreateComment(ctx, buyers[bi], social.CommentInput{PostID: post.ID, Body: "留言"})
			if err == nil {
				comments = append(comments, c)
			}
			step("留言", err)
		case op == 3 && len(comments) > 0:
			parent := comments[rng.IntN(len(comments))]
			if parent.ParentID != nil {
				continue
			}
			r, err := f.module.CreateComment(ctx, store, social.CommentInput{PostID: parent.PostID, Body: "店家回覆", ParentID: &parent.ID})
			if err == nil {
				comments = append(comments, r)
			}
			step("回覆", ignoreNotFound(err))
		case op == 4 && len(comments) > 0:
			c := comments[rng.IntN(len(comments))]
			caller := store
			if c.Actor.Kind == social.ActorBuyer {
				caller = buyers[indexOf(profiles, c.Actor.ID)]
			}
			step("刪除", f.module.DeleteComment(ctx, caller, c.ID))
		case op == 5 && len(comments) > 0:
			c := comments[rng.IntN(len(comments))]
			if c.Actor.Kind != social.ActorBuyer {
				continue
			}
			_, err := f.module.Hide(ctx, social.HideInput{Target: social.Target{Kind: social.TargetComment, ID: c.ID}, Category: "SPAM", AdminID: 1})
			if err == nil {
				hidden[c.ID] = true
			}
			step("隱藏", ignoreNotFound(err))
		case op == 6 && len(hidden) > 0:
			var id uuidT
			for id = range hidden {
				break
			}
			delete(hidden, id)
			_, err := f.module.Restore(ctx, social.RestoreInput{Target: social.Target{Kind: social.TargetComment, ID: id}, AdminID: 1, Reason: "誤判"})
			step("還原", ignoreNotFound(err))
		case op == 7:
			bi := pick()
			if purged[bi] {
				continue
			}
			_, err := f.module.LikePost(ctx, profiles[bi].BuyerID, post.ID)
			step("按讚", err)
		case op == 8:
			bi := pick()
			if purged[bi] {
				continue
			}
			_, err := f.module.UnlikePost(ctx, profiles[bi].BuyerID, post.ID)
			step("取消讚", err)
		case op == 9 && len(comments) > 0:
			bi := pick()
			if purged[bi] {
				continue
			}
			_, err := f.module.LikeComment(ctx, buyers[bi], comments[rng.IntN(len(comments))].ID)
			step("留言按讚", ignoreNotFound(err))
		case op == 10:
			bi := pick()
			if purged[bi] {
				continue
			}
			if rng.IntN(2) == 0 {
				e, err := f.module.CreateForumEntry(ctx, buyers[bi], social.ForumInput{StoreID: storeID, Body: "討論"})
				if err == nil {
					entries = append(entries, e)
				}
				step("討論區", err)
			} else if len(entries) > 0 {
				parent := entries[rng.IntN(len(entries))]
				e, err := f.module.CreateForumEntry(ctx, buyers[bi], social.ForumInput{StoreID: storeID, Body: "回", ParentID: &parent.ID})
				if err == nil {
					entries = append(entries, e)
				}
				step("討論區回覆", ignoreNotFound(err))
			}
		case op == 11 && i > 60 && len(purged) < 2:
			// 買家刪帳：申請（進入刪除中，可見性由 pub 擋住）之後 job 清除並重算。
			bi := pick()
			if purged[bi] {
				continue
			}
			purged[bi] = true
			must(t, f.buyers.BeginDeletion(ctx, profiles[bi].BuyerID))
			step("買家清除", f.module.PurgeBuyerContent(ctx, profiles[bi].BuyerID))
		}
	}
	if steps < 60 {
		t.Fatalf("資料集太小：%d 步", steps)
	}
	for _, p := range posts {
		got := mustV[social.Post](t)(f.module.OwnPost(ctx, storeID, p.ID))
		if got.Stats.Comments != f.expectedCommentCount(t, p.ID) {
			t.Fatalf("留言數 %d 與資料不符", got.Stats.Comments)
		}
	}
}

// ignoreNotFound 吞掉資料集的隨機操作遇到的業務拒絕（目標已不可見、身分不符等）；其他錯誤照回。
func ignoreNotFound(err error) error {
	if slices.Contains([]apperr.Kind{apperr.KindNotFound, apperr.KindPermissionDenied, apperr.KindInvalidInput, apperr.KindFailedPrecondition}, kindOf(err)) {
		return nil
	}
	return err
}

func indexOf(ps []buyerP, id int64) int {
	for i, p := range ps {
		if p.BuyerID == id {
			return i
		}
	}
	return 0
}

// 重算與並行按讚不互蓋：重算先鎖 stats 列再計數，並行的加減在它前後各自成立，最後 stats 等於列數、重算沒有漂移。
func TestRecountDoesNotOverwriteConcurrentLikes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	p := f.publishedPost(t, storeID, "熱門貼文")
	var buyers []buyerP
	for range 24 {
		buyers = append(buyers, f.newBuyer(t))
	}
	var wg sync.WaitGroup
	for _, b := range buyers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := f.module.LikePost(ctx, b.BuyerID, p.ID); err != nil {
				t.Error(err)
			}
		}()
	}
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := f.module.RecountAll(ctx); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if got := mustV[social.Post](t)(f.module.PublicPost(ctx, social.Caller{}, p.ID)); got.Stats.Likes != 24 {
		t.Fatalf("讚數 %d，預期 24", got.Stats.Likes)
	}
	f.assertStatsMatchTheDataset(t, "並行之後")
}

// 每日對帳：故意改壞 stats，job 修回並記下漂移（warn log）；瀏覽數不在對帳範圍。
func TestReconcileOverwritesDriftAndLogsIt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	p := f.publishedPost(t, storeID, "貼文")
	b := f.newBuyer(t)
	mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: "留言"}))
	mustV[bool](t)(f.module.RecordView(ctx, p.ID, social.Viewer{AccountID: 1}))
	rep, err := f.module.Reconcile(ctx)
	must(t, err)
	if rep.Drifted != 0 || strings.Contains(f.logs.String(), "漂移") {
		t.Fatalf("沒有漂移不警告：%+v", rep)
	}
	f.exec(t, "UPDATE social.post_stats SET comment_count = 7, like_count = 3, view_count = 99 WHERE post_id = $1", p.ID)
	rep, err = f.module.Reconcile(ctx)
	must(t, err)
	if rep.Drifted != 1 || rep.Drifts[0].Kind != "POST" || rep.Drifts[0].ID != p.ID {
		t.Fatalf("漂移的數量與對象：%+v", rep)
	}
	if !strings.Contains(f.logs.String(), "計數對帳發現漂移並已覆寫") {
		t.Fatalf("漂移要記 warn：%s", f.logs.String())
	}
	got := mustV[social.Post](t)(f.module.PublicPost(ctx, social.Caller{}, p.ID))
	if got.Stats.Comments != 1 || got.Stats.Likes != 0 || got.Stats.Views != 99 {
		t.Fatalf("修回可重算的欄位，瀏覽數不動：%+v", got.Stats)
	}
}

// DR-A1：全量重算是分批的集合語句。1,201 篇貼文跨三批（一批 500 列），每一批都有被改壞的列：全部被修回、報告的數量與前後值正確、
// 沒有漂移的列不被寫入（xmin 不變）；再跑一次沒有漂移。
func TestRecountAllIsBatchedAndOverwritesOnlyDriftedRows(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	f.exec(t, `INSERT INTO social.posts (id, store_id, status, version, created_at, updated_at)
		SELECT uuidv7(), 1, 'DRAFT', 1, now(), now() FROM generate_series(1, 1200)`)
	storeID, _ := f.exposableStore(t)
	p := f.publishedPost(t, storeID, "有留言的貼文")
	b := f.newBuyer(t)
	mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: "留言"}))
	mustV[bool](t)(f.module.LikePost(ctx, b.BuyerID, p.ID))
	// 依 post_id 的順序（不含有留言的那篇），在第 1、501、1001、1200 列各改壞一列：落在三批的開頭、結尾與中間。
	f.exec(t, `WITH ranked AS (SELECT post_id, row_number() OVER (ORDER BY post_id) AS rn FROM social.post_stats WHERE post_id <> $1)
		UPDATE social.post_stats s SET like_count = 5, share_count = 2 FROM ranked r
		WHERE r.post_id = s.post_id AND r.rn IN (1, 501, 1001, 1200)`, p.ID)
	untouched := f.scalar(t, `SELECT count(*) FROM social.post_stats WHERE like_count = 0 AND share_count = 0 AND comment_count = 0`)
	var xminBefore = map[string]string{}
	rows, err := f.db.Conn(ctx).Query(ctx, `SELECT post_id::text, xmin::text FROM social.post_stats WHERE like_count = 0 AND share_count = 0 AND comment_count = 0`)
	must(t, err)
	for rows.Next() {
		var id, x string
		must(t, rows.Scan(&id, &x))
		xminBefore[id] = x
	}
	must(t, rows.Err())
	rep, err := f.module.RecountAll(ctx)
	must(t, err)
	if rep.Posts != 1201 || rep.Drifted != 4 || len(rep.Drifts) != 4 {
		t.Fatalf("報告：貼文 %d（要 1201）、漂移 %d（要 4）：%+v", rep.Posts, rep.Drifted, rep.Drifts)
	}
	for _, d := range rep.Drifts {
		if d.Kind != "POST" || d.Before[0] != 5 || d.Before[2] != 2 {
			t.Fatalf("前後值：%+v", d)
		}
	}
	f.assertStatsMatchTheDataset(t, "分批重算")
	// 沒有漂移的列沒有被寫入。
	var changed int
	for id, x := range xminBefore {
		var now string
		must(t, f.db.Conn(ctx).QueryRow(ctx, `SELECT xmin::text FROM social.post_stats WHERE post_id = $1`, id).Scan(&now))
		if now != x {
			changed++
		}
	}
	if changed != 0 || int64(len(xminBefore)) != untouched {
		t.Fatalf("沒有漂移的列不能被覆寫：%d 列的 xmin 變了（共 %d 列）", changed, len(xminBefore))
	}
}
