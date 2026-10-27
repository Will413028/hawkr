//go:build integration

package social_test

import (
	"context"
	"slices"
	"testing"

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
	if w.Kind != social.SanctionWarning || w.EndsAt != nil || w.Category != "HARASSMENT" || w.SourceRef != "case-77" {
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
	must(t, f.module.DeleteComment(ctx, f.asBuyer(b), c.ID))
	if got := owner(social.TargetComment, c.ID); got.Exists {
		t.Fatalf("刪除後不存在 %+v", got)
	}
	if got := owner(social.TargetComment, uuidNew()); got.Exists {
		t.Fatalf("不存在的 ID %+v", got)
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
	draft := mustV[social.Post](t)(f.module.CreatePost(ctx, st, social.CreatePostInput{Body: "草稿", ImageToken: "t"}))
	if len(f.jobs.revalidations()) != 0 {
		t.Fatal("草稿不失效")
	}
	f.jobs.reset()
	p := mustV[social.Post](t)(f.module.PublishPost(ctx, st, draft.ID, draft.Version))
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
	up := mustV[social.Post](t)(f.module.UpdatePost(ctx, st, social.UpdatePostInput{PostID: p.ID, Version: p.Version, Body: "改"}))
	mustV[social.Post](t)(f.module.UnpublishPost(ctx, st, p.ID, up.Version))
	must(t, f.module.DeletePost(ctx, st, p.ID, 0))
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
