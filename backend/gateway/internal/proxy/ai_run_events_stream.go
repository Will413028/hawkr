package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	procurementwire "github.com/Will413028/hawkr/gateway/internal/procurement"
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// runEventNativeTypes 是 doc57 §4.2 事件表裡 wire carrier 為 AG-UI 原生型別、
// 且是「一筆 ledger event 對一個 frame」形狀的 ledger event type：只有
// RUN_STARTED／RUN_FINISHED 符合（原生 frame 就是 data_json 本身，只需要補上
// `type` 欄位）。MESSAGE_COMPLETED 的原生 carrier 是 TEXT_MESSAGE_START／
// CONTENT／END 三個 frame，形狀不同，走 writeMessageCompletedRunEvent 特殊處理
// 而不進這張表。TOOL_STARTED／TOOL_FINISHED 的原生 carrier 也是多個 frame
// （TOOL_CALL_START／ARGS、TOOL_CALL_RESULT／END），但目前沒有任何 producer
// 真的寫出這兩個型別（services/ai-service 全文搜尋零命中），沒有已知的
// data_json 形狀可供正確拆分，見 customSlugForRunEventType 的 default 分支。
//
// ⚠️ **RUN_ACCEPTED 是有 producer 的**（repository/run.go 的 ClaimRun 與
// CreateLedgerRunForCandidateSearch 都會寫首筆 RUN_ACCEPTED）——本註解原本斷言它
// 沒有，那個結論來自只掃 usecase 層（executeTurnPipeline 等四支確實只寫
// RUN_FINISHED／RUN_FAILED／CONFIRMATION_REQUIRED＋STEP_*／MESSAGE_COMPLETED），
// 漏掉了 repository 層；而且那三處當時是用**裸字串**寫 event type，連值域盤點都
// 掃不到。它走 CUSTOM name="run_accepted"，不在這張原生表裡。
//
// RUN_STARTED 目前確實還沒有 producer（usecase 與 repository 兩層皆零命中）；
// 留在這張表裡是 forward-compat（doc57 §4.2 明訂它是原生 carrier），現在是死路徑。
var runEventNativeTypes = map[string]bool{
	codemeta.ProcurementLedgerEventTypeRunStarted.Value:  true,
	codemeta.ProcurementLedgerEventTypeRunFinished.Value: true,
}

// writeAGUIFrameWithID 寫一個帶 `id:` 行的 AG-UI SSE frame（doc57 §2.4：
// `id:` 行等於 ledger seq，只在 GET replay 端點出現——這是這個 helper 沒有放
// 進 ag_ui_events.go 的原因，那個檔案的 writeAGUIFrame 服務的是永遠不帶
// `id:` 行的既有 POST 串流端點，兩者的不變式不同，混在一起會讓其中一個變得
// 容易被改壞）。單一次 io.WriteString 把 `id:`／`data:` 兩行原子寫出，理由
// 同 deadlineResponseWriter：每次 Write 都會重置一次寫入 deadline，兩行分開
// 寫等於白付一次沒必要的 syscall／deadline reset。錯誤吞掉的慣例同
// writeAGUIFrame。
func writeAGUIFrameWithID(w io.Writer, id, data string) {
	_, _ = io.WriteString(w, "id: "+id+"\ndata: "+data+"\n\n")
}

// rawJSONOrString 把 ledger event 的 data_json 包成 CUSTOM frame 的
// `value`：合法 JSON 就原樣內嵌（json.RawMessage），避免 json.Marshal 這個
// []byte 又跑一次 base64；不合法（含空值——json.Valid(nil) 為 false）就退回
// 字串型別，理由同 ai_assistant_stream.go 的 StreamEvent_DraftPatch case：
// json.RawMessage 必須自己是合法 JSON 才能被外層 json.Marshal 接受，否則
// marshalAGUI 會靜默吞掉整個 marshal 錯誤、送出一行空的 `data: \n\n`，把這筆
// 事件整個弄丟而不留下任何錯誤訊號。
func rawJSONOrString(b []byte) any {
	if json.Valid(b) {
		return json.RawMessage(b)
	}
	return string(b)
}

