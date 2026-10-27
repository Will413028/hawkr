//go:build integration

package seller_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgconn"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/closing"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

// terminate 以指定事由建立終止合作（預設不帶認定；MATERIAL_BREACH 與 LINKED_ACCOUNT 用 createTermination 另給）。
func (f *fixture) terminate(t *testing.T, storeID int64, ground TerminationGround) Measure {
	t.Helper()
	r, err := f.createTermination(storeID, ground, nil)
	must(t, err)
	return r.Measure
}

func (f *fixture) createTermination(storeID int64, ground TerminationGround, mutate func(*CreateMeasureInput)) (CreateMeasureResult, error) {
	in := CreateMeasureInput{
		AdminID: 7, StoreID: storeID, Kind: MeasureTermination, StatementToSeller: "終止合作", Note: "內部紀錄",
		Termination: &TerminationBasis{Ground: ground},
	}
	if mutate != nil {
		mutate(&in)
	}
	return f.module.CreateMeasure(context.Background(), in)
}

func (f *fixture) storeRow(t *testing.T, storeID int64) Store {
	t.Helper()
	s, err := f.module.Store(context.Background(), storeID)
	must(t, err)
	return s
}

// effect 跑一次終止生效的 job（內容）。
func (f *fixture) effect(t *testing.T, m Measure) {
	t.Helper()
	must(t, f.module.EffectTermination(context.Background(), m.ID))
}

// terminationJobs 是排入的終止生效 job：測量 id 與排程時間。
func (f *fixture) terminationJobs(measureID uuid.UUID) (scheduled []time.Time) {
	f.jobs.mu.Lock()
	defer f.jobs.mu.Unlock()
	for i, a := range f.jobs.args {
		if args, ok := a.(closing.TerminationEffectArgs); ok && args.MeasureID == measureID {
			opts := f.jobs.opts[i]
			if opts == nil || !opts.UniqueOpts.ByArgs {
				continue
			}
			scheduled = append(scheduled, opts.ScheduledAt)
		}
	}
	return scheduled
}

func constraintOf(err error) string {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		return pg.ConstraintName
	}
	return ""
}

// 21.2：建立後是 SCHEDULED，period.start_time 是建立時間加 720 小時（30 日），此時帳號不受限制、沒有 StoreSuspended；到了開始時間 job 讓它生效：
// ACTIVE、StoreSuspended 發一次（重跑不再發）；沒有未結訂單才關店，並隨店家關閉結束預告終止（系統作者、MeasureEnded）。
// mutation：NoticePeriod 改回 336 小時，期間與 CHECK 都失敗；StateOf 拿掉 SCHEDULED 分支，第一個斷言失敗。
func TestNoticeTerminationIsScheduledThenTakesEffect(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	suspended, unsuspended, ended := record[StoreSuspended](f), record[StoreUnsuspended](f), record[MeasureEnded](f)
	created := record[MeasureCreated](f)
	accountID, s := f.open(t)
	m := f.terminate(t, s.StoreID, GroundNotice)
	starts := f.now.Add(720 * time.Hour)
	if m.State != StateScheduled || !m.StartsAt.Equal(starts) || m.Basis != BasisContract || m.TerminationGround == nil || *m.TerminationGround != GroundNotice ||
		m.EndsAt != nil || m.ReviewBy != nil || m.SourceMeasureID != nil {
		t.Fatalf("m=%+v", m)
	}
	if d := m.AppealDeadline(); d == nil || !d.Equal(starts.Add(4416*time.Hour)) {
		t.Fatalf("appeal deadline=%v, want start+4416h", d)
	}
	if f.isSuspended(t, s.StoreID) || len(suspended.all()) != 0 {
		t.Fatal("預告期間帳號不受限制，也沒有 StoreSuspended")
	}
	if got := created.all(); len(got) != 1 || !got[0].StartsAt.Equal(starts) || got[0].TerminationGround == nil || *got[0].TerminationGround != GroundNotice || got[0].Kind != MeasureTermination {
		t.Fatalf("MeasureCreated 要帶開始時間與事由：%+v", got)
	}
	if got := f.terminationJobs(m.ID); len(got) != 1 || !got[0].Equal(starts) {
		t.Fatalf("job 要排在開始時間：%v", got)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.measure_identities WHERE measure_id=$1", m.ID); n != 0 {
		t.Fatalf("預告終止不取身分快照（不禁止重新申請）：%d", n)
	}
	stores, err := f.module.AccountMeasures(ctx, AccountMeasureListInput{AccountID: accountID, State: ptr(StateScheduled)})
	must(t, err)
	if len(stores.Items) != 1 || stores.Items[0].ID != m.ID {
		t.Fatalf("SCHEDULED 清單=%+v", stores.Items)
	}

	// 生效的前一刻：job 提早（時鐘慢）也以 starts_at 為準；有未結訂單時只暫停營業、不關店。
	f.orders.open = true
	f.now = starts.Add(-time.Microsecond)
	if got := f.measureOf(t, m.ID).State; got != StateScheduled {
		t.Fatalf("開始前一微秒 state=%s", got)
	}
	f.now = starts
	if got := f.measureOf(t, m.ID).State; got != StateActive {
		t.Fatalf("開始的那一刻 state=%s", got)
	}
	f.effect(t, m)
	f.effect(t, m)
	if !f.isSuspended(t, s.StoreID) || len(suspended.all()) != 1 || suspended.all()[0].StoreID != s.StoreID {
		t.Fatalf("StoreSuspended 發一次：%+v", suspended.all())
	}
	if got := f.storeRow(t, s.StoreID); got.Status != Active || got.Closure != nil || !f.isSeller(t, accountID) {
		t.Fatalf("有未結訂單不關店：%+v", got)
	}

	// 沒有未結訂單才關店；預告終止隨關店結束，系統作者。
	f.orders.open = false
	f.effect(t, m)
	closed := f.storeRow(t, s.StoreID)
	if closed.Status != Closed || closed.Closure == nil || closed.Closure.By != ClosedByTermination || closed.Closure.MeasureID == nil || *closed.Closure.MeasureID != m.ID || f.isSeller(t, accountID) {
		t.Fatalf("closed=%+v closure=%+v", closed, closed.Closure)
	}
	after := f.measureOf(t, m.ID)
	if after.State != StateEnded || after.EndsAt == nil || !after.EndsAt.Equal(f.now) || len(after.Revisions) != 2 {
		t.Fatalf("after=%+v", after)
	}
	last := after.Revisions[1]
	if last.AdminID != nil || last.SystemCause == nil || *last.SystemCause != RevisionStoreClosed || last.StatementToSeller != "終止合作" {
		t.Fatalf("系統版本：%+v", last)
	}
	if got := ended.all(); len(got) != 1 || got[0].MeasureID != m.ID {
		t.Fatalf("MeasureEnded=%+v", got)
	}
	if f.isSuspended(t, s.StoreID) || len(unsuspended.all()) != 1 {
		t.Fatalf("21.2 結束後帳號恢復：unsuspended=%+v", unsuspended.all())
	}
	if _, err := f.module.Apply(ctx, validApply(accountID)); err != nil {
		t.Fatalf("21.2 允許重新申請開店：%v", err)
	}
}

