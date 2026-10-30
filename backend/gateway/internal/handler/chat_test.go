package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	pb "github.com/Will413028/hawkr/go-pkg/proto/im"
	sellerpb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

// ==================== Mock IMServiceClient ====================

type mockIMServiceClient struct {
	SendMessageFn      func(ctx context.Context, in *pb.SendMessageRequest, opts ...grpc.CallOption) (*pb.SendMessageResponse, error)
	GetChatHistoryFn   func(ctx context.Context, in *pb.GetChatHistoryRequest, opts ...grpc.CallOption) (*pb.GetChatHistoryResponse, error)
	GetConversationsFn func(ctx context.Context, in *pb.GetConversationsRequest, opts ...grpc.CallOption) (*pb.GetConversationsResponse, error)
}

func (m *mockIMServiceClient) SendMessage(ctx context.Context, in *pb.SendMessageRequest, opts ...grpc.CallOption) (*pb.SendMessageResponse, error) {
	if m.SendMessageFn != nil {
		return m.SendMessageFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockIMServiceClient) BroadcastMessage(ctx context.Context, in *pb.BroadcastRequest, opts ...grpc.CallOption) (*pb.BroadcastResponse, error) {
	return nil, nil
}
func (m *mockIMServiceClient) IsUserOnline(ctx context.Context, in *pb.UserOnlineRequest, opts ...grpc.CallOption) (*pb.UserOnlineResponse, error) {
	return nil, nil
}
func (m *mockIMServiceClient) GetConnectionStats(ctx context.Context, in *pb.EmptyRequest, opts ...grpc.CallOption) (*pb.ConnectionStatsResponse, error) {
	return nil, nil
}
func (m *mockIMServiceClient) DisconnectUser(ctx context.Context, in *pb.DisconnectRequest, opts ...grpc.CallOption) (*pb.DisconnectResponse, error) {
	return nil, nil
}
func (m *mockIMServiceClient) GetUnreadCount(ctx context.Context, in *pb.GetUnreadCountRequest, opts ...grpc.CallOption) (*pb.GetUnreadCountResponse, error) {
	return nil, nil
}
func (m *mockIMServiceClient) GetChatHistory(ctx context.Context, in *pb.GetChatHistoryRequest, opts ...grpc.CallOption) (*pb.GetChatHistoryResponse, error) {
	if m.GetChatHistoryFn != nil {
		return m.GetChatHistoryFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockIMServiceClient) GetConversations(ctx context.Context, in *pb.GetConversationsRequest, opts ...grpc.CallOption) (*pb.GetConversationsResponse, error) {
	if m.GetConversationsFn != nil {
		return m.GetConversationsFn(ctx, in, opts...)
	}
	return nil, nil
}
func (m *mockIMServiceClient) MarkMessagesRead(ctx context.Context, in *pb.MarkMessagesReadRequest, opts ...grpc.CallOption) (*pb.MarkMessagesReadResponse, error) {
	return nil, nil
}
func (m *mockIMServiceClient) MarkConversationRead(ctx context.Context, in *pb.MarkConversationReadRequest, opts ...grpc.CallOption) (*pb.MarkConversationReadResponse, error) {
	return nil, nil
}
func (m *mockIMServiceClient) MarkAllRead(ctx context.Context, in *pb.MarkAllReadRequest, opts ...grpc.CallOption) (*pb.MarkAllReadResponse, error) {
	return nil, nil
}
func (m *mockIMServiceClient) HideMessage(ctx context.Context, in *pb.HideMessageRequest, opts ...grpc.CallOption) (*pb.HideMessageResponse, error) {
	return nil, nil
}
func (m *mockIMServiceClient) ChatStream(ctx context.Context, opts ...grpc.CallOption) (grpc.BidiStreamingClient[pb.ChatStreamRequest, pb.ChatStreamResponse], error) {
	return nil, nil
}

// ==================== Router Setup ====================

func setupChatRouter(h *Holder) *gin.Engine {
	r := gin.New()
	v1 := r.Group("/api/v1/app/chat")
	v1.Use(fakeJWTAuth())
	{
		v1.POST("/messages", h.SendChatMessage)
		v1.GET("/history/:targetId", h.GetChatHistory)
		v1.GET("/conversations", h.GetConversations)
	}
	return r
}

// ==================== GetChatHistory ====================

func TestGetChatHistory_Success_WithBeforeId(t *testing.T) {
	mock := &mockIMServiceClient{
		GetChatHistoryFn: func(ctx context.Context, in *pb.GetChatHistoryRequest, opts ...grpc.CallOption) (*pb.GetChatHistoryResponse, error) {
			if in.UserId != "42" {
				t.Errorf("UserId = %s, want 42", in.UserId)
			}
			if in.TargetId != "857045288972403572" {
				t.Errorf("TargetId = %s, want 857045288972403572", in.TargetId)
			}
			if in.BeforeId != "843856384063471789" {
				t.Errorf("BeforeId = %s, want 843856384063471789", in.BeforeId)
			}
			if in.PageSize != 10 {
				t.Errorf("PageSize = %d, want 10", in.PageSize)
			}
			return &pb.GetChatHistoryResponse{
				Code: 0,
				Messages: []*pb.ChatMessage{
					{Id: "499", FromUserId: "42", ToUserId: "100", Type: pb.MessageType_MESSAGE_TYPE_TEXT, Content: "hello", Timestamp: "1700000000", IsRead: true, SeqId: "10"},
					{Id: "498", FromUserId: "100", ToUserId: "42", Type: pb.MessageType_MESSAGE_TYPE_IMAGE, Content: "img.jpg", Timestamp: "1699999000", IsRead: false, SeqId: "9"},
				},
				Total:   "50",
				HasMore: true,
			}, nil
		},
	}
	h := &Holder{IM: grpcclient.NewIMClientForTest(mock), BuyerID: buyerGrandfatherResolverForTest()}

	r := setupChatRouter(h)
	req, _ := http.NewRequest("GET", "/api/v1/app/chat/history/857045288972403572?beforeId=843856384063471789&pageSize=10", nil)
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)

	data := body["data"].(map[string]interface{})
	messages := data["messages"].([]interface{})
	if len(messages) != 2 {
		t.Errorf("len(messages) = %d, want 2", len(messages))
	}
	if data["hasMore"] != true {
		t.Errorf("hasMore = %v, want true", data["hasMore"])
	}

	msg0 := messages[0].(map[string]interface{})
	if msg0["id"] != "499" {
		t.Errorf("messages[0].id = %v, want 499", msg0["id"])
	}
	if msg0["content"] != "hello" {
		t.Errorf("messages[0].content = %v, want hello", msg0["content"])
	}
	if msg0["seqId"] != "10" {
		t.Errorf("messages[0].seqId = %v, want 10", msg0["seqId"])
	}

	msg1 := messages[1].(map[string]interface{})
	if msg1["seqId"] != "9" {
		t.Errorf("messages[1].seqId = %v, want 9", msg1["seqId"])
	}
}

func TestGetChatHistory_Success_WithoutBeforeId(t *testing.T) {
	mock := &mockIMServiceClient{
		GetChatHistoryFn: func(ctx context.Context, in *pb.GetChatHistoryRequest, opts ...grpc.CallOption) (*pb.GetChatHistoryResponse, error) {
			if in.BeforeId != "" {
				t.Errorf("BeforeId = %s, want empty (not provided)", in.BeforeId)
			}
			if in.PageSize != 10 {
				t.Errorf("PageSize = %d, want 10 (default)", in.PageSize)
			}
			return &pb.GetChatHistoryResponse{
				Code:     0,
				Messages: []*pb.ChatMessage{},
				Total:    "0",
				HasMore:  false,
			}, nil
		},
	}
	h := &Holder{IM: grpcclient.NewIMClientForTest(mock), BuyerID: buyerGrandfatherResolverForTest()}

	r := setupChatRouter(h)
	req, _ := http.NewRequest("GET", "/api/v1/app/chat/history/100", nil)
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)
}

