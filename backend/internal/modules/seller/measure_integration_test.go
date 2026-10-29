//go:build integration

package seller_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/suspension"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
	"github.com/jackc/pgx/v5/pgconn"
)

func (f *fixture) openPeriods(t *testing.T) int64 {
	t.Helper()
	return f.count(t, "SELECT count(*) FROM seller.suspension_periods WHERE ended_at IS NULL")
}

// waitForLockWaiters 等到至少 n 個連線在等列鎖：測試先自己拿住鎖，確定請求都卡住了才放開。
func (f *fixture) waitForLockWaiters(t *testing.T, n int64) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for f.count(t, "SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'") < n {
		if time.Now().After(deadline) {
			t.Fatalf("%d requests never waited on the row lock", n)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func expiryJobs(t *testing.T, f *fixture) map[string]time.Time {
	t.Helper()
	f.jobs.mu.Lock()
	defer f.jobs.mu.Unlock()
	out := map[string]time.Time{}
	for i, args := range f.jobs.args {
		if args.Kind() != "seller.measure_expiry" {
			continue
		}
		opts := f.jobs.opts[i]
		if opts == nil || !opts.UniqueOpts.ByArgs {
			t.Fatalf("expiry job must be unique by args: %+v", opts)
		}
		raw, err := json.Marshal(args)
		must(t, err)
		out[string(raw)] = opts.ScheduledAt
	}
	return out
}

// 帳號任何一個消費者看到的停權：讀店家、能力缺 NOT_SUSPENDED、ActingStore 拒絕、後台清單的停權篩選。
type consumers struct{ store, capability, acting, listed bool }

func (f *fixture) consumersOf(t *testing.T, accountID int64, storeID int64) consumers {
	t.Helper()
	ctx := context.Background()
	s, err := f.module.Store(ctx, storeID)
	must(t, err)
	statuses := CapabilitiesOf(s)
	missing := false
	for _, st := range statuses {
		if slices.Contains(st.Missing, RequirementNotSuspended) {
			missing = true
		}
	}
	_, actingErr := f.module.ActingStore(ctx, accountID, CapProductManage)
	yes := true
	page, err := f.module.AdminStores(ctx, StoreListInput{Suspended: &yes, PageSize: 100})
	must(t, err)
	listed := false
	for _, item := range page.Items {
		listed = listed || item.Store.StoreID == storeID
	}
	return consumers{store: s.Suspended(), capability: missing, acting: apperr.KindOf(actingErr) == apperr.KindPermissionDenied, listed: listed}
}

func (f *fixture) wantSuspended(t *testing.T, label string, accountID, storeID int64, want bool) {
	t.Helper()
	got := f.consumersOf(t, accountID, storeID)
	if got != (consumers{want, want, want, want}) {
		t.Fatalf("%s：停權的消費者要一起是 %v：%+v", label, want, got)
	}
}

// EffectiveMeasures（SQL）與 model.StateOf（讀取時推導）在邊界時間一致：開始的那一刻已經生效、結束的那一刻已經不生效，前後各
// 1µs 也一致；沒有固定期間的 RISK 一直生效到被結束。
func TestEffectiveMeasuresAndStateAgreeAtTheBoundaries(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.open(t)
	_, risk := f.open(t)
	created := f.now
	ends := created.Add(time.Hour)
	m := f.violate(t, s.StoreID, SeverityHigh, MeasureSuspension, &ends)
	open := f.riskSuspend(t, risk.StoreID)
	us := time.Microsecond
	for _, c := range []struct {
		name string
		at   time.Time
		want bool
	}{
		{"尚未開始", created.Add(-us), false}, {"開始的那一刻", created, true}, {"開始後 1µs", created.Add(us), true},
		{"結束前 1µs", ends.Add(-us), true}, {"結束的那一刻", ends, false}, {"結束後 1µs", ends.Add(us), false},
	} {
		f.now = c.at
		if got := f.isSuspended(t, s.StoreID); got != c.want {
			t.Errorf("%s：Store.Suspended=%v，預期 %v", c.name, got, c.want)
		}
		if state := f.measureOf(t, m.ID).State; (state == StateActive) != c.want {
			t.Errorf("%s：State=%s，預期 active=%v", c.name, state, c.want)
		}
		if state := f.measureOf(t, open.ID).State; c.at.After(created) || c.at.Equal(created) {
			if state != StateActive || !f.isSuspended(t, risk.StoreID) {
				t.Errorf("%s：沒有固定期間的 RISK 要一直生效：%s", c.name, state)
			}
		}
	}
	f.now = created.Add(us)
	f.endRisk(t, open)
	if f.isSuspended(t, risk.StoreID) || f.measureOf(t, open.ID).State != StateEnded {
		t.Fatal("結束之後不生效")
	}
}

// 結束、撤銷、縮短之後，讀店家、能力的 NOT_SUSPENDED、ActingStore 與後台清單的停權篩選一起改變：它們是同一個推導。
func TestSuspensionStateChangesTogetherEverywhere(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	f.wantSuspended(t, "沒有措施", accountID, s.StoreID, false)

	risk := f.riskSuspend(t, s.StoreID)
	f.wantSuspended(t, "RISK 停權", accountID, s.StoreID, true)
	f.endRisk(t, risk)
	f.wantSuspended(t, "結束 RISK", accountID, s.StoreID, false)

	ends := f.now.Add(2 * time.Hour)
	m := f.violate(t, s.StoreID, SeverityHigh, MeasureSuspension, &ends)
	f.wantSuspended(t, "認定停權", accountID, s.StoreID, true)
	shorter := f.now.Add(30 * time.Minute)
	m, err := f.module.UpdateMeasure(ctx, UpdateMeasureInput{MeasureID: m.ID, AdminID: 7, ExpectedVersion: m.Version, ChangeEndsAt: true, EndsAt: &shorter})
	must(t, err)
	f.wantSuspended(t, "縮短到未來", accountID, s.StoreID, true)
	f.now = shorter.Add(time.Minute)
	f.wantSuspended(t, "縮短的結束時間過了", accountID, s.StoreID, false)

	f.now = f.now.Add(time.Hour)
	second := f.violate(t, s.StoreID, SeverityHigh, MeasureSuspension, ptr(f.now.Add(48*time.Hour)))
	f.wantSuspended(t, "第二個認定停權", accountID, s.StoreID, true)
	_, err = f.module.RevokeMeasure(ctx, RevokeMeasureInput{MeasureID: second.ID, ExpectedVersion: second.Version, AdminID: 7, Reason: RevokeAdminCorrection})
	must(t, err)
	f.wantSuspended(t, "撤銷", accountID, s.StoreID, false)
	_ = m
}

// 措施是唯一來源：資料庫裡沒有另外存一份效果的 suspensions 表。
func TestThereIsNoSeparateSuspensionsTable(t *testing.T) {
	t.Parallel()
	f := setup(t)
	if n := f.count(t, "SELECT count(*) FROM information_schema.tables WHERE table_schema = 'seller' AND table_name IN ('suspensions', 'suspension_identities', 'violation_revisions', 'violation_notes', 'appeal_evidence')"); n != 0 {
		t.Fatalf("舊的處分表還在：%d", n)
	}
	if n := f.count(t, "SELECT count(*) FROM information_schema.columns WHERE table_schema = 'seller' AND table_name = 'measures' AND column_name IN ('suspended', 'lifted_at', 'effective')"); n != 0 {
		t.Fatalf("措施不存推導出來的效果（ends_at 是賣家看得到的結束時間的目前值，不是推導的效果）：%d", n)
	}
}

// 同一個帳號可以有多個 RISK 停權（舊的管理員停權一個帳號只能一筆）；停權期間是同一段，最後一個結束才發 StoreUnsuspended。
func TestTwoRiskMeasuresOnOneAccountBothSucceedAndShareOnePeriod(t *testing.T) {
	t.Parallel()
	f := setup(t)
	suspended := record[StoreSuspended](f)
	unsuspended := record[StoreUnsuspended](f)
	accountID, s := f.open(t)
	first := f.riskSuspend(t, s.StoreID)
	second := f.riskSuspend(t, s.StoreID)
	if first.ID == second.ID || len(suspended.all()) != 1 || f.openPeriods(t) != 1 {
		t.Fatalf("兩個 RISK 停權：events=%d open=%d", len(suspended.all()), f.openPeriods(t))
	}
	f.endRisk(t, first)
	f.wantSuspended(t, "還有一個 RISK", accountID, s.StoreID, true)
	if len(unsuspended.all()) != 0 {
		t.Fatalf("還有生效中的措施就不能恢復：%+v", unsuspended.all())
	}
	f.endRisk(t, second)
	f.wantSuspended(t, "全部結束", accountID, s.StoreID, false)
	if got := unsuspended.all(); len(got) != 1 || got[0].PeriodID != suspended.all()[0].PeriodID || f.openPeriods(t) != 0 {
		t.Fatalf("StoreUnsuspended 只發一次、帶同一段期間：%+v", got)
	}
}

// 資料庫層的保證：裁定一定附理由、依據認定的措施一定有依據（commit 時檢查）、RISK 一定有複審期限、警告不能是 RISK、每個措施同時
// 只有一筆待處理的請求。
func TestMeasureSchemaConstraints(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s, m := f.appealable(t)
	f.appeal(t, accountID, m, "誤判")
	request := f.measureOf(t, m.ID).Appeal().ID
	for name, sql := range map[string]string{
		"接受不附理由":                "UPDATE seller.measure_requests SET state = 'ACCEPTED', decided_by_admin_id = 1, decided_at = now() WHERE id = $1",
		"駁回不附理由":                "UPDATE seller.measure_requests SET state = 'REJECTED', decided_by_admin_id = 1, decided_at = now() WHERE id = $1",
		"接受附空白理由":               "UPDATE seller.measure_requests SET state = 'ACCEPTED', decision_reason = '', decided_by_admin_id = 1, decided_at = now() WHERE id = $1",
		"結案帶理由":                 "UPDATE seller.measure_requests SET state = 'CLOSED', close_cause = 'REVOKED', decision_reason = 'x', decided_by_admin_id = 1, decided_at = now() WHERE id = $1",
		"待處理卻已有裁定者":             "UPDATE seller.measure_requests SET decided_by_admin_id = 1 WHERE id = $1",
		"不認得的狀態":                "UPDATE seller.measure_requests SET state = 'WITHDRAWN' WHERE id = $1",
		"第二筆待處理的請求":             "INSERT INTO seller.measure_requests (id, measure_id, kind, state, reason, revision, submitted_at, basis) SELECT uuidv7(), measure_id, kind, 'PENDING', 'x', revision, submitted_at, basis FROM seller.measure_requests WHERE id = $1",
		"第二次申訴":                 "INSERT INTO seller.measure_requests (id, measure_id, kind, state, reason, revision, submitted_at, decision_reason, decided_by_admin_id, decided_at, basis) SELECT uuidv7(), measure_id, kind, 'REJECTED', 'x', revision, submitted_at, 'x', 1, submitted_at, basis FROM seller.measure_requests WHERE id = $1",
		"RISK 沒有複審期限":           "UPDATE seller.measures SET basis = 'RISK', risk_ground = 'FRAUD', lead_finding_id = NULL, review_by = NULL WHERE id = $1",
		"撤銷沒有原因":                "UPDATE seller.measures SET status = 'REVOKED' WHERE id = $1",
		"不認得的撤銷原因":              "UPDATE seller.measures SET status = 'REVOKED', revoked_reason = 'WHATEVER', revoked_by_admin_id = 1, revoked_at = now() WHERE id = $1",
		"認定不能用 FINDING_REVOKED": "UPDATE seller.violations SET status = 'REVOKED', revoked_reason = 'FINDING_REVOKED', revoked_by_admin_id = 1, revoked_at = now() WHERE origin_measure_id = $1",
	} {
		id := any(request)
		if name == "RISK 沒有複審期限" || name == "撤銷沒有原因" || name == "不認得的撤銷原因" || name == "認定不能用 FINDING_REVOKED" {
			id = m.ID
		}
		if _, err := f.db.Conn(ctx).Exec(ctx, sql, id); err == nil {
			t.Errorf("%s：資料庫不該接受", name)
		}
	}

	// 依據認定的措施在 commit 時一定要有依據的認定（主要依據在 measure_findings 裡）；警告不能是 RISK。
	id := uuid.NewV7()
	tx, err := f.db.Pool().Begin(ctx)
	must(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `INSERT INTO seller.measures (id, account_id, store_id, kind, basis, lead_finding_id, statement_to_seller, current_revision, created_by_admin_id, created_at, updated_at, starts_at)
		VALUES ($1, $4, $2, 'WARNING', 'VIOLATION', $3, 'x', 1, 1, now(), now(), now())`, id, s.StoreID, m.Findings[0].ID, accountID)
	must(t, err)
	_, err = tx.Exec(ctx, `INSERT INTO seller.measure_revisions (measure_id, revision, statement_to_seller, admin_id, created_at) VALUES ($1, 1, 'x', 1, now())`, id)
	must(t, err)
	if err := tx.Commit(ctx); err == nil {
		t.Fatal("沒有 measure_findings 的 VIOLATION 措施必須在 commit 時失敗")
	}
	if _, err := f.db.Conn(ctx).Exec(ctx, `INSERT INTO seller.measures (id, account_id, store_id, kind, basis, risk_ground, review_by, statement_to_seller, current_revision, created_by_admin_id, created_at, updated_at, starts_at)
		VALUES ($1, $3, $2, 'WARNING', 'RISK', 'FRAUD', now() + interval '1 day', 'x', 1, 1, now(), now(), now())`, uuid.NewV7(), s.StoreID, accountID); err == nil {
		t.Fatal("警告不能依據風險類型")
	}
	endsInFuture := time.Now().Add(24 * time.Hour)
	for name, c := range map[string]struct {
		kind string
		ends *time.Time
	}{
		"警告有結束時間":       {"WARNING", &endsInFuture},
		"依據認定的停權沒有結束時間": {"SUSPENSION", nil},
	} {
		_, err := f.db.Conn(ctx).Exec(ctx, `INSERT INTO seller.measures (id, account_id, store_id, kind, basis, lead_finding_id, statement_to_seller, ends_at, current_revision, created_by_admin_id, created_at, updated_at, starts_at)
			VALUES ($1, $3, $2, $4, 'VIOLATION', $5, 'x', $6, 1, 1, now(), now(), now())`, uuid.NewV7(), s.StoreID, accountID, c.kind, uuid.NewV7(), c.ends)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.ConstraintName != "measures_period_check" {
			t.Errorf("%s：要被 measures_period_check 擋下：%v", name, err)
		}
	}
	for name, statement := range map[string]string{"理由是空字串": "", "理由超過 2000 字": strings.Repeat("字", 2001)} {
		_, err := f.db.Conn(ctx).Exec(ctx, `INSERT INTO seller.measures (id, account_id, store_id, kind, basis, lead_finding_id, statement_to_seller, current_revision, created_by_admin_id, created_at, updated_at, starts_at)
			VALUES ($1, $3, $2, 'WARNING', 'VIOLATION', $4, $5, 1, 1, now(), now(), now())`, uuid.NewV7(), s.StoreID, accountID, uuid.NewV7(), statement)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.ConstraintName != "measures_statement_check" {
			t.Errorf("%s：要被 measures_statement_check 擋下：%v", name, err)
		}
	}
	// 認定一定有建立它的措施（commit 時檢查）。
	tx, err = f.db.Pool().Begin(ctx)
	must(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `INSERT INTO seller.violations (id, store_id, account_id, category, severity, origin_measure_id, created_by_admin_id, created_at, updated_at)
		VALUES (uuidv7(), $1, $2, 'FRAUD', 'LOW', uuidv7(), 1, now(), now())`, s.StoreID, accountID)
	must(t, err)
	if err := tx.Commit(ctx); err == nil {
		t.Fatal("沒有建立它的措施的認定必須在 commit 時失敗")
	}
}

// 措施只能建在還沒關閉的 stint（盤點：舊的不檢查）。
func TestMeasureOnClosedStintIsRejected(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	_, err := f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	ends := f.now.Add(24 * time.Hour)
	review := f.now.Add(24 * time.Hour)
	for name, in := range map[string]CreateMeasureInput{
		"警告": f.warningFrom(s.StoreID),
		"認定停權": {AdminID: 7, StoreID: s.StoreID, Kind: MeasureSuspension, StatementToSeller: "y", Note: "x", EndsAt: &ends,
			Violation: &ViolationBasis{Finding: &NewFinding{Category: policy.Fraud, Severity: SeverityHigh}}},
		"RISK 停權": {AdminID: 7, StoreID: s.StoreID, Kind: MeasureSuspension, StatementToSeller: "y", Note: "x", ReviewBy: &review, Risk: &RiskBasis{Ground: RiskFraud}},
	} {
		// 店家存在而狀態不允許：failed_precondition，句子與 GetStore 的 create_measure_unavailabilities 同一個函式（StoreActionUnavailableText）。
		if _, err := f.module.CreateMeasure(ctx, in); apperr.KindOf(err) != apperr.KindFailedPrecondition || !strings.Contains(err.Error(), StoreActionUnavailableText(StoreReasonStoreClosed, StoreActionCreateMeasure).Text) {
			t.Fatalf("%s on a closed stint: %v", name, err)
		}
	}
	if f.count(t, "SELECT count(*) FROM seller.violations")+f.count(t, "SELECT count(*) FROM seller.measures") != 0 {
		t.Fatal("measure written on a closed stint")
	}
}

func TestCreateMeasureValidates(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	_, other := f.open(t)
	past, future, far := f.now.Add(-time.Hour), f.now.Add(time.Hour), f.now.Add(31*24*time.Hour)
	week := f.now.Add(7 * 24 * time.Hour)
	foreign := f.buyerReport(t, other.StoreID, 5, policy.Fraud).Report
	finding := &ViolationBasis{Finding: &NewFinding{Category: policy.Fraud, Severity: SeverityLow}}
	risk := &RiskBasis{Ground: RiskFraud}
	_, otherAccountFinding := f.open(t)
	foreignMeasure := f.violate(t, otherAccountFinding.StoreID, SeverityLow, MeasureWarning, nil)
	for name, in := range map[string]CreateMeasureInput{
		"認定停權沒有結束時間":     {Kind: MeasureSuspension, Violation: finding},
		"結束時間在過去":        {Kind: MeasureSuspension, Violation: finding, EndsAt: &past},
		"警告設結束時間":        {Kind: MeasureWarning, Violation: finding, EndsAt: &future},
		"不認得的種類":         {Kind: "FINE", Violation: finding},
		"沒有依據":           {Kind: MeasureWarning},
		"兩種依據":           {Kind: MeasureSuspension, Violation: finding, Risk: risk, ReviewBy: &week, EndsAt: &future},
		"認定沒有任何一筆":       {Kind: MeasureWarning, Violation: &ViolationBasis{}},
		"不認得的類別":         {Kind: MeasureWarning, Violation: &ViolationBasis{Finding: &NewFinding{Category: "X", Severity: SeverityLow}}},
		"不認得的嚴重度":        {Kind: MeasureWarning, Violation: &ViolationBasis{Finding: &NewFinding{Category: policy.Fraud, Severity: "X"}}},
		"引用別的帳號的認定":      {Kind: MeasureWarning, Violation: &ViolationBasis{CitedFindingIDs: []uuid.UUID{foreignMeasure.Findings[0].ID}}},
		"引用不存在的認定":       {Kind: MeasureWarning, Violation: &ViolationBasis{CitedFindingIDs: []uuid.UUID{uuid.NewV7()}}},
		"重複引用":           {Kind: MeasureWarning, Violation: &ViolationBasis{CitedFindingIDs: []uuid.UUID{foreignMeasure.Findings[0].ID, foreignMeasure.Findings[0].ID}}},
		"來源通報是別的店家的":     {Kind: MeasureWarning, Violation: &ViolationBasis{Finding: &NewFinding{Category: policy.Fraud, Severity: SeverityLow, SourceReportIDs: []uuid.UUID{foreign.ID}}}},
		"RISK 缺複審期限":     {Kind: MeasureSuspension, Risk: risk},
		"RISK 複審期限在過去":   {Kind: MeasureSuspension, Risk: risk, ReviewBy: &past},
		"RISK 複審期限超過三十天": {Kind: MeasureSuspension, Risk: risk, ReviewBy: &far},
		"RISK 是警告":       {Kind: MeasureWarning, Risk: risk, ReviewBy: &week},
		"RISK 結束時間在過去":   {Kind: MeasureSuspension, Risk: risk, ReviewBy: &week, EndsAt: &past},
		"不認得的風險類型":       {Kind: MeasureSuspension, Risk: &RiskBasis{Ground: "X"}, ReviewBy: &week},
		"認定依據帶複審期限":      {Kind: MeasureWarning, Violation: finding, ReviewBy: &week},
	} {
		in.StoreID, in.StatementToSeller, in.Note, in.AdminID = s.StoreID, "y", "x", 7
		if _, err := f.module.CreateMeasure(ctx, in); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：%v", name, err)
		}
	}
	missing := func(mut func(*CreateMeasureInput)) error {
		in := f.warningFrom(s.StoreID)
		mut(&in)
		_, err := f.module.CreateMeasure(ctx, in)
		return err
	}
	for name, mut := range map[string]func(*CreateMeasureInput){
		"沒有給賣家的理由": func(in *CreateMeasureInput) { in.StatementToSeller = " " },
		"沒有第一則註記":  func(in *CreateMeasureInput) { in.Note = "" },
		"沒有管理員":    func(in *CreateMeasureInput) { in.AdminID = 0 },
		"沒有店家":     func(in *CreateMeasureInput) { in.StoreID = 0 },
	} {
		if err := missing(mut); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：%v", name, err)
		}
	}
	// 引用的認定已撤銷也不行；不成立的通報不能成立認定（TestMeasureRejectsADismissedSourceReport）。
	own := f.violate(t, s.StoreID, SeverityLow, MeasureWarning, nil)
	_, err := f.module.RevokeMeasure(ctx, RevokeMeasureInput{MeasureID: own.ID, ExpectedVersion: own.Version, AdminID: 7, Reason: RevokeAdminCorrection, RevokeFindings: true})
	must(t, err)
	in := f.warningFrom(s.StoreID)
	in.Violation = &ViolationBasis{CitedFindingIDs: []uuid.UUID{own.Findings[0].ID}}
	if _, err := f.module.CreateMeasure(ctx, in); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Errorf("引用已撤銷的認定：%v", err)
	}
}

