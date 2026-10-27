package proxy

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ==================== writeRunEvent (projector) ====================

func TestWriteRunEvent_NativeRunFinished_MergesTypeAndOverridesRunId(t *testing.T) {
	var buf strings.Builder
	ev := &aipb.RunEvent{
		RunId: "5150", Seq: "7", Type: codemeta.ProcurementLedgerEventTypeRunFinished.Value,
		DataJson: []byte(`{"interruptId":"i-1","interruptKind":"crop_disambiguation"}`),
	}
	writeRunEvent(&buf, ev)

	got := buf.String()
	if !strings.HasPrefix(got, "id: 7\n") {
		t.Fatalf("expected id: line to equal the ledger seq, got %q", got)
	}
	if !strings.Contains(got, `"type":"RUN_FINISHED"`) {
		t.Fatalf("expected native RUN_FINISHED type, got %q", got)
	}
	// data_json 的既有欄位（interruptId／interruptKind）必須保留，同時 runId 必須
	// 由 RunEvent 自帶的值覆寫（即使 data_json 完全沒帶這個欄位也要補上）。
	if !strings.Contains(got, `"interruptId":"i-1"`) || !strings.Contains(got, `"interruptKind":"crop_disambiguation"`) {
		t.Fatalf("expected original data_json fields to survive the merge, got %q", got)
	}
	if !strings.Contains(got, `"runId":"5150"`) {
		t.Fatalf("expected runId backfilled from RunEvent.RunId, got %q", got)
	}
	if !strings.HasSuffix(got, "\n\n") {
		t.Fatalf("expected frame to end with a blank line (SSE frame terminator), got %q", got)
	}
}

func TestWriteRunEvent_NativeType_EmptyPayloadStillCarriesTypeAndRunId(t *testing.T) {
	// turn.go/answer.go/rewind.go 四個 usecase 的 terminalPayload 常見值就是
	// 字面 `{}`——native 分支必須在完全沒有既有欄位可合併時仍然送出可用的 frame。
	var buf strings.Builder
	ev := &aipb.RunEvent{
		RunId: "9001", Seq: "3", Type: codemeta.ProcurementLedgerEventTypeRunFinished.Value,
		DataJson: []byte(`{}`),
	}
	writeRunEvent(&buf, ev)

	got := buf.String()
	if !strings.Contains(got, `"type":"RUN_FINISHED"`) || !strings.Contains(got, `"runId":"9001"`) {
		t.Fatalf("expected type+runId even with an empty {} payload, got %q", got)
	}
}

// TestWriteRunEvent_RunFailed_ProjectsToNativeRunError 守住 doc57 §4.2 Run 那一列
// 的 carrier 定義：`RUN_STARTED`／`RUN_FINISHED`／`RUN_ERROR` 為原生。`RUN_ERROR`
// 是 AG-UI 協定裡「這個 run 以錯誤結束」的原生事件（`@ag-ui/core` 的
// RunErrorEventSchema：message 必填、code 選填、schema 為 passthrough），不是傳輸層
// 專用構造——把 RUN_FAILED 塞進 `CUSTOM name="run_status"` 等於用一個沒有判別欄位的
// 通用桶，去裝協定本來就有專屬事件的東西：七種 run 事件全塌成同一個 name，而它們的
// data_json 又不自描述（候選搜尋的 payload 是字面 `{}`），消費端結構上分不出失敗與接受。
//
// 機器碼走 `reason` 不走 `code`：`code` 被 website 以 /^-?\d+$/ 過濾，本 repo 既有慣例
// 是 UPPER_SNAKE 機器碼一律放 `reason`（見 agUIRunError 的文件）。這也讓消費端天然分辨
// 得出兩種 RUN_ERROR——傳輸失敗帶 HTTP 字串 code，業務失敗帶 reason。
func TestWriteRunEvent_RunFailed_ProjectsToNativeRunError(t *testing.T) {
	var buf strings.Builder
	ev := &aipb.RunEvent{
		RunId: "5150", Seq: "4", Type: codemeta.ProcurementLedgerEventTypeRunFailed.Value,
		DataJson: []byte(`{"failureReason":"LANE_ALL_FAILED"}`),
	}
	writeRunEvent(&buf, ev)

	got := buf.String()
	if !strings.HasPrefix(got, "id: 4\n") {
		t.Fatalf("expected id: 4 (ledger seq 仍是 replay cursor), got %q", got)
	}
	if !strings.Contains(got, `"type":"RUN_ERROR"`) {
		t.Fatalf("expected native RUN_ERROR, got %q", got)
	}
	if !strings.Contains(got, `"reason":"LANE_ALL_FAILED"`) {
		t.Fatalf("expected failureReason projected into reason, got %q", got)
	}
	if !strings.Contains(got, `"message":"全部通道失敗"`) {
		t.Fatalf("expected CodeMeta label as message, got %q", got)
	}
	if strings.Contains(got, `"code"`) {
		t.Fatalf("code 保留給傳輸層的 HTTP 字串碼，業務失敗不得佔用, got %q", got)
	}
}

