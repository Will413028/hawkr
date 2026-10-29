//go:build integration

package social_test

import (
	"context"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/purge"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// racer 讓測試在「一個 transaction 持有尚未提交的改動」的時候，確定地讓另一個操作撞上它：hold 在自己的 transaction 內做完第一步，
// Start 啟動 op，Settled 等 op 完成或卡在鎖上（以 pg_stat_activity 看到有人在等 Lock），hold 再做第二步或直接結束（提交）。
// 沒有睡眠猜時機：修正有效時 op 卡在鎖上，沒有修正時 op 不等任何東西、直接完成，兩種情形測試都確定地走到下一步。
type racer struct {
	t      *testing.T
	f      *fixture
	op     func() error
	opDone chan error

	mu       sync.Mutex
	finished bool
	opErr    error
}

func (r *racer) Start() {
	r.opDone = make(chan error, 1)
	go func() { r.opDone <- r.op() }()
}

// Finished 回報 op 是不是已經完成（Settled 之後才有意義）。
func (r *racer) Finished() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.finished
}

func (r *racer) Settled() {
	r.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-r.opDone:
			r.mu.Lock()
			r.finished, r.opErr = true, err
			r.mu.Unlock()
			return
		default:
		}
		if r.f.scalar(r.t, "SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'") > 0 {
			var q string
			_ = r.f.db.Conn(context.Background()).QueryRow(context.Background(), "SELECT left(query, 60) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock' LIMIT 1").Scan(&q)
			r.t.Logf("op 卡在鎖上：%s", strings.ReplaceAll(q, "\n", " "))
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	r.t.Fatal("op 既沒有完成也沒有卡在鎖上")
}

// race 在一個 transaction 內執行 hold，op 由 hold 內的 Start 啟動；回傳 hold 與 op 的錯誤（hold 結束提交之後才等 op）。
func (f *fixture) race(t *testing.T, op func() error, hold func(ctx context.Context, r *racer) error) (holdErr, opErr error) {
	t.Helper()
	r := &racer{t: t, f: f, op: op}
	holdErr = f.db.InTx(context.Background(), func(ctx context.Context) error { return hold(ctx, r) })
	if r.opDone == nil {
		t.Fatal("hold 沒有呼叫 Start")
	}
	r.mu.Lock()
	done := r.finished
	opErr = r.opErr
	r.mu.Unlock()
	if !done {
		select {
		case opErr = <-r.opDone:
		case <-time.After(15 * time.Second):
			t.Fatal("hold 提交之後 op 還是沒有完成")
		}
	}
	return holdErr, opErr
}

func businessRejection(err error) bool {
	return slices.Contains([]apperr.Kind{apperr.KindNotFound, apperr.KindFailedPrecondition}, kindOf(err))
}

// F1：隱藏與還原要鎖住目標列，與刪除序列化。「是否計入」同時取決於內容的 deleted_at 與 hides，兩個寫入不序列化時各自的 trigger 都看不到對方
// 還沒提交的改動而重複加減（隱藏與刪除：重複扣，單一留言時違反 CHECK 讓其中一邊失敗；還原與刪除：多加一，留下已刪除的留言仍被計入）。
// 這裡讓刪除先持有尚未提交的改動，再讓隱藏或還原撞上它：有鎖時它們等到刪除提交、看到內容已刪除而回 NotFound，沒有鎖時直接完成並留下錯誤的計數。
func TestHideAndRestoreSerializeWithDelete(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name           string
		forum, restore bool
	}{
		{"隱藏與刪除留言", false, false},
		{"還原與刪除留言", false, true},
		{"隱藏與刪除討論區回覆", true, false},
		{"還原與刪除討論區回覆", true, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			f := setup(t)
			storeID, _ := f.exposableStore(t)
			b := f.newBuyer(t)
			var target social.Target
			var remove func(ctx context.Context) error
			if c.forum {
				top := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asStore(t, storeID), social.ForumInput{StoreID: storeID, Body: "公告"}))
				e := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(b), social.ForumInput{StoreID: storeID, Body: "回覆", ParentID: &top.ID}))
				target = social.Target{Kind: social.TargetForumEntry, ID: e.ID}
				remove = func(ctx context.Context) error { return f.module.DeleteForumEntry(ctx, f.asBuyer(b), e.ID) }
			} else {
				p := f.publishedPost(t, storeID, "貼文")
				cm := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: "留言"}))
				target = social.Target{Kind: social.TargetComment, ID: cm.ID}
				remove = func(ctx context.Context) error { return f.module.DeleteComment(ctx, f.asBuyer(b), cm.ID) }
			}
			hide := func() error {
				_, err := f.module.Hide(context.Background(), social.HideInput{Target: target, Category: "SPAM", AdminID: 1})
				return err
			}
			restore := func() error {
				_, err := f.module.Restore(context.Background(), social.RestoreInput{Target: target, AdminID: 1, Reason: "誤判"})
				return err
			}
			op := hide
			if c.restore {
				must(t, hide())
				op = restore
			}
			holdErr, opErr := f.race(t, op, func(ctx context.Context, r *racer) error {
				if err := remove(ctx); err != nil {
					return err
				}
				r.Start()
				r.Settled()
				return nil
			})
			must(t, holdErr)
			t.Logf("隱藏或還原的結果：%v", opErr)
			if opErr != nil && !businessRejection(opErr) {
				t.Fatalf("隱藏或還原與刪除並行，只能成功或因內容已刪除被拒絕：%v", opErr)
			}
			f.assertStatsMatchTheDataset(t, c.name)
			if !c.forum {
				if n := f.scalar(t, "SELECT comment_count FROM social.post_stats WHERE post_id = (SELECT post_id FROM social.comments WHERE id = $1)", target.ID); n != 0 {
					t.Fatalf("留言已刪除，貼文的留言數 %d", n)
				}
			}
		})
	}
}

