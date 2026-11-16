//go:build integration

package social_test

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func storeCommentIDs(page social.CommentPage) []uuid.UUID {
	out := make([]uuid.UUID, len(page.Comments))
	for i, c := range page.Comments {
		out[i] = c.ID
	}
	return out
}

// listAll 走完店主留言列表的所有頁（每頁 size 筆），確認 keyset 沒有重複或漏掉。
func (f *fixture) listAllStoreComments(t *testing.T, storeID int64, postID *uuid.UUID, unreplied bool, size int32) []uuid.UUID {
	t.Helper()
	var out []uuid.UUID
	var after *social.CommentCursor
	for range 50 {
		page, err := f.module.ListStoreComments(context.Background(), f.store(t, storeID), postID, unreplied, after, size)
		must(t, err)
		out = append(out, storeCommentIDs(page)...)
		if page.Next == nil {
			return out
		}
		after = page.Next
	}
	t.Fatal("分頁沒有結束")
	return nil
}

// 店家範圍的留言：不指定貼文是本店所有貼文（含已下架貼文）的頂層留言，不含別家的；unreplied 只有「作者不是本店、沒有刪除、沒有被隱藏、
// 底下沒有計入的本店回覆」的頂層留言（已刪除或被隱藏的店家回覆不算回覆；本店自己的頂層留言不算）。
func TestStoreCommentsScopeAndUnreplied(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	otherStore, _ := f.exposableStore(t)
	asStore := f.asStore(t, storeID)
	post1 := f.publishedPost(t, storeID, "貼文一")
	post2 := f.publishedPost(t, storeID, "貼文二")
	foreign := f.publishedPost(t, otherStore, "別家的貼文")
	comment := func(p social.Post, who social.Caller, body string) social.Comment {
		return mustV[social.Comment](t)(f.module.CreateComment(ctx, who, social.CommentInput{PostID: p.ID, Body: body}))
	}
	reply := func(p social.Post, parent social.Comment) social.Comment {
		return mustV[social.Comment](t)(f.module.CreateComment(ctx, asStore, social.CommentInput{PostID: p.ID, Body: "回覆", ParentID: &parent.ID}))
	}
	b := func() social.Caller { return f.asBuyer(f.newBuyer(t)) }

	unreplied := comment(post1, b(), "未回覆")
	replied := comment(post1, b(), "已回覆")
	reply(post1, replied)
	repliedThenDeleted := comment(post1, b(), "回覆被刪")
	must(t, f.module.DeleteComment(ctx, asStore, reply(post1, repliedThenDeleted).ID, 0))
	repliedThenHidden := comment(post1, b(), "回覆被隱藏")
	hiddenReply := reply(post1, repliedThenHidden)
	f.takedowns(t).create(t, storeID, ref(seller.ContentComment, hiddenReply.ID))
	hidden := comment(post1, b(), "被隱藏")
	_, err := f.module.Hide(ctx, social.HideInput{Target: social.Target{Kind: social.TargetComment, ID: hidden.ID}, Category: "SPAM", AdminID: 1})
	must(t, err)
	author := b()
	deleted := comment(post1, author, "作者刪掉")
	must(t, f.module.DeleteComment(ctx, author, deleted.ID, 0))
	own := comment(post1, asStore, "店家自己的留言")
	onPost2 := comment(post2, b(), "貼文二的留言")
	fromForeign := comment(foreign, b(), "別家貼文的留言")
	// 貼文下架後留言仍在店主的範圍內（店主讀自己的貼文不必公開）。
	_, err = f.module.UnpublishPost(ctx, f.store(t, storeID), post2.ID, f.current(t, storeID, post2).Version, nil)
	must(t, err)

	all := f.listAllStoreComments(t, storeID, nil, false, 3)
	for _, want := range []social.Comment{unreplied, replied, repliedThenDeleted, repliedThenHidden, own, onPost2} {
		if !slices.Contains(all, want.ID) {
			t.Errorf("店家範圍缺 %q", want.Body)
		}
	}
	for _, not := range []social.Comment{hidden, deleted, fromForeign} {
		if slices.Contains(all, not.ID) {
			t.Errorf("店家範圍不該有 %q", not.Body)
		}
	}
	if !slices.Equal(all, f.listAllStoreComments(t, storeID, nil, false, 50)) {
		t.Error("不同頁大小的結果不同：keyset 重複或漏掉了")
	}

	got := f.listAllStoreComments(t, storeID, nil, true, 2)
	want := []uuid.UUID{unreplied.ID, repliedThenDeleted.ID, repliedThenHidden.ID, onPost2.ID}
	slices.SortFunc(got, func(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) })
	slices.SortFunc(want, func(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) })
	if !slices.Equal(got, want) {
		t.Errorf("unreplied = %v，預期 %v", got, want)
	}
	for _, c := range []social.Comment{replied, hidden, deleted, own, fromForeign} {
		if slices.Contains(got, c.ID) {
			t.Errorf("unreplied 不該有 %q", c.Body)
		}
	}

	post1ID := post1.ID
	onlyPost1 := f.listAllStoreComments(t, storeID, &post1ID, true, 50)
	if slices.Contains(onlyPost1, onPost2.ID) || !slices.Contains(onlyPost1, unreplied.ID) || len(onlyPost1) != 3 {
		t.Errorf("指定貼文的 unreplied = %v", onlyPost1)
	}
	foreignID := foreign.ID
	if _, err := f.module.ListStoreComments(ctx, f.store(t, storeID), &foreignID, false, nil, 10); kindOf(err) != apperr.KindNotFound {
		t.Errorf("別家的貼文：%v", err)
	}
	// 店主回覆之後，那則留言離開未回覆。
	reply(post1, unreplied)
	if slices.Contains(f.listAllStoreComments(t, storeID, nil, true, 50), unreplied.ID) {
		t.Error("店主回覆後仍在未回覆")
	}
}

