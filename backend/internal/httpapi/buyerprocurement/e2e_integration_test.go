//go:build integration

package buyerprocurement

import (
	"context"
	"fmt"
	"io"
	"iter"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	adkmodel "google.golang.org/adk/v2/model"
	"google.golang.org/genai"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/procurement/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/procurement/v1alpha1/procurementv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/llm"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

// stubLLM 是替身模型：reply 決定每次呼叫的回應文字；回 block 為真時阻塞到 ctx 結束（模擬進行中的 run）。
type stubLLM struct {
	reply   func(call int) string
	block   bool
	started chan struct{}
	calls   atomic.Int32
}

func (*stubLLM) Name() string { return "stub" }

func (l *stubLLM) GenerateContent(ctx context.Context, _ *adkmodel.LLMRequest, _ bool) iter.Seq2[*adkmodel.LLMResponse, error] {
	n := int(l.calls.Add(1))
	return func(yield func(*adkmodel.LLMResponse, error) bool) {
		if l.block {
			select {
			case l.started <- struct{}{}:
			default:
			}
			<-ctx.Done()
			yield(nil, ctx.Err())
			return
		}
		yield(&adkmodel.LLMResponse{Content: genai.NewContentFromText(l.reply(n), genai.RoleModel), FinishReason: genai.FinishReasonStop}, nil)
	}
}

type taxonomy struct{}

// catID 是測試用的確定性分類 ID：同一個數字永遠得到同一個 UUID，且不是零值。
func catID(n int) uuid.UUID {
	return uuid.MustParse(fmt.Sprintf("00000000-0000-7000-8000-%012d", n))
}

func parentOf(n int) *uuid.UUID {
	id := catID(n)
	return &id
}

func (taxonomy) CategoryTree(context.Context) ([]procurement.TaxonomyNode, error) {
	return []procurement.TaxonomyNode{
		{ID: catID(1), Name: "葉菜"}, {ID: catID(2), Name: "水果"}, {ID: catID(3), Name: "加工品"},
		{ID: catID(18), ParentID: parentOf(1), Name: "小白菜"},
		{ID: catID(21), ParentID: parentOf(2), Name: "蘋果"}, {ID: catID(22), ParentID: parentOf(3), Name: "蘋果"},
	}, nil
}

type products struct{}

func (products) Page(context.Context, procurement.SearchQuery, procurement.SearchCursor) (procurement.SearchPage, error) {
	return procurement.SearchPage{Hits: []procurement.SearchHit{{ProductID: catID(501), StoreID: 900}}}, nil
}

type stores struct{}

func (stores) ExposableStores(_ context.Context, ids []int64) ([]seller.ExposableStore, error) {
	out := make([]seller.ExposableStore, 0, len(ids))
	for _, id := range ids {
		out = append(out, seller.ExposableStore{StoreID: id})
	}
	return out, nil
}

func (stores) StoresBlockingBuyer(context.Context, int64, []int64) (map[int64]bool, error) {
	return map[int64]bool{}, nil
}

type entitlements struct{}

func (entitlements) Entitlement(context.Context, int64) (buyer.Entitlement, error) {
	start := time.Now().UTC().Truncate(24 * time.Hour)
	return buyer.Entitlement{MonthlyAnswerLimit: 10, PeriodStart: start, PeriodEnd: start.AddDate(0, 1, 0)}, nil
}

// lateQueue 讓模組先建好、River client 之後才指定（worker 要先登記模組的 worker 才能建）。
type lateQueue struct{ q procurement.JobQueue }

func (l *lateQueue) EnqueueJob(ctx context.Context, a river.JobArgs, o *river.InsertOpts) (int64, error) {
	return l.q.EnqueueJob(ctx, a, o)
}

func (l *lateQueue) CancelJob(ctx context.Context, id int64) (rivertype.JobState, bool, error) {
	return l.q.CancelJob(ctx, id)
}

// newClient 組出真正的模組（真的 Postgres、River worker、LISTEN hub、替身 LLM），經 Connect 測試 server 呼叫 handler；不經單體的掛載。
func newClient(t *testing.T, l *stubLLM) procurementv1alpha1connect.BuyerProcurementAppServiceClient {
	t.Helper()
	d := pg.NewDB(t, testdb.Migrations(migrate.Module{Name: "procurement", Dir: procurement.Migrations()}), testdb.Jobs())
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	lq := &lateQueue{}
	m := procurement.New(procurement.Deps{
		DB: d, Now: time.Now, Logger: log, Jobs: lq, LLM: llm.NewModel(l, "stub", llm.Gemini), Taxonomy: taxonomy{},
		Search: products{}, Sellers: stores{}, Entitlements: entitlements{}, Events: events.NewBus(),
	})
	workers := river.NewWorkers()
	m.AddWorkers(workers)
	jc, err := jobs.NewWorker(d, workers, map[string]int{river.QueueDefault: 5}, nil)
	if err != nil {
		t.Fatal(err)
	}
	lq.q = jc
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{}, 2)
	go func() { _ = jc.Run(ctx); done <- struct{}{} }()
	go func() { _ = m.RunHub(ctx); done <- struct{}{} }()
	t.Cleanup(func() {
		cancel()
		<-done
		<-done
	})
	return setup(t, m)
}

