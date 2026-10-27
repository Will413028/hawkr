package proxy

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAGUIEventStructs_MarshalExpectedShape(t *testing.T) {
	cases := []struct {
		name string
		v    any
		want map[string]any
	}{
		{"RunStarted", agUIRunStarted{Type: "RUN_STARTED", ThreadID: "t1", RunID: "r1"},
			map[string]any{"type": "RUN_STARTED", "threadId": "t1", "runId": "r1"}},
		{"RunFinished", agUIRunFinished{Type: "RUN_FINISHED", ThreadID: "t1", RunID: "r1", Result: map[string]string{"sessionId": "s1"}},
			map[string]any{"type": "RUN_FINISHED", "threadId": "t1", "runId": "r1", "result": map[string]any{"sessionId": "s1"}}},
		{"RunError", agUIRunError{Type: "RUN_ERROR", Message: "boom", Code: "500"},
			map[string]any{"type": "RUN_ERROR", "message": "boom", "code": "500"}},
		{"StepStarted", agUIStepStarted{Type: "STEP_STARTED", StepName: "EXTRACT"},
			map[string]any{"type": "STEP_STARTED", "stepName": "EXTRACT"}},
		{"StepFinished", agUIStepFinished{Type: "STEP_FINISHED", StepName: "EXTRACT"},
			map[string]any{"type": "STEP_FINISHED", "stepName": "EXTRACT"}},
		{"TextMessageStart", agUITextMessageStart{Type: "TEXT_MESSAGE_START", MessageID: "m1", Role: "assistant"},
			map[string]any{"type": "TEXT_MESSAGE_START", "messageId": "m1", "role": "assistant"}},
		{"TextMessageContent", agUITextMessageContent{Type: "TEXT_MESSAGE_CONTENT", MessageID: "m1", Delta: "hi"},
			map[string]any{"type": "TEXT_MESSAGE_CONTENT", "messageId": "m1", "delta": "hi"}},
		{"TextMessageEnd", agUITextMessageEnd{Type: "TEXT_MESSAGE_END", MessageID: "m1"},
			map[string]any{"type": "TEXT_MESSAGE_END", "messageId": "m1"}},
		{"ToolCallStart", agUIToolCallStart{Type: "TOOL_CALL_START", ToolCallID: "tc1", ToolCallName: "tool_result"},
			map[string]any{"type": "TOOL_CALL_START", "toolCallId": "tc1", "toolCallName": "tool_result"}},
		{"ToolCallResult", agUIToolCallResult{Type: "TOOL_CALL_RESULT", MessageID: "m1", ToolCallID: "tc1", Content: "{}"},
			map[string]any{"type": "TOOL_CALL_RESULT", "messageId": "m1", "toolCallId": "tc1", "content": "{}"}},
		{"StateSnapshot", agUIStateSnapshot{Type: "STATE_SNAPSHOT", Snapshot: map[string]int{"a": 1}},
			map[string]any{"type": "STATE_SNAPSHOT", "snapshot": map[string]any{"a": float64(1)}}},
		{"Custom", agUICustom{Type: "CUSTOM", Name: "step_detail", Value: map[string]string{"x": "y"}},
			map[string]any{"type": "CUSTOM", "name": "step_detail", "value": map[string]any{"x": "y"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.v)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var got map[string]any
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("key count mismatch: got %v want %v", got, tc.want)
			}
			for k, wantV := range tc.want {
				gotV, ok := got[k]
				if !ok {
					t.Fatalf("missing key %q in %v", k, got)
				}
				gotJSON, _ := json.Marshal(gotV)
				wantJSON, _ := json.Marshal(wantV)
				if string(gotJSON) != string(wantJSON) {
					t.Fatalf("key %q: got %s want %s", k, gotJSON, wantJSON)
				}
			}
		})
	}
}

func TestAGUIRunFinished_OmitsResultWhenNil(t *testing.T) {
	data, _ := json.Marshal(agUIRunFinished{Type: "RUN_FINISHED", ThreadID: "t1", RunID: "r1"})
	var got map[string]any
	json.Unmarshal(data, &got)
	if _, ok := got["result"]; ok {
		t.Fatalf("result should be omitted when nil, got %s", data)
	}
}

func TestAGUIRunError_OmitsCodeWhenEmpty(t *testing.T) {
	data, _ := json.Marshal(agUIRunError{Type: "RUN_ERROR", Message: "boom"})
	var got map[string]any
	json.Unmarshal(data, &got)
	if _, ok := got["code"]; ok {
		t.Fatalf("code should be omitted when empty, got %s", data)
	}
}

func TestWriteAGUIFrame_NoEventLine(t *testing.T) {
	var sb strings.Builder
	writeAGUIFrame(&sb, `{"type":"RUN_STARTED"}`)
	const want = "data: {\"type\":\"RUN_STARTED\"}\n\n"
	if got := sb.String(); got != want {
		t.Fatalf("frame mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestMarshalAGUI_ProducesJSONString(t *testing.T) {
	ev := agUIRunStarted{Type: "RUN_STARTED", ThreadID: "t1", RunID: "r1"}
	got := marshalAGUI(ev)
	const want = `{"type":"RUN_STARTED","threadId":"t1","runId":"r1"}`
	if got != want {
		t.Fatalf("marshal: got %q want %q", got, want)
	}
}
