//go:build integration

package social_test

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/stats"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/contentkind"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/revalidate"
)

// takedowns 是內容下架措施在這組測試裡的替身：seller 的措施列（只填 pub_content_takedowns 讀的欄位，CHECK 與外鍵放寬）直接寫，
// 事件由測試依真實 seller 的形狀發布（seller 自己的規則與真實的建立入口在 seller、httpapi 的測試）。
type takedowns struct {
	f *fixture
}

func (f *fixture) takedowns(t *testing.T) takedowns {
	t.Helper()
	events.Subscribe(f.bus, f.module.OnMeasureCreated)
	events.Subscribe(f.bus, f.module.OnMeasureRevoked)
	f.exec(t, `DO $$ DECLARE r record; BEGIN
		FOR r IN SELECT conrelid::regclass AS t, conname FROM pg_constraint
		         WHERE conrelid IN ('seller.measures'::regclass, 'seller.measure_contents'::regclass) AND contype IN ('c', 'f')
		LOOP EXECUTE format('ALTER TABLE %s DROP CONSTRAINT %I', r.t, r.conname); END LOOP; END $$`)
	return takedowns{f: f}
}

func (d takedowns) seed(t *testing.T, storeID int64, refs []seller.ContentRef) uuid.UUID {
	t.Helper()
	id := uuid.NewV7()
	d.f.exec(t, `INSERT INTO seller.measures (id, account_id, store_id, kind, basis, statement_to_seller, starts_at, status, current_revision, created_by_admin_id, created_at, updated_at)
		SELECT $1, account_id, store_id, 'CONTENT_TAKEDOWN', 'VIOLATION', '下架', $3, 'ENFORCED', 1, 9, $3, $3 FROM seller.stores WHERE store_id = $2`, id, storeID, d.f.clock())
	for _, r := range refs {
		d.f.exec(t, "INSERT INTO seller.measure_contents (measure_id, content_kind, content_id) VALUES ($1, $2, $3)", id, string(r.Kind), r.ID)
	}
	return id
}

func (d takedowns) create(t *testing.T, storeID int64, refs ...seller.ContentRef) uuid.UUID {
	t.Helper()
	id := d.seed(t, storeID, refs)
	d.f.jobs.reset()
	must(t, d.f.db.InTx(context.Background(), func(ctx context.Context) error {
		return events.Publish(ctx, d.f.bus, seller.MeasureCreated{MeasureID: id, StoreID: storeID, OccurredAt: d.f.clock(), Contents: refs})
	}))
	return id
}

func (d takedowns) revoke(t *testing.T, storeID int64, id uuid.UUID, refs ...seller.ContentRef) {
	t.Helper()
	d.f.exec(t, "UPDATE seller.measures SET status = 'REVOKED' WHERE id = $1", id)
	d.f.jobs.reset()
	must(t, d.f.db.InTx(context.Background(), func(ctx context.Context) error {
		return events.Publish(ctx, d.f.bus, seller.MeasureRevoked{MeasureID: id, StoreID: storeID, OccurredAt: d.f.clock(), Contents: refs})
	}))
}

func ref(kind seller.ContentKind, id uuid.UUID) seller.ContentRef {
	return seller.ContentRef{Kind: kind, ID: id}
}

type takedownWorld struct {
	storeID int64
	post    social.Post
	other   social.Post
	comment social.Comment
	entry   social.ForumEntry
}

func (f *fixture) takedownWorld(t *testing.T) takedownWorld {
	t.Helper()
	ctx := context.Background()
	storeID, _ := f.exposableStore(t)
	w := takedownWorld{storeID: storeID}
	w.post = f.publishedPost(t, storeID, "會被下架的貼文", "organic")
	w.other = f.publishedPost(t, storeID, "另一篇貼文", "organic")
	w.comment = mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asStore(t, storeID), social.CommentInput{PostID: w.other.ID, Body: "會被下架的留言"}))
	w.entry = mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asStore(t, storeID), social.ForumInput{StoreID: storeID, Body: "會被下架的討論"}))
	return w
}

func (f *fixture) postVisible(t *testing.T, id uuid.UUID) (get, feed, store, tag bool) {
	t.Helper()
	ctx := context.Background()
	if _, err := f.module.PublicPost(ctx, social.Caller{}, id); err == nil {
		get = true
	} else if kindOf(err) != apperr.KindNotFound {
		t.Fatal(err)
	}
	in := func(posts []social.Post) bool {
		return slices.ContainsFunc(posts, func(p social.Post) bool { return p.ID == id })
	}
	feedPage := mustV[social.PostPage](t)(f.module.ListFeed(ctx, social.Caller{}, nil, 50))
	storePage := mustV[social.PostPage](t)(f.module.ListStorePosts(ctx, social.Caller{}, f.storeOf(t, id), nil, 50))
	tagPage := mustV[social.PostPage](t)(f.module.ListHashtagPosts(ctx, social.Caller{}, "organic", nil, 50))
	return get, in(feedPage.Posts), in(storePage.Posts), in(tagPage.Posts)
}