// watch 訂閱到串流結束，回全部事件。
func watch(t *testing.T, c procurementv1alpha1connect.BuyerProcurementAppServiceClient, runID string, account int64) []*pb.WatchRunResponse {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	st, err := c.WatchRun(ctx, as(connect.NewRequest(&pb.WatchRunRequest{RunId: runID}), account))
	if err != nil {
		t.Fatal(err)
	}
	var out []*pb.WatchRunResponse
	for st.Receive() {
		out = append(out, st.Msg())
	}
	if err := st.Err(); err != nil {
		t.Fatalf("WatchRun：%v（已收 %d 筆）", err, len(out))
	}
	return out
}

func finished(t *testing.T, evs []*pb.WatchRunResponse) *pb.RunFinished {
	t.Helper()
	if len(evs) == 0 || evs[len(evs)-1].GetFinished() == nil {
		t.Fatalf("串流沒有以 finished 結尾：%v", evs)
	}
	for _, e := range evs[:len(evs)-1] {
		if e.GetFinished() != nil {
			t.Fatalf("finished 之前不得有 finished：%v", evs)
		}
	}
	return evs[len(evs)-1].GetFinished()
}

func submit(t *testing.T, c procurementv1alpha1connect.BuyerProcurementAppServiceClient, requestID, clientID string) *pb.SubmitTurnResponse {
	t.Helper()
	res, err := c.SubmitTurn(t.Context(), as(connect.NewRequest(&pb.SubmitTurnRequest{RequestId: requestID, Message: "我要20公斤小白菜送臺北", ClientRequestId: clientID}), accountID))
	if err != nil {
		t.Fatal(err)
	}
	return res.Msg
}

const (
	ready    = `{"item":"小白菜","unit":"KG","delivery_city":"臺北市","quantity":20}`
	ambigous = `{"item":"蘋果"}`
	cid1     = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	cid2     = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	cid3     = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
)

func snapshot(t *testing.T, c procurementv1alpha1connect.BuyerProcurementAppServiceClient, conv string) *pb.GetRuntimeSnapshotResponse {
	t.Helper()
	res, err := c.GetRuntimeSnapshot(t.Context(), as(connect.NewRequest(&pb.GetRuntimeSnapshotRequest{ConversationId: conv}), accountID))
	if err != nil {
		t.Fatal(err)
	}
	return res.Msg
}

