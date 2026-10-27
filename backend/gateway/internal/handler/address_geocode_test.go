package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	sellerpb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

const fakeAccountID = "123456"

func setupAddressGeocodeRouter(h *Holder) *gin.Engine {
	r := gin.New()
	address := r.Group("/api/v1/app/address")
	address.Use(fakeJWTAuth())
	{
		address.GET("/suggestions", h.AddressSuggestions)
		address.GET("/validate", h.AddressValidate)
	}
	return r
}

func TestAddressSuggestions_Unauthorized(t *testing.T) {
	h := &Holder{}
	r := setupAddressGeocodeRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/app/address/suggestions?county=台北市&district=中正區&query=中華路", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusUnauthorized)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 401)
}

func TestAddressSuggestions_MissingParams(t *testing.T) {
	cases := []struct {
		name string
		url  string
	}{
		{"缺 county", "/api/v1/app/address/suggestions?district=中正區&query=中華路"},
		{"缺 district", "/api/v1/app/address/suggestions?county=台北市&query=中華路"},
		{"缺 query", "/api/v1/app/address/suggestions?county=台北市&district=中正區"},
	}

	h := &Holder{}
	r := setupAddressGeocodeRouter(h)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", tc.url, nil)
			req.Header.Set("X-Account-Id", fakeAccountID)
			req.Header.Set("X-Login-Role", "1")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assertHTTPStatus(t, w, http.StatusBadRequest)
			body := parseJSON(t, w.Body)
			assertCode(t, body, 400)
		})
	}
}

func TestAddressSuggestions_NoActiveRole(t *testing.T) {
	h := &Holder{}
	r := setupAddressGeocodeRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/app/address/suggestions?county=台北市&district=中正區&query=中華路", nil)
	req.Header.Set("X-Account-Id", fakeAccountID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 400)
}

func TestAddressSuggestions_SellerRole_Success(t *testing.T) {
	mock := &mockSellerServiceClient{
		SuggestAddressFn: func(_ context.Context, in *sellerpb.SuggestAddressRequest, _ ...grpc.CallOption) (*sellerpb.SuggestAddressResponse, error) {
			return &sellerpb.SuggestAddressResponse{Code: 0, Data: []*sellerpb.AddressSuggestionData{
				{FullAddress: "台北市中正區中華路一段1號", County: "台北市", District: "中正區", Detail: "中華路一段1號"},
			}}, nil
		},
	}
	h := &Holder{Seller: grpcclient.NewSellerClientForTest(mock)}
	r := setupAddressGeocodeRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/app/address/suggestions?county=台北市&district=中正區&query=中華路", nil)
	req.Header.Set("X-Account-Id", fakeAccountID)
	req.Header.Set("X-Login-Role", "2")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)
	data, ok := body["data"].([]interface{})
	if !ok || len(data) != 1 {
		t.Fatalf("期望 1 筆結果，got: %v", body["data"])
	}
}

func TestAddressSuggestions_LimitApplied(t *testing.T) {
	items := make([]*sellerpb.AddressSuggestionData, 10)
	for i := range items {
		items[i] = &sellerpb.AddressSuggestionData{FullAddress: "台北市中正區中華路1號", County: "台北市", District: "中正區", Detail: "中華路1號"}
	}
	mock := &mockSellerServiceClient{
		SuggestAddressFn: func(_ context.Context, _ *sellerpb.SuggestAddressRequest, _ ...grpc.CallOption) (*sellerpb.SuggestAddressResponse, error) {
			return &sellerpb.SuggestAddressResponse{Code: 0, Data: items}, nil
		},
	}
	h := &Holder{Seller: grpcclient.NewSellerClientForTest(mock)}
	r := setupAddressGeocodeRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/app/address/suggestions?county=台北市&district=中正區&query=中華路&limit=3", nil)
	req.Header.Set("X-Account-Id", fakeAccountID)
	req.Header.Set("X-Login-Role", "2")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	data := body["data"].([]interface{})
	if len(data) != 3 {
		t.Errorf("limit=3 期望 3 筆，got %d", len(data))
	}
}

// TestAddressSuggestions_GrpcError 用一個「非 gRPC status」的裸 error 模擬 client-side
// 交易失敗（mock 直接回傳 plain error，不經過真實 gRPC 傳輸）。`response.GrpcErr`
// 對 `status.FromError` 回 ok=false 的情況一律映射 503（見 gateway/internal/response/response.go
// GrpcErr：`if !ok { ServiceUnavailable(c, service); return }`）——這裡不是 502，這個 gateway
// 的 GrpcErr helper整個檔案沒有任何 502 case，跟 GCIS 的 business_registration.go 走同一條錯誤映射。
func TestAddressSuggestions_GrpcError(t *testing.T) {
	mock := &mockSellerServiceClient{
		SuggestAddressFn: func(_ context.Context, _ *sellerpb.SuggestAddressRequest, _ ...grpc.CallOption) (*sellerpb.SuggestAddressResponse, error) {
			return nil, errors.New("places API timeout")
		},
	}
	h := &Holder{Seller: grpcclient.NewSellerClientForTest(mock)}
	r := setupAddressGeocodeRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/app/address/suggestions?county=台北市&district=中正區&query=中華路", nil)
	req.Header.Set("X-Account-Id", fakeAccountID)
	req.Header.Set("X-Login-Role", "2")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusServiceUnavailable)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 503)
}

