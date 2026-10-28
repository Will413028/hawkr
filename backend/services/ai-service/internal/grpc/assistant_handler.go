package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	adkagent "google.golang.org/adk/v2/agent"

	agentpkg "github.com/Will413028/hawkr/ai-service/internal/agent"
	"github.com/Will413028/hawkr/ai-service/internal/cache"
	"github.com/Will413028/hawkr/ai-service/internal/config"
	"github.com/Will413028/hawkr/ai-service/internal/handler"
	"github.com/Will413028/hawkr/ai-service/internal/usecase"
	"github.com/Will413028/hawkr/ai-service/internal/usecase/dto"
	"github.com/Will413028/hawkr/go-pkg/identity"
	pb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// AIAssistantGrpcServer implements pb.AIAssistantServiceServer.
type AIAssistantGrpcServer struct {
	pb.UnimplementedAIAssistantServiceServer
	runners       *agentpkg.Runners
	cfg           *config.Config
	sessionSvc    session.Service
	orderClient   usecase.OrderClient
	categoryIndex *cache.CategoryIndex
}

// NewAIAssistantGrpcServer creates a new AIAssistantGrpcServer.
func NewAIAssistantGrpcServer(runners *agentpkg.Runners, cfg *config.Config, sessionSvc session.Service, orderClient usecase.OrderClient, categoryIndex *cache.CategoryIndex) *AIAssistantGrpcServer {
	return &AIAssistantGrpcServer{runners: runners, cfg: cfg, sessionSvc: sessionSvc, orderClient: orderClient, categoryIndex: categoryIndex}
}

// genericStreamRPC 是 10 支通用 streaming RPC 的共用執行邏輯：解析 actor、決定
// message（button endpoint 用預設訊息）、呼叫既有 handler.RunAssistantStream。
func (s *AIAssistantGrpcServer) genericStreamRPC(stream grpcStreamSender, req *pb.AssistantRequest, r *runner.Runner, endpointKey string) error {
	ctx := stream.Context()
	act, ok := ActorFromContext(ctx)
	if !ok {
		return status.Error(codes.PermissionDenied, "actor not resolved")
	}

	message := req.Message
	if message == "" {
		var found bool
		message, found = handler.DefaultMessages[endpointKey]
		if !found {
			return status.Error(codes.InvalidArgument, "message is required")
		}
	}
	userMessage := genai.NewContentFromText(message, genai.RoleUser)
	if req.ImageUrl != "" {
		userMessage.Parts = append(userMessage.Parts, &genai.Part{FileData: &genai.FileData{FileURI: req.ImageUrl}})
	}

	sink := NewGRPCEventSink(stream)
	if err := handler.RunAssistantStream(ctx, s.sessionSvc, sink, r, act, req.SessionId, userMessage); err != nil {
		return status.Error(codes.Internal, "服務暫時無法使用")
	}
	if sink.Err() != nil {
		return sink.Err()
	}
	return nil
}

func (s *AIAssistantGrpcServer) SellerChat(req *pb.AssistantRequest, stream pb.AIAssistantService_SellerChatServer) error {
	return s.genericStreamRPC(stream, req, handler.ChatRunner(s.cfg, s.runners, "seller"), "")
}
func (s *AIAssistantGrpcServer) SellerInventory(req *pb.AssistantRequest, stream pb.AIAssistantService_SellerInventoryServer) error {
	return s.genericStreamRPC(stream, req, s.runners.Specialists[handler.ButtonSpecialistKey["inventory"]], "inventory")
}
func (s *AIAssistantGrpcServer) SellerPayment(req *pb.AssistantRequest, stream pb.AIAssistantService_SellerPaymentServer) error {
	return s.genericStreamRPC(stream, req, s.runners.Specialists[handler.ButtonSpecialistKey["payment"]], "payment")
}
func (s *AIAssistantGrpcServer) SellerProductListingChat(req *pb.AssistantRequest, stream pb.AIAssistantService_SellerProductListingChatServer) error {
	return s.genericStreamRPC(stream, req, s.runners.ProductListing, "")
}
func (s *AIAssistantGrpcServer) BuyerOrders(req *pb.AssistantRequest, stream pb.AIAssistantService_BuyerOrdersServer) error {
	return s.genericStreamRPC(stream, req, s.runners.Specialists[handler.ButtonSpecialistKey["orders"]], "orders")
}
func (s *AIAssistantGrpcServer) BuyerTracking(req *pb.AssistantRequest, stream pb.AIAssistantService_BuyerTrackingServer) error {
	return s.genericStreamRPC(stream, req, s.runners.Specialists[handler.ButtonSpecialistKey["tracking"]], "tracking")
}
func (s *AIAssistantGrpcServer) BuyerSellerMatch(req *pb.AssistantRequest, stream pb.AIAssistantService_BuyerSellerMatchServer) error {
	return s.genericStreamRPC(stream, req, s.runners.Specialists[handler.ButtonSpecialistKey["seller-match"]], "seller-match")
}
func (s *AIAssistantGrpcServer) BuyerChat(req *pb.AssistantRequest, stream pb.AIAssistantService_BuyerChatServer) error {
	return s.genericStreamRPC(stream, req, handler.ChatRunner(s.cfg, s.runners, "buyer"), "")
}

