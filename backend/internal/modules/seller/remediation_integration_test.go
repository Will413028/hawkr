//go:build integration

package seller_test

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
	"github.com/jackc/pgx/v5/pgconn"
)

const oneDay = 24 * time.Hour

// riskMeasure 建立一個依據風險類型的停權（複審期限七天後）；ends 為 nil 是沒有固定期間。
func (f *fixture) riskMeasure(t *testing.T, storeID int64, ends *time.Time) Measure {
	t.Helper()
	review := f.now.Add(7 * oneDay)
	r, err := f.module.CreateMeasure(context.Background(), CreateMeasureInput{
		AdminID: 7, StoreID: storeID, Kind: MeasureSuspension, StatementToSeller: "疑似詐欺，請依通知補正", Note: "內部紀錄", ReviewBy: &review,
		EndsAt: ends, Risk: &RiskBasis{Ground: RiskFraud},
	})
	must(t, err)
	return r.Measure
}

// remediable 建立帳號、店家與一個沒有固定期間的 RISK 停權。
func (f *fixture) remediable(t *testing.T) (int64, Store, Measure) {
	t.Helper()
	accountID, s := f.open(t)
	return accountID, s, f.riskMeasure(t, s.StoreID, nil)
}

// sellerView 讀賣家看到的措施。
func (f *fixture) sellerView(t *testing.T, accountID int64, id uuid.UUID) Measure {
	t.Helper()
	m, err := f.module.AccountMeasure(context.Background(), accountID, id)
	must(t, err)
	return m
}

func (f *fixture) remediationInput(t *testing.T, accountID int64, id uuid.UUID, reason string, tokens ...string) RequestRemediationInput {
	t.Helper()
	return RequestRemediationInput{
		AccountID: accountID, MeasureID: id, ExpectedVersion: f.sellerView(t, accountID, id).SellerVersion(), Reason: reason, EvidenceTokens: tokens,
	}
}

// remediate 以賣家讀到的版本請求解除，回傳後台看到的措施。
func (f *fixture) remediate(t *testing.T, accountID int64, id uuid.UUID, reason string, tokens ...string) Measure {
	t.Helper()
	_, err := f.module.RequestRemediation(context.Background(), f.remediationInput(t, accountID, id, reason, tokens...))
	must(t, err)
	return f.measureOf(t, id)
}

func (f *fixture) decideRemediation(id uuid.UUID, adminID int64, outcome RequestState, reason string) (Measure, error) {
	current, err := f.module.Measure(context.Background(), id)
	if err != nil {
		return Measure{}, err
	}
	if outcome == RequestAccepted {
		f.afterCreation(current)
	}
	return f.module.DecideRemediation(context.Background(), DecideRemediationInput{
		MeasureID: id, AdminID: adminID, ExpectedVersion: current.Version, Outcome: outcome, Reason: reason,
	})
}

func (f *fixture) decideAppeal(t *testing.T, id uuid.UUID, outcome RequestState, reason string) Measure {
	t.Helper()
	current := f.measureOf(t, id)
	m, err := f.module.DecideAppeal(context.Background(), DecideAppealInput{
		MeasureID: id, AdminID: 9, ExpectedVersion: current.Version, Outcome: outcome, Reason: reason,
	})
	must(t, err)
	return m
}

func wantFailure(t *testing.T, label string, err error, kind apperr.Kind) {
	t.Helper()
	if apperr.KindOf(err) != kind {
		t.Fatalf("%s：err=%v，want kind %v", label, err, kind)
	}
}

// 提出解除請求：後台的版本加一、賣家的 etag 不變（所以重送回 failed_precondition）；發 MeasureRequestSubmitted{REMEDIATION}；佐證與申覆共用。
func TestRequestRemediationSubmitsAPendingRequest(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	submitted := record[MeasureRequestSubmitted](f)
	accountID, _, m := f.remediable(t)
	before := f.sellerView(t, accountID, m.ID)
	if !before.CanRequestRemediation() || before.CanDecideRemediation() || len(remediationsOf(before)) != 0 || before.NextRemediationTime() != nil || before.RejectionCooldown() != 0 {
		t.Fatalf("RISK 生效中：可以補正、沒有紀錄、沒有冷卻：%+v", before)
	}

	in := f.remediationInput(t, accountID, m.ID, " 已改善並附上佐證 ", "r1", "r2")
	got, err := f.module.RequestRemediation(ctx, in)
	must(t, err)
	pending := got.PendingRemediation()
	if pending == nil || pending.Kind != RequestKindRemediation || pending.State != RequestPending || pending.Reason != "已改善並附上佐證" ||
		got.Appeal() != nil || got.CanRequestRemediation() || got.CanAppeal() || got.SellerVersion() != before.SellerVersion() {
		t.Fatalf("got=%+v", got)
	}
	admin := f.measureOf(t, m.ID)
	if admin.Version != m.Version+1 || !admin.CanDecideRemediation() || admin.CanEnd() || len(remediationsOf(admin)) != 1 ||
		len(remediationsOf(admin)[0].Evidence) != 2 || remediationsOf(admin)[0].Revision != 1 {
		t.Fatalf("admin=%+v", admin)
	}
	if events := submitted.all(); len(events) != 1 || events[0].Kind != RequestKindRemediation || events[0].RequestID != pending.ID || events[0].AccountID != accountID {
		t.Fatalf("submitted=%+v", events)
	}
	// 賣家 etag 沒變，所以請求生效後再送一次（含重送）是 failed_precondition，不是 VERSION_CONFLICT。
	_, err = f.module.RequestRemediation(ctx, in)
	wantFailure(t, "重送", err, apperr.KindFailedPrecondition)
	if n := f.count(t, "SELECT count(*) FROM seller.measure_requests WHERE measure_id = $1", m.ID); n != 1 {
		t.Fatalf("requests=%d", n)
	}
	// 請求的佐證與申覆同一個機制：以請求 ID 為 ref。
	if n := f.count(t, "SELECT count(*) FROM seller.measure_request_evidence WHERE request_id = $1", pending.ID); n != 2 {
		t.Fatalf("evidence=%d", n)
	}
	f.assertMeasureInvariants(t)
}

