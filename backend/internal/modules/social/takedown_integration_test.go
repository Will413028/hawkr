//go:build integration

package social_test

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/readcache"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/schedule"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/stats"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/contentkind"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/Will413028/hawkr/backend/internal/platform/revalidate"
	"github.com/Will413028/hawkr/backend/internal/testkit/sellerfixture"
)

type takedowns struct {
	f *fixture
}

func (f *fixture) takedowns(t *testing.T) takedowns {
	t.Helper()
	events.Subscribe(f.bus, f.module.OnMeasureCreated)
	events.Subscribe(f.bus, f.module.OnMeasureRevoked)
	return takedowns{f: f}
}

func (d takedowns) seed(t *testing.T, storeID int64, refs []seller.ContentRef) uuid.UUID {
	t.Helper()
	module, err := sellerfixture.NewModuleWithEvents(d.f.db, events.NewBus(), d.f.clock)
	must(t, err)
	created, err := module.CreateMeasure(context.Background(), d.input(t, storeID, refs))
	must(t, err)
	return created.Measure.ID
}

func (d takedowns) create(t *testing.T, storeID int64, refs ...seller.ContentRef) uuid.UUID {
	t.Helper()
	created, err := d.f.sellerMod.CreateMeasure(context.Background(), d.input(t, storeID, refs))
	must(t, err)
	d.f.jobs.reset()
	return created.Measure.ID
}

func (d takedowns) revoke(t *testing.T, storeID int64, id uuid.UUID, refs ...seller.ContentRef) {
	t.Helper()
	current, err := d.f.sellerMod.CurrentMeasure(context.Background(), id)
	must(t, err)
	_, err = d.f.sellerMod.RevokeMeasure(context.Background(), seller.RevokeMeasureInput{
		MeasureID: id, ExpectedVersion: current.Version, AdminID: 9, Reason: seller.RevokeAdminCorrection,
	})
	must(t, err)
	d.f.jobs.reset()
}

func (d takedowns) revokeWithoutEvent(t *testing.T, id uuid.UUID) {
	t.Helper()
	module, err := sellerfixture.NewModuleWithEvents(d.f.db, events.NewBus(), d.f.clock)
	must(t, err)
	current, err := module.CurrentMeasure(context.Background(), id)
	must(t, err)
	_, err = module.RevokeMeasure(context.Background(), seller.RevokeMeasureInput{
		MeasureID: id, ExpectedVersion: current.Version, AdminID: 9, Reason: seller.RevokeAdminCorrection,
	})
	must(t, err)
}