// 停權的店家仍讀得到自己的留言與貼文（收縮需要的讀取放行，CapContentContract），關閉的店家讀不到。
func TestStoreReadsFollowTheContractCapability(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	p := f.publishedPost(t, storeID, "貼文")
	live := f.store(t, storeID)
	suspended := live
	suspended.Sanctions = seller.SanctionedBy(seller.EffectiveMeasure{MeasureID: uuid.NewV7()})
	if _, err := f.module.ListStoreComments(ctx, suspended, nil, false, nil, 10); err != nil {
		t.Errorf("停權讀留言：%v", err)
	}
	if _, err := f.module.MyPost(ctx, suspended, p.ID); err != nil {
		t.Errorf("停權讀貼文：%v", err)
	}
	if _, err := f.module.ListMyPosts(ctx, suspended, "", nil, 10); err != nil {
		t.Errorf("停權列貼文：%v", err)
	}
	if _, err := f.module.StoreSocialSummary(ctx, suspended); err != nil {
		t.Errorf("停權動態數：%v", err)
	}
	closed := live
	closed.Conditions.Active = false
	if _, err := f.module.ListStoreComments(ctx, closed, nil, false, nil, 10); kindOf(err) != apperr.KindNotFound {
		t.Errorf("關閉的店家：%v", err)
	}
	if _, err := f.module.StoreSocialSummary(ctx, closed); kindOf(err) != apperr.KindNotFound {
		t.Errorf("關閉的店家動態數：%v", err)
	}
	otherStore, _ := f.exposableStore(t)
	if _, err := f.module.MyPost(ctx, f.store(t, otherStore), p.ID); kindOf(err) != apperr.KindNotFound {
		t.Errorf("別家的貼文：%v", err)
	}
}

