package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	buyerpb "github.com/Will413028/hawkr/go-pkg/proto/buyer"
	couponpb "github.com/Will413028/hawkr/go-pkg/proto/coupon"
	crmpb "github.com/Will413028/hawkr/go-pkg/proto/crm"
	orderpb "github.com/Will413028/hawkr/go-pkg/proto/order"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

type crmCustomerRekeyMock struct {
	crmpb.CrmServiceClient
	GetCustomerStatsFn     func(ctx context.Context, in *crmpb.GetCustomerStatsRequest, opts ...grpc.CallOption) (*crmpb.CustomerStatsResponse, error)
	UpdateCustomerStatusFn func(ctx context.Context, in *crmpb.UpdateCustomerStatusRequest, opts ...grpc.CallOption) (*crmpb.CrmResponse, error)
	AddCustomerTagsFn      func(ctx context.Context, in *crmpb.AddCustomerTagsRequest, opts ...grpc.CallOption) (*crmpb.CrmResponse, error)
	RemoveCustomerTagFn    func(ctx context.Context, in *crmpb.RemoveCustomerTagRequest, opts ...grpc.CallOption) (*crmpb.CrmResponse, error)
	GetCustomerTagsFn      func(ctx context.Context, in *crmpb.GetCustomerTagsRequest, opts ...grpc.CallOption) (*crmpb.TagListResponse, error)
}

func (m *crmCustomerRekeyMock) GetCustomerStats(ctx context.Context, in *crmpb.GetCustomerStatsRequest, opts ...grpc.CallOption) (*crmpb.CustomerStatsResponse, error) {
	return m.GetCustomerStatsFn(ctx, in, opts...)
}

func (m *crmCustomerRekeyMock) UpdateCustomerStatus(ctx context.Context, in *crmpb.UpdateCustomerStatusRequest, opts ...grpc.CallOption) (*crmpb.CrmResponse, error) {
	return m.UpdateCustomerStatusFn(ctx, in, opts...)
}

func (m *crmCustomerRekeyMock) AddCustomerTags(ctx context.Context, in *crmpb.AddCustomerTagsRequest, opts ...grpc.CallOption) (*crmpb.CrmResponse, error) {
	return m.AddCustomerTagsFn(ctx, in, opts...)
}

func (m *crmCustomerRekeyMock) RemoveCustomerTag(ctx context.Context, in *crmpb.RemoveCustomerTagRequest, opts ...grpc.CallOption) (*crmpb.CrmResponse, error) {
	return m.RemoveCustomerTagFn(ctx, in, opts...)
}

func (m *crmCustomerRekeyMock) GetCustomerTags(ctx context.Context, in *crmpb.GetCustomerTagsRequest, opts ...grpc.CallOption) (*crmpb.TagListResponse, error) {
	return m.GetCustomerTagsFn(ctx, in, opts...)
}