// 輸入驗證與歸屬：etag 必填、理由必填且不超過 2000 字、別人的措施是 not_found、舊的 etag 是 aborted（VERSION_CONFLICT）。
func TestRequestRemediationValidatesItsInput(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _, m := f.remediable(t)
	ok := f.remediationInput(t, accountID, m.ID, "已改善")
	for name, mutate := range map[string]func(*RequestRemediationInput){
		"沒有 etag":   func(in *RequestRemediationInput) { in.ExpectedVersion = 0 },
		"空白理由":      func(in *RequestRemediationInput) { in.Reason = "  " },
		"理由超過 2000": func(in *RequestRemediationInput) { in.Reason = strings.Repeat("字", 2001) },
		"六張佐證":      func(in *RequestRemediationInput) { in.EvidenceTokens = []string{"1", "2", "3", "4", "5", "6"} },
	} {
		in := ok
		mutate(&in)
		_, err := f.module.RequestRemediation(ctx, in)
		wantFailure(t, name, err, apperr.KindInvalidInput)
	}
	// 伺服器的錯誤文字原樣顯示給賣家：欄位名稱是「改善說明」，不是內部的「解除請求」。
	blank := ok
	blank.Reason = " "
	_, err := f.module.RequestRemediation(ctx, blank)
	var invalid *apperr.Error
	if !errors.As(err, &invalid) || !strings.Contains(invalid.Message, "改善說明") || strings.Contains(invalid.Message, "補正") {
		t.Fatalf("message=%v", err)
	}
	stranger := ok
	stranger.AccountID = f.account(t)
	_, err = f.module.RequestRemediation(ctx, stranger)
	wantFailure(t, "別人的措施", err, apperr.KindNotFound)
	stale := ok
	stale.ExpectedVersion = ok.ExpectedVersion + 1
	_, err = f.module.RequestRemediation(ctx, stale)
	wantFailure(t, "舊 etag", err, apperr.KindVersionConflict)
	if n := f.count(t, "SELECT count(*) FROM seller.measure_requests"); n != 0 {
		t.Fatalf("被拒絕的請求不能留下：%d", n)
	}
}

// VIOLATION 依據：can_request_remediation 為假、RequestRemediation 是 failed_precondition；資料庫也拒絕違規依據措施上的解除請求，
// 而且請求的 basis 一定跟著措施（複合外鍵）。
func TestRemediationIsOnlyForRiskMeasures(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _, v := f.appealable(t)
	seller := f.sellerView(t, accountID, v.ID)
	if seller.CanRequestRemediation() || seller.RejectionCooldown() != 0 {
		t.Fatalf("違規依據沒有補正：%+v", seller)
	}
	_, err := f.module.RequestRemediation(ctx, f.remediationInput(t, accountID, v.ID, "已改善"))
	wantFailure(t, "違規依據", err, apperr.KindFailedPrecondition)

	insert := `INSERT INTO seller.measure_requests (id, measure_id, kind, state, reason, revision, submitted_at, basis)
		VALUES (uuidv7(), $1, $2, 'PENDING', 'x', 1, now(), $3)`
	constraint := func(err error) string {
		var pg *pgconn.PgError
		if errors.As(err, &pg) {
			return pg.ConstraintName
		}
		return ""
	}
	_, err = f.db.Conn(ctx).Exec(ctx, insert, v.ID, "REMEDIATION", "VIOLATION")
	if got := constraint(err); got != "measure_requests_remediation_basis_check" {
		t.Fatalf("違規依據的補正要被 CHECK 擋下：%v", err)
	}
	_, err = f.db.Conn(ctx).Exec(ctx, insert, v.ID, "REMEDIATION", "RISK")
	if got := constraint(err); got != "measure_requests_measure_basis_fkey" {
		t.Fatalf("謊報 basis 要被複合外鍵擋下：%v", err)
	}
	_, err = f.db.Conn(ctx).Exec(ctx, insert, v.ID, "APPEAL", "RISK")
	if got := constraint(err); got != "measure_requests_measure_basis_fkey" {
		t.Fatalf("申訴的 basis 也要跟著措施：%v", err)
	}
}

// 資料庫保證每個措施同時至多一筆待處理的請求，不分種類。一個連線先寫入待處理的申覆（尚未 commit），另外兩個連線各自在自己的
// transaction 內同時寫入待處理的解除請求（繞過應用程式的鎖）：commit 後兩個都一定失敗。
func TestOnePendingRequestPerMeasureAcrossKindsIsEnforcedByTheDatabase(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, _, m := f.remediable(t)
	insert := `INSERT INTO seller.measure_requests (id, measure_id, kind, state, reason, revision, submitted_at, basis)
		VALUES (uuidv7(), $1, $2, 'PENDING', 'x', 1, now(), 'RISK')`
	first, err := f.db.Pool().Begin(ctx)
	must(t, err)
	defer func() { _ = first.Rollback(ctx) }()
	_, err = first.Exec(ctx, insert, m.ID, "APPEAL")
	must(t, err)
	errs := make(chan error, 2)
	for range 2 {
		go func() {
			tx, err := f.db.Pool().Begin(ctx)
			if err != nil {
				errs <- err
				return
			}
			defer func() { _ = tx.Rollback(ctx) }()
			if _, err := tx.Exec(ctx, insert, m.ID, "REMEDIATION"); err != nil {
				errs <- err
				return
			}
			errs <- tx.Commit(ctx)
		}()
	}
	f.waitForLockWaiters(t, 2)
	must(t, first.Commit(ctx))
	for range 2 {
		var pg *pgconn.PgError
		if err := <-errs; !errors.As(err, &pg) || pg.ConstraintName != "measure_requests_one_pending" {
			t.Fatalf("同時的第二筆待處理請求要被 measure_requests_one_pending 擋下：%v", err)
		}
	}
	if n := f.count(t, "SELECT count(*) FROM seller.measure_requests WHERE measure_id = $1 AND state = 'PENDING'", m.ID); n != 1 {
		t.Fatalf("pending=%d", n)
	}
}

// 應用程式層的同時提出：申覆與解除請求混著同時送出，帳號鎖串行，剛好一個成功，其餘是 failed_precondition。
func TestConcurrentRequestsOfMixedKindsLeaveExactlyOnePending(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s, m := f.remediable(t)
	remediation := f.remediationInput(t, accountID, m.ID, "已改善")
	appeal := SubmitAppealInput{AccountID: accountID, MeasureID: m.ID, ExpectedVersion: remediation.ExpectedVersion, Reason: "申訴"}
	holder, err := f.db.Pool().Begin(ctx)
	must(t, err)
	_, err = holder.Exec(ctx, "SELECT 1 FROM seller.stores WHERE store_id=$1 FOR UPDATE", s.StoreID)
	must(t, err)
	errs := make(chan error, 4)
	var wg sync.WaitGroup
	for i := range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if i%2 == 0 {
				_, err := f.module.RequestRemediation(ctx, remediation)
				errs <- err
				return
			}
			_, err := f.module.SubmitAppeal(ctx, appeal)
			errs <- err
		}()
	}
	f.waitForLockWaiters(t, 4)
	must(t, holder.Rollback(ctx))
	wg.Wait()
	close(errs)
	succeeded := 0
	for err := range errs {
		if err == nil {
			succeeded++
			continue
		}
		wantFailure(t, "輸家", err, apperr.KindFailedPrecondition)
	}
	if succeeded != 1 {
		t.Fatalf("succeeded=%d", succeeded)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.measure_requests WHERE measure_id = $1 AND state = 'PENDING'", m.ID); n != 1 {
		t.Fatalf("pending=%d", n)
	}
}

