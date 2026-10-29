package router

import (
	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

// registerProductRoutes 配置商品、類別、評價、橫幅相關路由
// 涵蓋 public / app / admin 三大 Namespace
func registerProductRoutes(public *gin.RouterGroup, app *gin.RouterGroup, admin *gin.RouterGroup, h *handler.Holder) *gin.RouterGroup {
	// ==========================================================
	// Public — 匿名可存取
	// ==========================================================

	// 公開商品 /api/v1/public/products
	pubProducts := public.Group(ProductsPath)
	pubProducts.Use(middleware.OptionalJWTAuth())
	{
		pubProducts.GET("", h.GetProductList)
		pubProducts.GET(ProductsIDPath, h.GetProductDetailByID)
		pubProducts.GET(ProductsBrief, h.GetProductInfoByID)
		pubProducts.GET(ProductSkus, h.ListSellerSkus)         // 列出商品的規格
		pubProducts.GET(ProductsRating, h.GetProductRating)    // 取得商品評分
		pubProducts.GET(ProductsRelated, h.GetRelatedProducts) // 取得相關商品
	}

	// 公開 hashtag auto-complete /api/v1/public/hashtags（02 §5.12）
	// 區間熱門 hashtag /api/v1/public/hashtags/popular（33-商品 Hashtag 模組詳細設計書 §5.2 / §6.2）
	pubHashtags := public.Group(HashtagsPath)
	pubHashtags.Use(middleware.OptionalJWTAuth())
	{
		pubHashtags.GET("", h.SearchHashtags)
		pubHashtags.GET("/popular", h.GetPopularHashtags)
	}

	// 公開商品產地聚合 /api/v1/public/origins/summary（02 §5.13）
	public.GET(OriginsSummaryPath, middleware.OptionalJWTAuth(), h.GetOriginsSummary)
	public.GET(SellerCitiesSummaryPath, middleware.OptionalJWTAuth(), h.GetSellerCitiesSummary)

	// 公開類別 /api/v1/public/categories
	pubCategories := public.Group(CategoriesPath)
	pubCategories.Use(middleware.OptionalJWTAuth())
	{
		pubCategories.GET("", h.GetCategoryList)
		pubCategories.GET(CategoriesTree, h.GetCategoryTree)
		pubCategories.GET(CategoryID, h.GetCategoryByID)
		pubCategories.GET(CategoryPathSeg, h.GetCategoryPath)
		pubCategories.GET(CategoryChildren, h.GetCategoryChildren)
	}

	// ==========================================================
	// App — 需要 JWT 認證
	// ==========================================================

	// 賣家商品管理 /api/v1/app/products
	// GET /app/products?view=selling → 賣家商品列表
	// POST /app/products → 建立商品（SELLER）
	// PUT /app/products/:id → 更新商品（SELLER）
	// 賣家商品管理：整組 CAP_PRODUCT_MANAGE（唯讀查詢也鎖，2026-07-10 裁決）。
	appProducts := app.Group(ProductsPath)
	appProducts.Use(middleware.JWTAuth())
	{
		sellerHandle(appProducts, "GET", "", authz.CapProductManage, h.ListSellerProducts)
		sellerHandle(appProducts, "POST", "", authz.CapProductManage, h.CreateSellerProduct)
		sellerHandle(appProducts, "GET", ProductID, authz.CapProductManage, h.GetSellerProductDetail)
		sellerHandle(appProducts, "PUT", ProductID, authz.CapProductManage, h.UpdateSellerProduct)
		sellerHandle(appProducts, "PUT", ProductStatus, authz.CapProductManage, h.UpdateSellerProductStatus)
		sellerHandle(appProducts, "DELETE", ProductID, authz.CapProductManage, h.DeleteSellerProduct)
		sellerHandle(appProducts, "GET", ProductSkus, authz.CapProductManage, h.GetSellerProductSkus)
		sellerHandle(appProducts, "GET", CertificationVerify, authz.CapProductManage, h.VerifyCertification)
		// 商品認證申請（19 §9.1）。與商品同一組能力：認證是商品的附屬資源。
		sellerHandle(appProducts, "GET", ProductCertificationApplications, authz.CapProductManage, h.ListSellerProductCertificationApplications)
		sellerHandle(appProducts, "POST", ProductCertificationApplications, authz.CapProductManage, h.SubmitSellerProductCertification)
		sellerHandle(appProducts, "POST", ProductCertificationApplicationWithdraw, authz.CapProductManage, h.WithdrawSellerProductCertification)
	}

	// 類別管理 /api/v1/app/categories（賣家新增/查候選分類，隨上架 → CAP_PRODUCT_MANAGE）
	appCategories := app.Group(CategoriesPath)
	appCategories.Use(middleware.JWTAuth())
	{
		sellerHandle(appCategories, "POST", "", authz.CapProductManage, h.CreateCategory)
		sellerHandle(appCategories, "GET", CategoriesTree, authz.CapProductManage, h.GetSellerCategoryTree)
	}

	// 配送報價 /api/v1/app/shipping-quotes（賣家建商品算運費）
	sellerHandleAuthed(app, "POST", ShippingQuotesPath, authz.CapProductManage, h.CalculateShippingOptions)

	// 賣家 hashtag auto-complete /api/v1/app/hashtags（02 §5.12；隨發文/上架）
	appHashtags := app.Group(HashtagsPath)
	appHashtags.Use(middleware.JWTAuth())
	{
		sellerHandle(appHashtags, "GET", "", authz.CapProductManage, h.SearchHashtags)
	}

	// ==========================================================
	// Admin — 管理員專用
	// ==========================================================

	// 商品管理 /api/v1/admin/products
	admin.GET(AdminProductsPath, h.SearchAdminProducts)
	admin.DELETE(AdminProductCache, h.PurgeProductCache)
	admin.GET(AdminProductDetail, h.GetAdminProductDetail)
	admin.PUT(AdminProductStatus, h.UpdateAdminProductStatus)

	// 商品認證審核 /api/v1/admin/product_certification_applications（19 §9.2 / §9.3）
	// 掛 admin group（JWTAuth + requireAdmin）——不是 adminNoAuth，那個 sibling group 是
	// 代行營運專用，掛錯會在 boot 斷言 panic。
	admin.GET(AdminProductCertificationApplications, h.ListAdminProductCertificationApplications)
	admin.GET(AdminProductCertificationApplicationDetail, h.GetAdminProductCertificationApplication)
	admin.GET(AdminProductCertificationAuditLogs, h.ListAdminProductCertificationAuditLogs)
	admin.POST(AdminProductCertificationApprove, h.ApproveAdminProductCertification)
	admin.POST(AdminProductCertificationReject, h.RejectAdminProductCertification)
	admin.POST(AdminProductCertificationRevoke, h.RevokeAdminProductCertification)
	admin.POST(AdminProductCertificationReverify, h.ReverifyAdminProductCertification)

	// 類別管理 /api/v1/admin/categories（2026-04-25 政策大改造後）
	admin.GET(CategoriesPath+CategoriesTree, h.GetAdminCategoryTree)
	admin.POST(CategoriesPath, h.CreateCategoryAdmin)
	admin.PATCH(CategoriesPath+CategoryID, h.UpdateCategoryAdmin)
	admin.DELETE(CategoriesPath, h.BatchSoftDeleteCategoryAdmin)
	admin.POST(CategoriesPath+CategoryRestore, h.RestoreCategoryAdmin)
	admin.GET(CategoriesPath+CategoryProducts, h.ListProductsByCategory)
	admin.POST(ProductsBatchChangeCat, h.BatchChangeProductCategory)

	return pubProducts
}