func (f *fixture) storeOf(t *testing.T, postID uuid.UUID) int64 {
	t.Helper()
	return f.scalar(t, "SELECT store_id FROM social.posts WHERE id = $1", postID)
}

// 建立內容下架後，貼文（單篇、動態牆、店面、標籤）、留言與討論區留言的公開讀取都不含該內容，計數同步扣掉；撤銷後立即回來。
func TestContentTakedownHidesTheContentAndRevokeRestoresItImmediately(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	td := f.takedowns(t)
	w := f.takedownWorld(t)
	refs := []seller.ContentRef{ref(seller.ContentPost, w.post.ID), ref(seller.ContentComment, w.comment.ID), ref(seller.ContentForumEntry, w.entry.ID)}

	if get, feed, st, tag := f.postVisible(t, w.post.ID); !get || !feed || !st || !tag {
		t.Fatalf("前提：下架前貼文可讀 %v %v %v %v", get, feed, st, tag)
	}
	id := td.create(t, w.storeID, refs...)

	if get, feed, st, tag := f.postVisible(t, w.post.ID); get || feed || st || tag {
		t.Fatalf("下架後貼文仍可讀：單篇 %v 動態牆 %v 店面 %v 標籤 %v", get, feed, st, tag)
	}
	if got := f.readable(t, w.other.ID, w.storeID); got.comments != 0 || got.forum != 0 || !got.post {
		t.Fatalf("下架後留言與討論區留言不該出現，另一篇貼文照常可讀：%+v", got)
	}
	if n := f.scalar(t, "SELECT comment_count FROM social.post_stats WHERE post_id = $1", w.other.ID); n != 0 {
		t.Fatalf("下架的留言不計入貼文的留言數：%d", n)
	}
	if _, err := f.module.ListComments(ctx, social.Caller{}, w.post.ID, nil, 10); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("下架貼文底下的留言跟著父貼文：%v", err)
	}
	f.assertStatsMatchTheDataset(t, "下架後")

	td.revoke(t, w.storeID, id, refs...)
	if get, feed, st, tag := f.postVisible(t, w.post.ID); !get || !feed || !st || !tag {
		t.Fatalf("撤銷後貼文立即回來 %v %v %v %v", get, feed, st, tag)
	}
	if got := f.readable(t, w.other.ID, w.storeID); got.comments != 1 || got.forum != 1 {
		t.Fatalf("撤銷後留言與討論區留言立即回來：%+v", got)
	}
	if n := f.scalar(t, "SELECT comment_count FROM social.post_stats WHERE post_id = $1", w.other.ID); n != 1 {
		t.Fatalf("撤銷後留言數回來：%d", n)
	}
	if n := f.scalar(t, "SELECT count(*) FROM social.content_takedowns"); n != 0 {
		t.Fatalf("撤銷後複本全刪：%d", n)
	}
	f.assertStatsMatchTheDataset(t, "撤銷後")
}

// 同一則內容被兩個措施下架：撤銷其中一個仍然隱藏，兩個都撤銷才回來。
func TestRevokingOneOfTwoTakedownsKeepsTheContentHidden(t *testing.T) {
	t.Parallel()
	f := setup(t)
	td := f.takedowns(t)
	w := f.takedownWorld(t)
	refs := []seller.ContentRef{ref(seller.ContentPost, w.post.ID), ref(seller.ContentComment, w.comment.ID), ref(seller.ContentForumEntry, w.entry.ID)}
	first, second := td.create(t, w.storeID, refs...), td.create(t, w.storeID, refs...)

	td.revoke(t, w.storeID, first, refs...)
	if get, feed, _, _ := f.postVisible(t, w.post.ID); get || feed {
		t.Fatal("還有另一個措施，貼文要維持隱藏")
	}
	if got := f.readable(t, w.other.ID, w.storeID); got.comments != 0 || got.forum != 0 {
		t.Fatalf("還有另一個措施，留言與討論區留言維持隱藏：%+v", got)
	}
	f.assertStatsMatchTheDataset(t, "撤銷一個")
	td.revoke(t, w.storeID, second, refs...)
	if get, _, _, _ := f.postVisible(t, w.post.ID); !get {
		t.Fatal("兩個都撤銷，貼文回來")
	}
	if got := f.readable(t, w.other.ID, w.storeID); got.comments != 1 || got.forum != 1 {
		t.Fatalf("兩個都撤銷，內容回來：%+v", got)
	}
}

