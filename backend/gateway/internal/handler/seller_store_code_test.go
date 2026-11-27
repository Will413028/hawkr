package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	couponpb "github.com/Will413028/hawkr/go-pkg/proto/coupon"
	orderpb "github.com/Will413028/hawkr/go-pkg/proto/order"
	"google.golang.org/grpc"
)

const (
	testStoreID   = "2636840881"
	testStoreCode = "2649605557"
)

func TestOrderList_CarriesSellerStoreCode(t *testing.T) {
	mock := &mockOrderServiceClient{
		GetMyOrdersFn: func(ctx context.Context, in *orderpb.GetMyOrdersRequest, opts ...grpc.CallOption) (*orderpb.GetMyOrdersResponse, error) {
			return &orderpb.GetMyOrdersResponse{
				Code: 0,
				Orders: []*orderpb.OrderSummaryView{{
					OrderNo:    "ORD20260801001",
					StoreId:    testStoreID,
					StoreCode:  testStoreCode,
					SellerName: "Seller A",
				}},
			}, nil
		},
	}
	h := &Holder{
		Order:   grpcclient.NewOrderClientForTest(mock),
		BuyerID: buyerGrandfatherResolverForTest(),
	}

	r := setupOrderRouter(h)
	req, _ := http.NewRequest("GET", "/api/v1/app/orders?page=1", nil)
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)

	order := firstRecord(t, body)
	assertJSONString(t, order, "storeCode", testStoreCode)
	assertJSONString(t, order, "storeId", testStoreID)
}

func TestOrderDetail_CarriesSellerStoreCode(t *testing.T) {
	mock := &mockOrderServiceClient{
		GetOrderFn: func(ctx context.Context, in *orderpb.GetOrderRequest, opts ...grpc.CallOption) (*orderpb.GetOrderResponse, error) {
			return &orderpb.GetOrderResponse{
				Code: 0,
				Data: &orderpb.OrderDetailView{
					OrderNo:    "ORD20260801001",
					StoreId:    testStoreID,
					StoreCode:  testStoreCode,
					SellerName: "Seller A",
				},
			}, nil
		},
	}
	h := &Holder{
		Order:   grpcclient.NewOrderClientForTest(mock),
		BuyerID: buyerGrandfatherResolverForTest(),
	}

	r := setupOrderRouter(h)
	req, _ := http.NewRequest("GET", "/api/v1/app/orders/ORD20260801001", nil)
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)

	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatal("data is not an object")
	}
	assertJSONString(t, data, "storeCode", testStoreCode)
}

func TestMyCoupons_CarrySellerStoreCode(t *testing.T) {
	couponMock := &couponMockCouponSvcClient{
		ListMyCouponsFn: func(ctx context.Context, in *couponpb.ListMyCouponsRequest, opts ...grpc.CallOption) (*couponpb.ListMyCouponsResponse, error) {
			return &couponpb.ListMyCouponsResponse{
				Code: 0,
				Records: []*couponpb.TemplateInfo{{
					TemplateId:        "1",
					Title:             "滿千折百",
					ApplicableStoreId: testStoreID,
					StoreCode:         testStoreCode,
					SellerName:        "Seller A",
				}},
				UsedRecords: []*couponpb.InstanceInfo{{
					InstanceId:        "9",
					TemplateId:        "1",
					ApplicableStoreId: testStoreID,
					StoreCode:         testStoreCode,
					SellerName:        "Seller A",
				}},
			}, nil
		},
	}
	h := &Holder{
		Coupon:  grpcclient.NewCouponClientForTest(couponMock),
		BuyerID: buyerGrandfatherResolverForTest(),
	}

	r := setupCouponRouter(h)
	req, _ := http.NewRequest("GET", "/api/v1/app/coupons", nil)
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)

	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatal("data is not an object")
	}
	for _, key := range []string{"records", "usedRecords"} {
		rows, ok := data[key].([]interface{})
		if !ok || len(rows) == 0 {
			t.Fatalf("%s is not a non-empty array", key)
		}
		row, ok := rows[0].(map[string]interface{})
		if !ok {
			t.Fatalf("%s[0] is not an object", key)
		}
		assertJSONString(t, row, "storeCode", testStoreCode)
	}
}

// 結帳預覽的 orderGroups 是買家「下單前最後一眼」看到的賣家區塊，和其他四條一樣
// 需要能連回賣家頁。CheckoutService 為了 sellerName 與 payuni_mer_id 本來就呼叫了
// getSellerMeta，store_code 同一份回應裡就有。
func TestCheckoutPreview_OrderGroupsCarrySellerStoreCode(t *testing.T) {
	mock := &mockOrderServiceClient{
		CheckoutPreviewFn: func(ctx context.Context, in *orderpb.CheckoutPreviewRequest, opts ...grpc.CallOption) (*orderpb.CheckoutPreviewResponse, error) {
			return &orderpb.CheckoutPreviewResponse{
				Code: 0,
				Data: &orderpb.CheckoutPreviewData{
					CheckoutId: "chk-uuid-store-code",
					OrderGroups: []*orderpb.OrderGroupPreview{{
						GroupIndex: "0",
						StoreId:    testStoreID,
						StoreCode:  testStoreCode,
						SellerName: "Seller A",
					}},
				},
			}, nil
		},
	}
	h := &Holder{Order: grpcclient.NewOrderClientForTest(mock), BuyerID: buyerGrandfatherResolverForTest()}

	r := setupCheckoutRouter(h)
	body, _ := json.Marshal(map[string]interface{}{
		"shippingMethod": "TAQBIN",
		"deliveryTemp":   "CHILLED",
		"items": []map[string]string{
			{"skuId": "SKU001", "quantity": "1", "productId": "PROD001"},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/checkout/preview", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 0)

	data, ok := respBody["data"].(map[string]interface{})
	if !ok {
		t.Fatal("data is not an object")
	}
	groups, ok := data["orderGroups"].([]interface{})
	if !ok || len(groups) == 0 {
		t.Fatal("orderGroups is not a non-empty array")
	}
	group, ok := groups[0].(map[string]interface{})
	if !ok {
		t.Fatal("orderGroups[0] is not an object")
	}
	assertJSONString(t, group, "storeCode", testStoreCode)
	assertJSONString(t, group, "storeId", testStoreID)
}

func firstRecord(t *testing.T, body map[string]interface{}) map[string]interface{} {
	t.Helper()
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatal("data is not an object")
	}
	rows, ok := data["records"].([]interface{})
	if !ok || len(rows) == 0 {
		t.Fatal("data.records is not a non-empty array")
	}
	row, ok := rows[0].(map[string]interface{})
	if !ok {
		t.Fatal("data.records[0] is not an object")
	}
	return row
}

func assertJSONString(t *testing.T, obj map[string]interface{}, key, want string) {
	t.Helper()
	got, ok := obj[key].(string)
	if !ok {
		t.Fatalf("%s missing or not a string (got %#v)", key, obj[key])
	}
	if got != want {
		t.Errorf("%s = %q, want %q", key, got, want)
	}
}
