package handler

// EventSink 是 RunAssistantStream 產生的 ADK 事件轉譯目標。HTTP SSE 版實作
// （sseEventSink）已隨 Phase C（移除 ai-service HTTP business surface）刪除，現在
// 唯一的實作是 gRPC server-stream 版（internal/grpc package 的 GRPCEventSink）。
// 介面留在這裡沒有跟著搬，是因為 RunAssistantStream（seller_assistant.go）與
// internal/grpc/stream_sink.go 都還在 import 這個 package，搬遷屬於另一輪範圍更大
// 的重構，不在本次純刪除範圍內。
type EventSink interface {
	Text(s string)
	Done(sessionID string)
	Error(code int, msg string)
	DraftPatch(patchJSON string)
	ToolResult(resultJSON string)
}
