package router

import (
	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

func registerProductRoutes(public *gin.RouterGroup, app *gin.RouterGroup, admin *gin.RouterGroup, h *handler.Holder) {

	pubProducts := public.Group(ProductsPath)
	pubProducts.Use(middleware.OptionalJWTAuth())
	{
		pubProducts.GET("", h.GetProductList)
		pubProducts.GET(ProductsIDPath, h.GetProductDetailByID)
		pubProducts.GET(ProductsBrief, h.GetProductInfoByID)
		pubProducts.GET(ProductSkus, h.ListSellerSkus)
		pubProducts.GET(ProductsRating, h.GetProductRating)
		pubProducts.GET(ProductsRelated, h.GetRelatedProducts)
	}

	pubHashtags := public.Group(HashtagsPath)
	pubHashtags.Use(middleware.OptionalJWTAuth())
	{
		pubHashtags.GET("", h.SearchHashtags)
		pubHashtags.GET("/popular", h.GetPopularHashtags)
	}

	public.GET(OriginsSummaryPath, middleware.OptionalJWTAuth(), h.GetOriginsSummary)
	public.GET(SellerCitiesSummaryPath, middleware.OptionalJWTAuth(), h.GetSellerCitiesSummary)

	pubCategories := public.Group(CategoriesPath)
	pubCategories.Use(middleware.OptionalJWTAuth())
	{
		pubCategories.GET("", h.GetCategoryList)
		pubCategories.GET(CategoriesTree, h.GetCategoryTree)
		pubCategories.GET(CategoryID, h.GetCategoryByID)
		pubCategories.GET(CategoryPathSeg, h.GetCategoryPath)
		pubCategories.GET(CategoryChildren, h.GetCategoryChildren)
	}


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
		sellerHandle(appProducts, "GET", ProductCertificationApplications, authz.CapProductManage, h.ListSellerProductCertificationApplications)
		sellerHandle(appProducts, "POST", ProductCertificationApplications, authz.CapProductManage, h.SubmitSellerProductCertification)
		sellerHandle(appProducts, "POST", ProductCertificationApplicationWithdraw, authz.CapProductManage, h.WithdrawSellerProductCertification)
	}

	appCategories := app.Group(CategoriesPath)
	appCategories.Use(middleware.JWTAuth())
	{
		sellerHandle(appCategories, "POST", "", authz.CapProductManage, h.CreateCategory)
		sellerHandle(appCategories, "GET", CategoriesTree, authz.CapProductManage, h.GetSellerCategoryTree)
	}

	sellerHandleAuthed(app, "POST", ShippingQuotesPath, authz.CapProductManage, h.CalculateShippingOptions)

	appHashtags := app.Group(HashtagsPath)
	appHashtags.Use(middleware.JWTAuth())
	{
		sellerHandle(appHashtags, "GET", "", authz.CapProductManage, h.SearchHashtags)
	}


	admin.GET(AdminProductsPath, h.SearchAdminProducts)
	admin.DELETE(AdminProductCache, h.PurgeProductCache)
	admin.GET(AdminProductDetail, h.GetAdminProductDetail)
	admin.PUT(AdminProductStatus, h.UpdateAdminProductStatus)

	admin.GET(AdminProductCertificationApplications, h.ListAdminProductCertificationApplications)
	admin.GET(AdminProductCertificationApplicationDetail, h.GetAdminProductCertificationApplication)
	admin.GET(AdminProductCertificationAuditLogs, h.ListAdminProductCertificationAuditLogs)
	admin.POST(AdminProductCertificationApprove, h.ApproveAdminProductCertification)
	admin.POST(AdminProductCertificationReject, h.RejectAdminProductCertification)
	admin.POST(AdminProductCertificationRevoke, h.RevokeAdminProductCertification)
	admin.POST(AdminProductCertificationReverify, h.ReverifyAdminProductCertification)

	admin.GET(CategoriesPath+CategoriesTree, h.GetAdminCategoryTree)
	admin.POST(CategoriesPath, h.CreateCategoryAdmin)
	admin.PATCH(CategoriesPath+CategoryID, h.UpdateCategoryAdmin)
	admin.DELETE(CategoriesPath, h.BatchSoftDeleteCategoryAdmin)
	admin.POST(CategoriesPath+CategoryRestore, h.RestoreCategoryAdmin)
	admin.GET(CategoriesPath+CategoryProducts, h.ListProductsByCategory)
	admin.POST(ProductsBatchChangeCat, h.BatchChangeProductCategory)

}
