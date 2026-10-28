package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	productpb "github.com/Will413028/hawkr/go-pkg/proto/product"
	shippingpb "github.com/Will413028/hawkr/go-pkg/proto/shipping"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

type ambientShippingClient struct {
	shippingpb.ShippingServiceClient
	onboardCvs  func(*shippingpb.OnboardCvsShippingRequest) (*shippingpb.OnboardShippingResponse, error)
	onboardings []*shippingpb.ShippingOnboardingView
	methods     []*shippingpb.SellerShippingMethodView
}

func (m *ambientShippingClient) OnboardCvsShipping(_ context.Context, in *shippingpb.OnboardCvsShippingRequest, _ ...grpc.CallOption) (*shippingpb.OnboardShippingResponse, error) {
	return m.onboardCvs(in)
}

func (m *ambientShippingClient) ListShippingOnboardings(_ context.Context, _ *shippingpb.ListShippingOnboardingsRequest, _ ...grpc.CallOption) (*shippingpb.ListShippingOnboardingsResponse, error) {
	return &shippingpb.ListShippingOnboardingsResponse{Code: 0, Onboardings: m.onboardings}, nil
}

func (m *ambientShippingClient) ListSellerShippingMethods(_ context.Context, _ *shippingpb.ListSellerShippingMethodsRequest, _ ...grpc.CallOption) (*shippingpb.ListSellerShippingMethodsResponse, error) {
	return &shippingpb.ListSellerShippingMethodsResponse{Code: 0, Methods: m.methods}, nil
}

func ambientRouter(h *Holder) *gin.Engine {
	r := gin.New()
	app := r.Group("/api/v1/app")
	app.Use(fakeSellerAuth())
	app.POST("/shipping_quotes", h.CalculateShippingOptions)
	app.POST("/shipping_onboardings/cvs", h.OnboardCvsShipping)
	app.GET("/shipping_onboardings", h.ListShippingOnboardings)
	app.GET("/shipping_methods", h.ListSellerShippingMethods)
	app.POST("/products", h.CreateSellerProduct)
	app.PUT("/products/:id", h.UpdateSellerProduct)
	return r
}

func doAmbient(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestShippingQuotes_ReturnsOnlyAmbientOptionsWithoutTemperature(t *testing.T) {
	mock := &mockProductServiceClient{
		CalculateShippingOptionsFn: func(context.Context, *productpb.CalculateShippingOptionsRequest, ...grpc.CallOption) (*productpb.CalculateShippingOptionsResponse, error) {
			return &productpb.CalculateShippingOptionsResponse{Code: 0, Data: &productpb.ShippingCalculationData{
				AvailableTemperatures: []*productpb.AvailableTemperatureProto{{Temperature: "3201"}, {Temperature: "3202"}, {Temperature: "3203"}},
				ShippingOptions: []*productpb.ShippingOptionProto{
					{ShippingMethod: "3101", Temperature: "3201", Fee: "100"},
					{ShippingMethod: "3101", Temperature: "3202", Fee: "150"},
					{ShippingMethod: "3101", Temperature: "3203", Fee: "200"},
				},
			}}, nil
		},
	}
	h := &Holder{Product: grpcclient.NewProductClientForTest(mock), Store: storeResolverForTest(7001)}

	w := doAmbient(ambientRouter(h), "POST", "/api/v1/app/shipping_quotes", shippingQuoteBody)

	assertHTTPStatus(t, w, http.StatusOK)
	var env struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	var opts []map[string]any
	if err := json.Unmarshal(env.Data["shippingOptions"], &opts); err != nil {
		t.Fatal(err)
	}
	if len(opts) != 1 || opts[0]["fee"] != "100" {
		t.Fatalf("shippingOptions = %v, want only the ambient row (fee 100)", opts)
	}
	if _, ok := opts[0]["temperature"]; ok {
		t.Errorf("option must not carry temperature: %v", opts[0])
	}
	if _, ok := env.Data["availableTemperatures"]; ok {
		t.Errorf("availableTemperatures must not be returned")
	}
}

func TestOnboardCvs_WithoutGoodsType_SendsAmbient(t *testing.T) {
	var got *shippingpb.OnboardCvsShippingRequest
	shipping := &ambientShippingClient{onboardCvs: func(in *shippingpb.OnboardCvsShippingRequest) (*shippingpb.OnboardShippingResponse, error) {
		got = in
		return &shippingpb.OnboardShippingResponse{Code: 0}, nil
	}}
	h := &Holder{Shipping: grpcclient.NewShippingClientForTest(shipping), Store: storeResolverForTest(7001)}

	body := `{"returnStoreCode":"123456","senderName":"王","senderPhone":"0912345678","goodsType":"3203"}`
	w := doAmbient(ambientRouter(h), "POST", "/api/v1/app/shipping_onboardings/cvs", body)

	assertHTTPStatus(t, w, http.StatusOK)
	if got == nil || got.GoodsType != "3201" {
		t.Fatalf("GoodsType = %+v, want 3201 regardless of client value", got)
	}
}

func TestShippingOnboardings_ListsAmbientAndUntypedRowsWithoutGoodsType(t *testing.T) {
	shipping := &ambientShippingClient{onboardings: []*shippingpb.ShippingOnboardingView{
		{OnboardingId: "1", ShippingType: "CVS_C2C", GoodsType: "3201"},
		{OnboardingId: "2", ShippingType: "CVS_C2C", GoodsType: "3203"},
		{OnboardingId: "3", ShippingType: "HOME_DELIVERY"},
	}}
	h := &Holder{Shipping: grpcclient.NewShippingClientForTest(shipping), Store: storeResolverForTest(7001)}

	w := doAmbient(ambientRouter(h), "GET", "/api/v1/app/shipping_onboardings", "")

	assertHTTPStatus(t, w, http.StatusOK)
	var env struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data) != 2 || env.Data[0]["onboardingId"] != "1" || env.Data[1]["onboardingId"] != "3" {
		t.Fatalf("rows = %v, want onboarding 1 and 3 only", env.Data)
	}
	for _, row := range env.Data {
		if _, ok := row["goodsType"]; ok {
			t.Errorf("row must not carry goodsType: %v", row)
		}
	}
}