func TestGetChatHistory_NonNumericTargetIdPassedThrough(t *testing.T) {
	// path id 改 string 透傳後 gateway 不再驗證數字格式；targetId 原樣傳給
	// im-service（由其判定查無 → 空結果）。驗證 gateway 不再對非數字 path 回 400。
	var captured *pb.GetChatHistoryRequest
	mock := &mockIMServiceClient{
		GetChatHistoryFn: func(ctx context.Context, in *pb.GetChatHistoryRequest, opts ...grpc.CallOption) (*pb.GetChatHistoryResponse, error) {
			captured = in
			return &pb.GetChatHistoryResponse{Code: 0, Messages: []*pb.ChatMessage{}, Total: "0"}, nil
		},
	}
	h := &Holder{IM: grpcclient.NewIMClientForTest(mock), BuyerID: buyerGrandfatherResolverForTest()}
	r := setupChatRouter(h)
	req, _ := http.NewRequest("GET", "/api/v1/app/chat/history/abc", nil)
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.TargetId != "abc" {
		t.Errorf("captured TargetId = %v, want abc (透傳)", captured)
	}
}

func TestGetChatHistory_Unauthorized(t *testing.T) {
	h := &Holder{}
	r := setupChatRouter(h)
	req, _ := http.NewRequest("GET", "/api/v1/app/chat/history/100", nil)
	// No X-Account-Id header
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusUnauthorized)
}

func TestGetChatHistory_GrpcError(t *testing.T) {
	mock := &mockIMServiceClient{
		GetChatHistoryFn: func(ctx context.Context, in *pb.GetChatHistoryRequest, opts ...grpc.CallOption) (*pb.GetChatHistoryResponse, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}
	h := &Holder{IM: grpcclient.NewIMClientForTest(mock), BuyerID: buyerGrandfatherResolverForTest()}

	r := setupChatRouter(h)
	req, _ := http.NewRequest("GET", "/api/v1/app/chat/history/100", nil)
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusServiceUnavailable)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 503)
}

// ==================== SendChatMessage ====================

