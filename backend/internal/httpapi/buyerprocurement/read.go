package buyerprocurement

import (
	"context"
	"fmt"

	"connectrpc.com/connect"

	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/procurement/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	httpseller "github.com/Will413028/hawkr/backend/internal/httpapi/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

var (
	listPaths   = rpc.FieldPaths{"page_size": "page_size", "page_token": "page_token"}
	renamePaths = rpc.FieldPaths{"title": "title"}
)

// GetSnapshot 回傳需求目前的狀態：欄位、缺漏的必填欄位、run、待回答的中斷與發布結果。
func (h *App) GetSnapshot(ctx context.Context, req *connect.Request[pb.GetSnapshotRequest]) (*connect.Response[pb.GetSnapshotResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	requestID, err := rpc.ParseUUID("request_id", req.Msg.GetRequestId())
	if err != nil {
		return nil, err
	}
	snap, err := h.m.GetSnapshot(ctx, buyerID, requestID)
	if err != nil {
		return nil, err
	}
	out, err := requestSnapshotToProto(snap)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(out), nil
}

// GetConversationMessages 回傳對話的訊息，由舊到新。
func (h *App) GetConversationMessages(ctx context.Context, req *connect.Request[pb.GetConversationMessagesRequest]) (*connect.Response[pb.GetConversationMessagesResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	conversationID, err := rpc.ParseUUID("conversation_id", req.Msg.GetConversationId())
	if err != nil {
		return nil, err
	}
	res, err := h.m.GetConversationMessages(ctx, buyerID, conversationID)
	if err != nil {
		return nil, err
	}
	status, err := requestStatusToProto(res.Status)
	if err != nil {
		return nil, err
	}
	messages, err := messagesToProto(res.Messages)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.GetConversationMessagesResponse{
		ConversationId: res.ConversationID.String(), RequestId: res.RequestID.String(), Status: status, Title: res.Title, Messages: messages,
	}), nil
}

// ListConversations 回傳買家的對話列表（keyset 分頁）。
func (h *App) ListConversations(ctx context.Context, req *connect.Request[pb.ListConversationsRequest]) (*connect.Response[pb.ListConversationsResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	page, err := h.m.ListConversations(ctx, buyerID, req.Msg.GetPageSize(), req.Msg.GetPageToken())
	if err != nil {
		return nil, rpc.WithFieldPaths(err, listPaths)
	}
	out := &pb.ListConversationsResponse{NextPageToken: page.NextPageToken}
	for _, c := range page.Conversations {
		out.Conversations = append(out.Conversations, &pb.ConversationSummary{
			Id: c.ID.String(), Title: c.Title, LastMessagePreview: c.LastMessagePreview, UpdateTime: timestampOf(c.UpdateTime),
		})
	}
	return connect.NewResponse(out), nil
}

// RenameConversation 改對話標題（目標狀態，冪等鍵只驗格式）。
func (h *App) RenameConversation(ctx context.Context, req *connect.Request[pb.RenameConversationRequest]) (*connect.Response[pb.RenameConversationResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	conversationID, err := rpc.ParseUUID("conversation_id", req.Msg.GetConversationId())
	if err != nil {
		return nil, err
	}
	if _, err := clientRequestID(req.Msg.GetClientRequestId()); err != nil {
		return nil, err
	}
	if err := h.m.RenameConversation(ctx, buyerID, conversationID, req.Msg.GetTitle()); err != nil {
		return nil, rpc.WithFieldPaths(err, renamePaths)
	}
	return connect.NewResponse(&pb.RenameConversationResponse{}), nil
}

// DeleteConversation 刪除對話並取消它進行中的 run（目標狀態，已刪除的重送回成功）。
func (h *App) DeleteConversation(ctx context.Context, req *connect.Request[pb.DeleteConversationRequest]) (*connect.Response[pb.DeleteConversationResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	conversationID, err := rpc.ParseUUID("conversation_id", req.Msg.GetConversationId())
	if err != nil {
		return nil, err
	}
	if _, err := clientRequestID(req.Msg.GetClientRequestId()); err != nil {
		return nil, err
	}
	if err := h.m.DeleteConversation(ctx, buyerID, conversationID); err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.DeleteConversationResponse{}), nil
}

// GetCandidateSearch 回傳一次候選搜尋的狀態與結果；候選的店家卡片在這裡組（沒公開過的店沒有卡片，候選照列、不設 store）。
func (h *App) GetCandidateSearch(ctx context.Context, req *connect.Request[pb.GetCandidateSearchRequest]) (*connect.Response[pb.GetCandidateSearchResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	requestID, err := rpc.ParseUUID("request_id", req.Msg.GetRequestId())
	if err != nil {
		return nil, err
	}
	runID, err := rpc.ParseUUID("run_id", req.Msg.GetRunId())
	if err != nil {
		return nil, err
	}
	res, err := h.m.GetCandidateSearch(ctx, buyerID, requestID, runID, uitext.LocaleFrom(ctx))
	if err != nil {
		return nil, err
	}
	ids := make([]int64, len(res.Candidates))
	for i, c := range res.Candidates {
		ids[i] = c.StoreID
	}
	cards, err := h.storeCards(ctx, ids)
	if err != nil {
		return nil, err
	}
	out, err := candidateSearchToProto(res, cards)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(out), nil
}

func (h *App) storeCards(ctx context.Context, ids []int64) (map[int64]*commonpb.StoreCard, error) {
	out := make(map[int64]*commonpb.StoreCard, len(ids))
	for start := 0; start < len(ids); start += seller.MaxBatchSize {
		end := min(start+seller.MaxBatchSize, len(ids))
		rows, err := h.cards.StoreCards(ctx, ids[start:end])
		if err != nil {
			return nil, fmt.Errorf("buyerprocurement：讀店家卡片：%w", err)
		}
		for _, c := range rows {
			card, err := httpseller.ToStoreCard(h.images, c)
			if err != nil {
				return nil, err
			}
			out[c.StoreID] = card
		}
	}
	return out, nil
}