// customSlugForRunEventType 對照 doc57 §4.2 的 wire carrier 欄，決定 CUSTOM
// frame 的 `name`。MESSAGE_COMPLETED 不在這裡——writeRunEvent 在呼叫這個函式
// 之前就已經把它導去 writeMessageCompletedRunEvent（原生 TEXT_MESSAGE_* 三個
// frame），不會落到這個 switch。
func customSlugForRunEventType(eventType string) string {
	switch eventType {
	// Run 家族的 CUSTOM 型別**各自具名**，不共用一個 `run_status` 桶。
	//
	// 判別欄位必須存在於某處：activity／step_detail／confirmation／artifact 四個
	// family 是靠自描述的 payload 帶著（例如 artifact 的 value 就是
	// {artifactId,type,version}），所以它們可以兩個型別共用一個 name。Run 家族沒有
	// 這個性質——這些事件的 public payload 多半是字面 `{}`（見四個 usecase 的
	// terminalPayload 預設值），共用一個 name 等於讓消費端結構上分不出是哪一種。
	//
	// RUN_FAILED 不在這裡：它走原生 RUN_ERROR（doc57 §4.2），見 writeRunFailedEvent。
	case codemeta.ProcurementLedgerEventTypeRunAccepted.Value:
		return "run_accepted"
	case codemeta.ProcurementLedgerEventTypeRunWaiting.Value:
		return "run_waiting"
	case codemeta.ProcurementLedgerEventTypeRunCancelRequested.Value:
		return "run_cancel_requested"
	case codemeta.ProcurementLedgerEventTypeRunCancelled.Value:
		return "run_cancelled"
	case codemeta.ProcurementLedgerEventTypeRunResumed.Value:
		return "run_resumed"
	case codemeta.ProcurementLedgerEventTypeRunExpired.Value:
		return "run_expired"
	case codemeta.ProcurementLedgerEventTypeStepStarted.Value, codemeta.ProcurementLedgerEventTypeStepFinished.Value:
		return "step_detail"
	case codemeta.ProcurementLedgerEventTypeAgentActivitySnapshot.Value, codemeta.ProcurementLedgerEventTypeAgentActivityDelta.Value:
		return "activity"
	case codemeta.ProcurementLedgerEventTypeArtifactUpserted.Value:
		return "artifact"
	case codemeta.ProcurementLedgerEventTypeConfirmationRequired.Value, codemeta.ProcurementLedgerEventTypeConfirmationResolved.Value:
		return "confirmation"
	default:
		// 目前只有 TOOL_STARTED／TOOL_FINISHED 會落到這裡：doc57 §4.2 標它們的
		// wire carrier 是原生 TOOL_CALL_*，但沒有任何 producer 真的寫出這兩個
		// 型別，也就沒有已知的 data_json 形狀可供正確拆成
		// TOOL_CALL_START/ARGS 或 TOOL_CALL_RESULT/END 兩三個 frame——硬做
		// 等於憑空猜一個從未被任何 producer／測試驗證過的 wire 形狀。退回
		// CUSTOM run_event 符合 doc57 §4.2 不變式 4（unknown optional event
		// 可略過並推進 cursor；這裡選擇「不略過、用最保守的通用殼傳遞」而非
		// 真的丟棄事件），等真正的 producer 落地時再回來補這個分支。
		return "run_event"
	}
}