func TestSendChatMessage_Success_ReturnsSeqId(t *testing.T) {
	mock := &mockIMServiceClient{
		SendMessageFn: func(ctx context.Context, in *pb.SendMessageRequest, opts ...grpc.CallOption) (*pb.SendMessageResponse, error) {
			if in.FromUserId != "42" {
				t.Errorf("FromUserId = %s, want 42", in.FromUserId)
			}
			if in.ToUserId != "857045288972403572" {
				t.Errorf("ToUserId = %s, want 857045288972403572", in.ToUserId)
			}
			if in.Content != "hi" {
				t.Errorf("Content = %q, want %q", in.Content, "hi")
			}
			return &pb.SendMessageResponse{
				Code:      0,
				Message:   "success",
				Delivered: true,
				MessageId: "789",
				SeqId:     "15",
				Timestamp: "1700000000",
			}, nil
		},
	}
	h := &Holder{
		IM:      grpcclient.NewIMClientForTest(mock),
		BuyerID: buyerGrandfatherResolverForTest(),
		Seller:  sellerClientResolvingStoreIDForTest("857045288972403572"),
	}

	reqBody, _ := json.Marshal(map[string]interface{}{
		"toUserId": "857045288972403572",
		"type":     "text",
		"content":  "hi",
	})

	r := setupChatRouter(h)
	req, _ := http.NewRequest("POST", "/api/v1/app/chat/messages", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)

	data := body["data"].(map[string]interface{})
	if data["messageId"] != "789" {
		t.Errorf("messageId = %v, want 789", data["messageId"])
	}
	if data["seqId"] != "15" {
		t.Errorf("seqId = %v, want 15", data["seqId"])
	}
	if data["delivered"] != true {
		t.Errorf("delivered = %v, want true", data["delivered"])
	}
}

// 買家傳訊的對象必須曾經公開過（has_public_card）；從沒公開過的店回 404，
// 且不得呼叫 im-service，否則對話列表會洩漏沒公開過的店名。
func TestSendChatMessage_NeverPublicStore_Returns404(t *testing.T) {
	mock := &mockIMServiceClient{
		SendMessageFn: func(ctx context.Context, in *pb.SendMessageRequest, opts ...grpc.CallOption) (*pb.SendMessageResponse, error) {
			t.Fatal("im-service must not be called for a store that was never public")
			return nil, nil
		},
	}
	h := &Holder{
		IM:      grpcclient.NewIMClientForTest(mock),
		BuyerID: buyerGrandfatherResolverForTest(),
		Seller: grpcclient.NewSellerClientForTest(&mockSellerServiceClient{
			GetSellerFn: func(ctx context.Context, in *sellerpb.GetSellerRequest, opts ...grpc.CallOption) (*sellerpb.SellerResponse, error) {
				return &sellerpb.SellerResponse{Data: &sellerpb.SellerInfo{StoreId: "100"}}, nil
			},
			BatchGetSellerMetaFn: func(ctx context.Context, in *sellerpb.BatchGetSellerMetaRequest, opts ...grpc.CallOption) (*sellerpb.BatchGetSellerMetaResponse, error) {
				return &sellerpb.BatchGetSellerMetaResponse{Sellers: map[int64]*sellerpb.SellerMeta{
					100: {CanAcceptOrders: false, HasPublicCard: false},
				}}, nil
			},
		}),
	}

	reqBody, _ := json.Marshal(map[string]interface{}{"toUserId": "100", "content": "hi"})
	r := setupChatRouter(h)
	req, _ := http.NewRequest("POST", "/api/v1/app/chat/messages", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusNotFound)
}

