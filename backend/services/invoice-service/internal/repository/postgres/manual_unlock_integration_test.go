//go:build integration

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// 本檔守的是**單元測試零鑑別力**的那些不變式：invoice_manual_unlocks 與
// invoice_provider_cutovers 的 CHECK、partial unique 與 CAS 述詞。
//
// fake repository 沒有 CHECK：「雙人不得同一人」「1–1000 字」「APPROVED 必有第二人」
// 在單元測試裡怎麼寫都可以是綠的——真正在守的是 DB，所以要在真 DB 上弄壞它。

const muEvidence = "致電光貿客服，確認 requestId 未產生任何發票"

func newUnlock(opID, requester int64, now time.Time) domain.NewManualUnlock {
	return domain.NewManualUnlock{
		ID:                    nextID(),
		OperationID:           opID,
		RequestedByAccountID:  requester,
		RequestReason:         "AMEGO 逾時，已查證未開立",
		ProviderCheckEvidence: muEvidence,
		ProviderCheckedAt:     now.Add(-time.Hour),
		ExpiresAt:             now.Add(domain.ManualUnlockExpiry),
		Now:                   now,
	}
}

// ---------------------------------------------------------------------------
// ① 雙人守衛：CAS 的 WHERE（回 rows=0）＋ chk_invmu_two_person（兜底）
// ---------------------------------------------------------------------------

