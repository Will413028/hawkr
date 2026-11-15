//go:build integration

package social_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

func (f *fixture) draft(t *testing.T, storeID int64) social.Post {
	t.Helper()
	p, err := f.module.CreatePost(context.Background(), f.store(t, storeID), social.CreatePostInput{
		Body: "排程的貼文", ImageToken: fmt.Sprintf("tok-%d", f.next()),
	}, nil)
	must(t, err)
	return p
}

func (f *fixture) owner() social.ScheduledBy { return social.ScheduledBy{Kind: social.ScheduleByOwner} }

func (f *fixture) delegated(ref string) social.ScheduledBy {
	return social.ScheduledBy{Kind: social.ScheduleByDelegation, Ref: ref}
}

// current 讀貼文此刻的狀態（排程與取消排程會遞增版本，之後的操作要帶新的）。
func (f *fixture) current(t *testing.T, storeID int64, p social.Post) social.Post {
	t.Helper()
	return mustV[social.Post](t)(f.module.OwnPost(context.Background(), storeID, p.ID))
}

// schedule 以貼文此刻的版本排一個 lead 之後發布的排程。
func (f *fixture) schedule(t *testing.T, storeID int64, p social.Post, lead time.Duration, by social.ScheduledBy) social.Schedule {
	t.Helper()
	s, err := f.module.SchedulePost(context.Background(), f.store(t, storeID), social.SchedulePostInput{
		PostID: p.ID, Version: f.current(t, storeID, p).Version, PublishAt: f.clock().Add(lead), By: by,
	}, nil)
	must(t, err)
	return s.Schedule
}

// cancelSchedule 以貼文此刻的版本取消排程。
func (f *fixture) cancelSchedule(t *testing.T, storeID int64, p social.Post) error {
	t.Helper()
	_, err := f.module.CancelPostSchedule(context.Background(), f.store(t, storeID), p.ID, f.current(t, storeID, p).Version)
	return err
}

func (f *fixture) scheduleState(t *testing.T, id uuid.UUID) (state, reason string, resolved bool) {
	t.Helper()
	var r *string
	var at *time.Time
	must(t, f.db.Conn(context.Background()).QueryRow(context.Background(),
		"SELECT state, cancel_reason, resolved_at FROM social.post_schedules WHERE id = $1", id).Scan(&state, &r, &at))
	if r != nil {
		reason = *r
	}
	return state, reason, at != nil
}

func (f *fixture) postStatus(t *testing.T, id uuid.UUID) string {
	t.Helper()
	var s string
	must(t, f.db.Conn(context.Background()).QueryRow(context.Background(), "SELECT status FROM social.posts WHERE id = $1", id).Scan(&s))
	return s
}

func (f *fixture) wantSchedule(t *testing.T, id uuid.UUID, state, reason string) {
	t.Helper()
	gotState, gotReason, resolved := f.scheduleState(t, id)
	if gotState != state || gotReason != reason || resolved != (state != "PENDING") {
		t.Fatalf("排程 %s：state=%s reason=%q resolved=%v，要 %s %q", id, gotState, gotReason, resolved, state, reason)
	}
}

func (f *fixture) pendingCount(t *testing.T, postID uuid.UUID) int64 {
	return f.scalar(t, "SELECT count(*) FROM social.post_schedules WHERE post_id = $1 AND state = 'PENDING'", postID)
}

// 排程與到點的 job 在同一個 transaction 寫入：job 帶 schedule_id、ScheduledAt 是 publish_at；到點的執行發布貼文、排程轉 PUBLISHED，
// 事件的 origin 是 SCHEDULED 並帶排程 ID 與發起者；快取失效在同一個 transaction 排。
func TestScheduledPostIsPublishedWhenDue(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, accountID := f.exposableStore(t)
	p := f.draft(t, storeID)
	f.jobs.reset()
	s := f.schedule(t, storeID, p, time.Hour, f.owner())
	if s.State != social.SchedulePending || s.PostID != p.ID || s.StoreID != storeID || s.By.Kind != social.ScheduleByOwner || s.By.Ref != fmt.Sprint(accountID) {
		t.Fatalf("%+v", s)
	}
	if jobs := f.jobs.scheduledJobs(); len(jobs) != 1 || !jobs[s.ID].Equal(s.PublishAt) {
		t.Fatalf("job 帶 schedule_id、ScheduledAt 是 publish_at：%v，排程 %+v", jobs, s)
	}
	if got := f.postStatus(t, p.ID); got != "DRAFT" {
		t.Fatalf("到點前不發布：%s", got)
	}
	f.advance(time.Hour)
	f.jobs.reset()
	must(t, f.module.PublishScheduled(ctx, s.ID))
	f.wantSchedule(t, s.ID, "PUBLISHED", "")
	got := mustV[social.Post](t)(f.module.PublicPost(ctx, social.Caller{}, p.ID))
	if got.Status != social.StatusPublished || got.PublishedAt == nil || !got.PublishedAt.Equal(f.clock()) {
		t.Fatalf("%+v", got)
	}
	evs := f.events.publishedEvents()
	if len(evs) != 1 || evs[0].Origin != social.OriginScheduled || evs[0].ScheduleID != s.ID || evs[0].ScheduledBy != s.By || evs[0].PostID != p.ID {
		t.Fatalf("%+v", evs)
	}
	if n := len(f.jobs.revalidations()); n != 1 {
		t.Fatalf("排程發布在同一個 transaction 排快取失效：%v", f.jobs.revalidations())
	}
}