// 暫停中（此刻不可曝光）但曾經公開過的店，買家仍可聯絡（售後）。
func TestSendChatMessage_SuspendedButOncePublicStore_Succeeds(t *testing.T) {
	var sent *pb.SendMessageRequest
	mock := &mockIMServiceClient{
		SendMessageFn: func(ctx context.Context, in *pb.SendMessageRequest, opts ...grpc.CallOption) (*pb.SendMessageResponse, error) {
			sent = in
			return &pb.SendMessageResponse{Code: 0, Delivered: true, Timestamp: "1700000000"}, nil
		},
	}
	h := &Holder{
		IM:      grpcclient.NewIMClientForTest(mock),
		BuyerID: buyerGrandfatherResolverForTest(),
		Seller: grpcclient.NewSellerClientForTest(&mockSellerServiceClient{
			GetSellerFn: func(ctx context.Context, in *sellerpb.GetSellerRequest, opts ...grpc.CallOption) (*sellerpb.SellerResponse, error) {
				return &sellerpb.SellerResponse{Data: &sellerpb.SellerInfo{StoreId: "100"}}, nil
			},
			BatchGetSellerMetaFn: func(ctx context.Context, in *sellerpb.BatchGetSellerMetaRequest, opts ...grpc.CallOption) (*sellerpb.BatchGetSellerMetaResponse, error) {
				return &sellerpb.BatchGetSellerMetaResponse{Sellers: map[int64]*sellerpb.SellerMeta{
					100: {CanAcceptOrders: false, HasPublicCard: true},
				}}, nil
			},
		}),
	}

	reqBody, _ := json.Marshal(map[string]interface{}{"toUserId": "100", "content": "hi"})
	r := setupChatRouter(h)
	req, _ := http.NewRequest("POST", "/api/v1/app/chat/messages", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if sent == nil || sent.ToUserId != "100" {
		t.Errorf("im-service request = %v, want ToUserId 100", sent)
	}
}

// 賣家不存在（seller 模組回 NotFound）時必須直接回錯，不能讓 room_id 存進
// 一個查無資料的代理鍵——那樣訊息「送出成功」但對話列表永遠空白，使用者拿不到
// 任何錯誤訊號。狀態碼比照 requireStore 既有慣例（見
// TestResolveStorefrontSellerID_NotFound）。
func TestSendChatMessage_UnknownSeller_Returns404(t *testing.T) {
	mock := &mockIMServiceClient{
		SendMessageFn: func(ctx context.Context, in *pb.SendMessageRequest, opts ...grpc.CallOption) (*pb.SendMessageResponse, error) {
			t.Fatal("im-service must not be called when the seller id can't be resolved")
			return nil, nil
		},
	}
	h := &Holder{
		IM:      grpcclient.NewIMClientForTest(mock),
		BuyerID: buyerGrandfatherResolverForTest(),
		Seller: grpcclient.NewSellerClientForTest(&mockSellerServiceClient{
			GetSellerFn: func(ctx context.Context, in *sellerpb.GetSellerRequest, opts ...grpc.CallOption) (*sellerpb.SellerResponse, error) {
				return nil, grpcstatus.Error(grpccodes.NotFound, "seller not found")
			},
		}),
	}

	reqBody, _ := json.Marshal(map[string]interface{}{
		"toUserId": "999999999",
		"content":  "hi",
	})

	r := setupChatRouter(h)
	req, _ := http.NewRequest("POST", "/api/v1/app/chat/messages", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusNotFound)
}

func TestSendChatMessage_GrpcError(t *testing.T) {
	mock := &mockIMServiceClient{
		SendMessageFn: func(ctx context.Context, in *pb.SendMessageRequest, opts ...grpc.CallOption) (*pb.SendMessageResponse, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}
	h := &Holder{
		IM:      grpcclient.NewIMClientForTest(mock),
		BuyerID: buyerGrandfatherResolverForTest(),
		Seller:  sellerClientResolvingStoreIDForTest("100"),
	}

	reqBody, _ := json.Marshal(map[string]interface{}{
		"toUserId": "100",
		"content":  "hi",
	})

	r := setupChatRouter(h)
	req, _ := http.NewRequest("POST", "/api/v1/app/chat/messages", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusServiceUnavailable)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 503)
}

func TestSendChatMessage_BadRequest(t *testing.T) {
	mock := &mockIMServiceClient{}
	h := &Holder{IM: grpcclient.NewIMClientForTest(mock), BuyerID: buyerGrandfatherResolverForTest()}

	// Missing required "content" field
	reqBody, _ := json.Marshal(map[string]interface{}{
		"toUserId": "100",
	})

	r := setupChatRouter(h)
	req, _ := http.NewRequest("POST", "/api/v1/app/chat/messages", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 400)
}

// ==================== GetConversations ====================

func TestGetConversations_Success_WithUnreadCount(t *testing.T) {
	mock := &mockIMServiceClient{
		GetConversationsFn: func(ctx context.Context, in *pb.GetConversationsRequest, opts ...grpc.CallOption) (*pb.GetConversationsResponse, error) {
			if in.UserId != "42" {
				t.Errorf("UserId = %s, want 42", in.UserId)
			}
			if in.Page != 1 {
				t.Errorf("Page = %d, want 1", in.Page)
			}
			if in.PageSize != 10 {
				t.Errorf("PageSize = %d, want 10", in.PageSize)
			}
			return &pb.GetConversationsResponse{
				Code: 0,
				Conversations: []*pb.Conversation{
					{
						TargetUserId:    "100",
						TargetNickname:  "Shop A",
						TargetAvatarKey: "https://example.com/a.jpg",
						LastMessage:     "hi there",
						LastMessageType: pb.MessageType_MESSAGE_TYPE_TEXT,
						LastMessageTime: "1700000000",
						UnreadCount:     "5",
					},
					{
						TargetUserId:    "200",
						TargetNickname:  "Shop B",
						TargetAvatarKey: "https://example.com/b.jpg",
						LastMessage:     "order shipped",
						LastMessageType: pb.MessageType_MESSAGE_TYPE_TEXT,
						LastMessageTime: "1699999000",
						UnreadCount:     "0",
					},
				},
				Total: "2",
			}, nil
		},
	}
	h := &Holder{IM: grpcclient.NewIMClientForTest(mock), BuyerID: buyerGrandfatherResolverForTest()}

	r := setupChatRouter(h)
	req, _ := http.NewRequest("GET", "/api/v1/app/chat/conversations", nil)
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)

	data := body["data"].(map[string]interface{})
	convs := data["conversations"].([]interface{})
	if len(convs) != 2 {
		t.Fatalf("len(conversations) = %d, want 2", len(convs))
	}

	conv0 := convs[0].(map[string]interface{})
	if conv0["unreadCount"] != "5" {
		t.Errorf("conversations[0].unreadCount = %v, want 5", conv0["unreadCount"])
	}
	if conv0["targetNickname"] != "Shop A" {
		t.Errorf("conversations[0].targetNickname = %v, want Shop A", conv0["targetNickname"])
	}
	if conv0["lastMessage"] != "hi there" {
		t.Errorf("conversations[0].lastMessage = %v, want hi there", conv0["lastMessage"])
	}

	conv1 := convs[1].(map[string]interface{})
	if conv1["unreadCount"] != "0" {
		t.Errorf("conversations[1].unreadCount = %v, want 0", conv1["unreadCount"])
	}
}

