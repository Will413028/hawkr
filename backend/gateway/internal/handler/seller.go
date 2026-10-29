package handler

import (
	"context"
	"strconv"
	"strings"

	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/go-pkg/identity"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/mediaurl"
	pb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"github.com/Will413028/hawkr/go-pkg/syncutil"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// SellerResponse is the API response for merchant endpoints
type SellerResponse struct {
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
	TraceID string      `json:"traceId,omitempty"`
	Code    int32       `json:"code"`
}

// PublicSellerInfo 是 GET /api/v1/public/sellers/{id} 的 response DTO —— 匿名可讀。
//
// 公開面只放店家的營業資訊，不含店主的 email／手機／生日／性別／寄退件街道地址：
// 網頁賣家頁的資料來源正是這支，任何人帶一個 store_id 就拿得到。
//
// 白名單設計（deny-by-default）：新欄位不會因為上游 proto 長胖而自動出現在公開
// 面，要公開得在這裡明確加一行。加之前先回答一句話——「這是店家的營業資訊，還是
// 店主的個人資料？」後者不進本結構。
//
// 明確排除且不得加回（見 seller_public_pii_test.go 的契約測試）：
//   - email / phone / contactPhone：三者都是帳號註冊時已驗證的**個人**信箱與手機。
//     seller 模組 `toResponse` 是 `Email: m.Email, ContactPhone: m.ContactPhone,
//     Phone: m.ContactPhone` —— phone 與 contactPhone 是同一個值；賣家自我讀取路徑
//     還會拿 identity 模組的已驗證手機回填 contact_phone，而 GetShippingProfile
//     把它們當寄件人 SenderPhone／ContactEmail 用。那是物流寄件人身分，不是客服專線。
//   - birthday / gender：店主個人資料，與「商家資訊」無關。
//   - address / addressDetail / postalCode：賣家端填寫時的語意是「營運地址 — 用於
//     物流開通、寄退件與店舖地區顯示」。**只有「地區顯示」是公開用途**，city +
//     district 已足夠；街道級地址是寄退件用，個人賣家填的多半就是住家。
//   - certificationDraftId / currentCertificationId：認證流程的內部記錄 id。
//   - accountType / needsOnboarding / lifecycleStatus / suspensionReasonCode：
//     帳號與治理內部狀態（46 §7.2 明載後兩者是給賣家自己的停權公告頁用）。公開查詢已在 seller 模組過濾 closed 店，公開面不需要它們。
//
// 保留 overallCertificationStatus：它是買家端的信任標章（買家 App 商家頁讀它顯示
// 認證徽章），屬於營業資訊。
type PublicSellerInfo struct {
	StoreID     string `json:"storeId"`
	StoreCode   string `json:"storeCode"`
	StoreName   string `json:"storeName"`
	SellerName  string `json:"sellerName"` // == storeName（seller 模組 toResponse: Nickname = StoreName）
	Description string `json:"description"`
	LogoKey     string `json:"logoKey"`  // 商家 logo objectKey（SSOT；49 §6.1）
	LogoUrl     string `json:"logoUrl"`  // 商家 logo 顯示 URL（Gateway 組）
	CoverKey    string `json:"coverKey"` // 商家封面 objectKey（SSOT；49 §6.1）
	CoverUrl    string `json:"coverUrl"` // 商家封面顯示 URL（Gateway 組）
	Prize       string `json:"prize"`
	// city / district 是「店舖地區顯示」——公開面地址精度到此為止，不含街道。
	City                       string  `json:"city"`
	District                   string  `json:"district"`
	SellerType                 string  `json:"sellerType"`
	OverallCertificationStatus string  `json:"overallCertificationStatus"`
	WebsiteUrl                 string  `json:"websiteUrl"`
	Rating                     float64 `json:"rating"`
}

// toPublicSellerInfoDTO 是 gRPC SellerInfo proto → 公開 DTO 的**唯一**映射點。
// 逐欄明列（不用 struct copy / 反射），漏一欄只會少顯示東西，不會多洩漏東西。
func toPublicSellerInfoDTO(m *pb.SellerInfo, mediaBase string) PublicSellerInfo {
	if m == nil {
		return PublicSellerInfo{}
	}
	// 刻意**沒有**登入 principal 的 fallback：
	// 44 號身分分離下，登入 principal 與賣家營運鍵 store_id 是兩個 id，公開面沒有
	// 理由讓 principal 出現。實務上也用不到——sellers.store_id 是 NOT NULL PK，
	// mapper 一定填得出來；真的空了就回空字串，不要拿另一個 id 頂上。
	return PublicSellerInfo{
		StoreID:                    m.StoreId,
		StoreCode:                  m.StoreCode,
		StoreName:                  m.Name,
		SellerName:                 m.Nickname,
		Description:                m.Description,
		LogoKey:                    m.LogoKey,
		LogoUrl:                    buildURL(mediaBase, m.LogoKey),
		CoverKey:                   m.CoverKey,
		CoverUrl:                   buildURL(mediaBase, m.CoverKey),
		Prize:                      m.Prize,
		City:                       m.City,
		District:                   m.District,
		SellerType:                 m.SellerType,
		OverallCertificationStatus: m.OverallCertificationStatus,
		WebsiteUrl:                 m.WebsiteUrl,
		Rating:                     m.Rating,
	}
}

// SellerStats represents merchant statistics for API response
type SellerStats struct {
	StoreID       string  `json:"storeId"`
	TotalProducts int32   `json:"totalProducts"`
	TotalOrders   int32   `json:"totalOrders"`
	TotalSales    float64 `json:"totalSales"`
	Rating        float64 `json:"rating"`
	Followers     int32   `json:"followers"`
}

// GetSellerInfo returns the anonymous-readable public storefront profile.
//
// 回的是 PublicSellerInfo（白名單）。這支匿名可打，店主的個人聯絡資料與街道地址
// 不得經由它外流。理由與排除清單見 PublicSellerInfo 的型別註解。
// @Summary      取得商家資訊（公開）
// @Description  取得店面的公開資訊。僅回營業面欄位；店主個人資料（email／手機／生日／性別）與街道級地址不在此端點暴露。
// @Tags         商家 (Seller)
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "店家 ID（store_id 或 store_code）"
// @Success      200  {object}  handler.SellerResponse{data=handler.PublicSellerInfo}
// @Failure      400  {object}  handler.SellerResponse
// @Failure      503  {object}  handler.SellerResponse
// @Router /public/sellers/{id} [get]
func (h *Holder) GetSellerInfo(c *gin.Context) {
	storeIDStr := c.Param("id")
	storeID, err := strconv.ParseInt(storeIDStr, 10, 64)
	if err != nil || storeID <= 0 {
		response.BadRequest(c, "invalid seller id")
		return
	}

	resp, err := h.Seller.GetSellerByStoreID(c.Request.Context(), storeID)
	if err != nil {
		pkglogger.Info(c.Request.Context(), "Failed to get seller info", zap.Error(err))
		response.GrpcErr(c, "seller", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}
	// fire-and-forget：訪客限定買家——GetSellerInfo 掛 OptionalJWTAuth，任何登入身分（含
	// 賣家帳號）都會有 accountId，但「訪客」語意只算買家，所以先解析現任 buyer_id，解析
	// 失敗（含 ErrNoCurrentBuyer：賣家帳號、尚未綁買家角色）直接略過不計入。
	// 自訪排除比對店主的 raw account_id（resp.Data.AccountId），不是 storeID（TSID
	// surrogate）——舊寫法 vid != storeID 對任何重鍵過的商店恆真，等於從沒真的排除過店主。
	if visitorID, ok := c.Get("accountId"); ok {
		if vid, ok := visitorID.(int64); ok && vid != resp.Data.AccountId {
			syncutil.SafeGo(context.Background(), func() {
				buyerID, err := h.BuyerID.CurrentBuyerID(context.Background(), identity.AccountID(vid))
				if err != nil {
					return
				}
				_ = h.Seller.RecordStoreVisit(context.Background(), storeID, int64(buyerID))
			})
		}
	}

	response.OK(c, toPublicSellerInfoDTO(resp.Data, h.MediaBaseURL))
}

// resolveStorefrontSellerID resolves a public storefront path id — which may
// be either the seller's store_id (PK surrogate, pre-2026-07-23 links) or
// store_code (the short display code storefront URLs use going forward) —
// into the real store_id every downstream service call (stats/products/
// reviews) is keyed by. Reuses the existing GetSeller RPC purely as a
// resolver — it's already store_code-aware via the query fix backing
// GetPublicSellerProfileByStoreID, so no new RPC is needed. On
// failure the error response has already been written; callers must return
// immediately when ok is false.
func (h *Holder) resolveStorefrontSellerID(c *gin.Context, idParam string) (storeID int64, ok bool) {
	rawID, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil || rawID <= 0 {
		response.BadRequest(c, "invalid seller id")
		return 0, false
	}
	resp, err := h.Seller.GetSellerByStoreID(c.Request.Context(), rawID)
	if err != nil {
		pkglogger.Info(c.Request.Context(), "Failed to resolve seller id", zap.Error(err))
		response.GrpcErr(c, "seller", err)
		return 0, false
	}
	if resp.Code != 0 || resp.Data == nil {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return 0, false
	}
	storeID, err = strconv.ParseInt(resp.Data.StoreId, 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid seller id")
		return 0, false
	}
	return storeID, true
}

// GetSellerStats returns merchant statistics
// @Summary      取得商家統計資料
// @Description  取得商家的商品數、訂單數、銷售額等統計資料
// @Tags         商家 (Seller)
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "店家 ID（store_id 或 store_code）"
// @Success      200  {object}  handler.SellerResponse{data=handler.SellerStats}
// @Failure      400  {object}  handler.SellerResponse
// @Router /public/sellers/{id}/stats [get]
func (h *Holder) GetSellerStats(c *gin.Context) {
	storeIDStr := c.Param("id")
	storeID, err := strconv.ParseInt(storeIDStr, 10, 64)
	if err != nil || storeID <= 0 {
		response.BadRequest(c, "invalid seller id")
		return
	}

	resp, err := h.Seller.GetSellerStatsByStoreID(c.Request.Context(), storeID)
	if err != nil {
		pkglogger.Info(c.Request.Context(), "Failed to get seller stats", zap.Error(err))
		response.GrpcErr(c, "seller", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}
	stats := resp.Data
	response.OK(c, SellerStats{
		StoreID:       stats.StoreId,
		TotalProducts: stats.TotalProducts,
		TotalOrders:   stats.TotalOrders,
		TotalSales:    stats.TotalSales,
		Rating:        stats.Rating,
		Followers:     stats.Followers,
	})
}

// GetSellerProducts 取得指定商家的商品列表（公開 API，僅 ACTIVE）
// @Summary      取得商家的商品列表
// @Description  根據商家帳號 ID 取得該商家上架中的商品列表，支援分類篩選、關鍵字搜尋與分頁
// @Tags         商家 (Seller)
// @Accept       json
// @Produce      json
// @Param        id           path   string  true   "店家 ID（store_id 或 store_code）"
// @Param        category_id  query  string  false  "分類 ID"
// @Param        keyword      query  string  false  "搜尋關鍵字"
// @Param        page         query  string  false  "頁碼（預設為 1）"
// @Param        page_size    query  string  false  "每頁筆數（預設為 20）"
// @Success      200  {object}  object{code=int,data=object{records=[]ProductItem,total=string,page=string,pageSize=string}}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /public/sellers/{id}/products [get]
func (h *Holder) GetSellerProducts(c *gin.Context) {
	storeID, ok := h.resolveStorefrontSellerID(c, c.Param("id"))
	if !ok {
		return
	}

	categoryID := c.DefaultQuery("category_id", "0")
	keyword := c.DefaultQuery("keyword", "")
	page := c.DefaultQuery("page", defaultPageStr)
	pageSize := c.DefaultQuery("page_size", defaultPageSizeStr)
	hashtags := parseHashtagsQuery(c.DefaultQuery("hashtags", ""))
	matchAll := strings.EqualFold(c.DefaultQuery("match", "any"), "all")

	resp, err := h.Product.GetPublicSellerProductListWithHashtags(c.Request.Context(), storeID, categoryID, keyword, page, pageSize, hashtags, matchAll)
	if err != nil {
		response.GrpcErr(c, "product", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	base := h.MediaBaseURL
	products := make([]ProductItem, 0, len(resp.Products))
	for _, p := range resp.Products {
		item := ProductItem{
			ID:              p.Id,
			Name:            p.Name,
			Rating:          p.Rating,
			Status:          p.Status,
			CategoryID:      p.CategoryId,
			CategoryName:    p.CategoryName,
			Origin:          p.Origin,
			ImageKey:        p.ImageKey,
			ImageURL:        mediaurl.BuildURL(base, p.ImageKey),
			ImageKeys:       p.ImageKeys,
			ImageUrls:       buildMediaURLs(base, p.ImageKeys),
			PriceMin:        p.PriceMin,
			PriceMax:        p.PriceMax,
			CreatedAt:       p.CreatedAt,
			ReviewCount:     p.ReviewCount,
			StoreID:         p.StoreId,
			CategoryLv2ID:   p.CategoryLv2Id,
			CategoryLv2Name: p.CategoryLv2Name,
			Hashtags:        toHashtagItems(p.Hashtags),
		}

		// 與 /public/products 對稱（同一支 GetProductList RPC）。本處歷來只手抄
		// name/logo/store_code，漏了 city/district/address——而前端商品卡的產地是
		// `origin || seller.city`，缺 city 時店內商品卡產地就空白；改走共用 builder
		// 一併補齊。攤平便利欄同理補上，與市集/搜尋的 ProductItem 保持同形。
		if p.Seller != nil {
			item.Seller = buildSellerInfoItem(base, p.Seller)
			item.SellerName, item.SellerLogoKey, item.SellerLogoUrl = flatSellerFields(item.Seller)
		}

		products = append(products, item)
	}

	// isRestockSubscribed aggregation (issue #1005 / B-203, 39 §5.5b)
	// buyer_id-first：這裡沒有其他地方已解析過 buyerID，自己做一次 soft-fail 解析
	// （同 product.go GetProductList 的理由）。
	accountID := getOptionalAccountID(c)
	if accountID != 0 && len(products) > 0 {
		ids := make([]string, 0, len(products))
		idx := make(map[string]int, len(products))
		for i, p := range products {
			ids = append(ids, p.ID)
			idx[p.ID] = i
		}
		if buyerID, berr := h.BuyerID.CurrentBuyerID(c.Request.Context(), identity.AccountID(accountID)); berr != nil {
			pkglogger.Debug(c.Request.Context(), "Failed to resolve buyer id for restock subscription aggregation", zap.Error(berr))
		} else {
			h.applyRestockSubscribed(c.Request.Context(), int64(buyerID), ids, func(pid string, subscribed bool) {
				if i, ok := idx[pid]; ok {
					products[i].IsRestockSubscribed = subscribed
				}
			})
		}
	}

	response.Page(c, products, resp.Total, resp.Page, resp.PageSize)
}
