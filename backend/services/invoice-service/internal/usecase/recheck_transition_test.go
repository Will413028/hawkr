package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// 本檔驗 INV.rc 的 I7 留痕（#3109）。
//
// 🔴 `applyDecision` 在 2026-09-04 之前**完全沒有測試**——`recheck_decide_test.go`
// 只測純函式 `decide`，而寫回那一段沒有任何覆蓋。這正是「六條轉移一列 history
// 都不寫」能活到 dev 才被發現的原因：缺陷不在難測的地方，是那裡根本沒有測試。

type fakeRecheckOps struct {
	RecheckOperationStore

	ok       bool
	resolved []domain.MarkRecheckResult
	unres    []domain.MarkRecheckResult
	attempts []domain.UpdateRecheckAttempt
}

func (f *fakeRecheckOps) MarkRecheckResolved(_ context.Context, in domain.MarkRecheckResult) (bool, error) {
	f.resolved = append(f.resolved, in)
	return f.ok, nil
}

func (f *fakeRecheckOps) MarkRecheckUnresolved(_ context.Context, in domain.MarkRecheckResult) (bool, error) {
	f.unres = append(f.unres, in)
	return f.ok, nil
}

func (f *fakeRecheckOps) UpdateRecheckAttempt(_ context.Context, in domain.UpdateRecheckAttempt) (bool, error) {
	f.attempts = append(f.attempts, in)
	return f.ok, nil
}

func rcFixture(t *testing.T, casOK bool) (*Recheck, *fakeRecheckOps, *fakeAudit) {
	t.Helper()
	ops := &fakeRecheckOps{ok: casOK}
	audit := &fakeAudit{}
	now := func() time.Time { return adminNow }
	return NewRecheck(ops, nil, nil, nil, nil, nil, audit, nil, now), ops, audit
}

func rcOp(status string) domain.Operation {
	return domain.Operation{ID: 8001, Status: status}
}

// J24：對帳確認未開立且有 ADM 取消在途 ⇒ CANCELLED_BY_ADMIN。這條邊在 dev 上
// 實際發生過，而 history 一列都沒有 ⇒ 軌跡從 CANCEL_REQUESTED 直接跳到
// CANCELLED_BY_ADMIN，前後兩列的狀態對不上。
func TestRecheck_ApplyDecision_RecordsHistory(t *testing.T) {
	uc, ops, audit := rcFixture(t, true)

	err := uc.applyDecision(context.Background(), rcOp("PROVIDER_STATUS_RECHECK"), RecheckDecision{
		Kind: RecheckResolved, NextStatus: "CANCELLED_BY_ADMIN", NextAttemptAt: adminNow,
	})
	if err != nil {
		t.Fatalf("applyDecision 失敗：%v", err)
	}
	if len(ops.resolved) != 1 {
		t.Fatalf("CAS 應被呼叫一次，實得 %d", len(ops.resolved))
	}
	if len(audit.entries) != 1 {
		t.Fatalf("I7：每一次 status 轉移都要有一列 history，實得 %d", len(audit.entries))
	}
	e := audit.entries[0]
	if derefString(e.FromStatus) != "PROVIDER_STATUS_RECHECK" || e.ToStatus != "CANCELLED_BY_ADMIN" {
		t.Fatalf("history 的 from/to 錯：%v -> %v", e.FromStatus, e.ToStatus)
	}
	if derefString(e.ReasonCode) != recheckReasonCode {
		t.Fatalf("reason_code 應為 %s，實得 %v", recheckReasonCode, e.ReasonCode)
	}
}

// 🔴 CAS rows=0 時**不得**留痕：此時 op.Status 是快照值、不是真實值，
// 拿它當 from 會寫出一條沒發生過的邊。
func TestRecheck_ApplyDecision_NoHistoryWhenCASFails(t *testing.T) {
	uc, _, audit := rcFixture(t, false)

	if err := uc.applyDecision(context.Background(), rcOp("PROVIDER_STATUS_RECHECK"), RecheckDecision{
		Kind: RecheckUnresolvedTerminal, NextStatus: "FAILED_UNKNOWN", NextAttemptAt: adminNow,
	}); err != nil {
		t.Fatalf("CAS rows=0 不是錯誤：%v", err)
	}
	if len(audit.entries) != 0 {
		t.Fatalf("CAS 未命中時不得寫 history，實得 %d 列", len(audit.entries))
	}
}

// 自轉移不寫：重查迴圈最短 30 秒一輪，逐輪寫會把真正的狀態變化淹掉。
func TestRecheck_ApplyDecision_SelfLoopNotRecorded(t *testing.T) {
	uc, ops, audit := rcFixture(t, true)

	if err := uc.applyDecision(context.Background(), rcOp("PROVIDER_STATUS_RECHECK"), RecheckDecision{
		Kind: RecheckStillWaiting, NextStatus: "PROVIDER_STATUS_RECHECK", NextAttemptAt: adminNow,
	}); err != nil {
		t.Fatalf("自轉移不是錯誤：%v", err)
	}
	if len(ops.attempts) != 1 {
		t.Fatalf("狀態仍要寫回（next_attempt_at 是階梯的載體），實得 %d", len(ops.attempts))
	}
	if len(audit.entries) != 0 {
		t.Fatalf("自轉移不得寫 history，實得 %d 列", len(audit.entries))
	}
}

