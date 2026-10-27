package grpc

import (
	"context"
	"errors"
	"testing"

	pb "github.com/Will413028/hawkr/go-pkg/proto/ai"
)

type fakeStreamSender struct {
	sent    []*pb.StreamEvent
	failAt  int // -1 = 不失敗
	callNum int
}

func (f *fakeStreamSender) Send(ev *pb.StreamEvent) error {
	f.callNum++
	if f.failAt >= 0 && f.callNum == f.failAt {
		return errors.New("send failed")
	}
	f.sent = append(f.sent, ev)
	return nil
}

func (f *fakeStreamSender) Context() context.Context { return context.Background() }

func TestGRPCEventSink_Text(t *testing.T) {
	fake := &fakeStreamSender{failAt: -1}
	sink := NewGRPCEventSink(fake)
	sink.Text("hello")

	if len(fake.sent) != 1 {
		t.Fatalf("sent %d events, want 1", len(fake.sent))
	}
	got := fake.sent[0].GetText().GetText()
	if got != "hello" {
		t.Fatalf("text = %q, want %q", got, "hello")
	}
	if sink.Err() != nil {
		t.Fatalf("unexpected Err(): %v", sink.Err())
	}
}

func TestGRPCEventSink_Done(t *testing.T) {
	fake := &fakeStreamSender{failAt: -1}
	sink := NewGRPCEventSink(fake)
	sink.Done("sess-123")

	got := fake.sent[0].GetDone().GetSessionId()
	if got != "sess-123" {
		t.Fatalf("session_id = %q, want %q", got, "sess-123")
	}
}

// TestGRPCEventSink_OneOffEvents 是 DraftPatch／ToolResult／Error 三個 oneof
// variant 的 table-driven 覆蓋——Text／Done 已有各自獨立的測試（見上），這裡
// 補齊 GRPCEventSink 產生的 5 個 StreamEvent oneof variant 中剩下的 3 個，
// 確保每個 variant 的具體型別與欄位值都被斷言到，而不只是「有送出東西」。
func TestGRPCEventSink_OneOffEvents(t *testing.T) {
	tests := []struct {
		name  string
		call  func(sink *GRPCEventSink)
		check func(t *testing.T, ev *pb.StreamEvent)
	}{
		{
			name: "DraftPatch",
			call: func(sink *GRPCEventSink) { sink.DraftPatch(`{"name":"apple"}`) },
			check: func(t *testing.T, ev *pb.StreamEvent) {
				t.Helper()
				payload, ok := ev.GetPayload().(*pb.StreamEvent_DraftPatch)
				if !ok {
					t.Fatalf("payload type = %T, want *pb.StreamEvent_DraftPatch", ev.GetPayload())
				}
				if got := payload.DraftPatch.GetPatchJson(); got != `{"name":"apple"}` {
					t.Fatalf("patch_json = %q, want %q", got, `{"name":"apple"}`)
				}
			},
		},
		{
			name: "ToolResult",
			call: func(sink *GRPCEventSink) { sink.ToolResult(`{"orders":[]}`) },
			check: func(t *testing.T, ev *pb.StreamEvent) {
				t.Helper()
				payload, ok := ev.GetPayload().(*pb.StreamEvent_ToolResult)
				if !ok {
					t.Fatalf("payload type = %T, want *pb.StreamEvent_ToolResult", ev.GetPayload())
				}
				if got := payload.ToolResult.GetResultJson(); got != `{"orders":[]}` {
					t.Fatalf("result_json = %q, want %q", got, `{"orders":[]}`)
				}
			},
		},
		{
			name: "Error",
			call: func(sink *GRPCEventSink) { sink.Error(500, "internal error") },
			check: func(t *testing.T, ev *pb.StreamEvent) {
				t.Helper()
				payload, ok := ev.GetPayload().(*pb.StreamEvent_Error)
				if !ok {
					t.Fatalf("payload type = %T, want *pb.StreamEvent_Error", ev.GetPayload())
				}
				if got := payload.Error.GetCode(); got != 500 {
					t.Fatalf("code = %d, want 500", got)
				}
				if got := payload.Error.GetMessage(); got != "internal error" {
					t.Fatalf("message = %q, want %q", got, "internal error")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeStreamSender{failAt: -1}
			sink := NewGRPCEventSink(fake)
			tt.call(sink)

			if len(fake.sent) != 1 {
				t.Fatalf("sent %d events, want 1", len(fake.sent))
			}
			tt.check(t, fake.sent[0])
			if sink.Err() != nil {
				t.Fatalf("unexpected Err(): %v", sink.Err())
			}
		})
	}
}

func TestGRPCEventSink_StopsAfterSendFailure(t *testing.T) {
	fake := &fakeStreamSender{failAt: 1} // 第一次 Send 就失敗
	sink := NewGRPCEventSink(fake)
	sink.Text("first")
	sink.Text("second")

	if sink.Err() == nil {
		t.Fatal("expected non-nil Err() after Send failure")
	}
	if len(fake.sent) != 0 {
		t.Fatalf("sent %d events after failure, want 0 (second call should be a no-op)", len(fake.sent))
	}
}
