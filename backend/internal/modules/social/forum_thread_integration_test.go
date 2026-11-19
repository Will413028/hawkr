//go:build integration

package social_test

import (
	"context"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func TestGetForumThreadReturnsOnlyThatThread(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	a := f.newBuyer(t)
	enter := func(in social.ForumInput) social.ForumEntry {
		in.StoreID = storeID
		return mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(a), in))
	}
	t1 := enter(social.ForumInput{Body: "串一"})
	t2 := enter(social.ForumInput{Body: "串二"})
	r1 := enter(social.ForumInput{Body: "回串一", ParentID: &t1.ID})
	enter(social.ForumInput{Body: "回串二", ParentID: &t2.ID})

	got := mustV[social.ForumThreadView](t)(f.module.GetForumThread(ctx, f.asBuyer(a), storeID, t1.ID, 10))
	if got.Top.ID != t1.ID || got.Top.State != social.CommentVisible || got.Top.Body != "串一" {
		t.Fatalf("頂層 = %+v", got.Top)
	}
	if !sameIDs(forumIDs(got.Replies), r1.ID) {
		t.Fatalf("回覆只能是這一串的：%v", forumIDs(got.Replies))
	}
}

func TestGetForumThreadShowsAPlaceholderForADeletedTopWithVisibleReplies(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	a, b := f.newBuyer(t), f.newBuyer(t)
	top := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(a), social.ForumInput{StoreID: storeID, Body: "會刪除"}))
	reply := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(b), social.ForumInput{StoreID: storeID, Body: "回覆", ParentID: &top.ID}))
	must(t, f.module.DeleteForumEntry(ctx, f.asBuyer(a), top.ID, 0))

	got := mustV[social.ForumThreadView](t)(f.module.GetForumThread(ctx, social.Caller{}, storeID, top.ID, 10))
	if got.Top.ID != top.ID || got.Top.State != social.CommentDeleted || got.Top.Body != "" || got.Top.Actor != (social.Actor{}) {
		t.Fatalf("已刪除的頂層要是佔位項目：%+v", got.Top)
	}
	if !sameIDs(forumIDs(got.Replies), reply.ID) {
		t.Fatalf("回覆 = %v", forumIDs(got.Replies))
	}
}

func TestGetForumThreadIsNotFoundForOtherStoresMissingAndGoneThreads(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	otherStore, _ := f.exposableStore(t)
	a := f.newBuyer(t)
	top := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(a), social.ForumInput{StoreID: storeID, Body: "串"}))
	reply := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(a), social.ForumInput{StoreID: storeID, Body: "回", ParentID: &top.ID}))
	gone := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(a), social.ForumInput{StoreID: storeID, Body: "已刪除且沒有回覆"}))
	must(t, f.module.DeleteForumEntry(ctx, f.asBuyer(a), gone.ID, 0))

	for name, call := range map[string]func() error{
		"別家店": func() error {
			_, err := f.module.GetForumThread(ctx, social.Caller{}, otherStore, top.ID, 10)
			return err
		},
		"不存在": func() error {
			_, err := f.module.GetForumThread(ctx, social.Caller{}, storeID, uuid.New(), 10)
			return err
		},
		"回覆不是討論串": func() error {
			_, err := f.module.GetForumThread(ctx, social.Caller{}, storeID, reply.ID, 10)
			return err
		},
		"已刪除而沒有可見回覆": func() error {
			_, err := f.module.GetForumThread(ctx, social.Caller{}, storeID, gone.ID, 10)
			return err
		},
	} {
		if err := call(); apperr.KindOf(err) != apperr.KindNotFound {
			t.Errorf("%s：want NotFound，got %v", name, err)
		}
	}
}