// fsm 不認得的邊 ⇒ 當場回 error。轉換表漏登記與「有人繞過它加了邊」都必須炸掉，
// 否則 history 會累積出一份與設計書不一致的事實。
func TestRecheck_ApplyDecision_IllegalEdgeFailsLoud(t *testing.T) {
	uc, _, audit := rcFixture(t, true)

	err := uc.applyDecision(context.Background(), rcOp("SUCCEEDED"), RecheckDecision{
		Kind: RecheckResolved, NextStatus: "READY", NextAttemptAt: adminNow,
	})
	if err == nil {
		t.Fatal("SUCCEEDED -> READY 不在 M2 轉換表裡，必須回 error")
	}
	if len(audit.entries) != 0 {
		t.Fatalf("非法邊不得留痕，實得 %d 列", len(audit.entries))
	}
}

// TestRecheck_ClaimTransition_RecordsEntryIntoRecheckLoop 守 **claim 那一步**的 I7 留痕。
//
// 🔴 dev 實測（2026-09-04 16:21，operation 7500300000000000002）：
// #3109 補了離開重查迴圈那一半，進入的那一半（J20／J22／J27）仍然沒有任何列 ⇒
// 軌跡是 `→ CANCEL_REQUESTED` 之後直接 `PROVIDER_STATUS_RECHECK → SUCCEEDED`，
// 上一列的 to 對不上下一列的 from。那正是 transition.go 檔頭引為病徵的形狀。
//
// 期望值一律硬寫字面值（不引用被測物用到的常數），否則常數一改斷言跟著變。
func TestRecheck_ClaimTransition_RecordsEntryIntoRecheckLoop(t *testing.T) {
	cases := []struct {
		name string
		from string
		want bool // 是否應留痕
	}{
		{"J22 強制最終查詢（ADM 取消在途）", "CANCEL_REQUESTED", true},
		{"J20 週期重查", "FAILED_UNKNOWN", true},
		{"J27 退避週期到期", "CANCEL_BLOCKED_UNKNOWN", true},
		// J15 是 30 秒一輪的輪詢對，逐輪留痕會把真正的狀態變化淹掉（同 fsm.TransitionSelfLoop 的理由）。
		{"J15 輪詢對的一半（刻意不記）", "WAITING_CONFIRMATION", false},
		// J16：租約到期後被重新 Claim，from == to。
		{"J16 自轉移（租約回收）", "PROVIDER_STATUS_RECHECK", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			uc, _, audit := rcFixture(t, true)
			if err := uc.recordClaimTransition(context.Background(), 8001, c.from, adminNow); err != nil {
				t.Fatalf("recordClaimTransition(%s)：%v", c.from, err)
			}
			if !c.want {
				if len(audit.entries) != 0 {
					t.Fatalf("%s 不該留痕，實得 %d 列", c.from, len(audit.entries))
				}
				return
			}
			if len(audit.entries) != 1 {
				t.Fatalf("%s 應留 1 列（否則軌跡在這裡斷開），實得 %d", c.from, len(audit.entries))
			}
			e := audit.entries[0]
			if derefString(e.FromStatus) != c.from {
				t.Errorf("from = %v，want %s", e.FromStatus, c.from)
			}
			if e.ToStatus != "PROVIDER_STATUS_RECHECK" {
				t.Errorf("to = %s，want PROVIDER_STATUS_RECHECK", e.ToStatus)
			}
			if derefString(e.ReasonCode) != "INV_RECHECK_CLAIMED" {
				t.Errorf("reason_code = %v，want INV_RECHECK_CLAIMED —— "+
					"與 INV_RECHECK 分開才分得出「誰撿起來」與「對帳結論是什麼」", e.ReasonCode)
			}
			if e.ActorType != "SYSTEM" {
				t.Errorf("actor_type = %s，want SYSTEM", e.ActorType)
			}
		})
	}
}

// TestRecheck_ClaimTransition_RejectsEdgeNotInM2 守「fsm 不認得的邊一律回 error」。
//
// 靜默寫進去的話，invoice_status_history 會累積出一份與設計書不一致的**事實**，
// 而它正是 A10 稽核與 AuditTrailDrawer 呈現給人看的東西。
func TestRecheck_ClaimTransition_RejectsEdgeNotInM2(t *testing.T) {
	uc, _, audit := rcFixture(t, true)
	// SUCCEEDED → PROVIDER_STATUS_RECHECK 不在 M2 表上（終態不回頭）。
	if err := uc.recordClaimTransition(context.Background(), 8001, "SUCCEEDED", adminNow); err == nil {
		t.Fatal("fsm 不認得的邊應回 error")
	}
	if len(audit.entries) != 0 {
		t.Fatalf("不合法的邊不得留痕，實得 %d 列", len(audit.entries))
	}
}