// 90 天內第 3 次 MEDIUM 升為 HIGH；撤銷的、90 天以前的不算。
func TestEscalationThirdMediumWithinNinetyDays(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	old := f.violate(t, s.StoreID, SeverityMedium, MeasureWarning, nil)
	f.now = f.now.Add(91 * 24 * time.Hour)
	revoked := f.violate(t, s.StoreID, SeverityMedium, MeasureWarning, nil)
	_, err := f.module.RevokeMeasure(ctx, RevokeMeasureInput{MeasureID: revoked.ID, ExpectedVersion: revoked.Version, AdminID: 7, Reason: RevokeAdminCorrection, RevokeFindings: true})
	must(t, err)
	first := f.violate(t, s.StoreID, SeverityMedium, MeasureWarning, nil)
	second := f.violate(t, s.StoreID, SeverityMedium, MeasureWarning, nil)
	third := f.violate(t, s.StoreID, SeverityMedium, MeasureWarning, nil)
	fourth := f.violate(t, s.StoreID, SeverityMedium, MeasureWarning, nil)
	for _, m := range []Measure{old, first, second} {
		if m.Findings[0].EscalatedFrom != nil {
			t.Fatalf("escalated too early: %+v", m.Findings[0])
		}
	}
	for _, m := range []Measure{third, fourth} {
		if v := m.Findings[0]; v.Severity != SeverityHigh || v.EscalatedFrom == nil || *v.EscalatedFrom != SeverityMedium {
			t.Fatalf("third medium in 90 days must become high: %+v", v)
		}
	}
	if low := f.violate(t, s.StoreID, SeverityLow, MeasureWarning, nil); low.Findings[0].EscalatedFrom != nil {
		t.Fatal("only MEDIUM escalates")
	}
}

// 升級以帳號計（新 stint 繼承舊 stint 的累計）：帳號在店 A 有兩筆 MEDIUM，關店、開店 B，第三筆 MEDIUM 升為 HIGH。
func TestEscalationCountsTheAccountAcrossStints(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, a := f.open(t)
	f.violate(t, a.StoreID, SeverityMedium, MeasureWarning, nil)
	f.violate(t, a.StoreID, SeverityMedium, MeasureWarning, nil)
	_, err := f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	b, err := f.module.Apply(ctx, validApply(accountID))
	must(t, err)
	third := f.violate(t, b.StoreID, SeverityMedium, MeasureWarning, nil)
	if v := third.Findings[0]; v.Severity != SeverityHigh || v.EscalatedFrom == nil || v.AccountID != accountID || v.StoreID != b.StoreID {
		t.Fatalf("關店重開不能讓升級歸零：%+v", v)
	}
	// 別的帳號不受影響。
	_, other := f.open(t)
	if m := f.violate(t, other.StoreID, SeverityMedium, MeasureWarning, nil); m.Findings[0].EscalatedFrom != nil {
		t.Fatal("別的帳號的累計不能算進來")
	}
}

// 兩個 MEDIUM 同時建立（帳號已有一個）：以帳號的鎖串行，剛好一個升級。測試先鎖住店家列，等兩個請求都在等鎖再放開，
// 所以沒有鎖時兩個請求都在放開前算完數量，結果是沒有升級。
func TestEscalationIsSerialisedByTheAccountLock(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	f.violate(t, s.StoreID, SeverityMedium, MeasureWarning, nil)
	tx, err := f.db.Pool().Begin(ctx)
	must(t, err)
	_, err = tx.Exec(ctx, "SELECT 1 FROM seller.stores WHERE store_id=$1 FOR UPDATE", s.StoreID)
	must(t, err)
	results := make(chan CreateMeasureResult, 2)
	errs := make(chan error, 2)
	for range 2 {
		go func() {
			in := f.warningFrom(s.StoreID)
			in.Violation.Finding.Severity = SeverityMedium
			r, err := f.module.CreateMeasure(ctx, in)
			results <- r
			errs <- err
		}()
	}
	f.waitForLockWaiters(t, 2)
	must(t, tx.Rollback(ctx))
	escalated := 0
	for range 2 {
		must(t, <-errs)
		if (<-results).Measure.Findings[0].EscalatedFrom != nil {
			escalated++
		}
	}
	if escalated != 1 {
		t.Fatalf("escalated %d of the 2nd and 3rd MEDIUM; want exactly 1", escalated)
	}
}

// 冪等的建立（request_id，AIP-155）：先查再驗證。
func TestCreateMeasureRequestIDIsIdempotent(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	created := record[MeasureCreated](f)
	suspended := record[StoreSuspended](f)
	accountID, s := f.open(t)
	review := f.now.Add(7 * 24 * time.Hour)
	requestID := uuid.New()
	in := CreateMeasureInput{
		AdminID: 7, RequestID: &requestID, StoreID: s.StoreID, Kind: MeasureSuspension, StatementToSeller: "疑似詐欺", Note: "內部", ReviewBy: &review,
		Risk: &RiskBasis{Ground: RiskFraud},
	}
	first, err := f.module.CreateMeasure(ctx, in)
	must(t, err)
	if first.Replayed {
		t.Fatal("第一次不是重送")
	}

	// 同 id、同內容：回傳同一個措施、一列、事件一次；奈秒精度不同視為相同（正規化到微秒）。
	nano := review.Add(300 * time.Nanosecond)
	same := in
	same.ReviewBy = &nano
	again, err := f.module.CreateMeasure(ctx, same)
	must(t, err)
	if !again.Replayed || again.Measure.ID != first.Measure.ID || again.Measure.Version != first.Measure.Version {
		t.Fatalf("重送要回傳原本的措施：%+v", again)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.measures"); n != 1 || len(created.all()) != 1 || len(suspended.all()) != 1 {
		t.Fatalf("measures=%d created=%d suspended=%d", n, len(created.all()), len(suspended.all()))
	}

	// 建立之後措施被改過：重送回傳措施「目前的狀態」與目前的 etag（AIP-155 的 stale-success 例外）。
	statement := "改過的理由"
	updated, err := f.module.UpdateMeasure(ctx, UpdateMeasureInput{MeasureID: first.Measure.ID, AdminID: 8, ExpectedVersion: first.Measure.Version, StatementToSeller: &statement})
	must(t, err)
	if r, err := f.module.CreateMeasure(ctx, in); err != nil || !r.Replayed || r.Measure.Version != updated.Version || r.Measure.StatementToSeller != statement {
		t.Fatalf("重送回傳目前的狀態：%+v err=%v", r, err)
	}

	// 先建立、再關店、再同 id 重送：回傳原措施，不跑有狀態的驗證。
	_, err = f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	if r, err := f.module.CreateMeasure(ctx, in); err != nil || !r.Replayed || r.Measure.ID != first.Measure.ID {
		t.Fatalf("關店之後重送：%+v err=%v", r, err)
	}
	// 把複審期限推到過去（時間過了三十天）。
	f.now = f.now.Add(40 * 24 * time.Hour)
	if r, err := f.module.CreateMeasure(ctx, in); err != nil || !r.Replayed || r.Measure.ID != first.Measure.ID {
		t.Fatalf("複審期限過了之後重送：%+v err=%v", r, err)
	}
	// 沒有 request_id 的同樣內容，這時候是新的請求：店家已關閉。
	noID := in
	noID.RequestID = nil
	if _, err := f.module.CreateMeasure(ctx, noID); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("沒有 request_id 的請求照常驗證：%v", err)
	}

	// 同 id、不同內容：invalid_argument，欄位 RequestID。
	different := in
	different.StatementToSeller = "不一樣的理由"
	_, err = f.module.CreateMeasure(ctx, different)
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind() != apperr.KindInvalidInput || len(ae.FieldViolations()) != 1 || ae.FieldViolations()[0].Field != "RequestID" {
		t.Fatalf("同 id 不同內容：%v", err)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.measures"); n != 1 {
		t.Fatalf("measures=%d", n)
	}
}

// request_id 的範圍是 (管理員, request_id)：另一位管理員用同一個 id 各自建立；驗證失敗不留紀錄，重送照常驗證。
func TestCreateMeasureRequestIDIsScopedToTheAdminAndFailuresLeaveNoRecord(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	requestID := uuid.New()
	in := f.warningFrom(s.StoreID)
	in.RequestID = &requestID
	invalid := in
	invalid.StatementToSeller = " "
	if _, err := f.module.CreateMeasure(ctx, invalid); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("invalid: %v", err)
	}
	if f.count(t, "SELECT count(*) FROM seller.measures WHERE request_id IS NOT NULL") != 0 {
		t.Fatal("驗證失敗不能留下 request_id")
	}
	a, err := f.module.CreateMeasure(ctx, in)
	must(t, err)
	other := in
	other.AdminID = 8
	b, err := f.module.CreateMeasure(ctx, other)
	must(t, err)
	if a.Measure.ID == b.Measure.ID || b.Replayed || f.count(t, "SELECT count(*) FROM seller.measures") != 2 {
		t.Fatalf("另一位管理員用同 id 要各自建立：%+v %+v", a, b)
	}
}

// 兩個並行的相同請求（帶同一個 request_id）只寫一列：兩個都在鎖上等著，後到的撞到唯一索引後回到查詢，回傳原措施。
func TestConcurrentCreateMeasureWithTheSameRequestIDWritesOneRow(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	requestID := uuid.New()
	tx, err := f.db.Pool().Begin(ctx)
	must(t, err)
	_, err = tx.Exec(ctx, "SELECT 1 FROM seller.stores WHERE store_id=$1 FOR UPDATE", s.StoreID)
	must(t, err)
	results := make(chan CreateMeasureResult, 2)
	errs := make(chan error, 2)
	for range 2 {
		go func() {
			in := f.warningFrom(s.StoreID)
			in.RequestID = &requestID
			r, err := f.module.CreateMeasure(ctx, in)
			results <- r
			errs <- err
		}()
	}
	f.waitForLockWaiters(t, 2)
	must(t, tx.Rollback(ctx))
	replayed := 0
	ids := map[uuid.UUID]bool{}
	for range 2 {
		must(t, <-errs)
		r := <-results
		ids[r.Measure.ID] = true
		if r.Replayed {
			replayed++
		}
	}
	if f.count(t, "SELECT count(*) FROM seller.measures") != 1 || len(ids) != 1 || replayed != 1 {
		t.Fatalf("rows=%d ids=%v replayed=%d", f.count(t, "SELECT count(*) FROM seller.measures"), ids, replayed)
	}
}

// cascade 建立：M1 建立認定 F（停權 30 天）；M2 建立認定 G 並引用 F（停權 30 天）。兩個措施同時生效。
func (f *fixture) cascadeWorld(t *testing.T) (accountID int64, s Store, m1, m2 Measure) {
	t.Helper()
	accountID, s = f.open(t)
	ends := f.now.Add(30 * 24 * time.Hour)
	m1 = f.violate(t, s.StoreID, SeverityHigh, MeasureSuspension, &ends)
	r, err := f.module.CreateMeasure(context.Background(), CreateMeasureInput{
		AdminID: 7, StoreID: s.StoreID, Kind: MeasureSuspension, StatementToSeller: "加重", Note: "內部", EndsAt: &ends,
		Violation: &ViolationBasis{Finding: &NewFinding{Category: policy.Fraud, Severity: SeverityHigh}, CitedFindingIDs: []uuid.UUID{m1.Findings[0].ID}},
	})
	must(t, err)
	m2 = r.Measure
	if len(m2.Findings) != 2 || m2.Findings[1].ID != m1.Findings[0].ID || m2.Findings[0].OriginMeasureID != m2.ID {
		t.Fatalf("m2 的依據：新的認定第一、引用的第二：%+v", m2.Findings)
	}
	return accountID, s, m1, m2
}

func (f *fixture) findingStatus(t *testing.T, id uuid.UUID) (ViolationStatus, *RevokeReason) {
	t.Helper()
	v, err := f.module.Violation(context.Background(), id)
	must(t, err)
	return v.Status, v.RevokedReason
}

// 措施的跨表不變量：接受申覆 ⇔ APPEAL_ACCEPTED；FINDING_REVOKED ⇒ 依據裡有被撤銷的認定；撤銷的措施沒有待處理的請求。
func (f *fixture) assertMeasureInvariants(t *testing.T) {
	t.Helper()
	for name, query := range map[string]string{
		"接受申訴 ⇔ APPEAL_ACCEPTED": `SELECT count(*) FROM seller.measures m
			WHERE EXISTS (SELECT 1 FROM seller.measure_requests r WHERE r.measure_id = m.id AND r.kind = 'APPEAL' AND r.state = 'ACCEPTED') <> (m.revoked_reason IS NOT DISTINCT FROM 'APPEAL_ACCEPTED')`,
		"接受補正 ⇒ 措施未撤銷、已結束": `SELECT count(*) FROM seller.measures m WHERE EXISTS (SELECT 1 FROM seller.measure_requests r WHERE r.measure_id = m.id AND r.kind = 'REMEDIATION' AND r.state = 'ACCEPTED')
			AND (m.status <> 'ENFORCED' OR m.ends_at IS NULL)`,
		"FINDING_REVOKED ⇒ 依據裡有被撤銷的認定": `SELECT count(*) FROM seller.measures m WHERE m.revoked_reason = 'FINDING_REVOKED'
			AND NOT EXISTS (SELECT 1 FROM seller.measure_findings mf JOIN seller.violations v ON v.id = mf.violation_id WHERE mf.measure_id = m.id AND v.status = 'REVOKED')`,
		"撤銷的措施沒有待處理的請求": `SELECT count(*) FROM seller.measures m WHERE m.status = 'REVOKED'
			AND EXISTS (SELECT 1 FROM seller.measure_requests r WHERE r.measure_id = m.id AND r.state = 'PENDING')`,
	} {
		if n := f.count(t, query); n != 0 {
			t.Errorf("不變量被破壞（%s）：%d", name, n)
		}
	}
}

// 連帶撤銷：F 由 M1 建立、被 M2 引用（M2 另引用 G 是自己的）；接受 M1 的申覆且 revoke_findings=true → M2 以 FINDING_REVOKED 撤銷、
// 效果解除，StoreUnsuspended 只發一次（M1、M2 都是停權時），M2 自己建立的認定 G 仍生效。
func TestAcceptingAnAppealWithRevokeFindingsRevokesTheMeasuresThatCiteTheFinding(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	unsuspended := record[StoreUnsuspended](f)
	revokedEvents := record[MeasureRevoked](f)
	accountID, s, m1, m2 := f.cascadeWorld(t)
	f.appeal(t, accountID, m1, "誤判")
	admin := f.measureOf(t, m1.ID)
	accepted, err := f.module.DecideAppeal(ctx, DecideAppealInput{
		MeasureID: m1.ID, AdminID: 9, ExpectedVersion: admin.Version, Outcome: RequestAccepted, Reason: "查證不屬實", RevokeFindings: true,
	})
	must(t, err)
	if accepted.Status != MeasureStatusRevoked || *accepted.RevokedReason != RevokeAppealAccepted || accepted.Appeal().State != RequestAccepted {
		t.Fatalf("accepted=%+v", accepted)
	}
	cascaded := f.measureOf(t, m2.ID)
	if cascaded.Status != MeasureStatusRevoked || *cascaded.RevokedReason != RevokeFindingRevoked || *cascaded.RevokedByAdminID != 9 || cascaded.State != StateRevoked ||
		cascaded.Version != m2.Version+1 {
		t.Fatalf("m2=%+v", cascaded)
	}
	if status, reason := f.findingStatus(t, m1.Findings[0].ID); status != ViolationStatusRevoked || *reason != RevokeAppealAccepted {
		t.Fatalf("F=%s %v", status, reason)
	}
	if status, _ := f.findingStatus(t, m2.Findings[0].ID); status != ViolationStatusEnforced {
		t.Fatalf("G 是 M2 自己建立的，不因為引用的 F 被撤銷而撤銷：%s", status)
	}
	if f.isSuspended(t, s.StoreID) {
		t.Fatal("兩個措施都撤銷了，效果要解除")
	}
	if got := unsuspended.all(); len(got) != 1 || f.openPeriods(t) != 0 {
		t.Fatalf("StoreUnsuspended 只發一次：%+v", got)
	}
	reasons := map[uuid.UUID]RevokeReason{}
	for _, e := range revokedEvents.all() {
		reasons[e.MeasureID] = e.Reason
	}
	if len(revokedEvents.all()) != 2 || reasons[m1.ID] != RevokeAppealAccepted || reasons[m2.ID] != RevokeFindingRevoked {
		t.Fatalf("MeasureRevoked=%+v", revokedEvents.all())
	}
	f.assertMeasureInvariants(t)
}

