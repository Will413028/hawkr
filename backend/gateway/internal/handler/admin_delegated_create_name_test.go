package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	accountpb "github.com/Will413028/hawkr/go-pkg/proto/account"
	productpb "github.com/Will413028/hawkr/go-pkg/proto/product"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// 建立類是唯一沒有 pre-image 的寫入形態，因此也是唯一「稽核列的 resource_name 只能
// 由 Gateway 從請求本身填」的形態。漏填的症狀完全無聲：HTTP 200、稽核列在、
// resource_id 由 confirm 補上——只有稽核主控台的「目標資源」欄恆為「—」，也就是
// 建立這個動作最需要交代的那一格。
//
// 測的是「intent 送了什麼」而非回應：回應在漏填時一切正常。

type createProductStub struct {
	productpb.ProductServiceClient
}

func (s *createProductStub) CreateProduct(ctx context.Context, in *productpb.CreateProductRequest, opts ...grpc.CallOption) (*productpb.CreateProductResponse, error) {
	return &productpb.CreateProductResponse{Code: 0, Data: &productpb.ProductDetail{
		Id:      "7494024721080516608",
		Name:    in.GetName(),
		Version: "1",
	}}, nil
}

func newDelegatedCreateRouter(t *testing.T) (*gin.Engine, *delegationStub) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	audit := &delegationStub{}
	h := &Holder{
		Account: grpcclient.NewAccountDelegationClientForTest(audit),
		Product: grpcclient.NewProductClientForTest(&createProductStub{}),
	}

	r := gin.New()
	g := r.Group("/api/v1/admin")
	g.Use(func(c *gin.Context) {
		middleware.SetDelegationSession(c, &accountpb.OperationSessionSummary{
			SessionId: "2701445580000001",
			Actor:     &accountpb.DelegationActor{AdminId: "2701338845", DisplayName: "Cora Chen"},
			Target:    &accountpb.DelegationTarget{StoreId: "2685914682"},
			Status:    "active",
		})
		c.Next()
	})
	g.POST("/sellers/:store_id/products", h.CreateDelegatedProduct)
	return r, audit
}

func postDelegated(t *testing.T, r *gin.Engine, path, key, body string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", key)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200; body = %s", w.Code, w.Body.String())
	}
}

func TestDelegatedProductCreate_AuditIntentCarriesResourceName(t *testing.T) {
	r, audit := newDelegatedCreateRouter(t)

	postDelegated(t, r, "/api/v1/admin/sellers/2685914682/products", "ik-product-create-0001",
		`{"operationReason":"協助新品上架","name":"嘉義布袋鮮蚵","categoryId":"10001",`+
			`"skus":[{"spec":"600g","price":"350","stockQty":"20","unit":"3301"}]}`)

	if len(audit.intents) != 1 {
		t.Fatalf("RecordAuditIntent 呼叫次數 = %d, want 1", len(audit.intents))
	}
	if got := audit.intents[0].GetResourceName(); got != "嘉義布袋鮮蚵" {
		t.Errorf("intent.resource_name = %q, want 嘉義布袋鮮蚵 —— 空值讓稽核主控台的「目標資源」恆為「—」", got)
	}
}