// 接受解除請求：寫一版 ends_at = 現在（作者是裁定的管理員、statement 不變）、措施結束（ENDED）、不是撤銷，兩個 etag 都變；發 MeasureEnded、
// MeasureRequestDecided{REMEDIATION, ACCEPTED}、StoreUnsuspended；仍可申覆；不經 CanEnd。
func TestAcceptingARemediationEndsTheMeasureWithoutRevokingIt(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	ended := record[MeasureEnded](f)
	decided := record[MeasureRequestDecided](f)
	revoked := record[MeasureRevoked](f)
	unsuspended := record[StoreUnsuspended](f)
	accountID, s, m := f.remediable(t)
	f.remediate(t, accountID, m.ID, "已改善")
	pending := f.measureOf(t, m.ID)
	if pending.CanEnd() {
		t.Fatal("補正待處理時 can_end 為假")
	}
	if _, err := f.module.EndMeasure(ctx, EndMeasureInput{MeasureID: m.ID, AdminID: 9, ExpectedVersion: pending.Version}); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("補正待處理時 End 要 failed_precondition：%v", err)
	}
	sellerBefore := f.sellerView(t, accountID, m.ID)
	f.now = f.now.Add(time.Hour)
	accepted, err := f.decideRemediation(m.ID, 11, RequestAccepted, "已確認改善")
	must(t, err)
	if accepted.Status != MeasureStatusEnforced || accepted.RevokedReason != nil || accepted.State != StateEnded || accepted.EndsAt == nil || !accepted.EndsAt.Equal(f.now) ||
		accepted.StatementToSeller != pending.StatementToSeller || accepted.Version != pending.Version+1 || accepted.CurrentRevision != pending.CurrentRevision+1 ||
		accepted.PendingRemediation() != nil || len(accepted.Findings) != 0 {
		t.Fatalf("accepted=%+v", accepted)
	}
	r := remediationsOf(accepted)[0]
	if r.State != RequestAccepted || *r.DecisionReason != "已確認改善" || *r.DecidedByAdminID != 11 || r.CloseCause != nil || r.Revision != 1 {
		t.Fatalf("request=%+v", r)
	}
	last := accepted.Revisions[len(accepted.Revisions)-1]
	if last.AdminID == nil || *last.AdminID != 11 || last.StatementToSeller != pending.StatementToSeller || last.EndsAt == nil || !last.EndsAt.Equal(f.now) || last.Revision != accepted.CurrentRevision {
		t.Fatalf("revision=%+v", last)
	}
	sellerAfter := f.sellerView(t, accountID, m.ID)
	if sellerAfter.SellerVersion() == sellerBefore.SellerVersion() || sellerAfter.State != StateEnded || !sellerAfter.CanAppeal() || sellerAfter.CanRequestRemediation() || sellerAfter.CanDecideRemediation() {
		t.Fatalf("賣家：etag 變、已結束、仍可申訴：%+v", sellerAfter)
	}
	if f.isSuspended(t, s.StoreID) || len(unsuspended.all()) != 1 {
		t.Fatalf("效果解除：suspended=%v events=%+v", f.isSuspended(t, s.StoreID), unsuspended.all())
	}
	if got := ended.all(); len(got) != 1 || got[0].MeasureID != m.ID || got[0].AccountID != accountID {
		t.Fatalf("ended=%+v", got)
	}
	if got := decided.all(); len(got) != 1 || got[0].Kind != RequestKindRemediation || got[0].Outcome != RequestAccepted || got[0].RequestID != r.ID {
		t.Fatalf("decided=%+v", got)
	}
	if len(revoked.all()) != 0 {
		t.Fatal("接受補正不是撤銷，不發 MeasureRevoked")
	}
	appealed := f.appeal(t, accountID, sellerAfter, "對已結束的措施仍可申訴")
	if appealed.Appeal() == nil {
		t.Fatal("接受補正後仍可申訴")
	}
	f.assertMeasureInvariants(t)
}

// 駁回解除請求：只有後台的 etag 變、賣家的不變、措施不變；附理由；可以再請求解除；請求留在紀錄裡（新到舊）。
func TestRejectingARemediationChangesOnlyTheAdminEtagAndAllowsAnotherRemediation(t *testing.T) {
	t.Parallel()
	f := setup(t)
	decided := record[MeasureRequestDecided](f)
	ended := record[MeasureEnded](f)
	accountID, _, m := f.remediable(t)
	f.remediate(t, accountID, m.ID, "第一次")
	pending := f.measureOf(t, m.ID)
	sellerBefore := f.sellerView(t, accountID, m.ID)
	f.now = f.now.Add(time.Hour)
	rejected, err := f.decideRemediation(m.ID, 11, RequestRejected, "佐證不足")
	must(t, err)
	if rejected.Version != pending.Version+1 || rejected.CurrentRevision != pending.CurrentRevision || rejected.State != StateActive || rejected.EndsAt != nil ||
		remediationsOf(rejected)[0].State != RequestRejected || len(ended.all()) != 0 {
		t.Fatalf("rejected=%+v", rejected)
	}
	if got := f.sellerView(t, accountID, m.ID); got.SellerVersion() != sellerBefore.SellerVersion() || !got.CanRequestRemediation() || got.NextRemediationTime() != nil {
		t.Fatalf("賣家 etag 不變、可以再補正：%+v", got)
	}
	f.remediate(t, accountID, m.ID, "第二次")
	view := f.sellerView(t, accountID, m.ID)
	if got := remediationsOf(view); len(got) != 2 || got[0].Reason != "第二次" || got[0].State != RequestPending || got[1].Reason != "第一次" || *got[1].DecisionReason != "佐證不足" {
		t.Fatalf("新到舊：%+v", got)
	}
	if got := decided.all(); len(got) != 1 || got[0].Kind != RequestKindRemediation || got[0].Outcome != RequestRejected {
		t.Fatalf("decided=%+v", got)
	}
}

