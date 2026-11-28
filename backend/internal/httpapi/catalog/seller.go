package catalog

import (
	"context"
	"fmt"
	"log/slog"
	"uuid"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/catalog/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/catalog/v1alpha1/catalogv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
)

// App 實作 SellerCatalogAppService：賣家管理自己店家的商品。店家由登入身分推導（seller.ActingStore），請求裡沒有 store_id；
// 商品不是這家店的與不存在一樣回 not_found（模組以 storeID 限定每一次讀寫）。
type App struct {
	m      SellerModule
	stores ActingStores
	images rpc.Images
	log    *slog.Logger
	table  map[string]declaration
}

var _ catalogv1alpha1connect.SellerCatalogAppServiceHandler = (*App)(nil)

// NewApp 建立賣家的 handler。能力宣告表（declarations）與 service 的方法對不上時回錯誤，不能啟動。
func NewApp(m SellerModule, stores ActingStores, images rpc.Images, log *slog.Logger) (*App, error) {
	return newApp(declarations, m, stores, images, log)
}

func newApp(table map[string]declaration, m SellerModule, stores ActingStores, images rpc.Images, log *slog.Logger) (*App, error) {
	switch {
	case log == nil:
		return nil, fmt.Errorf("httpapi/catalog：需要 logger")
	case m == nil:
		return nil, fmt.Errorf("httpapi/catalog：需要 catalog 模組")
	case stores == nil:
		return nil, fmt.Errorf("httpapi/catalog：需要賣家的店家來源")
	}
	if err := checkDeclarations(table, serviceDescriptor()); err != nil {
		return nil, err
	}
	return &App{m: m, stores: stores, images: images, log: log, table: table}, nil
}

type caller struct {
	storeID   int64
	accountID int64
}

func (h *App) acting(ctx context.Context, procedure string) (caller, error) {
	d, ok := h.table[procedure]
	if !ok || d.capability == "" {
		return caller{}, fmt.Errorf("httpapi/catalog：%s 沒有宣告賣家能力", procedure)
	}
	p, err := rpc.RequireAccount(ctx)
	if err != nil {
		return caller{}, err
	}
	st, err := h.stores.ActingStore(ctx, p.ID, d.capability)
	if err != nil {
		return caller{}, err
	}
	return caller{storeID: st.StoreID, accountID: p.ID}, nil
}

func sellerFilter(in *pb.MyProductFilter) (catalog.SellerFilter, error) {
	query := normalizeQuery(in.GetQuery())
	if err := tooLong("filter.query", query, maxMyQueryRunes); err != nil {
		return catalog.SellerFilter{}, err
	}
	return catalog.SellerFilter{
		Status:      rpc.EnumFromProto[catalog.ProductStatus](in.GetStatus()),
		StockStatus: rpc.EnumFromProto[catalog.StockStatus](in.GetStockStatus()),
		Query:       query,
	}, nil
}

func (h *App) ListMyProducts(ctx context.Context, req *connect.Request[pb.ListMyProductsRequest]) (*connect.Response[pb.ListMyProductsResponse], error) {
	msg := req.Msg
	c, err := h.acting(ctx, catalogv1alpha1connect.SellerCatalogAppServiceListMyProductsProcedure)
	if err != nil {
		return nil, err
	}
	if err := checkPageSize(msg.GetPageSize()); err != nil {
		return nil, err
	}
	f, err := sellerFilter(msg.GetFilter())
	if err != nil {
		return nil, err
	}
	scope := rpc.FilterScope(catalogv1alpha1connect.SellerCatalogAppServiceListMyProductsProcedure, rpc.FormatID(c.storeID), string(f.Status), string(f.StockStatus), f.Query)
	after, err := decodeCursor(scope, msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	page, err := h.m.ListSellerProducts(ctx, c.storeID, f, after, msg.GetPageSize())
	if err != nil {
		return nil, failure(err, sellerListPaths)
	}
	products, err := sellerSummaries(h.images, page.Items)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.ListMyProductsResponse{Products: products, NextPageToken: nextToken(scope, page.Next)}), nil
}