func (s *AIAssistantGrpcServer) BuyerReturnPolicy(req *pb.AssistantRequest, stream pb.AIAssistantService_BuyerReturnPolicyServer) error {
	sink := NewGRPCEventSink(stream)
	sink.Text(handler.ReturnPolicyText)
	sink.Done("")
	return sink.Err()
}

func (s *AIAssistantGrpcServer) SellerOrders(req *pb.AssistantRequest, stream pb.AIAssistantService_SellerOrdersServer) error {
	ctx := stream.Context()
	act, ok := ActorFromContext(ctx)
	if !ok {
		return status.Error(codes.PermissionDenied, "actor not resolved")
	}
	storeID, err := strconv.ParseInt(act.UserID, 10, 64)
	if err != nil {
		return status.Error(codes.Internal, "invalid store id in resolved actor")
	}

	orders, err := s.orderClient.GetSellerOrders(ctx, identity.StoreID(storeID), "20", 1, 10)
	if err != nil {
		return status.Error(codes.Internal, "查詢失敗，請稍後再試")
	}

	sink := NewGRPCEventSink(stream)

	orderItems := make([]dto.OrderItemResult, 0, len(orders))
	for _, o := range orders {
		orderItems = append(orderItems, dto.OrderItemResult{
			OrderNo:      o.OrderNo,
			Status:       agentpkg.StatusLabel(o.Status),
			BuyerName:    o.BuyerName,
			ItemsSummary: agentpkg.BuildItemsSummary(o.Items),
			AmountTWD:    o.Amount,
			CreateTimeMs: o.CreateTimeMs,
		})
	}
	toolResultJSON, _ := json.Marshal(dto.OrderListToolResult{ResultType: "order_list", Orders: orderItems})
	sink.ToolResult(string(toolResultJSON))

	var prompt string
	if len(orders) == 0 {
		prompt = "待出貨訂單查詢結果：目前沒有任何待出貨訂單（物流單已建立但尚未出貨）。"
	} else {
		var sb strings.Builder
		fmt.Fprintf(&sb, "待出貨訂單查詢結果（共 %d 筆）：\n\n", len(orders))
		for _, o := range orders {
			parts := make([]string, 0, len(o.Items))
			for _, item := range o.Items {
				name := item.ProductName
				if item.SkuName != "" && item.SkuName != item.ProductName {
					name = item.SkuName
				}
				parts = append(parts, fmt.Sprintf("%s x%d", name, item.Quantity))
			}
			fmt.Fprintf(&sb, "- 訂單編號：%s | 買家：%s | 商品：%s | 金額：%d TWD\n",
				o.OrderNo, o.BuyerName, strings.Join(parts, ", "), o.Amount)
		}
		prompt = sb.String()
	}

	userMessage := genai.NewContentFromText(prompt, genai.RoleUser)
	if err := handler.RunAssistantStream(ctx, s.sessionSvc, sink, s.runners.OrderPresenter, act, req.SessionId, userMessage); err != nil {
		return status.Error(codes.Internal, "服務暫時無法使用")
	}
	if sink.Err() != nil {
		return sink.Err()
	}
	return nil
}

