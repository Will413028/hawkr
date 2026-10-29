//go:build integration

package seller_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"
	"uuid"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/jackc/pgx/v5/pgconn"
)

func takedownContents(n int) []ContentRef {
	out := make([]ContentRef, n)
	kinds := []ContentKind{ContentPost, ContentComment, ContentForumEntry}
	for i := range out {
		out[i] = ContentRef{Kind: kinds[i%len(kinds)], ID: uuid.NewV7()}
	}
	return out
}

func (f *fixture) takedownFrom(storeID int64, contents []ContentRef) CreateMeasureInput {
	return CreateMeasureInput{
		AdminID: 7, StoreID: storeID, Kind: MeasureContentTakedown, StatementToSeller: "貼文含垃圾訊息", Note: "內部紀錄",
		Violation: &ViolationBasis{Finding: &NewFinding{Category: policy.Spam, Severity: SeverityLow}},
		Takedown:  &ContentTakedown{Contents: contents},
	}
}

func (f *fixture) takedown(t *testing.T, storeID int64, contents []ContentRef) Measure {
	t.Helper()
	r, err := f.module.CreateMeasure(context.Background(), f.takedownFrom(storeID, contents))
	must(t, err)
	return r.Measure
}

func sortedContents(refs []ContentRef) []ContentRef {
	out := slices.Clone(refs)
	slices.SortFunc(out, func(a, b ContentRef) int {
		if a.Kind != b.Kind {
			return int(a.Kind[0]) - int(b.Kind[0])
		}
		return a.ID.Compare(b.ID)
	})
	return out
}

func TestCreateContentTakedownStoresTargetsAndDoesNotRestrictTheAccount(t *testing.T) {
	t.Parallel()
	f := setup(t)
	created := record[MeasureCreated](f)
	accountID, s := f.open(t)
	contents := takedownContents(3)
	m := f.takedown(t, s.StoreID, contents)
	if m.Kind != MeasureContentTakedown || m.Basis != BasisViolation || m.State != StateIssued || m.EndsAt != nil || !m.StartsAt.Equal(m.CreatedAt) {
		t.Fatalf("措施 %+v", m)
	}
	if got := f.measureOf(t, m.ID).Contents; !slices.Equal(got, sortedContents(contents)) {
		t.Fatalf("讀回的對象 %v，預期 %v", got, sortedContents(contents))
	}
	if !slices.Equal(m.Contents, sortedContents(contents)) {
		t.Fatalf("建立的回應帶對象 %v", m.Contents)
	}
	if m.Findings[0].Category != policy.Spam {
		t.Fatalf("認定的類別 %v", m.Findings[0].Category)
	}
	events := created.all()
	if len(events) != 1 || events[0].Kind != MeasureContentTakedown || !slices.Equal(sortedContents(events[0].Contents), sortedContents(contents)) {
		t.Fatalf("MeasureCreated 要帶對象：%+v", events)
	}
	f.wantSuspended(t, "內容下架不影響能力與曝光", accountID, s.StoreID, false)
	if n := f.count(t, "SELECT count(*) FROM seller.suspension_periods"); n != 0 {
		t.Fatalf("內容下架不開停權期間：%d", n)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.measure_identities"); n != 0 {
		t.Fatalf("內容下架不取身分快照：%d", n)
	}
}