func TestSubmitTurnThenWatchRunFinishesAndTheSnapshotReflectsIt(t *testing.T) {
	c := newClient(t, &stubLLM{reply: func(int) string { return ready }})
	sub := submit(t, c, "", cid1)
	if sub.GetRequestId() == "" || sub.GetConversationId() == "" || sub.GetRunId() == "" {
		t.Fatalf("SubmitTurn = %v", sub)
	}
	evs := watch(t, c, sub.GetRunId(), accountID)
	if f := finished(t, evs); f.GetStatus() != pb.RunStatus_RUN_STATUS_SUCCEEDED || f.GetFailureReason() != pb.RunFailureReason_RUN_FAILURE_REASON_UNSPECIFIED {
		t.Fatalf("finished = %v", f)
	}
	if evs[0].GetRunAccepted().GetRunId() != sub.GetRunId() {
		t.Errorf("第一筆要是 run_accepted：%v", evs[0])
	}
	var result *pb.TurnResult
	for _, e := range evs {
		if e.GetTurnResult() != nil {
			result = e.GetTurnResult()
		}
		if e.GetCursor() == "" || e.GetEventTime() == nil {
			t.Errorf("事件缺 cursor 或時間：%v", e)
		}
	}
	if result == nil || result.GetSlots().GetItem() != "小白菜" || result.GetRunId() != sub.GetRunId() {
		t.Fatalf("turn_result = %v", result)
	}
	// 從最後一個 cursor 接續沒有新事件；重送回同一個 run。
	rest := watchFrom(t, c, sub.GetRunId(), evs[len(evs)-1].GetCursor())
	if len(rest) != 0 {
		t.Errorf("接續 = %v", rest)
	}
	again := submit(t, c, "", cid1)
	if again.GetRunId() != sub.GetRunId() || again.GetRequestId() != sub.GetRequestId() || again.GetConversationId() != sub.GetConversationId() {
		t.Errorf("重送 = %v，要回放 %v", again, sub)
	}

	snap := snapshot(t, c, sub.GetConversationId())
	if snap.GetStatus() != pb.RequestStatus_REQUEST_STATUS_REQUIREMENT_READY || snap.GetRequestId() != sub.GetRequestId() || snap.GetActiveRunId() != "" ||
		len(snap.GetRuns()) != 1 || snap.GetRuns()[0].GetStatus() != pb.RunStatus_RUN_STATUS_SUCCEEDED || !snap.GetRuns()[0].GetTerminal() ||
		snap.GetRuns()[0].GetLastCursor() != evs[len(evs)-1].GetCursor() || len(snap.GetMessages()) < 2 {
		t.Fatalf("快照 = %v", snap)
	}
	if _, err := c.GetRuntimeSnapshot(t.Context(), as(connect.NewRequest(&pb.GetRuntimeSnapshotRequest{ConversationId: sub.GetConversationId()}), accountID+1)); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("別的買家讀快照 = %v", err)
	}

	// StartCandidateSearch：需求已齊備，搜尋成功後轉 MATCHING。
	start, err := c.StartCandidateSearch(t.Context(), as(connect.NewRequest(&pb.StartCandidateSearchRequest{RequestId: sub.GetRequestId(), ClientRequestId: cid2}), accountID))
	if err != nil || start.Msg.GetRunId() == "" || start.Msg.GetCandidateSetId() == "" || start.Msg.GetRequestId() != sub.GetRequestId() {
		t.Fatalf("StartCandidateSearch = %v, %v", start, err)
	}
	sevs := watch(t, c, start.Msg.GetRunId(), accountID)
	if f := finished(t, sevs); f.GetStatus() != pb.RunStatus_RUN_STATUS_SUCCEEDED {
		t.Fatalf("搜尋 finished = %v", f)
	}
	if snap := snapshot(t, c, sub.GetConversationId()); snap.GetStatus() != pb.RequestStatus_REQUEST_STATUS_MATCHING || snap.GetRuns()[0].GetRunType() != pb.RunType_RUN_TYPE_CANDIDATE_SEARCH {
		t.Fatalf("搜尋之後的快照 = %v", snap)
	}
	if _, err := watchErr(t, c, sub.GetRunId(), accountID+1); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("別的買家訂閱 = %v", err)
	}
}

func watchFrom(t *testing.T, c procurementv1alpha1connect.BuyerProcurementAppServiceClient, runID, cursor string) []*pb.WatchRunResponse {
	t.Helper()
	st, err := c.WatchRun(t.Context(), as(connect.NewRequest(&pb.WatchRunRequest{RunId: runID, Cursor: cursor}), accountID))
	if err != nil {
		t.Fatal(err)
	}
	var out []*pb.WatchRunResponse
	for st.Receive() {
		out = append(out, st.Msg())
	}
	if err := st.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func watchErr(t *testing.T, c procurementv1alpha1connect.BuyerProcurementAppServiceClient, runID string, account int64) ([]*pb.WatchRunResponse, error) {
	t.Helper()
	st, err := c.WatchRun(t.Context(), as(connect.NewRequest(&pb.WatchRunRequest{RunId: runID}), account))
	if err != nil {
		return nil, err
	}
	var out []*pb.WatchRunResponse
	for st.Receive() {
		out = append(out, st.Msg())
	}
	return out, st.Err()
}

// watchUntilTurnResult 訂閱到 turn_result 就收線：停在 WAITING_FOR_INPUT 的 run 不會有 finished，串流會開到視窗結束。
func watchUntilTurnResult(t *testing.T, c procurementv1alpha1connect.BuyerProcurementAppServiceClient, runID string, account int64) *pb.TurnResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	st, err := c.WatchRun(ctx, as(connect.NewRequest(&pb.WatchRunRequest{RunId: runID}), account))
	if err != nil {
		t.Fatal(err)
	}
	for st.Receive() {
		if r := st.Msg().GetTurnResult(); r != nil {
			return r
		}
		if st.Msg().GetFinished() != nil {
			t.Fatalf("要停在中斷，卻收到 finished：%v", st.Msg())
		}
	}
	t.Fatalf("串流結束前沒有 turn_result：%v", st.Err())
	return nil
}