func TestSellerShippingMethods_ListsAmbientAndUntypedRowsWithoutGoodsType(t *testing.T) {
	shipping := &ambientShippingClient{methods: []*shippingpb.SellerShippingMethodView{
		{ShippingMethod: "3102", GoodsType: "3201"},
		{ShippingMethod: "3102", GoodsType: "3203"},
		{ShippingMethod: "3101"},
	}}
	h := &Holder{Shipping: grpcclient.NewShippingClientForTest(shipping), Store: storeResolverForTest(7001)}

	w := doAmbient(ambientRouter(h), "GET", "/api/v1/app/shipping_methods", "")

	assertHTTPStatus(t, w, http.StatusOK)
	var env struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data) != 2 {
		t.Fatalf("rows = %v, want 2 (ambient CVS + untyped home delivery)", env.Data)
	}
	for _, row := range env.Data {
		if _, ok := row["goodsType"]; ok {
			t.Errorf("row must not carry goodsType: %v", row)
		}
	}
}

func TestSellerProductCreate_ShippingMethodsCarryAmbientTemperature(t *testing.T) {
	var got *productpb.CreateProductRequest
	mock := &mockProductServiceClient{
		CreateProductFn: func(_ context.Context, in *productpb.CreateProductRequest, _ ...grpc.CallOption) (*productpb.CreateProductResponse, error) {
			got = in
			return &productpb.CreateProductResponse{Code: 0, Data: &productpb.ProductDetail{Id: "p1"}}, nil
		},
	}
	h := &Holder{Product: grpcclient.NewProductClientForTest(mock), Store: storeResolverForTest(7001)}

	body := `{"idempotencyKey":"k1","name":"高麗菜","categoryId":"1","skus":[` +
		`{"skuName":"s1","unit":"3301","price":"100","stockQty":"5","weight":"1","lengthCm":"10","widthCm":"10","heightCm":"10",` +
		`"shippingMethods":[{"type":"3101","temperature":"3202"}]}]}`
	w := doAmbient(ambientRouter(h), "POST", "/api/v1/app/products", body)

	assertHTTPStatus(t, w, http.StatusOK)
	if got == nil || len(got.Skus) != 1 || len(got.Skus[0].ShippingMethods) != 1 {
		t.Fatalf("request = %+v", got)
	}
	if temp := got.Skus[0].ShippingMethods[0].Temperature; temp != "3201" {
		t.Errorf("Temperature = %q, want 3201", temp)
	}
}

func TestSellerProductUpdate_ShippingMethodsCarryAmbientTemperature(t *testing.T) {
	var got *productpb.UpdateProductRequest
	mock := &mockProductServiceClient{
		UpdateProductFn: func(_ context.Context, in *productpb.UpdateProductRequest, _ ...grpc.CallOption) (*productpb.UpdateProductResponse, error) {
			got = in
			return &productpb.UpdateProductResponse{Code: 0, Data: &productpb.ProductDetail{Id: "p1"}}, nil
		},
	}
	h := &Holder{Product: grpcclient.NewProductClientForTest(mock), Store: storeResolverForTest(7001)}

	body := `{"idempotencyKey":"k1","skus":[{"id":"356","weight":"2","lengthCm":"11","widthCm":"11","heightCm":"11",` +
		`"shippingMethods":[{"type":"3101"}]}]}`
	w := doAmbient(ambientRouter(h), "PUT", "/api/v1/app/products/7312345678901234567", body)

	assertHTTPStatus(t, w, http.StatusOK)
	if got == nil || len(got.Skus) != 1 || len(got.Skus[0].ShippingMethods) != 1 {
		t.Fatalf("request = %+v", got)
	}
	if temp := got.Skus[0].ShippingMethods[0].Temperature; temp != "3201" {
		t.Errorf("Temperature = %q, want 3201", temp)
	}
}

func TestImportAndBatchAndDelegated_ShippingMethodsCarryAmbientTemperature(t *testing.T) {
	for _, m := range toPbShippingMethods([]ShippingMethodInputDTO{{Method: "3101"}}) {
		if m.Temperature != "3201" {
			t.Errorf("import update Temperature = %q, want 3201", m.Temperature)
		}
	}
	for _, m := range toPbBatchShippingMethods([]BatchShippingMethodInputDTO{{Type: "3101", FreeThreshold: "0"}}) {
		if m.Temperature != "3201" {
			t.Errorf("batch Temperature = %q, want 3201", m.Temperature)
		}
	}
	for _, m := range toShippingMethodProtosFromInput([]DelegatedShippingMethodInput{{Type: "3101"}}) {
		if m.Temperature != "3201" {
			t.Errorf("delegated input Temperature = %q, want 3201", m.Temperature)
		}
	}
	for _, m := range toShippingMethodProtos([]ShippingMethodResponse{{Type: "3101", Temperature: "3202"}}) {
		if m.Temperature != "3201" {
			t.Errorf("delegated before Temperature = %q, want 3201", m.Temperature)
		}
	}
}