func TestAddressValidate_Unauthorized(t *testing.T) {
	h := &Holder{}
	r := setupAddressGeocodeRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/app/address/validate?address=臺北市中正區中華路一段1號", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusUnauthorized)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 401)
}

func TestAddressValidate_MissingAddress(t *testing.T) {
	h := &Holder{}
	r := setupAddressGeocodeRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/app/address/validate", nil)
	req.Header.Set("X-Account-Id", fakeAccountID)
	req.Header.Set("X-Login-Role", "1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 400)
}

func TestAddressValidate_NoMatch(t *testing.T) {
	mock := &mockSellerServiceClient{
		ValidateAddressFn: func(_ context.Context, _ *sellerpb.ValidateAddressRequest, _ ...grpc.CallOption) (*sellerpb.ValidateAddressResponse, error) {
			return &sellerpb.ValidateAddressResponse{Code: 0, Data: &sellerpb.AddressValidationData{Valid: false}}, nil
		},
	}
	h := &Holder{Seller: grpcclient.NewSellerClientForTest(mock)}
	r := setupAddressGeocodeRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/app/address/validate?address=不存在的地址", nil)
	req.Header.Set("X-Account-Id", fakeAccountID)
	req.Header.Set("X-Login-Role", "2")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)
	data := body["data"].(map[string]interface{})
	if data["valid"] != false {
		t.Errorf("期望 valid=false")
	}
}

func TestAddressValidate_SellerRole_Success(t *testing.T) {
	mock := &mockSellerServiceClient{
		ValidateAddressFn: func(_ context.Context, in *sellerpb.ValidateAddressRequest, _ ...grpc.CallOption) (*sellerpb.ValidateAddressResponse, error) {
			return &sellerpb.ValidateAddressResponse{Code: 0, Data: &sellerpb.AddressValidationData{
				FullAddress: "台北市中正區中華路一段1號", County: "台北市", District: "中正區",
				Detail: "中華路一段1號", Lat: "25.0452", Lng: "121.5107", Valid: true,
			}}, nil
		},
	}
	h := &Holder{Seller: grpcclient.NewSellerClientForTest(mock)}
	r := setupAddressGeocodeRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/app/address/validate?address=台北市中正區中華路一段1號", nil)
	req.Header.Set("X-Account-Id", fakeAccountID)
	req.Header.Set("X-Login-Role", "2")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)
	data := body["data"].(map[string]interface{})
	if data["valid"] != true {
		t.Errorf("期望 valid=true")
	}
	if data["detail"] != "中華路一段1號" {
		t.Errorf("detail 期望 '中華路一段1號'，got %v", data["detail"])
	}
}

func TestAddressValidate_NoActiveRole(t *testing.T) {
	h := &Holder{}
	r := setupAddressGeocodeRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/app/address/validate?address=台北市中正區中華路一段1號", nil)
	req.Header.Set("X-Account-Id", fakeAccountID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 400)
}

func TestAddressValidate_GrpcError(t *testing.T) {
	mock := &mockSellerServiceClient{
		ValidateAddressFn: func(_ context.Context, _ *sellerpb.ValidateAddressRequest, _ ...grpc.CallOption) (*sellerpb.ValidateAddressResponse, error) {
			return nil, errors.New("places API timeout")
		},
	}
	h := &Holder{Seller: grpcclient.NewSellerClientForTest(mock)}
	r := setupAddressGeocodeRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/app/address/validate?address=台北市中正區中華路一段1號", nil)
	req.Header.Set("X-Account-Id", fakeAccountID)
	req.Header.Set("X-Login-Role", "2")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusServiceUnavailable)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 503)
}

// 買家端的地址查詢已隨 TGOS 移除（計畫決定 9a）：帶買家角色必須被拒，不得轉發到任何服務。
func TestAddressGeocode_BuyerRoleRejected(t *testing.T) {
	h := &Holder{}
	r := setupAddressGeocodeRouter(h)
	for _, path := range []string{
		"/api/v1/app/address/suggestions?county=台北市&district=中正區&query=中華路",
		"/api/v1/app/address/validate?address=台北市中正區中華路一段1號",
	} {
		req, _ := http.NewRequest("GET", path, nil)
		req.Header.Set("X-Account-Id", fakeAccountID)
		req.Header.Set("X-Login-Role", "1")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assertHTTPStatus(t, w, http.StatusBadRequest)
		assertCode(t, parseJSON(t, w.Body), 400)
	}
}