func TestCreateContentTakedownValidates(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	ends := f.now.Add(24 * time.Hour)
	review := f.now.Add(24 * time.Hour)
	dup := takedownContents(1)
	for name, mutate := range map[string]func(in *CreateMeasureInput){
		"沒有對象":    func(in *CreateMeasureInput) { in.Takedown = nil },
		"對象為空":    func(in *CreateMeasureInput) { in.Takedown.Contents = nil },
		"對象重複":    func(in *CreateMeasureInput) { in.Takedown.Contents = append(slices.Clone(dup), dup...) },
		"對象超過上限":  func(in *CreateMeasureInput) { in.Takedown.Contents = takedownContents(21) },
		"對象種類不合法": func(in *CreateMeasureInput) { in.Takedown.Contents = []ContentRef{{Kind: "STORE", ID: uuid.NewV7()}} },
		"有結束時間":   func(in *CreateMeasureInput) { in.EndsAt = &ends },
		"有複審期限":   func(in *CreateMeasureInput) { in.ReviewBy = &review },
		"依據風險類型": func(in *CreateMeasureInput) {
			in.Violation = nil
			in.Risk = &RiskBasis{Ground: RiskFraud}
			in.ReviewBy = &review
		},
		"其他種類帶對象":        func(in *CreateMeasureInput) { in.Kind = MeasureWarning },
		"沒有依據":           func(in *CreateMeasureInput) { in.Violation = nil },
		"認定類別不合法（OTHER）": func(in *CreateMeasureInput) { in.Violation.Finding.Category = policy.Other },
	} {
		in := f.takedownFrom(s.StoreID, takedownContents(1))
		mutate(&in)
		if _, err := f.module.CreateMeasure(ctx, in); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：%v，預期 invalid_argument", name, err)
		}
	}
	if n := f.count(t, "SELECT count(*) FROM seller.measures") + f.count(t, "SELECT count(*) FROM seller.measure_contents"); n != 0 {
		t.Fatalf("被拒絕的請求寫入了 %d 列", n)
	}
}

func TestContentTakedownSchemaConstraints(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	ends := time.Now().Add(24 * time.Hour)
	future := time.Now().Add(24 * time.Hour)
	insert := func(basis string, endsAt, startsAt *time.Time) error {
		tx, err := f.db.Pool().Begin(ctx)
		must(t, err)
		defer func() { _ = tx.Rollback(ctx) }()
		_, err = tx.Exec(ctx, `INSERT INTO seller.measures (id, account_id, store_id, kind, basis, risk_ground, review_by, lead_finding_id, statement_to_seller, ends_at, current_revision, created_by_admin_id, created_at, updated_at, starts_at)
			VALUES ($1, $2, $3, 'CONTENT_TAKEDOWN', $4, CASE WHEN $4 = 'RISK' THEN 'FRAUD' END, CASE WHEN $4 = 'RISK' THEN now() + interval '1 day' END,
			        CASE WHEN $4 = 'VIOLATION' THEN $5::uuid END, 'x', $6, 1, 1, now(), now(), COALESCE($7, now()))`,
			uuid.NewV7(), accountID, s.StoreID, basis, uuid.NewV7(), endsAt, startsAt)
		return err
	}
	for name, c := range map[string]struct {
		basis          string
		endsAt, starts *time.Time
	}{
		"有結束時間":  {"VIOLATION", &ends, nil},
		"未來才生效":  {"VIOLATION", nil, &future},
		"依據風險類型": {"RISK", nil, nil},
		"依據契約條款": {"CONTRACT", nil, nil},
	} {
		var pgErr *pgconn.PgError
		err := insert(c.basis, c.endsAt, c.starts)
		if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
			t.Errorf("%s：要被 CHECK 擋下：%v", name, err)
		}
	}
	m := f.takedown(t, s.StoreID, takedownContents(1))
	for name, sql := range map[string]string{
		"不認得的內容種類": "INSERT INTO seller.measure_contents (measure_id, content_kind, content_id) VALUES ($1, 'STORE', uuidv7())",
		"同一個對象兩次":  "INSERT INTO seller.measure_contents (measure_id, content_kind, content_id) SELECT measure_id, content_kind, content_id FROM seller.measure_contents WHERE measure_id = $1",
		"不存在的措施":   "INSERT INTO seller.measure_contents (measure_id, content_kind, content_id) VALUES (uuidv7(), 'POST', uuidv7())",
	} {
		if _, err := f.db.Conn(ctx).Exec(ctx, sql, m.ID); err == nil {
			t.Errorf("%s：資料庫不該接受", name)
		}
	}
}

