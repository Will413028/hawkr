//go:build integration

package turn_test

import (
	"context"
	"fmt"
	"io"
	"iter"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"uuid"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	adkmodel "google.golang.org/adk/v2/model"
	"google.golang.org/genai"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/extract"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/resolver"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/run"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/slots"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/turn"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/llm"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

var now = time.Date(2026, 11, 22, 3, 0, 0, 0, time.UTC)

const buyer int64 = 1

func newDB(t *testing.T) *db.DB {
	t.Helper()
	return pg.NewDB(t, testdb.Migrations(migrate.Module{Name: "procurement", Dir: procurement.Migrations()}))
}

type jobs struct{ n atomic.Int32 }

func (j *jobs) EnqueueJob(context.Context, river.JobArgs, *river.InsertOpts) (int64, error) {
	return int64(j.n.Add(1)), nil
}
func (j *jobs) CancelJob(context.Context, int64) (rivertype.JobState, bool, error) {
	return rivertype.JobStateRunning, true, nil
}

// taxonomy 是替身分類樹：小白菜只有一個、蘋果有兩個（跨大類）。
type taxonomy struct{}

func (taxonomy) CategoryTree(context.Context) ([]resolver.Node, error) {
	return []resolver.Node{
		{ID: 1, Name: "葉菜"}, {ID: 2, Name: "水果"}, {ID: 3, Name: "加工品"},
		{ID: 18, ParentID: 1, Name: "小白菜"},
		{ID: 21, ParentID: 2, Name: "蘋果"}, {ID: 22, ParentID: 3, Name: "蘋果"},
	}, nil
}

// scripted 是替身 model.LLM：依序回應，最後一個重複；onCall 在每次呼叫時執行（斷言、模擬取消）。
type scripted struct {
	steps  []func(ctx context.Context) (*adkmodel.LLMResponse, error)
	calls  atomic.Int32
	onCall func(ctx context.Context)
}

func (s *scripted) Name() string { return "fake" }
func (s *scripted) GenerateContent(ctx context.Context, _ *adkmodel.LLMRequest, _ bool) iter.Seq2[*adkmodel.LLMResponse, error] {
	n := int(s.calls.Add(1)) - 1
	return func(yield func(*adkmodel.LLMResponse, error) bool) {
		if s.onCall != nil {
			s.onCall(ctx)
		}
		yield(s.steps[min(n, len(s.steps)-1)](ctx))
	}
}

func say(json string) func(context.Context) (*adkmodel.LLMResponse, error) {
	return func(context.Context) (*adkmodel.LLMResponse, error) {
		return &adkmodel.LLMResponse{Content: genai.NewContentFromText(json, genai.RoleModel)}, nil
	}
}

func fail(err error) func(context.Context) (*adkmodel.LLMResponse, error) {
	return func(context.Context) (*adkmodel.LLMResponse, error) { return nil, err }
}

type fixture struct {
	t            *testing.T
	d            *db.DB
	request      uuid.UUID
	conversation uuid.UUID
	jobs         *jobs
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	f := &fixture{t: t, d: newDB(t), request: uuid.NewV7(), conversation: uuid.NewV7(), jobs: &jobs{}}
	q := store.New(f.d.Conn(ctx))
	b := buyer
	if err := q.InsertRequest(ctx, store.InsertRequestParams{ID: f.request, BuyerID: &b, Now: now}); err != nil {
		t.Fatal(err)
	}
	if err := q.InsertConversation(ctx, store.InsertConversationParams{ID: f.conversation, BuyerID: buyer, RequestID: f.request, Now: now}); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *fixture) ledger() run.Ledger {
	return run.Ledger{DB: f.d, Now: func() time.Time { return now }}
}

func (f *fixture) submit(message string, locale uitext.Locale) uuid.UUID {
	f.t.Helper()
	id, err := run.Submitter{DB: f.d, Ledger: f.ledger(), Jobs: f.jobs}.Submit(context.Background(), run.SubmitInput{
		BuyerID: buyer, RequestID: f.request, ConversationID: &f.conversation, RunType: run.RunTypeTurn, ClientRequestID: uuid.NewV7(),
		Message: message, Locale: locale, QuotaLimit: 5, PeriodStart: now, PeriodEnd: now.AddDate(0, 1, 0),
	})
	if err != nil {
		f.t.Fatal(err)
	}
	return id
}

func (f *fixture) executor(ext turn.SlotExtractor) turn.Executor {
	return turn.Executor{
		DB: f.d, Ledger: f.ledger(), Extractor: ext, Resolver: resolver.New(taxonomy{}, resolver.Synonyms{}, func() time.Time { return now }),
		Now: func() time.Time { return now }, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func llmExtractor(l adkmodel.LLM) turn.SlotExtractor {
	return extract.New(l, "fake", extract.ShapeSchema)
}

// runWith 以 Dispatcher 執行一個 run（含失敗收斂）。
func (f *fixture) runWith(ctx context.Context, id uuid.UUID, e turn.Executor) error {
	f.t.Helper()
	return run.Dispatcher{DB: f.d, Ledger: f.ledger(), Executors: map[string]run.Executor{run.RunTypeTurn: e}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}.
		Run(ctx, run.RunArgs{RunID: id})
}

func (f *fixture) run(id uuid.UUID) store.ProcurementAgentRun {
	f.t.Helper()
	r, err := store.New(f.d.Conn(context.Background())).GetRun(context.Background(), id)
	if err != nil {
		f.t.Fatal(err)
	}
	return r
}

func (f *fixture) count(sql string, args ...any) int {
	f.t.Helper()
	var n int
	if err := f.d.Pool().QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		f.t.Fatal(err)
	}
	return n
}

func (f *fixture) used() int32 {
	u, err := store.New(f.d.Conn(context.Background())).GetUsage(context.Background(), store.GetUsageParams{BuyerID: buyer, PeriodStart: now})
	if err != nil {
		return 0
	}
	return u
}

func (f *fixture) eventTypes(id uuid.UUID) []string {
	f.t.Helper()
	rows, err := f.d.Pool().Query(context.Background(), `SELECT event_type FROM procurement.agent_run_events WHERE run_id = $1 ORDER BY seq`, id)
	if err != nil {
		f.t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			f.t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}

func (f *fixture) noWrites(id uuid.UUID, why string) {
	f.t.Helper()
	if n := f.count(`SELECT count(*) FROM procurement.request_revisions WHERE request_id = $1`, f.request) + f.count(`SELECT count(*) FROM procurement.turns WHERE request_id = $1`, f.request); n != 0 {
		f.t.Errorf("%s：不該有版本或輪次，找到 %d 列", why, n)
	}
	for _, e := range f.eventTypes(id) {
		if e == run.EventTurnResult {
			f.t.Errorf("%s：不該有 turn_result 事件", why)
		}
	}
}

const ok = `{"item":"小白菜","unit":"KG","delivery_city":"臺北市","quantity":20}`

func TestSuccessWritesRevisionTurnEventsAndKeepsUsageConsumed(t *testing.T) { // I1
	ctx := context.Background()
	f := newFixture(t)
	id := f.submit("我要20公斤小白菜送臺北", uitext.ZhTW)
	var inTx atomic.Bool
	l := &scripted{steps: []func(context.Context) (*adkmodel.LLMResponse, error){say(ok)}, onCall: func(ctx context.Context) { // I10
		if _, has := db.TxFrom(ctx); has {
			inTx.Store(true)
		}
	}}
	if err := f.runWith(ctx, id, f.executor(llmExtractor(l))); err != nil {
		t.Fatal(err)
	}
	if inTx.Load() {
		t.Error("LLM 呼叫不得在 transaction 內（單體規則 5）")
	}
	r := f.run(id)
	if r.Status != run.StatusSucceeded {
		t.Fatalf("run = %s %v", r.Status, r.FailureReason)
	}
	if got := strings.Join(f.eventTypes(id), ","); got != "run_accepted,turn_result,finished" {
		t.Errorf("events = %s", got)
	}
	revs, err := store.New(f.d.Conn(ctx)).ListRevisions(ctx, store.ListRevisionsParams{RequestID: f.request, BuyerID: ptr(buyer)})
	if err != nil || len(revs) != 1 || revs[0].Status != "ACTIVE" || revs[0].RevisionNo != 1 {
		t.Fatalf("revisions = %+v %v", revs, err)
	}
	rev := revs[0]
	if rev.ItemResolutionKind == nil || *rev.ItemResolutionKind != "CATEGORY_CONFIRMED" || rev.ItemResolutionCategoryID == nil || *rev.ItemResolutionCategoryID != 18 { // item_resolution_category_id 寫入
		t.Errorf("resolution = %v %v %v", rev.ItemResolutionKind, rev.ItemResolutionLabel, rev.ItemResolutionCategoryID)
	}
	turns, _ := store.New(f.d.Conn(ctx)).ListTurnsByRequest(ctx, store.ListTurnsByRequestParams{RequestID: f.request, BuyerID: buyer})
	if len(turns) != 1 || turns[0].ClientRequestID != r.ClientRequestID || turns[0].ResultRevisionID != rev.ID || turns[0].Source != "LLM_EXTRACT" {
		t.Fatalf("turns = %+v", turns)
	}
	if f.used() != 1 {
		t.Errorf("成功後 usage.used = %d，額度維持已扣", f.used())
	}
	req, _ := store.New(f.d.Conn(ctx)).GetRequestForBuyer(ctx, store.GetRequestForBuyerParams{ID: f.request, BuyerID: ptr(buyer)})
	if req.Status != "REQUIREMENT_READY" {
		t.Errorf("request status = %s", req.Status)
	}
	conv, _ := store.New(f.d.Conn(ctx)).GetConversationForBuyer(ctx, store.GetConversationForBuyerParams{ID: f.conversation, BuyerID: buyer})
	if conv.Title != "小白菜｜臺北市" || conv.LastMessagePreview == "" {
		t.Errorf("conversation = %+v", conv)
	}
	if got := f.count(`SELECT count(*) FROM procurement.agent_runs WHERE id = $1 AND result_revision_id IS NOT NULL`, id); got != 0 {
		t.Error("agent_runs.result_revision_id 不寫（Q9）")
	}
}

func TestSecondTurnSupersedesCarriesProgressAndChainsRevisions(t *testing.T) { // I1 後半、A9-D25(1)
	ctx := context.Background()
	f := newFixture(t)
	first := f.submit("a", uitext.ZhTW)
	if err := f.runWith(ctx, first, f.executor(llmExtractor(&scripted{steps: []func(context.Context) (*adkmodel.LLMResponse, error){say(ok)}}))); err != nil {
		t.Fatal(err)
	}
	second := f.submit("b", uitext.ZhTW)
	if err := f.runWith(ctx, second, f.executor(llmExtractor(&scripted{steps: []func(context.Context) (*adkmodel.LLMResponse, error){say(`{"packaging":"箱"}`)}}))); err != nil {
		t.Fatal(err)
	}
	revs, _ := store.New(f.d.Conn(ctx)).ListRevisions(ctx, store.ListRevisionsParams{RequestID: f.request, BuyerID: ptr(buyer)})
	if len(revs) != 2 || revs[0].Status != "SUPERSEDED" || revs[1].Status != "ACTIVE" || revs[1].ParentRevisionID == nil || *revs[1].ParentRevisionID != revs[0].ID {
		t.Fatalf("revisions = %+v", revs)
	}
	if revs[1].Packaging == nil || *revs[1].Packaging != "箱" || revs[1].Item == nil || *revs[1].Item != "小白菜" || revs[1].ItemResolutionCategoryID == nil {
		t.Errorf("第二版要保留第一版的欄位與解析並加上包裝：%+v", revs[1])
	}
	last, err := store.New(f.d.Conn(ctx)).GetLatestTurnByRequest(ctx, store.GetLatestTurnByRequestParams{RequestID: f.request, BuyerID: buyer})
	if err != nil || last.ResultRevisionID != revs[1].ID {
		t.Fatalf("latest turn = %+v %v", last, err)
	}
	if last.OptionalStreak != 2 {
		t.Errorf("optional_streak = %d：每輪都問了一個選填問題，第二輪接著第一輪的 1 要是 2", last.OptionalStreak)
	}
}

func TestAmbiguousItemInterruptsWithMatchingInterruptID(t *testing.T) { // I2
	ctx := context.Background()
	f := newFixture(t)
	id := f.submit("蘋果", uitext.ZhTW)
	l := &scripted{steps: []func(context.Context) (*adkmodel.LLMResponse, error){say(`{"item":"蘋果"}`)}}
	if err := f.runWith(ctx, id, f.executor(llmExtractor(l))); err != nil {
		t.Fatal(err)
	}
	if r := f.run(id); r.Status != run.StatusWaitingForInput {
		t.Fatalf("run = %s %v", r.Status, r.FailureReason)
	}
	irq, found, err := f.ledger().OpenInterruptForRun(ctx, buyer, id)
	if err != nil || !found {
		t.Fatalf("interrupt = %+v %v %v", irq, found, err)
	}
	var ev string
	if err := f.d.Pool().QueryRow(ctx, `SELECT payload::text FROM procurement.agent_run_events WHERE run_id = $1 AND event_type = 'turn_result'`, id).Scan(&ev); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ev, irq.ID.String()) {
		t.Errorf("turn_result 的 interrupt.interruptId 要等於中斷列 ID %s：%s", irq.ID, ev)
	}
	if !strings.Contains(string(irq.Payload), `"category-21"`) || !strings.Contains(string(irq.Payload), "item-other") {
		t.Errorf("payload 要還原整張卡：%s", irq.Payload)
	}
	revs, _ := store.New(f.d.Conn(ctx)).ListRevisions(ctx, store.ListRevisionsParams{RequestID: f.request, BuyerID: ptr(buyer)})
	if len(revs) != 1 || revs[0].ItemResolutionKind != nil || revs[0].ItemResolutionCategoryID != nil || revs[0].Item == nil {
		t.Errorf("revision 要有品項、沒有解析：%+v", revs)
	}
	if f.used() != 1 {
		t.Errorf("used = %d，等待輸入時額度仍已扣", f.used())
	}
	next := f.submit("改問別的", uitext.ZhTW)
	if r := f.run(id); r.Status != run.StatusCancelled {
		t.Errorf("同需求的新一輪要取消等待中的 run：%s", r.Status)
	}
	_ = next
}

func TestCancelWhileTheModelIsRunningEndsCancelledWithoutWritesOrRefund(t *testing.T) { // I3
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	f := newFixture(t)
	id := f.submit("x", uitext.ZhTW)
	c := run.Canceller{DB: f.d, Ledger: f.ledger(), Jobs: f.jobs}
	l := &scripted{steps: []func(context.Context) (*adkmodel.LLMResponse, error){func(ctx context.Context) (*adkmodel.LLMResponse, error) {
		if _, err := c.Cancel(context.Background(), buyer, id); err != nil {
			return nil, err
		}
		stop() // River 取消 job 會取消它的 ctx
		<-ctx.Done()
		return nil, ctx.Err()
	}}}
	if err := f.runWith(ctx, id, f.executor(llmExtractor(l))); err != nil {
		t.Fatal(err)
	}
	if r := f.run(id); r.Status != run.StatusCancelled || r.FailureReason != nil {
		t.Fatalf("run = %s %v", r.Status, r.FailureReason)
	}
	f.noWrites(id, "取消")
	if f.used() != 1 {
		t.Errorf("買家取消不退額度，used = %d", f.used())
	}
}

type cancelRacingExtractor struct {
	f  *fixture
	id uuid.UUID
}

func (e cancelRacingExtractor) Extract(context.Context, string, slots.Set) (slots.Patch, error) {
	if _, err := e.f.d.Pool().Exec(context.Background(), `UPDATE procurement.agent_runs SET status = 'CANCEL_REQUESTED' WHERE id = $1`, e.id); err != nil {
		return slots.Patch{}, err
	}
	item := "小白菜"
	return slots.Patch{Item: &item}, nil
}

func TestCancelLandingBeforeTheCommitRollsEverythingBack(t *testing.T) { // I4
	f := newFixture(t)
	id := f.submit("x", uitext.ZhTW)
	if err := f.runWith(context.Background(), id, f.executor(cancelRacingExtractor{f: f, id: id})); err != nil {
		t.Fatal(err)
	}
	if r := f.run(id); r.Status != run.StatusCancelled {
		t.Fatalf("run = %s %v", r.Status, r.FailureReason)
	}
	f.noWrites(id, "取消搶在提交前")
	if f.used() != 1 {
		t.Errorf("取消不退額度，used = %d", f.used())
	}
}

func TestModelFailureFailsWithPipelineErrorAndRefundsOnce(t *testing.T) { // I5
	for name, c := range map[string]struct {
		llm  *scripted
		want string
	}{
		"4xx 錯誤":        {&scripted{steps: []func(context.Context) (*adkmodel.LLMResponse, error){fail(genai.APIError{Code: 400})}}, "TURN_PIPELINE_ERROR"},
		"兩次格式錯誤":        {&scripted{steps: []func(context.Context) (*adkmodel.LLMResponse, error){say(`{`)}}, "TURN_PIPELINE_ERROR"},
		"429 重試用盡":      {&scripted{steps: []func(context.Context) (*adkmodel.LLMResponse, error){fail(genai.APIError{Code: 429})}}, "MODEL_UNAVAILABLE"},
		"5xx 重試用盡":      {&scripted{steps: []func(context.Context) (*adkmodel.LLMResponse, error){fail(genai.APIError{Code: 503})}}, "MODEL_UNAVAILABLE"},
		"單次逾時（job 還活著）": {&scripted{steps: []func(context.Context) (*adkmodel.LLMResponse, error){fail(fmt.Errorf("call: %w", context.DeadlineExceeded))}}, "MODEL_UNAVAILABLE"},
	} {
		f := newFixture(t)
		id := f.submit("x", uitext.ZhTW)
		if err := f.runWith(context.Background(), id, f.executor(llmExtractor(c.llm))); err != nil {
			t.Fatalf("%s：%v", name, err)
		}
		r := f.run(id)
		if r.Status != run.StatusFailed || r.FailureReason == nil || *r.FailureReason != c.want {
			t.Errorf("%s：run = %s %v，要 FAILED %s", name, r.Status, r.FailureReason, c.want)
		}
		if f.used() != 0 {
			t.Errorf("%s：usage.used = %d，要退回恰好一次", name, f.used())
		}
		f.noWrites(id, name)
		if got := f.eventTypes(id); got[len(got)-1] != run.EventFinished {
			t.Errorf("%s：最後事件 %v", name, got)
		}
	}
}

func TestDisabledModelFailsWithPipelineErrorAndRefunds(t *testing.T) { // I6
	f := newFixture(t)
	id := f.submit("x", uitext.ZhTW)
	disabled := llm.New(llm.Procurement, llm.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := f.runWith(context.Background(), id, f.executor(llmExtractor(disabled.LLM()))); err != nil {
		t.Fatal(err)
	}
	r := f.run(id)
	if r.Status != run.StatusFailed || r.FailureReason == nil || *r.FailureReason != "TURN_PIPELINE_ERROR" || f.used() != 0 {
		t.Fatalf("run = %s %v used=%d", r.Status, r.FailureReason, f.used())
	}
}

func TestJobContextCancelledByShutdownIsWorkerLostAndRefunds(t *testing.T) { // I7
	ctx, stop := context.WithCancel(context.Background())
	f := newFixture(t)
	id := f.submit("x", uitext.ZhTW)
	l := &scripted{steps: []func(context.Context) (*adkmodel.LLMResponse, error){func(ctx context.Context) (*adkmodel.LLMResponse, error) {
		stop()
		<-ctx.Done()
		return nil, ctx.Err()
	}}}
	if err := f.runWith(ctx, id, f.executor(llmExtractor(l))); err != nil {
		t.Fatal(err)
	}
	r := f.run(id)
	if r.Status != run.StatusFailed || r.FailureReason == nil || *r.FailureReason != "WORKER_LOST" || f.used() != 0 {
		t.Fatalf("run = %s %v used=%d", r.Status, r.FailureReason, f.used())
	}
}

// 中斷寫入失敗（這裡讓同需求已有 PENDING 中斷，唯一索引擋下）要讓 run 失敗退額度，不得退回成功。
func TestInterruptWriteFailureFailsTheRunInsteadOfSucceeding(t *testing.T) { // I8
	ctx := context.Background()
	f := newFixture(t)
	other := uuid.NewV7()
	if _, err := f.d.Pool().Exec(ctx, `INSERT INTO procurement.agent_runs (id, buyer_id, request_id, run_type, status, client_request_id, created_at, updated_at)
		VALUES ($1, $2, $3, 'CANDIDATE_SEARCH', 'WAITING_FOR_INPUT', $4, $5, $5)`, other, buyer, f.request, uuid.NewV7(), now); err != nil {
		t.Fatal(err)
	}
	if _, err := f.d.Pool().Exec(ctx, `INSERT INTO procurement.agent_run_interrupts (id, run_id, request_id, interrupt_kind, status, payload, created_at) VALUES ($1, $2, $3, 'CLARIFICATION', 'PENDING', '{}', $4)`,
		uuid.NewV7(), other, f.request, now); err != nil {
		t.Fatal(err)
	}
	id, err := run.Submitter{DB: f.d, Ledger: f.ledger(), Jobs: f.jobs}.Submit(ctx, run.SubmitInput{
		BuyerID: buyer, RequestID: f.request, ConversationID: &f.conversation, RunType: run.RunTypeTurn, ClientRequestID: uuid.NewV7(),
		Message: "x", Locale: uitext.ZhTW, QuotaLimit: 5, PeriodStart: now, PeriodEnd: now.AddDate(0, 1, 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	// 送出時等待中的舊 run 被取代取消，所以再造一筆 PENDING 中斷擋唯一索引。
	if _, err := f.d.Pool().Exec(ctx, `UPDATE procurement.agent_run_interrupts SET status = 'PENDING', closed_at = NULL WHERE run_id = $1`, other); err != nil {
		t.Fatal(err)
	}
	l := &scripted{steps: []func(context.Context) (*adkmodel.LLMResponse, error){say(`{"item":"蘋果"}`)}}
	if err := f.runWith(ctx, id, f.executor(llmExtractor(l))); err != nil {
		t.Fatal(err)
	}
	r := f.run(id)
	if r.Status != run.StatusFailed || r.FailureReason == nil || *r.FailureReason != "TURN_PIPELINE_ERROR" || f.used() != 0 {
		t.Fatalf("run = %s %v used=%d：中斷寫入失敗要失敗退額度", r.Status, r.FailureReason, f.used())
	}
	f.noWrites(id, "中斷失敗要整筆回滾")
}

func TestExistingTurnForTheSameClientRequestFailsTheRun(t *testing.T) { // I9
	ctx := context.Background()
	f := newFixture(t)
	id := f.submit("x", uitext.ZhTW)
	r := f.run(id)
	// 先造一個同 (buyer, client_request_id) 的輪次：需要一個版本當結果。
	rev := uuid.NewV7()
	if _, err := f.d.Pool().Exec(ctx, `INSERT INTO procurement.request_revisions (id, request_id, revision_no, status, item, created_at) VALUES ($1, $2, 1, 'SUPERSEDED', '舊', $3)`, rev, f.request, now); err != nil {
		t.Fatal(err)
	}
	if _, err := f.d.Pool().Exec(ctx, `INSERT INTO procurement.turns (id, request_id, conversation_id, buyer_id, result_revision_id, source, client_request_id, created_at) VALUES ($1, $2, $3, $4, $5, 'LLM_EXTRACT', $6, $7)`,
		uuid.NewV7(), f.request, f.conversation, buyer, rev, r.ClientRequestID, now); err != nil {
		t.Fatal(err)
	}
	l := &scripted{steps: []func(context.Context) (*adkmodel.LLMResponse, error){say(ok)}}
	if err := f.runWith(ctx, id, f.executor(llmExtractor(l))); err != nil {
		t.Fatal(err)
	}
	got := f.run(id)
	if got.Status != run.StatusFailed || f.used() != 0 {
		t.Fatalf("run = %s %v used=%d", got.Status, got.FailureReason, f.used())
	}
	if n := f.count(`SELECT count(*) FROM procurement.request_revisions WHERE request_id = $1`, f.request); n != 1 {
		t.Errorf("revisions = %d：冪等衝突要連新版本一起回滾", n)
	}
}

func TestAutoTitleUpdatesOnlyWhileAutoAndStaysEmptyWithoutAnItem(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	run1 := f.submit("a", uitext.ZhTW)
	if err := f.runWith(ctx, run1, f.executor(llmExtractor(&scripted{steps: []func(context.Context) (*adkmodel.LLMResponse, error){say(`{"quantity":3}`)}}))); err != nil {
		t.Fatal(err)
	}
	title := func() (string, string) {
		c, _ := store.New(f.d.Conn(ctx)).GetConversationForBuyer(ctx, store.GetConversationForBuyerParams{ID: f.conversation, BuyerID: buyer})
		return c.Title, c.TitleSource
	}
	if got, _ := title(); got != "" {
		t.Fatalf("品項未知，標題要留空：%q", got)
	}
	run2 := f.submit("b", uitext.ZhTW)
	if err := f.runWith(ctx, run2, f.executor(llmExtractor(&scripted{steps: []func(context.Context) (*adkmodel.LLMResponse, error){say(ok)}}))); err != nil {
		t.Fatal(err)
	}
	if got, src := title(); got != "小白菜｜臺北市" || src != "AUTO" {
		t.Fatalf("AUTO 要更新：%q %s", got, src)
	}
	if n, err := store.New(f.d.Conn(ctx)).RenameConversation(ctx, store.RenameConversationParams{Title: "我的標題", Now: now, ID: f.conversation, BuyerID: buyer}); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	run3 := f.submit("c", uitext.ZhTW)
	if err := f.runWith(ctx, run3, f.executor(llmExtractor(&scripted{steps: []func(context.Context) (*adkmodel.LLMResponse, error){say(`{"delivery_city":"高雄"}`)}}))); err != nil {
		t.Fatal(err)
	}
	if got, src := title(); got != "我的標題" || src != "USER" {
		t.Errorf("USER 改過的標題不得被覆寫：%q %s", got, src)
	}
}

func TestMatchingRegressesWhenASearchFieldChangesAndStaysOtherwise(t *testing.T) {
	ctx := context.Background()
	for name, c := range map[string]struct {
		patch string
		want  string
	}{
		"改縣市": {`{"delivery_city":"高雄"}`, "REQUIREMENT_READY"},
		"改包裝": {`{"packaging":"箱"}`, "MATCHING"},
	} {
		f := newFixture(t)
		first := f.submit("a", uitext.ZhTW)
		if err := f.runWith(ctx, first, f.executor(llmExtractor(&scripted{steps: []func(context.Context) (*adkmodel.LLMResponse, error){say(ok)}}))); err != nil {
			t.Fatal(err)
		}
		if _, err := f.d.Pool().Exec(ctx, `UPDATE procurement.requests SET status = 'MATCHING' WHERE id = $1`, f.request); err != nil {
			t.Fatal(err)
		}
		second := f.submit("b", uitext.ZhTW)
		if err := f.runWith(ctx, second, f.executor(llmExtractor(&scripted{steps: []func(context.Context) (*adkmodel.LLMResponse, error){say(c.patch)}}))); err != nil {
			t.Fatal(err)
		}
		req, _ := store.New(f.d.Conn(ctx)).GetRequestForBuyer(ctx, store.GetRequestForBuyerParams{ID: f.request, BuyerID: ptr(buyer)})
		if req.Status != c.want {
			t.Errorf("%s：status = %s，要 %s", name, req.Status, c.want)
		}
	}
}

func TestLocaleIsStoredOnTheRunInputAndRendersTheReply(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	id := f.submit("x", uitext.EN)
	in, err := store.New(f.d.Conn(ctx)).GetRunInput(ctx, store.GetRunInputParams{RunID: id, BuyerID: buyer})
	if err != nil || in.Locale != "EN" {
		t.Fatalf("locale = %q %v", in.Locale, err)
	}
	if err := f.runWith(ctx, id, f.executor(llmExtractor(&scripted{steps: []func(context.Context) (*adkmodel.LLMResponse, error){say(ok)}}))); err != nil {
		t.Fatal(err)
	}
	turns, _ := store.New(f.d.Conn(ctx)).ListTurnsByRequest(ctx, store.ListTurnsByRequestParams{RequestID: f.request, BuyerID: buyer})
	if len(turns) != 1 || turns[0].AssistantReply == "" || hasCJK(turns[0].AssistantReply) {
		t.Errorf("reply 要用送出時的語系（en）渲染：%q", turns[0].AssistantReply)
	}
	_, err = run.Submitter{DB: f.d, Ledger: f.ledger(), Jobs: f.jobs}.Submit(ctx, run.SubmitInput{
		BuyerID: buyer, RequestID: f.request, ConversationID: &f.conversation, RunType: run.RunTypeTurn, ClientRequestID: uuid.NewV7(), Message: "x", Locale: "",
		QuotaLimit: 5, PeriodStart: now, PeriodEnd: now,
	})
	if err == nil {
		t.Error("空語系不得被接受")
	}
}

func hasCJK(s string) bool {
	for _, r := range s {
		if r >= 0x4e00 && r <= 0x9fff {
			return true
		}
	}
	return false
}

func ptr[T any](v T) *T { return &v }