// TestWriteRunEvent_RunFailed_UnknownReason_StillEmitsUsableRunError：AG-UI 的
// message 是必填 ZodString，缺了整個 frame 會在消費端被拒。payload 沒帶
// failureReason（或帶了一個不在 CodeMeta 值域裡的值）時仍要送得出一個合法 frame，
// 但**不得**憑空補一個機器碼——reason 直接省略。
func TestWriteRunEvent_RunFailed_UnknownReason_StillEmitsUsableRunError(t *testing.T) {
	var buf strings.Builder
	ev := &aipb.RunEvent{
		RunId: "5150", Seq: "9", Type: codemeta.ProcurementLedgerEventTypeRunFailed.Value,
		DataJson: []byte(`{}`),
	}
	writeRunEvent(&buf, ev)

	got := buf.String()
	if !strings.Contains(got, `"type":"RUN_ERROR"`) {
		t.Fatalf("expected native RUN_ERROR, got %q", got)
	}
	if !strings.Contains(got, `"message":`) {
		t.Fatalf("AG-UI message 必填，缺了會被消費端 schema 拒絕, got %q", got)
	}
	if strings.Contains(got, `"reason"`) {
		t.Fatalf("值域外／缺值時不得憑空補機器碼, got %q", got)
	}
}

// TestWriteRunEvent_RunFamilyCustom_EachTypeHasItsOwnName：Run 家族剩下的
// CUSTOM 型別各自具名，不再共用 `run_status`。判別欄位必須存在於某處——
// activity／artifact／step_detail／confirmation 是靠自描述的 payload 帶著，
// 而 Run 家族的 payload 不自描述，所以判別欄位只能是 CUSTOM 的 `name`。
func TestWriteRunEvent_RunFamilyCustom_EachTypeHasItsOwnName(t *testing.T) {
	for _, tc := range []struct{ ledgerType, wantName string }{
		{codemeta.ProcurementLedgerEventTypeRunAccepted.Value, "run_accepted"},
		{codemeta.ProcurementLedgerEventTypeRunCancelRequested.Value, "run_cancel_requested"},
		{codemeta.ProcurementLedgerEventTypeRunWaiting.Value, "run_waiting"},
		{codemeta.ProcurementLedgerEventTypeRunCancelled.Value, "run_cancelled"},
		{codemeta.ProcurementLedgerEventTypeRunResumed.Value, "run_resumed"},
		{codemeta.ProcurementLedgerEventTypeRunExpired.Value, "run_expired"},
	} {
		var buf strings.Builder
		writeRunEvent(&buf, &aipb.RunEvent{RunId: "5150", Seq: "2", Type: tc.ledgerType, DataJson: []byte(`{}`)})
		got := buf.String()
		want := `"type":"CUSTOM","name":"` + tc.wantName + `"`
		if !strings.Contains(got, want) {
			t.Errorf("%s: expected %s, got %q", tc.ledgerType, want, got)
		}
		if strings.Contains(got, `"name":"run_status"`) {
			t.Errorf("%s: run_status 這個共用桶已移除（判別欄位要能存在）, got %q", tc.ledgerType, got)
		}
	}
}

func TestWriteRunEvent_StepStarted_MapsToCustomStepDetail(t *testing.T) {
	var buf strings.Builder
	ev := &aipb.RunEvent{
		RunId: "5150", Seq: "1", Type: codemeta.ProcurementLedgerEventTypeStepStarted.Value,
		DataJson: []byte(`{"code":"EXTRACT","status":"RUNNING","label":"解析中","durationMs":0}`),
	}
	writeRunEvent(&buf, ev)

	got := buf.String()
	if !strings.Contains(got, `"type":"CUSTOM","name":"step_detail"`) {
		t.Fatalf("expected STEP_STARTED to become CUSTOM name=step_detail, got %q", got)
	}
	if !strings.Contains(got, `"code":"EXTRACT"`) {
		t.Fatalf("expected the ledger payload to survive as the CUSTOM value, got %q", got)
	}
}

