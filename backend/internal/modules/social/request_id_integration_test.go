//go:build integration

package social_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

// AIP-155（A7-D23 O7）：同一個互動者以同一個 request_id 重送，回第一次建立的留言，不重複建立、不重發事件。
func TestCommentRequestIDReplayReturnsTheFirstResultWithoutDuplicating(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	var created atomic.Int32
	events.Subscribe(f.bus, func(context.Context, social.CommentCreated) error { created.Add(1); return nil })
	storeID, _ := f.exposableStore(t)
	b := f.newBuyer(t)
	p := f.publishedPost(t, storeID, "貼文")
	rid := uuid.New()
	in := social.CommentInput{PostID: p.ID, Body: "留言", RequestID: &rid}

	first := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), in))
	again := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), in))
	if again.ID != first.ID || again.Body != first.Body {
		t.Fatalf("重送應回第一次的留言：%v 與 %v", first.ID, again.ID)
	}
	if n := f.scalar(t, `SELECT count(*) FROM social.comments WHERE post_id = $1`, p.ID); n != 1 {
		t.Fatalf("留言數 = %d，預期 1", n)
	}
	if got := mustV[social.Post](t)(f.module.PublicPost(ctx, social.Caller{}, p.ID)); got.Stats.Comments != 1 {
		t.Fatalf("貼文留言數 = %d，預期 1", got.Stats.Comments)
	}
	if created.Load() != 1 {
		t.Fatalf("CommentCreated 發了 %d 次，預期 1", created.Load())
	}

	other := social.CommentInput{PostID: p.ID, Body: "別的內容", RequestID: &rid}
	if _, err := f.module.CreateComment(ctx, f.asBuyer(b), other); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("同 request_id 不同內容：%v", err)
	}

	other2 := f.newBuyer(t)
	second := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(other2), in))
	if second.ID == first.ID {
		t.Fatal("request_id 的範圍是互動者：別人用同一個 id 是新留言")
	}

	noKey := social.CommentInput{PostID: p.ID, Body: "留言"}
	a := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), noKey))
	c := mustV[social.Comment](t)(f.module.CreateComment(ctx, f.asBuyer(b), noKey))
	if a.ID == c.ID {
		t.Fatal("沒帶 request_id 的重送各自建立")
	}
}

func TestCommentRequestIDConcurrentRetriesCreateOneComment(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	b := f.newBuyer(t)
	p := f.publishedPost(t, storeID, "貼文")
	rid := uuid.New()
	ids := make([]uuid.UUID, 8)
	var wg sync.WaitGroup
	for i := range ids {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, err := f.module.CreateComment(ctx, f.asBuyer(b), social.CommentInput{PostID: p.ID, Body: "留言", RequestID: &rid})
			if err != nil {
				t.Error(err)
				return
			}
			ids[i] = c.ID
		}()
	}
	wg.Wait()
	for _, id := range ids {
		if id != ids[0] {
			t.Fatalf("並行重送回了不同的留言：%v", ids)
		}
	}
	if n := f.scalar(t, `SELECT count(*) FROM social.comments WHERE post_id = $1`, p.ID); n != 1 {
		t.Fatalf("留言數 = %d，預期 1", n)
	}
}

func TestForumEntryRequestIDReplayReturnsTheFirstResultWithoutDuplicating(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	b := f.newBuyer(t)
	rid := uuid.New()
	in := social.ForumInput{StoreID: storeID, Body: "討論", RequestID: &rid}

	first := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(b), in))
	again := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(b), in))
	if again.ID != first.ID {
		t.Fatalf("重送應回第一次的項目：%v 與 %v", first.ID, again.ID)
	}
	if n := f.scalar(t, `SELECT count(*) FROM social.forum_entries WHERE store_id = $1`, storeID); n != 1 {
		t.Fatalf("項目數 = %d，預期 1", n)
	}
	if _, err := f.module.CreateForumEntry(ctx, f.asBuyer(b), social.ForumInput{StoreID: storeID, Body: "別的內容", RequestID: &rid}); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("同 request_id 不同內容：%v", err)
	}
	if second := mustV[social.ForumEntry](t)(f.module.CreateForumEntry(ctx, f.asBuyer(f.newBuyer(t)), in)); second.ID == first.ID {
		t.Fatal("request_id 的範圍是互動者")
	}
}
