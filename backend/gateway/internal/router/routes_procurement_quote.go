package router

import (
	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

// registerProcurementQuoteRoutes 掛「Hawkr 幫找」報價單邀請的兩個確認端點（doc 55 §5/§7）。
//
// 買家端（寄送／撤回報價單）不在這裡，而是跟其餘採購需求端點一起掛在
// routes_ai.go 的 /app/buyers/me/procurement 群組——那一組共用同一個 JWTAuth +
// requireBuyer 中介層鏈，拆開只會讓同一個資源的路由散在兩處。這個檔案存在的理由是
// 這兩個端點跟那一組的中介層鏈完全不同（一個是賣家 capability gate、一個完全免認證），
// 掛不進去。
//
// **capability 選 CapMarketing，不是計畫草稿寫的 CapOrderContract**：
// CapOrderContract 在 suspended 白名單裡（seller 模組的停權白名單，語意是
// 「收縮型操作」，例如取消出貨）。確認報價單邀請是承接一筆新生意——擴張型操作，
// 停權賣家不該做得到。CapMarketing 是 BLOCK-if-suspended，而且賣家 AI 助手整組
// （routes_ai.go 的 sellerAI 群組）本來就用它，這個端點是同一個 AI 採購功能的
// 賣家面，歸在同一個 capability 下最一致。
func registerProcurementQuoteRoutes(app *gin.RouterGroup, h *handler.Holder) {
	// #2538 賣家回價：capability 同上取 CapMarketing——它不在停權白名單，
	// 所以停權賣家送不出正式回價，這正是 Wish Pool policy v1（全站未停權賣家）在
	// 寫入端的鏡像。為這條線新增第 15 個 capability 是另一個決定，不在本票範圍。
	sellerProcurement := app.Group(SellerProcurementPath)
	sellerProcurement.Use(middleware.PrivateNoStore(), middleware.JWTAuth())
	{
		sellerHandle(sellerProcurement, "GET", SellerProcurementPublications, authz.CapMarketing, h.ProxySellerProcurementPublications)
		sellerHandle(sellerProcurement, "GET", SellerProcurementPublicationID, authz.CapMarketing, h.ProxySellerProcurementPublicationDetail)
		sellerHandle(sellerProcurement, "PUT", SellerProcurementResponseDraft, authz.CapMarketing, h.ProxySellerProcurementResponseDraft)
		sellerHandle(sellerProcurement, "POST", SellerProcurementResponses, authz.CapMarketing, h.ProxySellerProcurementSubmitResponse)
		sellerHandle(sellerProcurement, "POST", SellerProcurementWithdraw, authz.CapMarketing, h.ProxySellerProcurementWithdrawResponse)
		sellerHandle(sellerProcurement, "POST", SellerProcurementDecline, authz.CapMarketing, h.ProxySellerProcurementDecline)
	}

}