func TestWriteRunEvent_ConfirmationRequired_MapsToCustomConfirmation(t *testing.T) {
	var buf strings.Builder
	ev := &aipb.RunEvent{
		RunId: "5150", Seq: "9", Type: codemeta.ProcurementLedgerEventTypeConfirmationRequired.Value,
		DataJson: []byte(`{"interruptId":"i-1","interruptKind":"crop_disambiguation"}`),
	}
	writeRunEvent(&buf, ev)

	got := buf.String()
	if !strings.Contains(got, `"type":"CUSTOM","name":"confirmation"`) {
		t.Fatalf("expected CONFIRMATION_REQUIRED to become CUSTOM name=confirmation, got %q", got)
	}
}

// TestWriteRunEvent_UnproducedToolEventType_FallsBackToGenericCustom pins the
// deliberate degradation for TOOL_STARTED/TOOL_FINISHED: doc57 §4.2 says their
// wire carrier is native TOOL_CALL_*, but no producer anywhere in this
// codebase currently emits either type (verified: zero occurrences of
// codemeta.ProcurementLedgerEventTypeToolStarted/Finished as an AppendRunEvent
// EventType). Guessing an unproven multi-frame shape would be worse than this
// safe, forward-compatible fallback — see customSlugForRunEventType's default
// case comment.
func TestWriteRunEvent_UnproducedToolEventType_FallsBackToGenericCustom(t *testing.T) {
	var buf strings.Builder
	ev := &aipb.RunEvent{
		RunId: "5150", Seq: "2", Type: codemeta.ProcurementLedgerEventTypeToolStarted.Value,
		DataJson: []byte(`{"tool":"search_candidates"}`),
	}
	writeRunEvent(&buf, ev)

	got := buf.String()
	if !strings.Contains(got, `"type":"CUSTOM","name":"run_event"`) {
		t.Fatalf("expected the generic run_event fallback for an unimplemented ledger type, got %q", got)
	}
}

func TestWriteRunEvent_InvalidDataJson_FallsBackToStringValue(t *testing.T) {
	var buf strings.Builder
	ev := &aipb.RunEvent{
		RunId: "5150", Seq: "2", Type: codemeta.ProcurementLedgerEventTypeStepStarted.Value,
		DataJson: []byte(`not-json`),
	}
	writeRunEvent(&buf, ev)

	got := buf.String()
	// 不合法 JSON 必須仍然送出一個可解析的 frame（value 退回字串型別），而不是讓
	// 外層 json.Marshal 產出破損的 data: 行或整個事件消失。
	var parsed map[string]any
	dataLine := strings.TrimPrefix(strings.SplitN(got, "\n", 2)[1], "data: ")
	dataLine = strings.TrimSuffix(dataLine, "\n\n")
	if err := json.Unmarshal([]byte(dataLine), &parsed); err != nil {
		t.Fatalf("expected the emitted frame to itself be valid JSON, got %q: %v", got, err)
	}
	if parsed["value"] != "not-json" {
		t.Fatalf("expected invalid data_json to degrade to a plain string value, got %+v", parsed)
	}
}

