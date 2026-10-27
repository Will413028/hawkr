package handler

import (
	"strconv"
	"strings"

	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/go-pkg/httputil"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	pb "github.com/Will413028/hawkr/go-pkg/proto/review"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ReviewReply 賣家回覆（#1174；DTO surface 統一 camelCase 子結構，null = 未回覆）
type ReviewReply struct {
	Content   string `json:"content"`
	RepliedAt string `json:"repliedAt"`
}

// toReviewReply 將 proto ReplyInfo 轉為 DTO（#1174）。nil ReplyInfo → nil（未回覆）。
func toReviewReply(r *pb.ReplyInfo) *ReviewReply {
	if r == nil {
		return nil
	}
	return &ReviewReply{Content: r.GetContent(), RepliedAt: r.GetCreatedAt()}
}

// ReviewListItem 評論列表項目（公開）
type ReviewListItem struct {
	ReviewID  string `json:"reviewId"`
	BuyerID   string `json:"buyerId"`
	ProductID string `json:"productId"` // #1155：商品 ID，契約統一（每則評論帶 productId）
	Username  string `json:"username"`
	// 49 §6.6 S2：avatarKey = objectKey/外部 URL（proto 值）；avatarUrl = Gateway 組顯示 URL。
	UserAvatarKey string       `json:"avatarKey"`
	UserAvatar    string       `json:"avatarUrl"`
	Rating        string       `json:"rating"`
	Content       string       `json:"content"`
	CreatedAt     string       `json:"createdAt"`
	ImageKeys     []string     `json:"imageKeys"`       // 評論附圖 objectKey 陣列（49 §6.1）
	Images        []string     `json:"imageUrls"`       // 對應 imageKeys 的顯示 URL（Gateway 組）
	Reply         *ReviewReply `json:"reply,omitempty"` // #1174：賣家回覆，未回覆為 null
}

// toReviewListItem 轉換 proto ReviewInfo（images 值為 objectKey）為 DTO；
// images 由 Gateway 以 mediaBaseURL 組顯示 URL，imageKeys 保留裸 key（49 §6.1）。
func toReviewListItem(base string, r *pb.ReviewInfo) ReviewListItem {
	keys := r.ImageKeys
	if keys == nil {
		keys = []string{}
	}
	images := buildURLs(base, r.ImageKeys)
	if images == nil {
		images = []string{}
	}
	return ReviewListItem{
		ReviewID:      r.Id,
		BuyerID:       r.BuyerId,
		ProductID:     r.ProductId,
		Username:      maskBuyerDisplayName(r.UserName, r.UserDeleted),
		UserAvatarKey: r.UserAvatarKey,
		UserAvatar:    buildURL(base, r.UserAvatarKey),
		Rating:        strconv.Itoa(int(r.Rating)),
		Content:       r.Content,
		ImageKeys:     keys,
		Images:        images,
		CreatedAt:     r.CreatedAt,
		Reply:         toReviewReply(r.GetReply()),
	}
}

// ReviewDetail 單一評論詳情（/app/reviews/{id}）；扁平成對 key/url（49 §6.6 review 表＋S2）。
// 對外 JSON camelCase（API 規範 §2）、數字欄 string（§4），與 ReviewListItem 同慣例；
// avatarKey/imageKeys 為裸 objectKey，avatarUrl/images 為 Gateway 以 mediaBaseURL 組出的顯示 URL。
type ReviewDetail struct {
	ReviewID      string       `json:"reviewId"`
	ProductID     string       `json:"productId"`
	BuyerID       string       `json:"buyerId"`
	Username      string       `json:"username"`
	UserAvatarKey string       `json:"avatarKey"`
	UserAvatar    string       `json:"avatarUrl"`
	OrderID       string       `json:"orderId"`
	Rating        string       `json:"rating"`
	Content       string       `json:"content"`
	ImageKeys     []string     `json:"imageKeys"`
	Images        []string     `json:"imageUrls"`
	IsAnonymous   bool         `json:"isAnonymous"`
	LikeCount     string       `json:"likeCount"`
	ReplyCount    string       `json:"replyCount"`
	CreatedAt     string       `json:"createdAt"`
	Reply         *ReviewReply `json:"reply,omitempty"`
}

// toReviewDetail 轉換 proto ReviewInfo（image_keys/user_avatar_key 值為 objectKey）為成對 DTO；
// key 欄保留裸 key，對應 url 欄由 Gateway 以 mediaBaseURL 組（49 §6.6 review／S2）。
func toReviewDetail(base string, r *pb.ReviewInfo) ReviewDetail {
	keys := r.ImageKeys
	if keys == nil {
		keys = []string{}
	}
	images := buildURLs(base, r.ImageKeys)
	if images == nil {
		images = []string{}
	}
	return ReviewDetail{
		ReviewID:      r.Id,
		ProductID:     r.ProductId,
		BuyerID:       r.BuyerId,
		Username:      r.UserName,
		UserAvatarKey: r.UserAvatarKey,
		UserAvatar:    buildURL(base, r.UserAvatarKey),
		OrderID:       r.OrderId,
		Rating:        strconv.Itoa(int(r.Rating)),
		Content:       r.Content,
		ImageKeys:     keys,
		Images:        images,
		IsAnonymous:   r.IsAnonymous,
		LikeCount:     strconv.Itoa(int(r.LikeCount)),
		ReplyCount:    strconv.Itoa(int(r.ReplyCount)),
		CreatedAt:     r.CreatedAt,
		Reply:         toReviewReply(r.GetReply()),
	}
}

// maskUsername 遮罩買家暱稱：每個字段保留首字，其餘以 ** 取代
func maskUsername(name string) string {
	if name == "" {
		return ""
	}
	parts := strings.Fields(name)
	for i, p := range parts {
		runes := []rune(p)
		if len(runes) <= 1 {
			continue
		}
		keep := 1
		if len(runes) > 3 {
			keep = 2
		}
		parts[i] = string(runes[:keep]) + strings.Repeat("*", len(runes)-keep)
	}
	return strings.Join(parts, " ")
}

// ProductReviewsPageResponse 是商品評論列表的 data 內容：標準分頁四欄
// （records/total/page/pageSize，同 response.PageData）＋本端點特有的星級統計區塊。
// 全部收在 data 之下，不掛 envelope 頂層（10 檔 §2）。
type ProductReviewsPageResponse struct {
	Records  []ReviewListItem  `json:"records"`
	Stats    *ReviewStatsBlock `json:"stats"`
	Total    string            `json:"total"`
	Page     string            `json:"page"`
	PageSize string            `json:"pageSize"`
}

// ReviewStatsBlock 是星級統計區塊的對外 DTO。
//
// 這裡必須有一層 DTO，不能把 pb.ReviewStats 直接丟進 data：protoc-gen-go 產的
// tag 是 `json:"total_count,omitempty"`，直出會同時吃到兩個問題——欄名變 snake
// （違反 10 檔 §2）、以及 omitempty 把 0 整個欄位吞掉。零評論商品的
// totalCount / averageRating / 各星級筆數會全部從 wire 上消失，前端讀到
// undefined 而不是 0（與 #2220 hasCoupon 同一個形狀）。
type ReviewStatsBlock struct {
	TotalCount      int64   `json:"totalCount"`
	AverageRating   float64 `json:"averageRating"`
	Rating5Count    int64   `json:"rating5Count"`
	Rating4Count    int64   `json:"rating4Count"`
	Rating3Count    int64   `json:"rating3Count"`
	Rating2Count    int64   `json:"rating2Count"`
	Rating1Count    int64   `json:"rating1Count"`
	WithImagesCount int64   `json:"withImagesCount"`
}

// toReviewStatsBlock 把 proto 統計轉成對外 DTO；nil 進 nil 出（stats 為 null）。
func toReviewStatsBlock(s *pb.ReviewStats) *ReviewStatsBlock {
	if s == nil {
		return nil
	}
	return &ReviewStatsBlock{
		TotalCount:      s.TotalCount,
		AverageRating:   s.AverageRating,
		Rating5Count:    s.Rating_5Count,
		Rating4Count:    s.Rating_4Count,
		Rating3Count:    s.Rating_3Count,
		Rating2Count:    s.Rating_2Count,
		Rating1Count:    s.Rating_1Count,
		WithImagesCount: s.WithImagesCount,
	}
}

// @Summary      取得商品評論列表
// @Description  取得指定商品的評論列表與統計資訊，無需登入
// @Tags         評論 (Review)
// @Produce      json
// @Param        id          path   int     true   "商品 ID"
// @Param        page        query  int     false  "頁碼（預設 1）"
// @Param        size        query  int     false  "每頁筆數（預設 20）"
// @Param        rating      query  int     false  "星級篩選（1–5，0 表示不篩選）"
// @Param        with_images query  bool    false  "只回傳有圖片的評論"
// @Param        sort_by     query  string  false  "排序方式（latest / highest / lowest）"
// @Success      200  {object}  object{code=int,message=string,data=object{records=[]handler.ReviewListItem,stats=handler.ReviewStatsBlock,total=string,page=string,pageSize=string}}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router /public/products/{id}/reviews [get]
func (h *Holder) GetProductReviews(c *gin.Context) {
	productID := c.Param("id")
	if productID == "" {
		response.BadRequest(c, "invalid product id")
		return
	}

	page := httputil.ParsePageParam(c.Query("page"), 1)
	pageSize := httputil.ParsePageParam(c.Query("size"), 10)
	rating, _ := strconv.Atoi(c.DefaultQuery("rating", "0")) // rating=0 是「不篩選」語意，非分頁 default fallback，故不用 ParsePageParam
	withImages := c.Query("with_images") == "true"
	sortBy := c.DefaultQuery("sort_by", "latest")

	// B-135 block filter：viewer 的 CURRENT buyer_id，跟 seller_forum.go
	// GetSellerForum 同款 resolveOptionalBuyerID pattern。D11：只過濾 list，
	// 不影響 stats 聚合（review-service handler.go 只在 ViewerId!=0 時才過濾列表）。
	loginRoleRaw, _ := c.Get("loginRole")
	loginRole, _ := loginRoleRaw.(int)
	viewerID, err := h.resolveOptionalBuyerID(c.Request.Context(), getOptionalAccountID(c), loginRole)
	if err != nil {
		response.GrpcErr(c, "buyer", err)
		return
	}

	resp, err := h.Review.GetProductReviews(c.Request.Context(), productID, viewerID, int32(rating), withImages, sortBy, int32(page), int32(pageSize))
	if err != nil {
		pkglogger.Info(c.Request.Context(), "GetProductReviews failed", zap.Error(err))
		response.ServiceUnavailable(c, "review")
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	items := make([]ReviewListItem, 0, len(resp.Reviews))
	for _, r := range resp.Reviews {
		items = append(items, toReviewListItem(h.MediaBaseURL, r))
	}

	response.OK(c, ProductReviewsPageResponse{
		Records:  items,
		Stats:    toReviewStatsBlock(resp.Stats),
		Total:    strconv.FormatInt(resp.Total, 10),
		Page:     strconv.Itoa(int(resp.Page)),
		PageSize: strconv.Itoa(int(resp.PageSize)),
	})
}

// @Summary      取得單一評論詳情
// @Description  依評論 ID 查詢評論完整內容
// @Tags         評論 (Review)
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  int  true  "評論 ID"
// @Success      200  {object}  object{code=int,message=string,data=handler.ReviewDetail}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router /app/reviews/{id} [get]
func (h *Holder) GetReview(c *gin.Context) {
	reviewID := c.Param("id")
	if reviewID == "" {
		response.BadRequest(c, "invalid review id")
		return
	}

	resp, err := h.Review.GetReview(c.Request.Context(), reviewID)
	if err != nil {
		pkglogger.Info(c.Request.Context(), "GetReview failed", zap.Error(err))
		response.ServiceUnavailable(c, "review")
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}
	// proto image_keys/user_avatar_key 值為 objectKey；改為成對 DTO 輸出（欄名嚴格：
	// key 欄放裸 key、對應 url 欄由 Gateway 組），不再把 URL 值塞回 key 欄（49 §6.6 review／S2）。
	if resp.Review == nil {
		response.OK(c, nil)
		return
	}
	response.OK(c, toReviewDetail(h.MediaBaseURL, resp.Review))
}
