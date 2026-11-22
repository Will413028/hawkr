//go:build integration

package search_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"uuid"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/candidate"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/resolver"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/run"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/search"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

var now = time.Date(2026, 11, 23, 3, 0, 0, 0, time.UTC)

const buyer int64 = 1

type jobs struct{ n atomic.Int32 }

func (j *jobs) EnqueueJob(context.Context, river.JobArgs, *river.InsertOpts) (int64, error) {
	return int64(j.n.Add(1)), nil
}
func (j *jobs) CancelJob(context.Context, int64) (rivertype.JobState, bool, error) {
	return rivertype.JobStateRunning, true, nil
}

// fakes 同時是搜尋、營業中、封鎖與分類末層的替身；每次外部呼叫都斷言沒有 transaction 開著（單體規則 5）。
type fakes struct {
	t        *testing.T
	d        *db.DB
	pages    map[int]candidate.Page
	pageErr  error
	queries  []candidate.Query
	closed   map[int64]struct{}
	blocked  map[int64]struct{}
	blockErr error
	notLeaf  bool
	// leafErr 是分類樹讀取的失敗；onLeaf 在分類檢查時呼叫。
	leafErr error
	onLeaf  func()
	onPage  func()
	// notBefore 是最近一次分類檢查收到的時間下限。
	notBefore time.Time
}

func (f *fakes) noTx() {
	f.t.Helper()
	var n int
	if err := f.d.Pool().QueryRow(context.Background(), `SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND state LIKE 'idle in transaction%'`).Scan(&n); err != nil {
		f.t.Fatal(err)
	}
	if n != 0 {
		f.t.Errorf("外部呼叫時有 %d 個 transaction 開著", n)
	}
}

func (f *fakes) Page(_ context.Context, q candidate.Query, page int) (candidate.Page, error) {
	f.noTx()
	f.queries = append(f.queries, q)
	if f.onPage != nil {
		f.onPage()
	}
	if f.pageErr != nil {
		return candidate.Page{}, f.pageErr
	}
	pg, ok := f.pages[page]
	if !ok {
		return candidate.Page{Last: true}, nil
	}
	return pg, nil
}

func (f *fakes) OpenStoreIDs(_ context.Context, ids []int64) (map[int64]struct{}, error) {
	f.noTx()
	out := map[int64]struct{}{}
	for _, id := range ids {
		if _, closed := f.closed[id]; !closed {
			out[id] = struct{}{}
		}
	}
	return out, nil
}

func (f *fakes) StoreIDsBlockingBuyer(_ context.Context, _ int64, ids []int64) (map[int64]struct{}, error) {
	f.noTx()
	if f.blockErr != nil {
		return nil, f.blockErr
	}
	out := map[int64]struct{}{}
	for _, id := range ids {
		if _, b := f.blocked[id]; b {
			out[id] = struct{}{}
		}
	}
	return out, nil
}

func (f *fakes) LeafAsOf(_ context.Context, _ int64, notBefore time.Time) (resolver.Match, bool, error) {
	f.noTx()
	f.notBefore = notBefore
	if f.onLeaf != nil {
		f.onLeaf()
	}
	if f.leafErr != nil {
		return resolver.Match{}, false, f.leafErr
	}
	return resolver.Match{}, !f.notLeaf, nil
}

type fixture struct {
	t       *testing.T
	d       *db.DB
	request uuid.UUID
	rev     uuid.UUID
	jobs    *jobs
	fakes   *fakes
}

func ledger(d *db.DB) run.Ledger { return run.Ledger{DB: d, Now: func() time.Time { return now }} }

