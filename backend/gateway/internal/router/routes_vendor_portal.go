package router

import (
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

// registerVendorPortalRoutes 掛站外業者 Portal 的 RFQ 端點（#2539 E1）。
//
// 兌換、預覽與 Google 驗證在單體（VendorPortalPublicService）；單體驗過 session 後
// 在請求上寫 X-Vendor-Portal-* header，這裡只信任那些 header。
func registerVendorPortalRoutes(public *gin.RouterGroup, h *handler.Holder) {
	vp := public.Group(VendorPortalPath)
	vp.Use(middleware.PrivateNoStore())
	{
		vp.GET("/rfqs/:rfq_recipient_id",
			middleware.VendorPortalSession(), h.VendorPortalRFQ)

		vp.GET("/rfqs/:rfq_recipient_id/response_draft",
			middleware.VendorPortalSession(), h.VendorPortalResponseDraft)
		vp.PUT("/rfqs/:rfq_recipient_id/response_draft",
			middleware.VendorPortalSession(), h.VendorPortalUpsertResponseDraft)

		// 🔴 三支**正式寫入**：草稿可覆寫所以不要求驗證紀錄，這三支產生的是不可撤回
		// 的商務承諾（immutable sellerResponseVersion，買家接受 exact version 即 award）
		// ⇒ 一律要求 Google 驗證紀錄。⚠️ guard 必須排在 VendorPortalSession 之後，
		// 它讀的是那一支放進 context 的值。
		vp.POST("/rfqs/:rfq_recipient_id/response_submissions",
			middleware.VendorPortalSession(),
			middleware.RequireVendorPortalGoogleVerified(), h.VendorPortalSubmitResponse)
		vp.POST("/rfqs/:rfq_recipient_id/seller_responses/:seller_response_id/withdrawals",
			middleware.VendorPortalSession(),
			middleware.RequireVendorPortalGoogleVerified(), h.VendorPortalWithdrawResponse)
		vp.POST("/rfqs/:rfq_recipient_id/declines",
			middleware.VendorPortalSession(),
			middleware.RequireVendorPortalGoogleVerified(), h.VendorPortalDeclineRfq)
	}
}
