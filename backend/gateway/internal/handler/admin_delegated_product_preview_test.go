package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	accountpb "github.com/Will413028/hawkr/go-pkg/proto/account"
	productpb "github.com/Will413028/hawkr/go-pkg/proto/product"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

type previewProductStub struct {
	productpb.ProductServiceClient
	detail *productpb.ProductDetail
}

func (s *previewProductStub) GetProduct(ctx context.Context, in *productpb.GetProductRequest, opts ...grpc.CallOption) (*productpb.GetProductResponse, error) {
	return &productpb.GetProductResponse{Data: s.detail}, nil
}

func delegatedProductDetailJSON(t *testing.T, detail *productpb.ProductDetail) map[string]json.RawMessage {
	t.Helper()
	gin.SetMode(gin.TestMode)

	h := &Holder{Product: grpcclient.NewProductClientForTest(&previewProductStub{detail: detail})}
	r := gin.New()
	g := r.Group("/api/v1/admin")
	g.Use(func(c *gin.Context) {
		middleware.SetDelegationSession(c, &accountpb.OperationSessionSummary{
			SessionId: "2701445580000001",
			Actor:     &accountpb.DelegationActor{AdminId: "2701338845", DisplayName: "Cora Chen"},
			Target:    &accountpb.DelegationTarget{StoreId: scopeTestStoreID},
			Status:    "active",
		})
		c.Next()
	})
	g.GET("/sellers/:store_id/products/:product_id", h.GetDelegatedProduct)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet,
		"/api/v1/admin/sellers/"+scopeTestStoreID+"/products/"+scopeTestProductID, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200; body = %s", w.Code, w.Body.String())
	}

	var envelope struct {
		Code int                        `json:"code"`
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("回應不是合法 envelope（%v）：%s", err, w.Body.String())
	}
	if envelope.Code != 0 {
		t.Fatalf("envelope code = %d, want 0；body = %s", envelope.Code, w.Body.String())
	}
	return envelope.Data
}

func rawKeys(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// 草稿商品是本案最危險的一格：它正是「代行者要預覽、而買家還看不到」的那一類。
func TestDelegatedProductDetail_DraftCarriesBuyerVisibilityFlags(t *testing.T) {
	draft := fullProductDetail("無籽檸檬", "4001", "1")
	draft.PublicationState = "4011"
	draft.Purchasable = false

	data := delegatedProductDetailJSON(t, draft)

	for _, key := range []string{"publicationState", "purchasable"} {
		if _, ok := data[key]; !ok {
			t.Fatalf("回應 key set 少了 %q —— 預覽自繪就沒有素材可判「買家看不看得到、買不買得到」。"+
				"實際 key set = %v", key, rawKeys(data))
		}
	}

	if got := string(data["purchasable"]); got != "false" {
		t.Errorf("草稿商品的 purchasable = %s, want false —— 前端判定寫成 `purchasable !== false`，"+
			"任何非 false（含欄位整個消失）都會把草稿預覽成「可以購買」", got)
	}
	if got := string(data["publicationState"]); got != `"4011"` {
		t.Errorf("publicationState = %s, want \"4011\"（NEVER_PUBLISHED）", got)
	}
}

// 反向鑑別力：少了這支，一個把 purchasable 寫死成 false 的實作也能讓上面那支全綠，
// 而它會把每一件上架中的商品都預覽成不可購買。
func TestDelegatedProductDetail_ActiveProductStaysPurchasable(t *testing.T) {
	live := fullProductDetail("無籽檸檬", "4004", "12")
	live.PublicationState = "4012"
	live.Purchasable = true

	data := delegatedProductDetailJSON(t, live)

	if got := string(data["purchasable"]); got != "true" {
		t.Errorf("上架中商品的 purchasable = %s, want true —— 投影必須原樣帶出上游的判定，"+
			"不得寫死", got)
	}
	if got := string(data["publicationState"]); got != `"4012"` {
		t.Errorf("publicationState = %s, want \"4012\"（PUBLIC）", got)
	}
}
