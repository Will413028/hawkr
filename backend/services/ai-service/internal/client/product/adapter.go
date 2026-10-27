package product

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Will413028/hawkr/ai-service/internal/config"
	"github.com/Will413028/hawkr/ai-service/internal/usecase"
	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	"github.com/Will413028/hawkr/go-pkg/identity"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	productpb "github.com/Will413028/hawkr/go-pkg/proto/product"
	"google.golang.org/grpc"
)

type productClient struct {
	client productpb.ProductServiceClient
}

func NewProductClient(cfg *config.Config) (usecase.ProductClient, error) {
	target := fmt.Sprintf("%s:%s", cfg.ProductServiceHost, cfg.ProductServicePort)
	conn, err := grpcclient.Dial(target,
		grpc.WithStatsHandler(interceptor.OTelClientStatsHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to product-service at %s: %w", target, err)
	}
	return &productClient{client: productpb.NewProductServiceClient(conn)}, nil
}

func (c *productClient) GetProductList(ctx context.Context, page, pageSize int) ([]*usecase.ProductItem, int64, error) {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()

	resp, err := c.client.GetProductList(ctx, &productpb.GetProductListRequest{
		// CategoryId 留空 = 全部分類（02 §5.3.1 多值聯集）
		Page:     strconv.Itoa(page),
		PageSize: strconv.Itoa(pageSize),
		Status:   "4004", // listed only
	})
	if err != nil {
		return nil, 0, err
	}

	total, _ := strconv.ParseInt(resp.GetTotal(), 10, 64)
	items := make([]*usecase.ProductItem, 0, len(resp.GetProducts()))
	for _, p := range resp.GetProducts() {
		priceMin, _ := strconv.ParseInt(p.GetPriceMin(), 10, 64)
		priceMax, _ := strconv.ParseInt(p.GetPriceMax(), 10, 64)
		rating, _ := strconv.ParseFloat(p.GetRating(), 64)
		itemStoreID, _ := strconv.ParseInt(p.GetStoreId(), 10, 64)
		createdAt, _ := strconv.ParseInt(p.GetCreatedAt(), 10, 64)

		items = append(items, &usecase.ProductItem{
			ID:        parseID(p.GetId()),
			Name:      p.GetName(),
			ImageURL:  p.GetImageKey(),
			PriceMin:  priceMin,
			PriceMax:  priceMax,
			Rating:    rating,
			StoreID:   identity.StoreID(itemStoreID),
			CreatedAt: createdAt,
		})
	}
	return items, total, nil
}

// ListSellerProducts lists products for the seller. storeID is already the
// caller's CURRENT store_id (resolved by gateway before this call —
// account_id 命名債正名), not a raw login account_id needing resolution here.
func (c *productClient) ListSellerProducts(ctx context.Context, storeID identity.StoreID, page, pageSize int) ([]*usecase.ProductItem, error) {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()

	resp, err := c.client.GetProductList(ctx, &productpb.GetProductListRequest{
		// CategoryId 留空 = 全部分類（02 §5.3.1 多值聯集）
		Page:     strconv.Itoa(page),
		PageSize: strconv.Itoa(pageSize),
		Status:   "4004",
		StoreId:  strconv.FormatInt(int64(storeID), 10),
	})
	if err != nil {
		return nil, err
	}

	items := make([]*usecase.ProductItem, 0, len(resp.GetProducts()))
	for _, p := range resp.GetProducts() {
		priceMin, _ := strconv.ParseInt(p.GetPriceMin(), 10, 64)
		priceMax, _ := strconv.ParseInt(p.GetPriceMax(), 10, 64)
		itemStoreID, _ := strconv.ParseInt(p.GetStoreId(), 10, 64)

		items = append(items, &usecase.ProductItem{
			ID:       parseID(p.GetId()),
			Name:     p.GetName(),
			ImageURL: p.GetImageKey(),
			PriceMin: priceMin,
			PriceMax: priceMax,
			StoreID:  identity.StoreID(itemStoreID),
		})
	}
	return items, nil
}

func (c *productClient) GetStockBatch(ctx context.Context, skuIDs []string) (map[string]int64, error) {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()

	resp, err := c.client.GetStock(ctx, &productpb.GetStockRequest{
		SkuIds: skuIDs,
	})
	if err != nil {
		return nil, err
	}

	result := make(map[string]int64, len(resp.GetItems()))
	for _, item := range resp.GetItems() {
		qty, _ := strconv.ParseInt(item.GetStockQty(), 10, 64)
		result[item.GetSkuId()] = qty
	}
	return result, nil
}

// SearchProducts is used by procurement PLATFORM lane candidate search: that caller
// only sets Keyword (see doc 55 §6.2 — per-unit budget is NOT mapped to
// PriceMaxFilter; the denominators differ). PriceMaxNTD stays supported here with
// its own SKU-price meaning for other callers. No pagination, no pagination (max 50 results per request is sufficient to cover candidate card display
// volume, doc 55 §9.3 candidate search <10s budget doesn't require pagination like the listing page does).
func (c *productClient) SearchProducts(ctx context.Context, filter usecase.ProductSearchFilter) ([]*usecase.ProductItem, error) {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()

	req := &productpb.GetProductListRequest{
		Keyword:  filter.Keyword,
		Page:     "1",
		PageSize: "50",
		Status:   "4004", // listed only
	}
	if filter.PriceMaxNTD != nil {
		req.PriceMaxFilter = strconv.Itoa(*filter.PriceMaxNTD)
	}

	resp, err := c.client.GetProductList(ctx, req)
	if err != nil {
		return nil, err
	}

	items := make([]*usecase.ProductItem, 0, len(resp.GetProducts()))
	for _, p := range resp.GetProducts() {
		priceMin, _ := strconv.ParseInt(p.GetPriceMin(), 10, 64)
		priceMax, _ := strconv.ParseInt(p.GetPriceMax(), 10, 64)
		itemStoreID, _ := strconv.ParseInt(p.GetStoreId(), 10, 64)

		items = append(items, &usecase.ProductItem{
			ID:       parseID(p.GetId()),
			Name:     p.GetName(),
			ImageURL: p.GetImageKey(),
			PriceMin: priceMin,
			PriceMax: priceMax,
			StoreID:  identity.StoreID(itemStoreID),
		})
	}
	return items, nil
}

// GetProductStoreID 回傳商品當下的擁有者 store_id。
//
// 用 GetProduct（含權限檢查的賣家端 RPC）而不是 GetPublicProduct，有兩個理由，
// 兩個都關係到這個值的用途——它是賣家能不能確認一張報價單邀請的唯一判準：
//
//  1. **新鮮度**：product-service 的 getPublicProduct 走 Redis Cache-Aside
//     （TTL 1 小時，key 含 onboarding 指紋），getProduct 則明確「不使用快取，因為
//     需要檢查權限」（ProductService.java 的行內註解）。授權判斷不該讀快取。
//  2. **下架商品**：getPublicProduct 對非公開／已移除商品直接丟 NotFound，賣家把
//     商品暫時下架之後就再也確認不了自己收到的邀請；getProduct 讓擁有者看得到自己
//     的 DRAFT/PENDING_REVIEW/INACTIVE 商品，非擁有者則一律 NOT_FOUND（
//     ProductAccessDeniedException 也被刻意映射成 NOT_FOUND，不洩漏存在性）。
//
// 回傳值取 ProductDetail.seller.store_id：product.proto 明講 created_by 是
// 「audit only，非擁有權 — 擁有權見 SellerInfo.store_id」，而 product-service 是用
// entity.getStoreId()（商品自己那一列的 store_id）組出這個欄位，與呼叫端傳進去的
// store_id 無關，所以拿它跟 callerStoreID 比對是一個真正的擁有權檢查，不是把輸入
// 抄回輸出。SellerInfo 其餘欄位（店名/評分/logo）來自 seller-service，那邊掛掉時
// 會降級成「只有 store_id」的 DTO——store_id 本身仍然填得出來。
func (c *productClient) GetProductStoreID(ctx context.Context, productID int64, callerStoreID identity.StoreID) (identity.StoreID, error) {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()

	resp, err := c.client.GetProduct(ctx, &productpb.GetProductRequest{
		ProductId: strconv.FormatInt(productID, 10),
		StoreId:   int64(callerStoreID),
	})
	if err != nil {
		return 0, err
	}
	// 業務錯誤走 in-band code：查無商品與無權存取都是 NOT_FOUND，這裡不分辨——
	// 兩者對呼叫端的結論相同（不能確認）。
	if resp.GetCode() != 0 {
		return 0, fmt.Errorf("GetProduct(product_id=%d) returned code %d (%s): %w",
			productID, resp.GetCode(), resp.GetMessage(), usecase.ErrProductOwnerUnavailable)
	}

	raw := resp.GetData().GetSeller().GetStoreId()
	ownerStoreID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || ownerStoreID == 0 {
		// 空字串／0／不是數字：一律當成「問不到擁有者」，絕不退化成 0 再讓呼叫端
		// 拿 0 去比對（0 != 任何合法 store_id，比對會失敗，但那是碰巧安全，不是設計）。
		return 0, fmt.Errorf("GetProduct(product_id=%d) returned seller.store_id=%q: %w",
			productID, raw, usecase.ErrProductOwnerUnavailable)
	}
	return identity.StoreID(ownerStoreID), nil
}

func parseID(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}