// 被連帶撤銷的措施上待處理的申覆以 CLOSED 結案（不帶理由），發 MeasureRequestClosed。
func TestCascadeClosesThePendingAppealOfTheRevokedMeasure(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	closed := record[MeasureRequestClosed](f)
	accountID, _, m1, m2 := f.cascadeWorld(t)
	f.appeal(t, accountID, m2, "我也要申訴")
	revoked, err := f.module.RevokeMeasure(ctx, RevokeMeasureInput{MeasureID: m1.ID, ExpectedVersion: m1.Version, AdminID: 9, Reason: RevokeAdminCorrection, RevokeFindings: true})
	must(t, err)
	if *revoked.RevokedReason != RevokeAdminCorrection {
		t.Fatalf("revoked=%+v", revoked)
	}
	got := f.measureOf(t, m2.ID)
	appeal := got.Appeal()
	if got.Status != MeasureStatusRevoked || *got.RevokedReason != RevokeFindingRevoked || appeal.State != RequestClosed || appeal.DecisionReason != nil ||
		appeal.DecidedByAdminID == nil || *appeal.DecidedByAdminID != 9 || appeal.DecidedAt == nil || got.CanDecideAppeal() || got.CanAppeal() {
		t.Fatalf("m2=%+v appeal=%+v", got, appeal)
	}
	if events := closed.all(); len(events) != 1 || events[0].MeasureID != m2.ID || events[0].RequestID != appeal.ID {
		t.Fatalf("MeasureRequestClosed=%+v", events)
	}
	f.assertMeasureInvariants(t)
}

func TestRevokeWithoutRevokeFindingsLeavesTheCitingMeasuresAlone(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s, m1, m2 := f.cascadeWorld(t)
	_, err := f.module.RevokeMeasure(ctx, RevokeMeasureInput{MeasureID: m1.ID, ExpectedVersion: m1.Version, AdminID: 9, Reason: RevokeAdminCorrection})
	must(t, err)
	after := f.measureOf(t, m2.ID)
	if after.Status != MeasureStatusEnforced || after.Version != m2.Version || after.State != StateActive || !f.isSuspended(t, s.StoreID) {
		t.Fatalf("m2=%+v", after)
	}
	if status, _ := f.findingStatus(t, m1.Findings[0].ID); status != ViolationStatusEnforced {
		t.Fatalf("revoke_findings=false 時認定不動：%s", status)
	}
	_ = accountID
}

// 撤銷的輸入規則：原因不能空，也不能是系統寫入的三個值（模組自己擋，不依賴傳輸層的 AdminRevokeReason）；RISK 措施沒有自己建立的認定，revoke_findings 不能設；被撤銷的認定
// 不計入升級。
func TestRevokeInputRules(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	risk := f.riskSuspend(t, s.StoreID)
	warning := f.violate(t, s.StoreID, SeverityLow, MeasureWarning, nil)
	for name, in := range map[string]RevokeMeasureInput{
		"不認得的原因":        {MeasureID: warning.ID, ExpectedVersion: warning.Version, AdminID: 7, Reason: RevokeReason("99")},
		"沒有原因":          {MeasureID: warning.ID, ExpectedVersion: warning.Version, AdminID: 7},
		"接受申訴只由裁定寫入":    {MeasureID: warning.ID, ExpectedVersion: warning.Version, AdminID: 7, Reason: RevokeAppealAccepted},
		"連帶撤銷的原因不能手動選":  {MeasureID: warning.ID, ExpectedVersion: warning.Version, AdminID: 7, Reason: RevokeFindingRevoked},
		"來源撤銷的原因不能手動選":  {MeasureID: warning.ID, ExpectedVersion: warning.Version, AdminID: 7, Reason: RevokeSourceRevoked},
		"RISK 沒有可撤銷的認定": {MeasureID: risk.ID, ExpectedVersion: risk.Version, AdminID: 7, Reason: RevokeOther, RevokeFindings: true},
	} {
		if _, err := f.module.RevokeMeasure(ctx, in); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：%v", name, err)
		}
	}
	if m := f.measureOf(t, warning.ID); m.Status != MeasureStatusEnforced || m.Version != warning.Version {
		t.Fatalf("被拒絕的撤銷不能寫入：%+v", m)
	}
	revoked, err := f.module.RevokeMeasure(ctx, RevokeMeasureInput{MeasureID: risk.ID, ExpectedVersion: risk.Version, AdminID: 7, Reason: RevokeOther})
	must(t, err)
	if _, err := f.module.RevokeMeasure(ctx, RevokeMeasureInput{MeasureID: risk.ID, ExpectedVersion: revoked.Version, AdminID: 7, Reason: RevokeOther}); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("已撤銷：%v", err)
	}
	if _, err := f.module.RevokeMeasure(ctx, RevokeMeasureInput{MeasureID: uuid.NewV7(), ExpectedVersion: 1, AdminID: 7, Reason: RevokeOther}); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("不存在：%v", err)
	}
}

// 已撤銷的措施：連「沒有任何改動」的修改也回已撤銷（直接看狀態，不靠原因清單的順序），不寫入；訊息說已撤銷。
func TestUpdateRevokedMeasureIsRejectedEvenWhenNothingChanges(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	warning := f.violate(t, s.StoreID, SeverityLow, MeasureWarning, nil)
	revoked, err := f.module.RevokeMeasure(ctx, RevokeMeasureInput{MeasureID: warning.ID, ExpectedVersion: warning.Version, AdminID: 7, Reason: RevokeOther})
	must(t, err)
	same := revoked.StatementToSeller
	changed := "改過的理由"
	for name, in := range map[string]UpdateMeasureInput{
		"沒有任何改動":  {MeasureID: warning.ID, AdminID: 7, ExpectedVersion: revoked.Version, StatementToSeller: &same},
		"改給賣家的理由": {MeasureID: warning.ID, AdminID: 7, ExpectedVersion: revoked.Version, StatementToSeller: &changed},
	} {
		_, err := f.module.UpdateMeasure(ctx, in)
		if apperr.KindOf(err) != apperr.KindFailedPrecondition || err == nil || !strings.Contains(err.Error(), "已撤銷") {
			t.Errorf("%s：%v", name, err)
		}
	}
	if got := f.measureOf(t, warning.ID); got.Version != revoked.Version {
		t.Fatalf("被拒絕的修改不能寫入：version=%d→%d", revoked.Version, got.Version)
	}
}

// 撤銷自己建立的認定時，只撤銷自己建立的，引用來的不動；接受申覆 ⇔ APPEAL_ACCEPTED。
func TestRevokeFindingsOnlyRevokesTheFindingsTheMeasureCreated(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, _, m1, m2 := f.cascadeWorld(t)
	_, err := f.module.RevokeMeasure(ctx, RevokeMeasureInput{MeasureID: m2.ID, ExpectedVersion: m2.Version, AdminID: 9, Reason: RevokeExternalDecision, RevokeFindings: true})
	must(t, err)
	if status, reason := f.findingStatus(t, m2.Findings[0].ID); status != ViolationStatusRevoked || *reason != RevokeExternalDecision {
		t.Fatalf("G 是 M2 自己建立的，要撤銷：%s %v", status, reason)
	}
	if status, _ := f.findingStatus(t, m1.Findings[0].ID); status != ViolationStatusEnforced {
		t.Fatalf("F 是 M1 建立的，M2 引用而已，不能因為撤銷 M2 被撤銷：%s", status)
	}
	if m := f.measureOf(t, m1.ID); m.Status != MeasureStatusEnforced {
		t.Fatalf("m1=%+v", m)
	}
	f.assertMeasureInvariants(t)
}

// can_revoke_findings 是 false（只引用別的措施建立的認定）時，revoke_findings=true 在撤銷與接受申覆都被拒絕，什麼都不寫；不帶 revoke_findings 照常撤銷。
// 自己建立了認定的措施（m2）是 true。
func TestRevokeFindingsIsRejectedWhenTheMeasureOnlyCitesOthersFindings(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s, m1, m2 := f.cascadeWorld(t)
	ends := f.now.Add(30 * 24 * time.Hour)
	r, err := f.module.CreateMeasure(ctx, CreateMeasureInput{
		AdminID: 7, StoreID: s.StoreID, Kind: MeasureSuspension, StatementToSeller: "只引用", Note: "內部", EndsAt: &ends,
		Violation: &ViolationBasis{CitedFindingIDs: []uuid.UUID{m1.Findings[0].ID}},
	})
	must(t, err)
	m3 := r.Measure
	if m3.Basis != BasisViolation || m3.CanRevokeFindings() || !m2.CanRevokeFindings() || !m1.CanRevokeFindings() {
		t.Fatalf("m3 只引用別人的認定：m3=%+v can(m1,m2)=%v %v", m3, m1.CanRevokeFindings(), m2.CanRevokeFindings())
	}
	if _, err := f.module.RevokeMeasure(ctx, RevokeMeasureInput{MeasureID: m3.ID, ExpectedVersion: m3.Version, AdminID: 9, Reason: RevokeOther, RevokeFindings: true}); apperr.KindOf(err) != apperr.KindInvalidInput || !slices.Equal(errField(err), []string{"RevokeFindings"}) {
		t.Fatalf("撤銷：%v %v", err, errField(err))
	}
	f.appeal(t, accountID, m3, "申訴")
	pending := f.measureOf(t, m3.ID)
	if _, err := f.module.DecideAppeal(ctx, DecideAppealInput{MeasureID: m3.ID, ExpectedVersion: pending.Version, AdminID: 9, Outcome: RequestAccepted, Reason: "查證不屬實", RevokeFindings: true}); apperr.KindOf(err) != apperr.KindInvalidInput || !slices.Equal(errField(err), []string{"RevokeFindings"}) {
		t.Fatalf("接受申訴：%v %v", err, errField(err))
	}
	if after := f.measureOf(t, m3.ID); after.Status != MeasureStatusEnforced || after.Version != pending.Version {
		t.Fatalf("被拒絕的請求不能寫入：%+v", after)
	}
	if status, _ := f.findingStatus(t, m1.Findings[0].ID); status != ViolationStatusEnforced {
		t.Fatalf("別人建立的認定不動：%s", status)
	}
	if _, err := f.module.DecideAppeal(ctx, DecideAppealInput{MeasureID: m3.ID, ExpectedVersion: pending.Version, AdminID: 9, Outcome: RequestAccepted, Reason: "查證不屬實"}); err != nil {
		t.Fatalf("不帶 revoke_findings 的接受照常：%v", err)
	}
	f.assertMeasureInvariants(t)
}

// 每個動作的前置條件由 model.Measure 的 Can* 一份定義，API 的 can_* 與方法共用：對每個狀態（警告、停權；依據認定、風險類型；
// 生效中、已結束、已撤銷；有無申覆）逐一嘗試每個動作，「方法成功」要等於「can_X」。
func TestCanFlagsMatchWhetherTheActionSucceeds(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	base := f.now
	type scenario struct {
		name  string
		clock time.Duration
		build func() (int64, Measure)
	}
	long := func() *time.Time { return ptr(base.Add(30 * 24 * time.Hour)) }
	violation := func(kind MeasureKind, ends *time.Time) func() (int64, Measure) {
		return func() (int64, Measure) {
			accountID, s := f.open(t)
			return accountID, f.violate(t, s.StoreID, SeverityHigh, kind, ends)
		}
	}
	risk := func(ends *time.Time) func() (int64, Measure) {
		return func() (int64, Measure) {
			accountID, s := f.open(t)
			review := base.Add(7 * 24 * time.Hour)
			r, err := f.module.CreateMeasure(ctx, CreateMeasureInput{
				AdminID: 7, StoreID: s.StoreID, Kind: MeasureSuspension, StatementToSeller: "風險", Note: "n", ReviewBy: &review, EndsAt: ends,
				Risk: &RiskBasis{Ground: RiskFraud},
			})
			must(t, err)
			return accountID, r.Measure
		}
	}
	termination := func(ground TerminationGround) func() (int64, Measure) {
		return func() (int64, Measure) {
			accountID, s := f.open(t)
			return accountID, f.terminate(t, s.StoreID, ground)
		}
	}
	appealed := func(inner func() (int64, Measure)) func() (int64, Measure) {
		return func() (int64, Measure) {
			accountID, m := inner()
			f.appeal(t, accountID, m, "申訴")
			return accountID, f.measureOf(t, m.ID)
		}
	}
	rejected := func(inner func() (int64, Measure)) func() (int64, Measure) {
		return func() (int64, Measure) {
			accountID, m := appealed(inner)()
			_, err := f.module.DecideAppeal(ctx, DecideAppealInput{MeasureID: m.ID, AdminID: 7, ExpectedVersion: m.Version, Outcome: RequestRejected, Reason: "不成立"})
			must(t, err)
			return accountID, f.measureOf(t, m.ID)
		}
	}
	short := func() *time.Time { return ptr(base.Add(time.Hour)) }
	remediating := func(inner func() (int64, Measure)) func() (int64, Measure) {
		return func() (int64, Measure) {
			accountID, m := inner()
			f.remediate(t, accountID, m.ID, "補正")
			return accountID, f.measureOf(t, m.ID)
		}
	}
	rejectedRemediation := func(inner func() (int64, Measure)) func() (int64, Measure) {
		return func() (int64, Measure) {
			accountID, m := remediating(inner)()
			_, err := f.decideRemediation(m.ID, 7, RequestRejected, "佐證不足")
			must(t, err)
			return accountID, f.measureOf(t, m.ID)
		}
	}
	inCooldown := func(inner func() (int64, Measure)) func() (int64, Measure) {
		return func() (int64, Measure) {
			accountID, m := inner()
			for i, kind := range []string{"REMEDIATION", "APPEAL", "REMEDIATION"} {
				_, err := f.db.Conn(ctx).Exec(ctx, `INSERT INTO seller.measure_requests (id, measure_id, kind, state, reason, revision, submitted_at, decision_reason, decided_by_admin_id, decided_at, basis)
					VALUES (uuidv7(), $1, $2, 'REJECTED', 'x', 1, $3, 'x', 7, $3, 'RISK')`, m.ID, kind, base.Add(-time.Duration(3-i)*time.Hour))
				must(t, err)
			}
			return accountID, f.measureOf(t, m.ID)
		}
	}
	scenarios := []scenario{
		{"警告", 0, violation(MeasureWarning, nil)},
		{"認定停權", 0, violation(MeasureSuspension, long())},
		{"認定停權，申訴待裁定", 0, appealed(violation(MeasureSuspension, long()))},
		{"認定停權，申訴已駁回", 0, rejected(violation(MeasureSuspension, long()))},
		{"風險停權，沒有固定期間", 0, risk(nil)},
		{"風險停權，沒有固定期間，申訴待裁定", 0, appealed(risk(nil))},
		{"風險停權，有結束時間", 0, risk(long())},
		{"風險停權，補正待處理", 0, remediating(risk(nil))},
		{"風險停權有結束時間，補正待處理", 0, remediating(risk(long()))},
		{"風險停權，補正已駁回", 0, rejectedRemediation(risk(nil))},
		{"風險停權，申訴已駁回", 0, rejected(risk(nil))},
		{"風險停權，三次駁回後冷卻中", 0, inCooldown(risk(nil))},
		{"預告終止，預定生效", 0, termination(GroundNotice)},
		{"預告終止，預定生效，申訴待裁定", 0, appealed(termination(GroundNotice))},
		{"預告終止，生效中", 721 * time.Hour, termination(GroundNotice)},
		{"預告終止，申覆期限的最後一刻前", 720*time.Hour + 4416*time.Hour - time.Microsecond, termination(GroundNotice)},
		{"預告終止，申覆期限已過", 720*time.Hour + 4416*time.Hour, termination(GroundNotice)},
		{"預告終止，店家關閉而結束", 25 * time.Hour, func() (int64, Measure) {
			accountID, m := termination(GroundNotice)()
			f.now = base.Add(24 * time.Hour)
			_, err := f.module.CloseCurrent(ctx, accountID)
			must(t, err)
			f.now = base
			return accountID, f.measureOf(t, m.ID)
		}},
		{"21.3 終止，生效中", 0, termination(GroundCeasedBusiness)},
		{"21.3 終止，申訴已駁回", 0, rejected(termination(GroundFraud))},
		{"21.3 終止，申覆期限已過", 4416*time.Hour + time.Second, termination(GroundCeasedBusiness)},
		{"風險停權，補正待處理而措施已結束", 2 * time.Hour, remediating(risk(short()))},
		{"風險停權，補正待處理，縮短後已結束", 2 * time.Hour, func() (int64, Measure) {
			accountID, m := remediating(risk(long()))()
			_, err := f.module.UpdateMeasure(ctx, UpdateMeasureInput{MeasureID: m.ID, AdminID: 7, ExpectedVersion: m.Version, ChangeEndsAt: true, EndsAt: short()})
			must(t, err)
			return accountID, f.measureOf(t, m.ID)
		}},
		{"認定停權，已結束", 2 * time.Hour, violation(MeasureSuspension, short())},
		{"認定停權，已結束後才申訴", 2 * time.Hour, func() (int64, Measure) {
			accountID, m := violation(MeasureSuspension, short())()
			f.now = base.Add(2 * time.Hour)
			defer func() { f.now = base }()
			f.appeal(t, accountID, f.measureOf(t, m.ID), "已結束仍申訴")
			return accountID, f.measureOf(t, m.ID)
		}},
		{"風險停權，已由平台結束", time.Microsecond, func() (int64, Measure) {
			accountID, m := risk(nil)()
			return accountID, f.endRisk(t, m)
		}},
		{"已撤銷", 0, func() (int64, Measure) {
			accountID, m := violation(MeasureSuspension, long())()
			_, err := f.module.RevokeMeasure(ctx, RevokeMeasureInput{MeasureID: m.ID, ExpectedVersion: m.Version, AdminID: 7, Reason: RevokeOther})
			must(t, err)
			return accountID, f.measureOf(t, m.ID)
		}},
	}
	type action struct {
		name string
		can  func(m Measure) bool
		run  func(accountID int64, m Measure) error
		skip func(m Measure) bool
	}
	later := func(m Measure) *time.Time {
		if m.EndsAt == nil {
			return nil
		}
		return ptr(m.EndsAt.Add(24 * time.Hour))
	}
	actions := []action{
		{"改理由", Measure.CanUpdateStatement, func(_ int64, m Measure) error {
			_, err := f.module.UpdateMeasure(ctx, UpdateMeasureInput{MeasureID: m.ID, AdminID: 7, ExpectedVersion: m.Version, StatementToSeller: ptr("新的理由")})
			return err
		}, nil},
		{"延長", Measure.CanExtend, func(_ int64, m Measure) error {
			end := later(m)
			if m.State != StateActive || end == nil {
				end = ptr(f.now.Add(24 * time.Hour))
			}
			_, err := f.module.UpdateMeasure(ctx, UpdateMeasureInput{MeasureID: m.ID, AdminID: 7, ExpectedVersion: m.Version, ChangeEndsAt: true, EndsAt: end})
			return err
		}, func(m Measure) bool { return m.State == StateActive && m.EndsAt == nil && m.Kind != MeasureTermination }},
		{"縮短", Measure.CanShorten, func(_ int64, m Measure) error {
			_, err := f.module.UpdateMeasure(ctx, UpdateMeasureInput{MeasureID: m.ID, AdminID: 7, ExpectedVersion: m.Version, ChangeEndsAt: true, EndsAt: ptr(f.now.Add(30 * time.Minute))})
			return err
		}, nil},
		{"結束", Measure.CanEnd, func(_ int64, m Measure) error {
			f.afterCreation(m)
			_, err := f.module.EndMeasure(ctx, EndMeasureInput{MeasureID: m.ID, AdminID: 7, ExpectedVersion: m.Version})
			return err
		}, nil},
		{"撤銷", Measure.CanRevoke, func(_ int64, m Measure) error {
			_, err := f.module.RevokeMeasure(ctx, RevokeMeasureInput{MeasureID: m.ID, AdminID: 7, ExpectedVersion: m.Version, Reason: RevokeOther})
			return err
		}, nil},
		{"重排複審", Measure.CanScheduleReview, func(_ int64, m Measure) error {
			_, err := f.module.ScheduleMeasureReview(ctx, ScheduleReviewInput{MeasureID: m.ID, AdminID: 7, ExpectedVersion: m.Version, ReviewBy: f.now.Add(3 * 24 * time.Hour), Note: "延後"})
			return err
		}, nil},
		{"裁定申訴", Measure.CanDecideAppeal, func(_ int64, m Measure) error {
			_, err := f.module.DecideAppeal(ctx, DecideAppealInput{MeasureID: m.ID, AdminID: 7, ExpectedVersion: m.Version, Outcome: RequestRejected, Reason: "不成立"})
			return err
		}, nil},
		{"賣家申訴", Measure.CanAppeal, func(accountID int64, m Measure) error {
			_, err := f.module.SubmitAppeal(ctx, SubmitAppealInput{AccountID: accountID, MeasureID: m.ID, ExpectedVersion: m.SellerVersion(), Reason: "申訴"})
			return err
		}, nil},
		{"賣家請求補正", Measure.CanRequestRemediation, func(accountID int64, m Measure) error {
			_, err := f.module.RequestRemediation(ctx, RequestRemediationInput{AccountID: accountID, MeasureID: m.ID, ExpectedVersion: m.SellerVersion(), Reason: "補正"})
			return err
		}, nil},
		{"裁定補正", Measure.CanDecideRemediation, func(_ int64, m Measure) error {
			_, err := f.module.DecideRemediation(ctx, DecideRemediationInput{MeasureID: m.ID, AdminID: 7, ExpectedVersion: m.Version, Outcome: RequestRejected, Reason: "不足"})
			return err
		}, nil},
	}
	// 賣家的兩個請求被拒絕時，錯誤的訊息必須是可用性清單第一筆的句子（同一個函式）；可以做時清單是空的。
	firstMessages := map[string]func(m Measure) string{
		"賣家申訴": func(m Measure) string {
			if us := m.AppealUnavailabilities(uitext.ZhTW); len(us) > 0 {
				return us[0].Message.Text
			}
			return ""
		},
		"賣家請求補正": func(m Measure) string {
			if us := m.RemediationUnavailabilities(uitext.ZhTW); len(us) > 0 {
				return us[0].Message.Text
			}
			return ""
		},
	}
	for _, sc := range scenarios {
		for _, a := range actions {
			f.now = base
			accountID, built := sc.build()
			f.now = base.Add(sc.clock)
			m := f.measureOf(t, built.ID)
			if a.skip != nil && a.skip(m) {
				continue
			}
			want := a.can(m)
			err := a.run(accountID, m)
			if (err == nil) != want {
				t.Errorf("%s／%s：can=%v，方法結果=%v", sc.name, a.name, want, err)
			}
			if err != nil && apperr.KindOf(err) == apperr.KindInternal {
				t.Errorf("%s／%s：不預期的內部錯誤 %v", sc.name, a.name, err)
			}
			if firstMessage := firstMessages[a.name]; firstMessage != nil {
				first := firstMessage(m)
				var ae *apperr.Error
				switch {
				case want && first != "":
					t.Errorf("%s／%s：可以做卻列出原因 %q", sc.name, a.name, first)
				case !want && first == "":
					t.Errorf("%s／%s：不能做卻沒有原因", sc.name, a.name)
				case !want && (!errors.As(err, &ae) || ae.Message != first):
					t.Errorf("%s／%s：被拒絕的錯誤文字 %v 不是可用性清單第一筆的句子 %q", sc.name, a.name, err, first)
				}
			}
		}
	}
	f.now = base

	// 必含的格。
	_, ended := violation(MeasureSuspension, short())()
	f.now = base.Add(2 * time.Hour)
	ended = f.measureOf(t, ended.ID)
	if ended.State != StateEnded || ended.CanExtend() || ended.CanEnd() || !ended.CanAppeal() {
		t.Fatalf("ENDED：不能延長、不能 End、可以申訴：%+v", ended)
	}
	if _, err := f.module.UpdateMeasure(ctx, UpdateMeasureInput{MeasureID: ended.ID, AdminID: 7, ExpectedVersion: ended.Version, ChangeEndsAt: true, EndsAt: ptr(f.now.Add(24 * time.Hour))}); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("ENDED 時延長要 failed_precondition：%v", err)
	}
	f.now = base
	accountID, pending := appealed(violation(MeasureSuspension, long()))()
	_ = accountID
	if pending.CanUpdateStatement() || pending.CanExtend() || !pending.CanShorten() || pending.CanRevoke() || !pending.CanDecideAppeal() || pending.CanAppeal() {
		t.Fatalf("申訴待裁定：凍結理由與延長、只能縮短、撤銷只經裁定：%+v", pending)
	}
	_, active := violation(MeasureSuspension, long())()
	if _, err := f.module.EndMeasure(ctx, EndMeasureInput{MeasureID: active.ID, AdminID: 7, ExpectedVersion: active.Version}); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("依據認定的措施不能 End：%v", err)
	}
}