// newFixture 建一個需求（狀態 status）與它的現行版本；kind 是品項解析的種類。
func newFixture(t *testing.T, status, kind string) *fixture {
	t.Helper()
	ctx := context.Background()
	d := pg.NewDB(t, testdb.Migrations(migrate.Module{Name: "procurement", Dir: procurement.Migrations()}))
	f := &fixture{t: t, d: d, request: uuid.NewV7(), rev: uuid.NewV7(), jobs: &jobs{}, fakes: &fakes{t: t, d: d}}
	q := store.New(d.Conn(ctx))
	b := buyer
	must(t, q.InsertRequest(ctx, store.InsertRequestParams{ID: f.request, BuyerID: &b, Now: now}))
	item, label, unit, city := "小白菜", "小白菜", "KG", "臺北市"
	var cat *int64
	if kind == "CATEGORY_CONFIRMED" {
		c := int64(18)
		cat = &c
	}
	n, err := q.InsertRevision(ctx, store.InsertRevisionParams{
		ID: f.rev, RevisionNo: 1, Item: &item, ItemResolutionKind: &kind, ItemResolutionLabel: &label, ItemResolutionCategoryID: cat,
		Unit: &unit, DeliveryCity: &city, Now: now, RequestID: f.request, BuyerID: &b,
	})
	if err != nil || n != 1 {
		t.Fatalf("InsertRevision n=%d err=%v", n, err)
	}
	if status != "CLARIFYING" {
		if n, err := q.SetRequestStatus(ctx, store.SetRequestStatusParams{Status: status, Now: now, ID: f.request, BuyerID: &b, FromStatus: "CLARIFYING"}); err != nil || n != 1 {
			t.Fatalf("SetRequestStatus n=%d err=%v", n, err)
		}
	}
	return f
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) starter() search.Starter {
	return search.Starter{Submitter: run.Submitter{DB: f.d, Ledger: ledger(f.d), Jobs: f.jobs}}
}

func (f *fixture) start(cid uuid.UUID) (search.Started, error) {
	return f.starter().Start(context.Background(), search.StartInput{BuyerID: buyer, RequestID: f.request, ClientRequestID: cid})
}

func (f *fixture) mustStart() search.Started {
	f.t.Helper()
	s, err := f.start(uuid.NewV7())
	if err != nil {
		f.t.Fatal(err)
	}
	return s
}

func (f *fixture) executor() search.Executor {
	return search.Executor{
		DB: f.d, Ledger: ledger(f.d), Searcher: f.fakes, Open: f.fakes, Blocks: f.fakes, Resolver: f.fakes,
		Now: func() time.Time { return now }, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func (f *fixture) dispatch(id uuid.UUID) error {
	return run.Dispatcher{
		DB: f.d, Ledger: ledger(f.d), Executors: map[string]run.Executor{run.RunTypeCandidateSearch: f.executor()},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}.Run(context.Background(), run.RunArgs{RunID: id})
}

func (f *fixture) run(id uuid.UUID) store.ProcurementAgentRun {
	f.t.Helper()
	r, err := store.New(f.d.Conn(context.Background())).GetRun(context.Background(), id)
	must(f.t, err)
	return r
}

func (f *fixture) status() string {
	f.t.Helper()
	b := buyer
	r, err := store.New(f.d.Conn(context.Background())).GetRequestForBuyer(context.Background(), store.GetRequestForBuyerParams{ID: f.request, BuyerID: &b})
	must(f.t, err)
	return r.Status
}

func (f *fixture) count(sql string, args ...any) int {
	f.t.Helper()
	var n int
	must(f.t, f.d.Pool().QueryRow(context.Background(), sql, args...).Scan(&n))
	return n
}

func (f *fixture) candidates(set uuid.UUID) []store.ProcurementCandidate {
	f.t.Helper()
	cs, err := store.New(f.d.Conn(context.Background())).ListCandidates(context.Background(), store.ListCandidatesParams{CandidateSetID: set, BuyerID: buyer})
	must(f.t, err)
	return cs
}

func (f *fixture) eventTypes(id uuid.UUID) []string {
	f.t.Helper()
	rows, err := f.d.Pool().Query(context.Background(), `SELECT event_type FROM procurement.agent_run_events WHERE run_id = $1 ORDER BY seq`, id)
	must(f.t, err)
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		must(f.t, rows.Scan(&s))
		out = append(out, s)
	}
	return out
}

func hits(stores ...int64) candidate.Page {
	var h []candidate.Hit
	for _, s := range stores {
		h = append(h, candidate.Hit{ProductID: s * 100, StoreID: s})
	}
	return candidate.Page{Hits: h, Last: true}
}

func reasonOf(err error) (apperr.Reason, apperr.Kind) {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return ae.Reason, ae.Kind()
	}
	return apperr.Reason{}, apperr.KindOf(err)
}