func TestDecideRemediationRules(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _, m := f.remediable(t)
	admin := f.measureOf(t, m.ID)
	_, err := f.module.DecideRemediation(ctx, DecideRemediationInput{MeasureID: m.ID, AdminID: 9, ExpectedVersion: admin.Version, Outcome: RequestRejected, Reason: "x"})
	wantFailure(t, "沒有待處理的補正", err, apperr.KindFailedPrecondition)
	f.remediate(t, accountID, m.ID, "已改善")
	admin = f.measureOf(t, m.ID)
	for name, in := range map[string]DecideRemediationInput{
		"沒有 etag":    {MeasureID: m.ID, AdminID: 9, Outcome: RequestRejected, Reason: "x"},
		"沒有理由":       {MeasureID: m.ID, AdminID: 9, ExpectedVersion: admin.Version, Outcome: RequestAccepted},
		"裁定是待處理":     {MeasureID: m.ID, AdminID: 9, ExpectedVersion: admin.Version, Outcome: RequestPending, Reason: "x"},
		"裁定是 CLOSED": {MeasureID: m.ID, AdminID: 9, ExpectedVersion: admin.Version, Outcome: RequestClosed, Reason: "x"},
		"沒有裁定":       {MeasureID: m.ID, AdminID: 9, ExpectedVersion: admin.Version, Reason: "x"},
		"沒有管理員":      {MeasureID: m.ID, ExpectedVersion: admin.Version, Outcome: RequestRejected, Reason: "x"},
	} {
		_, err := f.module.DecideRemediation(ctx, in)
		wantFailure(t, name, err, apperr.KindInvalidInput)
	}
	_, err = f.module.DecideRemediation(ctx, DecideRemediationInput{MeasureID: m.ID, AdminID: 9, ExpectedVersion: admin.Version + 1, Outcome: RequestRejected, Reason: "x"})
	wantFailure(t, "舊 etag", err, apperr.KindVersionConflict)
	_, err = f.decideRemediation(m.ID, 9, RequestRejected, "佐證不足")
	must(t, err)
	// 裁定已經生效後帶舊 etag 再送一次：VERSION_CONFLICT，不寫入。
	_, err = f.module.DecideRemediation(ctx, DecideRemediationInput{MeasureID: m.ID, AdminID: 9, ExpectedVersion: admin.Version, Outcome: RequestRejected, Reason: "佐證不足"})
	wantFailure(t, "重送", err, apperr.KindVersionConflict)
	_, err = f.module.DecideRemediation(ctx, DecideRemediationInput{MeasureID: uuid.NewV7(), AdminID: 9, ExpectedVersion: 1, Outcome: RequestRejected, Reason: "x"})
	wantFailure(t, "不存在", err, apperr.KindNotFound)
}

// 撤銷時待處理的解除請求以 CLOSED（REVOKED）結案：帶撤銷的管理員；發 MeasureRequestClosed{Cause: REVOKED}；解除請求不擋撤銷。
func TestRevokingAMeasureClosesAPendingRemediationAsRevoked(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	closed := record[MeasureRequestClosed](f)
	accountID, _, m := f.remediable(t)
	f.remediate(t, accountID, m.ID, "已改善")
	admin := f.measureOf(t, m.ID)
	if !admin.CanRevoke() {
		t.Fatal("待處理的補正不擋撤銷")
	}
	revoked, err := f.module.RevokeMeasure(ctx, RevokeMeasureInput{MeasureID: m.ID, ExpectedVersion: admin.Version, AdminID: 12, Reason: RevokeAdminCorrection})
	must(t, err)
	r := remediationsOf(revoked)[0]
	if r.State != RequestClosed || r.CloseCause == nil || *r.CloseCause != CloseCauseRevoked || r.DecidedByAdminID == nil || *r.DecidedByAdminID != 12 || r.DecisionReason != nil {
		t.Fatalf("request=%+v", r)
	}
	if got := closed.all(); len(got) != 1 || got[0].Kind != RequestKindRemediation || got[0].Cause != CloseCauseRevoked || got[0].RequestID != r.ID || got[0].AccountID != accountID {
		t.Fatalf("closed=%+v", got)
	}
	f.assertMeasureInvariants(t)
}

// W3：待處理的解除請求凍結加重（延長與改理由是 failed_precondition），解除不受限（縮短成功）；縮短到期後，到期 job 把解除請求以 CLOSED（ENDED）
// 結案（系統結案，沒有管理員）。
func TestPendingRemediationFreezesAggravationButNotShortening(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	closed := record[MeasureRequestClosed](f)
	accountID, s := f.open(t)
	m := f.riskMeasure(t, s.StoreID, ptr(f.now.Add(30*oneDay)))
	f.remediate(t, accountID, m.ID, "已改善")
	admin := f.measureOf(t, m.ID)
	_, err := f.module.UpdateMeasure(ctx, UpdateMeasureInput{MeasureID: m.ID, AdminID: 9, ExpectedVersion: admin.Version, ChangeEndsAt: true, EndsAt: ptr(f.now.Add(40 * oneDay))})
	wantFailure(t, "延長", err, apperr.KindFailedPrecondition)
	_, err = f.module.UpdateMeasure(ctx, UpdateMeasureInput{MeasureID: m.ID, AdminID: 9, ExpectedVersion: admin.Version, StatementToSeller: ptr("加重的理由")})
	wantFailure(t, "改理由", err, apperr.KindFailedPrecondition)

	short := f.now.Add(2 * time.Hour)
	shortened, err := f.module.UpdateMeasure(ctx, UpdateMeasureInput{MeasureID: m.ID, AdminID: 9, ExpectedVersion: admin.Version, ChangeEndsAt: true, EndsAt: &short})
	must(t, err)
	if shortened.PendingRemediation() == nil || len(closed.all()) != 0 {
		t.Fatalf("縮短當下補正仍待處理：%+v", shortened.Requests)
	}
	f.now = f.now.Add(3 * time.Hour)
	must(t, f.module.ExpireMeasure(ctx, m.ID, short))
	// 到期 job 只重算停權；請求對外的狀態已是 CLOSED（ENDED、結案時間是措施的結束時間），儲存值與通知等結案掃描。
	got := f.measureOf(t, m.ID)
	r := remediationsOf(got)[0]
	if got.State != StateEnded || r.State != RequestClosed || r.CloseCause == nil || *r.CloseCause != CloseCauseEnded || r.DecidedByAdminID != nil || r.DecidedAt == nil ||
		!r.DecidedAt.Equal(short) || got.Version != shortened.Version || len(closed.all()) != 0 {
		t.Fatalf("got=%+v", got)
	}
	n, err := f.module.CloseEndedMeasureRequests(ctx)
	must(t, err)
	after := f.measureOf(t, m.ID)
	if n != 1 || after.Version != shortened.Version+1 || !slices.EqualFunc(remediationsOf(after), remediationsOf(got), func(a, b MeasureRequest) bool {
		return a.State == b.State && a.DecidedAt.Equal(*b.DecidedAt) && *a.CloseCause == *b.CloseCause
	}) {
		t.Fatalf("n=%d after=%+v", n, after)
	}
	if c := f.count(t, "SELECT count(*) FROM seller.measure_requests WHERE measure_id = $1 AND state = 'CLOSED' AND close_cause = 'ENDED' AND decided_at = $2", m.ID, short); c != 1 {
		t.Fatalf("儲存值要寫成 CLOSED、結案時間是措施的結束時間：%d", c)
	}
	if evs := closed.all(); len(evs) != 1 || evs[0].Cause != CloseCauseEnded || evs[0].Kind != RequestKindRemediation || evs[0].AccountID != accountID {
		t.Fatalf("closed=%+v", evs)
	}
	// job 與掃描重跑是冪等的：不再結案、不再發事件。
	must(t, f.module.ExpireMeasure(ctx, m.ID, short))
	n, err = f.module.CloseEndedMeasureRequests(ctx)
	must(t, err)
	if n != 0 || len(closed.all()) != 1 {
		t.Fatal("重跑不得再發 MeasureRequestClosed")
	}
	f.assertMeasureInvariants(t)
}