// F3：清除 job 依固定順序鎖 stats 列（先貼文再留言，各自依 ID），和 trigger 的順序相同。這裡讓另一個 transaction 先鎖貼文的 stats（刪除留言的
// trigger 的第一步），清除 job 撞上它之後，該 transaction 再要留言的 stats（trigger 的第二步）：順序固定時清除 job 在最前面就卡住、什麼也沒握，
// 兩邊都完成；清除 job 先握住留言的 stats 再等貼文的 stats 就會死結，Postgres 中止其中一邊。
func TestPurgeLocksStatsInTheSameOrderAsTheTriggers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	p := f.publishedPost(t, storeID, "貼文")
	b := f.newBuyer(t)
	top := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asStore(t, storeID), social.CommentInput{PostID: p.ID, Body: "店家的留言"}))
	mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: "買家的留言"}))
	mustV[bool](t)(f.module.LikeComment(ctx, f.asBuyer(b), top.ID))
	must(t, f.buyers.BeginDeletion(ctx, b.BuyerID))

	holdErr, opErr := f.race(t, func() error { return f.module.PurgeBuyerContent(context.Background(), b.BuyerID) },
		func(ctx context.Context, r *racer) error {
			conn := f.db.Conn(ctx)
			if _, err := conn.Exec(ctx, "SELECT 1 FROM social.post_stats WHERE post_id = $1 FOR UPDATE", p.ID); err != nil {
				return err
			}
			r.Start()
			r.Settled()
			_, err := conn.Exec(ctx, "SELECT 1 FROM social.comment_stats WHERE comment_id = $1 FOR UPDATE", top.ID)
			return err
		})
	must(t, holdErr)
	must(t, opErr)
	f.assertStatsMatchTheDataset(t, "鎖順序")
}

