//go:build integration

// 本檔跑真的 SQL，驗 B07 寫入面（§7.3.5 K01／K07／K11／K13）。
//
// 這些規則**沒有一條**是單元測試驗得到的：ON CONFLICT DO NOTHING 的 rows=0、
// 四條 CAS 的述詞邊界、以及 chk_invcomp_resolved_pair 這條雙向等式——K13 少寫
// 一句 `resolved_at = NULL` 會當場 23514，而 fake 不會有 CHECK。
package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

const (
	iBuyerA int64 = 2623900520
	iBuyerB int64 = 2688637900
)

func newComplaintFixture(t *testing.T) (*ComplaintRepository, context.Context, int64, int64, string) {
	t.Helper()
	ctx := context.Background()
	pool := testPool(t)
	issuerID := seedIssuerFor(ctx, t, pool, 2685914682)
	orderNo := fmt.Sprintf("O%d", nextID()%1_000_000_000_000)
	docID := seedOrderDocument(ctx, t, pool, issuerID, iBuyerA, orderNo, "ISSUED", 1, time.Now().UTC())
	return NewComplaintRepository(pool), ctx, issuerID, docID, orderNo
}

func newComplaintInput(id, issuerID, docID, buyerID int64, orderNo string, now time.Time) domain.NewComplaint {
	return domain.NewComplaint{
		ID: id, DocumentID: docID, IssuerID: issuerID, OrderNo: orderNo,
		ComplainantBuyerID: buyerID, ReasonCode: "NOT_RECEIVED", Description: "沒收到",
		SlaDueAt: now.Add(72 * time.Hour), Now: now,
	}
}

// K01：建案落地，且同一份 document 的第二件案子被 uq_invcomp_open_per_document 擋下。
//
// 🔴 擋下的形狀必須是 inserted=false 而不是 error：ON CONFLICT DO NOTHING 若指名了
// conflict target，衝突會變成 23505 而整個交易 aborted（25P02），之後那句「查既有案件」
// 的 SELECT 會炸成 500，而正確行為是回 9310 並導向既有案件。
func TestIntegration_Complaint_CreateAndOpenPerDocumentUniqueness(t *testing.T) {
	repo, ctx, issuerID, docID, orderNo := newComplaintFixture(t)
	now := time.Now().UTC()

	first, inserted, err := repo.Create(ctx, newComplaintInput(nextID(), issuerID, docID, iBuyerA, orderNo, now))
	if err != nil || !inserted {
		t.Fatalf("第一件應建立成功：inserted=%v err=%v", inserted, err)
	}
	if first.Status != "OPEN" || first.Version != 0 {
		t.Fatalf("建案後 status=%s version=%d，want OPEN/0", first.Status, first.Version)
	}
	if !first.SlaDueAt.Equal(now.Add(72*time.Hour).Truncate(time.Microsecond)) &&
		first.SlaDueAt.Sub(now.Add(72*time.Hour)).Abs() > time.Millisecond {
		t.Fatalf("sla_due_at = %s，want ≈ %s", first.SlaDueAt, now.Add(72*time.Hour))
	}

	_, inserted, err = repo.Create(ctx, newComplaintInput(nextID(), issuerID, docID, iBuyerA, orderNo, now))
	if err != nil {
		t.Fatalf("重複建案必須是 inserted=false 而不是 error（error 會讓交易 aborted）：%v", err)
	}
	if inserted {
		t.Fatal("同一份 document 建出了第二件未結案申訴 —— uq_invcomp_open_per_document 沒有生效")
	}
}

