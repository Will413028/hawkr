// Package assistant 實作買家助理 Connect API 的傳輸：請求驗證、錯誤對應、串流的 100 秒窗口與事件轉換。
// 輪次的邏輯（佔用、冪等、緩衝、owner 檢查）都在 modules/assistant，這裡不重做。
package assistant

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/assistant/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/assistant/v1alpha1/assistantv1alpha1connect"
	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	model "github.com/Will413028/hawkr/backend/internal/modules/assistant"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// WatchWindow 是一條 WatchAssistantTurn 串流的最長時間（A9-D12：Vercel 外部 rewrite 的上限），到了正常結束，client 以 cursor 接續。
const WatchWindow = 100 * time.Second

// Module 是助理業務的介面（*assistant.Module）。
type Module interface {
	SubmitTurn(ctx context.Context, in model.SubmitTurnInput) (model.TurnRef, error)
	WatchTurn(ctx context.Context, buyerID int64, turnID, cursor string) (iter.Seq2[model.TurnEvent, error], error)
	LatestSession(ctx context.Context, buyerID int64) (model.History, error)
}

// App 是 AssistantAppService 的 handler。
type App struct {
	m      Module
	buyers rpc.BuyerLookup
	log    *slog.Logger
	window time.Duration
}

var _ assistantv1alpha1connect.AssistantAppServiceHandler = (*App)(nil)

// NewApp 建立 handler；buyers 把登入的帳號對到買家（*buyer.Module）。
func NewApp(m Module, buyers rpc.BuyerLookup, log *slog.Logger) *App {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &App{m: m, buyers: buyers, log: log, window: WatchWindow}
}

// WithWatchWindow 改串流的最長時間，只給測試用。
func (h *App) WithWatchWindow(d time.Duration) *App {
	h.window = d
	return h
}

func (h *App) caller(ctx context.Context) (int64, error) {
	return rpc.AccountBuyerID(ctx, h.buyers)
}

var sendPaths = rpc.FieldPaths{"RequestID": "client_request_id"}

// SendAssistantMessage 受理一則訊息，立刻回輪次 ID。
func (h *App) SendAssistantMessage(ctx context.Context, req *connect.Request[pb.SendAssistantMessageRequest]) (*connect.Response[pb.SendAssistantMessageResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	topic, ok := topicFromProto(msg.GetTopic())
	if !ok {
		return nil, rpc.RequiredField("topic", "topic 必須是有效的主題")
	}
	if msg.GetMessage() == "" {
		return nil, rpc.RequiredField("message", "message 不能是空的")
	}
	if msg.GetClientRequestId() == "" {
		return nil, rpc.RequiredField("client_request_id", "client_request_id 不能是空的")
	}
	requestID, err := rpc.ParseRequestID(msg.GetClientRequestId())
	if err != nil {
		return nil, err
	}
	ref, err := h.m.SubmitTurn(ctx, model.SubmitTurnInput{
		BuyerID:   buyerID,
		Topic:     topic,
		SessionID: msg.GetSessionId(),
		Message:   msg.GetMessage(),
		ImageURL:  msg.GetImageUrl(),
		RequestID: requestID.String(),
	})
	if err != nil {
		return nil, sendError(err)
	}
	return connect.NewResponse(&pb.SendAssistantMessageResponse{TurnId: ref.TurnID, SessionId: ref.SessionID}), nil
}

// sendError 把送出的模組錯誤對應到 Connect 的分類與 reason；沒有列出的（建立對話失敗、transaction 內送出）是沒有分類的 internal。
func sendError(err error) error {
	switch {
	case errors.Is(err, model.ErrInvalidTopic):
		return rpc.RequiredField("topic", "topic 必須是有效的主題")
	case errors.Is(err, model.ErrMessageRequired):
		return rpc.RequiredField("message", "message 不能是空的")
	case errors.Is(err, model.ErrIdempotencyMismatch):
		return rpc.WithFieldPaths(apperr.InvalidInputOn([]string{"RequestID"}, "同一個 client_request_id 不能帶不同的內容"), sendPaths)
	case errors.Is(err, model.ErrSessionNotFound):
		return apperr.NotFound("找不到這個對話")
	case errors.Is(err, model.ErrTurnInProgress):
		return apperr.FailedPrecondition("這個對話還有一則回覆進行中，請等它結束再送出").WithReason(apperr.ReasonAssistantTurnInProgress)
	case errors.Is(err, model.ErrShuttingDown):
		return apperr.Unavailable("服務正在重新啟動，請稍後再試")
	}
	return err
}

// WatchAssistantTurn 訂閱一輪的事件；每條串流最長 WatchWindow，這一輪結束（終止事件之後）也結束。
func (h *App) WatchAssistantTurn(ctx context.Context, req *connect.Request[pb.WatchAssistantTurnRequest], stream *connect.ServerStream[pb.WatchAssistantTurnResponse]) error {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return err
	}
	if req.Msg.GetTurnId() == "" {
		return rpc.RequiredField("turn_id", "turn_id 不能是空的")
	}
	ctx, cancel := context.WithTimeout(ctx, h.window)
	defer cancel()
	events, err := h.m.WatchTurn(ctx, buyerID, req.Msg.GetTurnId(), req.Msg.GetCursor())
	if err != nil {
		switch {
		case errors.Is(err, model.ErrTurnNotFound):
			return apperr.NotFound("找不到這一輪回覆，請重新送出")
		case errors.Is(err, model.ErrCursorAhead):
			return apperr.FailedPrecondition("接續的位置不正確，請重新送出").WithReason(apperr.ReasonReplayCursorAhead)
		}
		return err
	}
	for ev, err := range events {
		if err != nil {
			return err
		}
		out, err := h.eventToProto(ctx, req.Msg.GetTurnId(), ev)
		if err != nil {
			return err
		}
		if err := stream.Send(out); err != nil {
			return err
		}
	}
	return nil
}

