package proxy

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"

	procurementwire "github.com/Will413028/hawkr/gateway/internal/procurement"
)

// ptr 只在本檔案的 fixture 建構中使用，把值取址成 SlotsDTO／ClarifyQuestionDTO
// 要求的指標欄位；不是給 production code 用的共用 helper。
func ptr[T any](v T) *T { return &v }

var updateGolden = flag.Bool("update-agui-golden", false, "重新產生 testdata/agui-frames 下的 fixture")

// TestAGUIGoldenFrames 產生一組 canonical 的 AG-UI SSE frame，並與版控的
// fixture 逐位元組比對。
//
// 這些 fixture 不只是本套件的迴歸測試——它們會被拷貝到 hawkr-website，
// 由那邊的測試餵給 @ag-ui/core 的官方 zod schema 驗證。所以「改了 encoder
// 沒更新 fixture」與「改了 encoder 但新格式不合 AG-UI spec」是兩道獨立的
// 關卡，前者在這裡擋，後者在 website 那邊擋。
func TestAGUIGoldenFrames(t *testing.T) {
	cases := []struct {
		name   string
		frames []any
	}{
		{"run-lifecycle", []any{
			agUIRunStarted{Type: "RUN_STARTED", ThreadID: "thread-golden", RunID: "run-golden"},
			agUITextMessageStart{Type: "TEXT_MESSAGE_START", MessageID: "msg-golden", Role: "assistant"},
			agUITextMessageContent{Type: "TEXT_MESSAGE_CONTENT", MessageID: "msg-golden", Delta: "嗨"},
			agUITextMessageContent{Type: "TEXT_MESSAGE_CONTENT", MessageID: "msg-golden", Delta: "，這是測試"},
			agUITextMessageEnd{Type: "TEXT_MESSAGE_END", MessageID: "msg-golden"},
			agUIRunFinished{Type: "RUN_FINISHED", ThreadID: "thread-golden", RunID: "run-golden"},
		}},
		{"run-error", []any{
			agUIRunStarted{Type: "RUN_STARTED", ThreadID: "thread-golden", RunID: "run-golden"},
			agUIRunError{Type: "RUN_ERROR", Message: "上游服務暫時無法使用", Code: "14"},
		}},
		{"tool-call", []any{
			agUIToolCallStart{Type: "TOOL_CALL_START", ToolCallID: "call-golden", ToolCallName: "get_my_orders"},
			agUIToolCallEnd{Type: "TOOL_CALL_END", ToolCallID: "call-golden"},
			agUIToolCallResult{Type: "TOOL_CALL_RESULT", MessageID: "msg-golden", ToolCallID: "call-golden", Content: `{"orders":[]}`},
		}},
		// custom-step-detail 刻意用真正的 stepDetailPayload struct（定義於
		// ai_assistant_stream.go:360-367），不是 map[string]any——production
		// 就是送這個 struct。Go 對 struct 依宣告序輸出 JSON key，對
		// map[string]any 則依字母序，兩者位元組不同。用 map 會把 fixture
		// 釘在 production 永遠不會送出的形狀上，日後 stepDetailPayload
		// 改壞時這條 golden test 也不會轉紅。
		//
		// Code／Label 是真實值域成員，不是隨手編的字面值：出處
		// services/ai-service/internal/procurement/domain/steps.go 的
		// StepExtractSlots 常數（"EXTRACT_SLOTS"）與 StepLabels[StepExtractSlots]
		// 這張固定字串表（"抓出品項、數量和交期"）。刻意不 import 該 package——
		// gateway 不該依賴 ai-service 的 domain package，兩者是不同模組；改用
		// 字面字串複製這兩個值，值域變動時要回來這裡同步。
		//
		// Status 用 "SUCCEEDED" 而非 "RUNNING"：2026-08-21 對 dev 實測，
		// durationMs 只會出現在終態 frame，RUNNING 的 step_detail 完全不帶這個
		// 欄位（stepDetailPayload.DurationMs 有 omitempty）。先前用 RUNNING 搭
		// 非零 DurationMs 是為了讓 omitempty 不吃掉欄位、確保它有被釘住，但那個
		// 組合是 production 不會送的混種。改成 SUCCEEDED 後兩個目的同時達成：
		// 欄位仍被釘住，而且是真實流量會送的組合。
		{"custom-step-detail", []any{
			agUICustom{Type: "CUSTOM", Name: "step_detail", Value: stepDetailPayload{
				TurnID: "turn-golden", Seq: 0, Code: "EXTRACT_SLOTS",
				Status: "SUCCEEDED", Label: "抓出品項、數量和交期", DurationMs: 1234,
			}},
		}},
		// custom-step-detail-with-safe-args 釘住 T11 前端早鳥版加的 safeArgs
		// 欄位（見 domain.Step.SafeArgs／StepDTO.SafeArgs）：只在少數 code
		// 上非空，序列化順序排在 durationMs 之後（依 struct 宣告序）。
		{"custom-step-detail-with-safe-args", []any{
			agUICustom{Type: "CUSTOM", Name: "step_detail", Value: stepDetailPayload{
				TurnID: "turn-golden", Seq: 0, Code: "EXTRACT_SLOTS",
				Status: "SUCCEEDED", Label: "抓出品項、數量和交期", DurationMs: 1234,
				SafeArgs: map[string]string{"item": "高麗菜", "city": "台中市"},
			}},
		}},
		// custom-activity 釘住 graph 路徑的 Activity frame（POST 串流的
		// StreamEvent_Activity 與 GET replay 的 AGENT_ACTIVITY_* ledger event
		// 送的是同一種 frame——doc57 §2.1）。
		//
		// ⚠️ 這個 case 守的範圍要講清楚，不要高估：它跟其他 case 一樣自己建構
		// agUICustom、自己呼叫 rawJSONOrString，**不經過 writeStreamEvent**。
		// 所以它釘住的是「rawJSONOrString + marshalAGUI + writeAGUIFrame 這條
		// 編碼路徑的位元組輸出」以及送去 website 給 @ag-ui/core schema 驗證的
		// 那份拷貝——**不是** production 的 mapping。把 writeStreamEvent 的
		// StreamEvent_Activity case 整個刪掉，或改成走 protojson 讓 value 變成
		// base64，這支測試都仍然全綠；那兩種迴歸是由
		// ai_assistant_stream_test.go 的 TestWriteStreamEvent_Activity_* 擋的。
		//
		// 反過來說這個 case 仍有它獨立的價值：rawJSONOrString 的「合法 JSON 就
		// 原樣內嵌、不再 marshal 一次」規則若被改壞（例如有人把 json.RawMessage
		// 換成 []byte，value 就會變成 base64），這裡會轉紅，而且會在 website
		// 端的 schema 驗證之前先擋下。
		//
		// payload 的欄位與型別（rowId／retryGroupKey 是十進位字串、attempt／
		// maxAttempts 是數字）依 api/proto/ai/ai.proto 的 ActivityEvent 註解，
		// 由 ai-service 逐字透傳；gateway 不重新 marshal，所以這個 literal 的
		// key 順序就是它被原樣 forward 出去的順序。但這裡的 rowId-first 順序
		// 是這個 case 手寫湊出來的，不是 production 真正送來的順序：
		// production 端 ledgerEventTypeForActivity（services/ai-service/
		// internal/procurement/usecase/graph_dispatch.go）marshal 的是
		// map[string]any，Go 的 encoding/json 對 map key 做字母排序，真正的
		// bytes 是 {"attempt":1,"code":"EXTRACT_SLOTS","maxAttempts":3,
		// "retryGroupKey":"7489","rowId":"7489","text":"..."}。key 順序在
		// 這裡沒有語意——前端按 key 讀，@ag-ui/core schema 驗證也不管順序——
		// 這個 fixture 的順序不能當成 upstream 位元組序的保證，不要在它上面
		// 疊加 byte-order 假設或 capture-and-compare 測試。
		//
		// 2026-09-06：literal 改用**字母序**，也就是 production 真正送出的順序
		// （ledgerEventTypeForActivity marshal 的是 map[string]any，Go 對 map key
		// 做字母排序）。原本的手寫 rowId-first 順序是 production 不會送的形狀，
		// 上一段警語正是為此而寫；照抄真實順序後那個落差就消失了。警語本身留著
		// ——順序仍然沒有語意，仍然不該被拿來當 byte-order 保證。
		//
		// 同一天補上 label／status 兩欄：T23（PR #3157）之後這兩欄恆存在
		// （ai.proto 的 ActivityEvent 註解：label 恆非空、status 恆非空）。fixture
		// 停在舊的六欄形狀時，跨 repo 逐位元組閘門與 website 的 schema 驗證都會
		// 過，卻對這兩欄零覆蓋。
		{"custom-activity", []any{
			agUICustom{Type: "CUSTOM", Name: "activity", Value: rawJSONOrString([]byte(
				`{"attempt":1,"code":"EXTRACT_SLOTS","label":"抓出品項、數量和交期","maxAttempts":3,"retryGroupKey":"7489","rowId":"7489","status":"SUCCEEDED","text":"整理你的採購條件"}`))},
		}},
		// custom-activity-candidate-search-rail 釘住 T23 的第二個 Activity
		// producer：候選搜尋 run（candidate_search_rail.go，脫離 ADK graph 的
		// 背景 goroutine，直接寫 ledger）。它跟上面那個 graph 路徑 case 的差別
		// 就是設計稿 01 右欄畫出來的那三件事，三者在既有 fixture 上都零覆蓋：
		//
		//  1. `text` 恆為空——這條 run 整趟沒有 LLM 呼叫，沒有 StepNarrator，
		//     所以沒有「這一次實際發生了什麼」可寫。consumer 必須據此顯示
		//     `label`（ai.proto：「空的時候 consumer 顯示 label，不是反過來」）。
		//     website 端舊消費端把 `text == ""` 當成「還在跑」，這三列會因此永遠
		//     停在進行中——這個 fixture 就是那條迴歸的證據面。
		//  2. `parentRowId`：兩條 lane 掛在「搜尋候選供應商」底下，rail 只支援
		//     一層（candidate_search_test.go 釘住父列不得帶這個欄位）。
		//  3. `count`：只有真的有計數概念的列才有這個欄位。**缺席 ≠ 0**——
		//     PLATFORM 查到 0 家時送 `count:"0"`，而父列與 SUMMARIZE 是整個欄位
		//     不存在（graph_dispatch.go：補 "0"／"" 會讓 consumer 分不出兩者）。
		//     所以這三個 frame 刻意只有中間那個帶 count。
		//
		// 值域全部取自 production：Attempt／MaxAttempts 恆為 1／1、RetryGroupKey
		// 等於自己的 RowID（見 candidate_search_rail.go 的 emit）；label 三則各自
		// 是 domain.StepLabels[SEARCH_CANDIDATES／SEARCH_PLATFORM／SUMMARIZE]。
		// 同 custom-step-detail，這裡刻意不 import ai-service 的 domain package
		// （gateway 不該依賴它），值域變動時要回來這裡同步。
		//
		// SUMMARIZE 那則是 PENDING：候選搜尋是固定 pipeline，run 開跑時就把全清單
		// 宣告出去（T23 D2），所以「還沒跑到的未來步驟」是真的會上線的狀態值，
		// 不是這裡編出來的。
		{"custom-activity-candidate-search-rail", []any{
			agUICustom{Type: "CUSTOM", Name: "activity", Value: rawJSONOrString([]byte(
				`{"attempt":1,"code":"SEARCH_CANDIDATES","label":"搜尋候選供應商","maxAttempts":1,"retryGroupKey":"9001","rowId":"9001","status":"RUNNING","text":""}`))},
			agUICustom{Type: "CUSTOM", Name: "activity", Value: rawJSONOrString([]byte(
				`{"attempt":1,"code":"SEARCH_PLATFORM","count":"12","label":"翻平台上的賣家","maxAttempts":1,"parentRowId":"9001","retryGroupKey":"9002","rowId":"9002","status":"SUCCEEDED","text":""}`))},
			agUICustom{Type: "CUSTOM", Name: "activity", Value: rawJSONOrString([]byte(
				`{"attempt":1,"code":"SUMMARIZE","label":"整理成給你的建議","maxAttempts":1,"retryGroupKey":"9004","rowId":"9004","status":"PENDING","text":""}`))},
		}},
		// procurement-turn-result 涵蓋 ai_assistant_stream.go 的
		// StreamEvent_TurnResult 分支（見該檔 case *aipb.StreamEvent_TurnResult）
		// ——production 對這個分支是「成對送出、payload 相同」：先送
		// STATE_SNAPSHOT（snapshot 欄位），再送 RUN_FINISHED（result 欄位），
		// 兩者攜帶同一個 procurementwire.TurnResultFromProto 結果。四個既有
		// case 都沒碰到這條路徑，改壞它不會有任何 gate 轉紅。
		//
		// result 只建構一次、被兩個 frame 共用，才釘得住「兩個 frame 帶同一份
		// payload」這個契約本身，而不只是各自巧合地長一樣。
		//
		// payload 用真正的 procurementwire.TurnResultDTO（定義於
		// internal/procurement/wire.go），不是 map[string]any——原因與
		// custom-step-detail 相同：Go struct 依宣告序輸出 JSON key，map 依
		// 字母序，用 map 會把 fixture 釘在 production 不會送的形狀上。
		// SlotsDTO 的欄位全是指標（*string／*int／*bool，帶 omitempty），用
		// package-level 的 ptr() helper 取址；未提供的欄位（packaging、
		// budget_per_unit、delivery_date）維持 nil，omitempty 會讓它們不出現，
		// 這也是 dev 實測到的真實形狀。MissingRequired 與 Question.Options
		// 都給非 nil 的空 slice（[]string{}／[]procurementwire.QuestionOptionDTO{}），
		// 因為兩者的 json tag 都沒有 omitempty——nil slice 會序列化成
		// null，但 dev 實測到的真實流量送的是 []。
		{"procurement-turn-result", func() []any {
			result := procurementwire.TurnResultDTO{
				RequestID:      "request-golden",
				ConversationID: "conversation-golden",
				RevisionID:     "revision-golden",
				Slots: procurementwire.SlotsDTO{
					Item:               ptr("高麗菜"),
					Quantity:           ptr(20),
					Unit:               ptr("KG"),
					DeliveryCity:       ptr("台中市"),
					ExactMatchRequired: ptr(true),
				},
				Rejected: []procurementwire.RejectedFieldDTO{
					{Key: "delivery_date", Reason: "日期格式需為 YYYY-MM-DD"},
				},
				MissingRequired: []string{},
				RecordedThisTurn: []procurementwire.RecordedFieldDTO{
					{Key: "item", Label: "品項", Value: "高麗菜"},
					{Key: "quantity", Label: "數量", Value: "20"},
				},
				Degraded: false,
				Reply:    "好的，高麗菜和臺中都有囉！",
				Steps: []procurementwire.StepDTO{
					{Code: "UNDERSTAND_INPUT", Status: "SUCCEEDED", Label: "讀你寫的需求", DurationMs: 37},
					{Code: "EXTRACT_SLOTS", Status: "SUCCEEDED", Label: "抓出品項、數量和交期", DurationMs: 14295},
				},
				Question: &procurementwire.ClarifyQuestionDTO{
					SlotKey:       "delivery_date",
					Prompt:        "希望什麼時候送達？",
					Options:       []procurementwire.QuestionOptionDTO{},
					AllowFreeText: true,
					Source:        "FIXED",
					CanSkip:       false,
				},
			}
			return []any{
				agUIStateSnapshot{Type: "STATE_SNAPSHOT", Snapshot: result},
				agUIRunFinished{Type: "RUN_FINISHED", ThreadID: "thread-golden", RunID: "run-golden", Result: result},
			}
		}()},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			for _, f := range tc.frames {
				writeAGUIFrame(&buf, marshalAGUI(f))
			}
			path := filepath.Join("testdata", "agui-frames", tc.name+".sse")

			if *updateGolden {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}

			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("讀不到 fixture（第一次請跑 go test -run TestAGUIGoldenFrames -update-agui-golden）：%v", err)
			}
			if !bytes.Equal(want, buf.Bytes()) {
				t.Fatalf("frame 與 fixture 不符。\n got: %q\nwant: %q\n"+
					"若這是預期的格式變更：跑 -update-agui-golden 重新產生"+
					"（website 的測試直接讀這份 fixture）。",
					buf.String(), string(want))
			}
		})
	}
}
