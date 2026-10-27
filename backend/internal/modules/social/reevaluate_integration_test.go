//go:build integration

package social_test

import (
	"context"
	"sync"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/social"
)

// hideRow 繞過服務層（服務層的 Hide 會先鎖內容列）直接寫一筆隱藏，測 trigger 與 reevaluate 自己的鎖。
func (f *fixture) hideRow(ctx context.Context, comment uuid.UUID) error {
	_, err := f.db.Conn(ctx).Exec(ctx, `INSERT INTO social.hides (id, comment_id, reason_category, reason_text, admin_id, hidden_at)
		VALUES ($1, $2, 'SPAM', 'x', 1, $3)`, uuid.NewV7(), comment, f.clock())
	return err
}

func (f *fixture) restoreRows(ctx context.Context, comment uuid.UUID) error {
	_, err := f.db.Conn(ctx).Exec(ctx, "UPDATE social.hides SET restored_at = $2, restored_by = 1, restore_reason = 'ok' WHERE comment_id = $1 AND restored_at IS NULL", comment, f.clock())
	return err
}

// 兩個並行的 reevaluate 對同一則旗標漂移的留言（旗標假、事實為計入）：先到的把旗標改真、計數加一並持有尚未提交的改動，後到的有鎖時
// 卡在最前面的鎖，等先到的提交之後才求值，看到旗標已經是真而什麼都不做；沒有鎖時後到的先求值（旗標還是假）得出要加一，等先到的提交後照加，
// 計數變成 2。這是「先鎖再求值」的判別測試（拿掉函式開頭的三個鎖，這支測試就失敗）。
func TestConcurrentReevaluatesDoNotCountTwice(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	b := f.newBuyer(t)
	p := f.publishedPost(t, storeID, "貼文")
	cm := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: "留言"}))
	// 旗標漂移：事實是計入，旗標與計數卻是不計入（計數與旗標一致，所以兩次加一的錯會看得出來）。
	f.exec(t, "UPDATE social.comments SET counted = FALSE WHERE id = $1", cm.ID)
	f.exec(t, "UPDATE social.post_stats SET comment_count = 0 WHERE post_id = $1", p.ID)
	reevaluate := func(ctx context.Context) error {
		_, err := f.db.Conn(ctx).Exec(ctx, "SELECT social.reevaluate_comments(ARRAY[$1::uuid])", cm.ID)
		return err
	}
	holdErr, opErr := f.race(t, func() error { return reevaluate(context.Background()) },
		func(ctx context.Context, r *racer) error {
			if err := reevaluate(ctx); err != nil {
				return err
			}
			r.Start()
			r.Settled()
			return nil
		})
	must(t, holdErr)
	must(t, opErr)
	if n := f.scalar(t, "SELECT comment_count FROM social.post_stats WHERE post_id = $1", p.ID); n != 1 {
		t.Fatalf("同一則留言只能計入一次，貼文的留言數 %d", n)
	}
	f.assertStatsMatchTheDataset(t, "並行的 reevaluate")
}

// 繞過服務層（服務層的 Hide 會先鎖內容列）直接寫一筆隱藏，撞上尚未提交的刪除：不論卡在哪一道鎖，結果都是「已刪除又被隱藏」只扣一次。
func TestDirectHideRowRacingADeleteCountsOnce(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	b := f.newBuyer(t)
	p := f.publishedPost(t, storeID, "貼文")
	cm := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: "留言"}))
	holdErr, opErr := f.race(t, func() error { return f.hideRow(context.Background(), cm.ID) },
		func(ctx context.Context, r *racer) error {
			if err := f.module.DeleteComment(ctx, f.asBuyer(b), cm.ID); err != nil {
				return err
			}
			r.Start()
			r.Settled()
			return nil
		})
	must(t, holdErr)
	must(t, opErr)
	if n := f.scalar(t, "SELECT comment_count FROM social.post_stats WHERE post_id = $1", p.ID); n != 0 {
		t.Fatalf("留言已刪除又被隱藏，貼文的留言數 %d", n)
	}
	f.assertStatsMatchTheDataset(t, "直接寫隱藏撞上刪除")
}

