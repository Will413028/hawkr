package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	orderpb "github.com/Will413028/hawkr/go-pkg/proto/order"
	"google.golang.org/grpc"
)

// 配送方式、超商門市、Uber 報價與免運判定已離開結帳契約；回應不得再出現這些 key。
var removedCheckoutKeys = []string{
	"shippingMethod", "shippingLabel", "shippingFeePayer",
	"freeShippingEligible", "tentativeFreeShippingEligible", "freeShippingThreshold",
	"availableShippingOptions", "cvsStoreCode", "cvsStoreName", "uberQuoteId",
}

func assertNoRemovedKeys(t *testing.T, where string, m map[string]interface{}) {
	t.Helper()
	for _, k := range removedCheckoutKeys {
		if _, present := m[k]; present {
			t.Errorf("%s 不得含已移除的 key %q", where, k)
		}
	}
}

func TestCheckoutPreview_ResponseHasNoShippingSelectionKeys(t *testing.T) {
	mock := &mockOrderServiceClient{
		CheckoutPreviewFn: func(context.Context, *orderpb.CheckoutPreviewRequest, ...grpc.CallOption) (*orderpb.CheckoutPreviewResponse, error) {
			return &orderpb.CheckoutPreviewResponse{Data: &orderpb.CheckoutPreviewData{
				CheckoutId:       "chk-1",
				TotalShippingFee: "0",
				OrderGroups: []*orderpb.OrderGroupPreview{{
					GroupIndex: "0", StoreId: "7", ShippingFee: "0", DeliveryTemp: "3201",
				}},
			}}, nil
		},
	}
	h := &Holder{Order: grpcclient.NewOrderClientForTest(mock), BuyerID: buyerGrandfatherResolverForTest()}
	body, _ := json.Marshal(map[string]interface{}{
		"deliveryTemp": "3201",
		"items":        []map[string]string{{"skuId": "1", "quantity": "1", "productId": "2"}},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/checkout/preview", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	setupCheckoutRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	data := parseJSON(t, w.Body)["data"].(map[string]interface{})
	assertNoRemovedKeys(t, "preview data", data)
	group := data["orderGroups"].([]interface{})[0].(map[string]interface{})
	assertNoRemovedKeys(t, "preview orderGroups[0]", group)
	if group["shippingFee"] != "0" || group["deliveryTemp"] != "3201" {
		t.Errorf("保留欄位 shippingFee／deliveryTemp 應仍在：%v", group)
	}
}

func TestCheckoutValidate_ResponseHasNoShippingOptions(t *testing.T) {
	mock := &mockOrderServiceClient{
		CheckoutValidateFn: func(context.Context, *orderpb.CheckoutValidateRequest, ...grpc.CallOption) (*orderpb.CheckoutValidateResponse, error) {
			return &orderpb.CheckoutValidateResponse{Data: &orderpb.CheckoutValidateData{Valid: true}}, nil
		},
	}
	h := &Holder{Order: grpcclient.NewOrderClientForTest(mock), BuyerID: buyerGrandfatherResolverForTest()}
	body, _ := json.Marshal(map[string]interface{}{
		"items": []map[string]string{{"skuId": "1", "quantity": "1", "productId": "2"}},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/checkout/validate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	setupCheckoutRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	assertNoRemovedKeys(t, "validate data", parseJSON(t, w.Body)["data"].(map[string]interface{}))
}

func TestCheckoutResponseDTOsDeclareNoRemovedKeys(t *testing.T) {
	for name, v := range map[string]interface{}{
		"CheckoutValidateDataResponse": CheckoutValidateDataResponse{},
		"CheckoutPreviewDataResponse":  CheckoutPreviewDataResponse{},
		"OrderGroupPreviewResponse":    OrderGroupPreviewResponse{},
		"CheckoutConfirmDataResponse":  CheckoutConfirmDataResponse{},
	} {
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]interface{}
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		assertNoRemovedKeys(t, name, m)
	}
}