func TestManualPublishEmitsDirectOrigin(t *testing.T) {
	t.Parallel()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	p := f.draft(t, storeID)
	_, err := f.module.PublishPost(context.Background(), f.store(t, storeID), p.ID, p.Version, nil)
	must(t, err)
	evs := f.events.publishedEvents()
	if len(evs) != 1 || evs[0].Origin != social.OriginDirect || evs[0].ScheduleID != (uuid.UUID{}) || evs[0].ScheduledBy != (social.ScheduledBy{}) {
		t.Fatalf("%+v", evs)
	}
	f.publishedPost(t, storeID, "建立時直接發布")
	if evs := f.events.publishedEvents(); len(evs) != 2 || evs[1].Origin != social.OriginDirect {
		t.Fatalf("建立時直接發布也是 DIRECT：%+v", evs)
	}
}

// job 重試或重複執行只發布一次（排程列是真相，不是 PENDING 就什麼都不做）。
func TestScheduledPublishIsIdempotent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	p := f.draft(t, storeID)
	s := f.schedule(t, storeID, p, time.Hour, f.owner())
	f.advance(time.Hour)
	must(t, f.module.PublishScheduled(ctx, s.ID))
	before := mustV[social.Post](t)(f.module.OwnPost(ctx, storeID, p.ID))
	f.jobs.reset()
	must(t, f.module.PublishScheduled(ctx, s.ID))
	after := mustV[social.Post](t)(f.module.OwnPost(ctx, storeID, p.ID))
	if after.Version != before.Version || len(f.events.publishedEvents()) != 1 || len(f.jobs.revalidations()) != 0 {
		t.Fatalf("第二次執行什麼都不做：版本 %d→%d、事件 %d、失效 %v", before.Version, after.Version, len(f.events.publishedEvents()), f.jobs.revalidations())
	}
	must(t, f.module.PublishScheduled(ctx, uuid.NewV7()))
}

func TestScheduleBounds(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	p := f.draft(t, storeID)
	st := f.store(t, storeID)
	version := p.Version
	try := func(lead time.Duration) error {
		out, err := f.module.SchedulePost(ctx, st, social.SchedulePostInput{PostID: p.ID, Version: version, PublishAt: f.clock().Add(lead), By: f.owner()}, nil)
		if err == nil {
			version = out.PostVersion
		}
		return err
	}
	for _, lead := range []time.Duration{-time.Hour, 0, social.MinScheduleLead - time.Microsecond, social.MaxScheduleLead + time.Microsecond} {
		if err := try(lead); kindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("lead %v 要被拒絕：%v", lead, err)
		}
	}
	if n := f.pendingCount(t, p.ID); n != 0 {
		t.Fatalf("被拒絕的不寫入：%d", n)
	}
	for _, lead := range []time.Duration{social.MinScheduleLead, social.MaxScheduleLead} {
		must(t, try(lead))
	}
}

func TestScheduleStateAndVersionRules(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	st := f.store(t, storeID)
	in := func(p social.Post, version int64) social.SchedulePostInput {
		return social.SchedulePostInput{PostID: p.ID, Version: version, PublishAt: f.clock().Add(time.Hour), By: f.owner()}
	}
	pub := f.publishedPost(t, storeID, "已發布")
	if _, err := f.module.SchedulePost(ctx, st, in(pub, pub.Version), nil); kindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("已發布的不能排程：%v", err)
	}
	d := f.draft(t, storeID)
	if _, err := f.module.SchedulePost(ctx, st, in(d, 0), nil); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("要帶版本：%v", err)
	}
	if _, err := f.module.SchedulePost(ctx, st, in(d, d.Version+1), nil); kindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("版本不符：%v", err)
	}
	other, _ := f.exposableStore(t)
	if _, err := f.module.SchedulePost(ctx, f.store(t, other), in(d, d.Version), nil); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("別家的貼文：%v", err)
	}
	empty := mustV[social.Post](t)(f.module.CreatePost(ctx, st, social.CreatePostInput{Body: "沒有圖"}, nil))
	if _, err := f.module.SchedulePost(ctx, st, in(empty, empty.Version), nil); kindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("草稿不完整不能排程：%v", err)
	}
	un := f.publishedPost(t, storeID, "要下架")
	_, err := f.module.UnpublishPost(ctx, st, un.ID, un.Version, nil)
	must(t, err)
	un = mustV[social.Post](t)(f.module.OwnPost(ctx, storeID, un.ID))
	must(t, mustErr(f.module.SchedulePost(ctx, st, in(un, un.Version), nil)))
	if _, err := f.module.SchedulePost(ctx, st, social.SchedulePostInput{PostID: d.ID, Version: d.Version, PublishAt: f.clock().Add(time.Hour), By: social.ScheduledBy{Kind: "OTHER", Ref: "x"}}, nil); err == nil {
		t.Fatal("不認得的發起者種類要被拒絕")
	}
	if _, err := f.module.SchedulePost(ctx, st, social.SchedulePostInput{PostID: d.ID, Version: d.Version, PublishAt: f.clock().Add(time.Hour), By: f.delegated(" x")}, nil); err == nil {
		t.Fatal("代行的 ref 頭尾不能有空白")
	}
	if _, err := f.module.SchedulePost(ctx, st, social.SchedulePostInput{PostID: d.ID, Version: d.Version, PublishAt: f.clock().Add(time.Hour), By: f.delegated("")}, nil); err == nil {
		t.Fatal("代行的 ref 不能是空的")
	}
}

func mustErr[T any](_ T, err error) error { return err }

// 重新排程：舊的待發布排程轉 REPLACED（取消原因凍結、發事件），同一篇貼文同時只有一個 PENDING。
func TestRescheduleReplacesThePendingSchedule(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	p := f.draft(t, storeID)
	first := f.schedule(t, storeID, p, time.Hour, f.owner())
	second := f.schedule(t, storeID, p, 2*time.Hour, f.delegated("sess-1"))
	f.wantSchedule(t, first.ID, "CANCELED", "REPLACED")
	f.wantSchedule(t, second.ID, "PENDING", "")
	if n := f.pendingCount(t, p.ID); n != 1 {
		t.Fatalf("同時只有一個 PENDING：%d", n)
	}
	evs := f.events.canceledEvents()
	if len(evs) != 1 || evs[0].ScheduleID != first.ID || evs[0].PostID != p.ID || evs[0].Reason != social.CancelReplaced {
		t.Fatalf("%+v", evs)
	}
	f.advance(time.Hour)
	must(t, f.module.PublishScheduled(ctx, first.ID))
	if got := f.postStatus(t, p.ID); got != "DRAFT" {
		t.Fatalf("被取代的排程的 job 到點什麼都不做：%s", got)
	}
	got := mustV[social.Schedule](t)(f.module.PendingSchedule(ctx, storeID, p.ID))
	if got.ID != second.ID {
		t.Fatalf("%+v", got)
	}
}