func TestResumeRunAnswersTheClarificationThenWatchRunFinishes(t *testing.T) {
	c := newClient(t, &stubLLM{reply: func(int) string { return ambigous }})
	sub := submit(t, c, "", cid1)
	asked := watchUntilTurnResult(t, c, sub.GetRunId(), accountID)
	if asked == nil || asked.GetInterrupt().GetInterruptId() == "" || len(asked.GetQuestion().GetOptions()) == 0 {
		t.Fatalf("要停在中斷並帶選項：%v", asked)
	}
	snap := snapshot(t, c, sub.GetConversationId())
	oc := snap.GetOpenConfirmation()
	if oc.GetInterruptId() != asked.GetInterrupt().GetInterruptId() || oc.GetProcessing() || snap.GetRuns()[0].GetStatus() != pb.RunStatus_RUN_STATUS_WAITING_FOR_INPUT {
		t.Fatalf("等待回答的快照 = %v", snap)
	}

	// 回答不合：選項不在卡上 → INVALID_ARGUMENT，路徑在巢狀的 clarification 底下。
	_, err := c.ResumeRun(t.Context(), as(connect.NewRequest(&pb.ResumeRunRequest{InterruptId: oc.GetInterruptId(), ClientRequestId: cid2,
		Response: &pb.ResumeRunRequest_Clarification{Clarification: &pb.ClarificationResponse{OptionId: "nope"}}}), accountID))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("不在卡上的選項 = %v", err)
	}
	if got := violationFields(t, err); len(got) != 1 || got[0] != "clarification.option_id" {
		t.Fatalf("欄位路徑 = %v", got)
	}
	_, err = c.ResumeRun(t.Context(), as(connect.NewRequest(&pb.ResumeRunRequest{InterruptId: oc.GetInterruptId(), ClientRequestId: cid2,
		Response: &pb.ResumeRunRequest_Clarification{Clarification: &pb.ClarificationResponse{OptionId: "x", FreeText: "y"}}}), accountID))
	if got := violationFields(t, err); len(got) != 2 || got[0] != "clarification.option_id" || got[1] != "clarification.free_text" {
		t.Fatalf("兩個都給的欄位路徑 = %v", got)
	}
	_, err = c.ResumeRun(t.Context(), as(connect.NewRequest(&pb.ResumeRunRequest{InterruptId: oc.GetInterruptId(), ClientRequestId: cid2,
		Response: &pb.ResumeRunRequest_Clarification{Clarification: &pb.ClarificationResponse{OptionId: oc.GetQuestion().GetOptions()[0].GetOptionId()}}}), accountID+1))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("別的買家回答 = %v", err)
	}

	res, err := c.ResumeRun(t.Context(), as(connect.NewRequest(&pb.ResumeRunRequest{InterruptId: oc.GetInterruptId(), ClientRequestId: cid3,
		Response: &pb.ResumeRunRequest_Clarification{Clarification: &pb.ClarificationResponse{OptionId: oc.GetQuestion().GetOptions()[0].GetOptionId()}}}), accountID))
	if err != nil || res.Msg.GetRunId() == "" || res.Msg.GetRunId() == sub.GetRunId() || res.Msg.GetRunStatus() == pb.RunStatus_RUN_STATUS_UNSPECIFIED {
		t.Fatalf("ResumeRun = %v, %v", res, err)
	}
	aevs := watch(t, c, res.Msg.GetRunId(), accountID)
	if f := finished(t, aevs); f.GetStatus() != pb.RunStatus_RUN_STATUS_SUCCEEDED {
		t.Fatalf("回答 run finished = %v", f)
	}
	after := snapshot(t, c, sub.GetConversationId())
	if after.GetOpenConfirmation() != nil || after.GetActiveRunId() != "" {
		t.Fatalf("回答之後不得還有待答的中斷：%v", after)
	}
	for _, r := range after.GetRuns() {
		if r.GetStatus() != pb.RunStatus_RUN_STATUS_SUCCEEDED {
			t.Errorf("run %v 要都是 SUCCEEDED", r)
		}
	}
}

