//go:build integration

package delegation

import (
	"context"
	"fmt"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/delegation/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// 代行貼文的 use case：每個寫入在同一個 transaction 內先寫單段 SUCCEEDED 稽核再呼叫 social；request_id 是 (session, 冪等鍵)。

type operator struct {
	storeID int64
	admin   int64
	session issuedSession
}

// newOperator 建一位開通的管理員、含三個貼文能力的店家授權與進行中的 session；seller 用真的讀取（貼文要經 social 的店家能力檢查）。
func (f *fixture) newOperator(t *testing.T, tag string, caps ...Capability) operator {
	t.Helper()
	f.storeFn = func(id int64) (seller.Store, error) { return f.sellers.Store(context.Background(), id) }
	if len(caps) == 0 {
		caps = []Capability{CapabilityPostsRead, CapabilityPostsWrite, CapabilityPostsPublish}
	}
	storeID := f.exposableStore(t)
	admin := f.delegationAdmin(t, tag+"@hawkr.example", true)
	f.grant(t, admin, storeID, append([]Capability{CapabilityProductsRead}, caps...)...)
	return operator{storeID: storeID, admin: admin, session: f.start(t, startInput(admin, storeID, "ik-"+tag, caps...))}
}

// caller 是 interceptor 解出的代行：以憑證重新解析（與傳輸層相同）。
func (f *fixture) caller(t *testing.T, o operator) DelegatedCaller {
	t.Helper()
	s, err := f.m.ResolveDelegation(context.Background(), o.session.Credential, true)
	if err != nil {
		t.Fatal(err)
	}
	return DelegatedCaller{Session: s, CorrelationID: "corr-it", UserAgent: "integration-test"}
}

func (f *fixture) op(reason string) DelegatedOperation {
	return DelegatedOperation{RequestID: uuid.New(), Reason: reason, NotifySeller: true}
}

func (f *fixture) auditRows(t *testing.T, sessionID int64, action string) int {
	t.Helper()
	return f.count(t, `SELECT count(*) FROM delegation.admin_operation_audit_logs WHERE session_id = $1 AND action = $2`, sessionID, action)
}

func (f *fixture) auditRowsOfKey(t *testing.T, sessionID int64, key string) int {
	t.Helper()
	return f.count(t, `SELECT count(*) FROM delegation.admin_operation_audit_logs WHERE session_id = $1 AND idempotency_key = $2`, sessionID, key)
}

func (f *fixture) postCount(t *testing.T, storeID int64) int {
	t.Helper()
	return f.count(t, `SELECT count(*) FROM social.posts WHERE store_id = $1`, storeID)
}

func create(f *fixture, t *testing.T, o operator, op DelegatedOperation, body string) DelegatedPostResult {
	t.Helper()
	r, err := f.m.DelegatedCreatePost(context.Background(), f.caller(t, o), DelegatedCreatePostInput{Op: op, Body: body, ImageToken: fmt.Sprintf("tok-%d", f.next())})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// 每個寫入都留下恰好一列 SUCCEEDED 的稽核：動作、能力、資源、版本、操作理由、冪等鍵與 request_id 都對。
func TestEveryDelegatedPostWriteLeavesOneSucceededAuditRow(t *testing.T) {
	f := build(t, true)
	ctx := context.Background()
	o := f.newOperator(t, "writer")
	sid := o.session.Session.ID

	created := create(f, t, o, f.op("店家請我們代為建立"), "代行建立的貼文")
	if created.Replayed || created.Post.Status != social.StatusDraft {
		t.Fatalf("created = %+v", created)
	}
	updOp := f.op("修改錯字")
	body := "改過的貼文"
	updated, err := f.m.DelegatedUpdatePost(ctx, f.caller(t, o), DelegatedUpdatePostInput{Op: updOp, Update: social.UpdatePostInput{PostID: created.Post.ID, Version: created.Post.Version, Body: &body}})
	if err != nil {
		t.Fatal(err)
	}
	pubOp := f.op("店家要求上架")
	published, err := f.m.DelegatedPublishPost(ctx, f.caller(t, o), DelegatedPostTransitionInput{Op: pubOp, PostID: created.Post.ID, Version: updated.Post.Version})
	if err != nil {
		t.Fatal(err)
	}
	if published.Post.Status != social.StatusPublished {
		t.Fatalf("published = %+v", published.Post)
	}
	unOp := f.op("內容有爭議先下架")
	unpublished, err := f.m.DelegatedUnpublishPost(ctx, f.caller(t, o), DelegatedPostTransitionInput{Op: unOp, PostID: created.Post.ID, Version: published.Post.Version})
	if err != nil {
		t.Fatal(err)
	}
	schedOp := f.op("改排在週末")
	at := f.now.Add(2 * time.Hour)
	scheduled, err := f.m.DelegatedSchedulePost(ctx, f.caller(t, o), DelegatedSchedulePostInput{Op: schedOp, PostID: created.Post.ID, Version: unpublished.Post.Version, PublishAt: at})
	if err != nil {
		t.Fatal(err)
	}
	if scheduled.PublishAt == nil || !scheduled.PublishAt.Equal(at) {
		t.Fatalf("排定時間 = %v", scheduled.PublishAt)
	}

	want := []struct {
		action, capability, reason string
		op                         DelegatedOperation
		result                     DelegatedPostResult
	}{
		{"POST_CREATE", "posts.write", "店家請我們代為建立", DelegatedOperation{}, created},
		{"POST_UPDATE", "posts.write", "修改錯字", updOp, updated},
		{"POST_PUBLISH", "posts.publish", "店家要求上架", pubOp, published},
		{"POST_UNPUBLISH", "posts.publish", "內容有爭議先下架", unOp, unpublished},
		{"POST_SCHEDULE", "posts.publish", "改排在週末", schedOp, scheduled},
	}
	for _, w := range want {
		if n := f.auditRows(t, sid, w.action); n != 1 {
			t.Fatalf("%s 的稽核要恰好一列：%d", w.action, n)
		}
		var (
			result, origin, capability, resourceType, resourceID, key, requestID string
			reason                                                               *string
			notify, confirmed                                                    bool
			after                                                                *int64
			id                                                                   uuid.UUID
		)
		if err := f.db.Pool().QueryRow(ctx, `SELECT id, result, origin, capability, resource_type, resource_id, idempotency_key, request_id, operation_reason, notify_seller,
			confirmed_at IS NOT NULL, resource_version_after FROM delegation.admin_operation_audit_logs WHERE session_id = $1 AND action = $2`, sid, w.action).
			Scan(&id, &result, &origin, &capability, &resourceType, &resourceID, &key, &requestID, &reason, &notify, &confirmed, &after); err != nil {
			t.Fatal(err)
		}
		if result != "SUCCEEDED" || origin != "INTERACTIVE" || capability != w.capability || resourceType != "POST" || resourceID != created.Post.ID.String() ||
			reason == nil || *reason != w.reason || !notify || !confirmed || id != w.result.AuditLogID || after == nil || *after != w.result.Post.Version {
			t.Fatalf("%s 的稽核列 = %s %s %s %s %s reason=%v notify=%v confirmed=%v id=%s after=%v", w.action, result, origin, capability, resourceType, resourceID, reason, notify, confirmed, id, after)
		}
		if w.op.RequestID != (uuid.UUID{}) && (key != w.op.RequestID.String() || requestID != w.op.RequestID.String()) {
			t.Fatalf("%s 的冪等鍵 %q、request_id %q，預期都是 %s", w.action, key, requestID, w.op.RequestID)
		}
	}
	// 通知賣家的完成事件：五個寫入各一則。
	if len(f.completed) != 5 {
		t.Fatalf("完成通知 %d 則，預期 5", len(f.completed))
	}
	// 排程是店家的、發起者是這個 session，且以 session 的名義到點發布時繼承 POST_SCHEDULE 的理由。
	var kind, ref string
	if err := f.db.Pool().QueryRow(ctx, `SELECT scheduled_by_kind, scheduled_by_ref FROM social.post_schedules WHERE post_id = $1 AND state = 'PENDING'`, scheduled.Post.ID).Scan(&kind, &ref); err != nil {
		t.Fatal(err)
	}
	if kind != "DELEGATION" || ref != fmt.Sprint(sid) {
		t.Fatalf("排程的發起者 = %s %s", kind, ref)
	}
}

// 以相同內容重送（同一個 request_id）：不呼叫 social、回同一篇貼文與同一列稽核。
func TestReplayWithTheSamePayloadDoesNotCallSocialAgain(t *testing.T) {
	f := build(t, true)
	ctx := context.Background()
	o := f.newOperator(t, "replay2")
	sid := o.session.Session.ID
	op := f.op("重送")
	in := DelegatedCreatePostInput{Op: op, Body: "只該有一篇", ImageToken: "tok-fixed"}

	first, err := f.m.DelegatedCreatePost(ctx, f.caller(t, o), in)
	if err != nil {
		t.Fatal(err)
	}
	completed := len(f.completed)
	again, err := f.m.DelegatedCreatePost(ctx, f.caller(t, o), in)
	if err != nil {
		t.Fatal(err)
	}
	if !again.Replayed || again.Post.ID != first.Post.ID || again.AuditLogID != first.AuditLogID {
		t.Fatalf("重送 = %+v，預期同一篇、同一列稽核", again)
	}
	if n := f.postCount(t, o.storeID); n != 1 {
		t.Fatalf("貼文 %d 篇，預期 1（social 不能被寫第二次）", n)
	}
	if n := f.auditRowsOfKey(t, sid, op.RequestID.String()); n != 1 {
		t.Fatalf("稽核 %d 列，預期 1", n)
	}
	if len(f.completed) != completed {
		t.Fatalf("重送不該再通知賣家：%d -> %d", completed, len(f.completed))
	}
	// 重送回的是貼文此刻的樣子：第一次之後貼文被改過，重送看到新的版本。
	body := "後來改的"
	if _, err := f.m.DelegatedUpdatePost(ctx, f.caller(t, o), DelegatedUpdatePostInput{Op: f.op("改"), Update: social.UpdatePostInput{PostID: first.Post.ID, Version: first.Post.Version, Body: &body}}); err != nil {
		t.Fatal(err)
	}
	later, err := f.m.DelegatedCreatePost(ctx, f.caller(t, o), in)
	if err != nil || !later.Replayed || later.Post.Body != body {
		t.Fatalf("重送要回目前的樣子：%+v %v", later.Post.Body, err)
	}
	// 同一把 request_id、不同內容不是重送。
	if _, err := f.m.DelegatedCreatePost(ctx, f.caller(t, o), DelegatedCreatePostInput{Op: op, Body: "別的內容", ImageToken: "tok-fixed"}); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("同一個 request_id、不同內容：%v，預期 failed_precondition", err)
	}
	if n := f.postCount(t, o.storeID); n != 1 {
		t.Fatalf("貼文 %d 篇，預期 1", n)
	}
}

// social 的寫入失敗時，稽核列跟著 rollback（同一個 transaction）；失敗的重送之後仍然可以用同一個 request_id 成功。
func TestSocialFailureLeavesNoAuditRow(t *testing.T) {
	f := build(t, true)
	ctx := context.Background()
	o := f.newOperator(t, "rollback")
	sid := o.session.Session.ID
	p := f.draftPost(t, o.storeID)

	op := f.op("版本過期的發布")
	_, err := f.m.DelegatedPublishPost(ctx, f.caller(t, o), DelegatedPostTransitionInput{Op: op, PostID: p.ID, Version: p.Version + 100})
	if err == nil {
		t.Fatal("過期的版本要失敗")
	}
	if n := f.auditRowsOfKey(t, sid, op.RequestID.String()); n != 0 {
		t.Fatalf("social 失敗後稽核 %d 列，預期 0", n)
	}
	if n := f.auditRows(t, sid, "POST_PUBLISH"); n != 0 {
		t.Fatalf("POST_PUBLISH 稽核 %d 列，預期 0", n)
	}
	if got := f.postStatus(t, p.ID); got != "DRAFT" {
		t.Fatalf("貼文 = %s", got)
	}
	// 同一個 request_id 以正確的版本重試成功，且只有一列稽核。
	if _, err := f.m.DelegatedPublishPost(ctx, f.caller(t, o), DelegatedPostTransitionInput{Op: op, PostID: p.ID, Version: p.Version}); err != nil {
		t.Fatal(err)
	}
	if n := f.auditRowsOfKey(t, sid, op.RequestID.String()); n != 1 {
		t.Fatalf("成功後稽核 %d 列，預期 1", n)
	}
}

// 已撤銷、已結束、已到期的 session 寫不進去：use case 擋下，沒有貼文、沒有稽核。
func TestEndedOrRevokedSessionCannotWrite(t *testing.T) {
	f := build(t, true)
	ctx := context.Background()
	o := f.newOperator(t, "ended")
	sid := o.session.Session.ID
	live := f.caller(t, o)

	// 傳輸層解析之後才被撤銷：用過期的快照（ACTIVE）呼叫，稽核的 session 檢查擋下。
	if _, err := f.m.RevokeDelegation(ctx, RevokeDelegationInput{SessionID: sid, AdminID: o.admin, Reason: "測試撤銷"}); err != nil {
		t.Fatal(err)
	}
	op := f.op("撤銷之後")
	_, err := f.m.DelegatedCreatePost(ctx, live, DelegatedCreatePostInput{Op: op, Body: "x", ImageToken: "tok-x"})
	if d, ok := DelegationDenialOf(err); !ok || d.Denial != DenialSessionRevoked {
		t.Fatalf("撤銷後（舊快照）：%v，預期 DenialSessionRevoked", err)
	}
	// 重新解析得到的快照是 REVOKED：use case 在進交易之前就擋下。
	fresh := f.caller(t, o)
	if fresh.Session.Status != DelegationRevoked {
		t.Fatalf("status = %s", fresh.Session.Status)
	}
	if _, err := f.m.DelegatedCreatePost(ctx, fresh, DelegatedCreatePostInput{Op: f.op("again"), Body: "x", ImageToken: "tok-y"}); err == nil {
		t.Fatal("REVOKED 的 session 不能寫")
	}
	if n := f.postCount(t, o.storeID); n != 0 {
		t.Fatalf("貼文 %d 篇，預期 0", n)
	}
	if n := f.count(t, `SELECT count(*) FROM delegation.admin_operation_audit_logs WHERE session_id = $1 AND action = 'POST_CREATE'`, sid); n != 0 {
		t.Fatalf("稽核 %d 列，預期 0", n)
	}

	// 結束的 session（到期）：以到期的快照呼叫。
	o2 := f.newOperator(t, "ended2")
	if err := f.m.EndDelegation(ctx, o2.session.Credential); err != nil {
		t.Fatal(err)
	}
	f.now = f.now.Add(time.Second)
	exp := f.caller(t, o2)
	if exp.Session.Status != DelegationExpired {
		t.Fatalf("status = %s", exp.Session.Status)
	}
	if _, err := f.m.DelegatedCreatePost(ctx, exp, DelegatedCreatePostInput{Op: f.op("結束後"), Body: "x", ImageToken: "tok-z"}); err == nil {
		t.Fatal("已結束的 session 不能寫")
	}
	if n := f.postCount(t, o2.storeID); n != 0 {
		t.Fatalf("貼文 %d 篇，預期 0", n)
	}
}

// 缺能力被 use case 擋下（傳輸層之外的第二道）：沒有 posts.write 不能建立與修改，沒有 posts.publish 不能發布、下架與排程，沒有 posts.read 不能讀。
func TestMissingCapabilityIsRejectedByTheUseCase(t *testing.T) {
	f := build(t, true)
	ctx := context.Background()
	o := f.newOperator(t, "readonly", CapabilityPostsRead)
	c := f.caller(t, o)
	p := f.draftPost(t, o.storeID)
	deny := func(name string, err error) {
		t.Helper()
		if d, ok := DelegationDenialOf(err); !ok || d.Denial != DenialCapabilityDenied {
			t.Errorf("%s：%v，預期 DenialCapabilityDenied", name, err)
		}
	}
	_, err := f.m.DelegatedCreatePost(ctx, c, DelegatedCreatePostInput{Op: f.op("r"), Body: "x"})
	deny("建立", err)
	body := "x"
	_, err = f.m.DelegatedUpdatePost(ctx, c, DelegatedUpdatePostInput{Op: f.op("r"), Update: social.UpdatePostInput{PostID: p.ID, Version: p.Version, Body: &body}})
	deny("修改", err)
	_, err = f.m.DelegatedPublishPost(ctx, c, DelegatedPostTransitionInput{Op: f.op("r"), PostID: p.ID, Version: p.Version})
	deny("發布", err)
	_, err = f.m.DelegatedUnpublishPost(ctx, c, DelegatedPostTransitionInput{Op: f.op("r"), PostID: p.ID, Version: p.Version})
	deny("下架", err)
	_, err = f.m.DelegatedSchedulePost(ctx, c, DelegatedSchedulePostInput{Op: f.op("r"), PostID: p.ID, Version: p.Version, PublishAt: f.now.Add(time.Hour)})
	deny("排程", err)
	if n := f.count(t, `SELECT count(*) FROM delegation.admin_operation_audit_logs WHERE session_id = $1 AND action LIKE 'POST_%'`, o.session.Session.ID); n != 0 {
		t.Fatalf("被擋下的寫入留下 %d 列稽核", n)
	}
	// 讀取有 posts.read：能讀。
	if _, err := f.m.DelegatedPost(ctx, c, p.ID); err != nil {
		t.Fatal(err)
	}
	page, err := f.m.DelegatedPosts(ctx, c, "", nil, 20)
	if err != nil || len(page.Page.Posts) != 1 {
		t.Fatalf("清單 = %+v %v", page.Page.Posts, err)
	}
	// 沒有 posts.read 的 session 讀不了。
	w := f.newOperator(t, "writeonly", CapabilityPostsWrite)
	_, err = f.m.DelegatedPost(ctx, f.caller(t, w), p.ID)
	deny("讀取", err)
}

// 代行 session 只碰自己的目標店家：別家的貼文與不存在一樣 NotFound；操作理由不合法被擋下且不留稽核。
func TestDelegatedReadsStayInTheTargetStoreAndTheReasonIsValidated(t *testing.T) {
	f := build(t, true)
	ctx := context.Background()
	o := f.newOperator(t, "scope")
	otherStore := f.exposableStore(t)
	other := f.draftPost(t, otherStore)
	if _, err := f.m.DelegatedPost(ctx, f.caller(t, o), other.ID); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("別家的貼文：%v，預期 not_found", err)
	}
	for name, reason := range map[string]string{"空白": "   ", "超過 100 字": string(make([]rune, 101)), "控制字元": "a\x07b"} {
		if name == "超過 100 字" {
			reason = ""
			for range 101 {
				reason += "字"
			}
		}
		_, err := f.m.DelegatedCreatePost(ctx, f.caller(t, o), DelegatedCreatePostInput{Op: DelegatedOperation{RequestID: uuid.New(), Reason: reason}, Body: "x"})
		if d, ok := DelegationDenialOf(err); !ok || d.Denial != DenialInvalidCommand {
			t.Errorf("操作原因 %s：%v，預期 DenialInvalidCommand", name, err)
		}
	}
	if n := f.count(t, `SELECT count(*) FROM delegation.admin_operation_audit_logs WHERE session_id = $1 AND action LIKE 'POST_%'`, o.session.Session.ID); n != 0 {
		t.Fatalf("不合法的請求留下 %d 列稽核", n)
	}
}

