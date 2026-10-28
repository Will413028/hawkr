package handler

import (
	"errors"
	"strconv"

	"github.com/Will413028/hawkr/gateway/internal/buyerid"
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/go-pkg/httputil"
	"github.com/Will413028/hawkr/go-pkg/identity"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/mediaurl"
	pb "github.com/Will413028/hawkr/go-pkg/proto/review"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// SubmitOrderItemReviewRequest 提交訂單商品評論請求
type SubmitOrderItemReviewRequest struct {
	OrderNo string `json:"orderNo" binding:"required"`
	Content string `json:"content"`
	SkuName string `json:"skuName"`
	// Deprecated: item image 改由後端從訂單商品圖快照回填（49 §5.3），此欄位一律被忽略；保留僅為相容既有前端。
	ItemImageURL string   `json:"productImageUrl"`
	StagedTokens []string `json:"stagedTokens"` // 評論圖片 staged token（最多 4 個，選填）
	OrderItemID  string   `json:"orderItemId" binding:"required"`
	ProductID    string   `json:"productId"`
	Price        int64    `json:"price"`
	Rating       int32    `json:"rating" binding:"required,min=1,max=5"`
}

// @Summary      提交訂單商品評論
// @Description  買家對已完成訂單的商品項目提交一次評論。圖片請先透過 /app/media/upload_sessions 上傳並取得 stagedToken，再傳入 staged_tokens（最多 4 個）
// @Tags         評論 (Review)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  handler.SubmitOrderItemReviewRequest  true  "評論資料"
// @Success      200  {object}  object{code=int,message=string,data=object{reviewId=string}}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Failure      409  {object}  object{code=int,message=string}
// @Failure      410  {object}  object{code=int,message=string}
// @Router /app/reviews [post]
func (h *Holder) SubmitOrderItemReview(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	buyerID, err := h.BuyerID.CurrentBuyerID(c.Request.Context(), identity.AccountID(accountID))
	if err != nil {
		if errors.Is(err, buyerid.ErrNoCurrentBuyer) {
			response.Forbidden(c, "buyer access required")
			return
		}
		response.GrpcErr(c, "buyer", err)
		return
	}

	var req SubmitOrderItemReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	resp, err := h.OrderReview.SubmitOrderReview(c.Request.Context(), &pb.SubmitOrderReviewRequest{
		OrderNo:      req.OrderNo,
		OrderItemId:  req.OrderItemID,
		BuyerId:      int64(buyerID),
		ProductId:    req.ProductID,
		Rating:       req.Rating,
		Content:      req.Content,
		StagedTokens: req.StagedTokens,
		SkuName:      req.SkuName,
		// item_image_url 不再由前端決定：review-service 從訂單商品圖快照回填（49 §5.3）。
		// 保留 request DTO 欄位相容既有前端，但不轉發（review 端一律忽略）。
		Price: strconv.FormatInt(req.Price, 10),
	})
	if err != nil {
		response.GrpcErr(c, "review", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}
	response.OK(c, gin.H{"reviewId": resp.ReviewId})
}

// @Summary      買家評論歷史
// @Description  查詢當前買家過去提交的所有評論
// @Tags         評論 (Review)
// @Produce      json
// @Security     BearerAuth
// @Param        page  query  int  false  "頁碼（預設 1）"
// @Param        size  query  int  false  "每頁筆數（預設 20）"
// @Success      200  {object}  object{code=int,message=string,data=object{records=[]handler.OrderReviewRecord,total=string,page=string,pageSize=string}}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router /app/me/reviews [get]
func (h *Holder) ListMyOrderReviews(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	buyerID, err := h.BuyerID.CurrentBuyerID(c.Request.Context(), identity.AccountID(accountID))
	if err != nil {
		if errors.Is(err, buyerid.ErrNoCurrentBuyer) {
			response.Forbidden(c, "buyer access required")
			return
		}
		response.GrpcErr(c, "buyer", err)
		return
	}

	page := httputil.ParsePageParam(c.Query("page"), 1)
	pageSize := httputil.ParsePageParam(c.Query("size"), 10)

	resp, err := h.OrderReview.ListMyOrderReviews(c.Request.Context(), &pb.ListMyOrderReviewsRequest{
		BuyerId:  int64(buyerID),
		Page:     int32(page),
		PageSize: int32(pageSize),
	})
	if err != nil {
		pkglogger.Info(c.Request.Context(), "ListMyOrderReviews failed", zap.Error(err))
		response.ServiceUnavailable(c, "review")
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}
	response.OK(c, gin.H{
		"records":  toOrderReviewRecords(h.MediaBaseURL, resp.Records),
		"total":    resp.Total,
		"page":     resp.Page,
		"pageSize": resp.PageSize,
	})
}

// PublicSellerReviewItem 公開賣家評論項目（排除買家身分資訊）
type PublicSellerReviewItem struct {
	ID      string `json:"id"`
	Content string `json:"content"`
	SkuName string `json:"skuName,omitempty"`
	// 49 §6.1 雙欄位：*Key = objectKey（DB 值），*Url = Gateway 組的顯示 URL。
	ItemImageKey string `json:"productImageKey,omitempty"`
	ItemImageURL string `json:"productImageUrl,omitempty"`
	ProductID    string `json:"productId,omitempty"` // #1155：商品 ID，前端連結商品頁；legacy product_id=0 留空避免死連結
	UserName     string `json:"userName"`
	// 49 §6.6 S2：avatarKey = objectKey/外部 URL（proto 值）；avatarUrl = Gateway 組顯示 URL。
	UserAvatarKey string       `json:"avatarKey,omitempty"`
	UserAvatar    string       `json:"avatarUrl,omitempty"`
	CreatedAt     string       `json:"createdAt"`
	ImageKeys     []string     `json:"imageKeys,omitempty"` // 評論附圖 objectKey 陣列
	Images        []string     `json:"imageUrls,omitempty"` // 對應 imageKeys 的顯示 URL
	Reply         *ReviewReply `json:"reply,omitempty"`     // #1174：賣家回覆，未回覆為 null
	Rating        int32        `json:"rating"`
}

// OrderReviewRecord 是 OrderReviewInfo 的 Gateway DTO（買家評論歷史 / 賣家收到評論）。
// 依 49 §6.1 成對輸出 objectKey 與 Gateway 組的顯示 URL（proto 值已是 objectKey）。
//
// 這個物件同時帶「被評論商品的封面」與「買家上傳的評論附圖」兩組圖，所以商品那組
// 保留 product 前綴（productImageKey/productImageUrl）：若收斂成 imageKey，單數
// imageKey 與複數 imageKeys 就會指涉兩件不同的事，比不一致更難讀。
type OrderReviewRecord struct {
	ID           string `json:"id"`
	OrderNo      string `json:"orderNo"`
	OrderItemID  string `json:"orderItemId"`
	BuyerID      string `json:"buyerId,omitempty"`
	StoreID      string `json:"storeId,omitempty"`
	Content      string `json:"content,omitempty"`
	SkuName      string `json:"skuName,omitempty"`
	ItemImageKey string `json:"productImageKey,omitempty"`
	ItemImageURL string `json:"productImageUrl,omitempty"`
	Price        string `json:"price,omitempty"`
	CreatedAt    string `json:"createdAt"`
	BuyerName    string `json:"buyerName,omitempty"`
	// 49 §6.6 S2：avatarKey = objectKey/外部 URL（proto 值）；avatarUrl = Gateway 組顯示 URL。
	BuyerAvatarKey string   `json:"avatarKey,omitempty"`
	BuyerAvatar    string   `json:"avatarUrl,omitempty"`
	ProductID      string   `json:"productId,omitempty"`
	ImageKeys      []string `json:"imageKeys,omitempty"`
	// imageUrls 沿用 proto JSON 欄名（改版前這兩個端點 raw passthrough 即以此名輸出評論圖 URL），
	// 值改由 Gateway 組（§6.1；非破壞性——欄名不變、值仍為完整 URL）。
	ImageURLs []string     `json:"imageUrls,omitempty"`
	Reply     *ReviewReply `json:"reply,omitempty"`
	Rating    int32        `json:"rating"`
}

// toOrderReviewRecords 將 proto OrderReviewInfo（image_urls/item_image_url 值為 objectKey）
// 轉為含雙欄位的 Gateway DTO，顯示 URL 以 mediaBaseURL 組（49 §6.1）。
// buyerDisplayName 是買家名稱的**最後防線**（#2830）。
//
// 主要答案來自 buyer 模組的 BatchGetBuyerDisplayByBuyerIDs：買家註銷後它回
// 墓碑字串「已刪除帳號」，所以正常路徑上這裡拿到的永遠是可顯示的值。
//
// 🔴 這兩個字串代表**不同狀態**，不是同一件事的兩種講法：
//
//	「已刪除帳號」＝ 買家存在過、自己註銷了（**解析成功**）
//	「匿名買家」  ＝ 連買家列都查不到，或上游降級（**解析失敗**）
//
// 把後者也講成「已刪除帳號」會把「我不知道」講成「我知道他刪了」，那是造假。
//
// ⚠️ 不要把這個 fallback 塞進 maskUsername：那支有測試釘住 ""→"" 的行為
// （review_test.go 的 TestMaskUsername），而且遮罩與缺值是兩個不同的關注點。
func buyerDisplayName(name string) string {
	if name == "" {
		return "匿名買家"
	}
	return name
}

// maskBuyerDisplayName 是公開評論列表專用：**只遮罩真名**。
//
// 為什麼不能直接 maskUsername(buyerDisplayName(x))（2026-08-29 R2 抓到）：
// 遮罩的目的是不讓匿名訪客看到某個真人的完整暱稱。而「已刪除帳號」與「匿名買家」
// 都是**我方產生的常數**，不是任何人的個資 —— 遮了不保護任何人，只會產生
// 「已刪***」「匿名**」這種既不好讀、又讓同一個買家在三個評論畫面顯示三種
// 字串的結果（賣家端的 order_review 兩條路徑本來就不遮罩）。
//
// 🔴 判斷「是不是墓碑」一律讀 deleted 旗標（proto `ReviewInfo.user_deleted`），
// 不要拿 name 去跟墓碑字串做比對 —— 那會在 gateway 再硬編碼一份文案，正是
// BatchGetBuyerDisplayByBuyerIDs 這支 RPC 要消滅的東西。
func maskBuyerDisplayName(name string, deleted bool) string {
	if deleted && name != "" {
		return name // 墓碑常數，原樣顯示
	}
	if name == "" {
		return buyerDisplayName(name) // 解析失敗 → 匿名買家，同樣不遮
	}
	return maskUsername(name)
}

func toOrderReviewRecords(base string, records []*pb.OrderReviewInfo) []OrderReviewRecord {
	out := make([]OrderReviewRecord, 0, len(records))
	for _, r := range records {
		out = append(out, OrderReviewRecord{
			ID:             r.Id,
			OrderNo:        r.OrderNo,
			OrderItemID:    r.OrderItemId,
			BuyerID:        r.BuyerId,
			StoreID:        r.StoreId,
			Content:        r.Content,
			SkuName:        r.SkuName,
			ItemImageKey:   r.ItemImageKey,
			ItemImageURL:   mediaurl.BuildURL(base, r.ItemImageKey),
			Price:          r.Price,
			CreatedAt:      r.CreatedAt,
			BuyerName:      buyerDisplayName(r.BuyerName),
			BuyerAvatarKey: r.BuyerAvatarKey,
			BuyerAvatar:    buildURL(base, r.BuyerAvatarKey),
			ProductID:      r.ProductId,
			ImageKeys:      r.ImageKeys,
			ImageURLs:      buildURLs(base, r.ImageKeys),
			Reply:          toReviewReply(r.GetReply()),
			Rating:         r.Rating,
		})
	}
	return out
}

// buildURLs maps a slice of objectKeys to display URLs（空 slice → nil）。
// emptyIfNil 讓 nil slice 序列化成 [] 而不是 null。對外契約中「沒有圖」只有一種
// 表示法（空陣列），前端不必為 null / [] 各寫一次判斷。
func emptyIfNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// buildURLsAlways 與 buildURLs 相同，但空輸入回傳 [] 而非 nil——給沒有 omitempty
// 的欄位用（imageUrls 等）。
func buildURLsAlways(base string, keys []string) []string {
	return emptyIfNil(buildURLs(base, keys))
}

func buildURLs(base string, keys []string) []string {
	if len(keys) == 0 {
		return nil
	}
	urls := make([]string, 0, len(keys))
	for _, k := range keys {
		urls = append(urls, mediaurl.BuildURL(base, k))
	}
	return urls
}

// buildURL 將單一 objectKey 組為顯示 URL（49 §6.1/§6.6：Gateway 唯一組裝點）。
// key 為空回傳空字串（omitempty 生效）；http 開頭值由 mediaurl.BuildURL passthrough（S2 外部頭像）。
func buildURL(base, key string) string {
	if key == "" {
		return ""
	}
	return mediaurl.BuildURL(base, key)
}

// PublicSellerReviewStatsResponse 公開賣家評論統計
type PublicSellerReviewStatsResponse struct {
	TotalCount   string `json:"totalCount"`
	AvgRating    string `json:"avgRating"`
	Rating5Count string `json:"rating5Count"`
	Rating4Count string `json:"rating4Count"`
	Rating3Count string `json:"rating3Count"`
	Rating2Count string `json:"rating2Count"`
	Rating1Count string `json:"rating1Count"`
}

// @Summary      公開賣家評論列表
// @Description  取得指定賣家收到的買家評論（匿名可存取）
// @Tags         商家 (Seller)
// @Produce      json
// @Param        id    path   string  true   "賣家帳號 ID（store_id 或 store_code）"
// @Param        page       query  string  false  "頁碼（預設 1）"
// @Param        page_size  query  string  false  "每頁筆數（預設 20）"
// @Success      200  {object}  object{code=int,data=object{records=[]PublicSellerReviewItem,total=string,page=string,pageSize=string}}
// @Failure      400  {object}  object{code=int,message=string}
// @Router       /public/sellers/{id}/reviews [get]
func (h *Holder) GetPublicSellerReviews(c *gin.Context) {
	storeID, ok := h.resolveStorefrontSellerID(c, c.Param("id"))
	if !ok {
		return
	}

	page := httputil.ParsePageParam(c.Query("page"), 1)
	pageSize := httputil.ParsePageParam(c.Query("page_size"), 10)

	resp, err := h.OrderReview.GetSellerOrderReviews(c.Request.Context(), &pb.GetSellerOrderReviewsRequest{
		StoreId:  strconv.FormatInt(storeID, 10),
		Page:     int32(page),
		PageSize: int32(pageSize),
	})
	if err != nil {
		pkglogger.Info(c.Request.Context(), "GetPublicSellerReviews failed", zap.Error(err))
		response.ServiceUnavailable(c, "review")
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	items := make([]PublicSellerReviewItem, 0, len(resp.Records))
	for _, r := range resp.Records {
		userName := buyerDisplayName(r.BuyerName)
		// #1155：product_id 為 "0"（legacy 未回填）或空時留空，前端不渲染死連結 /products/0
		productID := r.ProductId
		if productID == "0" {
			productID = ""
		}
		items = append(items, PublicSellerReviewItem{
			ID:      r.Id,
			Rating:  r.Rating,
			Content: r.Content,
			// proto image_urls/item_image_url 值為 objectKey；雙欄位輸出 key + 顯示 URL（49 §6.1）
			ImageKeys:     r.ImageKeys,
			Images:        buildURLs(h.MediaBaseURL, r.ImageKeys),
			SkuName:       r.SkuName,
			ItemImageKey:  r.ItemImageKey,
			ItemImageURL:  mediaurl.BuildURL(h.MediaBaseURL, r.ItemImageKey),
			ProductID:     productID,
			UserName:      userName,
			UserAvatarKey: r.BuyerAvatarKey,
			UserAvatar:    buildURL(h.MediaBaseURL, r.BuyerAvatarKey),
			CreatedAt:     r.CreatedAt,
			Reply:         toReviewReply(r.GetReply()),
		})
	}

	response.Page(c, items, resp.Total, resp.Page, resp.PageSize)
}

// @Summary      公開賣家評論統計
// @Description  取得指定賣家的評論統計（總數、平均評分、星級分布）
// @Tags         商家 (Seller)
// @Produce      json
// @Param        id  path  string  true  "賣家帳號 ID（store_id 或 store_code）"
// @Success      200  {object}  object{code=int,data=PublicSellerReviewStatsResponse}
// @Failure      400  {object}  object{code=int,message=string}
// @Router       /public/sellers/{id}/reviews/stats [get]
func (h *Holder) GetPublicSellerReviewStats(c *gin.Context) {
	storeID, ok := h.resolveStorefrontSellerID(c, c.Param("id"))
	if !ok {
		return
	}

	resp, err := h.OrderReview.GetSellerOrderReviewStats(c.Request.Context(), &pb.GetSellerOrderReviewStatsRequest{
		StoreId: strconv.FormatInt(storeID, 10),
	})
	if err != nil {
		pkglogger.Info(c.Request.Context(), "GetPublicSellerReviewStats failed", zap.Error(err))
		response.ServiceUnavailable(c, "review")
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	response.OK(c, PublicSellerReviewStatsResponse{
		TotalCount:   resp.TotalCount,
		AvgRating:    resp.AvgRating,
		Rating5Count: resp.Rating_5Count,
		Rating4Count: resp.Rating_4Count,
		Rating3Count: resp.Rating_3Count,
		Rating2Count: resp.Rating_2Count,
		Rating1Count: resp.Rating_1Count,
	})
}

// @Summary      賣家評論列表
// @Description  查詢當前賣家收到的所有商品評論，支援星級篩選
// @Tags         商家 (Seller)
// @Produce      json
// @Security     BearerAuth
// @Param        page    query  int  false  "頁碼（預設 1）"
// @Param        size    query  int  false  "每頁筆數（預設 20）"
// @Param        rating  query  int  false  "星級篩選（1–5，0 表示不篩選）"
// @Success      200  {object}  object{code=int,message=string,data=object{records=[]handler.OrderReviewRecord,total=string,page=string,pageSize=string}}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router /app/sellers/me/reviews [get]
func (h *Holder) GetSellerOrderReviews(c *gin.Context) {
	sellerID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	page := httputil.ParsePageParam(c.Query("page"), 1)
	pageSize := httputil.ParsePageParam(c.Query("size"), 10)
	rating, _ := strconv.Atoi(c.DefaultQuery("rating", "0")) // rating=0 是「不篩選」語意，非分頁 default fallback，故不用 ParsePageParam

	resp, err := h.OrderReview.GetSellerOrderReviews(c.Request.Context(), &pb.GetSellerOrderReviewsRequest{
		StoreId:  strconv.FormatInt(int64(sellerID), 10),
		Rating:   int32(rating),
		Page:     int32(page),
		PageSize: int32(pageSize),
	})
	if err != nil {
		pkglogger.Info(c.Request.Context(), "GetSellerOrderReviews failed", zap.Error(err))
		response.ServiceUnavailable(c, "review")
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}
	// 走 response.Page（＝ data.pageSize），與其餘分頁端點同形。
	// 唯一消費者是賣家 App 的 order_repository.dart（反查訂單評論），它只讀
	// data.records、用自己請求的 pageSize 判斷還有沒有下一頁，不讀回應的分頁欄，
	// 所以這支不像 /app/me/reviews 那樣被 web 卡住，可以直接收斂。
	response.Page(c, toOrderReviewRecords(h.MediaBaseURL, resp.Records), resp.Total, resp.Page, resp.PageSize)
}

// GetSellerOrderReviewStats returns review stats for a seller.
// @Summary      賣家評論統計
// @Description  取得當前賣家的評論統計資料（總數與平均評分）
// @Tags         商家 (Seller)
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  object{code=int,data=object{totalCount=int64,avgRating=float64}}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router /app/sellers/me/reviews/stats [get]
func (h *Holder) GetSellerOrderReviewStats(c *gin.Context) {
	sellerID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	resp, err := h.OrderReview.GetSellerOrderReviewStats(c.Request.Context(), &pb.GetSellerOrderReviewStatsRequest{
		StoreId: strconv.FormatInt(int64(sellerID), 10),
	})
	if err != nil {
		pkglogger.Info(c.Request.Context(), "GetSellerOrderReviewStats failed", zap.Error(err))
		response.ServiceUnavailable(c, "review")
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}
	response.OK(c, gin.H{
		"totalCount": resp.TotalCount,
		"avgRating":  resp.AvgRating,
	})
}

// ReplyReviewRequestBody 賣家回覆評論請求 body（#1174）
type ReplyReviewRequestBody struct {
	Content string `json:"content" binding:"required"`
}

// @Summary      賣家回覆評論
// @Description  賣家對自己商品收到的評論張貼一則官方回覆（可編輯覆蓋）。僅該評論的賣家可操作；已隱藏 / 軟刪除評論不可回覆。
// @Tags         商家 (Seller)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path  string                          true  "評論 ID"
// @Param        request  body  handler.ReplyReviewRequestBody  true  "回覆內容"
// @Success      200  {object}  object{code=int,data=object{reviewId=string,reply=handler.ReviewReply}}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Failure      409  {object}  object{code=int,message=string}
// @Router /app/sellers/reviews/{id}/reply [post]
func (h *Holder) ReplyReview(c *gin.Context) {
	sellerID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	reviewID := c.Param("id")
	if reviewID == "" {
		response.BadRequest(c, "invalid review id")
		return
	}

	var req ReplyReviewRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	resp, err := h.OrderReview.ReplyReview(c.Request.Context(), &pb.ReplyReviewRequest{
		ReviewId: reviewID,
		StoreId:  int64(sellerID),
		Content:  req.Content,
	})
	if err != nil {
		response.GrpcErr(c, "review", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	// 回 camelCase DTO（reply 必非 nil：剛回覆成功）
	response.OK(c, gin.H{
		"reviewId": resp.Review.GetId(),
		"reply":    toReviewReply(resp.Review.GetReply()),
	})
}
