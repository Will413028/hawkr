//go:build integration

package social_test

import (
	"context"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// SD27：對外的刪除沒刪到回 NotFound；連刪兩次第二次 NotFound、計數只扣一次、不做副作用。
func TestSD27DeleteOwnPostReportsNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	b := f.newBuyer(t)
	p := f.publishedPost(t, storeID, "貼文")
	cm := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: "留言"}))
	st := f.store(t, storeID)
	live := func() int64 {
		return f.scalar(t, `SELECT count(*) FROM social.posts WHERE store_id = $1 AND deleted_at IS NULL`, storeID)
	}
	versionOf := func() int64 { return f.scalar(t, `SELECT version FROM social.posts WHERE id = $1`, p.ID) }
	if live() != 1 {
		t.Fatalf("刪除前店內貼文數：%d", live())
	}
	v0 := versionOf()
	f.jobs.reset()
	must(t, f.module.DeleteOwnPost(ctx, st, p.ID, 0))
	if live() != 0 || versionOf() != v0+1 || len(f.jobs.revalidations()) != 1 {
		t.Fatalf("第一次刪除：店內貼文數 %d、版本 %d→%d、revalidation %d", live(), v0, versionOf(), len(f.jobs.revalidations()))
	}
	f.jobs.reset()
	if err := f.module.DeleteOwnPost(ctx, st, p.ID, 0); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("第二次刪除：%v", err)
	}
	if live() != 0 || versionOf() != v0+1 {
		t.Fatalf("第二次刪除不能再改計數與版本：店內貼文數 %d、版本 %d", live(), versionOf())
	}
	if len(f.jobs.revalidations()) != 0 {
		t.Fatal("沒刪到不做副作用")
	}
	if n := f.scalar(t, `SELECT count(*) FROM social.comments WHERE id = $1 AND deleted_at IS NULL`, cm.ID); n != 1 {
		t.Fatalf("軟刪貼文不動留言：%d", n)
	}
	if err := f.module.DeleteOwnPost(ctx, st, f.draft(t, storeID).ID, 0); err != nil {
		t.Fatalf("草稿可刪：%v", err)
	}
	if err := f.module.DeleteOwnPost(ctx, st, p.ID, 0); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("已刪除：%v", err)
	}
}

// SD26：沒有店的帳號回 NotFound；別家未公開（草稿、下架、已刪除）的貼文 NotFound；別家已發布的貼文，Update、Publish、
// Unpublish、Delete、Schedule、CancelSchedule 都回 PermissionDenied，且不改動貼文。
func TestSD26ForeignWriteOnPublishedIsDenied(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	owner, _ := f.exposableStore(t)
	other, _ := f.exposableStore(t)
	o := f.store(t, other)
	body := "別人改的"

	pub := f.publishedPost(t, owner, "已發布")
	draft := f.draft(t, owner)
	unpub := f.publishedPost(t, owner, "下架")
	unpub = mustV[social.Post](t)(f.module.UnpublishPost(ctx, f.store(t, owner), unpub.ID, unpub.Version, nil))
	gone := f.publishedPost(t, owner, "已刪除")
	must(t, f.module.DeleteOwnPost(ctx, f.store(t, owner), gone.ID, 0))

	calls := map[string]func(p social.Post) error{
		"Update": func(p social.Post) error {
			_, err := f.module.UpdatePost(ctx, o, social.UpdatePostInput{PostID: p.ID, Version: p.Version, Body: &body}, nil)
			return err
		},
		"Publish": func(p social.Post) error {
			_, err := f.module.PublishPost(ctx, o, p.ID, p.Version, nil)
			return err
		},
		"Unpublish": func(p social.Post) error {
			_, err := f.module.UnpublishPost(ctx, o, p.ID, p.Version, nil)
			return err
		},
		"Delete": func(p social.Post) error { return f.module.DeleteOwnPost(ctx, o, p.ID, 0) },
		"Schedule": func(p social.Post) error {
			_, err := f.module.SchedulePost(ctx, o, social.SchedulePostInput{PostID: p.ID, Version: p.Version, PublishAt: f.clock().Add(time.Hour), By: f.owner()}, nil)
			return err
		},
		"CancelSchedule": func(p social.Post) error {
			_, err := f.module.CancelPostSchedule(ctx, o, p.ID, p.Version)
			return err
		},
	}
	for name, call := range calls {
		if err := call(pub); kindOf(err) != apperr.KindPermissionDenied {
			t.Errorf("%s 別家已發布：%v", name, err)
		}
		for label, p := range map[string]social.Post{"草稿": draft, "下架": unpub, "已刪除": gone} {
			if name == "DeleteOld" && label == "已刪除" {
				// 模組的 DeletePost 冪等（SD5）：已刪除的貼文什麼都不做、回 nil。
				if err := call(p); err != nil {
					t.Errorf("%s 別家%s：%v", name, label, err)
				}
				continue
			}
			if err := call(p); kindOf(err) != apperr.KindNotFound {
				t.Errorf("%s 別家%s：%v", name, label, err)
			}
		}
	}
	got := mustV[social.Post](t)(f.module.OwnPost(ctx, owner, pub.ID))
	if got.Version != pub.Version || got.Body != pub.Body || got.Status != social.StatusPublished {
		t.Fatalf("被拒絕的寫入不該留下痕跡：%+v", got)
	}
}