// 多個 reevaluate 與隱藏、還原並行：不論傳入 ID 的順序，鎖都依固定順序取（內容列、貼文的 stats、父項的 stats，各自依 ID），所以不死結；
// 結束後旗標與計數等於資料集。
func TestReevaluateHasNoDeadlocksAndEndsConsistent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	b := f.newBuyer(t)
	var ids []uuid.UUID
	for _, body := range []string{"一", "二"} {
		p := f.publishedPost(t, storeID, body)
		for range 2 {
			c := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: "留言"}))
			ids = append(ids, c.ID)
		}
	}
	rev := []uuid.UUID{ids[3], ids[2], ids[1], ids[0]}
	var wg sync.WaitGroup
	errs := make(chan error, 64)
	run := func(fn func() error, n int) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range n {
				if err := fn(); err != nil {
					errs <- err
					return
				}
			}
		}()
	}
	reeval := func(in []uuid.UUID) func() error {
		return func() error {
			_, err := f.db.Conn(ctx).Exec(ctx, "SELECT social.reevaluate_comments($1::uuid[])", in)
			return err
		}
	}
	run(reeval(ids), 40)
	run(reeval(rev), 40)
	toggle := func(id uuid.UUID) func() error {
		return func() error {
			if err := f.hideRow(ctx, id); err != nil {
				return err
			}
			return f.restoreRows(ctx, id)
		}
	}
	run(toggle(ids[0]), 25)
	run(toggle(ids[3]), 25)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("並行的 reevaluate、隱藏、還原失敗（死結或違反計數的 CHECK）：%v", err)
	}
	f.assertStatsMatchTheDataset(t, "並行的 reevaluate")
}

// reevaluate 冪等：求值沒有變就什麼都不做，回傳旗標改變的筆數是 0；旗標被改壞時修回來並把計數一起修正。
func TestReevaluateIsIdempotentAndRepairsAFlag(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	b := f.newBuyer(t)
	p := f.publishedPost(t, storeID, "貼文")
	cm := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: "留言"}))
	changed := func() int64 {
		return f.scalar(t, "SELECT social.reevaluate_comments(ARRAY[$1::uuid])", cm.ID)
	}
	if n := changed(); n != 0 {
		t.Fatalf("旗標正確時重新求值改變了 %d 筆", n)
	}
	// 旗標被改成假（模擬維護漏了）：重新求值把它修回真，並把貼文的留言數補回。
	f.exec(t, "UPDATE social.comments SET counted = FALSE WHERE id = $1", cm.ID)
	f.exec(t, "UPDATE social.post_stats SET comment_count = 0 WHERE post_id = $1", p.ID)
	if n := changed(); n != 1 {
		t.Fatalf("旗標漂移時重新求值改變了 %d 筆", n)
	}
	if n := changed(); n != 0 {
		t.Fatalf("第二次重新求值改變了 %d 筆", n)
	}
	f.assertStatsMatchTheDataset(t, "旗標修復")
}

// 每日對帳先修旗標再覆寫計數：旗標與計數都被改壞時，一次對帳全部修好，並回報旗標修正的筆數與漂移。
func TestReconcileRepairsFlagsBeforeCounts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	b := f.newBuyer(t)
	p := f.publishedPost(t, storeID, "貼文")
	cm := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: "留言"}))
	e := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asStore(t, storeID), social.ForumInput{StoreID: storeID, Body: "公告"}))
	mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(b), social.ForumInput{StoreID: storeID, Body: "回覆", ParentID: &e.ID}))
	f.exec(t, "UPDATE social.comments SET counted = FALSE WHERE id = $1", cm.ID)
	f.exec(t, "UPDATE social.forum_entries SET counted = FALSE WHERE parent_id IS NOT NULL")
	f.exec(t, "UPDATE social.post_stats SET comment_count = 7 WHERE post_id = $1", p.ID)
	rep, err := f.module.Reconcile(ctx)
	must(t, err)
	if rep.FlagsFixed != 2 {
		t.Fatalf("修正的旗標 %d，預期 2（一則留言、一則討論區回覆）", rep.FlagsFixed)
	}
	if rep.Drifted < 2 {
		t.Fatalf("漂移 %d", rep.Drifted)
	}
	f.assertStatsMatchTheDataset(t, "對帳修旗標")
	rep, err = f.module.Reconcile(ctx)
	must(t, err)
	if rep.Drifted != 0 || rep.FlagsFixed != 0 {
		t.Fatalf("修好之後再對帳不該有漂移：%+v", rep)
	}
}
