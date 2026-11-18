//go:build integration

package social_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/revalidate"
)

// SD11（紀錄面）：警告寫成 social 的處分紀錄，作者讀得到型別與理由，別人讀不到。
func TestSD11WarningIsARecordTheBuyerCanRead(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	b, other := f.newBuyer(t), f.newBuyer(t)
	w, err := f.module.Warn(ctx, social.WarnInput{BuyerID: b.BuyerID, Category: "HARASSMENT", Text: "請勿辱罵他人", SourceRef: "case-77", AdminID: 5})
	must(t, err)
	if w.Kind != social.SanctionWarning || w.Category != "HARASSMENT" || w.SourceRef != "case-77" {
		t.Fatalf("%+v", w)
	}
	mustV[social.Sanction](t)(f.module.Warn(ctx, social.WarnInput{BuyerID: b.BuyerID, Category: "SPAM", AdminID: 5}))
	page := mustV[social.SanctionPage](t)(f.module.MySanctions(ctx, b.BuyerID, nil, 1))
	if len(page.Sanctions) != 1 || page.Next == nil || page.Sanctions[0].Category != "SPAM" {
		t.Fatalf("新的在前、keyset：%+v", page)
	}
	page = mustV[social.SanctionPage](t)(f.module.MySanctions(ctx, b.BuyerID, page.Next, 10))
	if len(page.Sanctions) != 1 || page.Sanctions[0].ID != w.ID || page.Sanctions[0].Text != "請勿辱罵他人" {
		t.Fatalf("%+v", page)
	}
	if got := mustV[social.SanctionPage](t)(f.module.MySanctions(ctx, other.BuyerID, nil, 10)); len(got.Sanctions) != 0 {
		t.Fatalf("別人讀不到：%+v", got)
	}
	for _, bad := range []social.WarnInput{{Category: "SPAM", AdminID: 1}, {BuyerID: b.BuyerID, Category: "OTHER", AdminID: 1}, {BuyerID: b.BuyerID, Category: "FOOD_SAFETY", AdminID: 1}, {BuyerID: b.BuyerID, Category: "SPAM"}} {
		if _, err := f.module.Warn(ctx, bad); kindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("%+v：%v", bad, err)
		}
	}
}

// ContentOwner 給 moderation 驗證內容歸屬：存在、作者 (kind, id)、所屬店家、是否公開可讀。
func TestContentOwnerReportsAuthorStoreAndReadability(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	b := f.newBuyer(t)
	p := f.publishedPost(t, storeID, "貼文")
	c := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: "留言"}))
	e := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(b), social.ForumInput{StoreID: storeID, Body: "討論"}))
	owner := func(k social.TargetKind, id uuidT) social.ContentOwner {
		return mustV[social.ContentOwner](t)(f.module.ContentOwner(ctx, social.Target{Kind: k, ID: id}))
	}
	want := social.ContentOwner{Exists: true, Author: social.Actor{Kind: social.ActorBuyer, ID: b.BuyerID}, StoreID: storeID, Readable: true}
	if got := owner(social.TargetComment, c.ID); got != want {
		t.Fatalf("留言 %+v", got)
	}
	if got := owner(social.TargetForumEntry, e.ID); got != want {
		t.Fatalf("討論區 %+v", got)
	}
	if got := owner(social.TargetPost, p.ID); !got.Exists || !got.Readable || got.Author != (social.Actor{Kind: social.ActorStore, ID: storeID}) || got.StoreID != storeID {
		t.Fatalf("貼文 %+v", got)
	}
	_, err := f.module.Hide(ctx, social.HideInput{Target: social.Target{Kind: social.TargetComment, ID: c.ID}, Category: "SPAM", AdminID: 1})
	must(t, err)
	if got := owner(social.TargetComment, c.ID); !got.Exists || got.Readable {
		t.Fatalf("被隱藏：存在但不可讀 %+v", got)
	}
	must(t, f.module.DeleteComment(ctx, f.asBuyer(b), c.ID, 0))
	if got := owner(social.TargetComment, c.ID); got.Exists {
		t.Fatalf("刪除後不存在 %+v", got)
	}
	if got := owner(social.TargetComment, uuidNew()); got.Exists {
		t.Fatalf("不存在的 ID %+v", got)
	}
}