// F8：買家的寫入（讚、分享、留言）與刪帳申請序列化。寫入在自己的 transaction 內持有買家列的共享鎖，BeginDeletion 的排他鎖要等它提交，
// 所以申請完成時寫入一定已經提交，清除 job（申請提交後立刻跑）一定看得到它。沒有鎖時申請先提交、清除 job 先跑完，寫入才提交，那一列永遠留著。
func TestBuyerWritesSerializeWithDeletionRequest(t *testing.T) {
	t.Parallel()
	for _, c := range []string{"讚", "分享", "留言"} {
		t.Run(c, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			f := setup(t)
			storeID, _ := f.exposableStore(t)
			p := f.publishedPost(t, storeID, "貼文")
			b := f.newBuyer(t)
			write := func(ctx context.Context) error {
				var err error
				switch c {
				case "讚":
					_, err = f.module.LikePost(ctx, b.BuyerID, p.ID)
				case "分享":
					_, err = f.module.RecordShare(ctx, b.BuyerID, p.ID)
				default:
					_, err = f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: "留言"})
				}
				return err
			}
			purged := false
			holdErr, opErr := f.race(t, func() error { return f.buyers.BeginDeletion(context.Background(), b.BuyerID) },
				func(ctx context.Context, r *racer) error {
					if err := write(ctx); err != nil {
						return err
					}
					r.Start()
					r.Settled()
					if r.Finished() {
						// 申請已經提交，清除 job 在寫入提交之前就跑了。
						purged = true
						return f.module.PurgeBuyerContent(context.Background(), b.BuyerID)
					}
					return nil
				})
			must(t, holdErr)
			must(t, opErr)
			if !purged {
				must(t, f.module.PurgeBuyerContent(ctx, b.BuyerID))
			}
			if n := f.scalar(t, `SELECT (SELECT count(*) FROM social.post_likes WHERE buyer_id = $1)
				+ (SELECT count(*) FROM social.post_shares WHERE buyer_id = $1)
				+ (SELECT count(*) FROM social.comments WHERE actor_kind = 'BUYER' AND actor_id = $1 AND deleted_at IS NULL)`, b.BuyerID); n != 0 {
				t.Fatalf("清除之後買家還留著 %d 列寫入", n)
			}
			f.assertStatsMatchTheDataset(t, c)
		})
	}
	t.Run("申請之後的寫入被拒絕", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		f := setup(t)
		storeID, _ := f.exposableStore(t)
		p := f.publishedPost(t, storeID, "貼文")
		b := f.newBuyer(t)
		must(t, f.buyers.BeginDeletion(ctx, b.BuyerID))
		if _, err := f.module.LikePost(ctx, b.BuyerID, p.ID); kindOf(err) != apperr.KindFailedPrecondition {
			t.Fatalf("按讚：%v", err)
		}
		if _, err := f.module.RecordShare(ctx, b.BuyerID, p.ID); kindOf(err) != apperr.KindFailedPrecondition {
			t.Fatalf("分享：%v", err)
		}
		if _, err := f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: "x"}); kindOf(err) != apperr.KindFailedPrecondition {
			t.Fatalf("留言：%v", err)
		}
	})
}

// F2：申請刪除之後、清除之前被作者刪掉的留言。申請的 transaction 已經讓旗標由真轉假、計數扣掉（DSYNC），之後的刪除重新求值還是不計入，
// 差額是 0，計數不會被扣兩次（扣兩次會違反 stats 的 CHECK）；清除 job 之後結果不變，討論區的父留言同理。
func TestPurgeKeepsCountsExactForContentDeletedBetweenRequestAndPurge(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	p := f.publishedPost(t, storeID, "貼文")
	b, other := f.newBuyer(t), f.newBuyer(t)
	cm := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: "留言"}))
	top := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(other), social.ForumInput{StoreID: storeID, Body: "討論"}))
	reply := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(b), social.ForumInput{StoreID: storeID, Body: "回覆", ParentID: &top.ID}))
	must(t, f.buyers.BeginDeletion(ctx, b.BuyerID))
	if n := f.scalar(t, "SELECT comment_count FROM social.post_stats WHERE post_id = $1", p.ID); n != 0 {
		t.Fatalf("前提：申請的 transaction 已讓旗標由真轉假，留言數 %d", n)
	}
	must(t, f.module.DeleteComment(ctx, f.asBuyer(b), cm.ID))
	must(t, f.module.DeleteForumEntry(ctx, f.asBuyer(b), reply.ID))
	if n := f.scalar(t, "SELECT comment_count FROM social.post_stats WHERE post_id = $1", p.ID); n != 0 {
		t.Fatalf("刪除重新求值，旗標本來就是假，差額 0，留言數 %d，預期 0", n)
	}
	if n := f.scalar(t, "SELECT reply_count FROM social.forum_entry_stats WHERE forum_entry_id = $1", top.ID); n != 0 {
		t.Fatalf("討論區父留言的回覆數 %d，預期 0", n)
	}
	must(t, f.module.PurgeBuyerContent(ctx, b.BuyerID))
	if n := f.scalar(t, "SELECT comment_count FROM social.post_stats WHERE post_id = $1", p.ID); n != 0 {
		t.Fatalf("清除之後貼文的留言數 %d，預期 0", n)
	}
	if n := f.scalar(t, "SELECT reply_count FROM social.forum_entry_stats WHERE forum_entry_id = $1", top.ID); n != 0 {
		t.Fatalf("清除之後討論區父留言的回覆數 %d，預期 0", n)
	}
	f.assertStatsMatchTheDataset(t, "清除之後")
}