func TestGetConversations_Success_WithSellerStoreInfo(t *testing.T) {
	mock := &mockIMServiceClient{
		GetConversationsFn: func(ctx context.Context, in *pb.GetConversationsRequest, opts ...grpc.CallOption) (*pb.GetConversationsResponse, error) {
			return &pb.GetConversationsResponse{
				Code: 0,
				Conversations: []*pb.Conversation{
					{
						TargetUserId:    "100",
						TargetNickname:  "Cora",
						TargetAvatarKey: "https://cdn/avatar.jpg",
						StoreName:       "Hawkr 有機農場",
						LogoKey:         "https://cdn/logo.jpg",
						LastMessage:     "你好",
						LastMessageType: pb.MessageType_MESSAGE_TYPE_TEXT,
						LastMessageTime: "1700000000",
						UnreadCount:     "2",
					},
					{
						TargetUserId:    "200",
						TargetNickname:  "Buyer Bob",
						TargetAvatarKey: "https://cdn/bob.jpg",
						StoreName:       "",
						LogoKey:         "",
						LastMessage:     "hi",
						LastMessageType: pb.MessageType_MESSAGE_TYPE_TEXT,
						LastMessageTime: "1699999000",
						UnreadCount:     "0",
					},
				},
				Total: "2",
			}, nil
		},
	}
	h := &Holder{IM: grpcclient.NewIMClientForTest(mock), BuyerID: buyerGrandfatherResolverForTest()}

	r := setupChatRouter(h)
	req, _ := http.NewRequest("GET", "/api/v1/app/chat/conversations", nil)
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)

	data := body["data"].(map[string]interface{})
	convs := data["conversations"].([]interface{})
	if len(convs) != 2 {
		t.Fatalf("len(conversations) = %d, want 2", len(convs))
	}

	// Seller target should have storeName and logoUrl
	conv0 := convs[0].(map[string]interface{})
	if conv0["storeName"] != "Hawkr 有機農場" {
		t.Errorf("conversations[0].storeName = %v, want Hawkr 有機農場", conv0["storeName"])
	}
	if conv0["logoUrl"] != "https://cdn/logo.jpg" {
		t.Errorf("conversations[0].logoUrl = %v, want https://cdn/logo.jpg", conv0["logoUrl"])
	}

	// Buyer target should have empty storeName and logoUrl
	conv1 := convs[1].(map[string]interface{})
	if conv1["storeName"] != "" {
		t.Errorf("conversations[1].storeName = %v, want empty", conv1["storeName"])
	}
	if conv1["logoUrl"] != "" {
		t.Errorf("conversations[1].logoUrl = %v, want empty", conv1["logoUrl"])
	}
}

func TestGetChatHistory_Success_WithParticipants(t *testing.T) {
	mock := &mockIMServiceClient{
		GetChatHistoryFn: func(ctx context.Context, in *pb.GetChatHistoryRequest, opts ...grpc.CallOption) (*pb.GetChatHistoryResponse, error) {
			return &pb.GetChatHistoryResponse{
				Code: 0,
				Messages: []*pb.ChatMessage{
					{Id: "1", FromUserId: "42", ToUserId: "100", Type: pb.MessageType_MESSAGE_TYPE_TEXT, Content: "hello", Timestamp: "1700000000", SeqId: "1"},
				},
				Total:   "1",
				HasMore: false,
				Participants: []*pb.ChatParticipant{
					{UserId: "42", Role: "buyer", DisplayName: "Alice", AvatarKey: "https://cdn/alice.jpg"},
					{UserId: "100", Role: "seller", DisplayName: "Hawkr 有機農場", AvatarKey: "https://cdn/logo.jpg"},
				},
			}, nil
		},
	}
	h := &Holder{IM: grpcclient.NewIMClientForTest(mock), BuyerID: buyerGrandfatherResolverForTest()}

	r := setupChatRouter(h)
	req, _ := http.NewRequest("GET", "/api/v1/app/chat/history/100", nil)
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)

	data := body["data"].(map[string]interface{})
	participants := data["participants"].([]interface{})
	if len(participants) != 2 {
		t.Fatalf("len(participants) = %d, want 2", len(participants))
	}

	buyer := participants[0].(map[string]interface{})
	if buyer["role"] != "buyer" {
		t.Errorf("participants[0].role = %v, want buyer", buyer["role"])
	}
	// participants[].userId 與 messages[].fromUserId/toUserId 同一個 id 空間（買家
	// buyer_id／賣家 store_id，皆為 CURRENT stint 代理鍵），舊 key accountId 名稱謊報
	// 內容物，已移除——client 拿它當「登入 account_id」比對會對分岔帳號永久錯判。
	if buyer["userId"] != "42" {
		t.Errorf("participants[0].userId = %v, want 42", buyer["userId"])
	}
	if _, ok := buyer["accountId"]; ok {
		t.Error("participants[0] must not emit the legacy accountId key")
	}
	if buyer["displayName"] != "Alice" {
		t.Errorf("participants[0].displayName = %v, want Alice", buyer["displayName"])
	}

	seller := participants[1].(map[string]interface{})
	if seller["role"] != "seller" {
		t.Errorf("participants[1].role = %v, want seller", seller["role"])
	}
	if seller["userId"] != "100" {
		t.Errorf("participants[1].userId = %v, want 100", seller["userId"])
	}
	if _, ok := seller["accountId"]; ok {
		t.Error("participants[1] must not emit the legacy accountId key")
	}
	if seller["displayName"] != "Hawkr 有機農場" {
		t.Errorf("participants[1].displayName = %v, want Hawkr 有機農場", seller["displayName"])
	}
	if seller["avatarUrl"] != "https://cdn/logo.jpg" {
		t.Errorf("participants[1].avatarUrl = %v, want https://cdn/logo.jpg", seller["avatarUrl"])
	}
}