func TestManualPublishCancelsTheSchedule(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	p := f.draft(t, storeID)
	s := f.schedule(t, storeID, p, time.Hour, f.delegated("sess"))
	_, err := f.module.PublishPost(ctx, f.store(t, storeID), p.ID, f.current(t, storeID, p).Version, nil)
	must(t, err)
	f.wantSchedule(t, s.ID, "CANCELED", "REPLACED")
	if evs := f.events.canceledEvents(); len(evs) != 1 || evs[0].ScheduleID != s.ID || evs[0].Reason != social.CancelReplaced {
		t.Fatalf("%+v", evs)
	}
	f.advance(time.Hour)
	must(t, f.module.PublishScheduled(ctx, s.ID))
	if len(f.events.publishedEvents()) != 1 || len(f.deleg.asked()) != 0 {
		t.Fatalf("到點什麼都不做（不問授權、不再發布）：%d 個事件、問了 %d 次", len(f.events.publishedEvents()), len(f.deleg.asked()))
	}
}

func TestDeleteCancelsTheSchedule(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	p := f.draft(t, storeID)
	s := f.schedule(t, storeID, p, time.Hour, f.owner())
	must(t, f.module.DeletePost(ctx, f.store(t, storeID), p.ID, 0))
	f.wantSchedule(t, s.ID, "CANCELED", "DELETED")
	if evs := f.events.canceledEvents(); len(evs) != 1 || evs[0].Reason != social.CancelDeleted {
		t.Fatalf("%+v", evs)
	}
	f.advance(time.Hour)
	must(t, f.module.PublishScheduled(ctx, s.ID))
	if n := len(f.events.publishedEvents()); n != 0 {
		t.Fatalf("已刪除的貼文不發布：%d", n)
	}
}

func TestTakedownCancelsTheSchedule(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	td := f.takedowns(t)
	storeID, _ := f.exposableStore(t)
	p := f.draft(t, storeID)
	other := f.draft(t, storeID)
	s := f.schedule(t, storeID, p, time.Hour, f.owner())
	keep := f.schedule(t, storeID, other, time.Hour, f.owner())
	td.create(t, storeID, ref(seller.ContentPost, p.ID))
	f.wantSchedule(t, s.ID, "CANCELED", "TAKEN_DOWN")
	f.wantSchedule(t, keep.ID, "PENDING", "")
	if evs := f.events.canceledEvents(); len(evs) != 1 || evs[0].Reason != social.CancelTakenDown || evs[0].PostID != p.ID {
		t.Fatalf("%+v", evs)
	}
	if revs := f.jobs.revalidations(); len(revs) == 0 {
		t.Fatal("取消在同一個 transaction 排快取失效")
	}
	f.advance(time.Hour)
	must(t, f.module.PublishScheduled(ctx, s.ID))
	if got := f.postStatus(t, p.ID); got != "DRAFT" {
		t.Fatalf("被下架的貼文不發布：%s", got)
	}
}

// 到點時貼文已被下架（排程之後才建立的複本直接寫入，模擬取消沒涵蓋到的路徑）：social 自己的檢查擋下，原因 TAKEN_DOWN，不問授權。
func TestDueScheduleOfATakenDownPostIsCanceledWithoutAskingTheAuthorizer(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	p := f.draft(t, storeID)
	s := f.schedule(t, storeID, p, time.Hour, f.delegated("sess"))
	id := uuid.NewV7()
	f.exec(t, `INSERT INTO social.content_takedowns (measure_id, post_id, created_at) VALUES ($1, $2, $3)`, id, p.ID, f.clock())
	f.advance(time.Hour)
	must(t, f.module.PublishScheduled(ctx, s.ID))
	f.wantSchedule(t, s.ID, "CANCELED", "TAKEN_DOWN")
	if len(f.deleg.asked()) != 0 || f.postStatus(t, p.ID) != "DRAFT" {
		t.Fatal("不問授權、不發布")
	}
}

// 到點時貼文列已被軟刪但排程仍是 PENDING（直接寫 SQL 模擬）：轉 DELETED，不發布。
func TestDueScheduleOfAGonePostIsCanceled(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	p := f.draft(t, storeID)
	s := f.schedule(t, storeID, p, time.Hour, f.owner())
	f.exec(t, "UPDATE social.posts SET deleted_at = $2 WHERE id = $1", p.ID, f.clock())
	f.advance(time.Hour)
	must(t, f.module.PublishScheduled(ctx, s.ID))
	f.wantSchedule(t, s.ID, "CANCELED", "DELETED")
}