func TestSuccessWritesRankedCandidatesMovesToMatchingAndSucceeds(t *testing.T) {
	f := newFixture(t, "REQUIREMENT_READY", "CATEGORY_CONFIRMED")
	f.fakes.pages = map[int]candidate.Page{1: hits(30, 10, 20)}
	f.fakes.closed = map[int64]struct{}{10: {}}
	f.fakes.blocked = map[int64]struct{}{20: {}}
	s := f.mustStart()
	if err := f.dispatch(s.RunID); err != nil {
		t.Fatal(err)
	}
	r := f.run(s.RunID)
	if r.Status != run.StatusSucceeded || r.FailureReason != nil {
		t.Fatalf("run = %s %v", r.Status, r.FailureReason)
	}
	cs := f.candidates(s.CandidateSetID)
	if len(cs) != 1 || cs[0].StoreID != 30 || cs[0].SourceID != "3000" || cs[0].Rank != 0 || cs[0].Selected {
		t.Fatalf("candidates = %+v，要只剩營業中而且沒封鎖的店", cs)
	}
	if got := f.status(); got != "MATCHING" {
		t.Errorf("status = %s", got)
	}
	if got := f.eventTypes(s.RunID); len(got) != 2 || got[0] != run.EventRunAccepted || got[1] != run.EventFinished {
		t.Errorf("events = %v，搜尋 run 只有 run_accepted 與 finished", got)
	}
	if len(f.fakes.queries) != 1 || f.fakes.queries[0] != (candidate.Query{CategoryID: 18}) {
		t.Errorf("queries = %+v，確認過的分類只帶分類 ID", f.fakes.queries)
	}
}

func TestResearchFromMatchingStaysMatchingAndZeroResultsSucceed(t *testing.T) {
	f := newFixture(t, "MATCHING", "CATEGORY_CONFIRMED")
	s := f.mustStart()
	if got := f.status(); got != "MATCHING" {
		t.Fatalf("Start 不改需求狀態：%s", got)
	}
	if err := f.dispatch(s.RunID); err != nil {
		t.Fatal(err)
	}
	if r := f.run(s.RunID); r.Status != run.StatusSucceeded {
		t.Fatalf("零結果也是成功：%s", r.Status)
	}
	if n := len(f.candidates(s.CandidateSetID)); n != 0 {
		t.Errorf("candidates = %d", n)
	}
	if got := f.status(); got != "MATCHING" {
		t.Errorf("status = %s", got)
	}
}

func TestBuyerDescribedSearchesByKeyword(t *testing.T) {
	f := newFixture(t, "REQUIREMENT_READY", "BUYER_DESCRIBED")
	f.fakes.pages = map[int]candidate.Page{1: hits(1)}
	s := f.mustStart()
	if err := f.dispatch(s.RunID); err != nil {
		t.Fatal(err)
	}
	if len(f.fakes.queries) != 1 || f.fakes.queries[0] != (candidate.Query{Keyword: "小白菜"}) {
		t.Fatalf("queries = %+v", f.fakes.queries)
	}
}

func TestSearchFailureFailsTheRunAndLeavesTheRequestAlone(t *testing.T) {
	for name, set := range map[string]func(*fakes){
		"search":  func(f *fakes) { f.pageErr = errors.New("offline") },
		"filters": func(f *fakes) { f.pages = map[int]candidate.Page{1: hits(1)}; f.blockErr = errors.New("db down") },
	} {
		f := newFixture(t, "REQUIREMENT_READY", "CATEGORY_CONFIRMED")
		set(f.fakes)
		s := f.mustStart()
		if err := f.dispatch(s.RunID); err != nil {
			t.Fatal(err)
		}
		r := f.run(s.RunID)
		if r.Status != run.StatusFailed || r.FailureReason == nil || *r.FailureReason != run.FailureSearchFailed {
			t.Errorf("%s：run = %s %v", name, r.Status, r.FailureReason)
		}
		if n := len(f.candidates(s.CandidateSetID)); n != 0 || f.status() != "REQUIREMENT_READY" {
			t.Errorf("%s：candidates=%d status=%s", name, n, f.status())
		}
	}
}