// 資料庫的 CHECK 擋下各種不合法的組合（每一列只違反一條，所以回報的約束名稱就是那一條）。
func TestTerminationSchemaConstraints(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	source := f.terminate(t, s.StoreID, GroundCeasedBusiness)
	insert := func(kind, basis, ground string, startsOffset time.Duration, riskGround, review, source any, ends any) error {
		id := uuid.NewV7()
		tx, err := f.db.Pool().Begin(ctx)
		must(t, err)
		defer func() { _ = tx.Rollback(ctx) }()
		if _, err = tx.Exec(ctx, `INSERT INTO seller.measures (id, account_id, store_id, kind, basis, risk_ground, review_by, termination_ground, source_measure_id, statement_to_seller, ends_at,
				current_revision, created_by_admin_id, created_at, updated_at, starts_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''), $9, 'x', $10, 1, 1, $11::timestamptz, $11::timestamptz, $11::timestamptz + $12::interval)`,
			id, accountID, s.StoreID, kind, basis, riskGround, review, ground, source, ends, f.now, fmt.Sprintf("%d hours", int(startsOffset.Hours()))); err != nil {
			return err
		}
		return nil
	}
	future := f.now.Add(24 * time.Hour)
	for name, tc := range map[string]struct {
		err  error
		want string
	}{
		"TERMINATION 配 RISK":    {insert("TERMINATION", "RISK", "CEASED_BUSINESS", 0, "FRAUD", future, nil, nil), "measures_basis_check"},
		"CONTRACT 配 SUSPENSION": {insert("SUSPENSION", "CONTRACT", "", 0, nil, nil, nil, future), "measures_contract_check"},
		"預告終止的開始時間不是建立加 720 小時": {insert("TERMINATION", "CONTRACT", "NOTICE", 0, nil, nil, nil, nil), "measures_starts_check"},
		"預告終止的開始時間是建立加 336 小時":  {insert("TERMINATION", "CONTRACT", "NOTICE", 336*time.Hour, nil, nil, nil, nil), "measures_starts_check"},
		"21.3 的開始時間不是建立時間":      {insert("TERMINATION", "CONTRACT", "CEASED_BUSINESS", time.Hour, nil, nil, nil, nil), "measures_starts_check"},
		"停權的開始時間不是建立時間":         {insert("SUSPENSION", "RISK", "", time.Hour, "FRAUD", future, nil, nil), "measures_starts_check"},
		"關聯帳號沒有來源措施":            {insert("TERMINATION", "CONTRACT", "LINKED_ACCOUNT", 0, nil, nil, nil, nil), "measures_source_check"},
		"非關聯帳號卻有來源措施":           {insert("TERMINATION", "CONTRACT", "CEASED_BUSINESS", 0, nil, nil, source.ID, nil), "measures_source_check"},
		"重大違約不是 VIOLATION":      {insert("TERMINATION", "CONTRACT", "MATERIAL_BREACH", 0, nil, nil, nil, nil), "measures_breach_check"},
		"終止沒有事由":                {insert("TERMINATION", "CONTRACT", "", 0, nil, nil, nil, nil), "measures_termination_check"},
		"停權卻有終止事由":              {insert("SUSPENSION", "RISK", "CEASED_BUSINESS", 0, "FRAUD", future, nil, nil), "measures_termination_check"},
		"不認得的終止事由":              {insert("TERMINATION", "CONTRACT", "SOMETHING", 0, nil, nil, nil, nil), "measures_termination_ground_check"},
	} {
		if tc.err == nil || constraintOf(tc.err) != tc.want {
			t.Errorf("%s：要被 %s 擋下：%v", name, tc.want, tc.err)
		}
	}
	// 合法的形狀可以寫入（延遲的外鍵在 commit 才檢查，這裡 rollback）。
	if err := insert("TERMINATION", "CONTRACT", "CEASED_BUSINESS", 0, nil, nil, nil, nil); err != nil {
		t.Errorf("合法形狀：%v", err)
	}
}