// 到點時貼文不再滿足公開要件（直接寫 SQL 把內文清空，模擬編輯後的漏網）：轉 NOT_PUBLISHABLE、發 PostScheduleCanceled、不重試也不發布；
// 再跑一次（sweep 或重送的 job）什麼都不做，不會回到 PENDING。
func TestDueScheduleOfAnUnpublishablePostIsCanceledAndNotRetried(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	p := f.draft(t, storeID)
	s := f.schedule(t, storeID, p, time.Hour, f.owner())
	f.exec(t, "UPDATE social.posts SET body = '' WHERE id = $1", p.ID)
	f.advance(time.Hour)
	must(t, f.module.PublishScheduled(ctx, s.ID))
	f.wantSchedule(t, s.ID, "CANCELED", "NOT_PUBLISHABLE")
	if evs := f.events.canceledEvents(); len(evs) != 1 || evs[0].Reason != social.CancelNotPublishable || evs[0].PostID != p.ID || evs[0].ScheduleID != s.ID {
		t.Fatalf("PostScheduleCanceled 要發一次、原因 NOT_PUBLISHABLE：%+v", evs)
	}
	if f.postStatus(t, p.ID) != "DRAFT" {
		t.Fatal("不發布")
	}
	must(t, f.module.PublishScheduled(ctx, s.ID))
	f.wantSchedule(t, s.ID, "CANCELED", "NOT_PUBLISHABLE")
	if n := len(f.events.canceledEvents()); n != 1 {
		t.Fatalf("重跑不再發事件：%d", n)
	}
}

func TestPurgeOfAStoreCancelsItsSchedules(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	p := f.draft(t, storeID)
	s := f.schedule(t, storeID, p, time.Hour, f.delegated("sess"))
	must(t, f.module.PurgeStoreContent(ctx, storeID))
	f.wantSchedule(t, s.ID, "CANCELED", "DELETED")
	if evs := f.events.canceledEvents(); len(evs) != 1 || evs[0].ScheduleID != s.ID {
		t.Fatalf("%+v", evs)
	}
}

func TestOwnerCancelsTheSchedule(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	p := f.draft(t, storeID)
	s := f.schedule(t, storeID, p, time.Hour, f.delegated("sess"))
	other, _ := f.exposableStore(t)
	ver := f.current(t, storeID, p).Version
	if _, err := f.module.CancelPostSchedule(ctx, f.store(t, other), p.ID, ver); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("別家的貼文：%v", err)
	}
	if _, err := f.module.CancelPostSchedule(ctx, f.store(t, storeID), p.ID, 0); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("要帶版本：%v", err)
	}
	if _, err := f.module.CancelPostSchedule(ctx, f.store(t, storeID), p.ID, ver-1); kindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("版本不符：%v", err)
	}
	f.wantSchedule(t, s.ID, "PENDING", "")
	f.jobs.reset()
	if got := mustV[int64](t)(f.module.CancelPostSchedule(ctx, f.store(t, storeID), p.ID, ver)); got != ver+1 {
		t.Fatalf("取消回傳新的版本：%d，要 %d", got, ver+1)
	}
	f.wantSchedule(t, s.ID, "CANCELED", "OWNER_CANCELED")
	if got := f.current(t, storeID, p).Version; got != ver+1 {
		t.Fatalf("取消遞增版本：%d→%d", ver, got)
	}
	if len(f.jobs.revalidations()) != 0 {
		t.Fatalf("有排程的貼文不是公開的，取消不排快取失效：%v", f.jobs.revalidations())
	}
	if got := mustV[int64](t)(f.module.CancelPostSchedule(ctx, f.store(t, storeID), p.ID, 1)); got != ver+1 {
		t.Fatalf("沒有待發布的排程：冪等、不檢查版本、回傳目前的版本：%d", got)
	}
	if evs := f.events.canceledEvents(); len(evs) != 1 || evs[0].Reason != social.CancelOwnerCanceled {
		t.Fatalf("再取消一次是 no-op：%+v", evs)
	}
	if _, err := f.module.PendingSchedule(ctx, storeID, p.ID); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("%v", err)
	}
	f.advance(time.Hour)
	must(t, f.module.PublishScheduled(ctx, s.ID))
	if got := f.postStatus(t, p.ID); got != "DRAFT" {
		t.Fatalf("取消後到點不發布：%s", got)
	}
}

// 授權實作拒絕：排程轉 CANCELED（AUTHORITY_LOST），貼文不發布、不重試（呼叫沒有錯誤）、不會卡在 PENDING；其他錯誤讓 job 重試，整個 transaction 回滾。
func TestAuthorizerDenialCancelsWithoutRetry(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	p := f.draft(t, storeID)
	s := f.schedule(t, storeID, p, time.Hour, f.delegated("sess-9"))
	f.advance(time.Hour)

	f.deleg.set(errors.New("session 讀取暫時失敗"))
	if err := f.module.PublishScheduled(ctx, s.ID); err == nil {
		t.Fatal("暫時的錯誤要讓 job 重試")
	}
	f.wantSchedule(t, s.ID, "PENDING", "")
	if f.postStatus(t, p.ID) != "DRAFT" || len(f.events.publishedEvents()) != 0 || len(f.events.canceledEvents()) != 0 {
		t.Fatal("錯誤時整個 transaction 回滾")
	}

	f.deleg.set(fmt.Errorf("代行授權已撤回：%w", social.ErrScheduleDenied))
	f.jobs.reset()
	must(t, f.module.PublishScheduled(ctx, s.ID))
	f.wantSchedule(t, s.ID, "CANCELED", "AUTHORITY_LOST")
	if got := f.postStatus(t, p.ID); got != "DRAFT" {
		t.Fatalf("貼文不發布：%s", got)
	}
	if evs := f.events.canceledEvents(); len(evs) != 1 || evs[0].ScheduleID != s.ID || evs[0].Reason != social.CancelAuthorityLost {
		t.Fatalf("%+v", evs)
	}
	if len(f.events.publishedEvents()) != 0 || len(f.jobs.revalidations()) != 0 {
		t.Fatalf("不發 PostPublished；取消不排快取失效（貼文不是公開的）：%v", f.jobs.revalidations())
	}
	asked := f.deleg.asked()
	if len(asked) != 2 || asked[1].ScheduleID != s.ID || asked[1].PostID != p.ID || asked[1].StoreID != storeID || asked[1].By != f.delegated("sess-9") || !asked[1].PublishAt.Equal(s.PublishAt) {
		t.Fatalf("授權實作收到排程的事實：%+v", asked)
	}
	must(t, f.module.PublishScheduled(ctx, s.ID))
	if len(f.deleg.asked()) != 2 {
		t.Fatal("已取消的排程不再問授權")
	}
}