// --- 從 claim 走起的行為測試 -------------------------------------------------
//
// 🔴 為什麼一定要從 reconcileOperations 走：既有的
// TestRecheck_ApplyDecision_RecordsHistory 直接呼叫 applyDecision，而且**把 from
// 手動設成 PROVIDER_STATUS_RECHECK** —— 它假設了正好壞掉的那件事。
// 真實流程是 FetchPendingRechecks 撈出 claim **之前**的狀態，claim 把 DB 改掉之後
// 記憶體那一份沒跟上，於是 applyDecision 拿舊值當 from。
// dev 實證（2026-09-05）：全表 `reason_code='INV_RECHECK'` 0 列、
// `to_status='PROVIDER_STATUS_RECHECK'` 0 列，而同表有 37 種其他 reason_code。
// 從入口走起，前提才會被真的執行到（`.claude/rules/06-no-small-tails.md` §3.13）。

type rcReconcileOps struct {
	RecheckOperationStore
	due       []domain.Operation
	claimOK   bool
	claimArgs []int64
}

func (f *rcReconcileOps) FetchPendingRechecks(context.Context, time.Time, int32) ([]domain.Operation, error) {
	return f.due, nil
}

func (f *rcReconcileOps) ClaimRecheck(_ context.Context, id int64, _, _ time.Time) (bool, error) {
	f.claimArgs = append(f.claimArgs, id)
	return f.claimOK, nil
}

// rcAllowanceSpy 記下**交給它的那個 op**——本測試要驗的就是那一份的 Status。
type rcAllowanceSpy struct{ got []domain.Operation }

func (s *rcAllowanceSpy) ConfirmAllowance(_ context.Context, op domain.Operation) error {
	s.got = append(s.got, op)
	return nil
}

// TestRecheck_ReconcileSyncsStatusAfterClaim 守「claim 之後記憶體那份 op 必須跟上」。
//
// 兩件事一起驗，因為它們是同一個 bug 的兩半：
//
//	① 交給下游（ConfirmAllowance／decideRecheck／applyDecision）的 op.Status
//	   必須已經是 PROVIDER_STATUS_RECHECK，不是撈出來時的舊值
//	② claim 那一步本身要留一列 history（J20：FAILED_UNKNOWN → PROVIDER_STATUS_RECHECK）
//
// 期望值硬寫字面值，不引用被測物用到的常數。
func TestRecheck_ReconcileSyncsStatusAfterClaim(t *testing.T) {
	ops := &rcReconcileOps{
		claimOK: true,
		due: []domain.Operation{{
			ID:            8101,
			Status:        "FAILED_UNKNOWN", // ← 撈出來時的狀態（claim 之前）
			OperationType: "ALLOWANCE",      // 走 ConfirmAllowance 那條分支，不需要 provider 依賴
		}},
	}
	spy := &rcAllowanceSpy{}
	audit := &fakeAudit{}
	uc := NewRecheck(ops, nil, nil, nil, nil, spy, audit, nil, func() time.Time { return adminNow })

	if err := uc.reconcileOperations(context.Background()); err != nil {
		t.Fatalf("reconcileOperations: %v", err)
	}
	if len(ops.claimArgs) != 1 {
		t.Fatalf("ClaimRecheck 應被呼叫一次，實得 %d", len(ops.claimArgs))
	}

	// ① 下游拿到的必須是 claim 之後的狀態。
	if len(spy.got) != 1 {
		t.Fatalf("ConfirmAllowance 應被呼叫一次，實得 %d", len(spy.got))
	}
	if got := spy.got[0].Status; got != "PROVIDER_STATUS_RECHECK" {
		t.Errorf("交給下游的 op.Status = %q，want PROVIDER_STATUS_RECHECK —— "+
			"用舊值會讓 fsm.CheckOperation 收到一條合併過的邊（X→Y），"+
			"多半不在 M2 表上 ⇒ 回 error ⇒ 整條 INV.rc 軸一列 history 都不寫", got)
	}

	// ② claim 那一步的 history（J20）。
	if len(audit.entries) != 1 {
		t.Fatalf("claim 應留 1 列 history，實得 %d —— 少了它，軌跡上「進入重查迴圈」那一步整段消失",
			len(audit.entries))
	}
	e := audit.entries[0]
	if derefString(e.FromStatus) != "FAILED_UNKNOWN" || e.ToStatus != "PROVIDER_STATUS_RECHECK" {
		t.Errorf("history 的 from/to = %v -> %v，want FAILED_UNKNOWN -> PROVIDER_STATUS_RECHECK",
			e.FromStatus, e.ToStatus)
	}
	if derefString(e.ReasonCode) != "INV_RECHECK_CLAIMED" {
		t.Errorf("reason_code = %v，want INV_RECHECK_CLAIMED", e.ReasonCode)
	}
}