// 建立時的驗證：終止合作不接受結束時間與複審期限，事由與依據要對應，重大違約要引用夠久的認定，只有關聯帳號帶來源措施。
func TestCreateTerminationValidates(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.open(t)
	end := f.now.Add(time.Hour)
	finding := &NewFinding{Category: policy.Fraud, Severity: SeverityLow}
	for name, tc := range map[string]struct {
		ground TerminationGround
		mutate func(*CreateMeasureInput)
		field  string
	}{
		"結束時間":                    {GroundCeasedBusiness, func(in *CreateMeasureInput) { in.EndsAt = &end }, "EndsAt"},
		"複審期限":                    {GroundCeasedBusiness, func(in *CreateMeasureInput) { in.ReviewBy = &end }, "ReviewBy"},
		"不認得的事由":                  {"SOMETHING", nil, "Termination.Ground"},
		"空的事由":                    {"", nil, "Termination.Ground"},
		"預告終止帶認定":                 {GroundNotice, func(in *CreateMeasureInput) { in.Termination.Violation = &ViolationBasis{Finding: finding} }, "Termination.Violation"},
		"重大違約沒有認定":                {GroundMaterialBreach, nil, "Termination.Violation.CitedFindingIDs"},
		"重大違約只有新認定沒有引用":           {GroundMaterialBreach, func(in *CreateMeasureInput) { in.Termination.Violation = &ViolationBasis{Finding: finding} }, "Termination.Violation.CitedFindingIDs"},
		"關聯帳號沒有來源":                {GroundLinkedAccount, nil, "Termination.SourceMeasureID"},
		"非關聯帳號卻有來源":               {GroundCeasedBusiness, func(in *CreateMeasureInput) { id := uuid.NewV7(); in.Termination.SourceMeasureID = &id }, "Termination.SourceMeasureID"},
		"種類是停權卻帶終止事由":             {GroundCeasedBusiness, func(in *CreateMeasureInput) { in.Kind = MeasureSuspension }, "Kind"},
		"種類是終止卻沒有事由":              {GroundCeasedBusiness, func(in *CreateMeasureInput) { in.Termination = nil; in.Risk = &RiskBasis{Ground: RiskFraud} }, "Kind"},
		"依據認定的終止（沒走 Termination）": {GroundCeasedBusiness, func(in *CreateMeasureInput) { in.Termination = nil; in.Violation = &ViolationBasis{Finding: finding} }, "Kind"},
	} {
		_, err := f.createTermination(s.StoreID, tc.ground, tc.mutate)
		if apperr.KindOf(err) != apperr.KindInvalidInput || !slices.Contains(errField(err), tc.field) {
			t.Errorf("%s：err=%v fields=%v，要 invalid_input 指 %s", name, err, errField(err), tc.field)
		}
	}
	// 句子本身也釘住：事由名稱來自共用文案目錄（後台受眾），契約限定的事由兩個都要有。
	for ground, want := range map[TerminationGround]string{
		GroundNotice:        "預告終止（第 21.2 條）只能依據契約條款，不可以帶認定",
		GroundLinkedAccount: "關聯帳號只能依據契約條款，不可以帶認定",
	} {
		sourceID := uuid.NewV7()
		_, err := f.createTermination(s.StoreID, ground, func(in *CreateMeasureInput) {
			in.Termination.Violation = &ViolationBasis{Finding: finding}
			in.Termination.SourceMeasureID = &sourceID
		})
		var ae *apperr.Error
		if !errors.As(err, &ae) || ae.Message != want {
			t.Errorf("%s：訊息 %v，要 %q", ground, err, want)
		}
	}
	if n := f.count(t, "SELECT count(*) FROM seller.measures"); n != 0 {
		t.Fatalf("驗證失敗不留紀錄：%d", n)
	}
}