// TestWriteRunEvent_MessageCompleted_EmitsNativeThreeFrameSequence pins the
// gap the brief's own sketch missed: MESSAGE_COMPLETED already has a real
// producer (turn.go ledgerEventTypeFor, TurnEventResult case) and doc57 §4.2
// documents its wire carrier as native TEXT_MESSAGE_START/CONTENT/END, not
// CUSTOM — unlike TOOL_STARTED/FINISHED, this one cannot be safely deferred.
func TestWriteRunEvent_MessageCompleted_EmitsNativeThreeFrameSequence(t *testing.T) {
	var buf strings.Builder
	ev := &aipb.RunEvent{
		RunId: "5150", Seq: "12", Type: codemeta.ProcurementLedgerEventTypeMessageCompleted.Value,
		DataJson: []byte(`{"messageId":"9003","revisionId":"9003","text":"已為您找到 3 個候選"}`),
	}
	writeRunEvent(&buf, ev)

	got := buf.String()
	wantInOrder := []string{
		`"type":"TEXT_MESSAGE_START","messageId":"9003","role":"assistant"`,
		`"type":"TEXT_MESSAGE_CONTENT","messageId":"9003","delta":"已為您找到 3 個候選"`,
		`"type":"TEXT_MESSAGE_END","messageId":"9003"`,
	}
	last := -1
	for _, want := range wantInOrder {
		idx := strings.Index(got, want)
		if idx < 0 {
			t.Fatalf("expected %q in output, got %q", want, got)
		}
		if idx <= last {
			t.Fatalf("frames out of order: %q appeared before an earlier expected frame in %q", want, got)
		}
		last = idx
	}
	// id: 行只能出現一次，且必須掛在最後一個 frame（TEXT_MESSAGE_END）之前——
	// 掛在 START／CONTENT 上會讓斷線重連在這個 group 中段時漏送 END（見
	// writeMessageCompletedRunEvent 的文件註解）。
	if n := strings.Count(got, "id: 12\n"); n != 1 {
		t.Fatalf("expected exactly one id: line for this ledger row, got %d in %q", n, got)
	}
	endIdx := strings.Index(got, `"type":"TEXT_MESSAGE_END"`)
	idIdx := strings.Index(got, "id: 12\n")
	if idIdx == -1 || idIdx > endIdx {
		t.Fatalf("expected the id: line to precede the TEXT_MESSAGE_END data: line, got %q", got)
	}
	startIdx := strings.Index(got, `"type":"TEXT_MESSAGE_START"`)
	if idx := strings.Index(got[:startIdx], "id: "); idx != -1 {
		t.Fatalf("expected no id: line before TEXT_MESSAGE_START, got %q", got)
	}
}

func TestWriteRunEvent_MessageCompleted_MalformedPayloadFallsBackToCustom(t *testing.T) {
	var buf strings.Builder
	ev := &aipb.RunEvent{
		RunId: "5150", Seq: "12", Type: codemeta.ProcurementLedgerEventTypeMessageCompleted.Value,
		DataJson: []byte(`{"revisionId":"9003"}`), // 沒有 messageId
	}
	writeRunEvent(&buf, ev)

	got := buf.String()
	if strings.Contains(got, "TEXT_MESSAGE_START") {
		t.Fatalf("expected no native TEXT_MESSAGE_* frames when messageId is missing, got %q", got)
	}
	if !strings.Contains(got, `"type":"CUSTOM","name":"run_event"`) {
		t.Fatalf("expected a CUSTOM run_event fallback for a malformed MESSAGE_COMPLETED payload, got %q", got)
	}
}

// TestActivityFrameParity_POSTStreamMatchesReplay 釘住 doc57 §2.1 的跨路徑
// 不變式：POST 串流（writeStreamEvent 的 StreamEvent_Activity case）與 GET
// replay（writeRunEvent 的 AGENT_ACTIVITY_* 分支）對同一份 data_json 必須送出
// **逐位元組相同**的 frame，唯一容許的差異是 replay 多一行 `id: <ledger seq>`
// ——那是 replay 專屬的 cursor，POST 串流沒有 replay 語意，不寫這行。
//
// 為什麼需要一支獨立的測試，而不是靠「兩邊都呼叫 rawJSONOrString」這個共用
// helper 的論證：共用 helper 只保證 **value 的編碼規則**一致，保證不了 frame
// 的其餘部分。最具體的漏洞是 `name`——replay 是**推導**出來的
// （customSlugForRunEventType 依 event type 回傳 "activity"），POST 是**寫死**
// 的字面 "activity"。把那個 slug 改名，replay 會跟著改、POST 不會，而
// custom-activity golden case（自建 agUICustom）與
// TestWriteStreamEvent_Activity_MapsToCustomActivity（自帶字面 want）**都不會
// 轉紅**。兩個 writer 就這樣無聲漂移——這正是 T12 造就這波工作的失敗形狀。
//
// 放在本檔而非 ai_assistant_stream_test.go：這個不變式的脆弱端是 replay 這側
// 的推導邏輯，改 customSlugForRunEventType 的人最可能先看到這裡。
func TestActivityFrameParity_POSTStreamMatchesReplay(t *testing.T) {
	const seq = "42"
	payload := []byte(`{"rowId":"7489","attempt":1,"maxAttempts":3,"retryGroupKey":"7489","code":"EXTRACT_SLOTS","text":"整理你的採購條件"}`)

	var post strings.Builder
	writeStreamEvent(&post, &aipb.StreamEvent{Payload: &aipb.StreamEvent_Activity{
		Activity: &aipb.ActivityEvent{DataJson: payload},
	}}, &agUIStreamState{}, "thread-1", "run-1")

	var replay strings.Builder
	writeRunEvent(&replay, &aipb.RunEvent{
		RunId: "run-1", Seq: seq,
		Type:     codemeta.ProcurementLedgerEventTypeAgentActivitySnapshot.Value,
		DataJson: payload,
	})

	if post.String() == "" {
		t.Fatal("POST 路徑沒有寫出任何 frame——StreamEvent_Activity case 不見了？")
	}
	if got, want := replay.String(), "id: "+seq+"\n"+post.String(); got != want {
		t.Fatalf("POST 與 replay 的 activity frame 已漂移（唯一容許的差異是 replay 多一行 id:）。\n"+
			"replay 實得：%q\n依 POST 推算應為：%q\n"+
			"最可能的原因：改了其中一側的 name／value 編碼／frame 結構卻沒同步另一側"+
			"（例如 customSlugForRunEventType 的 \"activity\" slug 改名）。", got, want)
	}
}