func (h *App) eventToProto(ctx context.Context, turnID string, ev model.TurnEvent) (*pb.WatchAssistantTurnResponse, error) {
	out := &pb.WatchAssistantTurnResponse{Cursor: ev.Cursor}
	switch ev.Kind {
	case model.EventText:
		out.Event = &pb.WatchAssistantTurnResponse_Text{Text: &pb.AssistantTextDelta{Text: ev.Text}}
	case model.EventToolResult:
		tool, ok := toolToProto(ev.Tool)
		if !ok {
			return nil, fmt.Errorf("assistant：事件的 tool %d 沒有對應的 AssistantTool", ev.Tool)
		}
		out.Event = &pb.WatchAssistantTurnResponse_ToolResult{ToolResult: &pb.AssistantToolResult{Tool: tool, Succeeded: ev.ToolSucceeded}}
	case model.EventFinished:
		out.Event = &pb.WatchAssistantTurnResponse_Finished{Finished: &pb.AssistantFinished{SessionId: ev.SessionID}}
	case model.EventError:
		// 失敗的細分（FailureKind）只進 log，對外一律 ASSISTANT_TURN_FAILED（A9-D17）。
		h.log.WarnContext(ctx, "assistant：這一輪以失敗結束", "turn_id", turnID, "failure", int(ev.Failure))
		out.Event = &pb.WatchAssistantTurnResponse_Error{Error: &pb.AssistantStreamError{
			Reason:           commonpb.ErrorReason_ASSISTANT_TURN_FAILED,
			LocalizedMessage: ev.Message,
		}}
	default:
		return nil, fmt.Errorf("assistant：不認得的事件種類 %d", ev.Kind)
	}
	return out, nil
}

// GetLatestAssistantSession 回呼叫者最近一次的對話與訊息。
func (h *App) GetLatestAssistantSession(ctx context.Context, _ *connect.Request[pb.GetLatestAssistantSessionRequest]) (*connect.Response[pb.GetLatestAssistantSessionResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	hist, err := h.m.LatestSession(ctx, buyerID)
	if err != nil {
		return nil, err
	}
	out := &pb.GetLatestAssistantSessionResponse{SessionId: hist.SessionID}
	for _, m := range hist.Messages {
		role := pb.AssistantMessageRole_ASSISTANT_MESSAGE_ROLE_ASSISTANT
		if m.Role == model.RoleUser {
			role = pb.AssistantMessageRole_ASSISTANT_MESSAGE_ROLE_USER
		}
		out.Messages = append(out.Messages, &pb.AssistantMessage{Role: role, Text: m.Text, CreateTime: timestamppb.New(m.Time)})
	}
	return connect.NewResponse(out), nil
}

func topicFromProto(t pb.AssistantTopic) (model.Topic, bool) {
	switch t {
	case pb.AssistantTopic_ASSISTANT_TOPIC_ORDERS:
		return model.TopicOrders, true
	case pb.AssistantTopic_ASSISTANT_TOPIC_TRACKING:
		return model.TopicTracking, true
	case pb.AssistantTopic_ASSISTANT_TOPIC_RETURN_POLICY:
		return model.TopicReturnPolicy, true
	case pb.AssistantTopic_ASSISTANT_TOPIC_CHAT:
		return model.TopicChat, true
	case pb.AssistantTopic_ASSISTANT_TOPIC_UNSPECIFIED:
	}
	return 0, false
}

func toolToProto(t model.Tool) (pb.AssistantTool, bool) {
	switch t {
	case model.ToolGetMyOrders:
		return pb.AssistantTool_ASSISTANT_TOOL_GET_MY_ORDERS, true
	case model.ToolGetBuyerOrderDetail:
		return pb.AssistantTool_ASSISTANT_TOOL_GET_BUYER_ORDER_DETAIL, true
	case model.ToolTrackMyPackage:
		return pb.AssistantTool_ASSISTANT_TOOL_TRACK_MY_PACKAGE, true
	}
	return pb.AssistantTool_ASSISTANT_TOOL_UNSPECIFIED, false
}