// 21.3 立即生效：建立就停權（StoreSuspended），而且逐措施在建立時取身分快照——帳號已因其他措施停權時也要有（不靠 Recompute 的轉換）。
// mutation：把 RecordIdentities 改成跟著 Recompute 的轉換，已停權帳號上的第二個措施沒有快照。
func TestImmediateTerminationSuspendsAtOnceAndSnapshotsTheIdentityPerMeasure(t *testing.T) {
	t.Parallel()
	f := setup(t)
	suspended := record[StoreSuspended](f)
	accountID, s, _ := f.submitted(t, idA, bankAcct)
	f.suspend(t, s.StoreID)
	if len(suspended.all()) != 1 {
		t.Fatal("先有一個風險停權")
	}
	m := f.terminate(t, s.StoreID, GroundFraud)
	if m.State != StateActive || !m.StartsAt.Equal(f.now) || m.Basis != BasisContract {
		t.Fatalf("m=%+v", m)
	}
	if len(suspended.all()) != 1 {
		t.Fatalf("已停權的帳號不再發 StoreSuspended：%d", len(suspended.all()))
	}
	if n := f.count(t, "SELECT count(*) FROM seller.measure_identities WHERE measure_id=$1", m.ID); n == 0 {
		t.Fatal("已因其他措施停權的帳號上建立的 21.3 終止要有身分快照")
	}
	_ = accountID
	// 預告終止從頭到尾沒有快照。
	_, s2, _ := f.submitted(t, idB, bankAcct)
	notice := f.terminate(t, s2.StoreID, GroundNotice)
	f.now = f.now.Add(721 * time.Hour)
	f.orders.open = true
	f.effect(t, notice)
	if n := f.count(t, "SELECT count(*) FROM seller.measure_identities WHERE measure_id=$1", notice.ID); n != 0 {
		t.Fatalf("21.2 從頭到尾沒有快照：%d", n)
	}
}

// 生效中的終止在沒有未結訂單時關店，closed_by 是 TERMINATION 並指向措施；有未結訂單或查詢失敗都不關（fail-closed）。
func TestTerminationClosesTheStoreOnlyWithoutOpenOrdersAndFailsClosed(t *testing.T) {
	t.Parallel()
	f := setup(t)
	accountID, s := f.open(t)
	m := f.terminate(t, s.StoreID, GroundCeasedBusiness)
	f.orders.open = true
	f.effect(t, m)
	if f.storeRow(t, s.StoreID).Status != Active {
		t.Fatal("有未結訂單不關店")
	}
	f.orders.open, f.orders.err = false, errors.New("offline")
	if err := f.module.EffectTermination(context.Background(), m.ID); err == nil {
		t.Fatal("訂單查詢失敗要回錯誤（讓 job 重試），不關店")
	}
	if f.storeRow(t, s.StoreID).Status != Active || !f.isSeller(t, accountID) {
		t.Fatal("查詢失敗不關店")
	}
	f.orders.err = nil
	f.effect(t, m)
	closed := f.storeRow(t, s.StoreID)
	if closed.Status != Closed || closed.Closure == nil || closed.Closure.By != ClosedByTermination || *closed.Closure.MeasureID != m.ID || f.isSeller(t, accountID) {
		t.Fatalf("closed=%+v", closed)
	}
	if row := f.closureOf(t, s.StoreID); row.by == nil || *row.by != "TERMINATION" || row.measureID == nil || *row.measureID != m.ID.String() {
		t.Fatalf("row=%+v", row)
	}
	if got := f.measureOf(t, m.ID); got.State != StateActive || got.EndsAt != nil {
		t.Fatalf("21.3 關店後措施仍生效、沒有結束時間：%+v", got)
	}
	// 重跑沒有副作用。
	versionBefore := f.storeRow(t, s.StoreID).Version
	f.effect(t, m)
	if f.storeRow(t, s.StoreID).Version != versionBefore {
		t.Fatal("重跑改動了店家")
	}
}

// 競態：訂單檢查之後、關店 transaction 之前措施被撤銷——關店 transaction 內重讀措施，不關。
// mutation：拿掉 closeTerminated 內的重讀（stateOf 判斷），這裡會關店。
func TestTerminationRechecksTheMeasureInsideTheClosingTransaction(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	m := f.terminate(t, s.StoreID, GroundCeasedBusiness)
	f.orders.onCall = func() {
		current := f.measureOf(t, m.ID)
		_, err := f.module.RevokeMeasure(ctx, RevokeMeasureInput{MeasureID: m.ID, AdminID: 9, ExpectedVersion: current.Version, Reason: RevokeOther})
		if err != nil {
			t.Error(err)
		}
	}
	f.effect(t, m)
	if got := f.storeRow(t, s.StoreID); got.Status != Active || !f.isSeller(t, accountID) {
		t.Fatalf("措施已撤銷就不該關店：%+v", got)
	}
}