// writeMessageCompletedRunEvent 把一筆 MESSAGE_COMPLETED ledger event 投影成
// 三個原生 AG-UI frame（doc57 §4.2 wire carrier 欄：TEXT_MESSAGE_START／
// CONTENT／END），比照 ai_assistant_stream.go writeStreamEvent 的
// StreamEvent_Message case——差別只在這裡外加 replay 專屬的 `id:` 行。
//
// `id:` 只掛在最後一個 frame（TEXT_MESSAGE_END）：三個 frame 來自同一筆
// ledger row／同一個 seq，若把 `id:` 也掛在 START／CONTENT 上，client 在收到
// START 之後、END 之前斷線重連時，瀏覽器的 Last-Event-ID 會已經前進到這個
// seq，導致下次重連時 server 端 `seq > cursor` 判斷這個事件「已送達」而跳過
// 整組重送——留下一個永遠沒有 END 的 TEXT_MESSAGE_START，訊息永遠不會收尾。
// 只在最後一個 frame 掛 `id:`，斷線點落在這個 group 中段時整組會被完整重送
// （client 需要能接受同一個 messageId 收到第二次 START/CONTENT/END，這跟 live
// 串流重連後的既有假設一致——reconnect 本來就可能重放尚未確認送達的資料）。
func writeMessageCompletedRunEvent(w io.Writer, ev *aipb.RunEvent) {
	var payload struct {
		MessageID string `json:"messageId"`
		Text      string `json:"text"`
	}
	if err := json.Unmarshal(ev.GetDataJson(), &payload); err != nil || payload.MessageID == "" {
		// 形狀跟預期不符（理論上不會發生：ai-service 端固定用
		// ledgerEventTypeFor 的 TurnEventResult 分支序列化）——退回 CUSTOM
		// run_event 而不是直接丟棄或讓整條 stream 中斷，理由同
		// customSlugForRunEventType 的 default 分支。
		writeAGUIFrameWithID(w, ev.GetSeq(), marshalAGUI(agUICustom{
			Type: "CUSTOM", Name: "run_event", Value: rawJSONOrString(ev.GetDataJson()),
		}))
		return
	}
	writeAGUIFrame(w, marshalAGUI(agUITextMessageStart{Type: "TEXT_MESSAGE_START", MessageID: payload.MessageID, Role: "assistant"}))
	writeAGUIFrame(w, marshalAGUI(agUITextMessageContent{Type: "TEXT_MESSAGE_CONTENT", MessageID: payload.MessageID, Delta: payload.Text}))
	writeAGUIFrameWithID(w, ev.GetSeq(), marshalAGUI(agUITextMessageEnd{Type: "TEXT_MESSAGE_END", MessageID: payload.MessageID}))
}

// writeRunEvent 把一筆 ledger RunEvent 投影成 AG-UI SSE frame（doc57 §2.2 的
// projector），與 ai_assistant_stream.go 的 writeStreamEvent 是姊妹函式但不
// 合併：writeStreamEvent 假設從頭到尾單向推進、輸入是 aipb.StreamEvent；這支
// 支援任意 cursor 起點的 replay、輸入是 aipb.RunEvent，狀態機不同（這支完全
// stateless——每筆事件不依賴前一筆的任何狀態，也不需要 agUIStreamState）。
//
// 刻意不回傳 error：跟 writeAGUIFrame／writeStreamEvent 同一個慣例——一筆
// data_json 形狀跟預期不符的事件不該讓整條 SSE 連線直接斷線（client 的
// EventSource 會用同一個 Last-Event-ID 立刻重連，若壞掉的是某個
// seq 本身的內容而不是暫時性錯誤，會變成無限重連死迴圈)，退回一個內容
// 保守但仍然送得出去的 frame 比讓連線消失更安全。
func writeRunEvent(w io.Writer, ev *aipb.RunEvent) {
	if ev.GetType() == codemeta.ProcurementLedgerEventTypeMessageCompleted.Value {
		writeMessageCompletedRunEvent(w, ev)
		return
	}

	if ev.GetType() == codemeta.ProcurementLedgerEventTypeRunFailed.Value {
		writeRunFailedEvent(w, ev)
		return
	}

	if runEventNativeTypes[ev.GetType()] {
		frame := map[string]any{}
		if len(ev.GetDataJson()) > 0 {
			// best-effort：解不出來就退回空 map，下面兩行仍會補上 type／runId
			// 兩個關鍵欄位，不會送出完全空白的 frame。
			_ = json.Unmarshal(ev.GetDataJson(), &frame)
		}
		frame["type"] = ev.GetType()
		// data_json 目前多數情況不含 runId（例如 RUN_FINISHED 終態 payload
		// 常是字面 `{}`，見 turn.go/answer.go/rewind.go 四個 usecase 的
		// terminalPayload 預設值）——這裡一律用 RunEvent 自帶、authoritative
		// 的 run_id 覆寫，確保原生 RUN_STARTED／RUN_FINISHED frame 永遠帶有
		// 正確的 runId，不依賴 producer 端 payload 是否剛好填了這個欄位。
		frame["runId"] = ev.GetRunId()
		// threadId 同理，而且它比 runId 更硬性：官方 AG-UI schema 把
		// RunStartedEvent／RunFinishedEvent 的 threadId 列為**必填**，缺了整個
		// frame 會在消費端的 zod 驗證被拒。這條路徑的 producer payload 恆是字面
		// `{}`，所以在本次修正之前，這個端點送出的原生 RUN_FINISHED 從來沒有通過
		// 過 client 的 schema 檢查——website 的 replay client 因此收不到終態，
		// 「run 結束就提早收線」在線上是死路徑（2026-09-06 由前端 review 追出來）。
		//
		// 值取 run 所屬的 conversation（RunEvent.conversation_id，由 ai-service
		// 從 run row 帶出）。POST turn stream 那條路徑目前傳的是 requestId
		// （`handler/ai.go` 的 ServeAIProcurementStream 呼叫點），兩者刻意不同：
		// thread 在語意上就是 conversation，而 run row 本來就存著它、每一種 run
		// type 都有值（doc57 §5.6）；要拿 requestId 反而得為 replay 另外造一條
		// 反查。沒有任何消費端跨 transport 對帳這個欄位（website 完全不讀它，
		// 只要求它存在且是字串），所以這個不一致目前不具行為意義——真要收斂，
		// 該收斂的是 POST 那條，屬另一輪。
		frame["threadId"] = ev.GetConversationId()
		writeAGUIFrameWithID(w, ev.GetSeq(), marshalAGUI(frame))
		return
	}

	writeAGUIFrameWithID(w, ev.GetSeq(), marshalAGUI(agUICustom{
		Type: "CUSTOM", Name: customSlugForRunEventType(ev.GetType()), Value: rawJSONOrString(ev.GetDataJson()),
	}))
}

