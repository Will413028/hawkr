package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	accountpb "github.com/Will413028/hawkr/go-pkg/proto/account"
	orderpb "github.com/Will413028/hawkr/go-pkg/proto/order"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// §6.1.10 的 Query 契約是 page／page_size／status／keyword／date_from／date_to，但
// Gateway 曾經只讀 status ——其餘三個被 gin 靜默丟掉：請求回 200、列表是全集、沒有任何
// 錯誤訊號，操作者只會覺得「搜尋沒有作用」。沒有任何既有測試釘住這個參數集合，所以漏
// 掉一個不會有東西轉紅（漏了三個也一樣）。
//
// 這支測試從「Gateway 真的送了什麼」下手，而不是看回應長什麼樣——後者在漏帶時完全
// 正常，正是這個缺口無聲的原因。

func newDelegatedOrderListRouter(t *testing.T) (*gin.Engine, *[]*orderpb.GetSellerOrdersRequest) {
	t.Helper()
	return newDelegatedOrderListRouterReturning(t, &orderpb.GetSellerOrdersResponse{
		Code: 0,
		Meta: &orderpb.PageMeta{TotalItems: "0", CurrentPage: "1"},
	})
}

// newDelegatedOrderListRouterReturning 讓呼叫端決定上游回什麼，給那些要驗**回應內容**
// （而不只是驗送出去的請求）的測試用。
//
// 參數化而不是另起一份 router：session 落點與路由形狀是代行列表成立的前提，複製一份
// 出去就會有兩份各自漂移的前提，而漂掉的那一份會安靜地測到另一條路徑上。
func newDelegatedOrderListRouterReturning(t *testing.T, resp *orderpb.GetSellerOrdersResponse) (*gin.Engine, *[]*orderpb.GetSellerOrdersRequest) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	var seen []*orderpb.GetSellerOrdersRequest
	mock := &mockOrderServiceClient{
		GetSellerOrdersFn: func(_ context.Context, in *orderpb.GetSellerOrdersRequest, _ ...grpc.CallOption) (*orderpb.GetSellerOrdersResponse, error) {
			seen = append(seen, in)
			return resp, nil
		},
	}
	h := &Holder{Order: grpcclient.NewOrderClientForTest(mock)}

	r := gin.New()
	g := r.Group("/api/v1/admin")
	// 真實路徑上 session 由 requireDelegated guard 放進 context；測試只需要同一個落點。
	g.Use(func(c *gin.Context) {
		middleware.SetDelegationSession(c, &accountpb.OperationSessionSummary{
			SessionId: "2701445580000001",
			Actor:     &accountpb.DelegationActor{AdminId: "2701338845", DisplayName: "Cora Chen"},
			Target:    &accountpb.DelegationTarget{StoreId: "2685914682"},
			Status:    "active",
		})
		c.Next()
	})
	g.GET("/sellers/:store_id/orders", h.ListDelegatedOrders)
	return r, &seen
}

func getDelegatedOrders(t *testing.T, r *gin.Engine, query string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/admin/sellers/2685914682/orders?"+query, nil))
	return w
}

