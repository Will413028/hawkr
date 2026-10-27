package router

import (
	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

// 商品匯入精靈服務 — 賣家端 REST 入口
// 對應 40 號詳細設計書 §4.1（共 17 endpoint），全部限定 SELLER 身份
//
// Namespace：/api/v1/app/product_import_jobs/*
const (
	ProductImportJobsPath          = "/product_import_jobs"
	ProductImportJobID             = "/:import_job_id"
	ProductImportJobCsv            = "/csv"
	ProductImportJobPreviewListing = "/preview_listing"
	ProductImportJobCancel         = "/cancel"
	ProductImportJobCommit         = "/commit"
	ProductImportJobItems          = "/items"
	ProductImportJobItemID         = "/:import_item_id"
	ProductImportJobItemPreview    = "/preview"
	ProductImportJobItemsRemove    = "/remove"
	ProductImportJobItemsRestore   = "/restore"
	ProductImportJobItemsRetry     = "/retry_assets"
	ProductImportJobItemsBatchShip = "/batch_update_shipping"
	ProductImportJobItemsBatchStat = "/batch_update_target_status"
	ProductImportJobItemsBatchCat  = "/batch_update_category"
	ProductImportJobCategoryTokens = "/category_confirmation_tokens"
)

// registerProductImportRoutes 配置商品匯入精靈服務路由（全部 /app/* 賣家身份）
func registerProductImportRoutes(app *gin.RouterGroup, h *handler.Holder) {
	// 商品匯入精靈全部 CAP_PRODUCT_MANAGE（第二條建商品路徑；唯讀查詢也鎖）。
	g := app.Group(ProductImportJobsPath)
	g.Use(middleware.JWTAuth())
	const cap = authz.CapProductManage
	{
		// ===== Job 級 =====
		sellerHandle(g, "POST", "", cap, h.CreateImportJob)
		sellerHandle(g, "GET", "", cap, h.ListImportJobs)
		sellerHandle(g, "PUT", ProductImportJobID+ProductImportJobCsv, cap, h.ReuploadImportJobCsv)
		sellerHandle(g, "GET", ProductImportJobID, cap, h.GetImportJob)
		sellerHandle(g, "GET", ProductImportJobID+ProductImportJobPreviewListing, cap, h.GetImportJobPreviewListing)
		sellerHandle(g, "POST", ProductImportJobID+ProductImportJobCancel, cap, h.CancelImportJob)
		sellerHandle(g, "POST", ProductImportJobID+ProductImportJobCommit, cap, h.CommitImportJob)

		// ===== Item 級 =====
		sellerHandle(g, "GET", ProductImportJobID+ProductImportJobItems, cap, h.ListImportItems)
		sellerHandle(g, "GET", ProductImportJobID+ProductImportJobItems+ProductImportJobItemID, cap, h.GetImportItem)
		sellerHandle(g, "GET", ProductImportJobID+ProductImportJobItems+ProductImportJobItemID+ProductImportJobItemPreview, cap, h.GetImportItemPreview)
		sellerHandle(g, "PUT", ProductImportJobID+ProductImportJobItems+ProductImportJobItemID, cap, h.UpdateImportItem)
		sellerHandle(g, "POST", ProductImportJobID+ProductImportJobItems+ProductImportJobItemsRemove, cap, h.RemoveImportItems)
		sellerHandle(g, "POST", ProductImportJobID+ProductImportJobItems+ProductImportJobItemsRestore, cap, h.RestoreImportItems)
		sellerHandle(g, "POST", ProductImportJobID+ProductImportJobItems+ProductImportJobItemsRetry, cap, h.RetryImportItemAssets)

		// ===== 批次操作 =====
		sellerHandle(g, "POST", ProductImportJobID+ProductImportJobItems+ProductImportJobItemsBatchShip, cap, h.BatchUpdateItemsShipping)
		sellerHandle(g, "POST", ProductImportJobID+ProductImportJobItems+ProductImportJobItemsBatchStat, cap, h.BatchUpdateItemsTargetStatus)
		sellerHandle(g, "POST", ProductImportJobID+ProductImportJobItems+ProductImportJobItemsBatchCat, cap, h.BatchUpdateItemsCategory)
		sellerHandle(g, "GET", ProductImportJobID+ProductImportJobCategoryTokens, cap, h.GetCategoryConfirmationToken)
	}
}