func TestCancelRunEndsTheStreamWithCancelled(t *testing.T) {
	l := &stubLLM{block: true, started: make(chan struct{}, 1)}
	c := newClient(t, l)
	sub := submit(t, c, "", cid1)
	select {
	case <-l.started:
	case <-time.After(60 * time.Second):
		t.Fatal("run 沒有開始執行")
	}
	res, err := c.CancelRun(t.Context(), as(connect.NewRequest(&pb.CancelRunRequest{RunId: sub.GetRunId()}), accountID))
	if err != nil || res.Msg.GetCancelDisposition() != pb.CancelDisposition_CANCEL_DISPOSITION_REQUESTED || res.Msg.GetRunId() != sub.GetRunId() {
		t.Fatalf("CancelRun = %v, %v", res, err)
	}
	if f := finished(t, watch(t, c, sub.GetRunId(), accountID)); f.GetStatus() != pb.RunStatus_RUN_STATUS_CANCELLED {
		t.Fatalf("finished = %v", f)
	}
	again, err := c.CancelRun(t.Context(), as(connect.NewRequest(&pb.CancelRunRequest{RunId: sub.GetRunId()}), accountID))
	if err != nil || again.Msg.GetCancelDisposition() != pb.CancelDisposition_CANCEL_DISPOSITION_ALREADY_CANCELLED || again.Msg.GetRunStatus() != pb.RunStatus_RUN_STATUS_CANCELLED {
		t.Fatalf("再取消 = %v, %v", again, err)
	}
	if _, err := c.CancelRun(t.Context(), as(connect.NewRequest(&pb.CancelRunRequest{RunId: sub.GetRunId()}), accountID+1)); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("別的買家取消 = %v", err)
	}
}

func TestSubmitTurnRejectsMessagesAndUnknownRequestsBeforeAnyRun(t *testing.T) {
	c := newClient(t, &stubLLM{reply: func(int) string { return ready }})
	long := make([]rune, 2001)
	for i := range long {
		long[i] = '蘋'
	}
	for name, msg := range map[string]string{"只有空白": "  \n ", "超過 2000 字元": string(long)} {
		_, err := c.SubmitTurn(t.Context(), as(connect.NewRequest(&pb.SubmitTurnRequest{Message: msg, ClientRequestId: cid1}), accountID))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s：%v", name, err)
			continue
		}
		if got := violationFields(t, err); len(got) != 1 || got[0] != "message" {
			t.Errorf("%s：欄位 = %v", name, got)
		}
	}
	_, err := c.SubmitTurn(t.Context(), as(connect.NewRequest(&pb.SubmitTurnRequest{RequestId: cid2, Message: "x", ClientRequestId: cid1}), accountID))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("不存在的需求 = %v", err)
	}
	// 別的買家的需求：同樣 NOT_FOUND。
	sub := submit(t, c, "", cid3)
	_, err = c.SubmitTurn(t.Context(), as(connect.NewRequest(&pb.SubmitTurnRequest{RequestId: sub.GetRequestId(), Message: "x", ClientRequestId: cid2}), accountID+1))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("別人的需求 = %v", err)
	}
}

// 新建的需求以 UUIDv7 為 ID：回給 client 的 request_id 要能原樣帶回來做後續的一輪。
func TestFollowUpSubmitTurnWithTheReturnedRequestIDSucceeds(t *testing.T) {
	c := newClient(t, &stubLLM{reply: func(int) string { return ready }})
	first := submit(t, c, "", cid1)
	if f := finished(t, watch(t, c, first.GetRunId(), accountID)); f.GetStatus() != pb.RunStatus_RUN_STATUS_SUCCEEDED {
		t.Fatalf("第一輪 finished = %v", f)
	}
	second := submit(t, c, first.GetRequestId(), cid2)
	if second.GetRequestId() != first.GetRequestId() || second.GetConversationId() != first.GetConversationId() || second.GetRunId() == first.GetRunId() {
		t.Fatalf("後續一輪 = %v，要同需求同對話、新的 run（第一輪 %v）", second, first)
	}
}
