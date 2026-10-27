package proxy

import (
	"encoding/json"
	"fmt"
	"io"
)

// AG-UI（Agent-User Interaction Protocol）事件型別。欄位與 json tag 對齊
// docs.ag-ui.com/sdk/js/core/events 的官方定義，2026-08-17 查證。
//
// SSE 沒有 `event:` 這一行——查證官方 TS encoder
// （sdks/typescript/packages/encoder/src/encoder.ts）：
//
//	encodeSSE(event) { return `data: ${JSON.stringify(event)}\n\n` }
//
// 接收端純靠這裡的 `Type` 欄位分派，所以每個 struct 都把它放第一個欄位。

type agUIRunStarted struct {
	Type     string `json:"type"`
	ThreadID string `json:"threadId"`
	RunID    string `json:"runId"`
}

type agUIRunFinished struct {
	Type     string          `json:"type"`
	ThreadID string          `json:"threadId"`
	RunID    string          `json:"runId"`
	Result   any             `json:"result,omitempty"`
	Outcome  *agUIRunOutcome `json:"outcome,omitempty"`
}

// agUIRunOutcome mirrors the official @ag-ui/core RunFinishedOutcomeSchema
// (doc57 §9.3): a strict discriminated union on "type" —
// {"type":"success"} or {"type":"interrupt","interrupts":[{"id":...,"reason":...}]}.
// interrupts is a wrapped array, not a bare interruptId/reason pair, and both
// branches are zod `strict` mode on the real schema, so any unrecognized key
// fails client-side validation. Only the procurement StreamEvent_TurnResult
// path (ai_assistant_stream.go) populates it — every other agUIRunFinished
// construction (the 12 AIAssistantService SSE endpoints' StreamEvent_Done
// case) leaves it nil, and the pointer+omitempty pair
// here means those wire shapes stay byte-identical to before this field
// existed: no "outcome" key at all, not even an empty one. That's why this is
// a *agUIRunOutcome, not a plain agUIRunOutcome (encoding/json's omitempty
// never omits a zero-value struct — only a nil pointer).
type agUIRunOutcome struct {
	Type       string               `json:"type"` // "success" | "interrupt"
	Interrupts []agUIInterruptEntry `json:"interrupts,omitempty"`
}

// agUIInterruptEntry is one element of agUIRunOutcome.Interrupts. The official
// schema also allows optional message/toolCallId/responseSchema/expiresAt/
// metadata fields here; doc57's v1 design has no use for them, so they're
// deliberately not modeled. v1 always emits exactly one entry (at most one
// open interrupt per run, per doc57's ledger design) — this is not
// multi-interrupt support, just the schema's required array wrapper.
type agUIInterruptEntry struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

// agUIRunError 的 Reason／Remaining／ResetAt 是官方 RunErrorEvent 以外的加法欄位。
// ServeAIProcurementStream 的註解已經先寫下這條規矩：Code 被 website
// （lib/ai-chat/stream-client.ts）以 /^-?\d+$/ 過濾，任何機器碼（reason／
// UPPER_SNAKE）都必須放進**新欄位**，不能塞進 Code。這裡就是那個新欄位，外加
// doc 57 §4.7 要求 AGENT_MONTHLY_QUOTA_EXCEEDED 一併帶出的 remaining／resetAt。
//
// 三者都 omitempty，所以只有帶 BusinessError detail 的 procurement 錯誤會多出這
// 幾個鍵；12 支 AIAssistantService streaming 端點與傳輸層失敗送出的 RUN_ERROR
// 位元組完全不變（testdata/agui-frames/run-error.sse 這條 golden fixture 因此
// 不需要重產，website 端的 @ag-ui/core zod 驗證也不受影響）。
type agUIRunError struct {
	Type      string `json:"type"`
	Message   string `json:"message"`
	Code      string `json:"code,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Remaining string `json:"remaining,omitempty"`
	ResetAt   string `json:"resetAt,omitempty"`
}

type agUIStepStarted struct {
	Type     string `json:"type"`
	StepName string `json:"stepName"`
}

type agUIStepFinished struct {
	Type     string `json:"type"`
	StepName string `json:"stepName"`
}

type agUITextMessageStart struct {
	Type      string `json:"type"`
	MessageID string `json:"messageId"`
	Role      string `json:"role"`
}

type agUITextMessageContent struct {
	Type      string `json:"type"`
	MessageID string `json:"messageId"`
	Delta     string `json:"delta"`
}

type agUITextMessageEnd struct {
	Type      string `json:"type"`
	MessageID string `json:"messageId"`
}

type agUIToolCallStart struct {
	Type         string `json:"type"`
	ToolCallID   string `json:"toolCallId"`
	ToolCallName string `json:"toolCallName"`
}

type agUIToolCallEnd struct {
	Type       string `json:"type"`
	ToolCallID string `json:"toolCallId"`
}

type agUIToolCallResult struct {
	Type       string `json:"type"`
	MessageID  string `json:"messageId"`
	ToolCallID string `json:"toolCallId"`
	Content    string `json:"content"`
}

type agUIStateSnapshot struct {
	Type     string `json:"type"`
	Snapshot any    `json:"snapshot"`
}

type agUICustom struct {
	Type  string `json:"type"`
	Name  string `json:"name"`
	Value any    `json:"value"`
}

// marshalAGUI marshals an AG-UI event struct to its JSON string form. Errors
// are ignored the same way the rest of this package already does for its
// hand-rolled payload struct (stepDetailPayload, in ai_assistant_stream.go) —
// every value passed in is one of the structs above built from data this
// package already controls, so a marshal failure is not a runtime condition.
func marshalAGUI(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}

// writeAGUIFrame emits one AG-UI SSE frame: "data: <json>\n\n" — no `event:`
// line (see doc comment above). Safe to pass marshalAGUI's output directly:
// encoding/json escapes newlines inside string values as the two bytes
// "\n", so a marshaled event is always exactly one physical line.
func writeAGUIFrame(w io.Writer, data string) {
	_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
}