// 授權實作通過：代行排程發布，事件帶發起者；店主排程不問授權實作。
func TestAuthorizerIsConsultedPerKind(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	a, b := f.draft(t, storeID), f.draft(t, storeID)
	sa := f.schedule(t, storeID, a, time.Hour, f.delegated("sess-1"))
	sb := f.schedule(t, storeID, b, time.Hour, f.owner())
	f.advance(time.Hour)
	must(t, f.module.PublishScheduled(ctx, sa.ID))
	must(t, f.module.PublishScheduled(ctx, sb.ID))
	if asked := f.deleg.asked(); len(asked) != 1 || asked[0].ScheduleID != sa.ID {
		t.Fatalf("只有代行排程問代行的授權實作：%+v", asked)
	}
	f.wantSchedule(t, sa.ID, "PUBLISHED", "")
	f.wantSchedule(t, sb.ID, "PUBLISHED", "")
	byPost := map[uuid.UUID]social.PostPublished{}
	for _, e := range f.events.publishedEvents() {
		byPost[e.PostID] = e
	}
	if byPost[a.ID].ScheduledBy != f.delegated("sess-1") || byPost[a.ID].Origin != social.OriginScheduled || byPost[b.ID].ScheduledBy.Kind != social.ScheduleByOwner {
		t.Fatalf("%+v", byPost)
	}
}

func TestCancelScheduledByAndDelegatedSchedulesOfStore(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	otherStore, _ := f.exposableStore(t)
	mk := func(sid int64, by social.ScheduledBy) social.Schedule {
		return f.schedule(t, sid, f.draft(t, sid), time.Hour, by)
	}
	s1, s2, s3 := mk(storeID, f.delegated("sess-1")), mk(storeID, f.delegated("sess-2")), mk(storeID, f.delegated("sess-3"))
	own := mk(storeID, f.owner())
	foreign := mk(otherStore, f.delegated("sess-1"))

	f.jobs.reset()
	ids, err := f.module.CancelScheduledBy(ctx, social.ScheduleByDelegation, nil, social.CancelAuthorityLost)
	if err != nil || len(ids) != 0 || len(f.events.canceledEvents()) != 0 {
		t.Fatalf("空的 refs 是 no-op：%v %v", ids, err)
	}
	ids, err = f.module.CancelScheduledBy(ctx, social.ScheduleByDelegation, []string{"sess-1", "sess-2"}, social.CancelAuthorityLost)
	must(t, err)
	if len(ids) != 3 || !sameScheduleIDs(ids, s1.ID, s2.ID, foreign.ID) {
		t.Fatalf("取消這批 ref 的全部排程（含別家店的）：%v", ids)
	}
	for _, id := range []uuid.UUID{s1.ID, s2.ID, foreign.ID} {
		f.wantSchedule(t, id, "CANCELED", "AUTHORITY_LOST")
	}
	f.wantSchedule(t, s3.ID, "PENDING", "")
	f.wantSchedule(t, own.ID, "PENDING", "")
	if len(f.events.canceledEvents()) != 3 {
		t.Fatalf("每個被取消的排程一個事件：%+v", f.events.canceledEvents())
	}
	if ids, err = f.module.CancelScheduledBy(ctx, social.ScheduleByDelegation, []string{"sess-1"}, social.CancelAuthorityLost); err != nil || len(ids) != 0 {
		t.Fatalf("已取消的不重複取消：%v %v", ids, err)
	}

	ids, err = f.module.CancelDelegatedSchedulesOfStore(ctx, storeID, social.CancelAuthorityLost)
	must(t, err)
	if !sameScheduleIDs(ids, s3.ID) {
		t.Fatalf("只取消這家店的代行排程，店主排的不動：%v", ids)
	}
	f.wantSchedule(t, own.ID, "PENDING", "")
	if _, err := f.module.CancelScheduledBy(ctx, social.ScheduleByDelegation, []string{"x"}, "BOGUS"); err == nil {
		t.Fatal("不認得的原因要被拒絕")
	}
}

func sameScheduleIDs(got []uuid.UUID, want ...uuid.UUID) bool {
	if len(got) != len(want) {
		return false
	}
	for _, w := range want {
		found := false
		for _, g := range got {
			found = found || g == w
		}
		if !found {
			return false
		}
	}
	return true
}

// 發布與取消鎖同一列貼文而序列化：不論誰先，結束時排程與貼文的狀態一致（發布成功：PUBLISHED 加貼文公開；取消成功：CANCELED 加貼文仍是草稿）。
func TestScheduledPublishAndOwnerCancelSerialize(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	st := f.store(t, storeID)
	published, canceled := 0, 0
	for i := range 15 {
		p := f.draft(t, storeID)
		s := f.schedule(t, storeID, p, time.Hour, f.delegated(fmt.Sprintf("sess-%d", i)))
		ver := f.current(t, storeID, p).Version
		f.advance(time.Hour)
		start := make(chan struct{})
		var wg sync.WaitGroup
		var runErr, cancelErr error
		wg.Add(2)
		go func() { defer wg.Done(); <-start; runErr = f.module.PublishScheduled(ctx, s.ID) }()
		go func() { defer wg.Done(); <-start; _, cancelErr = f.module.CancelPostSchedule(ctx, st, p.ID, ver) }()
		close(start)
		wg.Wait()
		must(t, runErr)
		must(t, cancelErr)
		state, reason, _ := f.scheduleState(t, s.ID)
		status := f.postStatus(t, p.ID)
		switch {
		case state == "PUBLISHED" && reason == "" && status == "PUBLISHED":
			published++
		case state == "CANCELED" && reason == "OWNER_CANCELED" && status == "DRAFT":
			canceled++
		default:
			t.Fatalf("不一致的結果：排程 %s/%s，貼文 %s", state, reason, status)
		}
		if f.pendingCount(t, p.ID) != 0 {
			t.Fatal("不能卡在 PENDING")
		}
	}
	t.Logf("發布先 %d 次、取消先 %d 次", published, canceled)
}

