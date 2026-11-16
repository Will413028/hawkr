//go:build integration

package social_test

import (
	"context"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
)

func adminPostIDs(p social.AdminPostPage) []uuid.UUID {
	out := make([]uuid.UUID, len(p.Posts))
	for i, post := range p.Posts {
		out[i] = post.ID
	}
	return out
}

// 後台列貼文：不含草稿與已刪除的，新建立的在前；只列被下架的時候只有有複本的貼文，帶措施 ID；措施撤銷後複本刪除，不再列出。
func TestListAdminPostsFiltersTakenDownAndExcludesDraftsAndDeleted(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	td := f.takedowns(t)
	storeID, _ := f.exposableStore(t)
	st := f.store(t, storeID)

	taken := f.publishedPost(t, storeID, "被下架", "organic")
	f.advance(time.Minute)
	plain := f.publishedPost(t, storeID, "正常")
	f.advance(time.Minute)
	f.draft(t, storeID)
	f.advance(time.Minute)
	gone := f.publishedPost(t, storeID, "已刪除")
	must(t, f.module.DeleteOwnPost(ctx, st, gone.ID, gone.Version))
	measure := td.create(t, storeID, ref(seller.ContentPost, taken.ID))

	all := mustV[social.AdminPostPage](t)(f.module.ListAdminPosts(ctx, social.AdminPostFilter{}, nil, 0))
	if got := adminPostIDs(all); len(got) != 2 || got[0] != plain.ID || got[1] != taken.ID {
		t.Fatalf("全部 = %v，預期 [正常, 被下架]（不含草稿與已刪除）", got)
	}
	if all.Posts[0].TakedownMeasureID != nil || all.Posts[1].TakedownMeasureID == nil || *all.Posts[1].TakedownMeasureID != measure {
		t.Fatalf("措施 ID：%v、%v，預期 nil 與 %s", all.Posts[0].TakedownMeasureID, all.Posts[1].TakedownMeasureID, measure)
	}

	only := mustV[social.AdminPostPage](t)(f.module.ListAdminPosts(ctx, social.AdminPostFilter{TakenDown: true}, nil, 0))
	if got := adminPostIDs(only); len(got) != 1 || got[0] != taken.ID || *only.Posts[0].TakedownMeasureID != measure {
		t.Fatalf("只列下架 = %v", got)
	}

	td.revoke(t, storeID, measure, ref(seller.ContentPost, taken.ID))
	only = mustV[social.AdminPostPage](t)(f.module.ListAdminPosts(ctx, social.AdminPostFilter{TakenDown: true}, nil, 0))
	if len(only.Posts) != 0 {
		t.Fatalf("撤銷後不該再列出：%v", adminPostIDs(only))
	}
}

// keyset 分頁：每頁一篇，走完沒有重複也沒有漏；最後一頁沒有下一頁。
func TestListAdminPostsKeysetPagingHasNoDuplicatesOrGaps(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	var want []uuid.UUID
	for range 3 {
		want = append([]uuid.UUID{f.publishedPost(t, storeID, "貼文").ID}, want...)
		f.advance(time.Minute)
	}
	var got []uuid.UUID
	var cur *social.AdminPostCursor
	for pages := 0; ; pages++ {
		if pages > 3 {
			t.Fatal("分頁沒有結束")
		}
		p := mustV[social.AdminPostPage](t)(f.module.ListAdminPosts(ctx, social.AdminPostFilter{}, cur, 1))
		got = append(got, adminPostIDs(p)...)
		if p.Next == nil {
			break
		}
		cur = p.Next
	}
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("走完的順序 = %v，預期 %v", got, want)
	}
}