func TestPubContentTakedownsListsOnlyActiveTakedownTargets(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	list := func() map[ContentRef]int {
		rows, err := f.db.Conn(ctx).Query(ctx, "SELECT measure_id, content_kind, content_id FROM seller.pub_content_takedowns()")
		must(t, err)
		defer rows.Close()
		out := map[ContentRef]int{}
		for rows.Next() {
			var measure, id uuid.UUID
			var kind string
			must(t, rows.Scan(&measure, &kind, &id))
			out[ContentRef{Kind: ContentKind(kind), ID: id}]++
		}
		must(t, rows.Err())
		return out
	}
	if got := list(); len(got) != 0 {
		t.Fatalf("一開始是空的：%v", got)
	}
	first := takedownContents(2)
	m1 := f.takedown(t, s.StoreID, first)
	shared := []ContentRef{first[0]}
	m2 := f.takedown(t, s.StoreID, shared)
	f.violate(t, s.StoreID, SeverityLow, MeasureWarning, nil)
	got := list()
	if len(got) != 2 || got[first[0]] != 2 || got[first[1]] != 1 {
		t.Fatalf("兩個措施下架同一則內容時各一列：%v", got)
	}
	_, err := f.module.RevokeMeasure(ctx, RevokeMeasureInput{MeasureID: m1.ID, ExpectedVersion: m1.Version, AdminID: 7, Reason: RevokeAdminCorrection})
	must(t, err)
	got = list()
	if len(got) != 1 || got[first[0]] != 1 {
		t.Fatalf("撤銷後只剩另一個措施的對象：%v", got)
	}
	_, err = f.module.RevokeMeasure(ctx, RevokeMeasureInput{MeasureID: m2.ID, ExpectedVersion: m2.Version, AdminID: 7, Reason: RevokeAdminCorrection})
	must(t, err)
	if got := list(); len(got) != 0 {
		t.Fatalf("全部撤銷後是空的：%v", got)
	}
}

func TestContentTakedownIsReleasedOnlyThroughAppealAndRevokeEventsCarryTheTargets(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	revoked := record[MeasureRevoked](f)
	accountID, s := f.open(t)
	contents := takedownContents(2)
	m := f.takedown(t, s.StoreID, contents)

	if _, err := f.module.EndMeasure(ctx, EndMeasureInput{MeasureID: m.ID, AdminID: 7, ExpectedVersion: m.Version}); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("內容下架不能結束：%v", err)
	}
	ends := f.now.Add(48 * time.Hour)
	if _, err := f.module.UpdateMeasure(ctx, UpdateMeasureInput{MeasureID: m.ID, AdminID: 7, ExpectedVersion: m.Version, ChangeEndsAt: true, EndsAt: &ends}); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("內容下架不能設結束時間：%v", err)
	}
	if _, err := f.module.RevokeMeasure(ctx, RevokeMeasureInput{MeasureID: m.ID, ExpectedVersion: m.Version, AdminID: 7, Reason: RevokeSourceRevoked}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("SOURCE_REVOKED 只由連帶撤銷寫入：%v", err)
	}
	if n := len(revoked.all()); n != 0 {
		t.Fatalf("被拒絕的撤銷不發事件：%d", n)
	}

	appealed := f.appeal(t, accountID, m, "這則不是垃圾訊息")
	if appealed.Appeal() == nil {
		t.Fatal("內容下架可以申覆")
	}
	accepted := f.decideAppeal(t, m.ID, RequestAccepted, "查證後撤銷")
	if accepted.Status != MeasureStatusRevoked {
		t.Fatalf("接受申覆撤銷措施：%+v", accepted)
	}
	events := revoked.all()
	if len(events) != 1 || events[0].MeasureID != m.ID || !slices.Equal(sortedContents(events[0].Contents), sortedContents(contents)) {
		t.Fatalf("MeasureRevoked 要帶對象：%+v", events)
	}
}

func TestManualRevokeOfContentTakedownEmitsTheTargets(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	revoked := record[MeasureRevoked](f)
	_, s := f.open(t)
	contents := takedownContents(1)
	m := f.takedown(t, s.StoreID, contents)
	out, err := f.module.RevokeMeasure(ctx, RevokeMeasureInput{MeasureID: m.ID, ExpectedVersion: m.Version, AdminID: 7, Reason: RevokeAdminCorrection})
	must(t, err)
	if out.Status != MeasureStatusRevoked {
		t.Fatalf("%+v", out)
	}
	events := revoked.all()
	if len(events) != 1 || events[0].Reason != RevokeAdminCorrection || !slices.Equal(events[0].Contents, contents) {
		t.Fatalf("MeasureRevoked：%+v", events)
	}
}