func TestGetConversations_LogoURLField(t *testing.T) {
	// Proves a seller-target conversation exposes the seller peer logo as the canonical
	// camelCase pair logoKey/logoUrl (49 §6.1). snake_case logo_key/logo_url is never emitted.
	mock := &mockIMServiceClient{
		GetConversationsFn: func(ctx context.Context, in *pb.GetConversationsRequest, opts ...grpc.CallOption) (*pb.GetConversationsResponse, error) {
			return &pb.GetConversationsResponse{
				Code: 0,
				Conversations: []*pb.Conversation{
					{
						TargetUserId:    "42",
						TargetNickname:  "老王鮮魚",
						TargetAvatarKey: "public/seller/42/avatar.png",
						StoreName:       "老王鮮魚",
						LogoKey:         "public/seller/42/logo.png",
						LastMessage:     "請問還有貨嗎？",
						LastMessageType: pb.MessageType_MESSAGE_TYPE_TEXT,
						LastMessageTime: "1700000000",
						UnreadCount:     "1",
					},
				},
				Total: "1",
			}, nil
		},
	}
	h := &Holder{IM: grpcclient.NewIMClientForTest(mock), BuyerID: buyerGrandfatherResolverForTest()}

	r := setupChatRouter(h)
	req, _ := http.NewRequest("GET", "/api/v1/app/chat/conversations", nil)
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)

	data := body["data"].(map[string]interface{})
	convs := data["conversations"].([]interface{})
	if len(convs) != 1 {
		t.Fatalf("len(conversations) = %d, want 1", len(convs))
	}

	conv := convs[0].(map[string]interface{})

	// Test that logoKey contains the exact proto value (canonical camelCase; 49 §6.1)
	if got := conv["logoKey"]; got != "public/seller/42/logo.png" {
		t.Errorf("logoKey = %v, want %q", got, "public/seller/42/logo.png")
	}

	// Test that logoUrl is present and non-empty
	logoURL := conv["logoUrl"]
	if logoURL == nil || logoURL == "" {
		t.Errorf("logoUrl = %v, want a non-empty built URL", logoURL)
	}
}

func TestGetConversations_GrpcError(t *testing.T) {
	mock := &mockIMServiceClient{
		GetConversationsFn: func(ctx context.Context, in *pb.GetConversationsRequest, opts ...grpc.CallOption) (*pb.GetConversationsResponse, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}
	h := &Holder{IM: grpcclient.NewIMClientForTest(mock), BuyerID: buyerGrandfatherResolverForTest()}

	r := setupChatRouter(h)
	req, _ := http.NewRequest("GET", "/api/v1/app/chat/conversations", nil)
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusServiceUnavailable)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 503)
}

func TestGetConversations_Unauthorized(t *testing.T) {
	h := &Holder{}
	r := setupChatRouter(h)
	req, _ := http.NewRequest("GET", "/api/v1/app/chat/conversations", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusUnauthorized)
}

// ==================== loginRole Regression (type=3 帳號角色隔離) ====================

// type=3 帳號 + loginRole=1 (buyer app 登入) → IM service 收到 accountType=1
func TestRegression_Type3_LoginRole_Buyer(t *testing.T) {
	var gotAccountType string
	mock := &mockIMServiceClient{
		GetConversationsFn: func(ctx context.Context, in *pb.GetConversationsRequest, opts ...grpc.CallOption) (*pb.GetConversationsResponse, error) {
			gotAccountType = in.AccountType
			return &pb.GetConversationsResponse{Code: 0}, nil
		},
	}
	h := &Holder{IM: grpcclient.NewIMClientForTest(mock), BuyerID: buyerGrandfatherResolverForTest()}
	r := setupChatRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/app/chat/conversations", nil)
	req.Header.Set("X-Account-Id", "100")
	req.Header.Set("X-Account-Type", "3")
	req.Header.Set("X-Login-Role", "1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if gotAccountType != "1" {
		t.Errorf("IM received accountType=%s, want 1 (buyer from loginRole)", gotAccountType)
	}
}

// type=3 帳號 + loginRole=2 (seller app 登入) → IM service 收到 accountType=2
func TestRegression_Type3_LoginRole_Seller(t *testing.T) {
	var gotAccountType string
	mock := &mockIMServiceClient{
		GetConversationsFn: func(ctx context.Context, in *pb.GetConversationsRequest, opts ...grpc.CallOption) (*pb.GetConversationsResponse, error) {
			gotAccountType = in.AccountType
			return &pb.GetConversationsResponse{Code: 0}, nil
		},
	}
	h := &Holder{IM: grpcclient.NewIMClientForTest(mock), Store: grandfatherResolverForTest()}
	r := setupChatRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/app/chat/conversations", nil)
	req.Header.Set("X-Account-Id", "100")
	req.Header.Set("X-Account-Type", "3")
	req.Header.Set("X-Login-Role", "2")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if gotAccountType != "2" {
		t.Errorf("IM received accountType=%s, want 2 (seller from loginRole)", gotAccountType)
	}
}

