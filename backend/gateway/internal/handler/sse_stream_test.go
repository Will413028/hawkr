package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Will413028/hawkr/gateway/internal/buyerid"
	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	"github.com/Will413028/hawkr/gateway/internal/realtime"
	"github.com/Will413028/hawkr/gateway/internal/storeid"
	socialpb "github.com/Will413028/hawkr/go-pkg/proto/social"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

func setupSSEStreamRouter(h *Holder) *gin.Engine {
	r := gin.New()
	v1 := r.Group("/api/v1")
	pub := v1.Group("/public/posts")
	pub.Use(fakeOptionalAuth())
	{
		pub.GET("/:id/comments/stream", h.StreamPostComments)
	}
	app := v1.Group("/app/notifications")
	app.Use(fakeOptionalAuth())
	{
		app.GET("/stream", h.StreamNotifications)
	}
	return r
}

// spyChannel 攔在 serveSSE 的第一步（AuthorizeSubscribe），記下**真正送進去的
// topic 字串**後回 ErrForbidden 讓 handler 在觸碰 h.SSEHub（測試未設置）之前
// 提早 return。錨下在 topic 字串本身：身分空間錯掉時壞的正是它，而 handler
// 其餘流程（Hub、write loop）與本 bug 無關。
type spyChannel struct {
	realtime.Channel
	topic     string
	accountID int64
	called    bool
}

func (s *spyChannel) AuthorizeSubscribe(_ context.Context, topic string, accountID int64) error {
	s.topic = topic
	s.accountID = accountID
	s.called = true
	return realtime.ErrForbidden
}

// TestStreamNotifications_TopicUsesInboxIdentitySpace 釘住訂閱端與發布端的身分空間一致。
//
// 發布端（realtime/notification_channel.go ResolveTopic）從事件 payload 的 userId
// 建 topic，而該值 = notifications.user_id ⇒ 買家是 buyer_id。訂閱端若用 JWT 的
// account_id，只要買家 re-key 過（buyer_id != account_id）就永遠對不上，SSE 靜默
// 失效而 REST 輪詢照常有資料。
func TestStreamNotifications_TopicUsesInboxIdentitySpace(t *testing.T) {
	tests := []struct {
		name          string
		loginRole     string
		buyerResolver *buyerid.Resolver
		storeResolver *storeid.Resolver
		wantTopic     string
		wantSubj      int64
	}{
		{
			// 買家：JWT account_id=42，當前 buyer_id=9999 ⇒ topic 必須是 9999。
			name:          "buyer uses current buyer_id",
			loginRole:     "1",
			buyerResolver: buyerResolverForTest(9999),
			storeResolver: nil,
			wantTopic:     "notification:user:9999",
			wantSubj:      9999,
		},
		{
			// 賣家：JWT account_id=42，當前 store_id=8001 ⇒ topic 必須是 8001
			// （2026-08-30 修復：賣家側也改成解析 store surrogate）。
			name:          "seller uses current store_id",
			loginRole:     "2",
			buyerResolver: nil,
			storeResolver: storeResolverForTest(8001),
			wantTopic:     "notification:user:8001",
			wantSubj:      8001,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spy := &spyChannel{}
			h := &Holder{
				BuyerID:      tt.buyerResolver,
				Store:        tt.storeResolver,
				NotifChannel: spy,
			}
			r := setupSSEStreamRouter(h)

			req := httptest.NewRequest("GET", "/api/v1/app/notifications/stream", nil)
			req.Header.Set("X-Account-Id", "42")
			req.Header.Set("X-Login-Role", tt.loginRole)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assertHTTPStatus(t, w, http.StatusForbidden)
			if !spy.called {
				t.Fatalf("AuthorizeSubscribe 未被呼叫，topic 沒有被建出來")
			}
			if spy.topic != tt.wantTopic {
				t.Errorf("SSE topic = %q, want %q（訂閱端身分空間必須與 notifications.user_id 一致）", spy.topic, tt.wantTopic)
			}
			if spy.accountID != tt.wantSubj {
				t.Errorf("授權主體 = %d, want %d（必須與 topic 同一個 id，否則 AuthorizeSubscribe 恆 403）", spy.accountID, tt.wantSubj)
			}
		})
	}
}

// TestStreamNotifications_NoCurrentBuyer 買家解析不到現任 buyer_id 時走 403，
// 不得退回 account_id 訂一個永遠收不到訊息的 topic。
func TestStreamNotifications_NoCurrentBuyer(t *testing.T) {
	spy := &spyChannel{}
	h := &Holder{BuyerID: noBuyerResolverForTest(), NotifChannel: spy}
	r := setupSSEStreamRouter(h)

	req := httptest.NewRequest("GET", "/api/v1/app/notifications/stream", nil)
	req.Header.Set("X-Account-Id", "42")
	req.Header.Set("X-Login-Role", "1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusForbidden)
	if spy.called {
		t.Errorf("解析失敗仍建了 topic %q —— 應在建 topic 前就 return", spy.topic)
	}
}

// TestStreamNotifications_NoCurrentStore 賣家解析不到現任 store_id 時走 403，
// 不得退回 account_id 訂一個永遠收不到訊息的 topic（鏡像 NoCurrentBuyer）。
func TestStreamNotifications_NoCurrentStore(t *testing.T) {
	spy := &spyChannel{}
	h := &Holder{Store: noStoreResolverForTest(), NotifChannel: spy}
	r := setupSSEStreamRouter(h)

	req := httptest.NewRequest("GET", "/api/v1/app/notifications/stream", nil)
	req.Header.Set("X-Account-Id", "42")
	req.Header.Set("X-Login-Role", "2")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusForbidden)
	if spy.called {
		t.Errorf("解析失敗仍建了 topic %q —— 應在建 topic 前就 return", spy.topic)
	}
}