// runFailedLedgerPayload 是 RUN_FAILED ledger event 的 public payload 形狀。
type runFailedLedgerPayload struct {
	FailureReason string `json:"failureReason"`
}

// runFailedFallbackMessage 在 payload 沒帶 failureReason（或帶了值域外的值）時
// 頂替。AG-UI 的 RunErrorEvent 把 `message` 列為必填 ZodString，缺了整個 frame 會
// 在消費端被 zod 拒絕、事件整個消失——所以這裡寧可送一句粗的，也不能不送。
const runFailedFallbackMessage = "任務執行失敗"

// writeRunFailedEvent 把 RUN_FAILED ledger event 投影成**原生** AG-UI RUN_ERROR
// （doc57 §4.2 Run 那一列：RUN_STARTED／RUN_FINISHED／RUN_ERROR 為原生）。
//
// 為什麼不是 CUSTOM：RUN_ERROR 是 AG-UI 協定裡「這個 run 以錯誤結束」的原生事件
// （@ag-ui/core 的 RunErrorEventSchema），不是傳輸層專用構造。把它塞進
// `CUSTOM name="run_status"` 是用一個沒有判別欄位的通用桶，去裝協定本來就有專屬
// 事件的東西——七種 run 事件全塌成同一個 name，而它們的 payload 又不自描述，
// 消費端因此結構上分不出「失敗」與「已接受」。
//
// 機器碼放 `reason` 不放 `code`：`code` 被 website 以 /^-?\d+$/ 過濾（見
// agUIRunError 的文件），本 repo 既有慣例是 UPPER_SNAKE 機器碼一律走 `reason`。
// 這個分工也順帶給了消費端一個天然的判準——**傳輸失敗**的 RUN_ERROR 帶 HTTP
// 字串 `code`（見上面 writeErrorFrame 的呼叫點），**業務失敗**帶 `reason`。
//
// 值域外的 failureReason 直接省略 reason，不憑空補一個機器碼：CodeMeta 是這個
// 值域的 SSOT，送一個不在值域裡的值等於讓消費端分流到一個不存在的分支。
func writeRunFailedEvent(w io.Writer, ev *aipb.RunEvent) {
	frame := agUIRunError{Type: "RUN_ERROR", Message: runFailedFallbackMessage}

	var payload runFailedLedgerPayload
	// best-effort：解不出來就用 fallback message，仍然送得出一個合法 frame。
	if err := json.Unmarshal(ev.GetDataJson(), &payload); err == nil {
		if reason, ok := codemeta.ProcurementRunFailureReasonFromCode(payload.FailureReason); ok {
			frame.Message = reason.Label
			frame.Reason = reason.Value
		}
	}
	writeAGUIFrameWithID(w, ev.GetSeq(), marshalAGUI(frame))
}