func errField(err error) []string {
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		return nil
	}
	var out []string
	for _, fv := range ae.FieldViolations() {
		out = append(out, fv.Field)
	}
	return out
}

// 結束時間的規則（G7）：新的結束時間必須晚於現在；沒有固定期間的 RISK 改成有期限算縮短、清除 RISK 的結束時間算延長；依據認定的
// 停權不能清除；警告不能設。
func TestEndTimeRules(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	update := func(m Measure, ends *time.Time) (Measure, error) {
		return f.module.UpdateMeasure(ctx, UpdateMeasureInput{MeasureID: m.ID, AdminID: 7, ExpectedVersion: f.measureOf(t, m.ID).Version, ChangeEndsAt: true, EndsAt: ends})
	}
	_, s := f.open(t)
	ends := f.now.Add(24 * time.Hour)
	suspension := f.violate(t, s.StoreID, SeverityHigh, MeasureSuspension, &ends)
	for name, at := range map[string]time.Time{"現在": f.now, "過去": f.now.Add(-time.Hour)} {
		if _, err := update(suspension, &at); apperr.KindOf(err) != apperr.KindInvalidInput || !slices.Equal(errField(err), []string{"EndsAt"}) {
			t.Errorf("結束時間在%s：%v", name, err)
		}
	}
	if _, err := update(suspension, nil); apperr.KindOf(err) != apperr.KindInvalidInput || !slices.Equal(errField(err), []string{"EndsAt"}) {
		t.Errorf("依據認定的停權不能清除結束時間：%v", err)
	}
	warning := f.violate(t, s.StoreID, SeverityLow, MeasureWarning, nil)
	// 結束時間不能調的措施先走可用性：failed_precondition，LocalizedMessage 等於 API 對應動作的第一個原因的文字（一個事實一句話），不是另一句 invalid_argument。
	unavailable := func(name string, m Measure, ends *time.Time, action MeasureAction) {
		t.Helper()
		_, err := update(m, ends)
		var ae *apperr.Error
		reasons := f.measureOf(t, m.ID).ActionUnavailableReasons(action)
		if apperr.KindOf(err) != apperr.KindFailedPrecondition || len(errField(err)) != 0 || len(reasons) == 0 || !errors.As(err, &ae) ||
			ae.Message != ActionUnavailableText(reasons[0], action).Text {
			t.Errorf("%s：want failed_precondition with %v's text, got %v", name, reasons, err)
		}
	}
	unavailable("警告設結束時間", warning, ptr(f.now.Add(time.Hour)), MeasureActionShorten)
	_, ts := f.open(t)
	termination := f.terminate(t, ts.StoreID, GroundCeasedBusiness)
	unavailable("終止合作設結束時間", termination, ptr(f.now.Add(time.Hour)), MeasureActionShorten)
	if same, err := update(warning, nil); err != nil || same.Version != warning.Version {
		t.Errorf("警告清除結束時間是沒有改變：%+v %v", same, err)
	}

	// RISK：無期限 → 有期限是縮短（凍結後仍可）；有期限 → 清除是延長（凍結後不可）。
	accountID, rs := f.open(t)
	risk := f.riskSuspend(t, rs.StoreID)
	shortened, err := update(risk, ptr(f.now.Add(48*time.Hour)))
	must(t, err)
	if shortened.EndsAt == nil || shortened.CurrentRevision != risk.CurrentRevision+1 {
		t.Fatalf("shortened=%+v", shortened)
	}
	appealed := f.appeal(t, accountID, shortened, "申訴")
	_ = appealed
	if _, err := update(shortened, nil); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("凍結後清除 RISK 的結束時間是延長：%v", err)
	}
	cleared, err := f.module.UpdateMeasure(ctx, UpdateMeasureInput{MeasureID: risk.ID, AdminID: 7, ExpectedVersion: f.measureOf(t, risk.ID).Version, ChangeEndsAt: true, EndsAt: ptr(f.now.Add(24 * time.Hour))})
	must(t, err)
	if cleared.EndsAt == nil {
		t.Fatal("凍結後仍可縮短")
	}
	_, rs2 := f.open(t)
	free := f.riskSuspend(t, rs2.StoreID)
	withEnd, err := update(free, ptr(f.now.Add(10*24*time.Hour)))
	must(t, err)
	back, err := update(withEnd, nil)
	must(t, err)
	if back.EndsAt != nil || back.CurrentRevision != withEnd.CurrentRevision+1 {
		t.Fatalf("沒有申訴時可以清除 RISK 的結束時間：%+v", back)
	}
}

// 提出申覆起措施凍結：改理由與延長回 failed_precondition（不帶欄位）；縮短、End、重排複審仍可以；沒申覆過的照舊可改。
func TestMeasureIsFrozenFromTheAppealSubmission(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	review := f.now.Add(7 * 24 * time.Hour)
	r, err := f.module.CreateMeasure(ctx, CreateMeasureInput{
		AdminID: 7, StoreID: s.StoreID, Kind: MeasureSuspension, StatementToSeller: "風險", Note: "n", ReviewBy: &review,
		EndsAt: ptr(f.now.Add(10 * 24 * time.Hour)), Risk: &RiskBasis{Ground: RiskFraud},
	})
	must(t, err)
	m := r.Measure

	// 沒申覆過：延長與改理由都可以。
	freeStatement := "沒申訴過可以改的理由"
	free, err := f.module.UpdateMeasure(ctx, UpdateMeasureInput{
		MeasureID: m.ID, AdminID: 7, ExpectedVersion: m.Version, StatementToSeller: &freeStatement, ChangeEndsAt: true, EndsAt: ptr(m.EndsAt.Add(48 * time.Hour)),
	})
	must(t, err)
	if free.StatementToSeller != freeStatement || !free.EndsAt.Equal(m.EndsAt.Add(48*time.Hour)) {
		t.Fatalf("沒申訴過的措施要可以改：%+v", free)
	}

	frozen := func(stage string, cur Measure) Measure {
		t.Helper()
		changed := "加重的理由"
		for name, c := range map[string]struct {
			in     UpdateMeasureInput
			action MeasureAction
		}{
			"extend":    {UpdateMeasureInput{ChangeEndsAt: true, EndsAt: ptr(cur.EndsAt.Add(time.Hour))}, MeasureActionExtend},
			"statement": {UpdateMeasureInput{StatementToSeller: &changed}, MeasureActionUpdateStatement},
			"clear":     {UpdateMeasureInput{ChangeEndsAt: true}, MeasureActionExtend},
		} {
			in := c.in
			in.MeasureID, in.AdminID, in.ExpectedVersion = m.ID, 7, cur.Version
			_, err := f.module.UpdateMeasure(ctx, in)
			if apperr.KindOf(err) != apperr.KindFailedPrecondition || len(errField(err)) != 0 {
				t.Fatalf("%s: %s must be failed_precondition without a field: %v", stage, name, err)
			}
			// 錯誤文字就是 API 回給 client 的那個原因的文字（同一個函式）。
			reasons := cur.ActionUnavailableReasons(c.action)
			var ae *apperr.Error
			if len(reasons) == 0 || !errors.As(err, &ae) || ae.Message != ActionUnavailableText(reasons[0], c.action).Text {
				t.Fatalf("%s: %s error text %v must be the first reason's text (reasons=%v)", stage, name, err, reasons)
			}
		}
		unchanged, ends := cur.StatementToSeller, *cur.EndsAt
		same, err := f.module.UpdateMeasure(ctx, UpdateMeasureInput{MeasureID: m.ID, AdminID: 7, ExpectedVersion: cur.Version, StatementToSeller: &unchanged, ChangeEndsAt: true, EndsAt: &ends})
		must(t, err)
		if same.Version != cur.Version {
			t.Fatalf("%s: resending the current values must stay a no-op: %+v", stage, same)
		}
		shorter := ends.Add(-time.Hour)
		shortened, err := f.module.UpdateMeasure(ctx, UpdateMeasureInput{MeasureID: m.ID, AdminID: 7, ExpectedVersion: cur.Version, ChangeEndsAt: true, EndsAt: &shorter})
		must(t, err)
		if !shortened.EndsAt.Equal(shorter) || shortened.Version != cur.Version+1 {
			t.Fatalf("%s: shortening must stay allowed: %+v", stage, shortened)
		}
		return shortened
	}
	f.appeal(t, accountID, free, "誤判")
	pending := frozen("pending", f.measureOf(t, m.ID))
	rejected, err := f.module.DecideAppeal(ctx, DecideAppealInput{MeasureID: m.ID, ExpectedVersion: pending.Version, AdminID: 7, Outcome: RequestRejected, Reason: "證據不足"})
	must(t, err)
	after := frozen("rejected", rejected)
	reviewed, err := f.module.ScheduleMeasureReview(ctx, ScheduleReviewInput{MeasureID: m.ID, AdminID: 7, ExpectedVersion: after.Version, ReviewBy: f.now.Add(3 * 24 * time.Hour), Note: "延後"})
	must(t, err)
	f.afterCreation(reviewed)
	if _, err := f.module.EndMeasure(ctx, EndMeasureInput{MeasureID: m.ID, AdminID: 7, ExpectedVersion: reviewed.Version}); err != nil {
		t.Fatalf("凍結後仍可 End：%v", err)
	}
}

// 兩種 etag（G10、G11）：寫註記兩種都不變；ScheduleMeasureReview 只改後台的；admin 改理由之後，賣家帶舊 etag 申覆回 VersionConflict。
func TestSellerAndAdminEtagsFollowWhatEachSideSees(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	risk := f.riskSuspend(t, s.StoreID)
	_, err := f.module.CreateMeasureNote(ctx, CreateMeasureNoteInput{MeasureID: risk.ID, AdminID: 9, Body: "補充"})
	must(t, err)
	afterNote := f.measureOf(t, risk.ID)
	if afterNote.Version != risk.Version || afterNote.SellerVersion() != risk.SellerVersion() {
		t.Fatalf("寫註記兩種 etag 都不變：%d/%d → %d/%d", risk.Version, risk.SellerVersion(), afterNote.Version, afterNote.SellerVersion())
	}
	newReview := f.now.Add(10 * 24 * time.Hour)
	reviewed, err := f.module.ScheduleMeasureReview(ctx, ScheduleReviewInput{MeasureID: risk.ID, AdminID: 9, ExpectedVersion: afterNote.Version, ReviewBy: newReview, Note: "延後複審"})
	must(t, err)
	if reviewed.Version != afterNote.Version+1 || reviewed.SellerVersion() != afterNote.SellerVersion() || !reviewed.ReviewBy.Equal(newReview) {
		t.Fatalf("ScheduleMeasureReview 只改後台的 etag：%+v", reviewed)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.governance_notes WHERE measure_id = $1 AND body = '延後複審'", risk.ID); n != 1 {
		t.Fatalf("重排複審同一個 transaction 寫一則註記：%d", n)
	}
	seller, err := f.module.AccountMeasure(ctx, accountID, risk.ID)
	must(t, err)
	if seller.ReviewBy != nil && !seller.ReviewBy.Equal(newReview) {
		t.Fatalf("seller=%+v", seller)
	}
	// admin 改理由 → 賣家的舊 etag 申覆衝突。
	statement := "更新後的理由"
	edited, err := f.module.UpdateMeasure(ctx, UpdateMeasureInput{MeasureID: risk.ID, AdminID: 9, ExpectedVersion: reviewed.Version, StatementToSeller: &statement})
	must(t, err)
	if edited.SellerVersion() == seller.SellerVersion() {
		t.Fatal("改理由要讓賣家的 etag 改變")
	}
	if _, err := f.module.SubmitAppeal(ctx, SubmitAppealInput{AccountID: accountID, MeasureID: risk.ID, ExpectedVersion: seller.SellerVersion(), Reason: "申訴"}); apperr.KindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("舊 etag 申訴：%v", err)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.measure_requests"); n != 0 {
		t.Fatalf("被擋下的申訴不能寫入：%d", n)
	}
}