// 建立與撤銷各排入受影響內容的快取失效 job（貼文：貼文 tag；留言：所屬貼文的 tag；討論區留言：店面頁 tag）。
func TestTakedownAndRevokeEnqueueCacheInvalidation(t *testing.T) {
	t.Parallel()
	f := setup(t)
	td := f.takedowns(t)
	w := f.takedownWorld(t)
	refs := []seller.ContentRef{ref(seller.ContentPost, w.post.ID), ref(seller.ContentComment, w.comment.ID), ref(seller.ContentForumEntry, w.entry.ID)}
	has := func(tag string) bool {
		for _, tags := range f.jobs.revalidations() {
			if slices.Contains(tags, tag) {
				return true
			}
		}
		return false
	}
	storeTags := revalidate.StoreTags(f.store(t, w.storeID).StoreID, f.store(t, w.storeID).StoreCode)

	id := td.create(t, w.storeID, refs...)
	for _, tag := range []string{revalidate.PostTag(w.post.ID), revalidate.PostTag(w.other.ID), storeTags[0]} {
		if !has(tag) {
			t.Fatalf("建立沒有排入 %q 的失效：%v", tag, f.jobs.revalidations())
		}
	}
	td.revoke(t, w.storeID, id, refs...)
	for _, tag := range []string{revalidate.PostTag(w.post.ID), revalidate.PostTag(w.other.ID), storeTags[0]} {
		if !has(tag) {
			t.Fatalf("撤銷沒有排入 %q 的失效：%v", tag, f.jobs.revalidations())
		}
	}
}

// 呼叫方回滾：複本跟著回滾（措施列與複本在同一個 transaction）。
func TestTakedownCopyRollsBackWithTheCallersTransaction(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	td := f.takedowns(t)
	w := f.takedownWorld(t)
	refs := []seller.ContentRef{ref(seller.ContentPost, w.post.ID), ref(seller.ContentComment, w.comment.ID)}
	errRollback := context.Canceled
	err := f.db.InTx(ctx, func(ctx context.Context) error {
		id := uuid.NewV7()
		if _, e := f.db.Conn(ctx).Exec(ctx, `INSERT INTO seller.measures (id, account_id, store_id, kind, basis, statement_to_seller, starts_at, status, current_revision, created_by_admin_id, created_at, updated_at)
			SELECT $1, account_id, store_id, 'CONTENT_TAKEDOWN', 'VIOLATION', '下架', $3, 'ENFORCED', 1, 9, $3, $3 FROM seller.stores WHERE store_id = $2`, id, w.storeID, f.clock()); e != nil {
			return e
		}
		if e := events.Publish(ctx, f.bus, seller.MeasureCreated{MeasureID: id, StoreID: w.storeID, OccurredAt: f.clock(), Contents: refs}); e != nil {
			return e
		}
		return errRollback
	})
	if !errors.Is(err, errRollback) {
		t.Fatal(err)
	}
	if n := f.scalar(t, "SELECT count(*) FROM social.content_takedowns"); n != 0 {
		t.Fatalf("回滾後複本還在：%d", n)
	}
	if n := f.scalar(t, "SELECT count(*) FROM seller.measures"); n != 0 {
		t.Fatalf("回滾後措施還在：%d", n)
	}
	if get, feed, _, _ := f.postVisible(t, w.post.ID); !get || !feed {
		t.Fatal("回滾後貼文照常可讀")
	}
	_ = td
}

// 其他種類的措施事件（Contents 為空）不碰複本。
func TestNonTakedownMeasureEventsAreIgnored(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	f.takedowns(t)
	must(t, f.db.InTx(ctx, func(ctx context.Context) error {
		if err := events.Publish(ctx, f.bus, seller.MeasureCreated{MeasureID: uuid.NewV7(), OccurredAt: f.clock()}); err != nil {
			return err
		}
		return events.Publish(ctx, f.bus, seller.MeasureRevoked{MeasureID: uuid.NewV7(), OccurredAt: f.clock()})
	}))
}