// ContentSnapshot 給 moderation 凍結內容：本文、媒體參照與作者；內容改過快照跟著變，刪除後不存在。
func TestContentSnapshotReturnsBodyMediaAndAuthor(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	b := f.newBuyer(t)
	p := f.publishedPost(t, storeID, "貼文本文")
	c := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: "留言本文"}))
	e := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(b), social.ForumInput{StoreID: storeID, Body: "討論本文"}))
	snap := func(k social.TargetKind, id uuidT) social.ContentSnapshot {
		return mustV[social.ContentSnapshot](t)(f.module.ContentSnapshot(ctx, social.Target{Kind: k, ID: id}))
	}
	buyer := social.Actor{Kind: social.ActorBuyer, ID: b.BuyerID}
	if got := snap(social.TargetComment, c.ID); !got.Exists || got.Body != "留言本文" || got.Author != buyer || got.StoreID != storeID || !got.Readable || len(got.MediaRefs) != 0 {
		t.Fatalf("留言 %+v", got)
	}
	if got := snap(social.TargetForumEntry, e.ID); !got.Exists || got.Body != "討論本文" || got.Author != buyer || got.StoreID != storeID {
		t.Fatalf("討論區 %+v", got)
	}
	got := snap(social.TargetPost, p.ID)
	if !got.Exists || got.Body != "貼文本文" || got.Author != (social.Actor{Kind: social.ActorStore, ID: storeID}) || !got.Readable || len(got.MediaRefs) != 1 || got.MediaRefs[0] == "" {
		t.Fatalf("貼文 %+v", got)
	}
	// 發文時間與連結目標：貼文連到自己，留言連到所屬貼文，討論區留言連到店家加討論串（頂層留言的討論串是自己）。
	postAuthored := f.timeOf(t, "SELECT COALESCE(published_at, created_at) FROM social.posts WHERE id = $1", p.ID)
	commentAuthored := f.timeOf(t, "SELECT created_at FROM social.comments WHERE id = $1", c.ID)
	entryAuthored := f.timeOf(t, "SELECT created_at FROM social.forum_entries WHERE id = $1", e.ID)
	if got := snap(social.TargetPost, p.ID); got.PostID != p.ID || got.ThreadID != (uuidT{}) || !got.AuthoredAt.Equal(postAuthored) || got.AuthoredAt.IsZero() {
		t.Fatalf("貼文的連結與發文時間 %+v", got)
	}
	if got := snap(social.TargetComment, c.ID); got.PostID != p.ID || got.ThreadID != (uuidT{}) || !got.AuthoredAt.Equal(commentAuthored) {
		t.Fatalf("留言的連結與發文時間 %+v", got)
	}
	if got := snap(social.TargetForumEntry, e.ID); got.ThreadID != e.ID || got.PostID != (uuidT{}) || got.StoreID != storeID || !got.AuthoredAt.Equal(entryAuthored) {
		t.Fatalf("討論區留言的連結與發文時間 %+v", got)
	}
	must(t, f.module.DeleteComment(ctx, f.asBuyer(b), c.ID, 0))
	if got := snap(social.TargetComment, c.ID); got.Exists || got.Body != "" {
		t.Fatalf("刪除後不存在 %+v", got)
	}
	if got := snap(social.TargetPost, uuidNew()); got.Exists {
		t.Fatalf("不存在的 ID %+v", got)
	}
	if _, err := f.module.ContentSnapshot(ctx, social.Target{Kind: "CHAT", ID: p.ID}); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("不認得的種類：%v", err)
	}
}