func TestCascadeRevokeOfATakedownCitingARevokedFindingEmitsTheTargets(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	revoked := record[MeasureRevoked](f)
	_, s := f.open(t)
	origin := f.violate(t, s.StoreID, SeverityLow, MeasureWarning, nil)
	contents := takedownContents(2)
	in := f.takedownFrom(s.StoreID, contents)
	in.Violation = &ViolationBasis{CitedFindingIDs: []uuid.UUID{origin.Findings[0].ID}}
	r, err := f.module.CreateMeasure(ctx, in)
	must(t, err)
	_, err = f.module.RevokeMeasure(ctx, RevokeMeasureInput{MeasureID: origin.ID, ExpectedVersion: origin.Version, AdminID: 7, Reason: RevokeAdminCorrection, RevokeFindings: true})
	must(t, err)
	var cascaded []MeasureRevoked
	for _, e := range revoked.all() {
		if e.MeasureID == r.Measure.ID {
			cascaded = append(cascaded, e)
		}
	}
	if len(cascaded) != 1 || cascaded[0].Reason != RevokeFindingRevoked || !slices.Equal(sortedContents(cascaded[0].Contents), sortedContents(contents)) {
		t.Fatalf("連帶撤銷的內容下架要帶對象：%+v", cascaded)
	}
}

// 內容對象在建立後不可修訂：UpdateMeasure 改理由時對象原封不動，沒有別的入口能改它；要改就撤銷再建立。
func TestUpdatingATakedownNeverChangesItsTargets(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	contents := takedownContents(2)
	m := f.takedown(t, s.StoreID, contents)
	statement := "更新後的理由"
	updated, err := f.module.UpdateMeasure(ctx, UpdateMeasureInput{MeasureID: m.ID, AdminID: 7, ExpectedVersion: m.Version, StatementToSeller: &statement})
	must(t, err)
	if updated.StatementToSeller != statement || !slices.Equal(updated.Contents, sortedContents(contents)) {
		t.Fatalf("%+v", updated)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.measure_contents WHERE measure_id = $1", m.ID); n != int64(len(contents)) {
		t.Fatalf("對象列數 %d", n)
	}
}

// 認定的升級規則只有一條累計：社群類別的 MEDIUM 認定照樣計入，同帳號 90 天內第 3 次 MEDIUM 升為 HIGH。
func TestEscalationCountsCommunityCategoryFindings(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	medium := func(category policy.Category) Measure {
		in := f.takedownFrom(s.StoreID, takedownContents(1))
		in.Violation.Finding.Category, in.Violation.Finding.Severity = category, SeverityMedium
		r, err := f.module.CreateMeasure(ctx, in)
		must(t, err)
		return r.Measure
	}
	first := medium(policy.Spam)
	second := f.violate(t, s.StoreID, SeverityMedium, MeasureWarning, nil)
	third := medium(policy.Harassment)
	for _, m := range []Measure{first, second} {
		if m.Findings[0].EscalatedFrom != nil {
			t.Fatalf("太早升級：%+v", m.Findings[0])
		}
	}
	if v := third.Findings[0]; v.Severity != SeverityHigh || v.EscalatedFrom == nil || *v.EscalatedFrom != SeverityMedium {
		t.Fatalf("第 3 次 MEDIUM 要升為 HIGH：%+v", v)
	}
}

func TestFindingStoresModerationReportReferences(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	reports := []uuid.UUID{uuid.NewV7(), uuid.NewV7()}
	in := f.takedownFrom(s.StoreID, takedownContents(1))
	in.Violation.Finding.SourceModerationReportIDs = reports
	r, err := f.module.CreateMeasure(ctx, in)
	must(t, err)
	var got []uuid.UUID
	rows, err := f.db.Conn(ctx).Query(ctx, "SELECT report_id FROM seller.finding_moderation_reports WHERE violation_id = $1 ORDER BY report_id", r.Measure.Findings[0].ID)
	must(t, err)
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		must(t, rows.Scan(&id))
		got = append(got, id)
	}
	if !slices.Equal(got, sortedUUIDs(reports)) {
		t.Fatalf("來源檢舉 %v，預期 %v", got, sortedUUIDs(reports))
	}
	for name, ids := range map[string][]uuid.UUID{
		"重複":   {reports[0], reports[0]},
		"空 ID": {uuid.Nil()},
		"超過上限": func() []uuid.UUID {
			var out []uuid.UUID
			for range 21 {
				out = append(out, uuid.NewV7())
			}
			return out
		}(),
	} {
		bad := f.takedownFrom(s.StoreID, takedownContents(1))
		bad.Violation.Finding.SourceModerationReportIDs = ids
		if _, err := f.module.CreateMeasure(ctx, bad); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：%v", name, err)
		}
	}
}