// TestNotificationChannel_AuthorizeSubscribeRejectsForeignTopic 守住那道「今天恆等」
// 的 topic 比對。
//
// StreamNotifications 的 topic 與授權主體都出自同一個 streamUserID ⇒ 比對在那條路徑上
// 不可能失敗，所以沒有任何 handler 級測試碰得到它的失敗分支。而它存在的理由恰恰是**下一個**
// 呼叫點：主體仍由 JWT 推導、topic 改成 caller 可控時（?topic=、Last-Event-ID 夾帶 topic、
// 新 endpoint 共用 serveSSE），它是唯一擋得住訂閱別人通知流的地方。
//
// 沒有這一支，它就是一段沒人驗過、註解卻說它在守的死碼——那比沒有這道檢查更糟，因為
// 假的保證會排擠掉真的警覺。測試放在 handler 而不是 realtime：這道檢查的意義由 serveSSE
// 怎麼餵它的兩個參數決定，而那件事在這裡。
func TestNotificationChannel_AuthorizeSubscribeRejectsForeignTopic(t *testing.T) {
	ch := realtime.NewNotificationChannel()
	tests := []struct {
		name      string
		topic     string
		accountID int64
		wantErr   bool
		reason    string
	}{
		{
			name:      "自己的 topic 放行",
			topic:     realtime.TopicNotificationUser(42),
			accountID: 42,
			wantErr:   false,
			reason:    "這是 StreamNotifications 今天唯一走得到的組合，擋掉它等於整條通知流失效",
		},
		{
			name:      "別人的 topic 擋下",
			topic:     realtime.TopicNotificationUser(9999),
			accountID: 42,
			wantErr:   true,
			reason:    "caller 可控 topic 的那一天，放行就是把別人的通知流交出去",
		},
		{
			name:      "匿名擋下",
			topic:     realtime.TopicNotificationUser(0),
			accountID: 0,
			wantErr:   true,
			reason:    "通知流沒有匿名訂閱者；accountID=0 是「未登入」而不是「使用者 0」",
		},
		{
			name:      "負數主體擋下",
			topic:     realtime.TopicNotificationUser(-1),
			accountID: -1,
			wantErr:   true,
			reason:    "只比對 topic 相等會讓負數主體自洽通過，所以 accountID 的下界要自己守",
		},
		{
			name:      "別的 channel 的 topic 擋下",
			topic:     realtime.TopicCommentPost(42),
			accountID: 42,
			wantErr:   true,
			reason:    "topic 命名空間不得跨 channel 借用，否則 Hub 上的分流形同虛設",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ch.AuthorizeSubscribe(context.Background(), tt.topic, tt.accountID)
			if tt.wantErr && err == nil {
				t.Errorf("AuthorizeSubscribe(%q, %d) = nil, want error —— %s", tt.topic, tt.accountID, tt.reason)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("AuthorizeSubscribe(%q, %d) = %v, want nil —— %s", tt.topic, tt.accountID, err, tt.reason)
			}
		})
	}
}

// newSSEStreamHolder wires a grandfather BuyerID default per newModerationHolder's
// convention (existing-behavior-preserving baseline for any test that doesn't
// override it).
func newSSEStreamHolder(mc *mockSocialServiceClient) *Holder {
	return &Holder{
		Social:  grpcclient.NewSocialClientForTest(mc),
		BuyerID: buyerGrandfatherResolverForTest(),
		Store:   noStoreResolverForTest(),
	}
}

// TestStreamPostComments_BuyerRoleResolvesCurrentBuyerID 用 GetPostDetail 回傳
// 「找不到貼文」讓 handler 在觸碰 h.SSEHub（測試未設置，會 nil-panic）之前提早
// return，藉此單獨驗證 buyer 身分解析邏輯而不需要起 SSE Hub。
func TestStreamPostComments_BuyerRoleResolvesCurrentBuyerID(t *testing.T) {
	var captured *socialpb.GetPostDetailRequest
	mc := &mockSocialServiceClient{
		GetPostDetailFn: func(ctx context.Context, in *socialpb.GetPostDetailRequest, opts ...grpc.CallOption) (*socialpb.PostDetailResponse, error) {
			captured = in
			return &socialpb.PostDetailResponse{Code: 1, Message: "not found"}, nil
		},
	}
	h := newSSEStreamHolder(mc)
	h.BuyerID = buyerResolverForTest(9999)
	r := setupSSEStreamRouter(h)

	req := httptest.NewRequest("GET", "/api/v1/public/posts/123/comments/stream", nil)
	req.Header.Set("X-Account-Id", "42")
	req.Header.Set("X-Login-Role", "1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusNotFound)
	if captured == nil || captured.BuyerId != 9999 {
		t.Errorf("captured.BuyerId = %v, want 9999 (current buyer_id, not raw account_id 42)", captured)
	}
}

func TestStreamPostComments_Anonymous_NoPersonalization(t *testing.T) {
	var captured *socialpb.GetPostDetailRequest
	mc := &mockSocialServiceClient{
		GetPostDetailFn: func(ctx context.Context, in *socialpb.GetPostDetailRequest, opts ...grpc.CallOption) (*socialpb.PostDetailResponse, error) {
			captured = in
			return &socialpb.PostDetailResponse{Code: 1, Message: "not found"}, nil
		},
	}
	h := newSSEStreamHolder(mc)
	h.BuyerID = mustNotResolveBuyer(t)
	r := setupSSEStreamRouter(h)

	req := httptest.NewRequest("GET", "/api/v1/public/posts/123/comments/stream", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusNotFound)
	if captured == nil || captured.BuyerId != 0 {
		t.Errorf("captured.BuyerId = %v, want 0 (anonymous)", captured)
	}
}