func TestIntegration_ManualUnlock_SameActorCannotApprove(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewManualUnlockRepository(pool)
	_, opID := seedManualJob(ctx, t, pool, "ISSUE_FAILED", "CANCELLED_BY_ADMIN")
	now := time.Now().UTC()

	row, taken, err := repo.Create(ctx, newUnlock(opID, 4201, now))
	if err != nil || taken {
		t.Fatalf("Create: err=%v taken=%v", err, taken)
	}

	// 發起人自己核准 ⇒ CAS 的 `requested_by_account_id <> $approver` 不成立 ⇒ rows=0。
	ok, err := repo.Decide(ctx, domain.DecideManualUnlock{
		ID: row.ID, OperationID: opID, NextStatus: domain.ManualUnlockApproved,
		ApprovedByAccountID: 4201, DecisionReason: "自己核准自己",
		ExpectedVersion: row.Version, Now: now,
	})
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if ok {
		t.Fatal("同一人核准必須 rows=0——這是全案唯一的雙人機制")
	}

	// 而且那一列完全沒被動過（rows=0 的危險形狀是「擋下來了但已經寫了一半」）。
	after, err := repo.GetByID(ctx, row.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if after.Status != domain.ManualUnlockAwaitingSecondApproval ||
		after.ApprovedByAccountID != nil || after.Version != row.Version {
		t.Fatalf("被擋下的那一列不得有任何變化：%+v", after)
	}

	// 換一個人就過得去（鑑別力：上面那個 false 不是因為別的原因）。
	ok, err = repo.Decide(ctx, domain.DecideManualUnlock{
		ID: row.ID, OperationID: opID, NextStatus: domain.ManualUnlockApproved,
		ApprovedByAccountID: 4202, DecisionReason: "已複核",
		ExpectedVersion: row.Version, Now: now,
	})
	if err != nil || !ok {
		t.Fatalf("第二個人核准應成功：ok=%v err=%v", ok, err)
	}
}

// 🔴 CAS 述詞與 chk_invmu_two_person 是**兩道獨立的守衛**，不是同一道寫兩次。
//
// 本支把 CHECK 卸掉之後再跑一次 Decide：CAS 仍然必須 rows=0。它存在的理由是
// 2026-08-30 的變異實測——把 CAS 的 `requested_by_account_id <> $approver` 刪掉之後，
// 上一支測試確實轉紅了，但**紅的原因換成了 23514**（CHECK 兜底）。兩者對 ADM 的結果
// 完全不同：rows=0 回得出 9345「不得同一人」，23514 是一句 500。只斷言「沒有成功」的
// 測試分不出這兩者，於是 CAS 那道守衛可以被刪掉而沒有任何東西轉紅到位。
func TestIntegration_ManualUnlock_CASGuardHoldsWithoutTheCheckConstraint(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewManualUnlockRepository(pool)
	_, opID := seedManualJob(ctx, t, pool, "ISSUE_FAILED", "CANCELLED_BY_ADMIN")
	now := time.Now().UTC()

	row, _, err := repo.Create(ctx, newUnlock(opID, 4201, now))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	// 每支測試拿到的是自己的 database（見本 package 的 dbtest 說明）⇒ 卸掉 CHECK
	// 不會影響其他測試。
	mustExec(ctx, t, pool,
		`ALTER TABLE invoice_manual_unlocks DROP CONSTRAINT chk_invmu_two_person`)

	ok, err := repo.Decide(ctx, domain.DecideManualUnlock{
		ID: row.ID, OperationID: opID, NextStatus: domain.ManualUnlockApproved,
		ApprovedByAccountID: 4201, DecisionReason: "自己核准自己",
		ExpectedVersion: row.Version, Now: now,
	})
	if err != nil {
		t.Fatalf("卸掉 CHECK 之後不該有 SQL 錯誤（那表示擋下來的是 CHECK 不是 CAS）：%v", err)
	}
	if ok {
		t.Fatal("CAS 的雙人述詞必須自己就擋得住，不能依賴 CHECK 兜底")
	}
}

// chk_invmu_two_person 是 DB 兜底：即使有人繞過 repository 直接下 SQL，
// 「核准人 ＝ 發起人」也寫不進去（23514）。
func TestIntegration_ManualUnlock_TwoPersonCheckRejectsDirectSQL(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewManualUnlockRepository(pool)
	_, opID := seedManualJob(ctx, t, pool, "ISSUE_FAILED", "CANCELLED_BY_ADMIN")
	now := time.Now().UTC()

	row, _, err := repo.Create(ctx, newUnlock(opID, 4201, now))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	_, err = pool.Exec(ctx,
		`UPDATE invoice_manual_unlocks
		    SET status = 'APPROVED', approved_by_account_id = requested_by_account_id,
		        decided_at = now()
		  WHERE id = $1`, row.ID)
	if err == nil {
		t.Fatal("chk_invmu_two_person 必須擋下「核准人＝發起人」")
	}
}

// ---------------------------------------------------------------------------
// ② chk_invmu_decided_pair：APPROVED／REJECTED ⇔ 有第二人且有決策時間（雙向）
// ---------------------------------------------------------------------------

func TestIntegration_ManualUnlock_DecidedPairIsBidirectional(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewManualUnlockRepository(pool)
	_, opID := seedManualJob(ctx, t, pool, "ISSUE_FAILED", "CANCELLED_BY_ADMIN")
	now := time.Now().UTC()

	row, _, err := repo.Create(ctx, newUnlock(opID, 4201, now))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// 正向：APPROVED 卻沒有第二人 ⇒ 23514。
	if _, err := pool.Exec(ctx,
		`UPDATE invoice_manual_unlocks SET status = 'APPROVED' WHERE id = $1`, row.ID); err == nil {
		t.Fatal("APPROVED 卻沒有第二人／決策時間，必須被 chk_invmu_decided_pair 擋下")
	}

	// 反向同樣要擋：AWAITING 卻掛著一個 approved_by，會讓稽核上出現兩個互相矛盾的答案。
	if _, err := pool.Exec(ctx,
		`UPDATE invoice_manual_unlocks
		    SET approved_by_account_id = 4202, decided_at = now()
		  WHERE id = $1`, row.ID); err == nil {
		t.Fatal("AWAITING 不得掛著第二人與決策時間")
	}
}

// REJECTED 一定要有理由：沒有理由的駁回在事後無法還原「為什麼不放行」。
func TestIntegration_ManualUnlock_RejectRequiresReason(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewManualUnlockRepository(pool)
	_, opID := seedManualJob(ctx, t, pool, "ISSUE_FAILED", "CANCELLED_BY_ADMIN")
	now := time.Now().UTC()

	row, _, err := repo.Create(ctx, newUnlock(opID, 4201, now))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE invoice_manual_unlocks
		    SET status = 'REJECTED', approved_by_account_id = 4202, decided_at = now()
		  WHERE id = $1`, row.ID); err == nil {
		t.Fatal("REJECTED 沒有 decision_reason 必須被 chk_invmu_reject_reason 擋下")
	}

	// 帶了理由就過得去（鑑別力）。
	if _, err := pool.Exec(ctx,
		`UPDATE invoice_manual_unlocks
		    SET status = 'REJECTED', approved_by_account_id = 4202,
		        decision_reason = '查證紀錄不足', decided_at = now()
		  WHERE id = $1`, row.ID); err != nil {
		t.Fatalf("帶了理由的駁回應可寫入：%v", err)
	}
}

// chk_invmu_evidence_len：1–1000 字。0 字與 1001 字都要被擋，1000 字要放行。
func TestIntegration_ManualUnlock_EvidenceLengthBounds(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewManualUnlockRepository(pool)
	_, opID := seedManualJob(ctx, t, pool, "ISSUE_FAILED", "CANCELLED_BY_ADMIN")
	now := time.Now().UTC()

	long := make([]rune, 1001)
	for i := range long {
		long[i] = '查'
	}

	in := newUnlock(opID, 4201, now)
	in.ProviderCheckEvidence = string(long)
	if _, _, err := repo.Create(ctx, in); err == nil {
		t.Fatal("1001 字必須被 chk_invmu_evidence_len 擋下")
	}

	// 交易在 23514 之後不會 abort（本例未包交易），改用新 pool 語句即可。
	in = newUnlock(opID, 4201, now)
	in.ProviderCheckEvidence = string(long[:1000])
	if _, taken, err := repo.Create(ctx, in); err != nil || taken {
		t.Fatalf("1000 字應合法：err=%v taken=%v", err, taken)
	}
}

// ---------------------------------------------------------------------------
// ③ uq_invmu_open_per_operation ＋ ExpireStale 的名額釋放
// ---------------------------------------------------------------------------

func TestIntegration_ManualUnlock_OnlyOneOpenRequestPerJob(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewManualUnlockRepository(pool)
	_, opID := seedManualJob(ctx, t, pool, "ISSUE_FAILED", "CANCELLED_BY_ADMIN")
	now := time.Now().UTC()

	if _, taken, err := repo.Create(ctx, newUnlock(opID, 4201, now)); err != nil || taken {
		t.Fatalf("第一列應建得起來：err=%v taken=%v", err, taken)
	}
	if _, taken, err := repo.Create(ctx, newUnlock(opID, 4201, now)); err != nil || !taken {
		t.Fatalf("第二列應撞 uq_invmu_open_per_operation（taken=true）：err=%v taken=%v", err, taken)
	}
}

// 🔴 這一支是「EXPIRED 不由排程器收斂」那條規則的機械保證：
// 少了 ExpireStale，一個 job 逾期一次之後就永遠再也發起不了第二次解鎖。
func TestIntegration_ManualUnlock_ExpireStaleReleasesTheSlot(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewManualUnlockRepository(pool)
	_, opID := seedManualJob(ctx, t, pool, "ISSUE_FAILED", "CANCELLED_BY_ADMIN")
	now := time.Now().UTC()

	if _, _, err := repo.Create(ctx, newUnlock(opID, 4201, now)); err != nil {
		t.Fatalf("Create: %v", err)
	}
	later := now.Add(domain.ManualUnlockExpiry + time.Minute)

	// 尚未收斂 ⇒ 仍然撞唯一索引（先證明那個索引真的擋著）。
	if _, taken, err := repo.Create(ctx, newUnlock(opID, 4201, later)); err != nil || !taken {
		t.Fatalf("收斂前應撞唯一索引：err=%v taken=%v", err, taken)
	}

	n, err := repo.ExpireStale(ctx, opID, later)
	if err != nil {
		t.Fatalf("ExpireStale: %v", err)
	}
	if n != 1 {
		t.Fatalf("應收斂一列，實得 %d", n)
	}
	if _, taken, err := repo.Create(ctx, newUnlock(opID, 4201, later)); err != nil || taken {
		t.Fatalf("收斂後應可重新發起：err=%v taken=%v", err, taken)
	}
}

// ExpireStale 不得碰未逾期的那一列（否則等待覆核中的請求會被下一次請求抹掉）。
func TestIntegration_ManualUnlock_ExpireStaleSkipsLiveRow(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewManualUnlockRepository(pool)
	_, opID := seedManualJob(ctx, t, pool, "ISSUE_FAILED", "CANCELLED_BY_ADMIN")
	now := time.Now().UTC()

	if _, _, err := repo.Create(ctx, newUnlock(opID, 4201, now)); err != nil {
		t.Fatalf("Create: %v", err)
	}
	n, err := repo.ExpireStale(ctx, opID, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("ExpireStale: %v", err)
	}
	if n != 0 {
		t.Fatalf("未逾期的列不得被收斂，實得 %d", n)
	}
}

// 逾期之後 CAS 也必須擋（`expires_at > now`）：光靠讀取面的投影是不夠的，
// 兩個分頁同時開著時 ADM 仍然按得下去。
func TestIntegration_ManualUnlock_ExpiredCannotBeApproved(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewManualUnlockRepository(pool)
	_, opID := seedManualJob(ctx, t, pool, "ISSUE_FAILED", "CANCELLED_BY_ADMIN")
	now := time.Now().UTC()

	row, _, err := repo.Create(ctx, newUnlock(opID, 4201, now))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	ok, err := repo.Decide(ctx, domain.DecideManualUnlock{
		ID: row.ID, OperationID: opID, NextStatus: domain.ManualUnlockApproved,
		ApprovedByAccountID: 4202, DecisionReason: "遲到的覆核",
		ExpectedVersion: row.Version, Now: now.Add(domain.ManualUnlockExpiry + time.Minute),
	})
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if ok {
		t.Fatal("逾期的解鎖請求不得被核准")
	}
}

// ---------------------------------------------------------------------------
// ④ J29：CANCELLED_BY_ADMIN → MANUAL_PROCESS_ALLOWED 的第二道雙人守衛
// ---------------------------------------------------------------------------

func TestIntegration_Operation_AllowManualProcessGuards(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewOperationRepository(pool)
	_, opID := seedManualJob(ctx, t, pool, "ISSUE_FAILED", "CANCELLED_BY_ADMIN")
	now := time.Now().UTC()

	// J29 的述詞要求 cancel_requested_by 非空（取消是雙人流程的第一步）。
	mustExec(ctx, t, pool,
		`UPDATE invoice_operations
		    SET cancel_requested_by = 4203, cancel_requested_at = now()
		  WHERE id = $1`, opID)

	op, err := repo.GetByID(ctx, opID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}

	// 取消發起人自己來解鎖 ⇒ rows=0。
	ok, err := repo.AllowManualProcess(ctx, opID, op.Version, 4203, now)
	if err != nil {
		t.Fatalf("AllowManualProcess: %v", err)
	}
	if ok {
		t.Fatal("取消發起人不得同時是解鎖覆核人")
	}

	// 換一個人就過得去（鑑別力）。
	ok, err = repo.AllowManualProcess(ctx, opID, op.Version, 4202, now)
	if err != nil || !ok {
		t.Fatalf("第二個人應可解鎖：ok=%v err=%v", ok, err)
	}
	after, err := repo.GetByID(ctx, opID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if after.Status != "MANUAL_PROCESS_ALLOWED" {
		t.Fatalf("狀態應為 MANUAL_PROCESS_ALLOWED，實得 %q", after.Status)
	}
}

// ---------------------------------------------------------------------------
// ⑤ invoice_provider_cutovers：uq_invpc_open_per_account
// ---------------------------------------------------------------------------

func TestIntegration_ProviderCutover_OnlyOneOpenRowPerAccount(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewProviderCutoverRepository(pool)
	issuerID := seedIssuerFor(ctx, t, pool, nextID()%1_000_000_000)
	accountID := seedProviderAccountRow(ctx, t, pool, issuerID)
	now := time.Now().UTC()

	first, err := repo.Create(ctx, domain.NewProviderCutover{
		ProviderAccountID: accountID, IssuerID: issuerID, AccountVersion: 7,
		CutoverAt: now.Add(time.Hour), CreatedByAccountID: 2685914682, Now: now,
	})
	if err != nil {
		t.Fatalf("第一列 Create: %v", err)
	}
	if first.DeactivatedAt != nil {
		t.Fatal("剛建立的那一列必須是現行有效")
	}

	// 沒先關舊列就插第二列 ⇒ 23505（這正是 Activate 必須 CloseOpen 的理由）。
	if _, err := repo.Create(ctx, domain.NewProviderCutover{
		ProviderAccountID: accountID, IssuerID: issuerID, AccountVersion: 8,
		CutoverAt: now.Add(2 * time.Hour), CreatedByAccountID: 2685914682, Now: now,
	}); err == nil {
		t.Fatal("同時只能有一列現行有效，第二列必須被 uq_invpc_open_per_account 擋下")
	}
}

func TestIntegration_ProviderCutover_CloseOpenThenCreate(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	repo := NewProviderCutoverRepository(pool)
	issuerID := seedIssuerFor(ctx, t, pool, nextID()%1_000_000_000)
	accountID := seedProviderAccountRow(ctx, t, pool, issuerID)
	now := time.Now().UTC()

	// 第一次：沒有前一列 ⇒ CloseOpen rows=0，**那不是錯誤**。
	n, err := repo.CloseOpen(ctx, accountID, now)
	if err != nil {
		t.Fatalf("CloseOpen: %v", err)
	}
	if n != 0 {
		t.Fatalf("第一次設定時本來就沒有前一列，實得 %d", n)
	}
	if _, err := repo.Create(ctx, domain.NewProviderCutover{
		ProviderAccountID: accountID, IssuerID: issuerID, AccountVersion: 7,
		CutoverAt: now.Add(time.Hour), CreatedByAccountID: 2685914682, Now: now,
	}); err != nil {
		t.Fatalf("第一列 Create: %v", err)
	}

	later := now.Add(time.Minute)
	if n, err = repo.CloseOpen(ctx, accountID, later); err != nil || n != 1 {
		t.Fatalf("第二次應關掉一列：n=%d err=%v", n, err)
	}
	if _, err := repo.Create(ctx, domain.NewProviderCutover{
		ProviderAccountID: accountID, IssuerID: issuerID, AccountVersion: 8,
		CutoverAt: later.Add(2 * time.Hour), CreatedByAccountID: 2685914682, Now: later,
	}); err != nil {
		t.Fatalf("關舊列之後第二列應建得起來：%v", err)
	}

	rows, err := repo.ListForAdmin(ctx, accountID, domain.Page{Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("ListForAdmin: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("應有兩列，實得 %d", len(rows))
	}
	// 新到舊：現行那一列在最前面（ADM 一進頁面看到的就是現在生效的切點）。
	if rows[0].DeactivatedAt != nil || rows[1].DeactivatedAt == nil {
		t.Fatalf("排序或關列狀態不對：%+v", rows)
	}
	total, err := repo.CountForAdmin(ctx, accountID)
	if err != nil || total != 2 {
		t.Fatalf("CountForAdmin：total=%d err=%v", total, err)
	}
}

// chk_invpc_mode 只有一個值是刻意的：第二個值一旦出現就代表本表的語意變了。
func TestIntegration_ProviderCutover_ModeIsConstrained(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	issuerID := seedIssuerFor(ctx, t, pool, nextID()%1_000_000_000)
	accountID := seedProviderAccountRow(ctx, t, pool, issuerID)

	if _, err := pool.Exec(ctx,
		`INSERT INTO invoice_provider_cutovers
		   (provider_account_id, issuer_id, account_version, cutover_at, mode, created_by_account_id)
		 VALUES ($1, $2, 1, now(), 'LEGACY_MANUAL', 2685914682)`,
		accountID, issuerID); err == nil {
		t.Fatal("chk_invpc_mode 必須擋下 AMEGO_AUTO 以外的值")
	}
}