func TestStaleRevisionFailsTheRunWithoutCandidatesOrStatusChange(t *testing.T) {
	f := newFixture(t, "REQUIREMENT_READY", "CATEGORY_CONFIRMED")
	f.fakes.pages = map[int]candidate.Page{1: hits(1, 2)}
	s := f.mustStart()
	f.fakes.onPage = func() {
		ctx := context.Background()
		q := store.New(f.d.Conn(ctx))
		b := buyer
		_, err := q.SupersedeActiveRevision(ctx, store.SupersedeActiveRevisionParams{RequestID: f.request, BuyerID: &b})
		must(t, err)
		item := "小白菜"
		n, err := q.InsertRevision(ctx, store.InsertRevisionParams{ID: uuid.NewV7(), RevisionNo: 2, ParentRevisionID: &f.rev, Item: &item, Now: now, RequestID: f.request, BuyerID: &b})
		if err != nil || n != 1 {
			t.Fatalf("InsertRevision n=%d err=%v", n, err)
		}
	}
	if err := f.dispatch(s.RunID); err != nil {
		t.Fatal(err)
	}
	r := f.run(s.RunID)
	if r.Status != run.StatusFailed || r.FailureReason == nil || *r.FailureReason != run.FailureSearchFailed {
		t.Fatalf("run = %s %v", r.Status, r.FailureReason)
	}
	if n := len(f.candidates(s.CandidateSetID)); n != 0 {
		t.Errorf("candidates = %d", n)
	}
	if got := f.status(); got != "REQUIREMENT_READY" {
		t.Errorf("status = %s，版本守衛失敗時需求狀態不動", got)
	}
}

