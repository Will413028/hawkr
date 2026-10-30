package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	"github.com/Will413028/hawkr/gateway/internal/response"
	sellerpb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

func setupStorefrontResolveProbe(h *Holder) *gin.Engine {
	r := gin.New()
	r.GET("/probe/:id", func(c *gin.Context) {
		id, ok := h.resolveStorefrontSellerID(c, c.Param("id"))
		if !ok {
			return
		}
		response.OK(c, gin.H{"storeId": strconv.FormatInt(id, 10)})
	})
	return r
}

func TestResolveStorefrontSellerID_PassesThroughLegacyStoreID(t *testing.T) {
	sellerMock := &mockSellerServiceClient{
		GetSellerFn: func(ctx context.Context, in *sellerpb.GetSellerRequest, opts ...grpc.CallOption) (*sellerpb.SellerResponse, error) {
			return &sellerpb.SellerResponse{
				Code: 0,
				Data: &sellerpb.SellerInfo{StoreId: "1234567890123456789", StoreCode: "2600123456"},
			}, nil
		},
	}
	h := &Holder{Seller: grpcclient.NewSellerClientForTest(sellerMock)}
	router := setupStorefrontResolveProbe(h)

	req := httptest.NewRequest(http.MethodGet, "/probe/1234567890123456789", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	data := body["data"].(map[string]interface{})
	if data["storeId"] != "1234567890123456789" {
		t.Errorf("storeId = %v, want 1234567890123456789", data["storeId"])
	}
}

func TestResolveStorefrontSellerID_InvalidID_BadRequest(t *testing.T) {
	h := &Holder{}
	router := setupStorefrontResolveProbe(h)

	req := httptest.NewRequest(http.MethodGet, "/probe/not-a-number", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
}

func TestResolveStorefrontSellerID_NotFound(t *testing.T) {
	sellerMock := &mockSellerServiceClient{
		GetSellerFn: func(ctx context.Context, in *sellerpb.GetSellerRequest, opts ...grpc.CallOption) (*sellerpb.SellerResponse, error) {
			return nil, grpcstatus.Error(grpccodes.NotFound, "seller not found")
		},
	}
	h := &Holder{Seller: grpcclient.NewSellerClientForTest(sellerMock)}
	router := setupStorefrontResolveProbe(h)

	req := httptest.NewRequest(http.MethodGet, "/probe/9999999999", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusNotFound)
}