// 帳號在等待刪除：job 視為終局、成功結束、不關店；刪帳以 ACCOUNT_DELETION 關店，21.3 維持。
// mutation：job 不判斷等待刪除，會回 FailedPrecondition 重試（這裡斷言 job 成功結束）。
func TestTerminationOfAnAccountWaitingForDeletionIsTerminalAndTheDeletionClosesIt(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	m := f.terminate(t, s.StoreID, GroundCeasedBusiness)
	must(t, f.db.InTx(ctx, func(ctx context.Context) error { return f.module.BeginDeletion(ctx, s.StoreID) }))
	f.effect(t, m)
	if got := f.storeRow(t, s.StoreID); got.Status != PendingDeletion || got.Closure != nil {
		t.Fatalf("等待刪除的店不由終止關閉：%+v", got)
	}
	must(t, f.db.InTx(ctx, func(ctx context.Context) error { return f.module.CloseForDeletion(ctx, s.StoreID) }))
	if row := f.closureOf(t, s.StoreID); row.by == nil || *row.by != "ACCOUNT_DELETION" || row.measureID != nil {
		t.Fatalf("row=%+v", row)
	}
	if got := f.measureOf(t, m.ID); got.State != StateActive {
		t.Fatalf("21.3 維持：%s", got.State)
	}
	f.effect(t, m)
}

// job 遺失時只靠每小時的掃描，結果相同；已排的 job 與掃描都跑過也相同（冪等）。
func TestReconcileTerminationsCoversALostJob(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	m := f.terminate(t, s.StoreID, GroundCeasedBusiness)
	scanned, err := f.module.ReconcileTerminations(ctx)
	must(t, err)
	if scanned != 1 {
		t.Fatalf("scanned=%d", scanned)
	}
	closed := f.storeRow(t, s.StoreID)
	if closed.Status != Closed || closed.Closure == nil || *closed.Closure.MeasureID != m.ID || f.isSeller(t, accountID) {
		t.Fatalf("closed=%+v", closed)
	}
	// 沒有目前 stint 的帳號不掃。
	scanned, err = f.module.ReconcileTerminations(ctx)
	must(t, err)
	if scanned != 0 {
		t.Fatalf("已關店的帳號不再掃描：%d", scanned)
	}
	// 事件加速：店家的訂單結案。
	_, s2 := f.open(t)
	m2 := f.terminate(t, s2.StoreID, GroundBankruptcy)
	f.orders.open = true
	must(t, f.module.AdvanceTerminations(ctx, s2.StoreID))
	if f.storeRow(t, s2.StoreID).Status != Active {
		t.Fatal("還有未結訂單")
	}
	f.orders.open = false
	must(t, f.module.AdvanceTerminations(ctx, s2.StoreID))
	if got := f.storeRow(t, s2.StoreID); got.Status != Closed || *got.Closure.MeasureID != m2.ID {
		t.Fatalf("加速之後關店：%+v", got)
	}
}

// 21.2 在單一關店路徑結束：預告期間內賣家自己關店、生效後賣家自己關店、刪帳關店，措施都結束（系統作者、MeasureEnded），帳號（未刪者）可以開新店。
// mutation：只在 termination job 寫 ends_at（不在 close()）——前兩者失敗。
func TestNoticeEndsOnEveryPathThatClosesTheStore(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		after   time.Duration
		close   func(f *fixture, accountID int64, s Store) error
		reapply bool
	}{
		{"賣家在預告期間關店", 24 * time.Hour, func(f *fixture, accountID int64, _ Store) error {
			_, err := f.module.CloseCurrent(context.Background(), accountID)
			return err
		}, true},
		{"賣家在生效之後關店", 721 * time.Hour, func(f *fixture, accountID int64, _ Store) error {
			_, err := f.module.CloseCurrent(context.Background(), accountID)
			return err
		}, true},
		{"刪帳關店", 24 * time.Hour, func(f *fixture, _ int64, s Store) error {
			return f.db.InTx(context.Background(), func(ctx context.Context) error { return f.module.CloseForDeletion(ctx, s.StoreID) })
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setup(t)
			ended := record[MeasureEnded](f)
			accountID, s := f.open(t)
			m := f.terminate(t, s.StoreID, GroundNotice)
			f.now = f.now.Add(tc.after)
			must(t, tc.close(f, accountID, s))
			got := f.measureOf(t, m.ID)
			if got.State != StateEnded || got.EndsAt == nil || !got.EndsAt.Equal(f.now) || len(got.Revisions) != 2 {
				t.Fatalf("got=%+v", got)
			}
			if r := got.Revisions[1]; r.AdminID != nil || r.SystemCause == nil || *r.SystemCause != RevisionStoreClosed {
				t.Fatalf("revision=%+v", r)
			}
			if len(ended.all()) != 1 {
				t.Fatalf("MeasureEnded=%+v", ended.all())
			}
			if f.isSuspended(t, s.StoreID) {
				t.Fatal("結束之後帳號不受限制")
			}
			if tc.reapply {
				if _, err := f.module.Apply(context.Background(), validApply(accountID)); err != nil {
					t.Fatalf("21.2 結束後可重新申請：%v", err)
				}
			}
		})
	}
}