func sortedUUIDs(ids []uuid.UUID) []uuid.UUID {
	out := slices.Clone(ids)
	slices.SortFunc(out, uuid.UUID.Compare)
	return out
}

// CreateMeasureInTx 加入呼叫方的 transaction：呼叫方回滾時措施、認定、對象與事件的效果都不存在。
func TestCreateMeasureInTxJoinsTheCallersTransaction(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	errRollback := errors.New("呼叫方回滾")
	var inside Measure
	err := f.db.InTx(ctx, func(ctx context.Context) error {
		r, err := f.module.CreateMeasureInTx(ctx, f.takedownFrom(s.StoreID, takedownContents(2)))
		if err != nil {
			return err
		}
		inside = r.Measure
		if n := f.countIn(ctx, t, "SELECT count(*) FROM seller.measures WHERE id = $1", inside.ID); n != 1 {
			t.Fatalf("同一個 transaction 內看得到措施：%d", n)
		}
		return errRollback
	})
	if !errors.Is(err, errRollback) {
		t.Fatalf("%v", err)
	}
	for _, q := range []string{
		"SELECT count(*) FROM seller.measures", "SELECT count(*) FROM seller.measure_contents",
		"SELECT count(*) FROM seller.violations", "SELECT count(*) FROM seller.governance_notes",
	} {
		if n := f.count(t, q); n != 0 {
			t.Errorf("%s = %d：呼叫方回滾後不該留下任何列", q, n)
		}
	}

	var kept Measure
	err = f.db.InTx(ctx, func(ctx context.Context) error {
		r, err := f.module.CreateMeasureInTx(ctx, f.takedownFrom(s.StoreID, takedownContents(1)))
		kept = r.Measure
		return err
	})
	must(t, err)
	if got := f.measureOf(t, kept.ID); got.CreatedByAdminID != 7 || got.Kind != MeasureContentTakedown {
		t.Fatalf("呼叫方 commit 後措施存在，建立者是傳入的管理員：%+v", got)
	}
}

func (f *fixture) countIn(ctx context.Context, t *testing.T, query string, args ...any) int64 {
	t.Helper()
	var n int64
	must(t, f.db.Conn(ctx).QueryRow(ctx, query, args...).Scan(&n))
	return n
}

func TestCreateMeasureInTxRejectsRequestIDAndMissingTransaction(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	requestID := uuid.NewV7()
	in := f.takedownFrom(s.StoreID, takedownContents(1))
	in.RequestID = &requestID
	err := f.db.InTx(ctx, func(ctx context.Context) error {
		_, err := f.module.CreateMeasureInTx(ctx, in)
		return err
	})
	if apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("CreateMeasureInTx 不收 request_id：%v", err)
	}
	if _, err := f.module.CreateMeasureInTx(ctx, f.takedownFrom(s.StoreID, takedownContents(1))); err == nil {
		t.Fatal("沒有呼叫方的 transaction 時要失敗，不能自己另開")
	}
	if n := f.count(t, "SELECT count(*) FROM seller.measures"); n != 0 {
		t.Fatalf("寫入了 %d 筆措施", n)
	}
}

// CreateMeasureInTx 走同一個鎖順序：先鎖目標帳號的 stint，之後的檢查（升級計數）因此串行。
func TestConcurrentCreateMeasureInTxSerialisesEscalation(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	var wg sync.WaitGroup
	results := make(chan Measure, 3)
	for range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			in := f.takedownFrom(s.StoreID, takedownContents(1))
			in.Violation.Finding.Severity = SeverityMedium
			err := f.db.InTx(ctx, func(ctx context.Context) error {
				r, err := f.module.CreateMeasureInTx(ctx, in)
				results <- r.Measure
				return err
			})
			if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	close(results)
	high := 0
	for m := range results {
		if m.Findings[0].Severity == SeverityHigh {
			high++
		}
	}
	if high != 1 {
		t.Fatalf("三筆並行的 MEDIUM 剛好一筆升為 HIGH：%d", high)
	}
}