// replayDeadlineExceededMessage 在 streamTurnDeadline 於 replay/訂閱
// Recv() 仍卡住時觸發時，換掉 grpc-go 的字面 status.FromContextError 文字
// （"context deadline exceeded"）——理由與事故等級同
// serveAIAssistantStreamWith 的同名處理（見該檔 DeadlineExceeded 分支的
// 註解），但這個端點的補救動作是「重新連線」而不是「重送這句話」：一個新的
// GET（瀏覽器 EventSource 自動重連，或手動重新整理）會用同一個
// Last-Event-ID 開一個新的訂閱，續得上。
const replayDeadlineExceededMessage = "連線已逾時，請重新整理頁面以繼續接收更新"

// RunEventStream 收窄 gRPC ClientStream 到這個端點需要的最小介面，同
// AssistantStream 的既有手法（見 ai_assistant_client.go），刻意不 embed
// grpc.ClientStream：這個端點只收不送，一個真正的
// aipb.AIProcurementService_WatchRunEventsClient 在結構上自然滿足這個較窄的
// 介面，兩邊都不用互相 import 對方（AIProcurementClient.WatchRunEvents 回傳
// 生成型別，轉換發生在 handler 呼叫端）。
type RunEventStream interface {
	Recv() (*aipb.RunEvent, error)
}

// runEventRecvResult 把 serveAIProcurementRunEventsStreamWith 轉發 goroutine
// 的每一次 stream.Recv() 結果帶進它的 select 迴圈——形狀同
// ai_assistant_stream.go 的 streamRecvResult，事件型別不同（aipb.RunEvent
// vs aipb.StreamEvent）所以不共用。
type runEventRecvResult struct {
	event *aipb.RunEvent
	err   error
}

// ServeAIProcurementRunEventsStream 是 GET
// /buyers/me/procurement/runs/:runId/events 的 serving 邏輯：共用
// serveAIAssistantStreamWith 同一套 write/flush/keepalive/deadline 骨架
// （deadlineResponseWriter／sseKeepaliveInterval／streamTurnDeadline），輸入
// 換成 Task 11 的 WatchRunEvents 而非既有 AssistantStream，每筆成功 frame
// 多帶一個 `id:` 行（writeRunEvent）。
//
// open 由呼叫端（handler.ProxyProcurementRunEvents）提供，關住要呼叫哪一支
// RPC、帶哪個 run_id／request_id——這裡對此一無所知，只知道要把 lastEventID
// 轉成 afterSeq 參數傳進去（doc57 §2.4：Last-Event-ID 與 afterSeq 是同一個
// 十進位字串 cursor 語意，只是換了 transport 的名字）。
func ServeAIProcurementRunEventsStream(c *gin.Context, lastEventID string, open func(ctx context.Context, afterSeq string) (RunEventStream, error)) {
	serveAIProcurementRunEventsStreamWith(c, lastEventID, open, streamTurnDeadline)
}