// 21.3：關店之後同帳號開店被擋，別的帳號以同一個身分證號送 KYC 被擋（既有的 stint.Derive 與身分快照），措施仍是 ACTIVE。
func TestMaterialTerminationBlocksReapplicationAndTheSameIdentityElsewhere(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s, _ := f.submitted(t, idA, bankAcct)
	m := f.terminate(t, s.StoreID, GroundFraud)
	f.effect(t, m)
	if f.storeRow(t, s.StoreID).Status != Closed {
		t.Fatal("應該已關店")
	}
	if _, err := f.module.Apply(ctx, validApply(accountID)); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("21.3 之後同帳號開店要被擋：%v", err)
	}
	other, _ := f.open(t)
	draft := f.savedDraft(t, kycDraft(other, idA, "999888777666"))
	_, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: other, ExpectedVersion: draft.Version})
	if apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("別的帳號以同一個身分證號送 KYC 要被擋：%v", err)
	}
	if got := f.measureOf(t, m.ID); got.State != StateActive {
		t.Fatalf("state=%s", got.State)
	}
}

// 沒有目前 stint 的帳號可以建 21.3（只禁止入駐，store_id 取最近的 stint）；預告終止只能建在開著的店（NotFound）；較舊的 stint 不行。
func TestTerminationOnAnAccountWithoutACurrentStint(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, first := f.open(t)
	_, err := f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	if _, err := f.createTermination(first.StoreID, GroundNotice, nil); apperr.KindOf(err) != apperr.KindFailedPrecondition || !strings.Contains(err.Error(), "店家已關閉。") {
		t.Fatalf("預告終止只能建在開著的店：%v", err)
	}
	m := f.terminate(t, first.StoreID, GroundFraud)
	if m.State != StateActive || m.StoreID != first.StoreID || m.AccountID != accountID {
		t.Fatalf("m=%+v", m)
	}
	// 開了新店（沒有被擋的帳號）：舊的 stint 不再是最近的。
	f2 := setup(t)
	account2, a := f2.open(t)
	_, err = f2.module.CloseCurrent(ctx, account2)
	must(t, err)
	second, err := f2.module.Apply(ctx, validApply(account2))
	must(t, err)
	if _, err := f2.createTermination(a.StoreID, GroundFraud, nil); apperr.KindOf(err) != apperr.KindFailedPrecondition || !strings.Contains(err.Error(), "店家已關閉，帳號有較新的店家。") {
		t.Fatalf("帳號有較新的 stint 時，建在舊 stint 上要 failed_precondition：%v", err)
	}
	if got := f2.terminate(t, second.StoreID, GroundFraud); got.StoreID != second.StoreID {
		t.Fatalf("got=%+v", got)
	}
}

// MATERIAL_BREACH（條款 21.3 第 1 款）：引用的認定要至少一筆未撤銷、建立它的措施已存在 7 日以上。
// mutation：拿掉「未撤銷」條件；把 7 天改成 0。
func TestMaterialBreachNeedsAFindingOlderThanSevenDays(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	warn := f.violate(t, s.StoreID, SeverityLow, MeasureWarning, nil)
	finding := warn.Findings[0].ID
	breach := func() (CreateMeasureResult, error) {
		return f.createTermination(s.StoreID, GroundMaterialBreach, func(in *CreateMeasureInput) {
			in.Termination.Violation = &ViolationBasis{CitedFindingIDs: []uuid.UUID{finding}}
		})
	}
	f.now = f.now.Add(7*24*time.Hour - time.Microsecond)
	if _, err := breach(); apperr.KindOf(err) != apperr.KindInvalidInput || !slices.Contains(errField(err), "Termination.Violation.CitedFindingIDs") {
		t.Fatalf("不足 7 天：%v %v", err, errField(err))
	}
	f.now = f.now.Add(time.Microsecond)
	res, err := breach()
	must(t, err)
	if res.Measure.Basis != BasisViolation || len(res.Measure.Findings) != 1 || res.Measure.Findings[0].ID != finding || res.Measure.State != StateActive {
		t.Fatalf("m=%+v", res.Measure)
	}
	// 已撤銷的認定不算。
	_, s2 := f.open(t)
	warn2 := f.violate(t, s2.StoreID, SeverityLow, MeasureWarning, nil)
	f.now = f.now.Add(8 * 24 * time.Hour)
	_, err = f.module.RevokeMeasure(ctx, RevokeMeasureInput{MeasureID: warn2.ID, AdminID: 9, ExpectedVersion: warn2.Version, Reason: RevokeOther, RevokeFindings: true})
	must(t, err)
	_, err = f.createTermination(s2.StoreID, GroundMaterialBreach, func(in *CreateMeasureInput) {
		in.Termination.Violation = &ViolationBasis{CitedFindingIDs: []uuid.UUID{warn2.Findings[0].ID}}
	})
	if apperr.KindOf(err) != apperr.KindInvalidInput || !slices.Equal(errField(err), []string{"Termination.Violation.CitedFindingIDs"}) {
		t.Fatalf("已撤銷的認定不算（指向引用的清單，不是逐筆的位置）：%v %v", err, errField(err))
	}
}

