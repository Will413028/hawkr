// Package buyerprocurement 實作買家採購 Connect API（BuyerProcurementAppService）的傳輸：請求驗證、錯誤與欄位路徑對應、
// 事件與讀取模型轉成 proto。送出、取消、串流、搜尋、接續與快照的規則都在 modules/procurement，這裡不重做。
//
// 實作 run 的送出、取消、串流、接續與搜尋，快照讀取與對話管理，需求編輯（ToggleSelection、SubmitAnswer、Rewind），以及詢價發布鏈（PreviewRfqPrice、ConfirmRfqSnapshot、PreviewRfqPublication、ExecuteRfqPublication，rfq.go）；其餘方法由內嵌的 Unimplemented 回 unimplemented，handler 尚未掛載。
package buyerprocurement

import (
	"context"
	"fmt"
	"log/slog"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
	"uuid"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/procurement/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/procurement/v1alpha1/procurementv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/requestid"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// Module 是採購業務的介面（*procurement.Module）。
type Module interface {
	SubmitTurn(ctx context.Context, in procurement.SubmitTurnInput) (procurement.TurnSubmitted, error)
	WatchRun(ctx context.Context, buyerID int64, runID uuid.UUID, cursor string, send func(procurement.RunEvent) error) error
	CancelRun(ctx context.Context, buyerID int64, runID uuid.UUID) (procurement.CancelRunOutcome, error)
	ResumeRun(ctx context.Context, in procurement.ResumeRunInput) (procurement.ResumeRunResult, error)
	StartCandidateSearch(ctx context.Context, in procurement.StartCandidateSearchInput) (procurement.CandidateSearchStarted, error)
	ToggleSelection(ctx context.Context, in procurement.ToggleSelectionInput) error
	SubmitAnswer(ctx context.Context, in procurement.SubmitAnswerInput) ([]byte, error)
	Rewind(ctx context.Context, in procurement.RewindInput) ([]byte, error)
	GetRuntimeSnapshot(ctx context.Context, buyerID int64, conversationID uuid.UUID) (procurement.RuntimeSnapshot, error)
	GetSnapshot(ctx context.Context, buyerID int64, requestID uuid.UUID) (procurement.RequestSnapshot, error)
	GetConversationMessages(ctx context.Context, buyerID int64, conversationID uuid.UUID) (procurement.ConversationMessages, error)
	ListConversations(ctx context.Context, buyerID int64, pageSize int32, pageToken string) (procurement.ConversationPage, error)
	RenameConversation(ctx context.Context, buyerID int64, conversationID uuid.UUID, title string) error
	DeleteConversation(ctx context.Context, buyerID int64, conversationID uuid.UUID) error
	GetCandidateSearch(ctx context.Context, buyerID int64, requestID, runID uuid.UUID, locale uitext.Locale) (procurement.CandidateSearch, error)
	PreviewRfqPrice(ctx context.Context, in procurement.PreviewRfqPriceInput) (procurement.PricePreview, error)
	ConfirmRfqSnapshot(ctx context.Context, in procurement.ConfirmRfqSnapshotInput) (procurement.ConfirmedRfqSnapshot, error)
	PreviewRfqPublication(ctx context.Context, in procurement.PreviewRfqPublicationInput) (procurement.PublicationPreview, error)
	ExecuteRfqPublication(ctx context.Context, in procurement.ExecuteRfqPublicationInput) (procurement.Publication, error)
}

// StoreCards 讀候選賣家的店家卡片（*seller.Module 滿足它）：從沒公開過與不存在的店不在回傳裡；一次最多 seller.MaxBatchSize 個。
type StoreCards interface {
	StoreCards(ctx context.Context, storeIDs []int64) ([]seller.StoreCard, error)
}

// App 是 BuyerProcurementAppService 的 handler。
type App struct {
	procurementv1alpha1connect.UnimplementedBuyerProcurementAppServiceHandler
	m      Module
	buyers rpc.BuyerLookup
	cards  StoreCards
	images rpc.Images
	log    *slog.Logger
}

var _ procurementv1alpha1connect.BuyerProcurementAppServiceHandler = (*App)(nil)

// NewApp 建立 handler；buyers 把登入的帳號對到買家（*buyer.Module），cards 與 images 組候選賣家的店家卡片。
func NewApp(m Module, buyers rpc.BuyerLookup, cards StoreCards, images rpc.Images, log *slog.Logger) *App {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &App{m: m, buyers: buyers, cards: cards, images: images, log: log}
}

func (h *App) caller(ctx context.Context) (int64, error) { return rpc.AccountBuyerID(ctx, h.buyers) }

// 模組欄位違規的欄位名（Go 欄位名或模組的請求欄位名）對到請求的 proto 路徑；ResumeRun 的回答在巢狀的 clarification 底下。
var (
	submitPaths = rpc.FieldPaths{"Message": "message", "RequestID": "request_id", "client_request_id": "client_request_id"}
	searchPaths = rpc.FieldPaths{"client_request_id": "client_request_id"}
	resumePaths = rpc.FieldPaths{
		"OptionID": "clarification.option_id", "FreeText": "clarification.free_text", "client_request_id": "client_request_id",
	}
	watchPaths  = rpc.FieldPaths{"cursor": "cursor"}
	answerPaths = rpc.FieldPaths{
		"SlotKey": "slot_key", "OptionID": "option_id", "FreeText": "free_text", "UnitOptionID": "unit_option_id", "Skip": "skip",
		"client_request_id": "client_request_id",
	}
	rewindPaths = rpc.FieldPaths{"SlotKey": "slot_key", "client_request_id": "client_request_id"}
)

// clientRequestID 解析必填的冪等鍵：UUIDv4。
func clientRequestID(raw string) (uuid.UUID, error) {
	if raw == "" {
		return uuid.UUID{}, rpc.RequiredField("client_request_id", "client_request_id 不能是空的")
	}
	id, err := uuid.Parse(raw)
	if err != nil || len(raw) != 36 || !requestid.IsV4(id) {
		return uuid.UUID{}, rpc.RequiredField("client_request_id", "client_request_id 必須是 UUIDv4")
	}
	return id, nil
}

// SubmitTurn 受理一則訊息，立刻回 run ID。
func (h *App) SubmitTurn(ctx context.Context, req *connect.Request[pb.SubmitTurnRequest]) (*connect.Response[pb.SubmitTurnResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	var requestID *uuid.UUID
	if raw := msg.GetRequestId(); raw != "" {
		id, err := rpc.ParseUUID("request_id", raw)
		if err != nil {
			return nil, err
		}
		requestID = &id
	}
	if msg.GetMessage() == "" {
		return nil, rpc.RequiredField("message", "message 不能是空的")
	}
	cid, err := clientRequestID(msg.GetClientRequestId())
	if err != nil {
		return nil, err
	}
	out, err := h.m.SubmitTurn(ctx, procurement.SubmitTurnInput{
		BuyerID: buyerID, RequestID: requestID, Message: msg.GetMessage(), ClientRequestID: cid, Locale: uitext.LocaleFrom(ctx),
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, submitPaths)
	}
	return connect.NewResponse(&pb.SubmitTurnResponse{
		RequestId: out.RequestID.String(), ConversationId: out.ConversationID.String(), RunId: out.RunID.String(),
	}), nil
}

// WatchRun 訂閱一個 run 的事件；結尾事件 finished 之後、串流視窗到期或 client 離開時結束。
func (h *App) WatchRun(ctx context.Context, req *connect.Request[pb.WatchRunRequest], stream *connect.ServerStream[pb.WatchRunResponse]) error {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return err
	}
	runID, err := rpc.ParseUUID("run_id", req.Msg.GetRunId())
	if err != nil {
		return err
	}
	err = h.m.WatchRun(ctx, buyerID, runID, req.Msg.GetCursor(), func(e procurement.RunEvent) error {
		out, err := eventToProto(e)
		if err != nil {
			h.log.ErrorContext(ctx, "procurement：run 事件無法轉成回應", "run_id", runID, "cursor", e.Cursor, "error", err)
			return err
		}
		return stream.Send(out)
	})
	return rpc.WithFieldPaths(err, watchPaths)
}

// eventToProto 把事件 payload（proto JSON，欄位與值名由模組的 golden 測試守住）解成 WatchRunResponse，補上 cursor 與事件時間。
func eventToProto(e procurement.RunEvent) (*pb.WatchRunResponse, error) {
	out := &pb.WatchRunResponse{}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(e.Payload, out); err != nil {
		return nil, fmt.Errorf("buyerprocurement：事件 payload 不是合法的 WatchRunResponse：%w", err)
	}
	if out.GetEvent() == nil {
		return nil, fmt.Errorf("buyerprocurement：事件 payload 沒有事件內容")
	}
	out.Cursor = e.Cursor
	out.EventTime = timestamppb.New(e.OccurredAt)
	return out, nil
}

// CancelRun 取消買家自己的 run（冪等）。
func (h *App) CancelRun(ctx context.Context, req *connect.Request[pb.CancelRunRequest]) (*connect.Response[pb.CancelRunResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	runID, err := rpc.ParseUUID("run_id", req.Msg.GetRunId())
	if err != nil {
		return nil, err
	}
	out, err := h.m.CancelRun(ctx, buyerID, runID)
	if err != nil {
		return nil, err
	}
	status, err := runStatusToProto(out.Status)
	if err != nil {
		return nil, err
	}
	disposition, err := dispositionToProto(out.Disposition)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.CancelRunResponse{RunId: runID.String(), RunStatus: status, CancelDisposition: disposition}), nil
}

// ResumeRun 回答一個中斷，立刻回回答 run 的 ID。
func (h *App) ResumeRun(ctx context.Context, req *connect.Request[pb.ResumeRunRequest]) (*connect.Response[pb.ResumeRunResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	interruptID, err := rpc.ParseUUID("interrupt_id", msg.GetInterruptId())
	if err != nil {
		return nil, err
	}
	cid, err := clientRequestID(msg.GetClientRequestId())
	if err != nil {
		return nil, err
	}
	answer := msg.GetClarification()
	if answer == nil {
		return nil, rpc.RequiredField("clarification", "要回答這個中斷：clarification 不能是空的")
	}
	out, err := h.m.ResumeRun(ctx, procurement.ResumeRunInput{
		BuyerID: buyerID, InterruptID: interruptID, ClientRequestID: cid,
		OptionID: answer.GetOptionId(), FreeText: answer.GetFreeText(), Locale: uitext.LocaleFrom(ctx),
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, resumePaths)
	}
	status, err := runStatusToProto(out.Status)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.ResumeRunResponse{RunId: out.RunID.String(), RunStatus: status}), nil
}

// StartCandidateSearch 開始為需求搜尋候選賣家。
func (h *App) StartCandidateSearch(ctx context.Context, req *connect.Request[pb.StartCandidateSearchRequest]) (*connect.Response[pb.StartCandidateSearchResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	requestID, err := rpc.ParseUUID("request_id", req.Msg.GetRequestId())
	if err != nil {
		return nil, err
	}
	cid, err := clientRequestID(req.Msg.GetClientRequestId())
	if err != nil {
		return nil, err
	}
	out, err := h.m.StartCandidateSearch(ctx, procurement.StartCandidateSearchInput{BuyerID: buyerID, RequestID: requestID, ClientRequestID: cid})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, searchPaths)
	}
	status, err := runStatusToProto(out.Status)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.StartCandidateSearchResponse{
		RequestId: requestID.String(), RunId: out.RunID.String(), Status: status, CandidateSetId: out.CandidateSetID.String(),
	}), nil
}