// 同一個 (管理員, request_id) 用在不同帳號的店家上並行送出：剛好一筆成功，另一筆回 InvalidInput，而且失敗的那筆沒有留下認定或註記（回滾）。
func TestConcurrentCreateMeasureWithTheSameRequestIDOnDifferentAccountsLeavesNothingOfTheLoser(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, a := f.open(t)
	_, b := f.open(t)
	requestID := uuid.NewV7()
	start := make(chan struct{})
	errs := make(chan error, 2)
	for _, storeID := range []int64{a.StoreID, b.StoreID} {
		go func() {
			in := f.warningFrom(storeID)
			in.RequestID = &requestID
			<-start
			_, err := f.module.CreateMeasure(ctx, in)
			errs <- err
		}()
	}
	close(start)
	ok, invalid := 0, 0
	for range 2 {
		switch err := <-errs; {
		case err == nil:
			ok++
		case apperr.KindOf(err) == apperr.KindInvalidInput:
			invalid++
		default:
			t.Errorf("非預期的錯誤：%v", err)
		}
	}
	if ok != 1 || invalid != 1 {
		t.Fatalf("成功 %d、InvalidInput %d，預期各 1", ok, invalid)
	}
	for _, q := range []string{"SELECT count(*) FROM seller.measures", "SELECT count(*) FROM seller.violations", "SELECT count(*) FROM seller.governance_notes"} {
		if n := f.count(t, q); n != 1 {
			t.Errorf("%s = %d，預期 1（失敗的請求要回滾）", q, n)
		}
	}
}

// 列表的 state 篩選與 StateOf 一致：未撤銷的內容下架是 ISSUED，撤銷後是 REVOKED，不會出現在 ACTIVE。
func TestMeasureListStateFilterAgreesWithStateOfForTakedowns(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	warning := f.violate(t, s.StoreID, SeverityLow, MeasureWarning, nil)
	takedown := f.takedown(t, s.StoreID, takedownContents(1))
	idsOf := func(state MeasureState) []uuid.UUID {
		t.Helper()
		account, err := f.module.AccountMeasures(ctx, AccountMeasureListInput{AccountID: accountID, State: &state})
		must(t, err)
		ids := make([]uuid.UUID, 0, len(account.Items))
		for _, m := range account.Items {
			ids = append(ids, m.ID)
			if m.State != state {
				t.Errorf("帳號列表 state=%s 回了 %s", state, m.State)
			}
		}
		admin, err := f.module.AdminMeasures(ctx, AdminMeasureListInput{State: &state, PageSize: 50})
		must(t, err)
		adminIDs := make([]uuid.UUID, 0, len(admin.Items))
		for _, m := range admin.Items {
			adminIDs = append(adminIDs, m.ID)
		}
		if !slices.Equal(sortedUUIDs(ids), sortedUUIDs(adminIDs)) {
			t.Errorf("state=%s：帳號列表 %v 與後台列表 %v 不一致", state, ids, adminIDs)
		}
		return sortedUUIDs(ids)
	}
	if got := idsOf(StateIssued); !slices.Equal(got, sortedUUIDs([]uuid.UUID{warning.ID, takedown.ID})) {
		t.Fatalf("ISSUED=%v", got)
	}
	if got := idsOf(StateActive); len(got) != 0 {
		t.Fatalf("ACTIVE=%v", got)
	}
	_, err := f.module.RevokeMeasure(ctx, RevokeMeasureInput{MeasureID: takedown.ID, ExpectedVersion: takedown.Version, AdminID: 7, Reason: RevokeAdminCorrection})
	must(t, err)
	if got := idsOf(StateIssued); !slices.Equal(got, []uuid.UUID{warning.ID}) {
		t.Fatalf("撤銷後 ISSUED=%v", got)
	}
	if got := idsOf(StateRevoked); !slices.Equal(got, []uuid.UUID{takedown.ID}) {
		t.Fatalf("REVOKED=%v", got)
	}
}