// 結案掃描把已結束措施上待處理的解除請求寫成 CLOSED（到期 job 不負責）；掃描前後請求對外的輸出相同（只有後台的版本加一）。
func TestCloseEndedSweepClosesAPendingRemediationOfAnEndedMeasure(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	closed := record[MeasureRequestClosed](f)
	accountID, s := f.open(t)
	short := f.now.Add(2 * time.Hour)
	m := f.riskMeasure(t, s.StoreID, &short)
	f.remediate(t, accountID, m.ID, "已改善")
	f.now = f.now.Add(3 * time.Hour)
	before, sellerBefore := f.measureOf(t, m.ID), f.sellerView(t, accountID, m.ID)
	closedBefore := remediationsOf(before)[0]
	if before.Requests[0].State != RequestPending || closedBefore.State != RequestClosed || len(closed.all()) != 0 {
		t.Fatalf("前提：儲存值是 PENDING、輸出已是 CLOSED：%+v", before.Requests)
	}
	n, err := f.module.CloseEndedMeasureRequests(ctx)
	must(t, err)
	after, sellerAfter := f.measureOf(t, m.ID), f.sellerView(t, accountID, m.ID)
	if n != 1 || after.Requests[0].State != RequestClosed || len(closed.all()) != 1 || after.Version != before.Version+1 || sellerAfter.SellerVersion() != sellerBefore.SellerVersion() {
		t.Fatalf("n=%d after=%+v closed=%+v", n, after.Requests, closed.all())
	}
	if got := remediationsOf(after)[0]; got.State != closedBefore.State || !got.DecidedAt.Equal(*closedBefore.DecidedAt) || *got.CloseCause != *closedBefore.CloseCause ||
		got.DecidedByAdminID != nil || got.DecisionReason != nil {
		t.Fatalf("收斂前後的輸出要相同：before=%+v after=%+v", closedBefore, got)
	}
	if !slices.Equal(sellerAfter.RemediationUnavailableReasons(), sellerBefore.RemediationUnavailableReasons()) || sellerAfter.CanAppeal() != sellerBefore.CanAppeal() {
		t.Fatalf("賣家的 can_* 與原因收斂前後相同：before=%+v after=%+v", sellerBefore, sellerAfter)
	}
}

// A3：措施結束（時間到了、沒有跑任何 job）而解除請求仍待處理：賣家可以申覆（申覆在同一個 transaction 先把解除請求以 CLOSED（ENDED）結案）、
// admin 不能裁定解除請求（failed_precondition）、也不能再請求解除。
func TestAnEndedMeasureWithAStaleRemediationStillAllowsAnAppeal(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	closed := record[MeasureRequestClosed](f)
	submitted := record[MeasureRequestSubmitted](f)
	accountID, s := f.open(t)
	short := f.now.Add(2 * time.Hour)
	m := f.riskMeasure(t, s.StoreID, &short)
	f.remediate(t, accountID, m.ID, "已改善")
	f.now = f.now.Add(3 * time.Hour)

	seller := f.sellerView(t, accountID, m.ID)
	admin := f.measureOf(t, m.ID)
	if seller.State != StateEnded || !seller.CanAppeal() || seller.CanRequestRemediation() || admin.CanDecideRemediation() || admin.PendingRemediation() != nil ||
		admin.MootRemediation() == nil || remediationsOf(seller)[0].State != RequestClosed || remediationsOf(admin)[0].State != RequestClosed ||
		!slices.Equal(seller.RemediationUnavailableReasons(), []RemediationUnavailableReason{ReasonNotActive}) {
		t.Fatalf("儲存值仍是 PENDING，但對外已是 CLOSED、賣家可申訴：seller=%+v", seller)
	}
	_, err := f.module.DecideRemediation(ctx, DecideRemediationInput{MeasureID: m.ID, AdminID: 9, ExpectedVersion: admin.Version, Outcome: RequestAccepted, Reason: "x"})
	wantFailure(t, "已結束的措施不能裁定補正", err, apperr.KindFailedPrecondition)
	_, err = f.module.RequestRemediation(ctx, f.remediationInput(t, accountID, m.ID, "再來一次"))
	wantFailure(t, "已結束的措施不能補正", err, apperr.KindFailedPrecondition)

	appealed, err := f.module.SubmitAppeal(ctx, SubmitAppealInput{AccountID: accountID, MeasureID: m.ID, ExpectedVersion: seller.SellerVersion(), Reason: "措施結束後仍要申訴"})
	must(t, err)
	if appealed.Appeal() == nil || appealed.Appeal().State != RequestPending {
		t.Fatalf("appealed=%+v", appealed)
	}
	r := remediationsOf(f.measureOf(t, m.ID))[0]
	if r.State != RequestClosed || r.CloseCause == nil || *r.CloseCause != CloseCauseEnded || r.DecidedByAdminID != nil {
		t.Fatalf("補正要以 CLOSED（ENDED）結案：%+v", r)
	}
	if got := closed.all(); len(got) != 1 || got[0].Kind != RequestKindRemediation || got[0].Cause != CloseCauseEnded {
		t.Fatalf("closed=%+v", got)
	}
	if !r.DecidedAt.Equal(short) || f.measureOf(t, m.ID).MootRemediation() != nil {
		t.Fatalf("結案時間是措施的結束時間、儲存值已寫回：%+v", r)
	}
	if got := submitted.all(); len(got) != 2 || got[1].Kind != RequestKindAppeal {
		t.Fatalf("submitted=%+v", got)
	}
	f.assertMeasureInvariants(t)
}