// ToggleSelection 把一個候選設成指定的勾選狀態（目標狀態的方法，重送結果相同）。
func (h *App) ToggleSelection(ctx context.Context, req *connect.Request[pb.ToggleSelectionRequest]) (*connect.Response[pb.ToggleSelectionResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	requestID, err := rpc.ParseUUID("request_id", msg.GetRequestId())
	if err != nil {
		return nil, err
	}
	setID, err := rpc.ParseUUID("candidate_set_id", msg.GetCandidateSetId())
	if err != nil {
		return nil, err
	}
	candidateID, err := rpc.ParseUUID("candidate_id", msg.GetCandidateId())
	if err != nil {
		return nil, err
	}
	if err := h.m.ToggleSelection(ctx, procurement.ToggleSelectionInput{
		BuyerID: buyerID, RequestID: requestID, CandidateSetID: setID, CandidateID: candidateID, Selected: msg.GetSelected(),
	}); err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.ToggleSelectionResponse{
		RequestId: requestID.String(), CandidateSetId: setID.String(), CandidateId: candidateID.String(), Selected: msg.GetSelected(),
	}), nil
}

// SubmitAnswer 回答一個釐清問題：同步回這一輪的結果（不建 run）。
func (h *App) SubmitAnswer(ctx context.Context, req *connect.Request[pb.SubmitAnswerRequest]) (*connect.Response[pb.SubmitAnswerResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	requestID, err := rpc.ParseUUID("request_id", msg.GetRequestId())
	if err != nil {
		return nil, err
	}
	slot, err := slotKeyFromProto(msg.GetSlotKey())
	if err != nil {
		return nil, err
	}
	expected, err := rpc.ParseUUID("expected_revision_id", msg.GetExpectedRevisionId())
	if err != nil {
		return nil, err
	}
	cid, err := clientRequestID(msg.GetClientRequestId())
	if err != nil {
		return nil, err
	}
	raw, err := h.m.SubmitAnswer(ctx, procurement.SubmitAnswerInput{
		BuyerID: buyerID, RequestID: requestID, Slot: slot, OptionID: msg.GetOptionId(), FreeText: msg.GetFreeText(),
		UnitOptionID: msg.GetUnitOptionId(), Skip: msg.GetSkip(), ExpectedRevisionID: expected, ClientRequestID: cid, Locale: uitext.LocaleFrom(ctx),
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, answerPaths)
	}
	result, err := turnResultToProto(raw)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.SubmitAnswerResponse{Result: result}), nil
}