// 複審期限：RISK 必填，建立與重排時落在 (now, now+30 天]；複審中的 RISK 進收件匣（review_due），只列 RISK、生效中、已進入複審的（review_by 前 24 小時進入，
// 3cd D′11，成員是 seller.review_measure_review），依 review_by 舊到新；重排之後離開收件匣。
func TestReviewDueListsOnlyDueActiveRiskMeasuresOldestFirst(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	mk := func(days float64) Measure {
		_, s := f.open(t)
		review := f.now.Add(time.Duration(days * float64(24*time.Hour)))
		r, err := f.module.CreateMeasure(ctx, CreateMeasureInput{
			AdminID: 7, StoreID: s.StoreID, Kind: MeasureSuspension, StatementToSeller: "風險", Note: "n", ReviewBy: &review, Risk: &RiskBasis{Ground: RiskFraud},
		})
		must(t, err)
		return r.Measure
	}
	later, soon, notDue, ended := mk(2.5), mk(1.5), mk(10), mk(1.5)
	f.endRisk(t, ended)
	_, vs := f.open(t)
	f.violate(t, vs.StoreID, SeverityHigh, MeasureSuspension, ptr(f.now.Add(30*24*time.Hour)))
	due := func() []uuid.UUID {
		t.Helper()
		page, err := f.module.AdminMeasures(ctx, AdminMeasureListInput{ReviewDue: true, PageSize: 10})
		must(t, err)
		var ids []uuid.UUID
		for _, m := range page.Items {
			ids = append(ids, m.ID)
		}
		return ids
	}
	if got := due(); len(got) != 0 {
		t.Fatalf("還沒進入複審（review_by 前 24 小時以前）：%v", got)
	}
	// 提前 24 小時進入：soon 的 review_by 在 1 天後，現在就在收件匣；later 還沒進入。
	f.now = f.now.Add(12 * time.Hour)
	if got := due(); !slices.Equal(got, []uuid.UUID{soon.ID}) {
		t.Fatalf("review_by 前 24 小時進入，還沒到 review_by 也在：%v", got)
	}
	f.now = f.now.Add(36 * time.Hour)
	if got := due(); !slices.Equal(got, []uuid.UUID{soon.ID, later.ID}) {
		t.Fatalf("review_due 只列 RISK、生效中、到期的，依 review_by 舊到新：%v", got)
	}
	page, err := f.module.AdminMeasures(ctx, AdminMeasureListInput{ReviewDue: true, PageSize: 1})
	must(t, err)
	next, err := f.module.AdminMeasures(ctx, AdminMeasureListInput{ReviewDue: true, PageSize: 1, After: page.Next})
	must(t, err)
	if len(page.Items) != 1 || page.Next == nil || len(next.Items) != 1 || next.Items[0].ID != later.ID || next.Next != nil {
		t.Fatalf("keyset：%+v %+v", page, next)
	}
	cur := f.measureOf(t, soon.ID)
	for name, at := range map[string]time.Time{"過去": f.now, "超過三十天": f.now.Add(31 * 24 * time.Hour)} {
		if _, err := f.module.ScheduleMeasureReview(ctx, ScheduleReviewInput{MeasureID: soon.ID, AdminID: 7, ExpectedVersion: cur.Version, ReviewBy: at, Note: "x"}); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("重排到%s：%v", name, err)
		}
	}
	if _, err := f.module.ScheduleMeasureReview(ctx, ScheduleReviewInput{MeasureID: soon.ID, AdminID: 7, ExpectedVersion: cur.Version, ReviewBy: f.now.Add(time.Hour), Note: " "}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Errorf("沒有註記：%v", err)
	}
	if _, err := f.module.ScheduleMeasureReview(ctx, ScheduleReviewInput{MeasureID: soon.ID, AdminID: 7, ExpectedVersion: cur.Version, ReviewBy: f.now.Add(5 * 24 * time.Hour), Note: "再觀察"}); err != nil {
		t.Fatal(err)
	}
	if got := due(); !slices.Equal(got, []uuid.UUID{later.ID}) {
		t.Fatalf("重排之後離開收件匣：%v", got)
	}
	_ = notDue
}

// review_due 與其他篩選一起套用（契約：每個篩選都生效）：店家、帳號、依據、申覆狀態、狀態都縮小到期收件匣的結果。
func TestReviewDueAppliesEveryOtherFilterToo(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	review := f.now.Add(24 * time.Hour)
	mk := func() (int64, Store, Measure) {
		accountID, s := f.open(t)
		r, err := f.module.CreateMeasure(ctx, CreateMeasureInput{
			AdminID: 7, StoreID: s.StoreID, Kind: MeasureSuspension, StatementToSeller: "風險", Note: "n", ReviewBy: &review, Risk: &RiskBasis{Ground: RiskFraud},
		})
		must(t, err)
		return accountID, s, r.Measure
	}
	accountA, storeA, appealed := mk()
	_, storeB, plain := mk()
	_, err := f.module.SubmitAppeal(ctx, SubmitAppealInput{AccountID: accountA, MeasureID: appealed.ID, ExpectedVersion: appealed.SellerVersion(), Reason: "申訴"})
	must(t, err)
	f.now = f.now.Add(2 * 24 * time.Hour)

	pending, risk, violation := RequestPending, BasisRisk, BasisViolation
	active, ended := StateActive, StateEnded
	ids := func(in AdminMeasureListInput) []uuid.UUID {
		t.Helper()
		in.ReviewDue, in.PageSize = true, 10
		page, err := f.module.AdminMeasures(ctx, in)
		must(t, err)
		var out []uuid.UUID
		for _, m := range page.Items {
			out = append(out, m.ID)
		}
		slices.SortFunc(out, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })
		return out
	}
	both := []uuid.UUID{appealed.ID, plain.ID}
	slices.SortFunc(both, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })
	for name, c := range map[string]struct {
		in   AdminMeasureListInput
		want []uuid.UUID
	}{
		"沒有其他篩選":         {AdminMeasureListInput{}, both},
		"依據 RISK":        {AdminMeasureListInput{Basis: &risk}, both},
		"依據 VIOLATION":   {AdminMeasureListInput{Basis: &violation}, nil},
		"申訴待裁定":          {AdminMeasureListInput{RequestState: &pending}, []uuid.UUID{appealed.ID}},
		"狀態 ACTIVE":      {AdminMeasureListInput{State: &active}, both},
		"狀態 ENDED":       {AdminMeasureListInput{State: &ended}, nil},
		"店家 A":           {AdminMeasureListInput{StoreID: storeA.StoreID}, []uuid.UUID{appealed.ID}},
		"店家 B":           {AdminMeasureListInput{StoreID: storeB.StoreID}, []uuid.UUID{plain.ID}},
		"帳號 A":           {AdminMeasureListInput{AccountID: accountA}, []uuid.UUID{appealed.ID}},
		"店家 B 且申訴待裁定":    {AdminMeasureListInput{StoreID: storeB.StoreID, RequestState: &pending}, nil},
		"依據 RISK 且申訴待裁定": {AdminMeasureListInput{Basis: &risk, RequestState: &pending}, []uuid.UUID{appealed.ID}},
	} {
		if got := ids(c.in); !slices.Equal(got, c.want) {
			t.Errorf("%s：%v，要 %v", name, got, c.want)
		}
	}
}

// 結束沒有固定期間的 RISK 措施：StoreUnsuspended 一次、停權期間結束、身分快照清掉。
func TestEndingAnOpenRiskMeasureEndsTheEffectAndDeletesItsIdentitySnapshot(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	unsuspended := record[StoreUnsuspended](f)
	ended := record[MeasureEnded](f)
	accountID, s := f.open(t)
	f.savedDraft(t, kycDraft(accountID, idA, bankAcct))
	risk := f.riskSuspend(t, s.StoreID)
	if n := f.count(t, "SELECT count(*) FROM seller.measure_identities WHERE measure_id = $1", risk.ID); n == 0 {
		t.Fatal("停權措施要快照身分")
	}
	f.endRisk(t, risk)
	if got := unsuspended.all(); len(got) != 1 || f.openPeriods(t) != 0 || len(ended.all()) != 1 || ended.all()[0].MeasureID != risk.ID {
		t.Fatalf("unsuspended=%+v open=%d ended=%+v", got, f.openPeriods(t), ended.all())
	}
	if n := f.count(t, "SELECT count(*) FROM seller.measure_identities"); n != 0 {
		t.Fatalf("結束後快照還在：%d", n)
	}
	if _, err := f.module.ActingStore(ctx, accountID, CapProductManage); err != nil {
		t.Fatalf("結束後恢復：%v", err)
	}
}

// 兩種依據的措施重疊時是同一段停權期間；一個結束另一個還在就不恢復。
func TestRiskAndViolationMeasuresShareThePeriod(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	suspended := record[StoreSuspended](f)
	unsuspended := record[StoreUnsuspended](f)
	created := record[MeasureCreated](f)
	accountID, s := f.open(t)
	ends := f.now.Add(30 * 24 * time.Hour)
	v := f.violate(t, s.StoreID, SeverityHigh, MeasureSuspension, &ends)
	if !v.EndsAt.Equal(ends) || v.Kind != MeasureSuspension || v.State != StateActive {
		t.Fatalf("v=%+v", v)
	}
	if got := suspended.all(); len(got) != 1 || got[0].AccountID != accountID || got[0].PeriodID <= 0 {
		t.Fatalf("suspended=%+v", got)
	}
	if got := created.all(); len(got) != 1 || got[0].MeasureID != v.ID || got[0].AccountID != accountID || got[0].Basis != BasisViolation || got[0].RiskGround != nil || !got[0].EndsAt.Equal(ends) {
		t.Fatalf("created=%+v", got)
	}
	if _, err := f.module.ActingStore(ctx, accountID, CapProductManage); apperr.KindOf(err) != apperr.KindPermissionDenied {
		t.Fatalf("suspension must deny business capabilities: %v", err)
	}
	if _, err := f.module.ActingStore(ctx, accountID, CapAccountSelfRead); err != nil {
		t.Fatalf("suspension keeps the whitelist: %v", err)
	}
	risk := f.riskSuspend(t, s.StoreID)
	if len(suspended.all()) != 1 || !f.isSuspended(t, s.StoreID) {
		t.Fatalf("risk over violation: events=%+v", suspended.all())
	}
	f.endRisk(t, risk)
	if !f.isSuspended(t, s.StoreID) || len(unsuspended.all()) != 0 {
		t.Fatalf("ending the risk leaves the violation: events=%+v", unsuspended.all())
	}
	_, err := f.module.RevokeMeasure(ctx, RevokeMeasureInput{MeasureID: v.ID, ExpectedVersion: v.Version, AdminID: 9, Reason: RevokeAdminCorrection})
	must(t, err)
	if got := unsuspended.all(); len(got) != 1 || got[0].PeriodID != suspended.all()[0].PeriodID {
		t.Fatalf("one continuous suspension must carry one period id: suspended=%+v unsuspended=%+v", suspended.all(), got)
	}
	detail := f.measureOf(t, v.ID)
	if detail.Status != MeasureStatusRevoked || detail.State != StateRevoked || *detail.RevokedByAdminID != 9 || f.isSuspended(t, s.StoreID) {
		t.Fatalf("revoked measure=%+v", detail)
	}
}

// 到期由排在 ends_at 的 job 重算：到期後恢復、發 StoreUnsuspended；是否生效看傳入的時間；重跑是冪等的。
func TestMeasureExpiryJobEnds(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	unsuspended := record[StoreUnsuspended](f)
	suspended := record[StoreSuspended](f)
	accountID, s := f.open(t)
	ends := f.now.Add(24 * time.Hour)
	v := f.violate(t, s.StoreID, SeverityHigh, MeasureSuspension, &ends)
	jobs := expiryJobs(t, f)
	if len(jobs) != 1 {
		t.Fatalf("expiry jobs=%v", jobs)
	}
	for _, at := range jobs {
		if !at.Equal(ends) {
			t.Fatalf("expiry scheduled at %v, want %v", at, ends)
		}
	}
	f.now = ends
	must(t, f.module.ExpireMeasure(ctx, v.ID, *v.EndsAt))
	if f.isSuspended(t, s.StoreID) {
		t.Fatal("expired measure still suspends")
	}
	if got := unsuspended.all(); len(got) != 1 || got[0].PeriodID != suspended.all()[0].PeriodID {
		t.Fatalf("unsuspended=%+v", got)
	}
	if _, err := f.module.ActingStore(ctx, accountID, CapProductManage); err != nil {
		t.Fatalf("expired: %v", err)
	}
	must(t, f.module.ExpireMeasure(ctx, v.ID, *v.EndsAt))
	if len(unsuspended.all()) != 1 {
		t.Fatal("expiry job is not idempotent")
	}
	must(t, f.module.ExpireMeasure(ctx, uuid.NewV7(), ends))
}

// 縮短結束時間會再排一個結束 job 並重算；已結束的措施不能延長（再限制要另立措施）；沒有改變不產生版本。
func TestShorteningRecomputesAndAnEndedMeasureCannotBeExtended(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	suspended := record[StoreSuspended](f)
	unsuspended := record[StoreUnsuspended](f)
	_, s := f.open(t)
	start := f.now
	ends := start.Add(48 * time.Hour)
	v := f.violate(t, s.StoreID, SeverityHigh, MeasureSuspension, &ends)
	f.jobs.args, f.jobs.opts = nil, nil
	shorter := start.Add(24 * time.Hour)
	updated, err := f.module.UpdateMeasure(ctx, UpdateMeasureInput{MeasureID: v.ID, AdminID: 7, ExpectedVersion: v.Version, ChangeEndsAt: true, EndsAt: &shorter})
	must(t, err)
	if updated.Version != v.Version+1 || !updated.EndsAt.Equal(shorter) || updated.StatementToSeller != v.StatementToSeller {
		t.Fatalf("updated=%+v", updated)
	}
	if jobs := expiryJobs(t, f); len(jobs) != 1 {
		t.Fatalf("new expiry job=%v", jobs)
	}
	f.now = start.Add(25 * time.Hour)
	must(t, f.module.ExpireMeasure(ctx, v.ID, shorter))
	if f.isSuspended(t, s.StoreID) || len(unsuspended.all()) != 1 {
		t.Fatalf("expired: events=%+v", unsuspended.all())
	}
	longer := f.now.Add(5 * 24 * time.Hour)
	if _, err := f.module.UpdateMeasure(ctx, UpdateMeasureInput{MeasureID: v.ID, AdminID: 7, ExpectedVersion: updated.Version, ChangeEndsAt: true, EndsAt: &longer}); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("extending an ended measure must be refused: %v", err)
	}
	if f.isSuspended(t, s.StoreID) || len(suspended.all()) != 1 {
		t.Fatalf("a refused extension must not suspend again: %+v", suspended.all())
	}
	statement := "  改寫的理由  "
	same, err := f.module.UpdateMeasure(ctx, UpdateMeasureInput{MeasureID: v.ID, AdminID: 7, ExpectedVersion: updated.Version, StatementToSeller: &statement})
	if apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("改已結束措施的理由：%v %+v", err, same)
	}
	// 再限制：另立一個引用同一筆認定的新措施。
	again, err := f.module.CreateMeasure(ctx, CreateMeasureInput{
		AdminID: 7, StoreID: s.StoreID, Kind: MeasureSuspension, StatementToSeller: "再限制", Note: "n", EndsAt: &longer,
		Violation: &ViolationBasis{CitedFindingIDs: []uuid.UUID{v.Findings[0].ID}},
	})
	must(t, err)
	if !f.isSuspended(t, s.StoreID) || again.Measure.Findings[0].ID != v.Findings[0].ID || len(suspended.all()) != 2 {
		t.Fatalf("再限制：suspended=%v events=%+v", f.isSuspended(t, s.StoreID), suspended.all())
	}
}

// 對帳掃描有進行中停權期間或此刻有生效中停權措施的帳號（已到期、期間已結束的措施雖然仍是 ENFORCED 也不掃；警告與已撤銷的
// 不掃），並修補兩種不一致：有期間卻沒有生效中的措施（結束 job 沒跑）、有生效中的措施卻沒有期間。
func TestReconcileScansOnlyAccountsWithAnOpenPeriodOrAnEffectiveSuspension(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	unsuspended := record[StoreUnsuspended](f)
	suspended := record[StoreSuspended](f)
	_, lagging := f.open(t)
	_, history := f.open(t)
	_, endedRisk := f.open(t)
	_, lostPeriod := f.open(t)
	_, healthy := f.open(t)
	_, warned := f.open(t)
	_, revoked := f.open(t)
	f.open(t)
	short, long := f.now.Add(time.Hour), f.now.Add(30*24*time.Hour)
	f.violate(t, lagging.StoreID, SeverityHigh, MeasureSuspension, &short)
	h := f.violate(t, history.StoreID, SeverityHigh, MeasureSuspension, &short)
	f.violate(t, healthy.StoreID, SeverityHigh, MeasureSuspension, &long)
	f.violate(t, warned.StoreID, SeverityLow, MeasureWarning, nil)
	f.riskSuspend(t, lostPeriod.StoreID)
	f.endRisk(t, f.riskSuspend(t, endedRisk.StoreID))
	r := f.violate(t, revoked.StoreID, SeverityHigh, MeasureSuspension, &long)
	_, err := f.module.RevokeMeasure(ctx, RevokeMeasureInput{MeasureID: r.ID, ExpectedVersion: r.Version, AdminID: 7, Reason: RevokeOther})
	must(t, err)

	f.now = short.Add(time.Minute)
	must(t, f.module.ExpireMeasure(ctx, h.ID, short)) // history 的結束 job 有跑
	if _, err := f.db.Conn(ctx).Exec(ctx, "DELETE FROM seller.suspension_periods WHERE store_id=$1 AND ended_at IS NULL", lostPeriod.StoreID); err != nil {
		t.Fatal(err) // 模擬寫入的 transaction 之外出了差錯：有生效中的措施、沒有期間
	}
	suspendedBefore, unsuspendedBefore := len(suspended.all()), len(unsuspended.all())

	scanned, err := f.module.ReconcileSuspensions(ctx)
	must(t, err)
	if scanned != 3 {
		t.Fatalf("scanned=%d, want lagging, lostPeriod and healthy only (history and endedRisk have nothing left to repair)", scanned)
	}
	if got := unsuspended.all(); len(got) != unsuspendedBefore+1 || got[len(got)-1].StoreID != lagging.StoreID {
		t.Fatalf("the lagging account must be unsuspended: %+v", got)
	}
	if got := suspended.all(); len(got) != suspendedBefore+1 || got[len(got)-1].StoreID != lostPeriod.StoreID {
		t.Fatalf("the account without a period must get one: %+v", got)
	}
	if n := f.openPeriods(t); n != 2 {
		t.Fatalf("open periods=%d, want lostPeriod and healthy", n)
	}
	scanned, err = f.module.ReconcileSuspensions(ctx)
	must(t, err)
	if scanned != 2 || len(suspended.all()) != suspendedBefore+1 || len(unsuspended.all()) != unsuspendedBefore+1 {
		t.Fatalf("second run scanned=%d must be idempotent", scanned)
	}
}

// 措施的效果以帳號為單位，到期之後可以開店（讀取時推導，不必等結束 job）；舊 stint 上留著的停權期間由之後第一次重算結束，不會
// 跟到新的 stint：新的 stint 再被停權時，舊的期間以 StoreUnsuspended 結束、新的 stint 開始自己的一段。
func TestMeasureFollowsTheAccount(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	unsuspended := record[StoreUnsuspended](f)
	suspended := record[StoreSuspended](f)
	accountID, s := f.open(t)
	ends := f.now.Add(24 * time.Hour)
	f.violate(t, s.StoreID, SeverityHigh, MeasureSuspension, &ends)
	_, err := f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	if _, err := f.module.Apply(ctx, validApply(accountID)); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("apply while a measure is active: %v", err)
	}
	f.now = ends
	reopened, err := f.module.Apply(ctx, validApply(accountID))
	must(t, err)
	if reopened.Suspended() || f.isSuspended(t, s.StoreID) {
		t.Fatalf("reopened=%+v old=%v", reopened, f.isSuspended(t, s.StoreID))
	}
	if f.openPeriods(t) != 1 || len(unsuspended.all()) != 0 {
		t.Fatal("the stale period belongs to the old stint until the next recompute")
	}
	f.riskSuspend(t, reopened.StoreID)
	if !f.isSuspended(t, reopened.StoreID) {
		t.Fatal("suspending the new stint must mark it")
	}
	gotUn, gotSus := unsuspended.all(), suspended.all()
	if len(gotUn) != 1 || gotUn[0].StoreID != s.StoreID || len(gotSus) != 2 || gotSus[1].StoreID != reopened.StoreID ||
		gotSus[1].PeriodID == gotSus[0].PeriodID || f.openPeriods(t) != 1 {
		t.Fatalf("the old period ends and the new stint gets its own: suspended=%+v unsuspended=%+v", gotSus, gotUn)
	}
}