// 冷卻：同一措施上被駁回（REJECTED，不分種類，CLOSED 不計）的請求累計超過兩次後，每再被駁回一次就起算一段冷卻：7、14、28、28 天。
// 申覆與解除請求混著算；冷卻中 can_request_remediation 為假、RequestRemediation 是 failed_precondition；冷卻結束就能再提出。
func TestRemediationCooldownTable(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _, m := f.remediable(t)
	reject := func(label string) {
		t.Helper()
		f.now = f.now.Add(time.Minute)
		_, err := f.decideRemediation(m.ID, 9, RequestRejected, label)
		must(t, err)
	}
	check := func(n int, wantNext, wantWarning time.Duration) {
		t.Helper()
		v := f.sellerView(t, accountID, m.ID)
		var decidedAt time.Time
		for _, r := range f.measureOf(t, m.ID).Requests {
			if r.State == RequestRejected && r.DecidedAt.After(decidedAt) {
				decidedAt = *r.DecidedAt
			}
		}
		switch {
		case wantNext == 0 && (v.NextRemediationTime() != nil || !v.CanRequestRemediation()):
			t.Fatalf("n=%d：不在冷卻中：%+v", n, v.NextRemediationTime())
		case wantNext != 0 && (v.NextRemediationTime() == nil || !v.NextRemediationTime().Equal(decidedAt.Add(wantNext)) || v.CanRequestRemediation()):
			t.Fatalf("n=%d：next=%v want=%s", n, v.NextRemediationTime(), decidedAt.Add(wantNext))
		}
		if got := v.RejectionCooldown(); got != wantWarning {
			t.Fatalf("n=%d：RejectionCooldown=%s want=%s", n, got, wantWarning)
		}
		if wantNext != 0 {
			_, err := f.module.RequestRemediation(ctx, f.remediationInput(t, accountID, m.ID, "冷卻中"))
			wantFailure(t, "冷卻中", err, apperr.KindFailedPrecondition)
		}
	}
	check(0, 0, 0)

	f.now = f.now.Add(time.Minute)
	f.remediate(t, accountID, m.ID, "補正一")
	reject("不足")
	check(1, 0, 0)

	f.appeal(t, accountID, f.sellerView(t, accountID, m.ID), "申訴")
	f.now = f.now.Add(time.Minute)
	f.decideAppeal(t, m.ID, RequestRejected, "不成立")
	check(2, 0, 7*oneDay)

	// CLOSED 不計：寫兩筆系統結案的解除請求，次數與冷卻都不變。
	for range 2 {
		_, err := f.db.Conn(ctx).Exec(ctx, `INSERT INTO seller.measure_requests (id, measure_id, kind, state, reason, revision, submitted_at, decided_at, close_cause, basis)
			VALUES (uuidv7(), $1, 'REMEDIATION', 'CLOSED', 'x', 1, $2, $2, 'ENDED', 'RISK')`, m.ID, f.now)
		must(t, err)
	}
	check(2, 0, 7*oneDay)

	f.now = f.now.Add(time.Minute)
	f.remediate(t, accountID, m.ID, "補正二")
	reject("仍不足")
	check(3, 7*oneDay, 14*oneDay)

	for n, step := range []struct{ next, warning time.Duration }{{14 * oneDay, 28 * oneDay}, {28 * oneDay, 28 * oneDay}, {28 * oneDay, 28 * oneDay}} {
		f.now = *f.sellerView(t, accountID, m.ID).NextRemediationTime()
		f.remediate(t, accountID, m.ID, "冷卻後再提出")
		reject("還是不足")
		check(4+n, step.next, step.warning)
	}

	// 冷卻結束的那一刻就可以再提出。
	next := f.sellerView(t, accountID, m.ID).NextRemediationTime()
	f.now = next.Add(-time.Microsecond)
	if f.sellerView(t, accountID, m.ID).CanRequestRemediation() {
		t.Fatal("冷卻結束前一微秒仍在冷卻中")
	}
	f.now = *next
	if !f.sellerView(t, accountID, m.ID).CanRequestRemediation() {
		t.Fatal("冷卻結束的那一刻可以再提出")
	}
	f.assertMeasureInvariants(t)
}

// 讀取單筆措施時，全部請求的佐證一次載入，各自對得上自己的請求。
func TestAdminReadLoadsEveryRequestsEvidence(t *testing.T) {
	t.Parallel()
	f := setup(t)
	accountID, _, m := f.remediable(t)
	f.remediate(t, accountID, m.ID, "一", "a1")
	f.now = f.now.Add(time.Minute)
	_, err := f.decideRemediation(m.ID, 9, RequestRejected, "不足")
	must(t, err)
	f.remediate(t, accountID, m.ID, "二", "b1", "b2", "b3")
	f.now = f.now.Add(time.Minute)
	_, err = f.decideRemediation(m.ID, 9, RequestRejected, "不足")
	must(t, err)
	f.appeal(t, accountID, f.sellerView(t, accountID, m.ID), "申訴", "c1", "c2")
	got := f.measureOf(t, m.ID)
	counts := map[string]int{}
	for _, r := range got.Requests {
		counts[r.Reason] = len(r.Evidence)
		if r.Evidence == nil {
			t.Fatalf("佐證不得是 nil（空的是空陣列）：%+v", r)
		}
	}
	if counts["一"] != 1 || counts["二"] != 3 || counts["申訴"] != 2 || len(got.Requests) != 3 {
		t.Fatalf("counts=%v", counts)
	}
}

// ListMeasures 的 request_state／request_kind：集合篩選（有任一筆請求對外的狀態是這個狀態）；PENDING 是申覆與解除請求合在一起的收件匣，舊到新；
// 優先序 review_due、request PENDING；組合合法；換了篩選的 page token 不能沿用（transport 層）。
func TestListMeasuresByRequestState(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	mk := func() (int64, Measure) {
		f.now = f.now.Add(time.Minute)
		accountID, s := f.open(t)
		return accountID, f.riskMeasure(t, s.StoreID, nil)
	}
	aAcc, a := mk()
	bAcc, b := mk()
	cAcc, c := mk()
	mk()
	// a：解除請求駁回後又待處理；b：解除請求待處理（比 a 晚提出）；c：只有被駁回的解除請求；第四個措施沒有請求。
	f.now = f.now.Add(time.Minute)
	f.remediate(t, aAcc, a.ID, "a1")
	f.now = f.now.Add(time.Minute)
	_, err := f.decideRemediation(a.ID, 9, RequestRejected, "不足")
	must(t, err)
	f.remediate(t, cAcc, c.ID, "c1")
	_, err = f.decideRemediation(c.ID, 9, RequestRejected, "不足")
	must(t, err)
	f.now = f.now.Add(time.Minute)
	f.remediate(t, aAcc, a.ID, "a2")
	f.now = f.now.Add(time.Minute)
	f.remediate(t, bAcc, b.ID, "b1")

	list := func(in AdminMeasureListInput) []uuid.UUID {
		t.Helper()
		in.PageSize = 50
		page, err := f.module.AdminMeasures(ctx, in)
		must(t, err)
		ids := make([]uuid.UUID, len(page.Items))
		for i, m := range page.Items {
			ids[i] = m.ID
		}
		return ids
	}
	same := func(label string, got, want []uuid.UUID) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("%s：got=%v want=%v", label, got, want)
		}
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("%s：got=%v want=%v", label, got, want)
			}
		}
	}
	pending, rejected, accepted, closed := RequestPending, RequestRejected, RequestAccepted, RequestClosed
	remediation, appealKind := RequestKindRemediation, RequestKindAppeal
	same("PENDING 收件匣：依請求提出時間舊到新", list(AdminMeasureListInput{RequestState: &pending}), []uuid.UUID{a.ID, b.ID})
	// 集合語意：REJECTED 是「有任一筆被駁回」，a 同時有待處理的也算；新到舊（預設排序）。
	same("REJECTED 集合", list(AdminMeasureListInput{RequestState: &rejected}), []uuid.UUID{c.ID, a.ID})
	same("ACCEPTED", list(AdminMeasureListInput{RequestState: &accepted}), nil)
	same("CLOSED", list(AdminMeasureListInput{RequestState: &closed}), nil)
	same("只有種類：補正", list(AdminMeasureListInput{RequestKind: &remediation}), []uuid.UUID{c.ID, b.ID, a.ID})
	same("只有種類：申訴", list(AdminMeasureListInput{RequestKind: &appealKind}), nil)
	same("PENDING 與店家篩選合用", list(AdminMeasureListInput{RequestState: &pending, AccountID: bAcc}), []uuid.UUID{b.ID})

	// 申覆與解除請求在同一個收件匣、同一個排序：b 的解除請求駁回後申覆，排在 a 的解除請求之後。
	_, err = f.decideRemediation(b.ID, 9, RequestRejected, "不足")
	must(t, err)
	f.now = f.now.Add(time.Minute)
	f.appeal(t, bAcc, f.sellerView(t, bAcc, b.ID), "b 申訴")
	same("兩種請求同一個收件匣", list(AdminMeasureListInput{RequestState: &pending}), []uuid.UUID{a.ID, b.ID})
	same("收件匣只看補正", list(AdminMeasureListInput{RequestState: &pending, RequestKind: &remediation}), []uuid.UUID{a.ID})
	same("收件匣只看申訴", list(AdminMeasureListInput{RequestState: &pending, RequestKind: &appealKind}), []uuid.UUID{b.ID})
	same("種類加狀態是同一筆請求：b 的補正被駁回", list(AdminMeasureListInput{RequestState: &rejected, RequestKind: &remediation}), []uuid.UUID{c.ID, b.ID, a.ID})
	same("種類加狀態是同一筆請求：b 沒有被駁回的申訴", list(AdminMeasureListInput{RequestState: &rejected, RequestKind: &appealKind}), nil)

	// review_due 優先：a 的複審到期時，review_due 的排序是 review_by，其餘篩選照 AND。
	f.now = f.now.Add(8 * oneDay)
	due := list(AdminMeasureListInput{ReviewDue: true, RequestState: &pending, RequestKind: &remediation})
	same("review_due 加 request_state=PENDING", due, []uuid.UUID{a.ID})

	// 不合法的值。
	bad := RequestState("WITHDRAWN")
	_, err = f.module.AdminMeasures(ctx, AdminMeasureListInput{RequestState: &bad, PageSize: 10})
	wantFailure(t, "不認得的狀態", err, apperr.KindInvalidInput)
	badKind := RequestKind("WITHDRAWAL")
	_, err = f.module.AdminMeasures(ctx, AdminMeasureListInput{RequestKind: &badKind, PageSize: 10})
	wantFailure(t, "不認得的種類", err, apperr.KindInvalidInput)
}