// 每日對帳（缺）：seller 有生效中的下架而複本沒有（訂閱者漏了）→ 補上、重新求值、回報漂移；補完再對帳沒有漂移。
func TestReconcileAddsTheTakedownsTheSubscriberMissed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	td := f.takedowns(t)
	w := f.takedownWorld(t)
	td.seed(t, w.storeID, []seller.ContentRef{ref(seller.ContentPost, w.post.ID), ref(seller.ContentComment, w.comment.ID), ref(seller.ContentForumEntry, w.entry.ID)})
	if get, _, _, _ := f.postVisible(t, w.post.ID); !get {
		t.Fatal("前提：沒有事件，社群不知道")
	}
	rep, err := f.module.Reconcile(ctx)
	must(t, err)
	if rep.TakedownsAdded != 3 || rep.TakedownsRemoved != 0 || rep.Drifted < 3 {
		t.Fatalf("對帳要補上 3 列並回報漂移：%+v", rep)
	}
	if get, feed, _, _ := f.postVisible(t, w.post.ID); get || feed {
		t.Fatal("補上之後貼文隱藏")
	}
	if got := f.readable(t, w.other.ID, w.storeID); got.comments != 0 || got.forum != 0 {
		t.Fatalf("補上之後留言與討論區留言隱藏：%+v", got)
	}
	f.assertStatsMatchTheDataset(t, "對帳補複本")
	rep, err = f.module.Reconcile(ctx)
	must(t, err)
	if rep.TakedownsAdded != 0 || rep.TakedownsRemoved != 0 || rep.Drifted != 0 {
		t.Fatalf("補好之後再對帳不該有漂移：%+v", rep)
	}
}

// 每日對帳（多）：複本有而 seller 的措施已撤銷（撤銷的訂閱者漏了）→ 刪掉、重新求值、回報漂移。
func TestReconcileRemovesTheTakedownsTheSubscriberMissedRevoking(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	td := f.takedowns(t)
	w := f.takedownWorld(t)
	refs := []seller.ContentRef{ref(seller.ContentPost, w.post.ID), ref(seller.ContentComment, w.comment.ID), ref(seller.ContentForumEntry, w.entry.ID)}
	id := td.create(t, w.storeID, refs...)
	f.exec(t, "UPDATE seller.measures SET status = 'REVOKED' WHERE id = $1", id)
	if get, _, _, _ := f.postVisible(t, w.post.ID); get {
		t.Fatal("前提：沒有撤銷事件，複本還在")
	}
	rep, err := f.module.Reconcile(ctx)
	must(t, err)
	if rep.TakedownsRemoved != 3 || rep.TakedownsAdded != 0 || rep.Drifted < 3 {
		t.Fatalf("對帳要刪除 3 列並回報漂移：%+v", rep)
	}
	if get, feed, _, _ := f.postVisible(t, w.post.ID); !get || !feed {
		t.Fatal("刪除之後貼文回來")
	}
	if got := f.readable(t, w.other.ID, w.storeID); got.comments != 1 || got.forum != 1 {
		t.Fatalf("刪除之後留言與討論區留言回來：%+v", got)
	}
	f.assertStatsMatchTheDataset(t, "對帳刪複本")
	rep, err = f.module.Reconcile(ctx)
	must(t, err)
	if rep.Drifted != 0 {
		t.Fatalf("再對帳不該有漂移：%+v", rep)
	}
}

// 內容不存在：複本的外鍵讓訂閱者失敗，措施跟著回滾。
func TestTakedownOfMissingContentFailsTheCallersTransaction(t *testing.T) {
	t.Parallel()
	f := setup(t)
	td := f.takedowns(t)
	w := f.takedownWorld(t)
	err := f.db.InTx(context.Background(), func(ctx context.Context) error {
		return events.Publish(ctx, f.bus, seller.MeasureCreated{MeasureID: uuid.NewV7(), StoreID: w.storeID, OccurredAt: f.clock(), Contents: []seller.ContentRef{ref(seller.ContentPost, uuid.NewV7())}})
	})
	if err == nil {
		t.Fatal("不存在的內容要讓建立失敗")
	}
	_ = td
}

// 不認得的內容種類：訂閱者回錯誤，措施跟著回滾，不是靜默略過。
func TestTakedownOfUnknownContentKindFailsTheCallersTransaction(t *testing.T) {
	t.Parallel()
	f := setup(t)
	f.takedowns(t)
	w := f.takedownWorld(t)
	err := f.db.InTx(context.Background(), func(ctx context.Context) error {
		return events.Publish(ctx, f.bus, seller.MeasureCreated{MeasureID: uuid.NewV7(), StoreID: w.storeID, OccurredAt: f.clock(), Contents: []seller.ContentRef{ref("BOGUS", w.post.ID)}})
	})
	if err == nil {
		t.Fatal("不認得的內容種類要讓建立失敗")
	}
	if n := f.scalar(t, "SELECT count(*) FROM social.content_takedowns"); n != 0 {
		t.Fatalf("回滾後複本還在：%d", n)
	}
}

