package grpc

import (
	"context"

	"github.com/Will413028/hawkr/ai-service/internal/handler"
	pb "github.com/Will413028/hawkr/go-pkg/proto/ai"
)

// grpcStreamSender 是所有 AIAssistantService streaming RPC 的 server-stream 共同子集合
// （每一支 pb.AIAssistantService_XxxServer 都滿足這個介面，Go 的 structural typing
// 讓呼叫端可以用同一個具體型別包住不同的 RPC stream）。Context() 讓
// assistant_handler.go 的 genericStreamRPC 能取出 method-policy interceptor 解析好
// 寫入 context 的 actor，不需要呼叫端額外傳入 ctx——每一支
// pb.AIAssistantService_XxxServer 都內嵌 grpc.ServerStream，本來就有 Context()，
// 這裡只是縮小過的介面宣告，不影響任何實際傳入值。
type grpcStreamSender interface {
	Send(*pb.StreamEvent) error
	Context() context.Context
}

// GRPCEventSink 把 streamRunner 的事件轉譯目標接到 gRPC server-stream，取代 HTTP
// 版的 sseEventSink。sendErr 記錄第一次 Send 失敗的錯誤，讓呼叫端在迴圈跑完後
// 能檢查串流是否中途壞掉（gRPC Send 失敗通常代表用戶端已斷線）。
type GRPCEventSink struct {
	stream  grpcStreamSender
	sendErr error
}

func NewGRPCEventSink(stream grpcStreamSender) *GRPCEventSink {
	return &GRPCEventSink{stream: stream}
}

// Err 回傳串流過程中第一次 Send 失敗的錯誤（nil 表示全部成功）。
func (s *GRPCEventSink) Err() error {
	return s.sendErr
}

func (s *GRPCEventSink) send(ev *pb.StreamEvent) {
	if s.sendErr != nil {
		return // 已經壞過一次，後續事件不用再送
	}
	if err := s.stream.Send(ev); err != nil {
		s.sendErr = err
	}
}

func (s *GRPCEventSink) Text(text string) {
	s.send(&pb.StreamEvent{Payload: &pb.StreamEvent_Text{Text: &pb.TextEvent{Text: text}}})
}

func (s *GRPCEventSink) Done(sessionID string) {
	s.send(&pb.StreamEvent{Payload: &pb.StreamEvent_Done{Done: &pb.DoneEvent{SessionId: sessionID}}})
}

func (s *GRPCEventSink) Error(code int, msg string) {
	s.send(&pb.StreamEvent{Payload: &pb.StreamEvent_Error{Error: &pb.ErrorEvent{Code: int32(code), Message: msg}}})
}

func (s *GRPCEventSink) DraftPatch(patchJSON string) {
	s.send(&pb.StreamEvent{Payload: &pb.StreamEvent_DraftPatch{DraftPatch: &pb.DraftPatchEvent{PatchJson: patchJSON}}})
}

func (s *GRPCEventSink) ToolResult(resultJSON string) {
	s.send(&pb.StreamEvent{Payload: &pb.StreamEvent_ToolResult{ToolResult: &pb.ToolResultEvent{ResultJson: resultJSON}}})
}

var _ handler.EventSink = (*GRPCEventSink)(nil)