// TestCustomerHandlers_ForwardPathBuyerIDVerbatim covers EVERY route in the
// /customers/{buyerId}... family (all 8 registered in router/routes_crm.go —
// stats, status patch, meta, orders, coupons, and the three tag routes) and
// asserts each one forwards the path value downstream UNCHANGED.
//
// The path value used is a buyer_id from a CLOSED stint, and h.BuyerID is wired
// to a resolver that would translate anything into a different value. So if any
// handler still resolves — or a future one re-introduces its own inline
// resolution — the captured value comes back as currentButWrongBuyerID and the
// case fails. That double duty is deliberate: it pins the new "no resolution"
// semantics AND keeps guarding the #1519 drift class (one handler in the family
// treating the param differently from its siblings), which is the reason
// resolveCustomerBuyerID exists as a single entry point at all.
func TestCustomerHandlers_ForwardPathBuyerIDVerbatim(t *testing.T) {
	const (
		// 已關閉 stint 的 buyer_id：crm 列上掛的就是它，端點必須原值使用
		staleStintBuyerID = "2619321233"
		// 仍活著的 stint（buyer-service 供得出 profile），用來驗 /meta 的軸一致
		liveStintBuyerID    = "2619399999"
		liveStintBuyerIDInt = int64(2619399999)
		// resolver 會把任何輸入翻成這個值——新語意下沒有任何 handler 該碰它
		currentButWrongBuyerID = int64(7484139444629864823)
	)

	var captured string // last buyer id observed by a downstream mock

	crmMock := &crmCustomerRekeyMock{
		GetCustomerStatsFn: func(_ context.Context, in *crmpb.GetCustomerStatsRequest, _ ...grpc.CallOption) (*crmpb.CustomerStatsResponse, error) {
			captured = in.BuyerId
			return &crmpb.CustomerStatsResponse{Code: 0, Data: &crmpb.CrmCustomer{}}, nil
		},
		UpdateCustomerStatusFn: func(_ context.Context, in *crmpb.UpdateCustomerStatusRequest, _ ...grpc.CallOption) (*crmpb.CrmResponse, error) {
			captured = in.BuyerId
			return &crmpb.CrmResponse{Code: 0}, nil
		},
		AddCustomerTagsFn: func(_ context.Context, in *crmpb.AddCustomerTagsRequest, _ ...grpc.CallOption) (*crmpb.CrmResponse, error) {
			captured = in.BuyerId
			return &crmpb.CrmResponse{Code: 0}, nil
		},
		RemoveCustomerTagFn: func(_ context.Context, in *crmpb.RemoveCustomerTagRequest, _ ...grpc.CallOption) (*crmpb.CrmResponse, error) {
			captured = in.BuyerId
			return &crmpb.CrmResponse{Code: 0}, nil
		},
		GetCustomerTagsFn: func(_ context.Context, in *crmpb.GetCustomerTagsRequest, _ ...grpc.CallOption) (*crmpb.TagListResponse, error) {
			captured = in.BuyerId
			return &crmpb.TagListResponse{Code: 0}, nil
		},
	}
	// Total="1" 讓 /meta 的關聯檢查（B-198 IDOR gate）過關；orders 端點拿到
	// 同一份回應也照樣回 200。
	orderMock := &mockOrderServiceClient{
		GetBuyerOrdersForSellerFn: func(_ context.Context, in *orderpb.GetBuyerOrdersForSellerRequest, _ ...grpc.CallOption) (*orderpb.GetBuyerOrdersForSellerResponse, error) {
			captured = strconv.FormatInt(in.BuyerId, 10)
			return &orderpb.GetBuyerOrdersForSellerResponse{Code: 0, Total: "1", Page: "1", PageSize: "20"}, nil
		},
	}
	couponMock := &couponMockCouponSvcClient{
		ListBuyerCouponsForSellerFn: func(_ context.Context, in *couponpb.ListBuyerCouponsForSellerRequest, _ ...grpc.CallOption) (*couponpb.ListBuyerCouponsForSellerResponse, error) {
			captured = in.BuyerId
			return &couponpb.ListBuyerCouponsForSellerResponse{Code: 0, Total: "0", Page: "1", PageSize: "20"}, nil
		},
	}

	buyerMock := &mockBuyerServiceClient{
		BatchGetBuyerMetaByBuyerIDsFn: func(_ context.Context, in *buyerpb.BatchGetBuyerMetaByBuyerIDsRequest, _ ...grpc.CallOption) (*buyerpb.BatchGetBuyerMetaByBuyerIDsResponse, error) {
			requested := in.BuyerIds[0]
			captured = strconv.FormatInt(requested, 10)
			if requested != liveStintBuyerIDInt {
				// 已關閉 stint：buyer-service 回空 map（deleted_at 已設）
				return &buyerpb.BatchGetBuyerMetaByBuyerIDsResponse{Code: 0, Buyers: map[int64]*buyerpb.BuyerMeta{}}, nil
			}
			return &buyerpb.BatchGetBuyerMetaByBuyerIDsResponse{
				Code:   0,
				Buyers: map[int64]*buyerpb.BuyerMeta{requested: {BuyerId: requested, Nickname: "Alice"}},
			}, nil
		},
	}

	h := &Holder{
		Crm:    grpcclient.NewCrmClientForTest(crmMock),
		Order:  grpcclient.NewOrderClientForTest(orderMock),
		Coupon: grpcclient.NewCouponClientForTest(couponMock),
		Buyer:  grpcclient.NewBuyerClientForTest(buyerMock),
		Store:  grandfatherResolverForTest(),
		// 任何輸入都翻成 currentButWrongBuyerID —— 新語意下不該被呼叫到
		BuyerID: buyerResolverForTest(currentButWrongBuyerID),
	}

	r := gin.New()
	r.Use(fakeSellerAuth())
	r.GET("/customers/:buyerId", h.GetCustomerStats)
	r.PATCH("/customers/:buyerId", h.UpdateCustomerStatus)
	r.GET("/customers/:buyerId/meta", h.GetBuyerMeta)
	r.GET("/customers/:buyerId/orders", h.GetCustomerOrders)
	r.GET("/customers/:buyerId/coupons", h.GetCustomerCoupons)
	r.GET("/customers/:buyerId/tags", h.GetCustomerTags)
	r.POST("/customers/:buyerId/tags", h.AddCustomerTags)
	r.DELETE("/customers/:buyerId/tags/:tagId", h.RemoveCustomerTag)

	// wantStatus 對 /meta 是 404 而非 200：已關閉 stint 的歷史（crm / order /
	// coupon / tags）查得到，但買家個資查不到——見 resolveCustomerBuyerID 註解
	// 的 PII 邊界說明。兩者都在這張表裡，正是為了讓這個差別無法被忽略。
	cases := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
	}{
		{"GetCustomerStats", http.MethodGet, "/customers/" + staleStintBuyerID, "", http.StatusOK},
		{"UpdateCustomerStatus", http.MethodPatch, "/customers/" + staleStintBuyerID, `{"customerType":"vip"}`, http.StatusOK},
		{"GetBuyerMeta_ClosedStint", http.MethodGet, "/customers/" + staleStintBuyerID + "/meta", "", http.StatusNotFound},
		{"GetBuyerMeta_LiveStint", http.MethodGet, "/customers/" + liveStintBuyerID + "/meta", "", http.StatusOK},
		{"GetCustomerOrders", http.MethodGet, "/customers/" + staleStintBuyerID + "/orders", "", http.StatusOK},
		{"GetCustomerCoupons", http.MethodGet, "/customers/" + staleStintBuyerID + "/coupons", "", http.StatusOK},
		{"GetCustomerTags", http.MethodGet, "/customers/" + staleStintBuyerID + "/tags", "", http.StatusOK},
		{"AddCustomerTags", http.MethodPost, "/customers/" + staleStintBuyerID + "/tags", `{"tagNames":["vip"]}`, http.StatusOK},
		{"RemoveCustomerTag", http.MethodDelete, "/customers/" + staleStintBuyerID + "/tags/1", "", http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			captured = ""
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("X-Account-Id", "99")
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assertHTTPStatus(t, w, tc.wantStatus)

			// 不論結果是 200 還是 404，gateway 送給下游的都必須是 path 原值。
			// /meta 的 404 來自 buyer-service 查無此 stint，不是 gateway 把 id
			// 換掉了——這個區別就是本斷言要守住的。
			wantID := staleStintBuyerID
			if tc.name == "GetBuyerMeta_LiveStint" {
				wantID = liveStintBuyerID
			}
			if captured != wantID {
				t.Errorf("%s sent buyer id %q downstream, want the path value verbatim %q — the param IS a buyer_id; resolving it (to %d here) is what made closed-stint crm rows unreachable and 404'd alias buyers",
					tc.name, captured, wantID, currentButWrongBuyerID)
			}
		})
	}

	t.Run("GetBuyerMeta_ResponseEchoesPathAxis", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/customers/"+liveStintBuyerID+"/meta", nil)
		req.Header.Set("X-Account-Id", "99")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assertHTTPStatus(t, w, http.StatusOK)
		want := `"buyerId":"` + liveStintBuyerID + `"`
		if got := w.Body.String(); !strings.Contains(got, want) {
			t.Errorf("response must echo the path buyer_id under %s (path and response on the SAME axis); got %s", want, got)
		}
	})
}