// Rewind 讓需求退回某個欄位重新回答：同步回這一輪的結果（不建 run）。
func (h *App) Rewind(ctx context.Context, req *connect.Request[pb.RewindRequest]) (*connect.Response[pb.RewindResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	requestID, err := rpc.ParseUUID("request_id", msg.GetRequestId())
	if err != nil {
		return nil, err
	}
	slot, err := slotKeyFromProto(msg.GetSlotKey())
	if err != nil {
		return nil, err
	}
	expected, err := rpc.ParseUUID("expected_revision_id", msg.GetExpectedRevisionId())
	if err != nil {
		return nil, err
	}
	cid, err := clientRequestID(msg.GetClientRequestId())
	if err != nil {
		return nil, err
	}
	raw, err := h.m.Rewind(ctx, procurement.RewindInput{
		BuyerID: buyerID, RequestID: requestID, Slot: slot, ExpectedRevisionID: expected, ClientRequestID: cid, Locale: uitext.LocaleFrom(ctx),
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, rewindPaths)
	}
	result, err := turnResultToProto(raw)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.RewindResponse{Result: result}), nil
}

// turnResultToProto 把輪次存下的 TurnResult wire JSON（欄位與值名由模組的 golden 測試守住）解成 proto。
func turnResultToProto(raw []byte) (*pb.TurnResult, error) {
	out := &pb.TurnResult{}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(raw, out); err != nil {
		return nil, fmt.Errorf("buyerprocurement：存下的 turn_result 不是合法的 TurnResult：%w", err)
	}
	return out, nil
}

// GetRuntimeSnapshot 回重建一個對話畫面需要的全部事實。
func (h *App) GetRuntimeSnapshot(ctx context.Context, req *connect.Request[pb.GetRuntimeSnapshotRequest]) (*connect.Response[pb.GetRuntimeSnapshotResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	conversationID, err := rpc.ParseUUID("conversation_id", req.Msg.GetConversationId())
	if err != nil {
		return nil, err
	}
	snap, err := h.m.GetRuntimeSnapshot(ctx, buyerID, conversationID)
	if err != nil {
		return nil, err
	}
	out, err := snapshotToProto(snap)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(out), nil
}