// 篩選與推導的一致（資料集）：每一個（狀態、種類）的篩選結果，都等於「措施上有任一筆請求的 EffectiveRequests 符合」；已結束措施上儲存為
// PENDING 的解除請求（沒有跑結案掃描）不在 PENDING 的收件匣、在 CLOSED；掃描之後（儲存值收斂）篩選結果不變。
func TestListMeasuresByRequestStateAgreesWithEffectiveRequest(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	mk := func(end *time.Time) (int64, Measure) {
		f.now = f.now.Add(time.Minute)
		accountID, s := f.open(t)
		return accountID, f.riskMeasure(t, s.StoreID, end)
	}
	short := f.now.Add(2 * time.Hour)
	mootAcc, moot := mk(&short)
	f.remediate(t, mootAcc, moot.ID, "失去標的")
	endedAcc, endedWithAppeal := mk(&short)
	f.now = f.now.Add(3 * time.Hour)
	f.appeal(t, endedAcc, f.sellerView(t, endedAcc, endedWithAppeal.ID), "已結束措施上的申訴")
	activeAcc, activePending := mk(nil)
	f.remediate(t, activeAcc, activePending.ID, "生效中待處理")
	rejectedAcc, rejectedOne := mk(nil)
	f.remediate(t, rejectedAcc, rejectedOne.ID, "會被駁回")
	_, err := f.decideRemediation(rejectedOne.ID, 9, RequestRejected, "不足")
	must(t, err)
	acceptedAcc, acceptedOne := mk(nil)
	f.remediate(t, acceptedAcc, acceptedOne.ID, "會被接受")
	_, err = f.decideRemediation(acceptedOne.ID, 9, RequestAccepted, "已改善")
	must(t, err)
	appealAcc, appealed := mk(nil)
	f.appeal(t, appealAcc, f.sellerView(t, appealAcc, appealed.ID), "申訴")
	mk(nil)

	check := func(phase string) {
		t.Helper()
		all, err := f.module.AdminMeasures(ctx, AdminMeasureListInput{PageSize: 100})
		must(t, err)
		if len(all.Items) != 7 {
			t.Fatalf("%s：資料集 %d 筆", phase, len(all.Items))
		}
		states := []RequestState{"", RequestPending, RequestAccepted, RequestRejected, RequestClosed}
		kinds := []RequestKind{"", RequestKindAppeal, RequestKindRemediation}
		for _, st := range states {
			for _, kd := range kinds {
				want := map[uuid.UUID]bool{}
				for _, m := range all.Items {
					for _, r := range m.EffectiveRequests() {
						if (st == "" || r.State == st) && (kd == "" || r.Kind == kd) {
							want[m.ID] = true
						}
					}
				}
				if st == "" && kd == "" {
					for _, m := range all.Items {
						want[m.ID] = true
					}
				}
				in := AdminMeasureListInput{PageSize: 100}
				if st != "" {
					in.RequestState = &st
				}
				if kd != "" {
					in.RequestKind = &kd
				}
				page, err := f.module.AdminMeasures(ctx, in)
				must(t, err)
				got := map[uuid.UUID]bool{}
				for _, m := range page.Items {
					got[m.ID] = true
				}
				if len(got) != len(page.Items) || !maps.Equal(got, want) {
					t.Errorf("%s：state=%q kind=%q 篩選 %d 筆，推導 %d 筆", phase, st, kd, len(got), len(want))
				}
			}
		}
		pending, closed := RequestPending, RequestClosed
		inbox, err := f.module.AdminMeasures(ctx, AdminMeasureListInput{RequestState: &pending, PageSize: 100})
		must(t, err)
		closedList, err := f.module.AdminMeasures(ctx, AdminMeasureListInput{RequestState: &closed, PageSize: 100})
		must(t, err)
		contains := func(page MeasurePage, id uuid.UUID) bool {
			return slices.ContainsFunc(page.Items, func(m Measure) bool { return m.ID == id })
		}
		if contains(inbox, moot.ID) || !contains(closedList, moot.ID) || !contains(inbox, activePending.ID) || !contains(inbox, appealed.ID) || contains(closedList, activePending.ID) {
			t.Errorf("%s：失去標的的補正不在收件匣、在 CLOSED；生效中的補正與申訴在收件匣", phase)
		}
	}
	check("結案掃描之前（儲存值仍是 PENDING）")
	if n := f.count(t, "SELECT count(*) FROM seller.measure_requests WHERE state = 'PENDING' AND kind = 'REMEDIATION' AND measure_id = $1", moot.ID); n != 1 {
		t.Fatalf("前提：儲存值仍是 PENDING：%d", n)
	}
	n, err := f.module.CloseEndedMeasureRequests(ctx)
	must(t, err)
	if n != 1 {
		t.Fatalf("掃描結案 %d 筆", n)
	}
	check("結案掃描之後")
	f.assertMeasureInvariants(t)
}