// ==================== ServeAIProcurementRunEventsStream ====================

type fakeRunEventStream struct {
	events []*aipb.RunEvent
	err    error // 若非 nil，events 送完後回傳這個 error 而不是 io.EOF
	idx    int
}

func (s *fakeRunEventStream) Recv() (*aipb.RunEvent, error) {
	if s.idx < len(s.events) {
		e := s.events[s.idx]
		s.idx++
		return e, nil
	}
	if s.err != nil {
		return nil, s.err
	}
	return nil, io.EOF
}

func TestServeAIProcurementRunEventsStream_Success_WritesFramesWithSSEHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/x", nil)

	stream := &fakeRunEventStream{events: []*aipb.RunEvent{
		{RunId: "5150", Seq: "1", Type: codemeta.ProcurementLedgerEventTypeStepStarted.Value, DataJson: []byte(`{"code":"EXTRACT"}`)},
		{RunId: "5150", Seq: "2", Type: codemeta.ProcurementLedgerEventTypeMessageCompleted.Value, DataJson: []byte(`{"messageId":"m1","text":"hi"}`)},
	}}

	var gotAfterSeq string
	ServeAIProcurementRunEventsStream(c, "0", func(ctx context.Context, afterSeq string) (RunEventStream, error) {
		gotAfterSeq = afterSeq
		return stream, nil
	})

	if gotAfterSeq != "0" {
		t.Fatalf("expected lastEventID to be forwarded as afterSeq, got %q", gotAfterSeq)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "private, no-store" {
		t.Fatalf("Cache-Control = %q, want private, no-store", cc)
	}
	body := w.Body.String()
	if strings.Contains(body, "event:") {
		t.Fatalf("AG-UI frames must not have an event: line, got %q", body)
	}
	if !strings.Contains(body, "id: 1\n") || !strings.Contains(body, "id: 2\n") {
		t.Fatalf("expected both ledger events' seq to appear as id: lines, got %q", body)
	}
	if !strings.Contains(body, `"type":"CUSTOM","name":"step_detail"`) {
		t.Fatalf("expected the STEP_STARTED event projected, got %q", body)
	}
	if !strings.Contains(body, `"type":"TEXT_MESSAGE_START"`) {
		t.Fatalf("expected the MESSAGE_COMPLETED event projected to native frames, got %q", body)
	}
}

