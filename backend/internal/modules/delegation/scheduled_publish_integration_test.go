//go:build integration

package delegation

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/delegation/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/adminperm"
	"github.com/Will413028/hawkr/backend/internal/testkit/sellerfixture"
)

func (f *fixture) next() int64 {
	f.nextID++
	return f.nextID
}

func (f *fixture) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := f.db.Conn(context.Background()).Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%v\n%s", err, sql)
	}
}

func (f *fixture) exposableStore(t *testing.T) int64 {
	t.Helper()
	account, err := sellerfixture.NewAccount(context.Background(), f.db, f.now)
	if err != nil {
		t.Fatal(err)
	}
	storeID, err := sellerfixture.ExposableStoreNamed(context.Background(), f.db, account, f.now, fmt.Sprintf("測試店 %d", f.next()))
	if err != nil {
		t.Fatal(err)
	}
	return storeID
}

func (f *fixture) draftPost(t *testing.T, storeID int64) social.Post {
	t.Helper()
	st, err := f.sellers.Store(context.Background(), storeID)
	if err != nil {
		t.Fatal(err)
	}
	p, err := f.realSocial.CreatePost(context.Background(), st, social.CreatePostInput{Body: "代行排程的貼文", ImageToken: fmt.Sprintf("tok-%d", f.next())}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func (f *fixture) scheduledPost(t *testing.T, storeID, sessionID int64, lead time.Duration) (social.Post, social.Schedule) {
	t.Helper()
	p := f.draftPost(t, storeID)
	st, err := f.sellers.Store(context.Background(), storeID)
	if err != nil {
		t.Fatal(err)
	}
	s, err := f.realSocial.SchedulePost(context.Background(), st, social.SchedulePostInput{
		PostID: p.ID, Version: p.Version, PublishAt: f.now.Add(lead),
		By: social.ScheduledBy{Kind: social.ScheduleByDelegation, Ref: fmt.Sprint(sessionID)},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return p, s.Schedule
}

func (f *fixture) scheduleRow(t *testing.T, id uuid.UUID) (state, reason string) {
	t.Helper()
	var r *string
	if err := f.db.Pool().QueryRow(context.Background(), "SELECT state, cancel_reason FROM social.post_schedules WHERE id = $1", id).Scan(&state, &r); err != nil {
		t.Fatal(err)
	}
	if r != nil {
		reason = *r
	}
	return state, reason
}

func (f *fixture) wantSchedule(t *testing.T, id uuid.UUID, state, reason string) {
	t.Helper()
	if gotState, gotReason := f.scheduleRow(t, id); gotState != state || gotReason != reason {
		t.Fatalf("排程 %s：state=%s reason=%q，要 %s %q", id, gotState, gotReason, state, reason)
	}
}

func (f *fixture) postStatus(t *testing.T, id uuid.UUID) string {
	t.Helper()
	var s string
	if err := f.db.Pool().QueryRow(context.Background(), "SELECT status FROM social.posts WHERE id = $1", id).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

func (f *fixture) scheduleAudits(t *testing.T, scheduleID uuid.UUID) int {
	return f.count(t, `SELECT count(*) FROM delegation.admin_operation_audit_logs WHERE idempotency_key = $1`, "schedule:"+scheduleID.String())
}

func (f *fixture) publishDue(t *testing.T, s social.Schedule) {
	t.Helper()
	if f.now.Before(s.PublishAt) {
		f.now = s.PublishAt
	}
	if err := f.realSocial.PublishScheduled(context.Background(), s.ID); err != nil {
		t.Fatal(err)
	}
}

type scheduler struct {
	storeID int64
	admin   int64
	session issuedSession
}

func (f *fixture) newScheduler(t *testing.T, tag string) scheduler {
	t.Helper()
	storeID := f.exposableStore(t)
	admin := f.delegationAdmin(t, tag+"@hawkr.example", true)
	f.grant(t, admin, storeID, model.CapabilityProductsRead, model.CapabilityPostsPublish)
	return scheduler{storeID: storeID, admin: admin, session: f.start(t, startInput(admin, storeID, "ik-"+tag, model.CapabilityPostsPublish))}
}

// 已結束的 session 發起的排程，授權還在就照發，並留下恰好一列 SUCCEEDED 的稽核（origin SCHEDULED、POST_PUBLISH、posts.publish）；
// job 重跑與事件重送都不寫第二列；操作理由繼承建立排程那一列。
func TestScheduledPublishOfAnEndedSessionPublishesWithOneSucceededAudit(t *testing.T) {
	f := build(t, true)
	ctx := context.Background()
	sc := f.newScheduler(t, "ops")
	post, sched := f.scheduledPost(t, sc.storeID, sc.session.Session.ID, time.Hour)
	reason := "賣家請求排程"
	f.intent(t, AuditIntentInput{
		SessionID: sc.session.Session.ID, Capability: CapabilityPostsPublish, Action: AuditPostSchedule, ResourceType: ResourcePost,
		ResourceID: post.ID.String(), OperationReason: reason, Origin: AuditInteractive, IdempotencyKey: "ik-schedule", RequestID: "req", CorrelationID: "corr", NotifySeller: true,
	})

	f.now = f.now.Add(45 * time.Minute)
	if s, _ := f.m.ResolveDelegation(ctx, sc.session.Credential, false); s.Status != model.DelegationExpired {
		t.Fatalf("session 已到期：%+v", s)
	}
	completedBefore := len(f.completed)
	f.publishDue(t, sched)
	f.wantSchedule(t, sched.ID, "PUBLISHED", "")
	if got := f.postStatus(t, post.ID); got != "PUBLISHED" {
		t.Fatalf("貼文 = %s", got)
	}
	if n := f.scheduleAudits(t, sched.ID); n != 1 {
		t.Fatalf("稽核要恰好一列：%d", n)
	}
	var (
		result, origin, capability, action, resourceType, resourceID, key string
		actor, target                                                     int64
		notify, confirmed                                                 bool
		gotReason                                                         *string
	)
	if err := f.db.Pool().QueryRow(ctx, `SELECT result, origin, capability, action, resource_type, resource_id, idempotency_key,
		actor_admin_id, target_store_id, notify_seller, confirmed_at IS NOT NULL, operation_reason
		FROM delegation.admin_operation_audit_logs WHERE session_id = $1 AND idempotency_key = $2`,
		sc.session.Session.ID, "schedule:"+sched.ID.String()).Scan(&result, &origin, &capability, &action, &resourceType, &resourceID, &key, &actor, &target, &notify, &confirmed, &gotReason); err != nil {
		t.Fatal(err)
	}
	if result != "SUCCEEDED" || origin != "SCHEDULED" || capability != "posts.publish" || action != "POST_PUBLISH" || resourceType != "POST" ||
		resourceID != post.ID.String() || actor != sc.admin || target != sc.storeID || !notify || !confirmed || gotReason == nil || *gotReason != reason {
		t.Fatalf("稽核列 = %s %s %s %s %s %s actor=%d target=%d notify=%v confirmed=%v reason=%v", result, origin, capability, action, resourceType, resourceID, actor, target, notify, confirmed, gotReason)
	}
	if len(f.completed) != completedBefore+1 {
		t.Fatalf("賣家要收到恰好一則完成通知：%d -> %d", completedBefore, len(f.completed))
	}
	if e := f.completed[len(f.completed)-1]; e.Action != AuditPostPublish || e.TargetStoreID != sc.storeID || e.SessionID != sc.session.Session.ID || e.CorrelationID != "schedule:"+sched.ID.String() {
		t.Fatalf("完成通知 = %+v", e)
	}
	if err := f.realSocial.PublishScheduled(ctx, sched.ID); err != nil {
		t.Fatal(err)
	}
	ev := social.PostPublished{
		EventID: uuid.NewV7(), PostID: post.ID, StoreID: sc.storeID, Origin: social.OriginScheduled, ScheduleID: sched.ID,
		ScheduledBy: social.ScheduledBy{Kind: social.ScheduleByDelegation, Ref: fmt.Sprint(sc.session.Session.ID)},
	}
	for range 2 {
		if err := f.m.OnPostPublished(ctx, ev); err != nil {
			t.Fatal(err)
		}
	}
	if n := f.scheduleAudits(t, sched.ID); n != 1 {
		t.Fatalf("重跑與重送之後稽核仍是一列：%d", n)
	}
	if len(f.completed) != completedBefore+1 {
		t.Fatalf("重送不再發完成通知：%d", len(f.completed)-completedBefore)
	}
	direct := ev
	direct.Origin, direct.ScheduleID = social.OriginDirect, uuid.UUID{}
	owner := ev
	owner.ScheduledBy.Kind = social.ScheduleByOwner
	owner.ScheduleID = uuid.NewV7()
	for _, e := range []social.PostPublished{direct, owner} {
		if err := f.m.OnPostPublished(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	if n := f.count(t, `SELECT count(*) FROM delegation.admin_operation_audit_logs WHERE action = 'POST_PUBLISH'`); n != 1 {
		t.Fatalf("POST_PUBLISH 稽核總共一列：%d", n)
	}
	if s, err := f.m.DelegationSessionSummary(ctx, sc.session.Session.ID); err != nil || s.Session.Status == model.DelegationRevoked {
		t.Fatalf("session 沒有被撤銷：%+v, %v", s, err)
	}
}

// 授權被直接改資料而沒有經過任何觸發點（觸發點之後新增的路徑、手動 SQL）：到點的重驗取消排程（AUTHORITY_LOST），不發布、不寫稽核、job 不重試。
func TestAuthorityLostByDirectDataChangeCancelsAtRun(t *testing.T) {
	cases := []struct {
		name   string
		change func(t *testing.T, f *fixture, sc scheduler)
	}{
		{"店家授權被撤回", func(t *testing.T, f *fixture, sc scheduler) {
			f.exec(t, `UPDATE delegation.seller_delegation_authorizations SET revoked_at = $2, revoked_by_admin_id = $3, revoke_reason = '直接改資料' WHERE target_store_id = $1`, sc.storeID, f.now, sc.admin)
		}},
		{"授權不再含 posts.publish", func(t *testing.T, f *fixture, sc scheduler) {
			f.exec(t, `UPDATE delegation.seller_delegation_authorizations SET granted_capabilities = ARRAY['products.read'] WHERE target_store_id = $1`, sc.storeID)
		}},
		{"管理員資格被刪", func(t *testing.T, f *fixture, sc scheduler) {
			f.exec(t, `DELETE FROM delegation.admin_eligibilities WHERE admin_id = $1`, sc.admin)
		}},
		{"管理員角色被刪", func(t *testing.T, f *fixture, sc scheduler) {
			f.exec(t, `DELETE FROM identity.admin_roles WHERE admin_id = $1`, sc.admin)
		}},
		{"管理員不存在", func(t *testing.T, f *fixture, sc scheduler) { f.deleteAdmin(t, sc.admin) }},
		{"session 被撤銷", func(t *testing.T, f *fixture, sc scheduler) {
			f.exec(t, `UPDATE delegation.delegated_operation_sessions SET status = 'REVOKED', revoked_at = $2, revoke_reason = '直接改資料' WHERE id = $1`, sc.session.Session.ID, f.now)
		}},
		{"店家不可代行", func(t *testing.T, f *fixture, sc scheduler) { f.storeFn = fixedFn(seller.Closed, false) }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := build(t, true)
			sc := f.newScheduler(t, "ops")
			post, sched := f.scheduledPost(t, sc.storeID, sc.session.Session.ID, time.Hour)
			c.change(t, f, sc)
			f.publishDue(t, sched)
			f.wantSchedule(t, sched.ID, "CANCELED", "AUTHORITY_LOST")
			if got := f.postStatus(t, post.ID); got != "DRAFT" {
				t.Fatalf("不發布：%s", got)
			}
			if n := f.scheduleAudits(t, sched.ID); n != 0 {
				t.Fatalf("沒有發布就沒有稽核：%d", n)
			}
		})
	}
}

// 授權另一家店的 session 不能發布這家店的排程（排程的店家要等於 session 的店家）。
func TestScheduleOfAnotherStoreThanTheSessionIsDenied(t *testing.T) {
	f := build(t, true)
	sc := f.newScheduler(t, "ops")
	other := f.exposableStore(t)
	post, sched := f.scheduledPost(t, other, sc.session.Session.ID, time.Hour)
	f.publishDue(t, sched)
	f.wantSchedule(t, sched.ID, "CANCELED", "AUTHORITY_LOST")
	if got := f.postStatus(t, post.ID); got != "DRAFT" {
		t.Fatalf("不發布：%s", got)
	}
}

// 四個觸發點各自在同一個 transaction 內取消排程，包含已經結束（到期）的 session 發起的；別的管理員、店家的排程不受影響。
func TestTriggersCancelSchedulesIncludingEndedSessions(t *testing.T) {
	triggers := []struct {
		name string
		fire func(t *testing.T, f *fixture, sc scheduler, governor int64)
	}{
		{"撤回店家授權", func(t *testing.T, f *fixture, sc scheduler, governor int64) {
			if _, err := f.m.RevokeDelegationAuthorization(context.Background(), DelegationAuthorizationRevocation{AdminID: governor, StoreID: sc.storeID, Reason: "賣家終止合作"}); err != nil {
				t.Fatal(err)
			}
		}},
		{"撤回管理員代行資格", func(t *testing.T, f *fixture, sc scheduler, governor int64) {
			if err := f.m.SetAdminEligibility(context.Background(), AdminEligibilityChange{AdminID: sc.admin, OperatorAdminID: governor}); err != nil {
				t.Fatal(err)
			}
		}},
		{"撤銷管理員角色", func(t *testing.T, f *fixture, sc scheduler, governor int64) {
			if _, err := f.identity.RevokeAdminRole(context.Background(), identity.AdminRoleChangeInput{
				AdminID: sc.admin, Role: adminperm.RoleOwner, Reason: "測試撤銷", Source: identity.AdminRoleSourceCLI,
			}); err != nil {
				t.Fatal(err)
			}
		}},
		{"治理撤銷 session", func(t *testing.T, f *fixture, sc scheduler, governor int64) {
			if _, err := f.m.RevokeDelegation(context.Background(), RevokeDelegationInput{SessionID: sc.session.Session.ID, AdminID: governor, Reason: "治理撤銷"}); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, c := range triggers {
		t.Run(c.name, func(t *testing.T) {
			f := build(t, true)
			governor := f.delegationAdmin(t, "governor@hawkr.example", true)
			mine := f.newScheduler(t, "mine")
			theirs := f.newScheduler(t, "theirs")
			_, minePending := f.scheduledPost(t, mine.storeID, mine.session.Session.ID, 2*time.Hour)
			_, mineSecond := f.scheduledPost(t, mine.storeID, mine.session.Session.ID, 3*time.Hour)
			_, theirPending := f.scheduledPost(t, theirs.storeID, theirs.session.Session.ID, 2*time.Hour)

			f.now = f.now.Add(40 * time.Minute)
			if s, _ := f.m.ResolveDelegation(context.Background(), mine.session.Credential, false); s.Status != model.DelegationExpired {
				t.Fatalf("要在 session 已結束時觸發：%+v", s)
			}
			c.fire(t, f, mine, governor)
			f.wantSchedule(t, minePending.ID, "CANCELED", "AUTHORITY_LOST")
			f.wantSchedule(t, mineSecond.ID, "CANCELED", "AUTHORITY_LOST")
			f.wantSchedule(t, theirPending.ID, "PENDING", "")
		})
	}
}

// 治理撤銷只取消被撤銷的那個 session 的排程。
func TestRevokingOneSessionCancelsOnlyItsSchedules(t *testing.T) {
	f := build(t, true)
	governor := f.delegationAdmin(t, "governor@hawkr.example", true)
	a := f.newScheduler(t, "a")
	b := f.newScheduler(t, "b")
	_, sa := f.scheduledPost(t, a.storeID, a.session.Session.ID, time.Hour)
	_, sb := f.scheduledPost(t, b.storeID, b.session.Session.ID, time.Hour)
	if _, err := f.m.RevokeDelegation(context.Background(), RevokeDelegationInput{SessionID: a.session.Session.ID, AdminID: governor, Reason: "治理撤銷"}); err != nil {
		t.Fatal(err)
	}
	f.wantSchedule(t, sa.ID, "CANCELED", "AUTHORITY_LOST")
	f.wantSchedule(t, sb.ID, "PENDING", "")
}

// 登出是結束不是撤銷：憑證立刻失效，但排程不取消，授權還在就照發並留稽核。
func TestSignOutEndsTheSessionAndTheScheduleStillPublishes(t *testing.T) {
	f := build(t, true)
	ctx := context.Background()
	sc := f.newScheduler(t, "ops")
	post, sched := f.scheduledPost(t, sc.storeID, sc.session.Session.ID, time.Hour)
	f.now = f.now.Add(time.Minute)
	if err := f.m.EndDelegationsOfAdmin(ctx, sc.admin); err != nil {
		t.Fatal(err)
	}
	got, err := f.m.ResolveDelegation(ctx, sc.session.Credential, false)
	if err != nil || got.Status != model.DelegationExpired || got.RevokedAt != nil {
		t.Fatalf("登出之後 credential 已失效（到期），不是撤銷：%+v, %v", got, err)
	}
	f.wantSchedule(t, sched.ID, "PENDING", "")
	f.publishDue(t, sched)
	f.wantSchedule(t, sched.ID, "PUBLISHED", "")
	if got := f.postStatus(t, post.ID); got != "PUBLISHED" {
		t.Fatalf("貼文 = %s", got)
	}
	if n := f.scheduleAudits(t, sched.ID); n != 1 {
		t.Fatalf("稽核 = %d", n)
	}
}

// 撤回店家授權（取消排程後才鎖授權列）與到點的 job（先鎖貼文列再重驗）同時進行：不死結，結果一致（發布或 AUTHORITY_LOST）。
// 撤銷管理員角色（identity 的角色列 → 取消排程）與 job 同理。
func TestRevokeAndScheduledJobDoNotDeadlock(t *testing.T) {
	revokers := []struct {
		name   string
		revoke func(f *fixture, sc scheduler, governor int64) error
	}{
		{"撤回店家授權", func(f *fixture, sc scheduler, governor int64) error {
			_, err := f.m.RevokeDelegationAuthorization(context.Background(), DelegationAuthorizationRevocation{AdminID: governor, StoreID: sc.storeID, Reason: "賣家終止合作"})
			return err
		}},
		{"撤銷管理員角色", func(f *fixture, sc scheduler, governor int64) error {
			_, err := f.identity.RevokeAdminRole(context.Background(), identity.AdminRoleChangeInput{
				AdminID: sc.admin, Role: adminperm.RoleOwner, Reason: "測試撤銷", Source: identity.AdminRoleSourceCLI,
			})
			return err
		}},
	}
	for _, r := range revokers {
		t.Run(r.name, func(t *testing.T) {
			f := build(t, true)
			governor := f.delegationAdmin(t, "governor@hawkr.example", true)
			for i := range 12 {
				sc := f.newScheduler(t, fmt.Sprintf("race%d", i))
				_, sched := f.scheduledPost(t, sc.storeID, sc.session.Session.ID, time.Hour)
				f.now = sched.PublishAt
				var wg sync.WaitGroup
				var jobErr, revokeErr error
				wg.Add(2)
				go func() { defer wg.Done(); jobErr = f.realSocial.PublishScheduled(context.Background(), sched.ID) }()
				go func() { defer wg.Done(); revokeErr = r.revoke(f, sc, governor) }()
				wg.Wait()
				if jobErr != nil || revokeErr != nil {
					t.Fatalf("第 %d 輪：job=%v revoke=%v", i, jobErr, revokeErr)
				}
				state, reason := f.scheduleRow(t, sched.ID)
				published, lost := state == "PUBLISHED" && reason == "", state == "CANCELED" && reason == "AUTHORITY_LOST"
				if !published && !lost {
					t.Fatalf("第 %d 輪：排程要是 PUBLISHED 或 AUTHORITY_LOST：%s %s", i, state, reason)
				}
				want := 0
				if state == "PUBLISHED" {
					want = 1
				}
				if n := f.scheduleAudits(t, sched.ID); n != want {
					t.Fatalf("第 %d 輪：state=%s 稽核 %d 列", i, state, n)
				}
			}
		})
	}
}

// 取消排程不依 session 的年齡截斷：逾期還是 PENDING 的排程（job 沒跑），即使它的 session 早在最長排程距離加代行有效期之前發出，觸發當下仍被取消。
func TestCancelOnTriggerCoversAnOverduePendingScheduleOfAnOldSession(t *testing.T) {
	f := build(t, true)
	governor := f.delegationAdmin(t, "governor@hawkr.example", true)
	sc := f.newScheduler(t, "ops")
	_, farthest := f.scheduledPost(t, sc.storeID, sc.session.Session.ID, social.MaxScheduleLead)

	f.now = f.now.Add(social.MaxScheduleLead + model.DelegationTTL + time.Minute)
	if err := f.m.SetAdminEligibility(context.Background(), AdminEligibilityChange{AdminID: sc.admin, OperatorAdminID: governor}); err != nil {
		t.Fatal(err)
	}
	f.wantSchedule(t, farthest.ID, "CANCELED", "AUTHORITY_LOST")
}

// 排程發布的稽核繼承建立排程那一列的 notify_seller：為真才寫 notified_at 並發完成事件，為假都不做。
func TestScheduledPublishAuditInheritsNotifySeller(t *testing.T) {
	for _, notify := range []bool{true, false} {
		t.Run(fmt.Sprint(notify), func(t *testing.T) {
			f := build(t, true)
			ctx := context.Background()
			sc := f.newScheduler(t, "ops")
			post, sched := f.scheduledPost(t, sc.storeID, sc.session.Session.ID, time.Hour)
			f.intent(t, AuditIntentInput{
				SessionID: sc.session.Session.ID, Capability: CapabilityPostsPublish, Action: AuditPostSchedule, ResourceType: ResourcePost,
				ResourceID: post.ID.String(), OperationReason: "賣家請求排程", Origin: AuditInteractive, IdempotencyKey: "ik-schedule", RequestID: "req", CorrelationID: "corr",
				NotifySeller: notify,
			})
			completedBefore := len(f.completed)
			f.publishDue(t, sched)
			var gotNotify, notified bool
			if err := f.db.Pool().QueryRow(ctx, `SELECT notify_seller, notified_at IS NOT NULL FROM delegation.admin_operation_audit_logs
				WHERE session_id = $1 AND idempotency_key = $2`, sc.session.Session.ID, "schedule:"+sched.ID.String()).Scan(&gotNotify, &notified); err != nil {
				t.Fatal(err)
			}
			want := 0
			if notify {
				want = 1
			}
			if gotNotify != notify || notified != notify || len(f.completed)-completedBefore != want {
				t.Fatalf("notify=%v：稽核 notify_seller=%v notified_at=%v、完成事件 %d 則", notify, gotNotify, notified, len(f.completed)-completedBefore)
			}
		})
	}
}