// serveAIProcurementRunEventsStreamWith 是內部實作，多帶一個 turnDeadline
// 參數供測試注入——理由與 serveAIAssistantStreamWith 的同名參數完全相同（見
// 該函式文件註解）：正式路徑只會透過 ServeAIProcurementRunEventsStream 呼叫，
// 固定帶 streamTurnDeadline；測試可以注入一個極小值，不用真的等 120 秒。
func serveAIProcurementRunEventsStreamWith(c *gin.Context, lastEventID string, open func(ctx context.Context, afterSeq string) (RunEventStream, error), turnDeadline time.Duration) {
	// WithTimeout（不是 WithCancel）：仍然衍生自 c.Request.Context()，client
	// 斷線一樣會取消它，但多了一個硬上限——見 streamTurnDeadline 的文件註解。
	// 對這個端點而言，這個上限同時扮演「連線最長存活時間」的角色。
	//
	// 它**不再是**回收這條連線的唯一機制：ai-service 的 WatchRunEvents 現在會在
	// 「run 已終態且該 run 的事件都送完」時主動結束 stream，下面的 io.EOF 分支
	// 因此會安靜關閉 SSE（成功路徑從撐滿 120 秒變成終態 frame 後隨即收線）。
	// 這個上限留著兜的是另外幾種情形：run 遲遲不進終態、上游卡在別的地方、
	// 或 client 開著一條沒有下一筆事件可等的連線不放。
	ctx, cancel := context.WithTimeout(c.Request.Context(), turnDeadline)
	defer cancel()

	stream, err := open(ctx, lastEventID)
	if err != nil {
		// 跟 serveAIAssistantStreamWith 的 open 失敗分支一樣：這裡還沒寫過任何
		// 東西進 c.Writer（沒有 SSE header、沒有 body），所以一個一般的 HTTP
		// 錯誤回應仍然可能——用同一個 FailClientFacing（424）信封，理由同
		// ai_assistant_stream.go 的說明。
		writeUpstreamFailureEnvelope(c.Writer, c.Request, FailClientFacing, err)
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", response.CacheControlPrivateNoStore)
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// 每一次 Write 的 deadline——見 streamWriteTimeout。
	w := &deadlineResponseWriter{
		ResponseWriter: c.Writer,
		rc:             http.NewResponseController(c.Writer),
		timeout:        streamWriteTimeout,
	}

	// recvCh／轉發 goroutine 的設計與 serveAIAssistantStreamWith 完全相同
	// （含「為什麼是無條件阻塞 send 而不是額外 race 一個 ctx.Done() case」的
	// 理由），這裡不重複那段推理，見該檔案的同名註解。
	recvCh := make(chan runEventRecvResult, 1)
	go func() {
		for {
			event, err := stream.Recv()
			recvCh <- runEventRecvResult{event: event, err: err}
			if err != nil {
				return
			}
		}
	}()

	pingTicker := time.NewTicker(sseKeepaliveInterval)
	defer pingTicker.Stop()

	for {
		select {
		case res := <-recvCh:
			if res.err != nil {
				// io.EOF：ai-service 的 WatchRunEvents 正常結束這個 stream
				// （client 斷線、或 wake channel 關閉——doc57 §2.5-8：只
				// abort subscription，不是錯誤）。安靜結束，不送 RUN_ERROR
				// 等價的 frame。
				if !errors.Is(res.err, io.EOF) {
					st := status.Convert(res.err)
					message := st.Message()
					if st.Code() == codes.DeadlineExceeded {
						message = replayDeadlineExceededMessage
					}
					// SSE header 已經送出（200 已提交），只能走既有的
					// procurement 錯誤碼 bucket 表把 gRPC code 轉成 in-band
					// 的 `code`——跟 ServeAIProcurementStream（既有的 POST
					// /turns/stream 端點）同一個既有慣例：RUN_ERROR-等價
					// frame 的 code 帶 HTTP status 字串而不是原始 gRPC
					// code，兩個消費端（website 的 err.code 分流／本次的
					// REPLAY_CURSOR_EXPIRED→410）才能共用同一張表。
					writeErrorFrame(w, &agUIStreamState{}, procurementErrorFrameCode(st.Code()), message, procurementwire.BusinessErrorFromStatus(st))
					c.Writer.Flush()
				}
				return
			}
			writeRunEvent(w, res.event)
			c.Writer.Flush()
		case <-pingTicker.C:
			_, _ = io.WriteString(w, ": ping\n\n")
			c.Writer.Flush()
		}
	}
}