// 收件匣（request_state=PENDING）的 keyset 以解除請求提出時間為位置：五個待處理的解除請求分三頁（2、2、1）取完，順序是提出時間舊到新，
// 沒有重複也沒有遺漏。
func TestPendingRemediationInboxPaginatesOldestFirst(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	var want []uuid.UUID
	for i := range 5 {
		f.now = f.now.Add(time.Minute)
		accountID, _, m := f.remediable(t)
		f.now = f.now.Add(time.Minute)
		f.remediate(t, accountID, m.ID, "補正"+string(rune('a'+i)))
		want = append(want, m.ID)
	}
	pending := RequestPending
	var got []uuid.UUID
	var after = AdminMeasureListInput{}.After
	pages := 0
	for {
		in := AdminMeasureListInput{RequestState: &pending, PageSize: 2}
		if after != nil {
			in.After = after
		}
		page, err := f.module.AdminMeasures(ctx, in)
		must(t, err)
		pages++
		for _, m := range page.Items {
			got = append(got, m.ID)
		}
		if page.Next == nil {
			break
		}
		if len(page.Items) != 2 {
			t.Fatalf("還有下一頁的一頁必須是滿的：%d", len(page.Items))
		}
		after = page.Next
	}
	if pages != 3 || len(got) != len(want) {
		t.Fatalf("pages=%d got=%v want=%v", pages, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("順序或內容不符 got=%v want=%v", got, want)
		}
	}
}

// 到期 job 遺失、同一帳號又被其他寫入重算過（期間已由那次重算結束，帳號沒有進行中的期間也沒有生效中的停權）：結案掃描仍要把已結束
// 措施上待處理的解除請求結案（全域掃描，不靠帳號的期間或生效狀態，也不靠停權的到期 job 與對帳）。
func TestCloseEndedSweepClosesAStaleRemediationAfterAnInterveningRecompute(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	closed := record[MeasureRequestClosed](f)
	accountID, s := f.open(t)
	short := f.now.Add(2 * time.Hour)
	m := f.riskMeasure(t, s.StoreID, &short)
	f.remediate(t, accountID, m.ID, "已改善")
	f.now = f.now.Add(3 * time.Hour)

	other := f.riskMeasure(t, s.StoreID, nil)
	otherView := f.measureOf(t, other.ID)
	_, err := f.module.RevokeMeasure(ctx, RevokeMeasureInput{MeasureID: other.ID, ExpectedVersion: otherView.Version, AdminID: 12, Reason: RevokeAdminCorrection})
	must(t, err)
	if f.measureOf(t, m.ID).MootRemediation() == nil || len(closed.all()) != 0 {
		t.Fatal("前提：其他寫入的重算與對帳都不結案，儲存值仍是 PENDING")
	}
	if _, err := f.module.ReconcileSuspensions(ctx); err != nil {
		t.Fatal(err)
	}
	if f.measureOf(t, m.ID).MootRemediation() == nil || len(closed.all()) != 0 {
		t.Fatal("停權的對帳不負責請求的結案")
	}

	n, err := f.module.CloseEndedMeasureRequests(ctx)
	must(t, err)
	r := f.measureOf(t, m.ID).Requests[0]
	if n != 1 || r.State != RequestClosed || r.CloseCause == nil || *r.CloseCause != CloseCauseEnded || r.DecidedByAdminID != nil {
		t.Fatalf("n=%d request=%+v", n, r)
	}
	if evs := closed.all(); len(evs) != 1 || evs[0].MeasureID != m.ID || evs[0].Cause != CloseCauseEnded {
		t.Fatalf("closed=%+v", evs)
	}
	n, err = f.module.CloseEndedMeasureRequests(ctx)
	must(t, err)
	if n != 0 || len(closed.all()) != 1 {
		t.Fatal("掃描重跑不得再發事件")
	}
	f.assertMeasureInvariants(t)
}

// 一個帳號的結案失敗不得讓排序在後面的帳號永遠輪不到：其餘帳號照樣結案，失敗以合併的錯誤回傳（river 重試）。
func TestCloseEndedSweepContinuesPastAFailingAccount(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	short := f.now.Add(2 * time.Hour)
	firstAccount, firstStore := f.open(t)
	first := f.riskMeasure(t, firstStore.StoreID, &short)
	f.remediate(t, firstAccount, first.ID, "已改善")
	secondAccount, secondStore := f.open(t)
	second := f.riskMeasure(t, secondStore.StoreID, &short)
	f.remediate(t, secondAccount, second.ID, "已改善")
	f.now = f.now.Add(3 * time.Hour)

	failing := true
	events.Subscribe(f.bus, func(_ context.Context, e MeasureRequestClosed) error {
		if failing && e.MeasureID == first.ID {
			return errors.New("boom")
		}
		return nil
	})
	n, err := f.module.CloseEndedMeasureRequests(ctx)
	if err == nil || !strings.Contains(err.Error(), "boom") || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if f.measureOf(t, first.ID).MootRemediation() == nil || f.measureOf(t, second.ID).MootRemediation() != nil {
		t.Fatal("失敗的帳號維持待結案，其餘帳號已結案")
	}
	failing = false
	n, err = f.module.CloseEndedMeasureRequests(ctx)
	must(t, err)
	if n != 1 || f.measureOf(t, first.ID).MootRemediation() != nil {
		t.Fatalf("重跑後補完：n=%d", n)
	}
	f.assertMeasureInvariants(t)
}

// SubmitAppeal 的失敗訊息說出真正的原因：生效中的措施有待處理的解除請求時不是「已申覆過或已撤銷」。
func TestSubmitAppealFailureMessagesNameTheRealReason(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _, m := f.remediable(t)
	f.remediate(t, accountID, m.ID, "已改善")
	_, err := f.module.SubmitAppeal(ctx, SubmitAppealInput{AccountID: accountID, MeasureID: m.ID, ExpectedVersion: f.sellerView(t, accountID, m.ID).SellerVersion(), Reason: "申訴"})
	wantFailure(t, "補正待處理時不能申訴", err, apperr.KindFailedPrecondition)
	var e *apperr.Error
	if !errors.As(err, &e) || !strings.Contains(e.Message, "解除請求") || strings.Contains(e.Message, "已提出過申覆") || strings.Contains(e.Message, "已撤銷") ||
		len(e.FieldViolations()) != 0 {
		t.Fatalf("err=%#v", err)
	}
	if us := f.sellerView(t, accountID, m.ID).AppealUnavailabilities(uitext.ZhTW); len(us) == 0 || us[0].Reason != AppealReasonRequestPending || us[0].Message.Text != e.Message {
		t.Fatalf("錯誤文字要等於可用性清單第一筆的句子：%+v 對 %q", us, e.Message)
	}
}

func remediationsOf(m Measure) []MeasureRequest {
	var out []MeasureRequest
	for _, r := range m.EffectiveRequests() {
		if r.Kind == RequestKindRemediation {
			out = append(out, r)
		}
	}
	return out
}