// K07：三條述詞（狀態、買家、版本）逐條驗，任一不成立即 rows=0。
func TestIntegration_Complaint_EscalateByBuyerGuards(t *testing.T) {
	repo, ctx, issuerID, docID, orderNo := newComplaintFixture(t)
	now := time.Now().UTC()

	row, _, err := repo.Create(ctx, newComplaintInput(nextID(), issuerID, docID, iBuyerA, orderNo, now))
	if err != nil {
		t.Fatal(err)
	}

	// 狀態不對（建案後是 OPEN，K07 只收 SELLER_RESPONDED）。
	if ok, err := repo.EscalateByBuyer(ctx, row.ID, iBuyerA, 0, now); err != nil || ok {
		t.Fatalf("OPEN 不該可以升級：ok=%v err=%v", ok, err)
	}

	// 推進到 SELLER_RESPONDED 再驗買家與版本。
	mustExec(ctx, t, repo.pool,
		`UPDATE invoice_complaints SET status='SELLER_RESPONDED', buyer_review_due_at=now()+interval '72 hours', version=version+1 WHERE id=$1`,
		row.ID)

	// 🔴 買家不對 ⇒ rows=0。這一條是 Tier A 的第二道防線：呼叫端的 GetForBuyer 若被
	// 改成 GetByID，這句 UPDATE 仍然擋得住。
	if ok, err := repo.EscalateByBuyer(ctx, row.ID, iBuyerB, 1, now); err != nil || ok {
		t.Fatalf("別的買家不該可以升級：ok=%v err=%v", ok, err)
	}
	// 版本不對 ⇒ rows=0。
	if ok, err := repo.EscalateByBuyer(ctx, row.ID, iBuyerA, 0, now); err != nil || ok {
		t.Fatalf("舊版本不該可以升級：ok=%v err=%v", ok, err)
	}
	// 三條都對 ⇒ rows=1。
	if ok, err := repo.EscalateByBuyer(ctx, row.ID, iBuyerA, 1, now); err != nil || !ok {
		t.Fatalf("合法升級失敗：ok=%v err=%v", ok, err)
	}
	after, err := repo.GetForBuyer(ctx, row.ID, iBuyerA)
	if err != nil {
		t.Fatal(err)
	}
	if after.Status != "ESCALATED_TO_ADM" || after.Version != 2 {
		t.Fatalf("轉移後 status=%s version=%d", after.Status, after.Version)
	}
}

// K11：撤回**不寫 resolved_at**（chk_invcomp_resolved_pair 只綁 RESOLVED／VIOLATION_CONFIRMED，
// 寫了反而 23514），且撤回後該 document 可以再建一件（WITHDRAWN 不在唯一索引的述詞內）。
func TestIntegration_Complaint_WithdrawLeavesResolvedAtNullAndFreesTheDocument(t *testing.T) {
	repo, ctx, issuerID, docID, orderNo := newComplaintFixture(t)
	now := time.Now().UTC()

	row, _, err := repo.Create(ctx, newComplaintInput(nextID(), issuerID, docID, iBuyerA, orderNo, now))
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := repo.Withdraw(ctx, row.ID, iBuyerB, 0, now); err != nil || ok {
		t.Fatalf("別的買家不該可以撤回：ok=%v err=%v", ok, err)
	}
	if ok, err := repo.Withdraw(ctx, row.ID, iBuyerA, 0, now); err != nil || !ok {
		t.Fatalf("撤回失敗：ok=%v err=%v", ok, err)
	}

	after, err := repo.GetForBuyer(ctx, row.ID, iBuyerA)
	if err != nil {
		t.Fatal(err)
	}
	if after.Status != "WITHDRAWN" {
		t.Fatalf("status = %s", after.Status)
	}
	if after.ResolvedAt != nil {
		t.Fatal("撤回不得寫 resolved_at")
	}

	n, err := repo.CountWithdrawnByDocument(ctx, docID)
	if err != nil || n != 1 {
		t.Fatalf("CountWithdrawnByDocument = %d, err=%v, want 1", n, err)
	}
	// 撤回之後名額釋放：誤開案要的正是這個行為（配額由 K01 守衛②另外守）。
	if _, inserted, err := repo.Create(ctx, newComplaintInput(nextID(), issuerID, docID, iBuyerA, orderNo, now)); err != nil || !inserted {
		t.Fatalf("撤回後應可重新申訴：inserted=%v err=%v", inserted, err)
	}
}