// 帳號所有 stint 都關閉時，停權期間留在那個 stint，到期之後結束；已結束的措施不能延長，所以帳號不會因此再次停權。
func TestExpiryOnAnAccountWithoutAStintEndsThePeriodOnTheClosedStint(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	suspended := record[StoreSuspended](f)
	unsuspended := record[StoreUnsuspended](f)
	accountID, s := f.open(t)
	start := f.now
	ends := start.Add(24 * time.Hour)
	v := f.violate(t, s.StoreID, SeverityHigh, MeasureSuspension, &ends)
	_, err := f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	if f.openPeriods(t) != 1 {
		t.Fatal("the period of a suspended seller stays on the closed stint")
	}
	f.now = start.Add(48 * time.Hour)
	must(t, f.module.ExpireMeasure(ctx, v.ID, ends))
	if f.openPeriods(t) != 0 || len(unsuspended.all()) != 1 {
		t.Fatalf("expiry must end the period on the closed stint: events=%+v", unsuspended.all())
	}
	longer := start.Add(5 * 24 * time.Hour)
	if _, err := f.module.UpdateMeasure(ctx, UpdateMeasureInput{MeasureID: v.ID, AdminID: 7, ExpectedVersion: v.Version, ChangeEndsAt: true, EndsAt: &longer}); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("an ended measure cannot be extended: %v", err)
	}
	if _, err := f.module.Apply(ctx, validApply(accountID)); err != nil {
		t.Fatalf("the ended measure no longer blocks opening: %v", err)
	}
	if f.openPeriods(t) != 0 || len(suspended.all()) != 1 {
		t.Fatalf("no extension, no new period: open=%d events=%+v", f.openPeriods(t), suspended.all())
	}
}

// 停權狀態不存：措施一到期，不必任何 job，讀店家、能力檢查、寫入與開店都已經看到恢復（ADR 2026-10-18-seller-governance-consolidated「考慮過的方案」的「停權狀態」）。
// 到期的那一刻（now = ends_at）已經不生效。
func TestSuspensionIsDerivedAtReadTimeWithoutAnyJob(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	if f.count(t, "SELECT count(*) FROM information_schema.columns WHERE table_schema='seller' AND table_name='stores' AND column_name='suspension'") != 0 {
		t.Fatal("stores must not store the suspension")
	}
	accountID, s := f.open(t)
	ends := f.now.Add(24 * time.Hour)
	f.violate(t, s.StoreID, SeverityHigh, MeasureSuspension, &ends)
	f.now = ends.Add(-time.Second)
	if !f.isSuspended(t, s.StoreID) {
		t.Fatal("a second before ends_at the account is still suspended")
	}
	if _, err := f.module.ActingStore(ctx, accountID, CapProductManage); apperr.KindOf(err) != apperr.KindPermissionDenied {
		t.Fatalf("suspended: %v", err)
	}
	f.now = ends
	if f.isSuspended(t, s.StoreID) {
		t.Fatal("expired measure still suspends")
	}
	if _, err := f.module.ActingStore(ctx, accountID, CapProductManage); err != nil {
		t.Fatalf("expired measure still denies: %v", err)
	}
	current, err := f.module.CurrentStore(ctx, accountID)
	must(t, err)
	if current.Suspended() || !current.Can(CapStorefrontWrite) {
		t.Fatalf("current=%+v", current)
	}
	if _, err := f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: s.Version, Prize: ptr("金牌")}); err != nil {
		t.Fatalf("write after expiry: %v", err)
	}
	if f.openPeriods(t) != 1 {
		t.Fatal("the period is the job's business: nothing has recomputed it yet")
	}
}

// worker 的時鐘比寫入措施的時鐘慢：job 在 ends_at 被取出，但 worker 的 now 還在 ends_at 之前。以 max(ends_at, now) 判斷，
// 這一輪仍然結束停權期間（CR-3）；用 now 判斷會把剛到期的措施當成還生效，什麼都不做。
func TestExpiryJobWithASlowWorkerClockStillEnds(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	suspended := record[StoreSuspended](f)
	unsuspended := record[StoreUnsuspended](f)
	_, s := f.open(t)
	ends := f.now.Add(24 * time.Hour)
	v := f.violate(t, s.StoreID, SeverityHigh, MeasureSuspension, &ends)
	f.now = ends.Add(-3 * time.Second)
	must(t, f.module.ExpireMeasure(ctx, v.ID, ends))
	got := unsuspended.all()
	if len(got) != 1 || got[0].PeriodID != suspended.all()[0].PeriodID || f.openPeriods(t) != 0 {
		t.Fatalf("a slow worker clock must not make the job a no-op: unsuspended=%+v open=%d", got, f.openPeriods(t))
	}
}

// 措施後來延後：舊的結束 job 照樣在舊的 ends_at 執行，判斷時間是舊的 ends_at，新的結束時間更晚，措施繼續生效，期間不會被提早結束。
func TestStaleExpiryJobAfterExtendingTheMeasureKeepsItEffective(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	unsuspended := record[StoreUnsuspended](f)
	_, s := f.open(t)
	start := f.now
	ends := start.Add(24 * time.Hour)
	v := f.violate(t, s.StoreID, SeverityHigh, MeasureSuspension, &ends)
	longer := start.Add(5 * 24 * time.Hour)
	_, err := f.module.UpdateMeasure(ctx, UpdateMeasureInput{MeasureID: v.ID, AdminID: 7, ExpectedVersion: v.Version, ChangeEndsAt: true, EndsAt: &longer})
	must(t, err)
	f.now = ends
	must(t, f.module.ExpireMeasure(ctx, v.ID, ends))
	if !f.isSuspended(t, s.StoreID) || len(unsuspended.all()) != 0 || f.openPeriods(t) != 1 {
		t.Fatalf("the old job must not end a measure that was extended: events=%+v", unsuspended.all())
	}
}

// 接受申覆在同一個 transaction 撤銷措施並解除效果：事件失敗時全部回滾。
func TestAppealAcceptEndsTheEffectInTheSameTransaction(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	submitted := record[MeasureRequestSubmitted](f)
	unsuspended := record[StoreUnsuspended](f)
	accountID, s, m := f.appealable(t)
	appealed := f.appeal(t, accountID, m, " 已改善 ", "a1")
	if appealed.Appeal() == nil || appealed.Appeal().State != RequestPending || appealed.Appeal().Reason != "已改善" || !appealed.CanDecideAppeal() ||
		f.media.finalized[len(f.media.finalized)-1] != media.RefTypeGovernanceEvidence {
		t.Fatalf("appealed=%+v", appealed.Appeal())
	}
	admin := f.measureOf(t, m.ID)
	if len(admin.Appeal().Evidence) != 1 {
		t.Fatalf("佐證=%+v", admin.Appeal().Evidence)
	}
	if got := submitted.all(); len(got) != 1 || got[0].AccountID != accountID || got[0].RequestID != admin.Appeal().ID || got[0].Kind != RequestKindAppeal {
		t.Fatalf("submitted=%+v", got)
	}
	failing := true
	events.Subscribe(f.bus, func(context.Context, MeasureRequestDecided) error {
		if failing {
			return errors.New("boom")
		}
		return nil
	})
	if _, err := f.module.DecideAppeal(ctx, DecideAppealInput{MeasureID: m.ID, ExpectedVersion: admin.Version, AdminID: 9, Outcome: RequestAccepted, Reason: "接受"}); err == nil {
		t.Fatal("decide must fail with its event")
	}
	rolledBack := f.measureOf(t, m.ID)
	if rolledBack.Status != MeasureStatusEnforced || rolledBack.Appeal().State != RequestPending || !f.isSuspended(t, s.StoreID) || rolledBack.Version != admin.Version {
		t.Fatalf("partial decide committed: %+v", rolledBack)
	}
	unsuspended.items = nil // 回滾的 transaction 裡發過的事件，handler 已經看過；只看成功的那一次
	failing = false
	accepted, err := f.module.DecideAppeal(ctx, DecideAppealInput{MeasureID: m.ID, ExpectedVersion: admin.Version, AdminID: 9, Outcome: RequestAccepted, Reason: "接受"})
	must(t, err)
	if accepted.Status != MeasureStatusRevoked || *accepted.RevokedReason != RevokeAppealAccepted || accepted.Appeal().State != RequestAccepted ||
		*accepted.Appeal().DecisionReason != "接受" || f.isSuspended(t, s.StoreID) || len(unsuspended.all()) != 1 {
		t.Fatalf("accepted=%+v", accepted)
	}
	f.assertMeasureInvariants(t)
}

func TestAppealRules(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	decided := record[MeasureRequestDecided](f)
	accountID, s, m := f.appealable(t)
	stranger, _ := f.open(t)
	submit := func(account int64, version int64, reason string) error {
		_, err := f.module.SubmitAppeal(ctx, SubmitAppealInput{AccountID: account, MeasureID: m.ID, ExpectedVersion: version, Reason: reason})
		return err
	}
	if err := submit(stranger, m.SellerVersion(), "x"); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("appeal someone else's measure: %v", err)
	}
	if err := submit(accountID, m.SellerVersion(), " "); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("blank reason: %v", err)
	}
	if err := submit(accountID, m.SellerVersion(), strings.Repeat("字", 2001)); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("too long: %v", err)
	}
	appealed := f.appeal(t, accountID, m, "誤判")
	if err := submit(accountID, appealed.SellerVersion(), "再一次"); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("second appeal: %v", err)
	}
	admin := f.measureOf(t, m.ID)
	if _, err := f.module.RevokeMeasure(ctx, RevokeMeasureInput{MeasureID: m.ID, ExpectedVersion: admin.Version, AdminID: 7, Reason: RevokeOther}); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("revoke while the appeal is pending: %v", err)
	}
	decide := func(outcome RequestState, reason string, revokeFindings bool) error {
		_, err := f.module.DecideAppeal(ctx, DecideAppealInput{MeasureID: m.ID, ExpectedVersion: f.measureOf(t, m.ID).Version, AdminID: 7, Outcome: outcome, Reason: reason, RevokeFindings: revokeFindings})
		return err
	}
	for name, err := range map[string]error{
		"裁定只能是接受或駁回":      decide(RequestPending, "x", false),
		"駁回不附理由":          decide(RequestRejected, "", false),
		"接受不附理由（條款 18.4）": decide(RequestAccepted, " ", false),
		"駁回不能一併撤銷認定":      decide(RequestRejected, "證據不足", true),
	} {
		if apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：%v", name, err)
		}
	}
	if err := decide(RequestRejected, "證據不足", false); err != nil {
		t.Fatal(err)
	}
	rejected := f.measureOf(t, m.ID)
	if rejected.Status != MeasureStatusEnforced || rejected.Appeal().State != RequestRejected || *rejected.Appeal().DecisionReason != "證據不足" || !f.isSuspended(t, s.StoreID) {
		t.Fatalf("rejected=%+v", rejected)
	}
	if err := decide(RequestAccepted, "x", false); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("decide twice: %v", err)
	}
	if got := decided.all(); len(got) != 1 || got[0].Outcome != RequestRejected {
		t.Fatalf("decided=%+v", got)
	}
	mine, err := f.module.AccountMeasure(ctx, accountID, m.ID)
	must(t, err)
	if mine.ID != m.ID || mine.CanAppeal() {
		t.Fatalf("mine=%+v", mine)
	}
	if _, err := f.module.AccountMeasure(ctx, stranger, m.ID); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("read someone else's measure: %v", err)
	}
	f.assertMeasureInvariants(t)
}

// 賣家看到帳號所有 stint 上的措施，新到舊，可依狀態篩選；admin 的措施列表以 keyset 翻頁並可依店家、帳號、狀態、依據、申覆篩選，
// 待裁定的申覆依提出時間舊到新。
func TestMeasureListsAcrossStintsAndFilters(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	first := f.violate(t, s.StoreID, SeverityLow, MeasureWarning, nil)
	_, err := f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	reopened, err := f.module.Apply(ctx, validApply(accountID))
	must(t, err)
	f.now = f.now.Add(time.Minute)
	second := f.violate(t, reopened.StoreID, SeverityLow, MeasureWarning, nil)
	f.now = f.now.Add(time.Minute)
	ends := f.now.Add(24 * time.Hour)
	suspension := f.violate(t, reopened.StoreID, SeverityHigh, MeasureSuspension, &ends)
	_, other := f.open(t)
	f.violate(t, other.StoreID, SeverityLow, MeasureWarning, nil)

	page, err := f.module.AccountMeasures(ctx, AccountMeasureListInput{AccountID: accountID, PageSize: 2})
	must(t, err)
	if len(page.Items) != 2 || page.Items[0].ID != suspension.ID || page.Items[1].ID != second.ID || page.Next == nil {
		t.Fatalf("page=%+v", page)
	}
	page, err = f.module.AccountMeasures(ctx, AccountMeasureListInput{AccountID: accountID, PageSize: 2, After: page.Next})
	must(t, err)
	if len(page.Items) != 1 || page.Items[0].ID != first.ID || page.Next != nil {
		t.Fatalf("second page=%+v", page)
	}
	stateIDs := func(state MeasureState) []uuid.UUID {
		t.Helper()
		got, err := f.module.AccountMeasures(ctx, AccountMeasureListInput{AccountID: accountID, State: &state})
		must(t, err)
		var ids []uuid.UUID
		for _, m := range got.Items {
			ids = append(ids, m.ID)
			if m.State != state {
				t.Errorf("state=%s 的篩選回了 %s", state, m.State)
			}
		}
		return ids
	}
	if got := stateIDs(StateActive); !slices.Equal(got, []uuid.UUID{suspension.ID}) {
		t.Fatalf("active=%v", got)
	}
	if got := stateIDs(StateIssued); !slices.Equal(got, []uuid.UUID{second.ID, first.ID}) {
		t.Fatalf("issued=%v", got)
	}
	f.now = ends.Add(time.Minute)
	if got := stateIDs(StateEnded); !slices.Equal(got, []uuid.UUID{suspension.ID}) || len(stateIDs(StateActive)) != 0 {
		t.Fatalf("ended=%v", got)
	}
	bad := MeasureState("WHATEVER")
	if _, err := f.module.AccountMeasures(ctx, AccountMeasureListInput{AccountID: accountID, State: &bad}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("bad state: %v", err)
	}

	all, err := f.module.AdminMeasures(ctx, AdminMeasureListInput{PageSize: 100})
	must(t, err)
	byStore, err := f.module.AdminMeasures(ctx, AdminMeasureListInput{StoreID: s.StoreID})
	must(t, err)
	byAccount, err := f.module.AdminMeasures(ctx, AdminMeasureListInput{AccountID: accountID})
	must(t, err)
	risk := BasisRisk
	byBasis, err := f.module.AdminMeasures(ctx, AdminMeasureListInput{Basis: &risk})
	must(t, err)
	pending := RequestPending
	appeals, err := f.module.AdminMeasures(ctx, AdminMeasureListInput{RequestState: &pending})
	must(t, err)
	if len(all.Items) != 4 || len(byStore.Items) != 1 || byStore.Items[0].ID != first.ID || len(byAccount.Items) != 3 || len(byBasis.Items) != 0 || len(appeals.Items) != 0 {
		t.Fatalf("all=%d byStore=%d byAccount=%d byBasis=%d appeals=%d", len(all.Items), len(byStore.Items), len(byAccount.Items), len(byBasis.Items), len(appeals.Items))
	}

	// 待裁定的申覆收件匣：依提出時間舊到新。
	f.now = f.now.Add(time.Hour)
	f.appeal(t, accountID, f.measureOf(t, suspension.ID), "晚的申訴")
	f.now = f.now.Add(-time.Minute)
	f.appeal(t, accountID, f.measureOf(t, second.ID), "早的申訴")
	queue, err := f.module.AdminMeasures(ctx, AdminMeasureListInput{RequestState: &pending, PageSize: 1})
	must(t, err)
	if len(queue.Items) != 1 || queue.Items[0].ID != second.ID || queue.Next == nil {
		t.Fatalf("queue=%+v", queue)
	}
	queue, err = f.module.AdminMeasures(ctx, AdminMeasureListInput{RequestState: &pending, PageSize: 1, After: queue.Next})
	must(t, err)
	if len(queue.Items) != 1 || queue.Items[0].ID != suspension.ID || queue.Next != nil {
		t.Fatalf("queue page 2=%+v", queue)
	}
	if _, err := f.module.AdminMeasures(ctx, AdminMeasureListInput{StoreID: -1}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("negative id: %v", err)
	}
}

// admin 的單筆與列表都帶店名（批次讀取，不依筆數多查），並帶各動作不能操作的原因。
func TestAdminMeasuresCarryTheStoreNameAndActionReasons(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, a := f.open(t)
	accountB, b := f.open(t)
	if a.StoreName == "" || b.StoreName == "" {
		t.Fatalf("店名：%q %q", a.StoreName, b.StoreName)
	}
	warning := f.violate(t, a.StoreID, SeverityLow, MeasureWarning, nil)
	f.now = f.now.Add(time.Minute)
	other := f.violate(t, b.StoreID, SeverityLow, MeasureWarning, nil)

	names := map[uuid.UUID]string{warning.ID: a.StoreName, other.ID: b.StoreName}
	page, err := f.module.AdminMeasures(ctx, AdminMeasureListInput{PageSize: 100})
	must(t, err)
	if len(page.Items) != 2 {
		t.Fatalf("items=%d", len(page.Items))
	}
	for _, m := range page.Items {
		if m.StoreName != names[m.ID] {
			t.Errorf("列表 %s：店名=%q want=%q", m.ID, m.StoreName, names[m.ID])
		}
	}
	single := f.measureOf(t, warning.ID)
	if single.StoreName != a.StoreName {
		t.Fatalf("單筆店名=%q", single.StoreName)
	}
	if !slices.Equal(single.ExtendUnavailableReasons(), []MeasureActionUnavailableReason{"KIND_HAS_NO_END_TIME", "NOT_ACTIVE"}) ||
		len(single.UpdateStatementUnavailableReasons()) != 0 || len(single.RevokeUnavailableReasons()) != 0 {
		t.Fatalf("警告的原因：extend=%v update=%v revoke=%v", single.ExtendUnavailableReasons(), single.UpdateStatementUnavailableReasons(), single.RevokeUnavailableReasons())
	}

	// 寫入的回應也帶店名。
	f.now = f.now.Add(time.Minute)
	ends := f.now.Add(48 * time.Hour)
	suspension := f.violate(t, a.StoreID, SeverityHigh, MeasureSuspension, &ends)
	shorter := f.now.Add(24 * time.Hour)
	updated, err := f.module.UpdateMeasure(ctx, UpdateMeasureInput{MeasureID: suspension.ID, AdminID: 7, ExpectedVersion: suspension.Version, ChangeEndsAt: true, EndsAt: &shorter})
	must(t, err)
	if updated.StoreName != a.StoreName {
		t.Fatalf("UpdateMeasure 回應店名=%q want=%q", updated.StoreName, a.StoreName)
	}
	f.now = f.now.Add(time.Minute)
	revoked, err := f.module.RevokeMeasure(ctx, RevokeMeasureInput{MeasureID: suspension.ID, AdminID: 7, ExpectedVersion: updated.Version, Reason: RevokeAdminCorrection})
	must(t, err)
	if revoked.StoreName != a.StoreName {
		t.Fatalf("RevokeMeasure 回應店名=%q want=%q", revoked.StoreName, a.StoreName)
	}

	// 個資清理之後店名是 NULL，措施的店名是空字串（列表與單筆）。
	if _, err := f.module.CloseCurrent(ctx, accountB); err != nil {
		t.Fatal(err)
	}
	f.now = f.now.Add(StoreRetention + time.Second)
	if n, err := f.module.PurgePII(ctx); err != nil || n != 1 {
		t.Fatalf("清理：n=%d err=%v", n, err)
	}
	if got := f.measureOf(t, other.ID).StoreName; got != "" {
		t.Fatalf("清除個資後單筆店名=%q", got)
	}
	page, err = f.module.AdminMeasures(ctx, AdminMeasureListInput{StoreID: b.StoreID, PageSize: 10})
	must(t, err)
	if len(page.Items) != 1 || page.Items[0].StoreName != "" {
		t.Fatalf("清除個資後列表：%+v", page.Items)
	}
	if got := f.measureOf(t, warning.ID).StoreName; got != a.StoreName {
		t.Fatalf("其他店不受影響：%q", got)
	}
}