// 有狀態的 DB 約束：關店紀錄 store_closures 的形狀。
func TestStoreClosuresSchemaConstraints(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	measureID := uuid.NewV7()
	for name, sql := range map[string]string{
		"終止合作沒有措施":          `INSERT INTO seller.store_closures (store_id, closed_at, closed_by, reopened_at) VALUES ($1, now(), 'TERMINATION', now())`,
		"賣家關店卻有措施":          `INSERT INTO seller.store_closures (store_id, closed_at, closed_by, closed_by_measure_id, reopened_at) VALUES ($1, now(), 'ACCOUNT_OWNER', '` + measureID.String() + `', now())`,
		"不認得的原因":            `INSERT INTO seller.store_closures (store_id, closed_at, closed_by, reopened_at) VALUES ($1, now(), 'ADMIN', now())`,
		"恢復早於關閉":            `INSERT INTO seller.store_closures (store_id, closed_at, closed_by, reopened_at) VALUES ($1, now(), 'ACCOUNT_OWNER', now() - interval '1 hour')`,
		"同一個 stint 兩筆目前的紀錄": `INSERT INTO seller.store_closures (store_id, closed_at, closed_by) VALUES ($1, now(), 'ACCOUNT_OWNER'), ($1, now() + interval '1 second', 'ACCOUNT_OWNER')`,
	} {
		if _, err := f.db.Pool().Exec(ctx, sql, s.StoreID); err == nil {
			t.Errorf("DB 接受了 %s", name)
		}
	}
}

// stores.status 是 CLOSED ⇔ 恰有一列目前的 store_closures，由資料庫在 commit 時保證（stores_current_closure_check、兩個可延遲的複合外鍵）。
// mutation：拿掉 stores_current_closure_fkey、store_closures_current_fkey 或 stores_current_closure_check → 對應的反例被接受，測試失敗。
func TestDatabaseGuaranteesClosedIffOneCurrentClosure(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	commits := func(statements ...string) error {
		t.Helper()
		tx, err := f.db.Pool().Begin(ctx)
		must(t, err)
		defer func() { _ = tx.Rollback(ctx) }()
		for _, sql := range statements {
			if _, err := tx.Exec(ctx, sql); err != nil {
				return err
			}
		}
		return tx.Commit(ctx)
	}
	closedStore := func() int64 {
		account, s := f.open(t)
		_, err := f.module.CloseCurrent(ctx, account)
		must(t, err)
		return s.StoreID
	}
	id := func(storeID int64) string { return strconv.FormatInt(storeID, 10) }

	_, active := f.open(t)
	for name, statements := range map[string][]string{
		"CLOSED 沒有指標":           {"UPDATE seller.stores SET status = 'CLOSED' WHERE store_id = " + id(active.StoreID)},
		"CLOSED 的指標沒有對應的紀錄":     {"UPDATE seller.stores SET status = 'CLOSED', current_closed_at = now() WHERE store_id = " + id(active.StoreID)},
		"ACTIVE 卻有指標":           {"UPDATE seller.stores SET current_closed_at = now() WHERE store_id = " + id(active.StoreID)},
		"目前的紀錄但店家不是 CLOSED":     {"INSERT INTO seller.store_closures (store_id, closed_at, closed_by) VALUES (" + id(active.StoreID) + ", now(), 'ACCOUNT_OWNER')"},
		"紀錄與指標都在，狀態卻沒有改":        {"INSERT INTO seller.store_closures (store_id, closed_at, closed_by) VALUES (" + id(active.StoreID) + ", now(), 'ACCOUNT_OWNER')", "UPDATE seller.stores SET current_closed_at = (SELECT closed_at FROM seller.store_closures WHERE store_id = " + id(active.StoreID) + ") WHERE store_id = " + id(active.StoreID)},
		"CLOSED 的指標指向沒有目前紀錄的時間": {"INSERT INTO seller.store_closures (store_id, closed_at, closed_by, reopened_at) VALUES (" + id(active.StoreID) + ", now(), 'ACCOUNT_OWNER', now())", "UPDATE seller.stores SET status = 'CLOSED', current_closed_at = (SELECT closed_at FROM seller.store_closures WHERE store_id = " + id(active.StoreID) + ") WHERE store_id = " + id(active.StoreID)},
	} {
		if err := commits(statements...); err == nil {
			t.Errorf("DB 接受了 %s", name)
		}
	}

	closed := id(closedStore())
	for name, statements := range map[string][]string{
		"同一個 stint 兩筆目前的紀錄":    {"INSERT INTO seller.store_closures (store_id, closed_at, closed_by) VALUES (" + closed + ", now() + interval '1 second', 'ACCOUNT_OWNER')"},
		"恢復營業卻沒有清指標":           {"UPDATE seller.store_closures SET reopened_at = closed_at + interval '1 second' WHERE store_id = " + closed},
		"恢復營業卻沒有結束目前的紀錄":       {"UPDATE seller.stores SET status = 'ACTIVE', current_closed_at = NULL WHERE store_id = " + closed},
		"只清指標、狀態仍是 CLOSED":     {"UPDATE seller.stores SET current_closed_at = NULL WHERE store_id = " + closed},
		"結束紀錄又清指標，狀態仍是 CLOSED": {"UPDATE seller.store_closures SET reopened_at = closed_at + interval '1 second' WHERE store_id = " + closed, "UPDATE seller.stores SET current_closed_at = NULL WHERE store_id = " + closed},
	} {
		if err := commits(statements...); err == nil {
			t.Errorf("DB 接受了 %s", name)
		}
	}
	if err := commits(
		"UPDATE seller.store_closures SET reopened_at = closed_at + interval '1 second' WHERE store_id = "+closed,
		"UPDATE seller.stores SET status = 'ACTIVE', current_closed_at = NULL WHERE store_id = "+closed,
	); err != nil {
		t.Fatalf("結束紀錄、清指標與恢復狀態在同一個 transaction 是合法的：%v", err)
	}
	if err := commits(
		"UPDATE seller.stores SET status = 'CLOSED', current_closed_at = now() WHERE store_id = "+closed,
		"INSERT INTO seller.store_closures (store_id, closed_at, closed_by) VALUES ("+closed+", now(), 'ACCOUNT_OWNER')",
	); err != nil {
		t.Fatalf("先改店家、後寫紀錄（可延遲）在同一個 transaction 是合法的：%v", err)
	}
}