// SellerProductAutofill 接受自由文字（與可選圖片），呼叫 Gemini structured
// output，回傳商品草稿。與 HTTP 版 SellerAssistantHandler.ProductAutofill
// （internal/handler/seller_assistant.go）邏輯一致，差異只在：auth 已由
// method-policy interceptor 在呼叫本方法前驗證完（見 registerMethodPolicy 於
// 下方 init 的登記），proto request 已型別化不需要 JSON bind。驗證順序刻意與
// HTTP 版一致：categoryIndex.Ready() 在文字驗證之前——這是既有行為（分類資料
// 未就緒時優先回報，即使文字同時也是空的），不是新設計選擇。
func (s *AIAssistantGrpcServer) SellerProductAutofill(ctx context.Context, req *pb.SellerProductAutofillRequest) (*pb.SellerProductAutofillResponse, error) {
	if !s.categoryIndex.Ready() {
		// codes.FailedPrecondition 而非 Unavailable（#3330）：這是**資料狀態**，本服務健康。
		// gateway 的 circuit breaker 以 target 為 key、一個服務的所有路由共用一顆，
		// 把它當 transport 失敗計入 ⇒ 單一使用者觸發就能推開 ai-service 全部端點
		//（買家 App 6 支、賣家 App 8 支、website procurement 20 支一起中箭）。
		return nil, status.Error(codes.FailedPrecondition, "分類資料未就緒")
	}
	if req.Text == "" {
		return nil, status.Error(codes.InvalidArgument, "請輸入商品描述")
	}
	if len([]rune(req.Text)) > 2000 {
		return nil, status.Error(codes.InvalidArgument, "輸入過長")
	}

	runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	userContent := genai.NewContentFromText(req.Text, genai.RoleUser)
	if req.ImageUrl != "" {
		userContent.Parts = append(userContent.Parts, &genai.Part{FileData: &genai.FileData{FileURI: req.ImageUrl}})
	}

	var lastText string
	for event, runErr := range s.runners.ProductAutofill.Run(runCtx, "autofill", "", userContent, adkagent.RunConfig{}) {
		if runCtx.Err() != nil {
			break
		}
		if runErr != nil {
			if strings.Contains(runErr.Error(), "429") || strings.Contains(runErr.Error(), "quota") {
				// codes.ResourceExhausted 而非 Unavailable（#3330）：上游 Gemini 回 429，
				// 那是**下游的下游**，本服務健康。#3230 已把 ResourceExhausted 移出 breaker
				// 的失敗清單，所以改用它之後限流不再推 breaker；HTTP 也從 503 變成語意
				// 正確的 429。
				return nil, status.Error(codes.ResourceExhausted, "AI 服務忙線中，請稍後再試")
			}
			return nil, status.Error(codes.Internal, "AI 解析失敗，請稍後再試")
		}
		if event == nil || event.Content == nil {
			continue
		}
		if !event.Partial {
			for _, part := range event.Content.Parts {
				if part.Text != "" {
					lastText = part.Text
				}
			}
		}
	}
	if runCtx.Err() != nil || lastText == "" {
		return nil, status.Error(codes.Internal, "AI 解析失敗，請稍後再試")
	}

	var raw dto.RawDraft
	if err := json.Unmarshal([]byte(lastText), &raw); err != nil {
		return nil, status.Error(codes.Internal, "AI 解析失敗，請稍後再試")
	}

	var warnings []string
	candidates := s.categoryIndex.Match(raw.CategoryKeyword, 3)
	catCandidates := make([]*pb.CategoryCandidate, 0, len(candidates))
	for _, c := range candidates {
		catCandidates = append(catCandidates, &pb.CategoryCandidate{CatalogId: c.CatalogID, Path: c.Path, Confidence: c.Confidence})
	}
	if len(catCandidates) == 0 {
		warnings = append(warnings, "分類匹配失敗，請手動選擇")
	}

	skus := make([]*pb.SkuDraft, 0, len(raw.Skus))
	for _, sk := range raw.Skus {
		if sk.Price <= 0 {
			warnings = append(warnings, fmt.Sprintf("規格「%s」價格不合法，已略過", sk.SkuName))
			continue
		}
		unit := sk.Unit
		if unit == "NONE" {
			unit = ""
		}
		if !handler.ValidWeightUnits[unit] {
			warnings = append(warnings, fmt.Sprintf("規格「%s」單位無法辨識，請手動選擇", sk.SkuName))
			unit = ""
		}
		skus = append(skus, &pb.SkuDraft{SkuName: sk.SkuName, Price: strconv.FormatInt(sk.Price, 10), Unit: unit})
	}

	return &pb.SellerProductAutofillResponse{
		Data: &pb.ProductDraft{
			Name:               raw.Name,
			Origin:             raw.Origin,
			Description:        raw.Description,
			CategoryCandidates: catCandidates,
			Skus:               skus,
			Tags:               raw.Tags,
			Warnings:           warnings,
		},
	}, nil
}