func TestListDelegatedOrders_ForwardsEveryContractQueryParam(t *testing.T) {
	cases := []struct {
		name                              string
		query                             string
		status, keyword, dateFrom, dateTo string
	}{
		{
			name:     "三個新參數與 status 一起帶下去",
			query:    "status=20&keyword=梨山&date_from=2026-08-01T00:00:00Z&date_to=2026-08-31T23:59:59Z",
			status:   "20",
			keyword:  "梨山",
			dateFrom: "2026-08-01T00:00:00Z",
			dateTo:   "2026-08-31T23:59:59Z",
		},
		{
			name:    "keyword 去頭尾空白",
			query:   "keyword=%20%20O005001417%20%20",
			keyword: "O005001417",
		},
		{
			name:    "只有空白的 keyword ＝ 不篩選",
			query:   "keyword=%20%20%20",
			keyword: "",
		},
		{
			name:  "三者皆未帶 ＝ 全部不篩選",
			query: "page=2",
		},
		{
			name:     "只帶 date_from 也要送下去（單邊區間合法）",
			query:    "date_from=2026-08-01T00:00:00Z",
			dateFrom: "2026-08-01T00:00:00Z",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, seen := newDelegatedOrderListRouter(t)
			if w := getDelegatedOrders(t, r, tc.query); w.Code != http.StatusOK {
				t.Fatalf("HTTP = %d, want 200；body=%s", w.Code, w.Body.String())
			}
			if len(*seen) != 1 {
				t.Fatalf("上游被呼叫 %d 次，want 1", len(*seen))
			}
			got := (*seen)[0]
			if got.GetStatusFilter() != tc.status {
				t.Errorf("status_filter = %q, want %q", got.GetStatusFilter(), tc.status)
			}
			if got.GetKeyword() != tc.keyword {
				t.Errorf("keyword = %q, want %q —— 沒帶下去的話搜尋框看起來有作用、實際回全集",
					got.GetKeyword(), tc.keyword)
			}
			if got.GetDateFrom() != tc.dateFrom {
				t.Errorf("date_from = %q, want %q", got.GetDateFrom(), tc.dateFrom)
			}
			if got.GetDateTo() != tc.dateTo {
				t.Errorf("date_to = %q, want %q", got.GetDateTo(), tc.dateTo)
			}
		})
	}
}

// 不合法的值一律 9260，而且必須在 gRPC 那一跳**之前**失敗：晚一步失敗會把一個輸入問題
// 變成一次上游來回，錯誤訊息也不再指得回是哪個參數寫錯。
func TestListDelegatedOrders_RejectsInvalidFiltersBeforeUpstream(t *testing.T) {
	cases := []struct {
		name  string
		query string
	}{
		{"keyword 超過 255 字", "keyword=" + strings.Repeat("梨", 256)},
		{"keyword 含控制字元", "keyword=%00"},
		{"date_from 非 RFC3339", "date_from=2026-08-01"},
		{"date_to 非 RFC3339", "date_to=2026%2F08%2F31"},
		{"date_from 晚於 date_to", "date_from=2026-08-31T00:00:00Z&date_to=2026-08-01T00:00:00Z"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, seen := newDelegatedOrderListRouter(t)
			w := getDelegatedOrders(t, r, tc.query)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("HTTP = %d, want 400；body=%s", w.Code, w.Body.String())
			}
			var body struct {
				Code int32 `json:"code"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("回應不是 JSON envelope：%v（body=%s）", err, w.Body.String())
			}
			if body.Code != bizcode.DelegationInvalidCommand {
				t.Errorf("code = %d, want %d", body.Code, bizcode.DelegationInvalidCommand)
			}
			if len(*seen) != 0 {
				t.Errorf("上游被呼叫了 %d 次 —— 值域檢查必須擋在 gRPC 之前", len(*seen))
			}
		})
	}
}

// 255 是「還可能有答案」的最後一點（product_name／sku_name／product.name 都是
// VARCHAR(255)），不是隨手挑的小數字：訂在更短的地方會擋掉合法查詢。
func TestListDelegatedOrders_KeywordAtMaxLengthIsAccepted(t *testing.T) {
	r, seen := newDelegatedOrderListRouter(t)
	kw := strings.Repeat("梨", delegatedKeywordMaxRunes)
	if w := getDelegatedOrders(t, r, "keyword="+kw); w.Code != http.StatusOK {
		t.Fatalf("HTTP = %d, want 200；body=%s", w.Code, w.Body.String())
	}
	if len(*seen) != 1 || (*seen)[0].GetKeyword() != kw {
		t.Errorf("剛好等於上限的 keyword 被擋掉了")
	}
}