// 關店的唯一一份紀錄是 store_closures：stores.status 是 CLOSED 的 stint 恰有一筆目前的紀錄（reopened_at 為 NULL），不是的沒有；每一條關店與恢復的路徑
// （賣家自己、終止合作與恢復、隨帳號刪除）之後都成立。
// mutation：close 不寫 OpenStoreClosure → 關店後的檢查失敗；Reopen 不結束目前的紀錄（EndStoreClosure）→ 恢復後的檢查失敗。
func TestClosedStoresHaveExactlyOneCurrentClosure(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	check := func(when string) {
		t.Helper()
		if n := f.count(t, `SELECT count(*) FROM seller.stores s WHERE (s.status = 'CLOSED') <>
			EXISTS (SELECT 1 FROM seller.store_closures c WHERE c.store_id = s.store_id AND c.reopened_at IS NULL)`); n != 0 {
			t.Fatalf("%s：%d 個 stint 的狀態與目前的關店紀錄不一致", when, n)
		}
		if n := f.count(t, `SELECT count(*) FROM (SELECT store_id FROM seller.store_closures WHERE reopened_at IS NULL GROUP BY store_id HAVING count(*) > 1) x`); n != 0 {
			t.Fatalf("%s：有 stint 同時有多筆目前的紀錄", when)
		}
	}
	ownerAccount, _ := f.open(t)
	_, err := f.module.CloseCurrent(ctx, ownerAccount)
	must(t, err)
	check("賣家自己關店")
	_, deleted := f.open(t)
	must(t, f.db.InTx(ctx, func(ctx context.Context) error { return f.module.CloseForDeletion(ctx, deleted.StoreID) }))
	check("隨帳號刪除關店")
	_, s, m := f.terminated(t, GroundFraud)
	check("終止關店")
	f.now = f.now.Add(time.Hour)
	f.revoke(t, m)
	if f.storeRow(t, s.StoreID).Status != Active {
		t.Fatal("撤銷後恢復")
	}
	check("恢復")
	f.now = f.now.Add(time.Hour)
	again := f.terminate(t, s.StoreID, GroundCeasedBusiness)
	f.effect(t, again)
	if f.storeRow(t, s.StoreID).Status != Closed || f.count(t, "SELECT count(*) FROM seller.store_closures WHERE store_id = $1", s.StoreID) != 2 {
		t.Fatal("同一個 stint 再次關店：歷史一筆、目前一筆")
	}
	check("再次關店")
}