// 買家刪帳申請的 transaction 內，訂閱者把他記進 inactive_authors 並讓他的內容重新求值：旗標與計數同時改，讀取立即看不到他的內容
// （不等清除 job，DSYNC）。再求值一次沒有東西改（冪等），清除 job 之後計數仍然對。
func TestBuyerDeletionRequestUncountsTheContentInTheSameTransaction(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	p := f.publishedPost(t, storeID, "貼文")
	b := f.newBuyer(t)
	mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: "留言一"}))
	mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: "留言二"}))
	if n := f.scalar(t, "SELECT comment_count FROM social.post_stats WHERE post_id = $1", p.ID); n != 2 {
		t.Fatalf("前提：申請之前留言數 %d", n)
	}
	must(t, f.buyers.BeginDeletion(ctx, b.BuyerID))
	if n := f.scalar(t, "SELECT count(*) FROM social.inactive_authors WHERE buyer_id = $1", b.BuyerID); n != 1 {
		t.Fatalf("申請的 transaction 內要記下複本：%d", n)
	}
	if n := f.scalar(t, "SELECT comment_count FROM social.post_stats WHERE post_id = $1", p.ID); n != 0 {
		t.Fatalf("申請之後、job 之前，留言數 %d，預期 0（訂閱者在同一個 transaction 重新求值）", n)
	}
	if n := f.scalar(t, "SELECT count(*) FROM social.comments WHERE actor_id = $1 AND counted", b.BuyerID); n != 0 {
		t.Fatalf("申請之後旗標都要是假：%d", n)
	}
	if n := f.scalar(t, "SELECT social.reevaluate_comments(ARRAY(SELECT id FROM social.comments WHERE actor_id = $1))", b.BuyerID); n != 0 {
		t.Fatalf("再求值一次沒有東西改（冪等）：%d", n)
	}
	must(t, f.module.PurgeBuyerContent(ctx, b.BuyerID))
	f.assertStatsMatchTheDataset(t, "清除之後")
}

// F4：買家寫的回覆（目前只有資料列能造出來：貼文的回覆限店家）刪帳之後，父留言的回覆數也要跟著扣（清除 job 的重新求值與軟刪都更新父留言的 stats）。
func TestPurgeCorrectsTheReplyCountOfTheParentComment(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	p := f.publishedPost(t, storeID, "貼文")
	b := f.newBuyer(t)
	top := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asStore(t, storeID), social.CommentInput{PostID: p.ID, Body: "店家的留言"}))
	f.exec(t, `INSERT INTO social.comments (id, post_id, parent_id, reply_to_id, actor_kind, actor_id, body, created_at, updated_at)
		VALUES ($1, $2, $3, $3, 'BUYER', $4, '買家的回覆', now(), now())`, uuidNew(), p.ID, top.ID, b.BuyerID)
	if n := f.scalar(t, "SELECT reply_count FROM social.comment_stats WHERE comment_id = $1", top.ID); n != 1 {
		t.Fatalf("前提：回覆數 %d", n)
	}
	must(t, f.buyers.BeginDeletion(ctx, b.BuyerID))
	must(t, f.module.PurgeBuyerContent(ctx, b.BuyerID))
	if n := f.scalar(t, "SELECT reply_count FROM social.comment_stats WHERE comment_id = $1", top.ID); n != 0 {
		t.Fatalf("清除之後父留言的回覆數 %d，預期 0", n)
	}
	f.assertStatsMatchTheDataset(t, "清除之後")
}