func (d takedowns) input(t *testing.T, storeID int64, refs []seller.ContentRef) seller.CreateMeasureInput {
	t.Helper()
	contents := make([]seller.TakedownContent, len(refs))
	for i, r := range refs {
		content, err := d.f.module.ContentSnapshot(context.Background(), social.Target{Kind: social.TargetKind(r.Kind), ID: r.ID})
		must(t, err)
		if !content.Exists {
			t.Fatalf("內容 %s %s 不存在", r.Kind, r.ID)
		}
		snapshot := seller.NewContentSnapshot(r.Kind, content.Body, content.AuthoredAt, len(content.MediaRefs), content.StoreID, content.PostID, content.ThreadID)
		contents[i] = seller.TakedownContent{ContentRef: r, Snapshot: &snapshot}
	}
	return seller.CreateMeasureInput{
		AdminID: 9, StoreID: storeID, Kind: seller.MeasureContentTakedown, StatementToSeller: "下架", Note: "測試處置",
		Violation: &seller.ViolationBasis{Finding: &seller.NewFinding{Category: policy.Spam, Severity: seller.SeverityLow}},
		Takedown:  &seller.ContentTakedown{Contents: contents},
	}
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
		if _, err := f.sellerMod.CreateMeasureInTx(ctx, td.input(t, w.storeID, refs)); err != nil {
			return err
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
	td.revokeWithoutEvent(t, id)
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

// 列已清除或不存在的內容：成立且不寫複本（歸屬由 moderation 以案件快照核對），措施照常建立，同一批的存在內容照常寫複本。
func TestTakedownOfPurgedContentSucceedsWithoutReplica(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	f.takedowns(t)
	w := f.takedownWorld(t)
	purged := f.publishedPost(t, w.storeID, "已清除的貼文", "organic")
	must(t, f.module.DeleteOwnPost(ctx, f.store(t, w.storeID), purged.ID, purged.Version))
	f.exec(t, `WITH c AS (SELECT id FROM social.comments WHERE post_id = $1),
a AS (DELETE FROM social.comment_likes WHERE comment_id IN (SELECT id FROM c)),
b AS (DELETE FROM social.comment_stats WHERE comment_id IN (SELECT id FROM c)),
d AS (DELETE FROM social.hides WHERE comment_id IN (SELECT id FROM c)),
e AS (DELETE FROM social.content_takedowns WHERE post_id = $1 OR comment_id IN (SELECT id FROM c)),
g AS (DELETE FROM social.post_hashtags WHERE post_id = $1),
h AS (DELETE FROM social.post_likes WHERE post_id = $1),
i AS (DELETE FROM social.post_stats WHERE post_id = $1),
j AS (DELETE FROM social.post_views WHERE post_id = $1),
k AS (DELETE FROM social.post_shares WHERE post_id = $1),
l AS (DELETE FROM social.comments WHERE post_id = $1)
DELETE FROM social.posts WHERE id = $1`, purged.ID)
	must(t, f.db.InTx(ctx, func(ctx context.Context) error {
		return events.Publish(ctx, f.bus, seller.MeasureCreated{
			MeasureID: uuid.NewV7(), StoreID: w.storeID, OccurredAt: f.clock(),
			Contents: []seller.ContentRef{ref(seller.ContentPost, purged.ID), ref(seller.ContentComment, uuid.NewV7()), ref(seller.ContentForumEntry, uuid.NewV7()), ref(seller.ContentPost, w.post.ID)},
		})
	}))
	if n := f.scalar(t, "SELECT count(*) FROM social.content_takedowns"); n != 1 {
		t.Fatalf("只有存在的貼文寫複本：%d", n)
	}
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

// 歸屬核對在訂閱者：買家寫的留言與討論區留言、別家店的內容（含別家店已刪除的）都讓建立失敗，回 *TakedownContentError 指出是哪一則，措施回滾、旗標不動。
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
	foreignDeleted := f.publishedPost(t, otherStore, "別家店已刪除的貼文", "organic")
	must(t, f.module.DeleteOwnPost(ctx, f.store(t, otherStore), foreignDeleted.ID, foreignDeleted.Version))
	for name, tc := range map[string]seller.ContentRef{
		"別家店的貼文":    ref(seller.ContentPost, foreign.ID),
		"買家的留言":     ref(seller.ContentComment, buyerComment.ID),
		"買家的討論區留言":  ref(seller.ContentForumEntry, buyerEntry.ID),
		"別家店已刪除的貼文": ref(seller.ContentPost, foreignDeleted.ID),
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

// 內容在被檢舉之後被作者自己刪除：措施仍成立（計入升級），歸屬在措施自己的快照核對；訂閱者對已軟刪的內容不寫複本（沒有公開可見的東西要隱藏），其餘內容照常寫。
func TestTakedownOfDeletedOwnContentSucceedsWithoutReplica(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	f.takedowns(t)
	w := f.takedownWorld(t)
	deletedPost := f.publishedPost(t, w.storeID, "已刪除的貼文", "organic")
	must(t, f.module.DeleteOwnPost(ctx, f.store(t, w.storeID), deletedPost.ID, deletedPost.Version))
	deletedComment := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asStore(t, w.storeID), social.CommentInput{PostID: w.post.ID, Body: "待刪的留言"}))
	must(t, f.module.DeleteComment(ctx, f.asStore(t, w.storeID), deletedComment.ID, 0))
	must(t, f.db.InTx(ctx, func(ctx context.Context) error {
		return events.Publish(ctx, f.bus, seller.MeasureCreated{
			MeasureID: uuid.NewV7(), StoreID: w.storeID, OccurredAt: f.clock(),
			Contents: []seller.ContentRef{ref(seller.ContentPost, deletedPost.ID), ref(seller.ContentComment, deletedComment.ID)},
		})
	}))
	if n := f.scalar(t, "SELECT count(*) FROM social.content_takedowns"); n != 0 {
		t.Fatalf("已刪除的內容不該有複本：%d", n)
	}
	must(t, f.db.InTx(ctx, func(ctx context.Context) error {
		return events.Publish(ctx, f.bus, seller.MeasureCreated{
			MeasureID: uuid.NewV7(), StoreID: w.storeID, OccurredAt: f.clock(),
			Contents: []seller.ContentRef{ref(seller.ContentPost, deletedPost.ID), ref(seller.ContentPost, w.post.ID)},
		})
	}))
	if n := f.scalar(t, "SELECT count(*) FROM social.content_takedowns"); n != 1 {
		t.Fatalf("只有未刪除的貼文寫複本：%d", n)
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
	td.revokeWithoutEvent(t, id)
	svc := stats.New(stats.Deps{DB: f.db, Logger: slog.New(slog.DiscardHandler), Now: f.clock, Cache: readcache.Cache{Jobs: f.jobs, Stores: f.sellers}, Cancel: schedule.Canceler{DB: f.db, Events: f.bus}})
	stale := []stats.Content{{Kind: contentkind.Post, ID: w.post.ID}, {Kind: contentkind.Comment, ID: w.comment.ID}}
	must(t, svc.ApplyMissingTakedowns(ctx, id, f.clock(), stale))
	if n := f.scalar(t, "SELECT count(*) FROM social.content_takedowns"); n != 0 {
		t.Fatalf("已撤銷的措施不該被補回複本：%d", n)
	}
	if get, feed, _, _ := f.postVisible(t, w.post.ID); !get || !feed {
		t.Fatal("貼文照常可讀")
	}
}

// 對帳補漏只取消實際寫入複本的貼文的排程：措施在讀到「缺」的清單之後被撤銷、語句沒有寫入任何列時，貼文沒有被下架，它的排程原樣保留。
func TestReconcileInsertCancelsOnlyTheSchedulesOfInsertedTakedowns(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	td := f.takedowns(t)
	storeID, _ := f.exposableStore(t)
	p := f.draft(t, storeID)
	s := f.schedule(t, storeID, p, time.Hour, f.owner())
	id := td.seed(t, storeID, []seller.ContentRef{ref(seller.ContentPost, p.ID)})
	td.revokeWithoutEvent(t, id)
	svc := stats.New(stats.Deps{DB: f.db, Logger: slog.New(slog.DiscardHandler), Now: f.clock, Cache: readcache.Cache{Jobs: f.jobs, Stores: f.sellers}, Cancel: schedule.Canceler{DB: f.db, Events: f.bus}})
	must(t, svc.ApplyMissingTakedowns(ctx, id, f.clock(), []stats.Content{{Kind: contentkind.Post, ID: p.ID}}))
	f.wantSchedule(t, s.ID, "PENDING", "")
	if n := len(f.events.canceledEvents()); n != 0 {
		t.Fatalf("沒有寫入複本就沒有取消：%d 個事件", n)
	}

	live := td.seed(t, storeID, []seller.ContentRef{ref(seller.ContentPost, p.ID)})
	must(t, svc.ApplyMissingTakedowns(ctx, live, f.clock(), []stats.Content{{Kind: contentkind.Post, ID: p.ID}}))
	f.wantSchedule(t, s.ID, "CANCELED", "TAKEN_DOWN")
}

// 每則內容至多一列複本（schema 鏡射 seller 的 measure_contents_one_enforced）：同一篇貼文被另一個措施再寫一列時唯一索引擋下，而不是靠讀取端挑一列。
func TestSecondReplicaRowForTheSameContentViolatesTheUniqueIndex(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.takedownWorld(t)
	f.exec(t, "INSERT INTO social.content_takedowns (measure_id, post_id) VALUES ($1, $2)", uuid.NewV7(), w.post.ID)
	_, err := f.db.Conn(context.Background()).Exec(context.Background(), "INSERT INTO social.content_takedowns (measure_id, post_id) VALUES ($1, $2)", uuid.NewV7(), w.post.ID)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" || pgErr.ConstraintName != "content_takedowns_post" {
		t.Fatalf("同一篇貼文的第二列該違反 content_takedowns_post：%v", err)
	}
	f.exec(t, "INSERT INTO social.content_takedowns (measure_id, comment_id) VALUES ($1, $2)", uuid.NewV7(), w.comment.ID)
	if _, err := f.db.Conn(context.Background()).Exec(context.Background(), "INSERT INTO social.content_takedowns (measure_id, comment_id) VALUES ($1, $2)", uuid.NewV7(), w.comment.ID); !errors.As(err, &pgErr) || pgErr.ConstraintName != "content_takedowns_comment" {
		t.Fatalf("同一則留言的第二列該違反 content_takedowns_comment：%v", err)
	}
	f.exec(t, "INSERT INTO social.content_takedowns (measure_id, forum_entry_id) VALUES ($1, $2)", uuid.NewV7(), w.entry.ID)
	if _, err := f.db.Conn(context.Background()).Exec(context.Background(), "INSERT INTO social.content_takedowns (measure_id, forum_entry_id) VALUES ($1, $2)", uuid.NewV7(), w.entry.ID); !errors.As(err, &pgErr) || pgErr.ConstraintName != "content_takedowns_forum_entry" {
		t.Fatalf("同一則討論區留言的第二列該違反 content_takedowns_forum_entry：%v", err)
	}
}

// 對帳補漏與撤銷的競爭：撤銷的 transaction 已經把措施改成 REVOKED 但尚未提交時，補漏等它（seller.pub_lock_content_takedowns 的 FOR SHARE），撤銷提交後補漏讀不到措施，不寫回複本。
// 補漏是否卡在鎖上由 pg_stat_activity 的 wait_event_type 判斷，不靠時間。
func TestReconcileInsertWaitsForAnUncommittedRevokeAndWritesNothing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	td := f.takedowns(t)
	w := f.takedownWorld(t)
	id := td.seed(t, w.storeID, []seller.ContentRef{ref(seller.ContentPost, w.post.ID)})
	svc := stats.New(stats.Deps{DB: f.db, Logger: slog.New(slog.DiscardHandler), Now: f.clock, Cache: readcache.Cache{Jobs: f.jobs, Stores: f.sellers}, Cancel: schedule.Canceler{DB: f.db, Events: f.bus}})
	revoker, err := sellerfixture.NewModuleWithEvents(f.db, events.NewBus(), f.clock)
	must(t, err)
	current, err := revoker.CurrentMeasure(ctx, id)
	must(t, err)

	revoking, release := make(chan struct{}), make(chan struct{})
	revokeDone := make(chan error, 1)
	go func() {
		revokeDone <- f.db.InTx(ctx, func(ctx context.Context) error {
			if _, err := revoker.RevokeMeasure(ctx, seller.RevokeMeasureInput{
				MeasureID: id, ExpectedVersion: current.Version, AdminID: 9, Reason: seller.RevokeAdminCorrection,
			}); err != nil {
				return err
			}
			close(revoking)
			<-release
			return nil
		})
	}()
	select {
	case <-revoking:
	case err := <-revokeDone:
		t.Fatalf("撤銷在持有變更之前就結束了：%v", err)
	}

	repairDone := make(chan error, 1)
	go func() {
		repairDone <- svc.ApplyMissingTakedowns(ctx, id, f.clock(), []stats.Content{{Kind: contentkind.Post, ID: w.post.ID}})
	}()
	waiting := func() bool {
		return f.scalar(t, `SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND pid <> pg_backend_pid() AND wait_event_type = 'Lock' AND query LIKE '%pub_lock_content_takedowns%'`) > 0
	}
	for !waiting() {
		select {
		case err := <-repairDone:
			t.Fatalf("補漏沒有等未提交的撤銷就結束了（err=%v）", err)
		default:
		}
		time.Sleep(5 * time.Millisecond)
	}

	close(release)
	must(t, <-revokeDone)
	must(t, <-repairDone)
	if n := f.scalar(t, "SELECT count(*) FROM social.content_takedowns"); n != 0 {
		t.Fatalf("撤銷提交後補漏不該寫回複本：%d 列", n)
	}
	if get, feed, _, _ := f.postVisible(t, w.post.ID); !get || !feed {
		t.Fatal("貼文照常可讀")
	}
}
