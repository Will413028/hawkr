//go:build integration

package social_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/delegatedactor"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

// A7-D26：ctx 帶著代行者時，貼文的每個寫入在 social 自己的 transaction 內發一個 PostWrittenByActor（含冪等的 no-op）；沒有代行者（店主）不發；
// 有代行者卻沒有訂閱者時失敗關閉：寫入 rollback。
func TestActorWritesPublishOneEventEachAndFailClosedWithoutASubscriber(t *testing.T) {
	t.Parallel()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	st := f.store(t, storeID)
	actor := delegatedactor.Actor{SessionID: 7, StoreID: storeID, Action: "POST_CREATE", RequestID: "r-1", Reason: "測試"}
	actx := delegatedactor.With(context.Background(), actor)

	before := f.scalar(t, `SELECT count(*) FROM social.posts WHERE store_id = $1`, storeID)
	if _, err := f.module.CreatePost(actx, st, social.CreatePostInput{Body: "沒有訂閱者", ImageToken: fmt.Sprintf("tok-%d", f.next())}); err == nil {
		t.Fatal("代行寫入沒有稽核的訂閱者卻成功")
	}
	if n := f.scalar(t, `SELECT count(*) FROM social.posts WHERE store_id = $1`, storeID); n != before {
		t.Fatalf("失敗關閉後貼文 %d -> %d，預期 transaction rollback", before, n)
	}

	var got []social.PostWrittenByActor
	events.Subscribe(f.bus, func(_ context.Context, e social.PostWrittenByActor) error {
		got = append(got, e)
		return nil
	})
	if _, err := f.module.CreatePost(context.Background(), st, social.CreatePostInput{Body: "店主自己的", ImageToken: fmt.Sprintf("tok-%d", f.next())}); err != nil || len(got) != 0 {
		t.Fatalf("店主的寫入：err=%v 事件=%d，預期沒有事件", err, len(got))
	}

	p, err := f.module.CreatePost(actx, st, social.CreatePostInput{Body: "代行建立", ImageToken: fmt.Sprintf("tok-%d", f.next())})
	must(t, err)
	p, err = f.module.UpdatePost(actx, st, social.UpdatePostInput{PostID: p.ID, Version: p.Version, Body: ptr("代行修改")})
	must(t, err)
	p, err = f.module.PublishPost(actx, st, p.ID, p.Version)
	must(t, err)
	if _, err = f.module.PublishPost(actx, st, p.ID, p.Version); err != nil {
		t.Fatal(err)
	}
	p, err = f.module.UnpublishPost(actx, st, p.ID, p.Version)
	must(t, err)
	if _, err = f.module.UnpublishPost(actx, st, p.ID, p.Version); err != nil {
		t.Fatal(err)
	}
	s, err := f.module.SchedulePost(actx, st, social.SchedulePostInput{
		PostID: p.ID, Version: p.Version, PublishAt: f.clock().Add(2 * time.Hour), By: social.ScheduledBy{Kind: social.ScheduleByDelegation, Ref: "7"},
	})
	must(t, err)

	want := []social.PostWriteKind{
		social.PostWriteCreated, social.PostWriteUpdated, social.PostWritePublished, social.PostWritePublished,
		social.PostWriteUnpublished, social.PostWriteUnpublished, social.PostWriteScheduled,
	}
	if len(got) != len(want) {
		t.Fatalf("事件 %d 個，預期 %d：%+v", len(got), len(want), got)
	}
	for i, k := range want {
		if got[i].Kind != k || got[i].PostID != p.ID || got[i].StoreID != storeID || got[i].Actor.SessionID != 7 || got[i].Version <= 0 {
			t.Fatalf("第 %d 個事件 = %+v，預期 %s", i, got[i], k)
		}
	}
	if got[0].Preview != "代行建立" || got[6].Version != s.Post.Version {
		t.Fatalf("預覽 %q、排程後版本 %d（預期 %d）", got[0].Preview, got[6].Version, s.Post.Version)
	}
}