// 編輯排程中的貼文仍要滿足公開要件（不能清掉唯一的圖或把內文清空）；沒有排程的草稿可以不完整。
func TestEditingAScheduledPostStillMeetsPublishRequirements(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	st := f.store(t, storeID)
	plain := f.draft(t, storeID)
	_, err := f.module.UpdatePost(ctx, st, social.UpdatePostInput{PostID: plain.ID, Version: plain.Version, Body: ptr(""), ClearImage: true}, nil)
	must(t, err)

	p := f.draft(t, storeID)
	s := f.schedule(t, storeID, p, time.Hour, f.owner())
	p = f.current(t, storeID, p)
	if _, err := f.module.UpdatePost(ctx, st, social.UpdatePostInput{PostID: p.ID, Version: p.Version, Body: ptr("新內文"), ClearImage: true}, nil); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("清掉圖要被拒絕：%v", err)
	}
	if _, err := f.module.UpdatePost(ctx, st, social.UpdatePostInput{PostID: p.ID, Version: p.Version, Body: ptr("")}, nil); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("清空內文要被拒絕：%v", err)
	}
	updated := mustV[social.Post](t)(f.module.UpdatePost(ctx, st, social.UpdatePostInput{PostID: p.ID, Version: p.Version, Body: ptr("新內文")}, nil))
	if updated.Body != "新內文" {
		t.Fatalf("%+v", updated)
	}
	f.wantSchedule(t, s.ID, "PENDING", "")
	must(t, f.cancelSchedule(t, storeID, p))
	_, err = f.module.UpdatePost(ctx, st, social.UpdatePostInput{PostID: p.ID, Version: f.current(t, storeID, p).Version, Body: ptr(""), ClearImage: true}, nil)
	must(t, err)
}

func TestListPendingSchedulesIsKeysetByPublishTime(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	other, _ := f.exposableStore(t)
	var want []uuid.UUID
	for i := range 3 {
		want = append(want, f.schedule(t, storeID, f.draft(t, storeID), time.Duration(i+1)*time.Hour, f.owner()).ID)
	}
	f.schedule(t, other, f.draft(t, other), time.Hour, f.owner())
	gone := f.schedule(t, storeID, f.draft(t, storeID), 4*time.Hour, f.owner())
	must(t, f.cancelSchedule(t, storeID, social.Post{ID: gone.PostID}))

	first := mustV[social.SchedulePage](t)(f.module.ListPendingSchedules(ctx, storeID, nil, 2))
	if len(first.Schedules) != 2 || first.Next == nil || first.Schedules[0].ID != want[0] || first.Schedules[1].ID != want[1] {
		t.Fatalf("%+v", first)
	}
	second := mustV[social.SchedulePage](t)(f.module.ListPendingSchedules(ctx, storeID, first.Next, 2))
	if len(second.Schedules) != 1 || second.Next != nil || second.Schedules[0].ID != want[2] {
		t.Fatalf("只列自己店的、待發布的：%+v", second)
	}
}

// 排程列的 CHECK 與索引：取消原因只在 CANCELED 有值、resolved_at 在離開 PENDING 時有值、同一篇貼文同時只有一個 PENDING。
func TestScheduleRowConstraints(t *testing.T) {
	t.Parallel()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	p := f.draft(t, storeID)
	ins := func(state string, reason, resolved any) error {
		_, err := f.db.Conn(context.Background()).Exec(context.Background(), `INSERT INTO social.post_schedules
			(id, post_id, store_id, publish_at, scheduled_by_kind, scheduled_by_ref, state, cancel_reason, created_at, resolved_at)
			VALUES ($1, $2, $3, $4, 'OWNER', 'a', $5, $6, $4, $7)`, uuid.NewV7(), p.ID, storeID, f.clock(), state, reason, resolved)
		return err
	}
	now := f.clock()
	for name, err := range map[string]error{
		"CANCELED 沒有原因":         ins("CANCELED", nil, now),
		"PENDING 有原因":           ins("PENDING", "DELETED", nil),
		"PENDING 有 resolved":    ins("PENDING", nil, now),
		"PUBLISHED 沒有 resolved": ins("PUBLISHED", nil, nil),
		"PUBLISHED 有原因":         ins("PUBLISHED", "DELETED", now),
		"不認得的原因":                ins("CANCELED", "BOGUS", now),
	} {
		if err == nil {
			t.Errorf("%s 要被拒絕", name)
		}
	}
	must(t, ins("PENDING", nil, nil))
	if err := ins("PENDING", nil, nil); err == nil {
		t.Error("同一篇貼文第二個 PENDING 要被拒絕")
	}
	must(t, ins("CANCELED", "REPLACED", now))
	must(t, ins("PUBLISHED", nil, now))
}