// 動態數由 DB 計數：每個狀態都有一項（沒有是 0），不含已刪除的貼文與別家的貼文；待發布排程算有排程的貼文。
func TestStoreSocialSummaryCountsPostsByStatus(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	otherStore, _ := f.exposableStore(t)
	st := f.store(t, storeID)

	empty := mustV[social.StoreSummary](t)(f.module.StoreSocialSummary(ctx, st))
	if len(empty.Posts) != len(social.PostStatusValues()) || empty.Scheduled != 0 {
		t.Fatalf("沒有貼文時每個狀態都要有一項：%+v", empty)
	}
	for status, n := range empty.Posts {
		if n != 0 {
			t.Errorf("%s = %d", status, n)
		}
	}

	f.draft(t, storeID)
	scheduledDraft := f.draft(t, storeID)
	deletedDraft := f.draft(t, storeID)
	f.publishedPost(t, storeID, "發布一")
	f.publishedPost(t, storeID, "發布二")
	f.publishedPost(t, storeID, "發布三")
	unpublished := f.publishedPost(t, storeID, "下架")
	_, err := f.module.UnpublishPost(ctx, st, unpublished.ID, f.current(t, storeID, unpublished).Version, nil)
	must(t, err)
	must(t, f.module.DeleteOwnPost(ctx, st, deletedDraft.ID, 0))
	f.publishedPost(t, otherStore, "別家")
	f.draft(t, otherStore)
	f.schedule(t, storeID, scheduledDraft, time.Hour, f.owner())

	got := mustV[social.StoreSummary](t)(f.module.StoreSocialSummary(ctx, st))
	if got.Posts[social.StatusDraft] != 2 || got.Posts[social.StatusPublished] != 3 || got.Posts[social.StatusUnpublished] != 1 || got.Scheduled != 1 {
		t.Fatalf("summary = %+v，預期 草稿 2、已發布 3、已下架 1、待發布排程 1", got)
	}
	// 排程被取消後不再算。
	must(t, f.cancelSchedule(t, storeID, scheduledDraft))
	if got = mustV[social.StoreSummary](t)(f.module.StoreSocialSummary(ctx, st)); got.Scheduled != 0 || got.Posts[social.StatusDraft] != 2 {
		t.Fatalf("取消排程後：%+v", got)
	}
}

// AIP-155（A7-D23 O7）：同一家店以同一個 request_id 重送，回第一次建立的貼文，不重複建立、不重發事件。
func TestPostRequestIDReplayReturnsTheFirstResultWithoutDuplicating(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	otherStore, _ := f.exposableStore(t)
	st := f.store(t, storeID)
	rid := uuid.New()
	in := social.CreatePostInput{Body: "新品", ImageToken: "tok-first", Hashtags: []string{"tea"}, Publish: true, RequestID: &rid}

	first := mustV[social.Post](t)(f.module.CreatePost(ctx, st, in, nil))
	again := mustV[social.Post](t)(f.module.CreatePost(ctx, st, in, nil))
	if again.ID != first.ID || again.Body != first.Body || again.Status != social.StatusPublished {
		t.Fatalf("重送應回第一次的貼文：%v 與 %v", first.ID, again.ID)
	}
	if n := f.scalar(t, `SELECT count(*) FROM social.posts WHERE store_id = $1`, storeID); n != 1 {
		t.Fatalf("貼文數 = %d，預期 1", n)
	}
	if n := len(f.events.publishedEvents()); n != 1 {
		t.Fatalf("PostPublished 發了 %d 次，預期 1", n)
	}

	different := in
	different.Body = "別的內容"
	if _, err := f.module.CreatePost(ctx, st, different, nil); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("同 request_id 不同內容：%v", err)
	}
	asDraft := in
	asDraft.Publish = false
	if _, err := f.module.CreatePost(ctx, st, asDraft, nil); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("同 request_id 但是否發布不同：%v", err)
	}
	other := mustV[social.Post](t)(f.module.CreatePost(ctx, f.store(t, otherStore), social.CreatePostInput{Body: "新品", ImageToken: "tok-other", Hashtags: []string{"tea"}, Publish: true, RequestID: &rid}, nil))
	if other.ID == first.ID {
		t.Fatal("request_id 的範圍是店家：別家用同一個 id 是新貼文")
	}

	noKey := social.CreatePostInput{Body: "沒有 key"}
	a := mustV[social.Post](t)(f.module.CreatePost(ctx, st, noKey, nil))
	c := mustV[social.Post](t)(f.module.CreatePost(ctx, st, noKey, nil))
	if a.ID == c.ID {
		t.Fatal("沒帶 request_id 的重送各自建立")
	}

	// 貼文刪除後 request_id 仍占著：重送回原本那一篇，而它讀不到。
	must(t, f.module.DeleteOwnPost(ctx, st, first.ID, 0))
	if _, err := f.module.CreatePost(ctx, st, in, nil); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("刪除後重送：%v，預期 not_found", err)
	}
	if n := f.scalar(t, `SELECT count(*) FROM social.posts WHERE store_id = $1 AND request_id = $2`, storeID, rid); n != 1 {
		t.Fatalf("同一個 request_id 的貼文數 = %d，預期 1", n)
	}
}