// 歸屬核對在訂閱者：買家寫的留言與討論區留言、別家店的內容、已刪除與不存在的內容都讓建立失敗，回 *TakedownContentError 指出是哪一則，措施回滾、旗標不動。
func TestTakedownRejectsContentThatIsNotTheMeasureStoresOwn(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	f.takedowns(t)
	w := f.takedownWorld(t)
	otherStore, _ := f.exposableStore(t)
	foreign := f.publishedPost(t, otherStore, "別家店的貼文", "organic")
	buyer := f.asBuyer(f.newBuyer(t))
	buyerComment := mustV[social.Comment](t)(f.module.CreateComment(ctx, buyer, social.CommentInput{PostID: w.post.ID, Body: "買家在店家貼文下的留言"}))
	buyerEntry := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, buyer, social.ForumInput{StoreID: w.storeID, Body: "買家的討論"}))
	deletedPost := f.publishedPost(t, w.storeID, "已刪除的貼文", "organic")
	must(t, f.module.DeletePost(ctx, f.store(t, w.storeID), deletedPost.ID, deletedPost.Version))
	deletedComment := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asStore(t, w.storeID), social.CommentInput{PostID: w.post.ID, Body: "待刪的留言"}))
	must(t, f.module.DeleteComment(ctx, f.asStore(t, w.storeID), deletedComment.ID))
	for name, tc := range map[string]seller.ContentRef{
		"別家店的貼文":    ref(seller.ContentPost, foreign.ID),
		"買家的留言":     ref(seller.ContentComment, buyerComment.ID),
		"買家的討論區留言":  ref(seller.ContentForumEntry, buyerEntry.ID),
		"已刪除的貼文":    ref(seller.ContentPost, deletedPost.ID),
		"已刪除的留言":    ref(seller.ContentComment, deletedComment.ID),
		"不存在的討論區留言": ref(seller.ContentForumEntry, uuid.NewV7()),
	} {
		err := f.db.InTx(ctx, func(ctx context.Context) error {
			return events.Publish(ctx, f.bus, seller.MeasureCreated{
				MeasureID: uuid.NewV7(), StoreID: w.storeID, OccurredAt: f.clock(),
				Contents: []seller.ContentRef{ref(seller.ContentPost, w.post.ID), tc},
			})
		})
		var bad *social.TakedownContentError
		if !errors.As(err, &bad) || bad.Kind != tc.Kind || bad.ID != tc.ID {
			t.Fatalf("%s：err=%v，預期 *TakedownContentError 指向 %v", name, err, tc)
		}
	}
	if n := f.scalar(t, "SELECT count(*) FROM social.content_takedowns"); n != 0 {
		t.Fatalf("被拒絕後不該有複本：%d", n)
	}
	if get, feed, _, _ := f.postVisible(t, w.post.ID); !get || !feed {
		t.Fatal("被拒絕後貼文照常可讀")
	}
}

// 對帳補漏在寫入的語句裡再看一次 seller.pub_content_takedowns()：先讀到「缺」的清單、寫入之前措施被撤銷時，不把已撤銷措施的複本寫回去。
// （撤銷的 transaction 與對帳的讀取交錯無法在不加 hook 的情況下穩定重現；這裡直接把過期的清單交給補漏入口。）
func TestReconcileInsertRechecksThatTheMeasureIsStillActive(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	td := f.takedowns(t)
	w := f.takedownWorld(t)
	refs := []seller.ContentRef{ref(seller.ContentPost, w.post.ID), ref(seller.ContentComment, w.comment.ID)}
	id := td.seed(t, w.storeID, refs)
	f.exec(t, "UPDATE seller.measures SET status = 'REVOKED' WHERE id = $1", id)
	svc := stats.New(stats.Deps{DB: f.db, Logger: slog.New(slog.DiscardHandler), Now: f.clock})
	stale := []stats.Content{{Kind: contentkind.Post, ID: w.post.ID}, {Kind: contentkind.Comment, ID: w.comment.ID}}
	must(t, svc.ApplyMissingTakedowns(ctx, id, f.clock(), stale))
	if n := f.scalar(t, "SELECT count(*) FROM social.content_takedowns"); n != 0 {
		t.Fatalf("已撤銷的措施不該被補回複本：%d", n)
	}
	if get, feed, _, _ := f.postVisible(t, w.post.ID); !get || !feed {
		t.Fatal("貼文照常可讀")
	}
}