// F5：清除 job 以 BuyerID（StoreID）去重：同一位買家排兩次只有一個 job，不同買家各一個；執行中的也算。
func TestPurgeJobsAreUniquePerSubject(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	client, err := jobs.NewInsertOnly(f.db)
	must(t, err)
	count := func(kind string) int64 {
		return f.scalar(t, "SELECT count(*) FROM river.river_job WHERE kind = $1", kind)
	}
	must(t, f.db.InTx(ctx, func(ctx context.Context) error {
		for _, id := range []int64{7, 7, 8} {
			if err := purge.EnqueueBuyer(ctx, client, id); err != nil {
				return err
			}
		}
		for _, id := range []int64{3, 3, 3, 4} {
			if err := purge.EnqueueStore(ctx, client, id); err != nil {
				return err
			}
		}
		return nil
	}))
	if n := count("social.purge_buyer_content"); n != 2 {
		t.Fatalf("買家 7 排兩次、買家 8 一次，要 2 個 job：%d", n)
	}
	if n := count("social.purge_store_content"); n != 2 {
		t.Fatalf("店家 3 排三次、店家 4 一次，要 2 個 job：%d", n)
	}
	f.exec(t, "UPDATE river.river_job SET state = 'running', attempted_at = now(), attempt = 1, attempted_by = ARRAY['w'] WHERE kind = 'social.purge_buyer_content' AND args->>'buyer_id' = '7'")
	must(t, f.db.InTx(ctx, func(ctx context.Context) error { return purge.EnqueueBuyer(ctx, client, 7) }))
	if n := count("social.purge_buyer_content"); n != 2 {
		t.Fatalf("執行中的 job 也算重複：%d", n)
	}
	f.exec(t, "UPDATE river.river_job SET state = 'completed', finalized_at = now() WHERE kind = 'social.purge_buyer_content' AND args->>'buyer_id' = '7'")
	must(t, f.db.InTx(ctx, func(ctx context.Context) error { return purge.EnqueueBuyer(ctx, client, 7) }))
	if n := count("social.purge_buyer_content"); n != 3 {
		t.Fatalf("已完成的不算重複，之後的申請要再排一個：%d", n)
	}
}

// F9：reply_to 只能是父本身，或同一串（同一個父）底下的另一則回覆；服務層回有意義的錯誤，DB 的複合 FK 是後備。
func TestReplyToStaysInTheThread(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	st := f.asStore(t, storeID)
	p := f.publishedPost(t, storeID, "貼文")
	a := mustV[social.Comment](t)(f.module.CreateComment(ctx, st, social.CommentInput{PostID: p.ID, Body: "串 A"}))
	bTop := mustV[social.Comment](t)(f.module.CreateComment(ctx, st, social.CommentInput{PostID: p.ID, Body: "串 B"}))
	a1 := mustV[social.Comment](t)(f.module.CreateComment(ctx, st, social.CommentInput{PostID: p.ID, Body: "A 的回覆", ParentID: &a.ID}))
	b1 := mustV[social.Comment](t)(f.module.CreateComment(ctx, st, social.CommentInput{PostID: p.ID, Body: "B 的回覆", ParentID: &bTop.ID}))
	if _, err := f.module.CreateComment(ctx, st, social.CommentInput{PostID: p.ID, Body: "x", ParentID: &a.ID, ReplyToID: &b1.ID}); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("reply_to 在另一串（回覆）：%v", err)
	}
	if _, err := f.module.CreateComment(ctx, st, social.CommentInput{PostID: p.ID, Body: "x", ParentID: &a.ID, ReplyToID: &bTop.ID}); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("reply_to 是另一串的頂層：%v", err)
	}
	for _, rt := range []*uuidT{&a.ID, &a1.ID} {
		got, err := f.module.CreateComment(ctx, st, social.CommentInput{PostID: p.ID, Body: "同一串", ParentID: &a.ID, ReplyToID: rt})
		if err != nil || got.ReplyToID == nil || *got.ReplyToID != *rt {
			t.Fatalf("同一串可以：%+v %v", got, err)
		}
	}
	// 回覆的 parent 傳另一則回覆時壓成頂層，reply_to 以壓平後的串為準。
	if _, err := f.module.CreateComment(ctx, st, social.CommentInput{PostID: p.ID, Body: "x", ParentID: &a1.ID, ReplyToID: &b1.ID}); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("parent 壓平之後 reply_to 仍在另一串：%v", err)
	}
	ins := func(table, container string, containerID, parent, replyTo any) error {
		_, err := f.db.Conn(ctx).Exec(ctx, `INSERT INTO social.`+table+` (id, `+container+`, parent_id, reply_to_id, actor_kind, actor_id, body, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'STORE', 1, 'x', now(), now())`, uuidNew(), containerID, parent, replyTo)
		return err
	}
	if err := ins("comments", "post_id", p.ID, a.ID, b1.ID); err == nil || !strings.Contains(err.Error(), "comments_reply_to_fk") {
		t.Fatalf("DB 要拒絕另一串的 reply_to：%v", err)
	}
	if err := ins("comments", "post_id", p.ID, a.ID, a1.ID); err != nil {
		t.Fatalf("DB：同一串可以：%v", err)
	}

	f1 := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, st, social.ForumInput{StoreID: storeID, Body: "討論 A"}))
	f2 := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, st, social.ForumInput{StoreID: storeID, Body: "討論 B"}))
	f1r := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, st, social.ForumInput{StoreID: storeID, Body: "A 的回覆", ParentID: &f1.ID}))
	f2r := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, st, social.ForumInput{StoreID: storeID, Body: "B 的回覆", ParentID: &f2.ID}))
	if _, err := f.module.CreateForumEntry(ctx, st, social.ForumInput{StoreID: storeID, Body: "x", ParentID: &f1.ID, ReplyToID: &f2r.ID}); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("討論區 reply_to 在另一串：%v", err)
	}
	if _, err := f.module.CreateForumEntry(ctx, st, social.ForumInput{StoreID: storeID, Body: "同一串", ParentID: &f1.ID, ReplyToID: &f1r.ID}); err != nil {
		t.Fatalf("討論區同一串可以：%v", err)
	}
	if err := ins("forum_entries", "store_id", storeID, f1.ID, f2r.ID); err == nil || !strings.Contains(err.Error(), "forum_entries_reply_to_fk") {
		t.Fatalf("DB 要拒絕另一串的 reply_to（討論區）：%v", err)
	}
}