// 排程加入呼叫方的 transaction：回滾時排程列一起消失（job 的 transaction 性質由 platform/jobs 的 integration 測試守）。
func TestScheduleRollsBackWithTheCallersTransaction(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	p := f.draft(t, storeID)
	f.jobs.reset()
	rollback := errors.New("rollback")
	err := f.db.InTx(ctx, func(ctx context.Context) error {
		if _, err := f.module.SchedulePost(ctx, f.store(t, storeID), social.SchedulePostInput{PostID: p.ID, Version: p.Version, PublishAt: f.clock().Add(time.Hour), By: f.owner()}, nil); err != nil {
			return err
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
	if n := f.pendingCount(t, p.ID); n != 0 {
		t.Fatalf("回滾後沒有排程：%d", n)
	}
}

// 店主排程到點時要過和手動發布同一個店家授權：店家已不能經營（這裡是等待刪除）就取消為 AUTHORITY_LOST，不發布。
func TestOwnerScheduleIsCanceledWhenTheStoreHasNoRightToPost(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	p := f.draft(t, storeID)
	s := f.schedule(t, storeID, p, time.Hour, f.owner())
	f.exec(t, `WITH closure AS (
		INSERT INTO seller.store_closures (store_id, closed_at, closed_by) VALUES ($1, $2, 'ACCOUNT_OWNER') RETURNING closed_at)
		UPDATE seller.stores SET status = 'CLOSED', current_closed_at = (SELECT closed_at FROM closure) WHERE store_id = $1`, storeID, f.clock())
	f.advance(time.Hour)
	must(t, f.module.PublishScheduled(ctx, s.ID))
	f.wantSchedule(t, s.ID, "CANCELED", "AUTHORITY_LOST")
	if got := f.postStatus(t, p.ID); got != "DRAFT" || len(f.events.publishedEvents()) != 0 {
		t.Fatalf("不發布：%s、%d 個事件", got, len(f.events.publishedEvents()))
	}
}

// 店家只是暫時不滿足前置條件（等待刪除）不是永久失去權利：job 回錯誤重試，排程維持 PENDING。
func TestOwnerScheduleIsRetriedWhileThePreconditionIsMissing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	p := f.draft(t, storeID)
	s := f.schedule(t, storeID, p, time.Hour, f.owner())
	f.exec(t, "UPDATE seller.stores SET status = 'PENDING_DELETION' WHERE store_id = $1", storeID)
	f.advance(time.Hour)
	if err := f.module.PublishScheduled(ctx, s.ID); err == nil {
		t.Fatal("FailedPrecondition 是暫時的，要回錯誤讓 job 重試")
	}
	f.wantSchedule(t, s.ID, "PENDING", "")
	if len(f.events.canceledEvents()) != 0 || f.postStatus(t, p.ID) != "DRAFT" {
		t.Fatal("重試不取消、不發布")
	}
}

// 到點時貼文已經是公開的（直接寫 SQL 模擬取消沒涵蓋到的路徑）：排程轉 CANCELED（REPLACED）並發事件，不重試、不再發布，也不會卡在 PENDING。
func TestDueScheduleOfAnAlreadyPublishedPostResolvesAsReplaced(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	p := f.draft(t, storeID)
	s := f.schedule(t, storeID, p, time.Hour, f.owner())
	f.exec(t, "UPDATE social.posts SET status = 'PUBLISHED', published_at = $2 WHERE id = $1", p.ID, f.clock())
	f.advance(time.Hour)
	must(t, f.module.PublishScheduled(ctx, s.ID))
	f.wantSchedule(t, s.ID, "CANCELED", "REPLACED")
	if evs := f.events.canceledEvents(); len(evs) != 1 || evs[0].ScheduleID != s.ID || evs[0].Reason != social.CancelReplaced {
		t.Fatalf("PostScheduleCanceled 要發一次、原因 REPLACED：%+v", evs)
	}
	if len(f.events.publishedEvents()) != 0 || len(f.deleg.asked()) != 0 {
		t.Fatal("不再發布、不問授權")
	}
	must(t, f.module.PublishScheduled(ctx, s.ID))
	if n := len(f.events.canceledEvents()); n != 1 {
		t.Fatalf("重跑不再發事件：%d", n)
	}
}

// 排程遞增貼文版本：舊的 etag 隨之失效，編輯與發布要重新讀取；重排要帶新版本。
func TestScheduleBumpsThePostVersion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	st := f.store(t, storeID)
	p := f.draft(t, storeID)
	before := p.Version
	f.schedule(t, storeID, p, time.Hour, f.owner())
	after := f.current(t, storeID, p).Version
	if after != before+1 {
		t.Fatalf("排程遞增版本：%d→%d", before, after)
	}
	if _, err := f.module.PublishPost(ctx, st, p.ID, before, nil); kindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("舊版本發布要衝突：%v", err)
	}
	if _, err := f.module.UpdatePost(ctx, st, social.UpdatePostInput{PostID: p.ID, Version: before, Body: ptr("舊版本的編輯")}, nil); kindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("舊版本編輯要衝突：%v", err)
	}
	if _, err := f.module.SchedulePost(ctx, st, social.SchedulePostInput{PostID: p.ID, Version: before, PublishAt: f.clock().Add(2 * time.Hour), By: f.owner()}, nil); kindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("舊版本重排要衝突：%v", err)
	}
	f.schedule(t, storeID, p, 2*time.Hour, f.owner())
	if got := f.current(t, storeID, p).Version; got != after+1 {
		t.Fatalf("重排再遞增版本：%d→%d", after, got)
	}
}

// 被內容下架的貼文不能排程。
func TestScheduleRejectsAPostUnderTakedown(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	td := f.takedowns(t)
	storeID, _ := f.exposableStore(t)
	p := f.draft(t, storeID)
	td.create(t, storeID, ref(seller.ContentPost, p.ID))
	_, err := f.module.SchedulePost(ctx, f.store(t, storeID), social.SchedulePostInput{PostID: p.ID, Version: f.current(t, storeID, p).Version, PublishAt: f.clock().Add(time.Hour), By: f.owner()}, nil)
	if kindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("被下架的貼文不能排程：%v", err)
	}
	if n := f.pendingCount(t, p.ID); n != 0 {
		t.Fatalf("沒有排程：%d", n)
	}
}