// type=3 帳號沒有 loginRole（舊 JWT）→ IM service 收到 accountType=""
func TestRegression_Type3_NoLoginRole_ReturnsZero(t *testing.T) {
	var gotAccountType string
	mock := &mockIMServiceClient{
		GetConversationsFn: func(ctx context.Context, in *pb.GetConversationsRequest, opts ...grpc.CallOption) (*pb.GetConversationsResponse, error) {
			gotAccountType = in.AccountType
			return &pb.GetConversationsResponse{Code: 0}, nil
		},
	}
	h := &Holder{IM: grpcclient.NewIMClientForTest(mock), BuyerID: buyerGrandfatherResolverForTest()}
	r := setupChatRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/app/chat/conversations", nil)
	req.Header.Set("X-Account-Id", "100")
	req.Header.Set("X-Account-Type", "3")
	// No loginRole (舊 JWT，還沒重新登入)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if gotAccountType != "" {
		t.Errorf("IM received accountType=%s, want empty (no loginRole in old JWT)", gotAccountType)
	}
}

// type=1 帳號自動判斷 buyer，loginRole 不影響
func TestRegression_Type1_IgnoresLoginRole(t *testing.T) {
	var gotAccountType string
	mock := &mockIMServiceClient{
		GetConversationsFn: func(ctx context.Context, in *pb.GetConversationsRequest, opts ...grpc.CallOption) (*pb.GetConversationsResponse, error) {
			gotAccountType = in.AccountType
			return &pb.GetConversationsResponse{Code: 0}, nil
		},
	}
	h := &Holder{IM: grpcclient.NewIMClientForTest(mock), BuyerID: buyerGrandfatherResolverForTest()}
	r := setupChatRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/app/chat/conversations", nil)
	req.Header.Set("X-Account-Id", "100")
	req.Header.Set("X-Account-Type", "1")
	req.Header.Set("X-Login-Role", "2") // loginRole=2 但 accountType=1，accountType 優先
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if gotAccountType != "1" {
		t.Errorf("IM received accountType=%s, want 1 (type=1 takes priority over loginRole)", gotAccountType)
	}
}

// ==================== chatWireIdentity substitution (X7) ====================

func TestSendChatMessage_SellerRole_SubstitutesSurrogate(t *testing.T) {
	imMock := &mockIMServiceClient{
		SendMessageFn: func(ctx context.Context, in *pb.SendMessageRequest, opts ...grpc.CallOption) (*pb.SendMessageResponse, error) {
			if in.FromUserId != "777" {
				t.Errorf("FromUserId = %q, want \"777\" (current store surrogate, not JWT 99)", in.FromUserId)
			}
			return &pb.SendMessageResponse{Delivered: true, Timestamp: "1710000000000"}, nil
		},
	}
	h := &Holder{
		IM:    grpcclient.NewIMClientForTest(imMock),
		Store: storeResolverForTest(777),
	}
	r := gin.New()
	r.POST("/api/v1/app/chat/messages", fakeSellerAuth(), h.SendChatMessage)
	req, _ := http.NewRequest("POST", "/api/v1/app/chat/messages",
		strings.NewReader(`{"toUserId":"123","content":"hi"}`))
	req.Header.Set("X-Account-Id", "99")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assertHTTPStatus(t, w, http.StatusOK)
}

func TestSendChatMessage_RejectsJSONNumberToUserId(t *testing.T) {
	// §4 防護：toUserId 為 string，前端誤送 JSON number（18-19 位 id 在 JS 已先
	// 精度丟失）必須被 binding 拒絕回 400，逼前端送 string。
	h := &Holder{IM: grpcclient.NewIMClientForTest(&mockIMServiceClient{}), BuyerID: buyerGrandfatherResolverForTest()}
	reqBody, _ := json.Marshal(map[string]interface{}{
		"toUserId": 857045288972403572,
		"content":  "hi",
	})
	r := setupChatRouter(h)
	req, _ := http.NewRequest("POST", "/api/v1/app/chat/messages", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assertHTTPStatus(t, w, http.StatusBadRequest)
}

func TestChatHandlers_PageParamParsing(t *testing.T) {
	scenarios := []struct {
		name         string
		query        string
		wantPageSize int32
	}{
		{"合法數字", "pageSize=5", 5},
		{"非數字字串 fallback 到 default", "pageSize=xyz", 10},
		{"空值 fallback 到 default", "", 10},
		{"負數 fallback 到 default", "pageSize=-5", 10},
	}

	t.Run("GetChatHistory", func(t *testing.T) {
		for _, tt := range scenarios {
			t.Run(tt.name, func(t *testing.T) {
				var captured *pb.GetChatHistoryRequest
				mock := &mockIMServiceClient{
					GetChatHistoryFn: func(ctx context.Context, in *pb.GetChatHistoryRequest, opts ...grpc.CallOption) (*pb.GetChatHistoryResponse, error) {
						captured = in
						return &pb.GetChatHistoryResponse{Code: 0}, nil
					},
				}
				// chatWireIdentity resolves via h.BuyerID for the default (non-seller)
				// account type — required here or GetChatHistory 401s before parsing
				// page params at all (confirmed by Read of this file's other
				// GetChatHistory/GetConversations tests, which all set this).
				h := &Holder{IM: grpcclient.NewIMClientForTest(mock), BuyerID: buyerGrandfatherResolverForTest()}
				r := setupChatRouter(h)
				url := "/api/v1/app/chat/history/857045288972403572"
				if tt.query != "" {
					url += "?" + tt.query
				}
				req := httptest.NewRequest(http.MethodGet, url, nil)
				req.Header.Set("X-Account-Id", "42")
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)

				assertHTTPStatus(t, w, http.StatusOK)
				if captured == nil {
					t.Fatal("GetChatHistory was not called")
				}
				if captured.PageSize != tt.wantPageSize {
					t.Errorf("PageSize = %d, want %d", captured.PageSize, tt.wantPageSize)
				}
			})
		}
	})

	pageScenarios := []struct {
		name         string
		query        string
		wantPage     int32
		wantPageSize int32
	}{
		{"合法數字", "page=3&pageSize=5", 3, 5},
		{"非數字字串 fallback 到 default", "page=abc&pageSize=xyz", 1, 10},
		{"空值 fallback 到 default", "", 1, 10},
		{"負數 fallback 到 default", "page=-1&pageSize=-5", 1, 10},
	}
	t.Run("GetConversations", func(t *testing.T) {
		for _, tt := range pageScenarios {
			t.Run(tt.name, func(t *testing.T) {
				var captured *pb.GetConversationsRequest
				mock := &mockIMServiceClient{
					GetConversationsFn: func(ctx context.Context, in *pb.GetConversationsRequest, opts ...grpc.CallOption) (*pb.GetConversationsResponse, error) {
						captured = in
						return &pb.GetConversationsResponse{Code: 0}, nil
					},
				}
				h := &Holder{IM: grpcclient.NewIMClientForTest(mock), BuyerID: buyerGrandfatherResolverForTest()}
				r := setupChatRouter(h)
				url := "/api/v1/app/chat/conversations"
				if tt.query != "" {
					url += "?" + tt.query
				}
				req := httptest.NewRequest(http.MethodGet, url, nil)
				req.Header.Set("X-Account-Id", "42")
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)

				assertHTTPStatus(t, w, http.StatusOK)
				if captured == nil {
					t.Fatal("GetConversations was not called")
				}
				if captured.Page != tt.wantPage || captured.PageSize != tt.wantPageSize {
					t.Errorf("Page/PageSize = %d/%d, want %d/%d", captured.Page, captured.PageSize, tt.wantPage, tt.wantPageSize)
				}
			})
		}
	})
}