// SellerLatestSession／BuyerLatestSession 共用 handler.FetchLatestSession 核心：
// 列出 actor 的所有 session，回傳最新一筆的完整歷史。與 HTTP 版
// SellerAssistantHandler.GetLatestSession／BuyerAssistantHandler.GetLatestSession
// 差異只在 auth：這裡的 actor 已由 method-policy interceptor 解析完寫入 context。
func (s *AIAssistantGrpcServer) SellerLatestSession(ctx context.Context, req *pb.LatestSessionRequest) (*pb.LatestSessionResponse, error) {
	act, ok := ActorFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.PermissionDenied, "actor not resolved")
	}
	sessionID, messages, err := handler.FetchLatestSession(ctx, s.sessionSvc, act)
	if err != nil {
		return nil, status.Error(codes.Internal, "取得對話失敗")
	}
	return toLatestSessionResponse(sessionID, messages), nil
}

func (s *AIAssistantGrpcServer) BuyerLatestSession(ctx context.Context, req *pb.LatestSessionRequest) (*pb.LatestSessionResponse, error) {
	act, ok := ActorFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.PermissionDenied, "actor not resolved")
	}
	sessionID, messages, err := handler.FetchLatestSession(ctx, s.sessionSvc, act)
	if err != nil {
		return nil, status.Error(codes.Internal, "取得對話失敗")
	}
	return toLatestSessionResponse(sessionID, messages), nil
}

func toLatestSessionResponse(sessionID string, messages []handler.LatestSessionMessage) *pb.LatestSessionResponse {
	pbMessages := make([]*pb.LatestSessionMessage, 0, len(messages))
	for _, m := range messages {
		pbMessages = append(pbMessages, &pb.LatestSessionMessage{Role: m.Role, Text: m.Text, Timestamp: m.Timestamp})
	}
	return &pb.LatestSessionResponse{SessionId: sessionID, Messages: pbMessages}
}

func init() {
	seller := ActorSeller
	buyer := ActorBuyer
	registerMethodPolicy("/hawkr.ai.AIAssistantService/SellerChat", MethodPolicy{Actor: seller})
	registerMethodPolicy("/hawkr.ai.AIAssistantService/SellerInventory", MethodPolicy{Actor: seller})
	registerMethodPolicy("/hawkr.ai.AIAssistantService/SellerPayment", MethodPolicy{Actor: seller})
	registerMethodPolicy("/hawkr.ai.AIAssistantService/SellerProductListingChat", MethodPolicy{Actor: seller})
	registerMethodPolicy("/hawkr.ai.AIAssistantService/SellerOrders", MethodPolicy{Actor: seller})
	registerMethodPolicy("/hawkr.ai.AIAssistantService/BuyerOrders", MethodPolicy{Actor: buyer})
	registerMethodPolicy("/hawkr.ai.AIAssistantService/BuyerTracking", MethodPolicy{Actor: buyer})
	registerMethodPolicy("/hawkr.ai.AIAssistantService/BuyerSellerMatch", MethodPolicy{Actor: buyer})
	registerMethodPolicy("/hawkr.ai.AIAssistantService/BuyerChat", MethodPolicy{Actor: buyer})
	registerMethodPolicy("/hawkr.ai.AIAssistantService/BuyerReturnPolicy", MethodPolicy{Actor: buyer})
	registerMethodPolicy("/hawkr.ai.AIAssistantService/SellerProductAutofill", MethodPolicy{Actor: seller})
	registerMethodPolicy("/hawkr.ai.AIAssistantService/SellerLatestSession", MethodPolicy{Actor: seller})
	registerMethodPolicy("/hawkr.ai.AIAssistantService/BuyerLatestSession", MethodPolicy{Actor: buyer})
}
