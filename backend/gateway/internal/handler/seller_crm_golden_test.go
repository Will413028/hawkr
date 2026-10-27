package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	crmpb "github.com/Will413028/hawkr/go-pkg/proto/crm"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// TestGolden_SellerCustomerDetail pins the exact JSON wire shape of
// GET /app/sellers/me/customers/:buyerId (GetCustomerStats success path).
// Every CrmCustomer field is populated so the snake→camel passthrough flip
// (Track 2) shows up in the golden diff as pure key-casing changes, nothing else.
func TestGolden_SellerCustomerDetail(t *testing.T) {
	mock := &shapeCrmMock{
		GetCustomerStatsFn: func(ctx context.Context, in *crmpb.GetCustomerStatsRequest, opts ...grpc.CallOption) (*crmpb.CustomerStatsResponse, error) {
			return &crmpb.CustomerStatsResponse{
				Code:    0,
				Message: "success",
				Data: &crmpb.CrmCustomer{
					Id:           "7301111111111111111",
					StoreId:      "7302222222222222222",
					BuyerId:      "7303333333333333333",
					CustomerType: "vip",
					TotalSpent:   "128000",
					OrderCount:   "12",
					Nickname:     "王小明",
					AvatarKey:    "https://cdn.example.com/avatars/u1.jpg",
					Tags: []*crmpb.Tag{
						{Id: "7304444444444444444", StoreId: "7302222222222222222", Name: "常客"},
					},
					Phone:              "0912345678",
					Address:            "台北市中正區忠孝東路一段1號",
					Email:              "wang@example.com",
					CouponCount:        "3",
					FrequentCategories: []string{"蔬菜", "水果"},
				},
			}, nil
		},
	}
	h := &Holder{Crm: grpcclient.NewCrmClientForTest(mock), Store: grandfatherResolverForTest(), BuyerID: buyerGrandfatherResolverForTest()}

	r := gin.New()
	v1 := r.Group("/api/v1/app/sellers/me")
	v1.Use(fakeSellerAuth())
	v1.GET("/customers/:buyerId", h.GetCustomerStats)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/sellers/me/customers/7303333333333333333", nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d body=%s", w.Code, w.Body.String())
	}
	assertGolden(t, "seller_customer_detail_success", w.Body.Bytes())
}

// TestGolden_SellerCustomerList pins the wire shape of
// GET /app/sellers/me/customers (GetCustomerList success path).
//
// Two things this golden is load-bearing for:
//  1. avatarKey/avatarUrl are BOTH present (49 §6.6 S9). crm-service only ever
//     returns the objectKey; if the Gateway stops composing the URL the seller
//     App's customer list goes back to blank avatars (it only renders
//     `startsWith('http')` values).
//  2. every key is camelCase, matching the detail endpoint. This was flipped
//     from snake_case together with the seller App's parser — a build that
//     only knows the snake names silently reads `id` (the crm row PK) as the
//     customer id instead. Any future rename here is likewise an
//     App-release-coupled change.
func TestGolden_SellerCustomerList(t *testing.T) {
	mock := &shapeCrmMock{
		GetCustomerListFn: func(ctx context.Context, in *crmpb.GetCustomerListRequest, opts ...grpc.CallOption) (*crmpb.CustomerListResponse, error) {
			return &crmpb.CustomerListResponse{
				Code:    0,
				Message: "ok",
				Data: &crmpb.CustomerListData{
					Total: "2",
					List: []*crmpb.CrmCustomer{
						{
							Id:           "7301111111111111111",
							StoreId:      "7302222222222222222",
							BuyerId:      "7303333333333333333",
							CustomerType: "vip",
							TotalSpent:   "128000",
							OrderCount:   "12",
							CouponCount:  "3",
							Nickname:     "王小明",
							AvatarKey:    "public/buyer_avatar/7303333333333333333/a.png",
							Phone:        "0912345678",
							Tags: []*crmpb.Tag{
								{Id: "7304444444444444444", StoreId: "7302222222222222222", Name: "常客"},
							},
						},
						{
							// 外部頭像（http 值）由 mediaurl.BuildURL passthrough，不再前綴一次 base。
							Id:           "7305555555555555555",
							StoreId:      "7302222222222222222",
							BuyerId:      "7306666666666666666",
							CustomerType: "normal",
							TotalSpent:   "0",
							OrderCount:   "0",
							CouponCount:  "0",
							Nickname:     "李小華",
							AvatarKey:    "https://cdn.example.com/avatars/u2.jpg",
							Phone:        "0987654321",
						},
					},
				},
			}, nil
		},
	}
	h := &Holder{
		Crm:          grpcclient.NewCrmClientForTest(mock),
		MediaBaseURL: "https://media.example.com",
		Store:        grandfatherResolverForTest(),
		BuyerID:      buyerGrandfatherResolverForTest(),
	}

	r := gin.New()
	v1 := r.Group("/api/v1/app/sellers/me")
	v1.Use(fakeSellerAuth())
	v1.GET("/customers", h.GetCustomerList)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/sellers/me/customers?page=1&page_size=20", nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d body=%s", w.Code, w.Body.String())
	}
	assertGolden(t, "seller_customer_list_success", w.Body.Bytes())
}