// F6：隱藏、還原、警告的理由與來源先去頭尾空白；只有空白的還原原因被拒絕。DB 的 CHECK 是後備（cmd/hawkr 的 trimmedColumns）。
func TestModerationReasonsAreTrimmed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	p := f.publishedPost(t, storeID, "貼文")
	b := f.newBuyer(t)
	cm := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: "留言"}))
	target := social.Target{Kind: social.TargetComment, ID: cm.ID}
	h := mustV[social.Hide](t)(f.module.Hide(ctx, social.HideInput{Target: target, Category: "SPAM", AdminID: 1, Text: "  廣告  ", SourceRef: " case-1\t"}))
	if h.Text != "廣告" || h.SourceRef != "case-1" {
		t.Fatalf("隱藏的理由與來源要去頭尾空白：%+v", h)
	}
	if _, err := f.module.Restore(ctx, social.RestoreInput{Target: target, AdminID: 1, Reason: " \t　"}); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("只有空白的還原原因：%v", err)
	}
	r := mustV[social.Hide](t)(f.module.Restore(ctx, social.RestoreInput{Target: target, AdminID: 1, Reason: " 誤判 "}))
	if f.scalar(t, "SELECT count(*) FROM social.hides WHERE id = $1 AND restore_reason = '誤判'", r.ID) != 1 {
		t.Fatal("還原的原因要去頭尾空白")
	}
	s := mustV[social.Sanction](t)(f.module.Warn(ctx, social.WarnInput{BuyerID: b.BuyerID, Category: "SPAM", AdminID: 1, Text: " 警告 ", SourceRef: "  "}))
	if s.Text != "警告" || s.SourceRef != "" {
		t.Fatalf("警告：%+v", s)
	}
}