func TestCommitWaitsForAnUncommittedRevisionReplacementThenFailsWithoutCandidates(t *testing.T) {
	f := newFixture(t, "REQUIREMENT_READY", "CATEGORY_CONFIRMED")
	f.fakes.pages = map[int]candidate.Page{1: hits(1, 2)}
	s := f.mustStart()
	held, release, replaced := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	f.fakes.onPage = func() {
		go func() {
			replaced <- f.d.InTx(context.Background(), func(ctx context.Context) error {
				q := store.New(f.d.Conn(ctx))
				b := buyer
				if _, err := q.SupersedeActiveRevision(ctx, store.SupersedeActiveRevisionParams{RequestID: f.request, BuyerID: &b}); err != nil {
					return err
				}
				item := "小白菜"
				if _, err := q.InsertRevision(ctx, store.InsertRevisionParams{ID: uuid.NewV7(), RevisionNo: 2, ParentRevisionID: &f.rev, Item: &item, Now: now, RequestID: f.request, BuyerID: &b}); err != nil {
					return err
				}
				close(held)
				<-release
				return nil
			})
		}()
		<-held
	}
	dispatched := make(chan error, 1)
	go func() { dispatched <- f.dispatch(s.RunID) }()
	deadline := time.Now().Add(10 * time.Second)
	for f.count(`SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock' AND query LIKE '%FOR UPDATE OF v%'`) == 0 {
		select {
		case err := <-dispatched:
			t.Fatalf("搜尋提交沒有等版本列的鎖就結束了：%v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("逾時：搜尋提交沒有卡在版本列的鎖上")
		}
		time.Sleep(5 * time.Millisecond)
	}
	close(release)
	if err := <-replaced; err != nil {
		t.Fatal(err)
	}
	if err := <-dispatched; err != nil {
		t.Fatal(err)
	}
	r := f.run(s.RunID)
	if r.Status != run.StatusFailed || r.FailureReason == nil || *r.FailureReason != run.FailureSearchFailed {
		t.Fatalf("run = %s %v", r.Status, r.FailureReason)
	}
	if n := len(f.candidates(s.CandidateSetID)); n != 0 {
		t.Errorf("candidates = %d，對方提交後版本已變，不得寫候選", n)
	}
	if got := f.status(); got != "REQUIREMENT_READY" {
		t.Errorf("status = %s", got)
	}
}

func TestCancelRequestedBeforeCommitRollsEverythingBack(t *testing.T) {
	f := newFixture(t, "REQUIREMENT_READY", "CATEGORY_CONFIRMED")
	f.fakes.pages = map[int]candidate.Page{1: hits(1)}
	s := f.mustStart()
	f.fakes.onPage = func() {
		if _, err := ledger(f.d).Transition(context.Background(), s.RunID, run.StatusCancelRequested, ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.dispatch(s.RunID); err != nil {
		t.Fatal(err)
	}
	if r := f.run(s.RunID); r.Status != run.StatusCancelled {
		t.Fatalf("run = %s", r.Status)
	}
	if n := len(f.candidates(s.CandidateSetID)); n != 0 || f.status() != "REQUIREMENT_READY" {
		t.Errorf("取消搶先要整筆回滾：candidates=%d status=%s", n, f.status())
	}
}

func TestCategoryNoLongerALeafClearsTheResolutionAndReopensClarification(t *testing.T) {
	f := newFixture(t, "MATCHING", "CATEGORY_CONFIRMED")
	f.fakes.notLeaf = true
	f.fakes.pages = map[int]candidate.Page{1: hits(1)}
	s := f.mustStart()
	wantNotBefore := f.activeRevision().CreatedAt
	if err := f.dispatch(s.RunID); err != nil {
		t.Fatal(err)
	}
	if !f.fakes.notBefore.Equal(wantNotBefore) {
		t.Errorf("分類檢查的時間下限 = %v，要是候選集所指版本的建立時間 %v", f.fakes.notBefore, wantNotBefore)
	}
	r := f.run(s.RunID)
	if r.Status != run.StatusFailed || r.FailureReason == nil || *r.FailureReason != run.FailureItemCategoryUnavailable {
		t.Fatalf("run = %s %v", r.Status, r.FailureReason)
	}
	if got := f.status(); got != "CLARIFYING" {
		t.Errorf("status = %s", got)
	}
	b := buyer
	active, err := store.New(f.d.Conn(context.Background())).GetActiveRevision(context.Background(), store.GetActiveRevisionParams{RequestID: f.request, BuyerID: &b})
	must(t, err)
	if active.ID == f.rev || active.ParentRevisionID == nil || *active.ParentRevisionID != f.rev {
		t.Fatalf("要寫一個以舊版本為 parent 的新版本：%+v", active)
	}
	if active.ItemResolutionKind != nil || active.ItemResolutionLabel != nil || active.ItemResolutionCategoryID != nil {
		t.Errorf("新版本的品項解析要清掉：%+v", active)
	}
	if active.Item == nil || *active.Item != "小白菜" || active.Unit == nil || *active.Unit != "KG" {
		t.Errorf("品項文字與其他欄位要保留：%+v", active)
	}
	if len(f.fakes.queries) != 0 || len(f.candidates(s.CandidateSetID)) != 0 {
		t.Error("分類不再是末層就不搜尋、不寫候選")
	}
	if got := f.eventTypes(s.RunID); len(got) != 2 || got[1] != run.EventFinished {
		t.Errorf("events = %v", got)
	}
}

func (f *fixture) activeRevision() store.ProcurementRequestRevision {
	f.t.Helper()
	b := buyer
	a, err := store.New(f.d.Conn(context.Background())).GetActiveRevision(context.Background(), store.GetActiveRevisionParams{RequestID: f.request, BuyerID: &b})
	must(f.t, err)
	return a
}

func TestUnreadableTaxonomyFailsTheRunWithoutClearingTheResolution(t *testing.T) {
	for name, set := range map[string]func(*fakes){
		"tree read fails": func(f *fakes) { f.leafErr = errors.New("catalog down") },
	} {
		f := newFixture(t, "MATCHING", "CATEGORY_CONFIRMED")
		set(f.fakes)
		s := f.mustStart()
		if err := f.dispatch(s.RunID); err != nil {
			t.Fatal(err)
		}
		r := f.run(s.RunID)
		if r.Status != run.StatusFailed || r.FailureReason == nil || *r.FailureReason != run.FailureSearchFailed {
			t.Errorf("%s：run = %s %v", name, r.Status, r.FailureReason)
		}
		if a := f.activeRevision(); a.ID != f.rev || a.ItemResolutionKind == nil || f.status() != "MATCHING" {
			t.Errorf("%s：拿不到分類樹不得清解析或改狀態：%+v %s", name, a, f.status())
		}
	}
}

func TestCancelRequestedBeforeTheCategoryInvalidationCommitRollsEverythingBack(t *testing.T) {
	f := newFixture(t, "MATCHING", "CATEGORY_CONFIRMED")
	f.fakes.notLeaf = true
	s := f.mustStart()
	f.fakes.onLeaf = func() {
		if _, err := ledger(f.d).Transition(context.Background(), s.RunID, run.StatusCancelRequested, ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.dispatch(s.RunID); err != nil {
		t.Fatal(err)
	}
	if r := f.run(s.RunID); r.Status != run.StatusCancelled {
		t.Fatalf("run = %s %v", r.Status, r.FailureReason)
	}
	if a := f.activeRevision(); a.ID != f.rev || a.ItemResolutionKind == nil || f.status() != "MATCHING" {
		t.Errorf("取消搶先要整筆回滾：%+v %s", a, f.status())
	}
}

func TestStartPreconditions(t *testing.T) {
	f := newFixture(t, "CLARIFYING", "CATEGORY_CONFIRMED")
	if _, err := f.start(uuid.NewV7()); err == nil {
		t.Fatal("CLARIFYING 的需求不能搜尋")
	} else if reason, kind := reasonOf(err); kind != apperr.KindFailedPrecondition || reason != apperr.ReasonRequirementNotReady {
		t.Fatalf("err = %v", err)
	}
	if n := f.count(`SELECT count(*) FROM procurement.candidate_sets`) + f.count(`SELECT count(*) FROM procurement.agent_runs`); n != 0 {
		t.Errorf("被擋下的 Start 不留列：%d", n)
	}
	_, err := f.starter().Start(context.Background(), search.StartInput{BuyerID: 999, RequestID: f.request, ClientRequestID: uuid.NewV7()})
	if apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("他人的需求要 NOT_FOUND：%v", err)
	}
}

func TestStartRejectsASecondRunWhileOneIsInProgress(t *testing.T) {
	f := newFixture(t, "REQUIREMENT_READY", "CATEGORY_CONFIRMED")
	f.mustStart()
	_, err := f.start(uuid.NewV7())
	if reason, kind := reasonOf(err); kind != apperr.KindFailedPrecondition || reason != apperr.ReasonProcurementRunInProgress {
		t.Fatalf("err = %v", err)
	}
	if n := f.count(`SELECT count(*) FROM procurement.candidate_sets`); n != 1 {
		t.Errorf("被擋下的 Start 不得多建候選集：%d", n)
	}
}

func TestStartDoesNotChangeStatusOrChargeQuota(t *testing.T) {
	f := newFixture(t, "REQUIREMENT_READY", "CATEGORY_CONFIRMED")
	s := f.mustStart()
	if got := f.status(); got != "REQUIREMENT_READY" {
		t.Errorf("status = %s", got)
	}
	if n := f.count(`SELECT count(*) FROM procurement.usage`); n != 0 {
		t.Errorf("usage 列 = %d，CANDIDATE_SEARCH 不扣額度", n)
	}
	r := f.run(s.RunID)
	if r.QuotaPeriodStart != nil || r.CandidateSetID == nil || *r.CandidateSetID != s.CandidateSetID || r.Status != run.StatusQueued || s.Status != run.StatusQueued {
		t.Errorf("run = %+v started = %+v", r, s)
	}
	if f.jobs.n.Load() != 1 {
		t.Errorf("jobs = %d", f.jobs.n.Load())
	}
	if n := f.count(`SELECT count(*) FROM procurement.agent_run_inputs WHERE run_id = $1`, s.RunID); n != 0 {
		t.Errorf("候選搜尋沒有買家訊息，不寫 agent_run_inputs：%d", n)
	}
}

func TestStartReplayReturnsTheSameRunAndCandidateSet(t *testing.T) {
	f := newFixture(t, "REQUIREMENT_READY", "CATEGORY_CONFIRMED")
	cid := uuid.NewV7()
	first, err := f.start(cid)
	must(t, err)
	again, err := f.start(cid)
	must(t, err)
	if first.RunID != again.RunID || first.CandidateSetID != again.CandidateSetID {
		t.Fatalf("重送要回同一個 run 與候選集：%+v %+v", first, again)
	}
	if n := f.count(`SELECT count(*) FROM procurement.candidate_sets`); n != 1 {
		t.Errorf("candidate_sets = %d", n)
	}
	if n := f.count(`SELECT count(*) FROM procurement.agent_runs`); n != 1 || f.jobs.n.Load() != 1 {
		t.Errorf("agent_runs = %d jobs = %d", n, f.jobs.n.Load())
	}
	other := newFixture(t, "REQUIREMENT_READY", "CATEGORY_CONFIRMED")
	_, err = other.start(cid)
	must(t, err)
	if _, err := f.starter().Start(context.Background(), search.StartInput{BuyerID: buyer, RequestID: other.request, ClientRequestID: cid}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Errorf("同一個 client_request_id 不同需求要 INVALID_ARGUMENT：%v", err)
	}
}