// resolveChatTargetID 的 seller RPC 失敗一律 fail closed：不得打到 im-service。
func TestSendChatMessage_SellerRPCFailure_FailsClosed(t *testing.T) {
	okSeller := func(ctx context.Context, in *sellerpb.GetSellerRequest, opts ...grpc.CallOption) (*sellerpb.SellerResponse, error) {
		return &sellerpb.SellerResponse{Data: &sellerpb.SellerInfo{StoreId: "100"}}, nil
	}
	cases := map[string]*mockSellerServiceClient{
		"GetSeller RPC error": {GetSellerFn: func(ctx context.Context, in *sellerpb.GetSellerRequest, opts ...grpc.CallOption) (*sellerpb.SellerResponse, error) {
			return nil, grpcstatus.Error(grpccodes.Unavailable, "down")
		}},
		"BatchGetSellerMeta RPC error": {GetSellerFn: okSeller, BatchGetSellerMetaFn: func(ctx context.Context, in *sellerpb.BatchGetSellerMetaRequest, opts ...grpc.CallOption) (*sellerpb.BatchGetSellerMetaResponse, error) {
			return nil, grpcstatus.Error(grpccodes.Unavailable, "down")
		}},
		"BatchGetSellerMeta code != 0": {GetSellerFn: okSeller, BatchGetSellerMetaFn: func(ctx context.Context, in *sellerpb.BatchGetSellerMetaRequest, opts ...grpc.CallOption) (*sellerpb.BatchGetSellerMetaResponse, error) {
			return &sellerpb.BatchGetSellerMetaResponse{Code: 5000}, nil
		}},
		"meta entry missing": {GetSellerFn: okSeller, BatchGetSellerMetaFn: func(ctx context.Context, in *sellerpb.BatchGetSellerMetaRequest, opts ...grpc.CallOption) (*sellerpb.BatchGetSellerMetaResponse, error) {
			return &sellerpb.BatchGetSellerMetaResponse{Sellers: map[int64]*sellerpb.SellerMeta{}}, nil
		}},
	}
	for name, sm := range cases {
		t.Run(name, func(t *testing.T) {
			h := &Holder{
				IM: grpcclient.NewIMClientForTest(&mockIMServiceClient{
					SendMessageFn: func(ctx context.Context, in *pb.SendMessageRequest, opts ...grpc.CallOption) (*pb.SendMessageResponse, error) {
						t.Fatal("im-service must not be called when the target gate fails")
						return nil, nil
					},
				}),
				BuyerID: buyerGrandfatherResolverForTest(),
				Seller:  grpcclient.NewSellerClientForTest(sm),
			}
			reqBody, _ := json.Marshal(map[string]interface{}{"toUserId": "100", "content": "hi"})
			req, _ := http.NewRequest("POST", "/api/v1/app/chat/messages", bytes.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Account-Id", "42")
			w := httptest.NewRecorder()
			setupChatRouter(h).ServeHTTP(w, req)
			if w.Code == http.StatusOK && strings.Contains(w.Body.String(), `"code":0`) {
				t.Fatalf("gate failure must not succeed: %s", w.Body.String())
			}
		})
	}
}