func TestPostRequestIDConcurrentRetriesCreateOnePost(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	st := f.store(t, storeID)
	rid := uuid.New()
	got := make([]uuid.UUID, 8)
	var wg sync.WaitGroup
	for i := range got {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p, err := f.module.CreatePost(ctx, st, social.CreatePostInput{Body: "草稿", RequestID: &rid}, nil)
			if err != nil {
				t.Error(err)
				return
			}
			got[i] = p.ID
		}()
	}
	wg.Wait()
	for _, id := range got {
		if id != got[0] {
			t.Fatalf("並行重送回了不同的貼文：%v", got)
		}
	}
	if n := f.scalar(t, `SELECT count(*) FROM social.posts WHERE store_id = $1`, storeID); n != 1 {
		t.Fatalf("貼文數 = %d，預期 1", n)
	}
}

// 並行的同 request_id 重送：序列化在圖片轉正之前，全部回同一篇；內容不同的並行請求被拒，不會被錯誤地吞成「圖片不可用」。
func TestPostRequestIDConcurrentRetriesWithImageCreateOnePost(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	st := f.store(t, storeID)
	rid := uuid.New()
	in := social.CreatePostInput{Body: "並行", ImageToken: "tok-race", Hashtags: []string{"tea"}, Publish: true, RequestID: &rid}
	ids := make([]uuid.UUID, 8)
	var wg sync.WaitGroup
	for i := range ids {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p, err := f.module.CreatePost(ctx, st, in, nil)
			if err != nil {
				t.Error(err)
				return
			}
			ids[i] = p.ID
		}()
	}
	wg.Wait()
	for _, id := range ids {
		if id != ids[0] {
			t.Fatalf("並行重送回了不同的貼文：%v", ids)
		}
	}
	if n := f.scalar(t, `SELECT count(*) FROM social.posts WHERE store_id = $1`, storeID); n != 1 {
		t.Fatalf("貼文數 = %d，預期 1", n)
	}
	different := in
	different.Body = "內容不同"
	if _, err := f.module.CreatePost(ctx, st, different, nil); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("同 request_id 不同內容：%v", err)
	}
}

// 同 request_id 不同內容：循序與並行都回 InvalidInput，且只有先到的那一篇存在。
func TestPostRequestIDWithDifferentContentIsInvalidInputEvenConcurrently(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	st := f.store(t, storeID)
	rid := uuid.New()
	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = f.module.CreatePost(ctx, st, social.CreatePostInput{Body: fmt.Sprintf("內容%d", i%2), ImageToken: "tok-diff", Hashtags: []string{"tea"}, Publish: true, RequestID: &rid}, nil)
		}()
	}
	wg.Wait()
	ok, bad := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case kindOf(err) == apperr.KindInvalidInput:
			bad++
		default:
			t.Fatalf("非預期的錯誤：%v", err)
		}
	}
	if ok == 0 || bad == 0 {
		t.Fatalf("成功 %d、被拒 %d：兩種內容都應該各自有結果", ok, bad)
	}
	if n := f.scalar(t, `SELECT count(*) FROM social.posts WHERE store_id = $1`, storeID); n != 1 {
		t.Fatalf("貼文數 = %d，預期 1", n)
	}
}

// Update 的回應是交易內讀出的寫入結果。
func TestUpdatePostResponseReflectsTheWrite(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	st := f.store(t, storeID)
	p := mustV[social.Post](t)(f.module.CreatePost(ctx, st, social.CreatePostInput{Body: "舊", Hashtags: []string{"tea"}}, nil))
	tags := []string{"coffee"}
	up := mustV[social.Post](t)(f.module.UpdatePost(ctx, st, social.UpdatePostInput{PostID: p.ID, Version: p.Version, Body: ptr("新"), Hashtags: &tags}, nil))
	if up.Body != "新" || up.Version != p.Version+1 || len(up.Hashtags) != 1 || up.Hashtags[0].Name != "coffee" {
		t.Fatalf("回應 = %+v", up)
	}
	if _, err := f.module.UpdatePost(ctx, st, social.UpdatePostInput{PostID: p.ID, Version: p.Version, Body: ptr("過期")}, nil); kindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("過期版本：%v", err)
	}
}
