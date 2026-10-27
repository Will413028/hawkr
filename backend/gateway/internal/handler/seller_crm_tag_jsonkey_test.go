package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	crmpb "github.com/Will413028/hawkr/go-pkg/proto/crm"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// tagKeyCrmMock is a local CRM mock covering only the three tag-returning RPCs.
// Defined locally (rather than extending a shared mock) to stay consistent with
// shapeCrmMock — parallel domain PRs run in this same package.
type tagKeyCrmMock struct {
	crmpb.CrmServiceClient
	ListTagsFn        func(ctx context.Context, in *crmpb.ListTagsRequest, opts ...grpc.CallOption) (*crmpb.TagListResponse, error)
	CreateTagFn       func(ctx context.Context, in *crmpb.CreateTagRequest, opts ...grpc.CallOption) (*crmpb.TagResponse, error)
	GetCustomerTagsFn func(ctx context.Context, in *crmpb.GetCustomerTagsRequest, opts ...grpc.CallOption) (*crmpb.TagListResponse, error)
}

func (m *tagKeyCrmMock) ListTags(ctx context.Context, in *crmpb.ListTagsRequest, opts ...grpc.CallOption) (*crmpb.TagListResponse, error) {
	return m.ListTagsFn(ctx, in, opts...)
}

func (m *tagKeyCrmMock) CreateTag(ctx context.Context, in *crmpb.CreateTagRequest, opts ...grpc.CallOption) (*crmpb.TagResponse, error) {
	return m.CreateTagFn(ctx, in, opts...)
}

func (m *tagKeyCrmMock) GetCustomerTags(ctx context.Context, in *crmpb.GetCustomerTagsRequest, opts ...grpc.CallOption) (*crmpb.TagListResponse, error) {
	return m.GetCustomerTagsFn(ctx, in, opts...)
}

func TestSellerTagEndpoints_OutwardJSONKeyIsStoreID(t *testing.T) {
	const (
		tagID   = "7304444444444444444"
		storeID = "7302222222222222222"
	)
	tag := func() *crmpb.Tag {
		return &crmpb.Tag{Id: tagID, StoreId: storeID, Name: "常客"}
	}

	mock := &tagKeyCrmMock{
		ListTagsFn: func(context.Context, *crmpb.ListTagsRequest, ...grpc.CallOption) (*crmpb.TagListResponse, error) {
			return &crmpb.TagListResponse{Code: 0, Message: "success", Data: []*crmpb.Tag{tag()}}, nil
		},
		CreateTagFn: func(context.Context, *crmpb.CreateTagRequest, ...grpc.CallOption) (*crmpb.TagResponse, error) {
			return &crmpb.TagResponse{Code: 0, Message: "success", Data: tag()}, nil
		},
		GetCustomerTagsFn: func(context.Context, *crmpb.GetCustomerTagsRequest, ...grpc.CallOption) (*crmpb.TagListResponse, error) {
			return &crmpb.TagListResponse{Code: 0, Message: "success", Data: []*crmpb.Tag{tag()}}, nil
		},
	}

	h := &Holder{
		Crm:     grpcclient.NewCrmClientForTest(mock),
		Store:   grandfatherResolverForTest(),
		BuyerID: buyerGrandfatherResolverForTest(),
	}

	r := gin.New()
	v1 := r.Group("/api/v1/app/sellers/me")
	v1.Use(fakeSellerAuth())
	v1.GET("/tags", h.ListMerchantTags)
	v1.POST("/tags", h.CreateSellerTag)
	v1.GET("/customers/:buyerId/tags", h.GetCustomerTags)

	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"ListMerchantTags", http.MethodGet, "/api/v1/app/sellers/me/tags", ""},
		{"CreateSellerTag", http.MethodPost, "/api/v1/app/sellers/me/tags", `{"name":"常客"}`},
		{"GetCustomerTags", http.MethodGet, "/api/v1/app/sellers/me/customers/7303333333333333333/tags", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var bodyReader *strings.Reader
			if tc.body == "" {
				bodyReader = strings.NewReader("")
			} else {
				bodyReader = strings.NewReader(tc.body)
			}
			req := httptest.NewRequest(tc.method, tc.path, bodyReader)
			req.Header.Set("X-Account-Id", "99")
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("want 200; got %d body=%s", w.Code, w.Body.String())
			}
			got := w.Body.String()

			if !strings.Contains(got, `"storeId"`) {
				t.Errorf("response must expose %q; got %s", "storeId", got)
			}
			if strings.Contains(got, `"store_id"`) {
				t.Errorf("response leaked the internal proto key %q — the mapping struct is missing or bypassed; got %s", "store_id", got)
			}
			if strings.Contains(got, `"account_id"`) {
				t.Errorf("response still uses the pre-rename key %q; the CRM seller contract is camelCase storeId now; got %s", "account_id", got)
			}
			// the value must survive the mapping unchanged
			if !strings.Contains(got, storeID) {
				t.Errorf("store surrogate value %q missing from response; got %s", storeID, got)
			}
			if !strings.Contains(got, tagID) {
				t.Errorf("tag id %q missing from response; got %s", tagID, got)
			}
		})
	}
}