// 快取失效：發布、隱藏、還原各排入一個 revalidate job（貼文 tag 加店家 tag）；重複隱藏與重複刪除不做副作用；編輯草稿不失效。
func TestRevalidateIsEnqueuedInTheSameTransaction(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	st := f.store(t, storeID)
	b := f.newBuyer(t)
	tags := func(postID uuidT) []string {
		return append([]string{revalidate.PostTag(postID)}, revalidate.StoreTags(st.StoreID, st.StoreCode)...)
	}
	draft := mustV[social.Post](t)(f.module.CreatePost(ctx, st, social.CreatePostInput{Body: "草稿", ImageToken: "t"}, nil))
	if len(f.jobs.revalidations()) != 0 {
		t.Fatal("草稿不失效")
	}
	f.jobs.reset()
	p := mustV[social.Post](t)(f.module.PublishPost(ctx, st, draft.ID, draft.Version, nil))
	if got := f.jobs.revalidations(); len(got) != 1 || !slices.Equal(got[0], tags(p.ID)) {
		t.Fatalf("發布：%v", got)
	}
	c := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: "留言"}))
	f.jobs.reset()
	_, err := f.module.Hide(ctx, social.HideInput{Target: social.Target{Kind: social.TargetComment, ID: c.ID}, Category: "SPAM", AdminID: 1})
	must(t, err)
	if got := f.jobs.revalidations(); len(got) != 1 || !slices.Equal(got[0], tags(p.ID)) {
		t.Fatalf("隱藏：%v", got)
	}
	f.jobs.reset()
	_, err = f.module.Hide(ctx, social.HideInput{Target: social.Target{Kind: social.TargetComment, ID: c.ID}, Category: "SPAM", AdminID: 1})
	must(t, err)
	if len(f.jobs.revalidations()) != 0 {
		t.Fatal("重複隱藏不做副作用")
	}
	_, err = f.module.Restore(ctx, social.RestoreInput{Target: social.Target{Kind: social.TargetComment, ID: c.ID}, AdminID: 1, Reason: "誤判"})
	must(t, err)
	if got := f.jobs.revalidations(); len(got) != 1 {
		t.Fatalf("還原：%v", got)
	}
	f.jobs.reset()
	up := mustV[social.Post](t)(f.module.UpdatePost(ctx, st, social.UpdatePostInput{PostID: p.ID, Version: p.Version, Body: ptr("改")}, nil))
	mustV[social.Post](t)(f.module.UnpublishPost(ctx, st, p.ID, up.Version, nil))
	must(t, f.module.DeleteOwnPost(ctx, st, p.ID, 0))
	if got := f.jobs.revalidations(); len(got) != 3 {
		t.Fatalf("編輯已發布的貼文、下架、刪除各一個：%v", got)
	}
	// 討論區的隱藏與還原沿用店面頁的 tag。
	e := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(b), social.ForumInput{StoreID: storeID, Body: "討論"}))
	f.jobs.reset()
	_, err = f.module.Hide(ctx, social.HideInput{Target: social.Target{Kind: social.TargetForumEntry, ID: e.ID}, Category: "HATE", AdminID: 1})
	must(t, err)
	if got := f.jobs.revalidations(); len(got) != 1 || !slices.Equal(got[0], revalidate.StoreTags(st.StoreID, st.StoreCode)) {
		t.Fatalf("討論區隱藏：%v", got)
	}
}

func (f *fixture) timeOf(t *testing.T, sql string, args ...any) time.Time {
	t.Helper()
	var at time.Time
	if err := f.db.Conn(context.Background()).QueryRow(context.Background(), sql, args...).Scan(&at); err != nil {
		t.Fatalf("%v\n%s", err, sql)
	}
	return at
}

// AvailableLinks 回報連結目標此刻還在不在：貼文沒有軟刪、討論串的起始留言沒有軟刪；不存在的目標與別家店的討論串都是不在。一批目標每類一個查詢。
// mutation：AvailableLinkPosts 拿掉 deleted_at IS NULL → 刪除後的貼文仍回在；AvailableLinkThreads 的店家條件拿掉 → 錯店的討論串回在。
func TestAvailableLinksFollowDeletionOfPostsAndThreads(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	b := f.newBuyer(t)
	live := f.publishedPost(t, storeID, "還在")
	gone := f.publishedPost(t, storeID, "會刪")
	e1 := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(b), social.ForumInput{StoreID: storeID, Body: "串一"}))
	e2 := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(b), social.ForumInput{StoreID: storeID, Body: "串二"}))
	f.exec(t, "UPDATE social.posts SET deleted_at = now() WHERE id = $1", gone.ID)
	f.exec(t, "UPDATE social.forum_entries SET deleted_at = now() WHERE id = $1", e2.ID)
	threads := []social.ForumThread{{StoreID: storeID, ThreadID: e1.ID}, {StoreID: storeID, ThreadID: e2.ID}, {StoreID: storeID + 999, ThreadID: e1.ID}}
	got := mustV[social.LinkAvailability](t)(f.module.AvailableLinks(ctx, []uuidT{live.ID, gone.ID, uuidNew()}, threads))
	if !got.Posts[live.ID] || got.Posts[gone.ID] || len(got.Posts) != 1 {
		t.Errorf("貼文 %v", got.Posts)
	}
	if !got.Threads[threads[0]] || got.Threads[threads[1]] || got.Threads[threads[2]] || len(got.Threads) != 1 {
		t.Errorf("討論串 %v", got.Threads)
	}
	if empty := mustV[social.LinkAvailability](t)(f.module.AvailableLinks(ctx, nil, nil)); len(empty.Posts)+len(empty.Threads) != 0 {
		t.Errorf("沒有目標 %v", empty)
	}
}
