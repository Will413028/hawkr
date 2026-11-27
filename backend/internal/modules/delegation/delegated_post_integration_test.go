//go:build integration

package delegation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/delegation/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/delegatedactor"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)


var (
	methodRead      = DelegatedMethod{Capability: CapabilityPostsRead, Action: AuditPostRead, Resource: ResourcePost}
	methodCreate    = DelegatedMethod{Capability: CapabilityPostsWrite, Action: AuditPostCreate, Resource: ResourcePost}
	methodUpdate    = DelegatedMethod{Capability: CapabilityPostsWrite, Action: AuditPostUpdate, Resource: ResourcePost}
	methodPublish   = DelegatedMethod{Capability: CapabilityPostsPublish, Action: AuditPostPublish, Resource: ResourcePost}
	methodUnpublish = DelegatedMethod{Capability: CapabilityPostsPublish, Action: AuditPostUnpublish, Resource: ResourcePost}
	methodSchedule  = DelegatedMethod{Capability: CapabilityPostsPublish, Action: AuditPostSchedule, Resource: ResourcePost}
)

type operator struct {
	storeID int64
	admin   int64
	session issuedSession
}

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

func (f *fixture) caller(t *testing.T, o operator, m DelegatedMethod) DelegatedCaller {
	t.Helper()
	s, err := f.m.ResolveDelegation(context.Background(), o.session.Credential, true)
	if err != nil {
		t.Fatal(err)
	}
	return DelegatedCaller{Session: s, Method: m, CorrelationID: "corr-it", UserAgent: "integration-test"}
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

func (f *fixture) resultRows(t *testing.T, sessionID int64, result string) int {
	t.Helper()
	return f.count(t, `SELECT count(*) FROM delegation.admin_operation_audit_logs WHERE session_id = $1 AND result = $2`, sessionID, result)
}

func (f *fixture) postCount(t *testing.T, storeID int64) int {
	t.Helper()
	return f.count(t, `SELECT count(*) FROM social.posts WHERE store_id = $1`, storeID)
}

type failedRow struct {
	failureReason              *string
	reason, key                *string
	requestID, capability, cor string
	notify                     bool
	n                          int
}

func (f *fixture) failedRow(t *testing.T, sessionID int64, action string) failedRow {
	t.Helper()
	var r failedRow
	r.n = f.count(t, `SELECT count(*) FROM delegation.admin_operation_audit_logs WHERE session_id = $1 AND action = $2 AND result = 'FAILED'`, sessionID, action)
	if r.n == 0 {
		return r
	}
	if err := f.db.Pool().QueryRow(context.Background(), `SELECT failure_reason, operation_reason, idempotency_key, request_id, capability, correlation_id, notify_seller
		FROM delegation.admin_operation_audit_logs WHERE session_id = $1 AND action = $2 AND result = 'FAILED' ORDER BY created_at DESC, id DESC LIMIT 1`, sessionID, action).
		Scan(&r.failureReason, &r.reason, &r.key, &r.requestID, &r.capability, &r.cor, &r.notify); err != nil {
		t.Fatal(err)
	}
	return r
}

func wantFailureReason(t *testing.T, what string, got *string, want string) {
	t.Helper()
	if got == nil || *got != want {
		t.Fatalf("%s 的 failure_reason = %v，預期 %s", what, got, want)
	}
}

type written struct {
	Post      social.Post
	Res       DelegatedWriteResult
	PublishAt *time.Time
}

func (f *fixture) writePost(c DelegatedCaller, w DelegatedWrite, do func(ctx context.Context, st seller.Store, actor *delegatedactor.Actor) (social.Post, *time.Time, error)) (written, error) {
	ctx := context.Background()
	var out written
	res, err := f.m.DelegatedWrite(ctx, c, w, func(ctx context.Context, st seller.Store, actor *delegatedactor.Actor) error {
		p, at, err := do(ctx, st, actor)
		if err != nil {
			return err
		}
		out.Post, out.PublishAt = p, at
		return nil
	})
	if err != nil {
		return written{}, err
	}
	out.Res = res
	if res.Replayed {
		id, err := uuid.Parse(res.ResourceID)
		if err != nil {
			return written{}, err
		}
		if out.Post, err = f.realSocial.MyPost(ctx, res.Store, id); err != nil {
			return written{}, err
		}
	}
	return out, nil
}

func (f *fixture) createPost(c DelegatedCaller, op DelegatedOperation, body, token string) (written, error) {
	return f.writePost(c, DelegatedWrite{Op: op, PayloadHash: DelegatedPayloadHash(AuditPostCreate, body, token)},
		func(ctx context.Context, st seller.Store, actor *delegatedactor.Actor) (social.Post, *time.Time, error) {
			p, err := f.realSocial.CreatePost(ctx, st, social.CreatePostInput{Body: body, ImageToken: token}, actor)
			return p, nil, err
		})
}

func (f *fixture) mustCreate(t *testing.T, o operator, op DelegatedOperation, body string) written {
	t.Helper()
	r, err := f.createPost(f.caller(t, o, methodCreate), op, body, fmt.Sprintf("tok-%d", f.next()))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func (f *fixture) updatePost(c DelegatedCaller, op DelegatedOperation, id uuid.UUID, version int64, body string) (written, error) {
	return f.writePost(c, DelegatedWrite{Op: op, ResourceID: id.String(), Version: &version, PayloadHash: DelegatedPayloadHash(AuditPostUpdate, id, version, body)},
		func(ctx context.Context, st seller.Store, actor *delegatedactor.Actor) (social.Post, *time.Time, error) {
			p, err := f.realSocial.UpdatePost(ctx, st, social.UpdatePostInput{PostID: id, Version: version, Body: &body}, actor)
			return p, nil, err
		})
}

func (f *fixture) transition(c DelegatedCaller, op DelegatedOperation, id uuid.UUID, version int64) (written, error) {
	action := c.Method.Action
	return f.writePost(c, DelegatedWrite{Op: op, ResourceID: id.String(), Version: &version, PayloadHash: DelegatedPayloadHash(action, id, version)},
		func(ctx context.Context, st seller.Store, actor *delegatedactor.Actor) (social.Post, *time.Time, error) {
			var p social.Post
			var err error
			if action == AuditPostPublish {
				p, err = f.realSocial.PublishPost(ctx, st, id, version, actor)
			} else {
				p, err = f.realSocial.UnpublishPost(ctx, st, id, version, actor)
			}
			return p, nil, err
		})
}

func (f *fixture) schedulePost(c DelegatedCaller, op DelegatedOperation, id uuid.UUID, version int64, at time.Time) (written, error) {
	by := social.ScheduledBy{Kind: social.ScheduleByDelegation, Ref: fmt.Sprint(c.Session.ID)}
	return f.writePost(c, DelegatedWrite{Op: op, ResourceID: id.String(), Version: &version, PayloadHash: DelegatedPayloadHash(AuditPostSchedule, id, version, at.UTC())},
		func(ctx context.Context, st seller.Store, actor *delegatedactor.Actor) (social.Post, *time.Time, error) {
			s, err := f.realSocial.SchedulePost(ctx, st, social.SchedulePostInput{PostID: id, Version: version, PublishAt: at, By: by}, actor)
			sat := s.Schedule.PublishAt
			return s.Post, &sat, err
		})
}

func (f *fixture) readPost(c DelegatedCaller, id uuid.UUID) (social.Post, error) {
	var p social.Post
	err := f.m.DelegatedRead(context.Background(), c, func(ctx context.Context, st seller.Store) error {
		var err error
		p, err = f.realSocial.MyPost(ctx, st, id)
		return err
	})
	return p, err
}

// 每個寫入都留下恰好一列 SUCCEEDED 的稽核：動作、能力、資源、版本、操作理由、冪等鍵與 request_id 都對。
func TestEveryDelegatedPostWriteLeavesOneSucceededAuditRow(t *testing.T) {
	f := build(t, true)
	ctx := context.Background()
	o := f.newOperator(t, "writer")
	sid := o.session.Session.ID

	createOp := f.op("店家請我們代為建立")
	created := f.mustCreate(t, o, createOp, "代行建立的貼文")
	if created.Res.Replayed || created.Post.Status != social.StatusDraft {
		t.Fatalf("created = %+v", created)
	}
	updOp := f.op("修改錯字")
	updated, err := f.updatePost(f.caller(t, o, methodUpdate), updOp, created.Post.ID, created.Post.Version, "改過的貼文")
	if err != nil {
		t.Fatal(err)
	}
	pubOp := f.op("店家要求上架")
	published, err := f.transition(f.caller(t, o, methodPublish), pubOp, created.Post.ID, updated.Post.Version)
	if err != nil {
		t.Fatal(err)
	}
	if published.Post.Status != social.StatusPublished {
		t.Fatalf("published = %+v", published.Post)
	}
	unOp := f.op("內容有爭議先下架")
	unpublished, err := f.transition(f.caller(t, o, methodUnpublish), unOp, created.Post.ID, published.Post.Version)
	if err != nil {
		t.Fatal(err)
	}
	schedOp := f.op("改排在週末")
	at := f.now.Add(2 * time.Hour)
	scheduled, err := f.schedulePost(f.caller(t, o, methodSchedule), schedOp, created.Post.ID, unpublished.Post.Version, at)
	if err != nil {
		t.Fatal(err)
	}
	if scheduled.PublishAt == nil || !scheduled.PublishAt.Equal(at) {
		t.Fatalf("排定時間 = %v", scheduled.PublishAt)
	}

	want := []struct {
		action, capability, reason string
		op                         DelegatedOperation
		result                     written
	}{
		{"POST_CREATE", "posts.write", "店家請我們代為建立", createOp, created},
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
			reason == nil || *reason != w.reason || !notify || !confirmed || id != w.result.Res.AuditLogID || after == nil || *after != w.result.Post.Version {
			t.Fatalf("%s 的稽核列 = %s %s %s %s %s reason=%v notify=%v confirmed=%v id=%s after=%v", w.action, result, origin, capability, resourceType, resourceID, reason, notify, confirmed, id, after)
		}
		if key != w.op.RequestID.String() || requestID != w.op.RequestID.String() {
			t.Fatalf("%s 的冪等鍵 %q、request_id %q，預期都是 %s", w.action, key, requestID, w.op.RequestID)
		}
	}
	if n := f.resultRows(t, sid, "FAILED"); n != 0 {
		t.Fatalf("成功的寫入留下 %d 列 FAILED", n)
	}
	if len(f.completed) != 5 {
		t.Fatalf("完成通知 %d 則，預期 5", len(f.completed))
	}
	var kind, ref string
	if err := f.db.Pool().QueryRow(ctx, `SELECT scheduled_by_kind, scheduled_by_ref FROM social.post_schedules WHERE post_id = $1 AND state = 'PENDING'`, scheduled.Post.ID).Scan(&kind, &ref); err != nil {
		t.Fatal(err)
	}
	if kind != "DELEGATION" || ref != fmt.Sprint(sid) {
		t.Fatalf("排程的發起者 = %s %s", kind, ref)
	}
}

// 以相同內容重送（同一個 request_id）：不呼叫 social、回同一篇貼文與同一列稽核，不再通知賣家，也不寫 FAILED。
func TestReplayWithTheSamePayloadDoesNotCallSocialAgain(t *testing.T) {
	f := build(t, true)
	o := f.newOperator(t, "replay2")
	sid := o.session.Session.ID
	op := f.op("重送")
	c := f.caller(t, o, methodCreate)

	first, err := f.createPost(c, op, "只該有一篇", "tok-fixed")
	if err != nil {
		t.Fatal(err)
	}
	completed := len(f.completed)
	again, err := f.createPost(c, op, "只該有一篇", "tok-fixed")
	if err != nil {
		t.Fatal(err)
	}
	if !again.Res.Replayed || again.Post.ID != first.Post.ID || again.Res.AuditLogID != first.Res.AuditLogID {
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
	if n := f.resultRows(t, sid, "FAILED"); n != 0 {
		t.Fatalf("重送留下 %d 列 FAILED", n)
	}
	if _, err := f.updatePost(f.caller(t, o, methodUpdate), f.op("改"), first.Post.ID, first.Post.Version, "後來改的"); err != nil {
		t.Fatal(err)
	}
	later, err := f.createPost(c, op, "只該有一篇", "tok-fixed")
	if err != nil || !later.Res.Replayed || later.Post.Body != "後來改的" {
		t.Fatalf("重送要回目前的樣子：%+v %v", later.Post.Body, err)
	}
	_, err = f.createPost(c, op, "別的內容", "tok-fixed")
	if apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("同一個 request_id、不同內容：%v，預期 failed_precondition", err)
	}
	if n := f.postCount(t, o.storeID); n != 1 {
		t.Fatalf("貼文 %d 篇，預期 1", n)
	}
	row := f.failedRow(t, sid, "POST_CREATE")
	if row.n != 1 {
		t.Fatalf("FAILED %d 列，預期 1", row.n)
	}
	wantFailureReason(t, "request_id 用在不同內容", row.failureReason, "IDEMPOTENCY_KEY_REUSED")
	if n := f.auditRowsOfKey(t, sid, op.RequestID.String()); n != 1 {
		t.Fatalf("冪等鍵 %s 的列 %d，預期仍是第一次的那一列", op.RequestID, n)
	}
}

// 並行的重送（同一個 session、同一個 request_id、內容相同）：兩個請求都成功，只有一篇貼文、一列 SUCCEEDED 稽核，沒有 FAILED；
// 撞唯一鍵的那一個由 delegation 讀回稽核列收斂成重送，或由 social 自己的 request_id 去重（A7-D26）。
func TestConcurrentReplaysConvergeOnOneWriteAndOneAuditRow(t *testing.T) {
	f := build(t, true)
	o := f.newOperator(t, "concurrent")
	sid := o.session.Session.ID
	op := f.op("並行重送")
	c := f.caller(t, o, methodCreate)
	const n = 4
	errs := make(chan error, n)
	for range n {
		go func() {
			_, err := f.createPost(c, op, "並行的一篇", "")
			errs <- err
		}()
	}
	for range n {
		if err := <-errs; err != nil {
			t.Fatalf("並行重送：%v", err)
		}
	}
	if got := f.postCount(t, o.storeID); got != 1 {
		t.Fatalf("貼文 %d 篇，預期 1", got)
	}
	if got := f.auditRowsOfKey(t, sid, op.RequestID.String()); got != 1 {
		t.Fatalf("稽核 %d 列，預期 1", got)
	}
	if got := f.resultRows(t, sid, "FAILED"); got != 0 {
		t.Fatalf("FAILED %d 列，預期 0", got)
	}
}

// social 的寫入失敗時，SUCCEEDED 稽核跟著 rollback（同一個 transaction），但另開 transaction 留下一列 FAILED（failure_reason 是 social 回的 ErrorReason VERSION_CONFLICT、沒有冪等鍵）；
// 失敗的嘗試不占用 request_id，之後用同一個 request_id 重試成功。
func TestSocialFailureLeavesNoSucceededRowButAFailedRow(t *testing.T) {
	f := build(t, true)
	o := f.newOperator(t, "rollback")
	sid := o.session.Session.ID
	p := f.draftPost(t, o.storeID)
	c := f.caller(t, o, methodPublish)

	op := f.op("版本過期的發布")
	stale := p.Version + 100
	_, err := f.transition(c, op, p.ID, stale)
	if err == nil {
		t.Fatal("過期的版本要失敗")
	}
	if n := f.resultRows(t, sid, "SUCCEEDED"); n != 0 {
		t.Fatalf("social 失敗後 SUCCEEDED %d 列，預期 0", n)
	}
	row := f.failedRow(t, sid, "POST_PUBLISH")
	if row.n != 1 || row.failureReason == nil || *row.failureReason != "VERSION_CONFLICT" || row.key != nil || row.requestID != op.RequestID.String() || row.capability != "posts.publish" || row.notify {
		t.Fatalf("FAILED 列 = %+v", row)
	}
	if row.reason == nil || *row.reason != "版本過期的發布" {
		t.Fatalf("FAILED 列的操作原因 = %v", row.reason)
	}
	if got := f.postStatus(t, p.ID); got != "DRAFT" {
		t.Fatalf("貼文 = %s", got)
	}
	if _, err := f.transition(c, op, p.ID, p.Version); err != nil {
		t.Fatal(err)
	}
	if n := f.auditRowsOfKey(t, sid, op.RequestID.String()); n != 1 {
		t.Fatalf("成功後冪等鍵的列 %d，預期 1", n)
	}
	if f.resultRows(t, sid, "SUCCEEDED") != 1 || f.resultRows(t, sid, "FAILED") != 1 {
		t.Fatalf("SUCCEEDED %d、FAILED %d，預期各 1", f.resultRows(t, sid, "SUCCEEDED"), f.resultRows(t, sid, "FAILED"))
	}
}

// 已撤銷、已結束的 session 寫不進去：沒有貼文、沒有 SUCCEEDED，被拒絕的嘗試留下 FAILED（撤銷 9271、到期 9270）。
func TestEndedOrRevokedSessionCannotWriteAndLeavesFailedRows(t *testing.T) {
	f := build(t, true)
	ctx := context.Background()
	o := f.newOperator(t, "ended")
	sid := o.session.Session.ID
	live := f.caller(t, o, methodCreate)

	if _, err := f.m.RevokeDelegation(ctx, RevokeDelegationInput{SessionID: sid, AdminID: o.admin, Reason: "測試撤銷"}); err != nil {
		t.Fatal(err)
	}
	_, err := f.createPost(live, f.op("撤銷之後"), "x", "tok-x")
	if d, ok := DelegationDenialOf(err); !ok || d.Denial != DenialSessionRevoked {
		t.Fatalf("撤銷後（舊快照）：%v，預期 DenialSessionRevoked", err)
	}
	fresh := f.caller(t, o, methodCreate)
	if fresh.Session.Status != DelegationRevoked {
		t.Fatalf("status = %s", fresh.Session.Status)
	}
	if _, err := f.createPost(fresh, f.op("again"), "x", "tok-y"); err == nil {
		t.Fatal("REVOKED 的 session 不能寫")
	}
	if n := f.postCount(t, o.storeID); n != 0 {
		t.Fatalf("貼文 %d 篇，預期 0", n)
	}
	if n := f.resultRows(t, sid, "SUCCEEDED"); n != 0 {
		t.Fatalf("SUCCEEDED %d 列，預期 0", n)
	}
	row := f.failedRow(t, sid, "POST_CREATE")
	if row.n != 2 {
		t.Fatalf("FAILED %d 列，預期 2", row.n)
	}
	wantFailureReason(t, "撤銷的 session", row.failureReason, "SESSION_REVOKED")

	o2 := f.newOperator(t, "ended2")
	if err := f.m.EndDelegation(ctx, o2.session.Credential); err != nil {
		t.Fatal(err)
	}
	f.now = f.now.Add(time.Second)
	exp := f.caller(t, o2, methodCreate)
	if exp.Session.Status != DelegationExpired {
		t.Fatalf("status = %s", exp.Session.Status)
	}
	if _, err := f.createPost(exp, f.op("結束後"), "x", "tok-z"); err == nil {
		t.Fatal("已結束的 session 不能寫")
	}
	if n := f.postCount(t, o2.storeID); n != 0 {
		t.Fatalf("貼文 %d 篇，預期 0", n)
	}
	row = f.failedRow(t, o2.session.Session.ID, "POST_CREATE")
	if row.n != 1 {
		t.Fatalf("FAILED %d 列，預期 1", row.n)
	}
	wantFailureReason(t, "結束的 session", row.failureReason, "SESSION_EXPIRED")
}

// 缺能力被元件擋下：沒有 posts.write 不能建立與修改，沒有 posts.publish 不能發布、下架與排程，沒有 posts.read 不能讀；每一次都留下 FAILED（9262），讀取也一樣。
func TestMissingCapabilityIsRejectedAndLeavesFailedRows(t *testing.T) {
	f := build(t, true)
	o := f.newOperator(t, "readonly", CapabilityPostsRead)
	sid := o.session.Session.ID
	p := f.draftPost(t, o.storeID)
	deny := func(name string, err error) {
		t.Helper()
		if d, ok := DelegationDenialOf(err); !ok || d.Denial != DenialCapabilityDenied {
			t.Errorf("%s：%v，預期 DenialCapabilityDenied", name, err)
		}
	}
	_, err := f.createPost(f.caller(t, o, methodCreate), f.op("r"), "x", "tok-1")
	deny("建立", err)
	_, err = f.updatePost(f.caller(t, o, methodUpdate), f.op("r"), p.ID, p.Version, "x")
	deny("修改", err)
	_, err = f.transition(f.caller(t, o, methodPublish), f.op("r"), p.ID, p.Version)
	deny("發布", err)
	_, err = f.transition(f.caller(t, o, methodUnpublish), f.op("r"), p.ID, p.Version)
	deny("下架", err)
	_, err = f.schedulePost(f.caller(t, o, methodSchedule), f.op("r"), p.ID, p.Version, f.now.Add(time.Hour))
	deny("排程", err)
	if n := f.resultRows(t, sid, "SUCCEEDED"); n != 0 {
		t.Fatalf("被擋下的寫入留下 %d 列 SUCCEEDED", n)
	}
	for _, a := range []string{"POST_CREATE", "POST_UPDATE", "POST_PUBLISH", "POST_UNPUBLISH", "POST_SCHEDULE"} {
		row := f.failedRow(t, sid, a)
		if row.n != 1 {
			t.Fatalf("%s 的 FAILED %d 列，預期 1", a, row.n)
		}
		wantFailureReason(t, a, row.failureReason, "CAPABILITY_DENIED")
	}
	if n := f.postCount(t, o.storeID); n != 1 {
		t.Fatalf("貼文 %d 篇，預期只有一開始的草稿", n)
	}
	if _, err := f.readPost(f.caller(t, o, methodRead), p.ID); err != nil {
		t.Fatal(err)
	}
	if n := f.auditRows(t, sid, "POST_READ"); n != 0 {
		t.Fatalf("成功的讀取留下 %d 列稽核", n)
	}
	w := f.newOperator(t, "writeonly", CapabilityPostsWrite)
	_, err = f.readPost(f.caller(t, w, methodRead), p.ID)
	deny("讀取", err)
	row := f.failedRow(t, w.session.Session.ID, "POST_READ")
	if row.n != 1 {
		t.Fatalf("被拒絕的讀取 FAILED %d 列，預期 1", row.n)
	}
	wantFailureReason(t, "被拒絕的讀取", row.failureReason, "CAPABILITY_DENIED")
	if row.notify {
		t.Fatal("讀取的 FAILED 列不通知賣家")
	}
}

// 店家此刻不可代行：寫入與讀取都只被拒絕、寫 FAILED，session 不被撤銷、不被結束（請求時發現只拒絕，不撤銷）。
func TestStoreNotDelegableAtRequestTimeRejectsWithoutRevoking(t *testing.T) {
	f := build(t, true)
	ctx := context.Background()
	o := f.newOperator(t, "closedstore")
	sid := o.session.Session.ID
	p := f.draftPost(t, o.storeID)
	f.storeFn = fixedFn(seller.Closed, false)

	_, err := f.createPost(f.caller(t, o, methodCreate), f.op("店家關了"), "x", "tok-closed")
	if d, ok := DelegationDenialOf(err); !ok || d.Denial != DenialStoreClosed {
		t.Fatalf("寫入：%v，預期 DenialStoreClosed", err)
	}
	_, err = f.readPost(f.caller(t, o, methodRead), p.ID)
	if d, ok := DelegationDenialOf(err); !ok || d.Denial != DenialStoreClosed {
		t.Fatalf("讀取：%v，預期 DenialStoreClosed", err)
	}
	for _, a := range []string{"POST_CREATE", "POST_READ"} {
		row := f.failedRow(t, sid, a)
		if row.n != 1 {
			t.Fatalf("%s 的 FAILED %d 列，預期 1", a, row.n)
		}
		wantFailureReason(t, a, row.failureReason, "STORE_CLOSED")
	}
	if n := f.resultRows(t, sid, "SUCCEEDED"); n != 0 {
		t.Fatalf("SUCCEEDED %d 列，預期 0", n)
	}
	s, err := f.m.ResolveDelegation(ctx, o.session.Credential, false)
	if err != nil || s.Status != DelegationActive || s.RevokedAt != nil {
		t.Fatalf("請求時發現店家不可代行只拒絕，不撤銷：%+v %v", s, err)
	}
}

// 代行 session 只碰自己的目標店家：別家的貼文與不存在一樣 NotFound（讀取的業務失敗不稽核）；操作理由不合法被擋下，FAILED 列的操作原因是 NULL。
func TestDelegatedReadsStayInTheTargetStoreAndTheReasonIsValidated(t *testing.T) {
	f := build(t, true)
	o := f.newOperator(t, "scope")
	sid := o.session.Session.ID
	other := f.draftPost(t, f.exposableStore(t))
	if _, err := f.readPost(f.caller(t, o, methodRead), other.ID); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("別家的貼文：%v，預期 not_found", err)
	}
	if n := f.auditRows(t, sid, "POST_READ"); n != 0 {
		t.Fatalf("讀取的業務失敗留下 %d 列稽核", n)
	}
	reasons := map[string]string{"空白": "   ", "超過 100 字": strings.Repeat("字", 101), "控制字元": "a\x07b"}
	for name, reason := range reasons {
		_, err := f.createPost(f.caller(t, o, methodCreate), DelegatedOperation{RequestID: uuid.New(), Reason: reason}, "x", "tok-r")
		if d, ok := DelegationDenialOf(err); !ok || d.Denial != DenialInvalidCommand {
			t.Errorf("操作原因 %s：%v，預期 DenialInvalidCommand", name, err)
		}
	}
	if n := f.resultRows(t, sid, "SUCCEEDED"); n != 0 {
		t.Fatalf("不合法的請求留下 %d 列 SUCCEEDED", n)
	}
	if n := f.count(t, `SELECT count(*) FROM delegation.admin_operation_audit_logs WHERE session_id = $1 AND result = 'FAILED' AND action = 'POST_CREATE' AND failure_reason = $2 AND biz_code IS NULL`, sid, DenialCodes()[DenialInvalidCommand]); n != len(reasons) {
		t.Fatalf("FAILED（%s）%d 列，預期 %d", DenialCodes()[DenialInvalidCommand], n, len(reasons))
	}
	if n := f.count(t, `SELECT count(*) FROM delegation.admin_operation_audit_logs WHERE session_id = $1 AND result = 'FAILED' AND operation_reason IS NOT NULL AND (char_length(operation_reason) NOT BETWEEN 1 AND 100 OR operation_reason ~ '[[:cntrl:]]')`, sid); n != 0 {
		t.Fatalf("不合法的操作原因被寫進 FAILED 列 %d 列", n)
	}
}

// 排程：POST_SCHEDULE 的稽核先寫，到點發布時 POST_PUBLISH（SCHEDULED）繼承它的理由；model 的常數與 DB 的 CHECK 一致。
func TestScheduleWritesPostScheduleAuditAndThePublishInheritsItsReason(t *testing.T) {
	f := build(t, true)
	ctx := context.Background()
	o := f.newOperator(t, "sched")
	p := f.draftPost(t, o.storeID)
	op := f.op("先排好")
	s, err := f.schedulePost(f.caller(t, o, methodSchedule), op, p.ID, p.Version, f.now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if n := f.auditRows(t, o.session.Session.ID, string(model.AuditPostSchedule)); n != 1 {
		t.Fatalf("POST_SCHEDULE 稽核 %d 列", n)
	}
	page, err := f.realSocial.ListPendingSchedules(ctx, o.storeID, nil, 10)
	if err != nil || len(page.Schedules) != 1 {
		t.Fatalf("待發布的排程 %v %v", page.Schedules, err)
	}
	sched := page.Schedules[0]
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

// 第五個觸發點：seller 的 StoreSuspended、StoreClosed 在同一個 transaction 內結束該店進行中的 session（結束，不是撤銷），別家店的 session 不受影響。
func TestStoreSuspendedAndClosedEndThatStoresSessions(t *testing.T) {
	for name, event := range map[string]func(storeID int64) any{
		"StoreSuspended": func(id int64) any { return seller.StoreSuspended{StoreID: id, AccountID: id + 100, PeriodID: 1} },
		"StoreClosed":    func(id int64) any { return seller.StoreClosed{StoreID: id, StoreCode: id + 200, AccountID: id + 100} },
	} {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			ctx := context.Background()
			admin := f.delegationAdmin(t, "ops@hawkr.example", true)
			other := f.delegationAdmin(t, "other@hawkr.example", true)
			f.grant(t, admin, testStore)
			f.grant(t, other, testStore+1)
			mine := f.start(t, startInput(admin, testStore, "ik-1"))
			theirs := f.start(t, startInput(other, testStore+1, "ik-2"))

			f.now = f.now.Add(time.Minute)
			if err := f.db.InTx(ctx, func(ctx context.Context) error {
				switch e := event(testStore).(type) {
				case seller.StoreSuspended:
					return events.Publish(ctx, f.bus, e)
				case seller.StoreClosed:
					return events.Publish(ctx, f.bus, e)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			got, err := f.m.ResolveDelegation(ctx, mine.Credential, true)
			if err != nil || got.Status != DelegationExpired {
				t.Fatalf("該店的 session 被結束（到期，不是進行中）：%+v %v", got, err)
			}
			s, err := f.m.DelegationSessionSummary(ctx, mine.Session.ID)
			if err != nil || s.Session.EndedAt == nil || s.Session.RevokedAt != nil || s.Session.RevokeReason != nil {
				t.Fatalf("是結束（有 ended_at），不是撤銷：%+v %v", s.Session, err)
			}
			if !s.Session.ExpiresAt.Equal(f.now) {
				t.Fatalf("expires_at 被拉回現在（截秒）：%v，現在 %v", s.Session.ExpiresAt, f.now)
			}
			if o, _ := f.m.ResolveDelegation(ctx, theirs.Credential, false); o.Status != DelegationActive {
				t.Fatalf("別家店的 session 不受影響：%+v", o)
			}
			again := f.start(t, startInput(admin, testStore, "ik-3"))
			boom := fmt.Errorf("boom")
			if err := f.db.InTx(ctx, func(ctx context.Context) error {
				if err := f.m.OnStoreSuspended(ctx, seller.StoreSuspended{StoreID: testStore}); err != nil {
					return err
				}
				return boom
			}); !errors.Is(err, boom) {
				t.Fatalf("err = %v", err)
			}
			if o, _ := f.m.ResolveDelegation(ctx, again.Credential, false); o.Status != DelegationActive {
				t.Fatalf("transaction 回滾，session 仍進行中：%+v", o)
			}
		})
	}
}

// 沒有 ErrorReason 的失敗（內部錯誤）寫明確的 INTERNAL，不是空值。
func TestReasonlessFailureRecordsInternal(t *testing.T) {
	f := build(t, true)
	o := f.newOperator(t, "reasonless")
	sid := o.session.Session.ID
	c := f.caller(t, o, methodCreate)
	op := f.op("內部錯誤")
	_, err := f.writePost(c, DelegatedWrite{Op: op, PayloadHash: DelegatedPayloadHash(AuditPostCreate, "x", "tok-internal")},
		func(context.Context, seller.Store, *delegatedactor.Actor) (social.Post, *time.Time, error) {
			return social.Post{}, nil, errors.New("boom")
		})
	if err == nil {
		t.Fatal("要失敗")
	}
	row := f.failedRow(t, sid, "POST_CREATE")
	if row.n != 1 {
		t.Fatalf("FAILED %d 列，預期 1", row.n)
	}
	wantFailureReason(t, "沒有原因的錯誤", row.failureReason, FailureReasonInternal)
}

// failure_reason 的 CHECK：FAILED 一定有值、值必須是 UPPER_SNAKE 代碼（不是 UNSPECIFIED）、非 FAILED 不得有值。
func TestFailureReasonCheckRejectsUnknownCodes(t *testing.T) {
	f := build(t, true)
	o := f.newOperator(t, "check")
	insert := func(result string, reason any) error {
		_, err := f.db.Pool().Exec(context.Background(), `INSERT INTO delegation.admin_operation_audit_logs
			(id, session_id, actor_admin_id, actor_display_name, target_store_id, target_seller_account_id, target_store_name,
			 origin, capability, action, resource_type, result, request_id, correlation_id, risk_level, notify_seller,
			 created_at, confirmed_at, failure_reason)
			VALUES ($1, $2, 1, 'x', $3, 0, 'x', 'INTERACTIVE', 'posts.write', 'POST_CREATE', 'POST', $4, 'r', 'c', 'LOW', false,
			        $5, CASE WHEN $4 = 'PENDING' THEN NULL ELSE $5::timestamptz END, $6)`,
			uuid.NewV7(), o.session.Session.ID, o.storeID, result, f.now, reason)
		return err
	}
	for name, tc := range map[string]struct {
		result string
		reason any
	}{
		"failed without reason": {"FAILED", nil},
		"lowercase":             {"FAILED", "version_conflict"},
		"free text":             {"FAILED", "bad code"},
		"unspecified":           {"FAILED", "UNSPECIFIED"},
		"succeeded with reason": {"SUCCEEDED", "INTERNAL"},
	} {
		if err := insert(tc.result, tc.reason); err == nil || !strings.Contains(err.Error(), "admin_operation_audit_logs_failure_reason_check") {
			t.Errorf("%s：%v", name, err)
		}
	}
	if err := insert("FAILED", "VERSION_CONFLICT"); err != nil {
		t.Errorf("合法的代碼被拒絕：%v", err)
	}
}

// 店主自己的寫入（actor 傳 nil）一樣發事件，但訂閱者看到沒有代行者就略過：任何貼文寫入都不留稽核列（A7-D27）。
func TestOwnerPostWritesLeaveNoAuditRows(t *testing.T) {
	f := build(t, true)
	ctx := context.Background()
	o := f.newOperator(t, "owner-writes")
	st, err := f.sellers.Store(ctx, o.session.Session.Target.StoreID)
	if err != nil {
		t.Fatal(err)
	}
	p, err := f.realSocial.CreatePost(ctx, st, social.CreatePostInput{Body: "店主的貼文", ImageToken: fmt.Sprintf("tok-%d", f.next())}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p, err = f.realSocial.UpdatePost(ctx, st, social.UpdatePostInput{PostID: p.ID, Version: p.Version, Body: new("店主改過")}, nil); err != nil {
		t.Fatal(err)
	}
	if p, err = f.realSocial.PublishPost(ctx, st, p.ID, p.Version, nil); err != nil {
		t.Fatal(err)
	}
	if _, err = f.realSocial.PublishPost(ctx, st, p.ID, p.Version, nil); err != nil {
		t.Fatal(err)
	}
	if p, err = f.realSocial.UnpublishPost(ctx, st, p.ID, p.Version, nil); err != nil {
		t.Fatal(err)
	}
	if _, err = f.realSocial.UnpublishPost(ctx, st, p.ID, p.Version, nil); err != nil {
		t.Fatal(err)
	}
	if _, err = f.realSocial.SchedulePost(ctx, st, social.SchedulePostInput{
		PostID: p.ID, Version: p.Version, PublishAt: f.now.Add(2 * time.Hour), By: social.ScheduledBy{Kind: social.ScheduleByOwner},
	}, nil); err != nil {
		t.Fatal(err)
	}
	if n := f.count(t, `SELECT count(*) FROM delegation.admin_operation_audit_logs WHERE resource_id = $1`, p.ID.String()); n != 0 {
		t.Fatalf("店主的寫入留下 %d 列稽核，預期 0", n)
	}
}

// 代行上傳（B1-D16）：缺 media.upload 的 session 被拒絕並留一列 FAILED（MEDIA_UPLOAD／MEDIA／CAPABILITY_DENIED，不通知賣家）；有能力的 session 通過檢查，
// 而且成功的上傳（開會話與段上傳都不經 delegation 寫入）在這個 session 底下不留任何稽核列。
func TestMediaUploadDenialLeavesAFailedRowAndSuccessLeavesNone(t *testing.T) {
	f := build(t, true)
	methodUpload := DelegatedMethod{Capability: CapabilityMediaUpload, Action: AuditMediaUpload, Resource: ResourceMedia}
	read := func(c DelegatedCaller) error {
		return f.m.DelegatedRead(context.Background(), c, func(context.Context, seller.Store) error { return nil })
	}
	no := f.newOperator(t, "noupload", CapabilityPostsRead)
	if d, ok := DelegationDenialOf(read(f.caller(t, no, methodUpload))); !ok || d.Denial != DenialCapabilityDenied {
		t.Fatalf("缺 media.upload：%v，預期 DenialCapabilityDenied", d)
	}
	row := f.failedRow(t, no.session.Session.ID, "MEDIA_UPLOAD")
	if row.n != 1 || row.capability != "media.upload" || row.notify {
		t.Fatalf("FAILED 列 = %+v", row)
	}
	wantFailureReason(t, "缺 media.upload", row.failureReason, "CAPABILITY_DENIED")
	if n := f.count(t, `SELECT count(*) FROM delegation.admin_operation_audit_logs WHERE session_id = $1 AND action = 'MEDIA_UPLOAD' AND resource_type = 'MEDIA'`, no.session.Session.ID); n != 1 {
		t.Fatalf("MEDIA 資源的稽核列 %d 列，預期 1", n)
	}

	yes := f.newOperator(t, "upload", CapabilityMediaUpload)
	if err := read(f.caller(t, yes, methodUpload)); err != nil {
		t.Fatal(err)
	}
	if n := f.count(t, `SELECT count(*) FROM delegation.admin_operation_audit_logs WHERE session_id = $1`, yes.session.Session.ID); n != 0 {
		t.Fatalf("成功的上傳留下 %d 列稽核，預期 0", n)
	}
}