// DR-A1：批次重算先依 ID 鎖住這批的 stats 列、再用另一個語句計數。讓一個讚（或留言）先持有尚未提交的改動（讚列已寫、stats 已加一），
// 再讓全量重算撞上它：有鎖時重算在鎖那一步等到寫入提交，之後的計數看得到它，結果與資料一致；沒有鎖時計數語句用的是寫入提交之前的 snapshot，
// 等到寫入提交才覆寫，把那一個讚蓋掉。計數要先有漂移（重算才會覆寫；沒有漂移時用舊 snapshot 算出的值與現值相同，不會寫入，看不出差別）。
func TestBatchRecountDoesNotOverwriteAConcurrentWrite(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name  string
		write func(f *fixture, ctx context.Context, postID uuid.UUID, buyerID int64) error
	}{
		{"讚", func(f *fixture, ctx context.Context, postID uuid.UUID, buyerID int64) error {
			_, err := f.module.LikePost(ctx, buyerID, postID)
			return err
		}},
		{"留言", func(f *fixture, ctx context.Context, postID uuid.UUID, buyerID int64) error {
			_, err := f.module.CreateComment(ctx, social.Caller{BuyerID: buyerID}, social.CommentInput{PostID: postID, Body: "並行的留言"})
			return err
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			f := setup(t)
			storeID, _ := f.exposableStore(t)
			p := f.publishedPost(t, storeID, "貼文")
			b := f.newBuyer(t)
			f.exec(t, "UPDATE social.post_stats SET like_count = 5, comment_count = 5 WHERE post_id = $1", p.ID)
			holdErr, opErr := f.race(t, func() error { _, err := f.module.RecountAll(context.Background()); return err },
				func(ctx context.Context, r *racer) error {
					if err := c.write(f, ctx, p.ID, b.BuyerID); err != nil {
						return err
					}
					r.Start()
					r.Settled()
					return nil
				})
			must(t, holdErr)
			must(t, opErr)
			got := mustV[social.Post](t)(f.module.PublicPost(ctx, social.Caller{}, p.ID))
			if got.Stats.Likes+got.Stats.Comments != 1 {
				t.Fatalf("並行的寫入被重算蓋掉了：%+v", got.Stats)
			}
			f.assertStatsMatchTheDataset(t, "批次重算與並行的"+c.name)
		})
	}
}

// R3：留言與討論區留言的編輯先鎖買家列、再鎖內容列（鎖順序 identity → buyer → seller → social）。刪帳申請持有買家列的排他鎖，再由訂閱者鎖
// 作者的內容：編輯若先鎖內容列再等買家列，兩邊互相等待而死結。這裡讓一個 transaction 持有買家列的排他鎖（BeginDeletion 的第一步），編輯撞上它
// 並卡在鎖上，此時內容列必須還沒被編輯握住（另一條連線 FOR UPDATE NOWAIT 拿得到）；順序反過來時內容列已被握住，NOWAIT 回 lock_not_available。
func TestEditLocksTheBuyerBeforeTheContentRow(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"留言", "討論區留言"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			f := setup(t)
			storeID, _ := f.exposableStore(t)
			p := f.publishedPost(t, storeID, "貼文")
			b := f.newBuyer(t)
			var op func() error
			var contentTable string
			var id uuid.UUID
			if kind == "留言" {
				cm := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: "原文"}))
				id, contentTable = cm.ID, "social.comments"
				op = func() error {
					_, err := f.module.UpdateComment(context.Background(), f.asBuyer(b), cm.ID, "改過")
					return err
				}
			} else {
				e := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(b), social.ForumInput{StoreID: storeID, Body: "原文"}))
				id, contentTable = e.ID, "social.forum_entries"
				op = func() error {
					_, err := f.module.UpdateForumEntry(context.Background(), f.asBuyer(b), e.ID, "改過")
					return err
				}
			}
			holdErr, opErr := f.race(t, op, func(ctx context.Context, r *racer) error {
				if _, err := f.db.Conn(ctx).Exec(ctx, "SELECT 1 FROM buyer.buyers WHERE buyer_id = $1 FOR UPDATE", b.BuyerID); err != nil {
					return err
				}
				r.Start()
				r.Settled()
				if r.Finished() {
					t.Error("編輯沒有被買家列的排他鎖擋住")
					return nil
				}
				// 另一條連線（不在 hold 的 transaction 內）嘗試鎖內容列。
				if err := f.db.InTx(context.Background(), func(ctx context.Context) error {
					_, err := f.db.Conn(ctx).Exec(ctx, "SELECT 1 FROM "+contentTable+" WHERE id = $1 FOR UPDATE NOWAIT", id)
					return err
				}); err != nil {
					t.Errorf("編輯卡在買家列時已握住內容列（鎖順序反了）：%v", err)
				}
				return nil
			})
			must(t, holdErr)
			// 排他鎖提交後買家仍是 ACTIVE（沒有真的申請刪除），編輯完成。
			must(t, opErr)
		})
	}
}

