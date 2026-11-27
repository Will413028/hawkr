//go:build integration

package social_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/delegatedactor"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

// A7-D27：貼文的每個成功寫入在 social 自己的 transaction 內發一個一般的領域事件（含冪等的 no-op），代行者是方法的明確參數、作為事件的選用附帶資訊；
// 店主的寫入（actor 傳 nil）也發，Actor 是 nil；帶代行者卻沒有訂閱者時失敗關閉（events.PublishRequired）：寫入 rollback。
func TestPostWritesPublishOneGeneralEventEachAndFailClosedWithoutASubscriber(t *testing.T) {
	t.Parallel()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	st := f.store(t, storeID)
	actor := &delegatedactor.Actor{SessionID: 7, StoreID: storeID, RequestID: "r-1", Reason: "測試"}
	ctx := context.Background()

	events.Subscribe(f.bus, func(context.Context, social.PostCreated) error { return nil })
	before := f.scalar(t, `SELECT count(*) FROM social.posts WHERE store_id = $1`, storeID)
	if _, err := f.module.CreatePost(ctx, st, social.CreatePostInput{Body: "沒有訂閱者", ImageToken: fmt.Sprintf("tok-%d", f.next())}, actor); !errors.Is(err, events.ErrNoSubscriber) {
		t.Fatalf("代行寫入沒有訂閱者：err=%v，預期 ErrNoSubscriber", err)
	}
	if n := f.scalar(t, `SELECT count(*) FROM social.posts WHERE store_id = $1`, storeID); n != before {
		t.Fatalf("失敗關閉後貼文 %d -> %d，預期 transaction rollback", before, n)
	}

	type seen struct {
		kind  string
		w     social.PostWritten
		actor *delegatedactor.Actor
	}
	var got []seen
	sub := func(kind string) func(social.PostWritten) {
		return func(w social.PostWritten) { got = append(got, seen{kind, w, w.Actor}) }
	}
	on := func(kind string, add func(*events.Bus, func(social.PostWritten))) { add(f.bus, sub(kind)) }
	on("created", func(b *events.Bus, h func(social.PostWritten)) {
		events.SubscribeNamed(b, delegatedactor.AuditHandler, func(_ context.Context, e social.PostCreated) error { h(e.PostWritten); return nil })
	})
	on("updated", func(b *events.Bus, h func(social.PostWritten)) {
		events.SubscribeNamed(b, delegatedactor.AuditHandler, func(_ context.Context, e social.PostUpdated) error { h(e.PostWritten); return nil })
	})
	on("unpublished", func(b *events.Bus, h func(social.PostWritten)) {
		events.SubscribeNamed(b, delegatedactor.AuditHandler, func(_ context.Context, e social.PostUnpublished) error { h(e.PostWritten); return nil })
	})
	on("scheduled", func(b *events.Bus, h func(social.PostWritten)) {
		events.SubscribeNamed(b, delegatedactor.AuditHandler, func(_ context.Context, e social.PostScheduled) error { h(e.PostWritten); return nil })
	})
	on("publish-repeated", func(b *events.Bus, h func(social.PostWritten)) {
		events.SubscribeNamed(b, delegatedactor.AuditHandler, func(_ context.Context, e social.PostPublishRepeated) error { h(e.PostWritten); return nil })
	})
	on("unpublish-repeated", func(b *events.Bus, h func(social.PostWritten)) {
		events.SubscribeNamed(b, delegatedactor.AuditHandler, func(_ context.Context, e social.PostUnpublishRepeated) error { h(e.PostWritten); return nil })
	})
	var published []social.PostPublished
	events.SubscribeNamed(f.bus, delegatedactor.AuditHandler, func(_ context.Context, e social.PostPublished) error { published = append(published, e); return nil })

	own, err := f.module.CreatePost(ctx, st, social.CreatePostInput{Body: "店主自己的", ImageToken: fmt.Sprintf("tok-%d", f.next())}, nil)
	must(t, err)
	if len(got) != 1 || got[0].kind != "created" || got[0].actor != nil || got[0].w.PostID != own.ID {
		t.Fatalf("店主的建立：事件 %+v，預期一個 Actor 為 nil 的 created", got)
	}
	got = nil

	p, err := f.module.CreatePost(ctx, st, social.CreatePostInput{Body: "代行建立", ImageToken: fmt.Sprintf("tok-%d", f.next())}, actor)
	must(t, err)
	p, err = f.module.UpdatePost(ctx, st, social.UpdatePostInput{PostID: p.ID, Version: p.Version, Body: ptr("代行修改")}, actor)
	must(t, err)
	p, err = f.module.PublishPost(ctx, st, p.ID, p.Version, actor)
	must(t, err)
	if _, err = f.module.PublishPost(ctx, st, p.ID, p.Version, actor); err != nil {
		t.Fatal(err)
	}
	p, err = f.module.UnpublishPost(ctx, st, p.ID, p.Version, actor)
	must(t, err)
	if _, err = f.module.UnpublishPost(ctx, st, p.ID, p.Version, actor); err != nil {
		t.Fatal(err)
	}
	s, err := f.module.SchedulePost(ctx, st, social.SchedulePostInput{
		PostID: p.ID, Version: p.Version, PublishAt: f.clock().Add(2 * time.Hour), By: social.ScheduledBy{Kind: social.ScheduleByDelegation, Ref: "7"},
	}, actor)
	must(t, err)

	want := []string{"created", "updated", "publish-repeated", "unpublished", "unpublish-repeated", "scheduled"}
	if len(got) != len(want) {
		t.Fatalf("事件 %d 個，預期 %d：%+v", len(got), len(want), got)
	}
	for i, k := range want {
		if got[i].kind != k || got[i].w.PostID != p.ID || got[i].w.StoreID != storeID || got[i].actor == nil || got[i].actor.SessionID != 7 || got[i].w.Version <= 0 {
			t.Fatalf("第 %d 個事件 = %+v，預期 %s", i, got[i], k)
		}
	}
	if len(published) != 1 || published[0].Actor == nil || published[0].Actor.SessionID != 7 || published[0].PostID != p.ID || published[0].Version <= 0 {
		t.Fatalf("PostPublished = %+v，預期一個帶代行者的", published)
	}
	if got[0].w.Preview != "代行建立" || got[5].w.Version != s.Post.Version {
		t.Fatalf("預覽 %q、排程後版本 %d（預期 %d）", got[0].w.Preview, got[5].w.Version, s.Post.Version)
	}
}
