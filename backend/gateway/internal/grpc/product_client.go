package grpc

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	"github.com/Will413028/hawkr/go-pkg/identity"
	pb "github.com/Will413028/hawkr/go-pkg/proto/product"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// ProductClient is the gRPC client for product service
type ProductClient struct {
	conn   *grpc.ClientConn
	client pb.ProductServiceClient
}

// NewProductClient creates a new product gRPC client
func NewProductClient() (*ProductClient, error) {
	host := getEnvOrDefault("PRODUCT_SERVICE_HOST", "127.0.0.1")
	port := getEnvOrDefault("PRODUCT_SERVICE_GRPC_PORT", "9913")
	target := fmt.Sprintf("%s:%s", host, port)

	conn, err := grpcclient.Dial(target, DefaultDialOpts()...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to product service at %s: %w", target, err)
	}

	zap.S().Infof("Connected to product gRPC service at %s", target)

	return &ProductClient{
		conn:   conn,
		client: pb.NewProductServiceClient(conn),
	}, nil
}

// Close closes the gRPC connection
func (c *ProductClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// ========== 公開商品查詢 ==========

// GetProductList 取得商品列表
func (c *ProductClient) GetProductList(ctx context.Context, categoryID string, keyword string, page, pageSize string, sort, status string, ownerID int64) (*pb.GetProductListResponse, error) {
	return c.GetProductListWithHashtags(ctx, categoryID, keyword, page, pageSize, sort, status, ownerID, nil, false)
}

// GetProductListWithHashtags 取得商品列表（含 hashtag 精確篩選；02 §3.9 / §5.3）。
func (c *ProductClient) GetProductListWithHashtags(ctx context.Context, categoryID, keyword, page, pageSize, sort, status string, ownerID int64, hashtags []string, matchAll bool) (*pb.GetProductListResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req := &pb.GetProductListRequest{
		CategoryId:       toCategoryIDList(categoryID),
		Keyword:          keyword,
		Page:             page,
		PageSize:         pageSize,
		Sort:             sort,
		Status:           status,
		OwnerStoreId:     ownerID,
		Hashtags:         hashtags,
		HashtagMatchMode: hashtagMatchMode(matchAll),
	}

	return c.client.GetProductList(ctx, req)
}

// toCategoryIDList 將單一 categoryID 字串轉為 proto 多值 repeated string。
// 規則（與 02 §5.3.1 對齊）：空字串 / "0" → nil；支援 CSV；trim + 去重 + 過濾 "0" 與空。
// 上限驗證留在 product-service 入口（INVALID_ARGUMENT）。
func toCategoryIDList(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" || s == "0" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]bool, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || p == "0" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// GetPublicProductList 公開 API 取得商品列表（accountId=0 → Product Service 強制 ACTIVE + 填充商家資訊）
func (c *ProductClient) GetPublicProductList(ctx context.Context, categoryID, keyword, page, pageSize, sort string) (*pb.GetProductListResponse, error) {
	return c.GetPublicProductListWithHashtags(ctx, categoryID, keyword, page, pageSize, sort, nil, false)
}

// GetPublicProductListWithHashtags 公開 API 加 hashtag 篩選。
func (c *ProductClient) GetPublicProductListWithHashtags(ctx context.Context, categoryID, keyword, page, pageSize, sort string, hashtags []string, matchAll bool) (*pb.GetProductListResponse, error) {
	return c.GetPublicProductListFiltered(ctx, PublicProductListFilters{
		CategoryIDs: toCategoryIDList(categoryID),
		Keyword:     keyword,
		Page:        page,
		PageSize:    pageSize,
		Sort:        sort,
		Hashtags:    hashtags,
		MatchAll:    matchAll,
	})
}

// PublicProductListFilters 公開列表全部 server-side filter（02 §5.3.4）。
// 任何新增 filter 都集中加在這裡，避免函數簽名爆炸。
type PublicProductListFilters struct {
	CategoryIDs         []string // 分類 ID 多值聯集（02 §5.3.1）；nil/空 = 不篩選分類
	Keyword             string
	Page                string
	PageSize            string
	Sort                string
	Hashtags            []string
	MatchAll            bool
	DeliveryTemp        []string // ShippingTemperature.value 列表
	CertificationTypes  []string // 商品認證篩選（CertificationType value，如 3701/3702），OR 命中
	PriceMin            string   // 新台幣元數字字串，空字串=不篩選
	PriceMax            string   // 新台幣元數字字串
	StoreIDs            []string // 多賣家篩選（數字字串）
	SellerCityCode      string   // 商家註冊所在地 ISO 3166-2:TW
	FollowedProductOnly bool     // 僅回登入者收藏的商品；需 ViewerBuyerID > 0
	FollowedSellerOnly  bool     // 僅回登入者追蹤賣家的所有商品；需 ViewerBuyerID > 0；與 StoreIDs 並存時取交集
	// ViewerBuyerID 與 proto AccountId 語意分離：
	//   proto AccountId = 擁有者篩選（seller 場景）— 公開 API 一律 0
	//   ViewerBuyerID = 觀看者的 buyer_id，由 gateway 解析（resolveBuyerID）後傳入 — 僅供 followed_* filter 使用
	ViewerBuyerID int64
}

// GetPublicProductListFiltered 公開 API 取得商品列表（完整 filter 支援）。
func (c *ProductClient) GetPublicProductListFiltered(ctx context.Context, f PublicProductListFilters) (*pb.GetProductListResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req := &pb.GetProductListRequest{
		CategoryId:       f.CategoryIDs,
		Keyword:          f.Keyword,
		Page:             f.Page,
		PageSize:         f.PageSize,
		Sort:             f.Sort,
		Hashtags:         f.Hashtags,
		HashtagMatchMode: hashtagMatchMode(f.MatchAll),
		// 公開 API：owner_store_id 一律 0（不套用 owner filter）
		OwnerStoreId:        0,
		ViewerBuyerId:       f.ViewerBuyerID,
		DeliveryTempFilter:  f.DeliveryTemp,
		CertificationTypes:  f.CertificationTypes,
		PriceMinFilter:      f.PriceMin,
		PriceMaxFilter:      f.PriceMax,
		StoreIds:            f.StoreIDs,
		SellerCityCode:      f.SellerCityCode,
		FollowedProductOnly: f.FollowedProductOnly,
		FollowedSellerOnly:  f.FollowedSellerOnly,
	}

	return c.client.GetProductList(ctx, req)
}

// GetPublicOriginsSummary 公開商品產地聚合（02 §5.13）
func (c *ProductClient) GetPublicOriginsSummary(ctx context.Context, categoryID string, limit int32) (*pb.GetPublicOriginsSummaryResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return c.client.GetPublicOriginsSummary(ctx, &pb.GetPublicOriginsSummaryRequest{
		CategoryId: categoryID,
		Limit:      limit,
	})
}

func (c *ProductClient) GetPublicSellerCitiesSummary(ctx context.Context) (*pb.GetPublicSellerCitiesSummaryResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return c.client.GetPublicSellerCitiesSummary(ctx, &pb.GetPublicSellerCitiesSummaryRequest{})
}

// SearchHashtags hashtag auto-complete（02 §5.12）。
func (c *ProductClient) SearchHashtags(ctx context.Context, prefix string, mode pb.HashtagSearchMode, limit int32) (*pb.SearchHashtagsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return c.client.SearchHashtags(ctx, &pb.SearchHashtagsRequest{
		Prefix: prefix,
		Mode:   mode,
		Limit:  limit,
	})
}

// GetPopularHashtags 區間熱門 hashtag 查詢（33-商品 Hashtag 模組詳細設計書 §5.2）。
func (c *ProductClient) GetPopularHashtags(ctx context.Context, startFrom, endTo string, limit int32) (*pb.GetPopularHashtagsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return c.client.GetPopularHashtags(ctx, &pb.GetPopularHashtagsRequest{
		StartFrom: startFrom,
		EndTo:     endTo,
		Limit:     limit,
	})
}

func hashtagMatchMode(matchAll bool) pb.HashtagMatchMode {
	if matchAll {
		return pb.HashtagMatchMode_HASHTAG_MATCH_ALL
	}
	return pb.HashtagMatchMode_HASHTAG_MATCH_ANY
}

// GetPublicSellerProductList 公開 API 取得指定賣家的商品列表（accountId=0 → 強制 ACTIVE + 填充商家資訊）
func (c *ProductClient) GetPublicSellerProductList(ctx context.Context, storeID int64, categoryID, keyword, page, pageSize string) (*pb.GetProductListResponse, error) {
	return c.GetPublicSellerProductListWithHashtags(ctx, storeID, categoryID, keyword, page, pageSize, nil, false)
}

// GetPublicSellerProductListWithHashtags 公開賣家列表（含 hashtag 精確篩選）
func (c *ProductClient) GetPublicSellerProductListWithHashtags(ctx context.Context, storeID int64, categoryID, keyword, page, pageSize string, hashtags []string, matchAll bool) (*pb.GetProductListResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req := &pb.GetProductListRequest{
		StoreId:          strconv.FormatInt(storeID, 10),
		CategoryId:       toCategoryIDList(categoryID),
		Keyword:          keyword,
		Page:             page,
		PageSize:         pageSize,
		Hashtags:         hashtags,
		HashtagMatchMode: hashtagMatchMode(matchAll),
	}

	return c.client.GetProductList(ctx, req)
}

// GetProduct 取得商品詳情
// 對應新 proto: GetProduct
// productID 為 UUID 字串
// viewerAccountID 供 product-service 判斷 selfOwned，共用於 owner 與 viewer 兩種
// 呼叫端——owner 端（賣家管理頁）已解析為 identity.StoreID，呼叫時自行
// int64(storeID) 轉型；viewer 端（公開商品頁 / SKU 公開列表）語意上
// 可能是買家、賣家、或匿名（0），未必經過 store resolver，因此本參數維持裸
// int64，不 narrow 成 identity.StoreID（同 GetProductListWithHashtags 的委派點
// 原則：型別只在能保證語意成立處收斂，不能保證時維持裸 int64 由呼叫端自行轉型）。
func (c *ProductClient) GetProduct(ctx context.Context, productID string, viewerAccountID int64) (*pb.GetProductResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req := &pb.GetProductRequest{
		ProductId: productID,
		StoreId:   viewerAccountID,
	}

	return c.client.GetProduct(ctx, req)
}

// GetPublicProduct 公開商品詳情（依 publication contract，支援 Redis 快取）
// 供買家或匿名使用者瀏覽，不需要 userId
func (c *ProductClient) GetPublicProduct(ctx context.Context, productID string) (*pb.GetProductResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req := &pb.GetPublicProductRequest{
		ProductId: productID,
	}

	return c.client.GetPublicProduct(ctx, req)
}

// GetPublicSellerProducts 公開 API 取得指定賣家的商品列表（僅 ACTIVE）
func (c *ProductClient) GetPublicSellerProducts(ctx context.Context, ownerID int64, categoryID, keyword, page, pageSize string) (*pb.GetProductListResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req := &pb.GetProductListRequest{
		OwnerStoreId: ownerID,
		CategoryId:   toCategoryIDList(categoryID),
		Keyword:      keyword,
		Page:         page,
		PageSize:     pageSize,
	}

	return c.client.GetProductList(ctx, req)
}

// ========== 賣家商品管理 ==========

// ListSellerProducts 取得商家的商品列表
// 透過 GetProductList 並傳入 ownerID 實現
// 無呼叫端（dead code）；ownerID 未必已解析（同 SearchAdminProducts 家族），
// 故不經過已收斂型別的 ListSellerProductsWithHashtags，直接呼叫裸 int64 的
// GetProductListWithHashtags，避免無依據地把未保證解析的值 narrow 成
// identity.StoreID。
func (c *ProductClient) ListSellerProducts(ctx context.Context, ownerID int64, page, pageSize string, status string, categoryID string, keyword string) (*pb.GetProductListResponse, error) {
	return c.GetProductListWithHashtags(ctx, categoryID, keyword, page, pageSize, "", status, ownerID, nil, false)
}

// ListSellerProductsWithHashtags 賣家商品列表（支援 hashtag 精確篩選；02 §3.9 / §5.3）
func (c *ProductClient) ListSellerProductsWithHashtags(ctx context.Context, storeID identity.StoreID, page, pageSize, status, categoryID, keyword string, hashtags []string, matchAll bool) (*pb.GetProductListResponse, error) {
	return c.GetProductListWithHashtags(ctx, categoryID, keyword, page, pageSize, "", status, int64(storeID), hashtags, matchAll)
}

// CreateProduct 建立商品
func (c *ProductClient) CreateProduct(ctx context.Context, req *pb.CreateProductRequest) (*pb.CreateProductResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	return c.client.CreateProduct(ctx, req)
}

// UpdateProduct 更新商品
func (c *ProductClient) UpdateProduct(ctx context.Context, req *pb.UpdateProductRequest) (*pb.UpdateProductResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	return c.client.UpdateProduct(ctx, req)
}

// DeleteProduct 軟刪除商品（賣家用）
func (c *ProductClient) DeleteProduct(ctx context.Context, productID string, storeID identity.StoreID) (*pb.DeleteProductResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.DeleteProduct(ctx, &pb.DeleteProductRequest{
		ProductId: productID,
		StoreId:   int64(storeID),
	})
}

// ========== 管理員商品操作 ==========

// GetProductForAdmin 管理員取得商品詳情（不檢查擁有者，不限狀態）
func (c *ProductClient) GetProductForAdmin(ctx context.Context, productID string) (*pb.GetProductResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req := &pb.GetProductForAdminRequest{
		ProductId: productID,
	}

	return c.client.GetProductForAdmin(ctx, req)
}

// SearchAdminProducts 管理員查詢商品列表（不強制 status，支援所有篩選條件）
func (c *ProductClient) SearchAdminProducts(ctx context.Context, page, pageSize string, status string, ownerID int64, categoryID string, keyword string) (*pb.GetProductListResponse, error) {
	return c.SearchAdminProductsWithHashtags(ctx, page, pageSize, status, ownerID, categoryID, keyword, nil, false)
}

// SearchAdminProductsWithHashtags 管理員查詢商品列表（支援 hashtag 精確篩選；02 §4.1.3）
func (c *ProductClient) SearchAdminProductsWithHashtags(ctx context.Context, page, pageSize, status string, ownerID int64, categoryID, keyword string, hashtags []string, matchAll bool) (*pb.GetProductListResponse, error) {
	return c.GetProductListWithHashtags(ctx, categoryID, keyword, page, pageSize, "", status, ownerID, hashtags, matchAll)
}

// UpdateProductStatus 管理員更新商品狀態
func (c *ProductClient) UpdateProductStatus(ctx context.Context, productID, status string) (*pb.UpdateProductResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req := &pb.UpdateProductStatusRequest{
		ProductId: productID,
		Status:    status,
	}

	return c.client.UpdateProductStatus(ctx, req)
}

// UpdateSellerProductStatus 賣家更新商品狀態（上架/下架）
func (c *ProductClient) UpdateSellerProductStatus(ctx context.Context, productID string, storeID identity.StoreID, status string) (*pb.UpdateProductResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req := &pb.UpdateSellerProductStatusRequest{
		ProductId: productID,
		StoreId:   int64(storeID),
		Status:    status,
	}

	return c.client.UpdateSellerProductStatus(ctx, req)
}

// ========== 運費計算 ==========

// CalculateShippingOptions 計算可用配送選項與運費（依溫層分組）
func (c *ProductClient) CalculateShippingOptions(ctx context.Context, req *pb.CalculateShippingOptionsRequest) (*pb.CalculateShippingOptionsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.CalculateShippingOptions(ctx, req)
}

// ========== 相關商品 ==========

// GetRelatedProducts 取得相關商品（同分類、排除自身、僅 ACTIVE、上限 8 筆）
func (c *ProductClient) GetRelatedProducts(ctx context.Context, productID string) (*pb.GetRelatedProductsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.GetRelatedProducts(ctx, &pb.GetRelatedProductsRequest{
		ProductId: productID,
	})
}

// ========== 認證驗證 ==========

// VerifyCertification 驗證認證編號是否存在於政府開放資料平台
func (c *ProductClient) VerifyCertification(ctx context.Context, certNo, certType string) (*pb.VerifyCertificationResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	return c.client.VerifyCertification(ctx, &pb.VerifyCertificationRequest{
		CertNo: certNo,
		Type:   certType,
	})
}

// ========== 兼容舊 API 的包裝方法 ==========

// ListProducts 兼容舊 API（內部調用 GetProductList）
func (c *ProductClient) ListProducts(ctx context.Context, categoryID string, keyword string, page, pageSize string) (*pb.GetProductListResponse, error) {
	return c.GetProductList(ctx, categoryID, keyword, page, pageSize, "", "", 0)
}

// GetProductDetail 兼容舊 API（內部調用 GetProduct）
// 注意：參數從 int64 改為 string
// userId 為 0 表示匿名查詢
func (c *ProductClient) GetProductDetail(ctx context.Context, productID string) (*pb.GetProductResponse, error) {
	return c.GetProduct(ctx, productID, 0)
}

// ========== 維運管理 ==========

// PurgeProductCache 清除商品快取
// scope: all（全部）、detail（商品詳情）、list（列表）
// productID: 當 scope=detail 時必填
// ownerID: 當 scope=list 時可選，指定則只清除該擁有者的列表
func (c *ProductClient) PurgeProductCache(ctx context.Context, scope, productID string, ownerID int64) (*pb.PurgeProductCacheResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req := &pb.PurgeProductCacheRequest{
		Scope:     scope,
		ProductId: productID,
		StoreId:   ownerID,
	}

	return c.client.PurgeProductCache(ctx, req)
}

// ListProductsByCategoryForAdmin lists products under a category for admin (含分頁)
func (c *ProductClient) ListProductsByCategoryForAdmin(ctx context.Context, categoryID, page, pageSize string) (*pb.ListProductsByCategoryForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req := &pb.ListProductsByCategoryForAdminRequest{
		CategoryId: categoryID,
		Page:       page,
		PageSize:   pageSize,
	}

	return c.client.ListProductsByCategoryForAdmin(ctx, req)
}

// BatchChangeProductCategoryForAdmin batch changes category for products (限 L3→L3)
func (c *ProductClient) BatchChangeProductCategoryForAdmin(
	ctx context.Context,
	productIDs []string,
	sourceCategoryID, targetCategoryID string,
	updatedBy int64,
) (*pb.BatchChangeProductCategoryForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req := &pb.BatchChangeProductCategoryForAdminRequest{
		ProductIds:       productIDs,
		SourceCategoryId: sourceCategoryID,
		TargetCategoryId: targetCategoryID,
		UpdatedBy:        updatedBy,
	}

	return c.client.BatchChangeProductCategoryForAdmin(ctx, req)
}

// ========== 商品認證申請與審核（19 號設計書 §9 / §10.1）==========
//
// 十支 wrapper 沿用本檔既有形狀：呼叫端只給業務參數，deadline 在這裡設。
// 逾時值分兩級——一般 CRUD 用 30s（與本檔其他寫入一致），Reverify 用 90s
// （19 §9.3：它會同步打農業部 API，繞過同步用斷路器，上游本身就慢）。

// SubmitProductCertification 賣家送出一筆認證申請。
func (c *ProductClient) SubmitProductCertification(ctx context.Context, req *pb.SubmitProductCertificationRequest) (*pb.ProductCertificationApplicationResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.SubmitProductCertification(ctx, req)
}

// ListProductCertificationApplications 賣家查自己商品的認證申請列表。
func (c *ProductClient) ListProductCertificationApplications(ctx context.Context, req *pb.ListProductCertificationApplicationsRequest) (*pb.ListProductCertificationApplicationsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.ListProductCertificationApplications(ctx, req)
}

// WithdrawProductCertification 賣家撤回一筆待審申請。
func (c *ProductClient) WithdrawProductCertification(ctx context.Context, req *pb.WithdrawProductCertificationRequest) (*pb.ProductCertificationApplicationResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.WithdrawProductCertification(ctx, req)
}

// ListProductCertificationApplicationsForAdmin ADM 審核佇列。
func (c *ProductClient) ListProductCertificationApplicationsForAdmin(ctx context.Context, req *pb.ListProductCertificationApplicationsForAdminRequest) (*pb.ListProductCertificationApplicationsForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.ListProductCertificationApplicationsForAdmin(ctx, req)
}

// GetProductCertificationApplicationForAdmin ADM 申請詳情（含 gov payload / 內部註記）。
func (c *ProductClient) GetProductCertificationApplicationForAdmin(ctx context.Context, req *pb.GetProductCertificationApplicationForAdminRequest) (*pb.ProductCertificationApplicationAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.GetProductCertificationApplicationForAdmin(ctx, req)
}

// ListProductCertificationAuditLogsForAdmin ADM 審核事件流水（append-only）。
func (c *ProductClient) ListProductCertificationAuditLogsForAdmin(ctx context.Context, req *pb.ListProductCertificationAuditLogsForAdminRequest) (*pb.ListProductCertificationAuditLogsForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.ListProductCertificationAuditLogsForAdmin(ctx, req)
}

// ApproveProductCertificationForAdmin ADM 人工核准。
func (c *ProductClient) ApproveProductCertificationForAdmin(ctx context.Context, req *pb.ReviewProductCertificationForAdminRequest) (*pb.ProductCertificationApplicationAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.ApproveProductCertificationForAdmin(ctx, req)
}

// RejectProductCertificationForAdmin ADM 駁回（reasonCode 必填，缺 → 3028）。
func (c *ProductClient) RejectProductCertificationForAdmin(ctx context.Context, req *pb.ReviewProductCertificationForAdminRequest) (*pb.ProductCertificationApplicationAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.RejectProductCertificationForAdmin(ctx, req)
}

// RevokeProductCertificationForAdmin ADM 撤銷已核准的認證（reasonCode 必填）。
func (c *ProductClient) RevokeProductCertificationForAdmin(ctx context.Context, req *pb.ReviewProductCertificationForAdminRequest) (*pb.ProductCertificationApplicationAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.client.RevokeProductCertificationForAdmin(ctx, req)
}

// ReverifyProductCertificationForAdmin ADM 重新查驗（只改 gov_verify_*，不改 status）。
//
// 90 秒 deadline 是 19 §9.3 的明文要求：這一支同步觸發背景查驗邏輯、繞過同步用斷路器，
// 上游是農業部開放資料平台。用本檔預設的 30s 會讓正常但慢的查驗被 Gateway 先砍掉，
// 而 product-service 那邊的外部呼叫仍在跑（配額照樣消耗），對操作者則顯示逾時。
func (c *ProductClient) ReverifyProductCertificationForAdmin(ctx context.Context, req *pb.ReverifyProductCertificationForAdminRequest) (*pb.ProductCertificationApplicationAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	return c.client.ReverifyProductCertificationForAdmin(ctx, req)
}