// K13：重開必須在同一句 UPDATE 內清空 resolved_at。
//
// 🔴 這是本檔最重要的一支：少寫 `resolved_at = NULL`，每一次重開都會以 23514 失敗，
// 而 fake 沒有 CHECK ⇒ 單元測試會全綠。
func TestIntegration_Complaint_ReopenClearsResolvedAtAndEnforcesQuota(t *testing.T) {
	repo, ctx, issuerID, docID, orderNo := newComplaintFixture(t)
	now := time.Now().UTC()

	row, _, err := repo.Create(ctx, newComplaintInput(nextID(), issuerID, docID, iBuyerA, orderNo, now))
	if err != nil {
		t.Fatal(err)
	}
	// 模擬 K06：進 RESOLVED 時同時寫 resolved_at 與 reopen_allowed_after（冷卻已過）。
	toResolved := func(reopenCount int) {
		mustExec(ctx, t, repo.pool,
			`UPDATE invoice_complaints
			    SET status='RESOLVED', resolved_at=now(),
			        reopen_allowed_after=now()-interval '1 hour',
			        reopen_count=$2, version=version+1
			  WHERE id=$1`, row.ID, reopenCount)
	}

	toResolved(0)
	current, err := repo.GetForBuyer(ctx, row.ID, iBuyerA)
	if err != nil {
		t.Fatal(err)
	}

	// 冷卻未到 ⇒ rows=0。
	mustExec(ctx, t, repo.pool,
		`UPDATE invoice_complaints SET reopen_allowed_after=now()+interval '1 hour', version=version+1 WHERE id=$1`, row.ID)
	if ok, err := repo.Reopen(ctx, row.ID, iBuyerA, current.Version+1, now.Add(72*time.Hour), now); err != nil || ok {
		t.Fatalf("冷卻未到不該可以重開：ok=%v err=%v", ok, err)
	}

	toResolved(0)
	current, err = repo.GetForBuyer(ctx, row.ID, iBuyerA)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := repo.Reopen(ctx, row.ID, iBuyerA, current.Version, now.Add(72*time.Hour), now); err != nil || !ok {
		t.Fatalf("合法重開失敗（少寫 resolved_at=NULL 會在這裡以 23514 出現）：ok=%v err=%v", ok, err)
	}
	after, err := repo.GetForBuyer(ctx, row.ID, iBuyerA)
	if err != nil {
		t.Fatal(err)
	}
	if after.Status != "OPEN" || after.ResolvedAt != nil || after.ReopenCount != 1 || after.ReopenAllowedAfter != nil {
		t.Fatalf("重開後 status=%s resolved_at=%v reopen_count=%d reopen_allowed_after=%v",
			after.Status, after.ResolvedAt, after.ReopenCount, after.ReopenAllowedAfter)
	}

	// 次數用完（chk_invcomp_reopen 上限 2）⇒ rows=0。
	toResolved(2)
	current, err = repo.GetForBuyer(ctx, row.ID, iBuyerA)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := repo.Reopen(ctx, row.ID, iBuyerA, current.Version, now.Add(72*time.Hour), now); err != nil || ok {
		t.Fatalf("已達 2 次上限不該可以重開：ok=%v err=%v", ok, err)
	}
}

// 時間軸：買家事件的 action 必須是 NULL，而 actor_account_id 不得為空
// （chk_invcompev_actor_id：actor_type <> 'SYSTEM' ⇒ 非空）。
func TestIntegration_Complaint_AppendEventKeepsActionNullForBuyer(t *testing.T) {
	repo, ctx, issuerID, docID, orderNo := newComplaintFixture(t)
	now := time.Now().UTC()

	row, _, err := repo.Create(ctx, newComplaintInput(nextID(), issuerID, docID, iBuyerA, orderNo, now))
	if err != nil {
		t.Fatal(err)
	}
	ev, err := repo.AppendEvent(ctx, domain.NewComplaintEvent{
		ComplaintID: row.ID, EventType: "CREATED", ActorType: "BUYER",
		ActorAccountID: iBuyerA, Message: "沒收到", Now: now,
	})
	if err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}
	if ev.Action != nil {
		t.Fatalf("買家事件的 action 必須是 NULL，實際 %q", *ev.Action)
	}
	if ev.ActorAccountID == nil || *ev.ActorAccountID != iBuyerA {
		t.Fatalf("actor_account_id = %v", ev.ActorAccountID)
	}
	events, err := repo.ListEvents(ctx, row.ID)
	if err != nil || len(events) != 1 {
		t.Fatalf("ListEvents = %d 列 err=%v", len(events), err)
	}
}