// 公開要件只在寫入之前檢查：發布之後訂閱者的 InvalidInput 不是 NOT_PUBLISHABLE，整個 transaction 回滾、排程維持 PENDING、job 重試。
func TestSubscriberFailureAfterPublishRollsBackInsteadOfCancelingAsNotPublishable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	p := f.draft(t, storeID)
	s := f.schedule(t, storeID, p, time.Hour, f.owner())
	f.advance(time.Hour)
	events.Subscribe(f.bus, func(context.Context, social.PostPublished) error {
		return apperr.InvalidInput("%s", "訂閱者拒絕")
	})
	if err := f.module.PublishScheduled(ctx, s.ID); err == nil {
		t.Fatal("訂閱者的錯誤要原樣回傳讓 job 重試")
	}
	f.wantSchedule(t, s.ID, "PENDING", "")
	if got := f.postStatus(t, p.ID); got != "DRAFT" {
		t.Fatalf("回滾後貼文不是公開的：%s", got)
	}
	if len(f.events.canceledEvents()) != 0 {
		t.Fatal("不發 PostScheduleCanceled")
	}
}

// 系統取消（授權失效、下架、不可發布、過期）都遞增貼文版本，持有舊 etag 的編輯與發布要重新讀取。
func TestEverySystemCancelBumpsThePostVersion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cases := map[string]func(t *testing.T, f *fixture, storeID int64, p social.Post, s social.Schedule){
		"CancelScheduledBy": func(t *testing.T, f *fixture, _ int64, _ social.Post, _ social.Schedule) {
			mustV[[]uuid.UUID](t)(f.module.CancelScheduledBy(ctx, social.ScheduleByDelegation, []string{"sess"}, social.CancelAuthorityLost))
		},
		"CancelDelegatedSchedulesOfStore": func(t *testing.T, f *fixture, storeID int64, _ social.Post, _ social.Schedule) {
			mustV[[]uuid.UUID](t)(f.module.CancelDelegatedSchedulesOfStore(ctx, storeID, social.CancelAuthorityLost))
		},
		"takedown": func(t *testing.T, f *fixture, storeID int64, p social.Post, _ social.Schedule) {
			f.takedowns(t).create(t, storeID, ref(seller.ContentPost, p.ID))
		},
		"NOT_PUBLISHABLE": func(t *testing.T, f *fixture, _ int64, p social.Post, s social.Schedule) {
			f.exec(t, "UPDATE social.posts SET body = '' WHERE id = $1", p.ID)
			f.advance(time.Hour)
			must(t, f.module.PublishScheduled(ctx, s.ID))
		},
		"EXPIRED": func(t *testing.T, f *fixture, _ int64, _ social.Post, s social.Schedule) {
			f.advance(time.Hour + social.ScheduleExpiry + time.Minute)
			must(t, f.module.PublishScheduled(ctx, s.ID))
		},
		"AUTHORITY_LOST at run time": func(t *testing.T, f *fixture, _ int64, _ social.Post, s social.Schedule) {
			f.deleg.set(social.ErrScheduleDenied)
			f.advance(time.Hour)
			must(t, f.module.PublishScheduled(ctx, s.ID))
		},
	}
	for name, run := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := setup(t)
			storeID, _ := f.exposableStore(t)
			p := f.draft(t, storeID)
			s := f.schedule(t, storeID, p, time.Hour, f.delegated("sess"))
			before := f.current(t, storeID, p).Version
			run(t, f, storeID, p, s)
			if state, _, _ := f.scheduleState(t, s.ID); state != "CANCELED" {
				t.Fatalf("排程要被取消：%s", state)
			}
			if got := f.current(t, storeID, p).Version; got != before+1 {
				t.Fatalf("取消遞增版本恰好一次：%d→%d", before, got)
			}
		})
	}
}

// 到點超過 ScheduleExpiry 不補發，取消為 EXPIRED 而且不問授權；恰好在期限上仍發布。
func TestDueScheduleBeyondTheExpiryWindowIsCanceledAsExpired(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	onTime := f.draft(t, storeID)
	late := f.draft(t, storeID)
	a := f.schedule(t, storeID, onTime, time.Hour, f.delegated("sess"))
	b := f.schedule(t, storeID, late, time.Hour, f.delegated("sess"))
	f.advance(time.Hour + social.ScheduleExpiry)
	must(t, f.module.PublishScheduled(ctx, a.ID))
	f.wantSchedule(t, a.ID, "PUBLISHED", "")
	f.advance(time.Microsecond)
	f.jobs.reset()
	asked := len(f.deleg.asked())
	must(t, f.module.PublishScheduled(ctx, b.ID))
	f.wantSchedule(t, b.ID, "CANCELED", "EXPIRED")
	if f.postStatus(t, late.ID) != "DRAFT" || len(f.deleg.asked()) != asked {
		t.Fatal("過期不發布、不問授權")
	}
}

// 重試用盡：最後一次失敗在自己的 transaction 把排程取消為 RETRY_EXHAUSTED 並記 error log；還沒到最後一次不動。
func TestRetryExhaustionCancelsTheSchedule(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	p := f.draft(t, storeID)
	s := f.schedule(t, storeID, p, time.Hour, f.delegated("sess"))
	before := f.current(t, storeID, p).Version
	f.advance(time.Hour)
	f.deleg.set(errors.New("session 讀取暫時失敗"))
	if err := f.module.PublishScheduledAttempt(ctx, s.ID, false); err == nil {
		t.Fatal("失敗要回傳讓 job 重試")
	}
	f.wantSchedule(t, s.ID, "PENDING", "")
	if err := f.module.PublishScheduledAttempt(ctx, s.ID, true); err == nil {
		t.Fatal("最後一次仍回傳原錯誤")
	}
	f.wantSchedule(t, s.ID, "CANCELED", "RETRY_EXHAUSTED")
	if evs := f.events.canceledEvents(); len(evs) != 1 || evs[0].Reason != social.CancelRetryExhausted {
		t.Fatalf("%+v", evs)
	}
	if got := f.current(t, storeID, p).Version; got != before+1 {
		t.Fatalf("遞增版本：%d→%d", before, got)
	}
	if !strings.Contains(f.logs.String(), "level=ERROR") {
		t.Fatalf("要記 error log：%s", f.logs.String())
	}
}