func (h *App) GetMyProduct(ctx context.Context, req *connect.Request[pb.GetMyProductRequest]) (*connect.Response[pb.GetMyProductResponse], error) {
	c, err := h.acting(ctx, catalogv1alpha1connect.SellerCatalogAppServiceGetMyProductProcedure)
	if err != nil {
		return nil, err
	}
	id, err := rpc.ParseUUID("product_id", req.Msg.GetProductId())
	if err != nil {
		return nil, err
	}
	p, err := h.m.Product(ctx, c.storeID, id)
	if err != nil {
		return nil, failure(err, noPaths)
	}
	out, err := sellerProduct(h.images, p)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.GetMyProductResponse{Product: out}), nil
}

func (h *App) CountMyProducts(ctx context.Context, req *connect.Request[pb.CountMyProductsRequest]) (*connect.Response[pb.CountMyProductsResponse], error) {
	c, err := h.acting(ctx, catalogv1alpha1connect.SellerCatalogAppServiceCountMyProductsProcedure)
	if err != nil {
		return nil, err
	}
	f, err := sellerFilter(req.Msg.GetFilter())
	if err != nil {
		return nil, err
	}
	n, err := h.m.CountSellerProducts(ctx, c.storeID, f)
	if err != nil {
		return nil, failure(err, sellerListPaths)
	}
	return connect.NewResponse(&pb.CountMyProductsResponse{Count: n, DefaultLowStockThreshold: catalog.DefaultLowStockThreshold}), nil
}

func (h *App) CreateProduct(ctx context.Context, req *connect.Request[pb.CreateProductRequest]) (*connect.Response[pb.CreateProductResponse], error) {
	msg := req.Msg
	c, err := h.acting(ctx, catalogv1alpha1connect.SellerCatalogAppServiceCreateProductProcedure)
	if err != nil {
		return nil, err
	}
	if msg.GetProduct() == nil {
		return nil, rpc.RequiredField("product", "product 必填")
	}
	rid, err := requestID(msg.GetRequestId())
	if err != nil {
		return nil, err
	}
	in, err := toInput(msg.GetProduct())
	if err != nil {
		return nil, err
	}
	p, err := h.m.CreateProduct(ctx, c.storeID, media.Uploader{ID: c.accountID}, rid, in, nil)
	if err != nil {
		return nil, failure(err, createPaths)
	}
	out, err := sellerProduct(h.images, p)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.CreateProductResponse{Product: out}), nil
}

func (h *App) UpdateProduct(ctx context.Context, req *connect.Request[pb.UpdateProductRequest]) (*connect.Response[pb.UpdateProductResponse], error) {
	msg := req.Msg
	c, err := h.acting(ctx, catalogv1alpha1connect.SellerCatalogAppServiceUpdateProductProcedure)
	if err != nil {
		return nil, err
	}
	id, err := rpc.ParseUUID("product_id", msg.GetProductId())
	if err != nil {
		return nil, err
	}
	version, err := rpc.RequiredVersionField(msg.GetEtag(), "etag")
	if err != nil {
		return nil, err
	}
	if msg.GetProduct() == nil {
		return nil, rpc.RequiredField("product", "product 必填")
	}
	in, err := toInput(msg.GetProduct())
	if err != nil {
		return nil, err
	}
	p, err := h.m.UpdateProduct(ctx, c.storeID, media.Uploader{ID: c.accountID}, id, version, in, nil)
	if err != nil {
		return nil, failure(err, updatePaths)
	}
	out, err := sellerProduct(h.images, p)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.UpdateProductResponse{Product: out}), nil
}