func TestServeAIProcurementRunEventsStream_OpenFailure_Returns424EnvelopeNotSSE(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/x", nil)

	openCalls := 0
	ServeAIProcurementRunEventsStream(c, "", func(ctx context.Context, afterSeq string) (RunEventStream, error) {
		openCalls++
		return nil, status.Error(codes.NotFound, "run not found")
	})

	if openCalls != 1 {
		t.Fatalf("expected exactly one open attempt, got %d", openCalls)
	}
	if w.Code != http.StatusFailedDependency {
		t.Fatalf("expected HTTP 424 on open failure, got %d (body %q)", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected a JSON envelope, got Content-Type %q", ct)
	}
	if strings.Contains(w.Body.String(), "event:") || strings.Contains(w.Body.String(), "data:") {
		t.Fatalf("open failure must not emit an SSE frame, got %q", w.Body.String())
	}
}

// TestServeAIProcurementRunEventsStream_ReplayCursorExpiredMapsTo410InBand is
// the end-to-end regression guard for the wire.go GRPCCodeToHTTPStatus gap
// this task fixes: a mid-stream codes.OutOfRange (REPLAY_CURSOR_EXPIRED) must
// surface as an in-band RUN_ERROR-equivalent frame carrying code "410", the
// same bucket-table convention ServeAIProcurementStream already uses for its
// own mid-stream errors — not the pre-fix default of 500.
func TestServeAIProcurementRunEventsStream_ReplayCursorExpiredMapsTo410InBand(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/x", nil)

	stream := &fakeRunEventStream{err: status.Error(codes.OutOfRange, "游標已早於此 run 目前可 replay 的最小 seq")}

	ServeAIProcurementRunEventsStream(c, "1", func(ctx context.Context, afterSeq string) (RunEventStream, error) {
		return stream, nil
	})

	body := w.Body.String()
	if !strings.Contains(body, `"type":"RUN_ERROR"`) {
		t.Fatalf("expected a RUN_ERROR frame, got %q", body)
	}
	if !strings.Contains(body, `"code":"410"`) {
		t.Fatalf("expected code 410 (REPLAY_CURSOR_EXPIRED bucket), got %q", body)
	}
	if !strings.Contains(body, "游標已早於此 run 目前可 replay 的最小 seq") {
		t.Fatalf("expected the gRPC status message to reach the client, got %q", body)
	}
}

func TestServeAIProcurementRunEventsStream_ReplayCursorAheadMapsTo409InBand(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/x", nil)

	stream := &fakeRunEventStream{err: status.Error(codes.Aborted, "游標高於此 run 目前的 high-water mark")}

	ServeAIProcurementRunEventsStream(c, "999", func(ctx context.Context, afterSeq string) (RunEventStream, error) {
		return stream, nil
	})

	body := w.Body.String()
	if !strings.Contains(body, `"code":"409"`) {
		t.Fatalf("expected code 409 (REPLAY_CURSOR_AHEAD bucket), got %q", body)
	}
}

func TestServeAIProcurementRunEventsStream_CleanEOFEndsQuietlyWithoutErrorFrame(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/x", nil)

	stream := &fakeRunEventStream{events: []*aipb.RunEvent{
		{RunId: "5150", Seq: "1", Type: codemeta.ProcurementLedgerEventTypeStepStarted.Value, DataJson: []byte(`{}`)},
	}} // err is nil，送完 events 後 Recv 回 io.EOF

	ServeAIProcurementRunEventsStream(c, "", func(ctx context.Context, afterSeq string) (RunEventStream, error) {
		return stream, nil
	})

	body := w.Body.String()
	if strings.Contains(body, "RUN_ERROR") {
		t.Fatalf("a clean upstream EOF (doc57 §2.5-8: disconnect only aborts the subscription) must not produce a RUN_ERROR frame, got %q", body)
	}
}

// blockingRunEventStream simulates a real gRPC stream whose Recv() only
// unblocks when the RPC's context ends — same technique as
// ai_assistant_stream_test.go's blockingAssistantStream (see its doc comment
// for why status.FromContextError, not a bare ctx.Err(), is required to
// exercise the codes.DeadlineExceeded branch at all).
type blockingRunEventStream struct {
	ctx context.Context
}

func (s *blockingRunEventStream) Recv() (*aipb.RunEvent, error) {
	<-s.ctx.Done()
	return nil, status.FromContextError(s.ctx.Err()).Err()
}

// TestServeAIProcurementRunEventsStream_TurnDeadlineMapsToFriendlyMessage is
// the regression guard for this file's own codes.DeadlineExceeded → friendly
// message swap (replayDeadlineExceededMessage) — same production-incident
// class ai_assistant_stream.go's identical swap guards against (a raw
// "context deadline exceeded" string must never reach the user), but with
// this endpoint's own remedy wording ("reconnect", not "resend the
// message"). Uses turnDeadline (serveAIProcurementRunEventsStreamWith's
// injectable parameter) to avoid waiting out the real 120s streamTurnDeadline.
func TestServeAIProcurementRunEventsStream_TurnDeadlineMapsToFriendlyMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/x", nil)

	const tinyDeadline = 50 * time.Millisecond

	done := make(chan struct{})
	go func() {
		serveAIProcurementRunEventsStreamWith(c, "", func(ctx context.Context, afterSeq string) (RunEventStream, error) {
			return &blockingRunEventStream{ctx: ctx}, nil
		}, tinyDeadline)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("serveAIProcurementRunEventsStreamWith did not return after its turn deadline fired — the stream goroutine hung instead of converging")
	}

	body := w.Body.String()
	if !strings.Contains(body, `"type":"RUN_ERROR"`) {
		t.Fatalf("expected a RUN_ERROR frame after the turn deadline fired, got %q", body)
	}
	if strings.Contains(body, "context deadline exceeded") {
		t.Fatalf("grpc-go's literal DeadlineExceeded error text leaked into the client-facing message, got %q", body)
	}
	if !strings.Contains(body, replayDeadlineExceededMessage) {
		t.Fatalf("expected the fixed Traditional Chinese timeout message %q, got %q", replayDeadlineExceededMessage, body)
	}
}