// R1：重算的第二步（把計數覆寫成旗標的加總）撞上買家刪帳。重算先修旗標、再鎖 stats 範圍；卡在 stats 鎖的時候，買家的刪帳申請提交（訂閱者在同一個
// transaction 讓旗標由真轉假、計數扣掉）。重算放行後算出的新計數必須等於旗標的加總，所以與申請後的計數相同（漂移 0），之後的清除 job 也不會把計數扣成負數
// （違反 stats 的 CHECK）。舊設計的重算在讀取時用買家的現況重新推導，而旗標要等清除 job 才跟上，兩者在這個空窗不一致。
// 做法：stats 範圍的第一列是一篇沒有留言的貼文（重算的第一步碰不到它，第二步的範圍鎖第一個就等它）。
func TestRecountSumsTheStoredFlagsWhenABuyerLeavesBetweenTheTwoSteps(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	empty := f.publishedPost(t, storeID, "沒有留言的貼文")
	p := f.publishedPost(t, storeID, "貼文")
	if empty.ID.Compare(p.ID) >= 0 {
		t.Fatalf("前提：沒有留言的貼文的 ID 要排在前面（%s %s）", empty.ID, p.ID)
	}
	b := f.newBuyer(t)
	mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: "留言一"}))
	mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: "留言二"}))
	var rep social.RecountReport
	holdErr, opErr := f.race(t, func() error {
		var err error
		rep, err = f.module.RecountAll(context.Background())
		return err
	}, func(ctx context.Context, r *racer) error {
		if _, err := f.db.Conn(ctx).Exec(ctx, "SELECT 1 FROM social.post_stats WHERE post_id = $1 FOR UPDATE", empty.ID); err != nil {
			return err
		}
		r.Start()
		r.Settled()
		if r.Finished() {
			t.Error("重算的第二步沒有卡在 stats 鎖上")
			return nil
		}
		return f.buyers.BeginDeletion(ctx, b.BuyerID)
	})
	must(t, holdErr)
	must(t, opErr)
	if rep.Drifted != 0 {
		t.Fatalf("申請在同一個 transaction 改好旗標與計數，重算（旗標的加總）不該發現漂移：%+v", rep.Drifts)
	}
	if n := f.scalar(t, "SELECT comment_count FROM social.post_stats WHERE post_id = $1", p.ID); n != 0 {
		t.Fatalf("重算之後留言數 %d，預期 0", n)
	}
	must(t, f.module.PurgeBuyerContent(ctx, b.BuyerID))
	f.assertStatsMatchTheDataset(t, "重算與刪帳交錯之後")
}

// Q2／FDR3、A7-1a C4-6：買家刪帳的訂閱者（MarkAuthorInactive）以一次集合呼叫 reevaluate_*，函式依固定順序鎖 stats（先貼文、再父留言，各依 ID），和單列寫入的
// trigger 同序；它不呼叫 stats.Service.Lock（函式自己鎖整批，Lock 取的是同一批同一個順序，沒有行為差異）。所以這個測試釘住的是 reevaluate 內部的順序，
// 不是 Lock。這裡讓另一個 transaction 先鎖貼文的 stats（單列寫入的第一步），刪帳申請撞上它並卡住（還什麼都沒握），該 transaction 再要父留言的 stats
// （單列寫入的第二步）：順序固定時兩邊都完成；訂閱者先握父留言的 stats 再等貼文的 stats 就會死結，Postgres 中止其中一邊。
func TestMarkAuthorInactiveLocksStatsInTheSameOrderAsTheTriggers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	p := f.publishedPost(t, storeID, "貼文")
	b := f.newBuyer(t)
	top := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asStore(t, storeID), social.CommentInput{PostID: p.ID, Body: "店家的留言"}))
	f.exec(t, `INSERT INTO social.comments (id, post_id, parent_id, reply_to_id, actor_kind, actor_id, body, created_at, updated_at)
		VALUES ($1, $2, $3, $3, 'BUYER', $4, '買家的回覆', now(), now())`, uuidNew(), p.ID, top.ID, b.BuyerID)
	holdErr, opErr := f.race(t, func() error { return f.buyers.BeginDeletion(context.Background(), b.BuyerID) },
		func(ctx context.Context, r *racer) error {
			conn := f.db.Conn(ctx)
			if _, err := conn.Exec(ctx, "SELECT 1 FROM social.post_stats WHERE post_id = $1 FOR UPDATE", p.ID); err != nil {
				return err
			}
			r.Start()
			r.Settled()
			_, err := conn.Exec(ctx, "SELECT 1 FROM social.comment_stats WHERE comment_id = $1 FOR UPDATE", top.ID)
			return err
		})
	must(t, holdErr)
	must(t, opErr)
	f.assertStatsMatchTheDataset(t, "鎖順序")
}