// 賣家的措施列表帶期間與依據：停權有開始與結束，警告沒有結束時間；時間到了不改變列表看到的值也不改版本，調整結束時間與撤銷才改。
func TestAccountMeasuresCarryThePeriodAndState(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	start := f.now
	ends := start.Add(24 * time.Hour)
	suspension := f.violate(t, s.StoreID, SeverityHigh, MeasureSuspension, &ends)
	f.now = start.Add(time.Minute)
	warning := f.violate(t, s.StoreID, SeverityLow, MeasureWarning, nil)
	list := func() map[uuid.UUID]Measure {
		t.Helper()
		page, err := f.module.AccountMeasures(ctx, AccountMeasureListInput{AccountID: accountID, PageSize: 10})
		must(t, err)
		out := make(map[uuid.UUID]Measure, len(page.Items))
		for _, m := range page.Items {
			out[m.ID] = m
		}
		return out
	}
	got := list()
	if m := got[suspension.ID]; m.Kind != MeasureSuspension || !m.CreatedAt.Equal(start) || m.EndsAt == nil || !m.EndsAt.Equal(ends) || m.State != StateActive || len(m.Findings) != 1 || m.Findings[0].Category != policy.MisleadingInfo {
		t.Fatalf("suspension=%+v", m)
	}
	if m := got[warning.ID]; m.EndsAt != nil || m.State != StateIssued || m.Kind != MeasureWarning {
		t.Fatalf("a warning has no end: %+v", m)
	}
	f.now = ends.Add(time.Hour)
	expired := list()[suspension.ID]
	if expired.SellerVersion() != suspension.SellerVersion() || expired.Version != suspension.Version || expired.State != StateEnded {
		t.Fatalf("expiry is derived at read time and must not change either version: %+v", expired)
	}
	f.now = f.now.Add(time.Minute)
	active := f.violate(t, s.StoreID, SeverityHigh, MeasureSuspension, ptr(f.now.Add(48*time.Hour)))
	shorter := f.now.Add(24 * time.Hour)
	updated, err := f.module.UpdateMeasure(ctx, UpdateMeasureInput{MeasureID: active.ID, AdminID: 7, ExpectedVersion: active.Version, ChangeEndsAt: true, EndsAt: &shorter})
	must(t, err)
	if now := list()[active.ID]; now.SellerVersion() != active.SellerVersion()+1 || !now.EndsAt.Equal(shorter) || updated.Version != now.Version {
		t.Fatalf("changing the end time is a seller-visible change: %+v", now)
	}
	f.now = f.now.Add(time.Minute)
	revoked, err := f.module.RevokeMeasure(ctx, RevokeMeasureInput{MeasureID: active.ID, AdminID: 7, ExpectedVersion: updated.Version, Reason: RevokeAdminCorrection})
	must(t, err)
	got2 := list()[active.ID]
	if got2.Version != revoked.Version || got2.State != StateRevoked || got2.RevokedAt == nil || !got2.RevokedAt.Equal(f.now) || *got2.RevokedReason != RevokeAdminCorrection {
		t.Fatalf("revoking: %+v", got2)
	}
}

// 通報與案件的資料庫保證：檢舉人剛好一種、已採取措施一定在案件裡、結果與時間成對、通報的店家與平台判定類別一定等於案件的（複合外鍵）、
// 移動紀錄的通報與兩個案件是同一家店、措施的來源案件是同一家店的、註記剛好掛在案件或措施的其中一個、改嚴重度的改前改後只在案件的註記、
// 同一家店同一個類別同時最多一個進行中的案件、佐證的位置在一筆通報全部佐證的張數之內。
// 每個案例只違反一條約束，而且指名是哪一條（wantViolates）：只看「有錯誤」會把參數數量不符之類不是資料庫擋下的錯誤也當成通過，
// 也會讓同時違反兩條約束的案例遮住其中一條（拿掉它測試仍然通過）。SQL 裡的 $1 是 s 的 store_id，直接代入文字（不用參數）。
func TestReportSchemaConstraints(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	_, other := f.open(t)
	f.anonymousReport(t, s.StoreID, "0912345678")
	c, members := f.newCase(t, s.StoreID, policy.Fraud, SeverityLow, 2)
	foreign, _ := f.newCase(t, other.StoreID, policy.Fraud, SeverityLow, 1)
	foreignSecond, _ := f.newCase(t, other.StoreID, policy.Counterfeit, SeverityLow, 1) // 別家店的第二個案件：移動紀錄的來源與目標要不同
	m := f.violate(t, s.StoreID, SeverityLow, MeasureWarning, nil)
	// 同一位買家再送一次：members[0] 有一則補充（下面的佐證用它）。
	if r, err := f.module.SubmitReport(ctx, SubmitReportInput{StoreID: s.StoreID, ReporterBuyerID: *members[0].ReporterBuyerID, Category: policy.Fraud, Description: "補充"}); err != nil || !r.Supplemented {
		t.Fatalf("supplement=%+v err=%v", r, err)
	}
	insertReport := "INSERT INTO seller.reports (id, store_id, category, description, created_at, updated_at, "
	insertMove := "INSERT INTO seller.report_case_moves (id, report_id, store_id, from_case_id, to_case_id, admin_id, moved_at) VALUES (uuidv7(), '"
	storeID, otherStoreID := fmt.Sprint(s.StoreID), fmt.Sprint(other.StoreID)
	report0, report1, caseID, foreignID, foreignSecondID, measureID := members[0].ID.String(), members[1].ID.String(), c.ID.String(), foreign.ID.String(), foreignSecond.ID.String(), m.ID.String()
	for _, v := range []struct{ name, constraint, sql string }{
		{"買家又留電話", "reports_reporter_check", insertReport + "reporter_buyer_id, reporter_phone) VALUES (uuidv7(), $1, 'FRAUD', 'x', now(), now(), 5, '0912345678')"},
		{"只有姓名", "reports_reporter_check", insertReport + "reporter_name) VALUES (uuidv7(), $1, 'FRAUD', 'x', now(), now(), '王')"},
		{"email 大寫", "reports_reporter_email_check", insertReport + "reporter_email) VALUES (uuidv7(), $1, 'FRAUD', 'x', now(), now(), 'A@b.c')"},
		{"已採取措施而沒有案件", "reports_action_taken_check", insertReport + "reporter_buyer_id, outcome, resolved_at, resolved_by_admin_id) VALUES (uuidv7(), $1, 'FRAUD', 'x', now(), now(), 5, 'ACTION_TAKEN', now(), 1)"},
		{"結果不認得", "reports_outcome_check", insertReport + "reporter_buyer_id, outcome, resolved_at, resolved_by_admin_id) VALUES (uuidv7(), $1, 'FRAUD', 'x', now(), now(), 5, 'VERIFIED', now(), 1)"},
		{"有結果沒有時間", "reports_resolved_check", insertReport + "reporter_buyer_id, outcome) VALUES (uuidv7(), $1, 'FRAUD', 'x', now(), now(), 5, 'NO_VIOLATION')"},
		{"有時間沒有結果", "reports_resolved_check", insertReport + "reporter_buyer_id, resolved_at, resolved_by_admin_id) VALUES (uuidv7(), $1, 'FRAUD', 'x', now(), now(), 5, now(), 1)"},
		{"說明只有空白", "reports_resolved_check", insertReport + "reporter_buyer_id, outcome, resolved_at, resolved_by_admin_id, resolution_note) VALUES (uuidv7(), $1, 'FRAUD', 'x', now(), now(), 5, 'ABUSIVE', now(), 1, '')"},
		{"補充計數是負的", "reports_counts_check", insertReport + "reporter_buyer_id, discarded_submissions) VALUES (uuidv7(), $1, 'FRAUD', 'x', now(), now(), 5, -1)"},
		{"版本是 0", "reports_counts_check", insertReport + "reporter_buyer_id, version) VALUES (uuidv7(), $1, 'FRAUD', 'x', now(), now(), 5, 0)"},
		{"類別是 PLATFORM_ABUSE（檢舉人不能選）", "reports_category_check", insertReport + "reporter_buyer_id) VALUES (uuidv7(), $1, 'PLATFORM_ABUSE', 'x', now(), now(), 5)"},
		{"掛了案件沒有平台判定類別", "reports_triage_check", "UPDATE seller.reports SET case_id = (SELECT id FROM seller.cases WHERE store_id = $1 LIMIT 1) WHERE id = (SELECT id FROM seller.reports WHERE case_id IS NULL AND store_id = $1 LIMIT 1)"},
		{"通報掛到別家店的案件", "reports_case_fkey", "UPDATE seller.reports SET case_id = '" + foreignID + "', assessed_category = 'FRAUD', triaged_at = now(), triaged_by_admin_id = 1 WHERE id = '" + report0 + "'"},
		{"平台判定類別和案件的不同", "reports_case_fkey", "UPDATE seller.reports SET assessed_category = 'COUNTERFEIT' WHERE id = '" + report0 + "'"},
		{"分案時間早於建立", "reports_triage_check", "UPDATE seller.reports SET triaged_at = created_at - interval '1 day' WHERE id = '" + report0 + "'"},
		{"佐證的位置是負的", "report_evidence_position_check", "INSERT INTO seller.report_evidence (report_id, submission_seq, position, asset_id, object_key) VALUES ('" + report0 + "', 0, -1, 'a', 'k')"},
		{"一次送出的佐證超過 5 張（位置 5）", "report_evidence_position_check", fmt.Sprintf("INSERT INTO seller.report_evidence (report_id, submission_seq, position, asset_id, object_key) VALUES ('%s', 0, %d, 'a', 'k')", report0, model.MaxEvidence)},
		{"補充的佐證沒有指向補充", "report_evidence_supplement_check", "INSERT INTO seller.report_evidence (report_id, submission_seq, position, asset_id, object_key) VALUES ('" + report0 + "', 1, 0, 'a', 'k')"},
		{"補充的佐證指向的補充不是這一次送出", "report_evidence_supplement_fkey", "INSERT INTO seller.report_evidence (report_id, submission_seq, position, supplement_id, asset_id, object_key) SELECT '" + report0 + "', 2, 0, id, 'a', 'k' FROM seller.report_supplements WHERE report_id = '" + report0 + "'"},
		{"佐證的補充屬於別的通報", "report_evidence_supplement_fkey", "INSERT INTO seller.report_evidence (report_id, submission_seq, position, supplement_id, asset_id, object_key) SELECT '" + report1 + "', 1, 0, id, 'a', 'k' FROM (SELECT id FROM seller.report_supplements LIMIT 1) x"},
		{"補充的比對方式不認得", "report_supplements_matched_by_check", "INSERT INTO seller.report_supplements (id, report_id, seq, description, matched_by, created_at) VALUES (uuidv7(), '" + report0 + "', 2, 'x', 'NAME', now())"},
		{"補充超過 10 則", "report_supplements_seq_check", fmt.Sprintf("INSERT INTO seller.report_supplements (id, report_id, seq, description, created_at) VALUES (uuidv7(), '%s', %d, 'x', now())", report0, model.MaxSupplements+1)},
		{"同一筆通報的補充序號重複", "report_supplements_report_seq_key", "INSERT INTO seller.report_supplements (id, report_id, seq, description, created_at) VALUES (uuidv7(), '" + report0 + "', 1, 'x', now())"},
		{"補充是空的", "report_supplements_description_check", "INSERT INTO seller.report_supplements (id, report_id, seq, description, created_at) VALUES (uuidv7(), '" + report0 + "', 2, '', now())"},
		{"移動紀錄的來源案件是別家店的", "report_case_moves_from_fkey", insertMove + report0 + "', $1, '" + foreignID + "', '" + caseID + "', 1, now())"},
		{"移動紀錄的目標案件是別家店的", "report_case_moves_to_fkey", insertMove + report0 + "', $1, '" + caseID + "', '" + foreignID + "', 1, now())"},
		{"移動紀錄的通報是別家店的", "report_case_moves_report_fkey", insertMove + report0 + "', " + otherStoreID + ", '" + foreignID + "', '" + foreignSecondID + "', 1, now())"},
		{"移動紀錄的來源和目標相同", "report_case_moves_distinct_check", insertMove + report0 + "', $1, '" + caseID + "', '" + caseID + "', 1, now())"},
		{"措施的來源案件是別家店的", "measures_source_case_fkey", "UPDATE seller.measures SET source_case_id = '" + foreignID + "' WHERE id = '" + measureID + "'"},
		{"註記兩個都不掛", "governance_notes_owner_check", "INSERT INTO seller.governance_notes (note_id, admin_id, body, created_at) VALUES (uuidv7(), 1, 'x', now())"},
		{"註記兩個都掛", "governance_notes_owner_check", "INSERT INTO seller.governance_notes (note_id, measure_id, case_id, admin_id, body, created_at) VALUES (uuidv7(), '" + measureID + "', '" + caseID + "', 1, 'x', now())"},
		{"措施的註記帶嚴重度", "governance_notes_severity_check", "INSERT INTO seller.governance_notes (note_id, measure_id, admin_id, body, severity_from, severity_to, created_at) VALUES (uuidv7(), '" + measureID + "', 1, 'x', 'LOW', 'HIGH', now())"},
		{"案件的註記改前改後相同", "governance_notes_severity_check", "INSERT INTO seller.governance_notes (note_id, case_id, admin_id, body, severity_from, severity_to, created_at) VALUES (uuidv7(), '" + caseID + "', 1, 'x', 'LOW', 'LOW', now())"},
		{"案件的註記只有改前", "governance_notes_severity_check", "INSERT INTO seller.governance_notes (note_id, case_id, admin_id, body, severity_from, created_at) VALUES (uuidv7(), '" + caseID + "', 1, 'x', 'LOW', now())"},
		{"同一家店同一個類別第二個進行中的案件", "cases_one_open", "INSERT INTO seller.cases (id, store_id, category, severity, opened_at, opened_by_admin_id, updated_at) VALUES (uuidv7(), $1, 'FRAUD', 'LOW', now(), 1, now())"},
		{"案件的類別是 OTHER", "cases_category_check", "INSERT INTO seller.cases (id, store_id, category, severity, opened_at, opened_by_admin_id, updated_at) VALUES (uuidv7(), $1, 'OTHER', 'LOW', now(), 1, now())"},
		{"案件結案沒有原因", "cases_close_check", "UPDATE seller.cases SET closed_at = now(), closed_by_admin_id = 1 WHERE id = '" + caseID + "'"},
		{"以沒有通報關閉卻帶決定", "cases_close_check", "UPDATE seller.cases SET closed_at = now(), closed_by_admin_id = 1, close_reason = 'NO_REPORTS', decision = 'NO_ACTION' WHERE id = '" + caseID + "'"},
		{"決定了卻沒有決定的值", "cases_close_check", "UPDATE seller.cases SET closed_at = now(), closed_by_admin_id = 1, close_reason = 'DECIDED' WHERE id = '" + caseID + "'"},
		{"結案時間早於開案", "cases_close_check", "UPDATE seller.cases SET closed_at = opened_at - interval '1 day', closed_by_admin_id = 1, close_reason = 'DECIDED', decision = 'NO_ACTION' WHERE id = '" + caseID + "'"},
		{"案件版本是 0", "cases_version_check", "UPDATE seller.cases SET version = 0 WHERE id = '" + caseID + "'"},
		{"認定被撤銷卻沒有撤銷的紀錄", "violations_revoked_check", "UPDATE seller.violations SET status = 'REVOKED' WHERE id = '" + m.Findings[0].ID.String() + "'"},
		{"認定帶升級前的嚴重度卻沒有升級", "violations_escalation_check", "UPDATE seller.violations SET escalated_from = 'MEDIUM' WHERE id = '" + m.Findings[0].ID.String() + "'"},
		{"認定的嚴重度不認得", "violations_severity_check", "UPDATE seller.violations SET severity = 'WHATEVER' WHERE id = '" + m.Findings[0].ID.String() + "'"},
	} {
		f.wantViolates(t, v.name, v.constraint, strings.ReplaceAll(v.sql, "$1", storeID))
	}
	// 每次送出各自編號：原通報（第 0 次）與第 1 則補充各接受次內位置 0 到 4，同一次送出的同一個位置不能重複。
	for _, e := range []struct{ seq, position int }{{0, 0}, {0, model.MaxEvidence - 1}} {
		if _, err := f.db.Conn(ctx).Exec(ctx, "INSERT INTO seller.report_evidence (report_id, submission_seq, position, asset_id, object_key) VALUES ($1, $2, $3, 'a', 'k')", members[0].ID, e.seq, e.position); err != nil {
			t.Errorf("第 %d 次送出的位置 %d 的佐證要接受：%v", e.seq, e.position, err)
		}
	}
	for _, position := range []int{0, model.MaxEvidence - 1} {
		if _, err := f.db.Conn(ctx).Exec(ctx, "INSERT INTO seller.report_evidence (report_id, submission_seq, position, supplement_id, asset_id, object_key) SELECT report_id, seq, $2, id, 'a', 'k' FROM seller.report_supplements WHERE report_id = $1 AND seq = 1", members[0].ID, position); err != nil {
			t.Errorf("補充的位置 %d 的佐證要接受：%v", position, err)
		}
	}
	// 同一次送出的同一個位置不能重複（主鍵有 evidence_accesses 的外鍵依賴，wantViolates 的「拿掉再試」做不了，這裡只比對被擋下的約束）。
	_, err := f.db.Conn(ctx).Exec(ctx, "INSERT INTO seller.report_evidence (report_id, submission_seq, position, asset_id, object_key) VALUES ($1, 0, 0, 'a', 'k')", members[0].ID)
	var pg *pgconn.PgError
	if !errors.As(err, &pg) || pg.ConstraintName != "report_evidence_pkey" {
		t.Errorf("同一次送出的同一個位置要被 report_evidence_pkey 擋下：%v", err)
	}
}