// 排程：POST_SCHEDULE 的稽核先寫，到點發布時 POST_PUBLISH（SCHEDULED）繼承它的理由；model 的常數與 DB 的 CHECK 一致。
func TestScheduleWritesPostScheduleAuditAndThePublishInheritsItsReason(t *testing.T) {
	f := build(t, true)
	ctx := context.Background()
	o := f.newOperator(t, "sched")
	p := f.draftPost(t, o.storeID)
	op := f.op("先排好")
	s, err := f.m.DelegatedSchedulePost(ctx, f.caller(t, o), DelegatedSchedulePostInput{Op: op, PostID: p.ID, Version: p.Version, PublishAt: f.now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if n := f.auditRows(t, o.session.Session.ID, string(model.AuditPostSchedule)); n != 1 {
		t.Fatalf("POST_SCHEDULE 稽核 %d 列", n)
	}
	var sched social.Schedule
	page, err := f.realSocial.ListPendingSchedules(ctx, o.storeID, nil, 10)
	if err != nil || len(page.Schedules) != 1 {
		t.Fatalf("待發布的排程 %v %v", page.Schedules, err)
	}
	sched = page.Schedules[0]
	if sched.By.Kind != social.ScheduleByDelegation || sched.By.Ref != fmt.Sprint(o.session.Session.ID) || sched.PostID != s.Post.ID {
		t.Fatalf("排程 = %+v", sched)
	}
	f.publishDue(t, sched)
	var reason *string
	if err := f.db.Pool().QueryRow(ctx, `SELECT operation_reason FROM delegation.admin_operation_audit_logs WHERE session_id = $1 AND idempotency_key = $2`,
		o.session.Session.ID, "schedule:"+sched.ID.String()).Scan(&reason); err != nil {
		t.Fatal(err)
	}
	if reason == nil || *reason != "先排好" {
		t.Fatalf("到點發布的稽核理由 = %v", reason)
	}
}