// TestWriteRunEvent_NativeType_ProjectsThreadIDFromConversationID 釘住原生
// RUN_STARTED／RUN_FINISHED frame 一定帶 threadId——官方 AG-UI schema 把它列為
// 必填，缺了整個 frame 會在消費端的 zod 驗證被拒。在補上這個欄位之前，這個端點
// 送出的原生 RUN_FINISHED 從來沒有通過過 client 的 schema 檢查（2026-09-06 由
// website 的 replay client review 追出來），「run 結束就提早收線」因此在線上
// 是死路徑。
func TestWriteRunEvent_NativeType_ProjectsThreadIDFromConversationID(t *testing.T) {
	var buf strings.Builder
	ev := &aipb.RunEvent{
		RunId: "9001", Seq: "3", Type: codemeta.ProcurementLedgerEventTypeRunFinished.Value,
		DataJson: []byte(`{}`), ConversationId: "777",
	}
	writeRunEvent(&buf, ev)

	got := buf.String()
	if !strings.Contains(got, `"threadId":"777"`) {
		t.Fatalf("expected threadId projected from RunEvent.ConversationId, got %q", got)
	}
}

// TestWriteRunEvent_NativeType_ThreadIDPresentEvenWhenConversationAbsent 釘住
// 「欄位一定在」這件事本身：run 沒有掛在任何對話上時值是空字串，但 key 不得消失
// ——schema 要的是 threadId 存在且是字串，省略這個 key 會讓整個 frame 被拒，
// 那正是本次要修掉的失敗模式。
func TestWriteRunEvent_NativeType_ThreadIDPresentEvenWhenConversationAbsent(t *testing.T) {
	var buf strings.Builder
	ev := &aipb.RunEvent{
		RunId: "9001", Seq: "3", Type: codemeta.ProcurementLedgerEventTypeRunFinished.Value,
		DataJson: []byte(`{}`),
	}
	writeRunEvent(&buf, ev)

	got := buf.String()
	if !strings.Contains(got, `"threadId":""`) {
		t.Fatalf("expected threadId key to exist with an empty value, got %q", got)
	}
}

// TestWriteRunEvent_CustomType_HasNoThreadID 釘住這個欄位只補在原生 frame 上：
// CUSTOM frame 的 schema 沒有 threadId，多塞一個未宣告的 key 會被 zod 的
// strict 模式拒絕，反而製造出第二種「整個 frame 被丟掉」的失敗。
func TestWriteRunEvent_CustomType_HasNoThreadID(t *testing.T) {
	var buf strings.Builder
	ev := &aipb.RunEvent{
		RunId: "5150", Seq: "4", Type: codemeta.ProcurementLedgerEventTypeAgentActivityDelta.Value,
		DataJson: []byte(`{"rowId":"9001"}`), ConversationId: "777",
	}
	writeRunEvent(&buf, ev)

	got := buf.String()
	if strings.Contains(got, "threadId") {
		t.Fatalf("CUSTOM frame must not carry threadId, got %q", got)
	}
}