// Recompute 先鎖住帳號的 stint 再讀措施（CR-1）。測試先拿住店家列的鎖，Recompute 卡在鎖上；這時候另一個交易撤銷了措施並
// commit，放開鎖之後 Recompute 看到的必須是撤銷之後的措施：先讀再鎖的話，它拿著撤銷之前的措施判斷，開始一段沒有任何措施支撐
// 的停權期間。
func TestRecomputeReadsMeasuresAfterLockingTheStores(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	suspended := record[StoreSuspended](f)
	accountID, s := f.open(t)
	measureID := uuid.NewV7()
	must(t, f.db.InTx(ctx, func(ctx context.Context) error {
		conn := f.db.Conn(ctx)
		if _, err := conn.Exec(ctx, `INSERT INTO seller.measures (id, account_id, store_id, kind, basis, risk_ground, review_by, statement_to_seller, current_revision, created_by_admin_id, created_at, updated_at, starts_at)
			VALUES ($1, $2, $3, 'SUSPENSION', 'RISK', 'FRAUD', $4, 'x', 1, 1, $5, $5, $5)`, measureID, accountID, s.StoreID, f.now.Add(7*24*time.Hour), f.now); err != nil {
			return err
		}
		_, err := conn.Exec(ctx, `INSERT INTO seller.measure_revisions (measure_id, revision, statement_to_seller, admin_id, created_at) VALUES ($1, 1, 'x', 1, $2)`, measureID, f.now)
		return err
	}))
	holder, err := f.db.Pool().Begin(ctx)
	must(t, err)
	defer func() { _ = holder.Rollback(ctx) }()
	_, err = holder.Exec(ctx, "SELECT 1 FROM seller.stores WHERE store_id=$1 FOR UPDATE", s.StoreID)
	must(t, err)
	done := make(chan error, 1)
	go func() {
		done <- f.db.InTx(ctx, func(ctx context.Context) error {
			return suspension.Recompute(ctx, store.New(f.db.Conn(ctx)), f.bus, f.jobs, f.exposures(), accountID, f.now)
		})
	}()
	f.waitForLockWaiters(t, 1)
	_, err = holder.Exec(ctx, "UPDATE seller.measures SET status = 'REVOKED', revoked_reason = 'OTHER', revoked_by_admin_id = 1, revoked_at = $2 WHERE id = $1", measureID, f.now)
	must(t, err)
	must(t, holder.Commit(ctx))
	must(t, <-done)
	if f.openPeriods(t) != 0 || len(suspended.all()) != 0 {
		t.Fatalf("recompute used the measure that was revoked while it waited: open=%d events=%+v", f.openPeriods(t), suspended.all())
	}
}

// 申覆的資格是帳號擁有這筆措施，不是帳號現在有沒有店家（DR B-5）：所有 stint 都關閉的帳號照樣受處分（關店不解除），必須還能申覆。
func TestAppealWorksForAnAccountWhoseStintsAreAllClosed(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _, m := f.appealable(t)
	_, err := f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	if _, err := f.module.CurrentStore(ctx, accountID); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("the account must have no current store: %v", err)
	}
	appealed := f.appeal(t, accountID, m, "誤判")
	if appealed.Appeal() == nil || appealed.Appeal().State != RequestPending {
		t.Fatalf("appealed=%+v", appealed)
	}
	if _, err := f.module.SubmitAppeal(ctx, SubmitAppealInput{AccountID: f.account(t), MeasureID: m.ID, ExpectedVersion: m.SellerVersion(), Reason: "別人的"}); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("another account: %v", err)
	}
	admin := f.measureOf(t, m.ID)
	accepted, err := f.module.DecideAppeal(ctx, DecideAppealInput{MeasureID: m.ID, ExpectedVersion: admin.Version, AdminID: 9, Outcome: RequestAccepted, Reason: "接受"})
	must(t, err)
	if accepted.State != StateRevoked || f.openPeriods(t) != 0 {
		t.Fatalf("accepting must end the effect and the period on the closed stint: %+v open=%d", accepted, f.openPeriods(t))
	}
}

// 撤銷、提出申覆、裁定申覆都依據管理員可以在狀態不變時改掉的內容（UpdateMeasure）：後台的版本必填，過時的版本回 VersionConflict、
// 不寫入；賣家的申覆看賣家看得到的版本。不推斷重送：指令生效後帶舊版本再送一次，後台的指令一樣是 VersionConflict（不寫入、不重發
// 事件），賣家重送同一個 etag 的申覆是「已申覆過」（failed_precondition，賣家的版本沒有因為申覆改變）。
func TestMeasureCommandsRequireTheCurrentVersion(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	revokedEvents := record[MeasureRevoked](f)
	submitted := record[MeasureRequestSubmitted](f)
	decided := record[MeasureRequestDecided](f)
	edits := 0
	stale := func(m Measure) Measure {
		edits++
		statement := fmt.Sprintf("更新後的理由 %d", edits)
		updated, err := f.module.UpdateMeasure(ctx, UpdateMeasureInput{MeasureID: m.ID, AdminID: 7, ExpectedVersion: f.measureOf(t, m.ID).Version, StatementToSeller: &statement})
		must(t, err)
		if updated.Version != m.Version+1 && updated.Version <= m.Version {
			t.Fatalf("UpdateMeasure 必須讓版本前進：%d → %d", m.Version, updated.Version)
		}
		return updated
	}
	accountID, _, m := f.appealable(t)

	// 沒帶版本：InvalidInput，不寫入。
	if _, err := f.module.RevokeMeasure(ctx, RevokeMeasureInput{MeasureID: m.ID, AdminID: 7, Reason: RevokeOther}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("revoke 沒帶版本：%v", err)
	}
	if _, err := f.module.SubmitAppeal(ctx, SubmitAppealInput{AccountID: accountID, MeasureID: m.ID, Reason: "誤判"}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("appeal 沒帶版本：%v", err)
	}
	if _, err := f.module.DecideAppeal(ctx, DecideAppealInput{MeasureID: m.ID, AdminID: 7, Outcome: RequestAccepted, Reason: "x"}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("decide 沒帶版本：%v", err)
	}

	// 過時的版本：撤銷、申覆都被擋下，措施不變。
	updated := stale(m)
	if _, err := f.module.RevokeMeasure(ctx, RevokeMeasureInput{MeasureID: m.ID, AdminID: 7, ExpectedVersion: m.Version, Reason: RevokeOther}); apperr.KindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("revoke 過時版本：%v", err)
	}
	if _, err := f.module.SubmitAppeal(ctx, SubmitAppealInput{AccountID: accountID, MeasureID: m.ID, ExpectedVersion: m.SellerVersion(), Reason: "誤判"}); apperr.KindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("appeal 過時版本：%v", err)
	}
	if got := f.measureOf(t, m.ID); got.Version != updated.Version {
		t.Fatalf("被擋下的指令不能改版本：%d", got.Version)
	}

	// 申覆：成功之後賣家的 etag 沒有改變（內容沒變），重送是「已申覆過」；後台的版本加一、事件一次。
	appealed := f.appeal(t, accountID, updated, "誤判")
	if appealed.Version != updated.Version+1 || appealed.SellerVersion() != updated.SellerVersion() {
		t.Fatalf("appealed version=%d seller=%d", appealed.Version, appealed.SellerVersion())
	}
	if _, err := f.module.SubmitAppeal(ctx, SubmitAppealInput{AccountID: accountID, MeasureID: m.ID, ExpectedVersion: updated.SellerVersion(), Reason: "誤判"}); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("申訴重送：%v", err)
	}
	if got := f.measureOf(t, m.ID); got.Version != appealed.Version || len(submitted.all()) != 1 {
		t.Fatalf("申訴重送不能寫入：version=%d events=%d", got.Version, len(submitted.all()))
	}

	// 裁定：管理員在申覆待裁定時縮短了結束時間，依舊版本的裁定被擋下。
	shorter := appealed.EndsAt.Add(-time.Hour)
	changed, err := f.module.UpdateMeasure(ctx, UpdateMeasureInput{MeasureID: m.ID, AdminID: 7, ExpectedVersion: appealed.Version, ChangeEndsAt: true, EndsAt: &shorter})
	must(t, err)
	if changed.Version != appealed.Version+1 {
		t.Fatalf("縮短必須讓版本加一：%d → %d", appealed.Version, changed.Version)
	}
	if _, err := f.module.DecideAppeal(ctx, DecideAppealInput{MeasureID: m.ID, AdminID: 7, ExpectedVersion: appealed.Version, Outcome: RequestAccepted, Reason: "x"}); apperr.KindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("decide 過時版本：%v", err)
	}
	accepted, err := f.module.DecideAppeal(ctx, DecideAppealInput{MeasureID: m.ID, AdminID: 7, ExpectedVersion: changed.Version, Outcome: RequestAccepted, Reason: "接受"})
	must(t, err)
	if accepted.Version != changed.Version+1 || accepted.Status != MeasureStatusRevoked {
		t.Fatalf("accepted=%+v", accepted)
	}
	if _, err := f.module.DecideAppeal(ctx, DecideAppealInput{MeasureID: m.ID, AdminID: 7, ExpectedVersion: changed.Version, Outcome: RequestAccepted, Reason: "接受"}); apperr.KindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("裁定重送：%v", err)
	}
	if got := f.measureOf(t, m.ID); got.Version != accepted.Version || len(decided.all()) != 1 || len(revokedEvents.all()) != 1 {
		t.Fatalf("裁定重送不能寫入：version=%d decided=%d revoked=%d", got.Version, len(decided.all()), len(revokedEvents.all()))
	}
	// 帶目前的版本再裁定：沒有待裁定的申覆。
	if _, err := f.module.DecideAppeal(ctx, DecideAppealInput{MeasureID: m.ID, AdminID: 7, ExpectedVersion: accepted.Version, Outcome: RequestAccepted, Reason: "接受"}); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("帶目前版本再裁定：%v", err)
	}

	// 撤銷：過時、重送。
	_, s := f.open(t)
	w := f.violate(t, s.StoreID, SeverityMedium, MeasureWarning, nil)
	w2 := stale(w)
	if _, err := f.module.RevokeMeasure(ctx, RevokeMeasureInput{MeasureID: w.ID, AdminID: 7, ExpectedVersion: w.Version, Reason: RevokeAdminCorrection}); apperr.KindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("revoke 過時版本：%v", err)
	}
	revoked, err := f.module.RevokeMeasure(ctx, RevokeMeasureInput{MeasureID: w.ID, AdminID: 7, ExpectedVersion: w2.Version, Reason: RevokeAdminCorrection})
	must(t, err)
	before := len(revokedEvents.all())
	if _, err := f.module.RevokeMeasure(ctx, RevokeMeasureInput{MeasureID: w.ID, AdminID: 7, ExpectedVersion: w2.Version, Reason: RevokeAdminCorrection}); apperr.KindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("撤銷重送：%v", err)
	}
	if got := f.measureOf(t, w.ID); got.Version != revoked.Version || len(revokedEvents.all()) != before {
		t.Fatalf("撤銷重送不能寫入：version=%d events=%d→%d", got.Version, before, len(revokedEvents.all()))
	}
	if _, err := f.module.RevokeMeasure(ctx, RevokeMeasureInput{MeasureID: w.ID, AdminID: 7, ExpectedVersion: revoked.Version, Reason: RevokeAdminCorrection}); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("帶目前版本再撤銷：%v", err)
	}
}

// 申覆遇到後台改過的措施：賣家帶改動前的 etag 是衝突，不轉正新的佐證、不寫入。
func TestStaleSellerEtagDoesNotFinalizeOrWrite(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	submitted := record[MeasureRequestSubmitted](f)
	accountID, _, m := f.appealable(t)
	statement := "管理員改過的理由"
	_, err := f.module.UpdateMeasure(ctx, UpdateMeasureInput{MeasureID: m.ID, AdminID: 7, ExpectedVersion: m.Version, StatementToSeller: &statement})
	must(t, err)
	finalized := len(f.media.finalized)
	for _, in := range []SubmitAppealInput{
		{AccountID: accountID, MeasureID: m.ID, ExpectedVersion: m.SellerVersion(), Reason: "誤判", EvidenceTokens: []string{"new1"}},
		{AccountID: accountID, MeasureID: m.ID, ExpectedVersion: m.SellerVersion(), Reason: "別的理由"},
	} {
		if _, err := f.module.SubmitAppeal(ctx, in); apperr.KindOf(err) != apperr.KindVersionConflict {
			t.Fatalf("過時的 etag：%v", err)
		}
	}
	if len(submitted.all()) != 0 || len(f.media.finalized) != finalized || f.count(t, "SELECT count(*) FROM seller.measure_requests") != 0 {
		t.Fatal("過時的 etag 不能轉正佐證或寫入")
	}
}

// can_appeal 與 SubmitAppeal 的前置檢查是同一個規則：未撤銷且沒申覆過才為真；申覆提出後與撤銷後為假。
func TestMeasureCanAppealFollowsTheSubmitAppealRule(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s, m := f.appealable(t)
	if !m.CanAppeal() {
		t.Fatal("an enforced measure without an appeal must be appealable")
	}
	appealed := f.appeal(t, accountID, m, "誤判")
	if appealed.CanAppeal() {
		t.Fatal("an appealed measure must not be appealable")
	}
	mine, err := f.module.AccountMeasure(ctx, accountID, m.ID)
	must(t, err)
	if mine.CanAppeal() {
		t.Fatal("the seller read must agree after the appeal")
	}
	other := f.violate(t, s.StoreID, SeverityLow, MeasureWarning, nil)
	revoked, err := f.module.RevokeMeasure(ctx, RevokeMeasureInput{MeasureID: other.ID, ExpectedVersion: other.Version, AdminID: 7, Reason: RevokeAdminCorrection})
	must(t, err)
	if revoked.CanAppeal() {
		t.Fatal("a revoked measure must not be appealable")
	}
	if _, err := f.module.SubmitAppeal(ctx, SubmitAppealInput{AccountID: accountID, MeasureID: other.ID, ExpectedVersion: revoked.SellerVersion(), Reason: "x"}); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("appeal a revoked measure: %v", err)
	}
}

// Postgres 只存到微秒：重送「目前的結束時間加 500ns」是同一個值，不能被當成延長（凍結時）或一次修改（沒凍結時）。
func TestUpdateMeasureComparesEndTimeAtMicrosecondPrecision(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, _, m := f.appealable(t)
	resend := func(stage string, cur Measure) {
		t.Helper()
		sub := cur.EndsAt.Add(500 * time.Nanosecond)
		got, err := f.module.UpdateMeasure(ctx, UpdateMeasureInput{MeasureID: m.ID, AdminID: 7, ExpectedVersion: f.measureOf(t, m.ID).Version, ChangeEndsAt: true, EndsAt: &sub})
		if err != nil {
			t.Fatalf("%s: a sub-microsecond difference must be a no-op, got %v", stage, err)
		}
		if got.Version != f.measureOf(t, m.ID).Version || !got.EndsAt.Equal(*cur.EndsAt) || got.CurrentRevision != cur.CurrentRevision {
			t.Fatalf("%s: revision %d -> %d, end %v -> %v", stage, cur.CurrentRevision, got.CurrentRevision, cur.EndsAt, got.EndsAt)
		}
	}
	resend("not appealed", m)
	f.appeal(t, accountID, m, "誤判")
	resend("appealed", f.measureOf(t, m.ID))
}

// 單張佐證的問題（空白、media 拒絕 token、不是機敏上傳）指到那一張（EvidenceTokens[i]），整個欄位的問題（張數上限）指 EvidenceTokens；
// media 的內部錯誤照舊（不是 invalid_argument）。通報與申覆共用同一組檢查與轉正。
func TestEvidenceFailuresPointAtTheFailingToken(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s, m := f.appealable(t)
	f.everExposed(t, s.StoreID)
	f.media.rejected = map[string]error{
		"expired":  fmt.Errorf("mediaservice：轉正（code=404）：%w：%w", media.ErrStagedTokenRejected, apperr.InvalidInput("圖片無法使用，請重新上傳")),
		"expired2": fmt.Errorf("mediaservice：轉正（code=404）：%w：%w", media.ErrStagedTokenRejected, apperr.InvalidInput("圖片無法使用，請重新上傳")),
		"down":     errors.New("mediaservice：轉正失敗（code=500）"),
	}
	submitReport := func(tokens ...string) error {
		_, err := f.module.SubmitReport(ctx, SubmitReportInput{StoreID: s.StoreID, ReporterBuyerID: 5, AccountID: accountID + 1000, Category: policy.Fraud, Description: "附圖", EvidenceTokens: tokens})
		return err
	}
	submitAppeal := func(tokens ...string) error {
		_, err := f.module.SubmitAppeal(ctx, SubmitAppealInput{AccountID: accountID, MeasureID: m.ID, ExpectedVersion: m.SellerVersion(), Reason: "已改善", EvidenceTokens: tokens})
		return err
	}
	fields := func(err error) []string {
		var ae *apperr.Error
		if !errors.As(err, &ae) || ae.Kind() != apperr.KindInvalidInput {
			return nil
		}
		return errField(err)
	}
	for name, submit := range map[string]func(...string) error{"通報": submitReport, "申訴": submitAppeal} {
		for _, c := range []struct {
			name   string
			tokens []string
			want   []string
		}{
			{"media 拒絕第二張", []string{"ok", "expired"}, []string{"EvidenceTokens[1]"}},
			{"兩張被 media 拒絕，一次全部回報", []string{"expired", "ok", "expired2"}, []string{"EvidenceTokens[0]", "EvidenceTokens[2]"}},
			{"第三張空白", []string{"ok", "ok2", " "}, []string{"EvidenceTokens[2]"}},
			{"張數上限指整個欄位", []string{"1", "2", "3", "4", "5", "6"}, []string{"EvidenceTokens"}},
		} {
			if got := fields(submit(c.tokens...)); !slices.Equal(got, c.want) {
				t.Errorf("%s／%s：欄位 = %v，預期 %v", name, c.name, got, c.want)
			}
		}
		// 欄位錯誤的文字（進 log）仍留著 media 的原因（mediaservice 的 code 與訊息）。
		if err := submit("ok", "expired"); err == nil || !strings.Contains(err.Error(), "mediaservice：轉正（code=404）") {
			t.Errorf("%s：缺少 media 的原因：%v", name, err)
		}
		if err := submit("ok", "down"); err == nil || apperr.KindOf(err) == apperr.KindInvalidInput {
			t.Errorf("%s：media 的內部錯誤不能變成欄位錯誤：%v", name, err)
		}
		f.media.NotSecure = true
		// 每張都轉正並檢查：兩張都不是機敏上傳時，兩張都回報。
		if got := fields(submit("a", "b")); !slices.Equal(got, []string{"EvidenceTokens[0]", "EvidenceTokens[1]"}) {
			t.Errorf("%s：非機敏上傳欄位 = %v", name, got)
		}
		f.media.NotSecure = false
	}
}

// measures、violations 的 account_id 是 stores.account_id 的複本：複合外鍵 (store_id, account_id) 讓資料庫保證兩者相等；
// measure_requests 沒有讀者，不存帳號。
func TestAccountCopiesAreHeldEqualToTheStoreByCompositeForeignKeys(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	other := accountID + 1000
	for name, sql := range map[string]string{
		"violations": `INSERT INTO seller.violations (id, store_id, account_id, category, severity, origin_measure_id, created_by_admin_id, created_at, updated_at)
			VALUES ($1, $2, $3, 'FRAUD', 'LOW', $1, 1, now(), now())`,
		"measures": `INSERT INTO seller.measures (id, account_id, store_id, kind, basis, risk_ground, review_by, statement_to_seller, current_revision, created_by_admin_id, created_at, updated_at, starts_at)
			VALUES ($1, $3, $2, 'SUSPENSION', 'RISK', 'FRAUD', now() + interval '1 day', 'x', 1, 1, now(), now(), now())`,
	} {
		_, err := f.db.Conn(ctx).Exec(ctx, sql, uuid.NewV7(), s.StoreID, other)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23503" || !strings.Contains(pgErr.ConstraintName, "store_account_fkey") {
			t.Errorf("%s：帳號和店家對不上要被複合外鍵擋下：%v", name, err)
		}
	}
	if n := f.count(t, `SELECT count(*) FROM information_schema.columns WHERE table_schema = 'seller' AND table_name = 'measure_requests' AND column_name = 'account_id'`); n != 0 {
		t.Error("measure_requests 不存 account_id（沒有讀者）")
	}
}