func (h *App) SetProductStatus(ctx context.Context, req *connect.Request[pb.SetProductStatusRequest]) (*connect.Response[pb.SetProductStatusResponse], error) {
	msg := req.Msg
	c, err := h.acting(ctx, catalogv1alpha1connect.SellerCatalogAppServiceSetProductStatusProcedure)
	if err != nil {
		return nil, err
	}
	id, err := rpc.ParseUUID("product_id", msg.GetProductId())
	if err != nil {
		return nil, err
	}
	version, err := rpc.RequiredVersionField(msg.GetEtag(), "etag")
	if err != nil {
		return nil, err
	}
	to := rpc.EnumFromProto[catalog.ProductStatus](msg.GetStatus())
	if to == "" {
		return nil, fieldError("status", "status 必填")
	}
	p, err := h.m.SetProductStatus(ctx, c.storeID, id, version, to, nil)
	if err != nil {
		return nil, failure(err, statusPaths)
	}
	out, err := sellerProduct(h.images, p)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.SetProductStatusResponse{Product: out}), nil
}

func (h *App) DeleteProduct(ctx context.Context, req *connect.Request[pb.DeleteProductRequest]) (*connect.Response[pb.DeleteProductResponse], error) {
	msg := req.Msg
	c, err := h.acting(ctx, catalogv1alpha1connect.SellerCatalogAppServiceDeleteProductProcedure)
	if err != nil {
		return nil, err
	}
	id, err := rpc.ParseUUID("product_id", msg.GetProductId())
	if err != nil {
		return nil, err
	}
	version, err := rpc.VersionFromEtag(msg.GetEtag())
	if err != nil {
		return nil, err
	}
	if err := h.m.DeleteProduct(ctx, c.storeID, id, version); err != nil {
		return nil, failure(err, deletePaths)
	}
	return connect.NewResponse(&pb.DeleteProductResponse{}), nil
}

func (h *App) AdjustStock(ctx context.Context, req *connect.Request[pb.AdjustStockRequest]) (*connect.Response[pb.AdjustStockResponse], error) {
	msg := req.Msg
	c, err := h.acting(ctx, catalogv1alpha1connect.SellerCatalogAppServiceAdjustStockProcedure)
	if err != nil {
		return nil, err
	}
	sku, err := rpc.ParseUUID("sku_id", msg.GetSkuId())
	if err != nil {
		return nil, err
	}
	rid, err := requestID(msg.GetRequestId())
	if err != nil {
		return nil, err
	}
	if msg.GetDelta() == 0 {
		return nil, fieldError("delta", "調整的數量不得為 0")
	}
	if err := h.m.AdjustStock(ctx, c.storeID, rid.String(), sku, msg.GetDelta()); err != nil {
		return nil, failure(err, adjustPaths)
	}
	out, err := h.stockResult(ctx, c, sku)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.AdjustStockResponse{Sku: out}), nil
}

func (h *App) SetStock(ctx context.Context, req *connect.Request[pb.SetStockRequest]) (*connect.Response[pb.SetStockResponse], error) {
	msg := req.Msg
	c, err := h.acting(ctx, catalogv1alpha1connect.SellerCatalogAppServiceSetStockProcedure)
	if err != nil {
		return nil, err
	}
	sku, err := rpc.ParseUUID("sku_id", msg.GetSkuId())
	if err != nil {
		return nil, err
	}
	rid, err := requestID(msg.GetRequestId())
	if err != nil {
		return nil, err
	}
	if msg.GetQuantity() < 0 {
		return nil, fieldError("quantity", "庫存不得為負數")
	}
	if msg.GetExpectedQuantity() < 0 {
		return nil, fieldError("expected_quantity", "預期的庫存不得為負數")
	}
	if err := h.m.SetStock(ctx, c.storeID, rid.String(), sku, msg.GetQuantity(), msg.GetExpectedQuantity()); err != nil {
		return nil, failure(err, setPaths)
	}
	out, err := h.stockResult(ctx, c, sku)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.SetStockResponse{Sku: out}), nil
}

func (h *App) stockResult(ctx context.Context, c caller, sku uuid.UUID) (*pb.Sku, error) {
	st, err := h.m.OwnSku(ctx, c.storeID, sku)
	if err != nil {
		return nil, failure(err, noPaths)
	}
	return sellerSku(st), nil
}
