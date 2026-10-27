package handler

import (
	"context"
	"strconv"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	accountpb "github.com/Will413028/hawkr/go-pkg/proto/account"
	productpb "github.com/Will413028/hawkr/go-pkg/proto/product"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// 代行建立商品的落地狀態碼。
//
// 這一格沒有任何自然的訊號會在它出錯時響：漏填 status 時 product-service 落地
// ACTIVE（ProductEntityConverter.toEntity），HTTP 仍是 200、稽核列仍記
// PRODUCT_CREATE，只有買家那側多出一件沒有人按過上架的商品。填成 DRAFT(4001)
// 同樣安靜，症狀是賣家在自己的商品管理裡找不到代行者說已經建好的那件商品
// （4001 的語意是「僅建立者可查看」）。
//
// 兩個方向都要釘：不是 ACTIVE（capability 繞過），也不是 DRAFT（隱藏狀態）。

type capturingCreateProductStub struct {
	productpb.ProductServiceClient
	last *productpb.CreateProductRequest
}

func (s *capturingCreateProductStub) CreateProduct(ctx context.Context, in *productpb.CreateProductRequest, opts ...grpc.CallOption) (*productpb.CreateProductResponse, error) {
	s.last = in
	return &productpb.CreateProductResponse{Code: 0, Data: &productpb.ProductDetail{
		Id:      "7494024721080516610",
		Name:    in.GetName(),
		Version: "1",
	}}, nil
}

func newCreateStatusRouter(t *testing.T) (*gin.Engine, *capturingCreateProductStub) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	product := &capturingCreateProductStub{}
	h := &Holder{
		Account: grpcclient.NewAccountDelegationClientForTest(&delegationStub{}),
		Product: grpcclient.NewProductClientForTest(product),
	}

	r := gin.New()
	g := r.Group("/api/v1/admin")
	g.Use(func(c *gin.Context) {
		middleware.SetDelegationSession(c, &accountpb.OperationSessionSummary{
			SessionId: "2701445580000002",
			Actor:     &accountpb.DelegationActor{AdminId: "2701338845", DisplayName: "Cora Chen"},
			Target:    &accountpb.DelegationTarget{StoreId: "2685914682"},
			Status:    "active",
		})
		c.Next()
	})
	g.POST("/sellers/:store_id/products", h.CreateDelegatedProduct)
	return r, product
}

func TestDelegatedProductCreate_LandsInactiveNotDraftNorActive(t *testing.T) {
	r, product := newCreateStatusRouter(t)

	postDelegated(t, r, "/api/v1/admin/sellers/2685914682/products", "ik-product-status-0001",
		`{"operationReason":"協助新品上架","name":"嘉義布袋鮮蚵","categoryId":"10001",`+
			`"skus":[{"spec":"600g","price":"350","stockQty":"20","unit":"3301"}]}`)

	if product.last == nil {
		t.Fatal("CreateProduct 沒有被呼叫，下面的斷言會空過")
	}
	got := product.last.GetStatus()

	if want := strconv.Itoa(codemeta.ProductStatusInactive); got != want {
		t.Errorf("CreateProduct.status = %q, want %q（未上架）", got, want)
	}
	if got == strconv.Itoa(codemeta.ProductStatusDraft) {
		t.Error("落地 4001 草稿 ⇒ 語意是「僅建立者可查看」，賣家在自己的商品管理裡看不到代行者剛建好的商品")
	}
	if got == strconv.Itoa(codemeta.ProductStatusActive) || got == "" {
		t.Error("落地 4004 或空值 ⇒ 商品直接對買家可見，繞過 products.publish（空值時由 product-service 預設成 ACTIVE）")
	}
}
