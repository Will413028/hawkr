package handler

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/go-pkg/identity"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/mediaurl"
	"github.com/Will413028/hawkr/go-pkg/syncutil"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GetMarketplace 取得 Marketplace 聚合資料
// @Summary      取得 Marketplace 聚合資料
// @Description  一次回傳推薦商家、最新貼文、熱門商品三個區塊，取代前端多次 API 呼叫
// @Tags         Marketplace
// @Accept       json
// @Produce      json
// @Param        seller_limit   query  int  false  "推薦商家筆數（預設 10，上限 50）"   default(10)
// @Param        post_limit     query  int  false  "最新貼文筆數（預設 10，上限 50）"   default(10)
// @Param        product_limit  query  int  false  "熱門商品筆數（預設 10，上限 50）"   default(10)
// @Success      200  {object}  object{code=int,data=object{recommendedSellers=[]object{storeId=string,storeCode=string,storeName=string,logoKey=string,logoUrl=string,coverKey=string,coverUrl=string,description=string,rating=number,city=string,district=string},latestPosts=[]object{id=string,content=string,imageKeys=[]string,imageUrls=[]string,sellerId=string,storeCode=string,sellerName=string,logoKey=string,logoUrl=string,product=object{id=string,name=string,imageKey=string,imageUrl=string,price=string},likeCount=int,commentCount=int,viewCount=int,shareCount=int,isLiked=bool,createdAt=int},hotProducts=[]ProductItem}}
// @Router       /public/marketplace [get]
func (h *Holder) GetMarketplace(c *gin.Context) {
	sellerLimit := parseIntQuery(c, "seller_limit", 10)
	if sellerLimit > 50 {
		sellerLimit = 50
	}
	postLimit := parseIntQuery(c, "post_limit", 10)
	if postLimit > 50 {
		postLimit = 50
	}
	productLimit := parseIntQuery(c, "product_limit", 10)
	if productLimit > 50 {
		productLimit = 50
	}

	var (
		sellers  []gin.H
		posts    []PostListItemData
		products []ProductItem
	)

	// syncutil.Group：panic 會被 recover 並記錄，不會讓整個 gateway process crash
	// （不用 WithContext — 三個區塊各自 best-effort，互不 cancel）。
	g := syncutil.NewGroup(c.Request.Context())

	// 1. 推薦商家
	g.Go(func() error {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		resp, err := h.Seller.GetRecommendedSellers(ctx, int32(sellerLimit))
		if err != nil {
			pkglogger.Warn(c.Request.Context(), "marketplace: GetRecommendedSellers failed", zap.Error(err))
			return nil
		}
		if resp.Code != 0 {
			pkglogger.Warn(c.Request.Context(), "marketplace: GetRecommendedSellers non-zero code", zap.Int32("code", resp.Code), zap.String("msg", resp.Message))
			return nil
		}
		result := make([]gin.H, 0, len(resp.Sellers))
		for _, s := range resp.Sellers {
			result = append(result, gin.H{
				"storeId":     s.StoreId,
				"storeCode":   s.StoreCode,
				"storeName":   s.Name,
				"logoKey":     s.LogoKey,                            // 商家 logo objectKey（SSOT；49 §6.1）
				"logoUrl":     buildURL(h.MediaBaseURL, s.LogoKey),  // 顯示 URL（Gateway 組）
				"coverKey":    s.CoverKey,                           // 商家封面 objectKey（SSOT；49 §6.1）
				"coverUrl":    buildURL(h.MediaBaseURL, s.CoverKey), // 顯示 URL（Gateway 組）
				"description": s.Description,
				"rating":      s.Rating,
				"city":        s.City,
				"district":    s.District,
			})
		}
		sellers = result
		return nil
	})

	// 2. 最新貼文（帶 userID 以取得 is_liked 狀態）
	// ② buyer store-stint re-key：is_liked 個人化要用 CURRENT buyer_id；c.Get 在
	// goroutine 外先同步讀完（沿用既有寫法慣例），goroutine 內只做網路呼叫。
	var accountID int64
	if id, ok := c.Get("accountId"); ok {
		accountID, _ = id.(int64)
	}
	g.Go(func() error {
		buyerID, err := h.currentBuyerIDOrZero(c.Request.Context(), accountID)
		if err != nil {
			pkglogger.Warn(c.Request.Context(), "marketplace: buyerid resolve failed", zap.Error(err))
			buyerID = 0
		}
		resp, err := h.Social.GetPostList(c.Request.Context(), buyerID, "", 1, int32(postLimit), "")
		if err != nil {
			pkglogger.Warn(c.Request.Context(), "marketplace: GetPostList failed", zap.Error(err))
			return nil
		}
		posts = toPostListItems(h.MediaBaseURL, resp.Data)
		return nil
	})

	// 3. 熱門商品
	g.Go(func() error {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		resp, err := h.Product.GetPublicProductList(ctx, "0", "", "1", fmt.Sprintf("%d", productLimit), "sales_desc")
		if err != nil {
			pkglogger.Warn(c.Request.Context(), "marketplace: GetPublicProductList failed", zap.Error(err))
			return nil
		}
		if resp.Code != 0 {
			pkglogger.Warn(c.Request.Context(), "marketplace: GetPublicProductList non-zero code", zap.Int32("code", resp.Code), zap.String("msg", resp.Message))
			return nil
		}
		base := h.MediaBaseURL
		result := make([]ProductItem, 0, len(resp.Products))
		for _, p := range resp.Products {
			item := ProductItem{
				ID:           p.Id,
				Name:         p.Name,
				ImageKey:     p.ImageKey,
				ImageURL:     mediaurl.BuildURL(base, p.ImageKey),
				ImageKeys:    p.ImageKeys,
				ImageUrls:    buildMediaURLs(base, p.ImageKeys),
				PriceMin:     p.PriceMin,
				PriceMax:     p.PriceMax,
				ListPriceMin: p.ListPriceMin,
				ListPriceMax: p.ListPriceMax,
				SalePriceMin: p.SalePriceMin,
				SalePriceMax: p.SalePriceMax,
				Rating:       p.Rating,
				SkuNames:     p.SkuNames,
				Hashtags:     toHashtagItems(p.Hashtags),
				// StoreID 故意不在這裡設：B-139 guard 透過下方 Seller!=nil
				// 分支控制，避免 orphan product 讓前端打 /public/sellers/{id} 拿 404。
			}
			// storeId 只在 Java 端成功補到 seller meta 時才回填，避免 orphan
			// product（created_by 對應的 seller 不存在）讓前端打 /public/sellers/{id} 拿 404
			if p.Seller != nil {
				item.StoreID = p.Seller.StoreId
				item.Seller = buildSellerInfoItem(base, p.Seller)
				item.SellerName, item.SellerLogoKey, item.SellerLogoUrl = flatSellerFields(item.Seller)
			}
			result = append(result, item)
		}
		products = result
		return nil
	})

	_ = g.Wait()

	// 用 seller 模組補上貼文缺少的賣家名稱/頭貼
	h.enrichPostsWithSellerInfo(c.Request.Context(), posts)

	// 確保空陣列而非 null
	if sellers == nil {
		sellers = []gin.H{}
	}
	if products == nil {
		products = []ProductItem{}
	}

	// isRestockSubscribed aggregation (issue #1005 / B-203, 39 §5.5b)
	// buyer_id-first：這裡的 re-key 先前刻意延後（見 git blame），現在補上——沒有
	// 其他地方已解析過 buyerID，自己做一次 soft-fail 解析（同 product.go
	// GetProductList 的理由：ErrNoCurrentBuyer 等同無 stint，略過聚合、不阻斷
	// marketplace 回應）。
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

	latestPosts := make([]gin.H, 0, len(posts))
	for _, p := range posts {
		latestPosts = append(latestPosts, gin.H{
			"id":           p.ID,
			"imageKeys":    p.ImageKeys,
			"imageUrls":    p.Images,
			"content":      p.Content,
			"sellerId":     p.SellerID,
			"storeCode":    p.StoreCode, // canonical 店面定址碼，與 /public/posts 一致（#2098 階段三）
			"sellerName":   p.SellerName,
			"logoKey":      p.SellerLogoKey, // 49 §6.1：key 與 URL 成對輸出，與 /public/posts 一致
			"logoUrl":      p.SellerLogoUrl,
			"likeCount":    p.LikeCount,
			"commentCount": p.CommentCount,
			"viewCount":    p.ViewCount,
			"shareCount":   p.ShareCount,
			"isLiked":      p.IsLiked,
			"createdAt":    p.CreatedAt,
			"product":      p.Product,
		})
	}

	response.OK(c, gin.H{
		"recommendedSellers": sellers,
		"latestPosts":        latestPosts,
		"hotProducts":        products,
	})
}

// enrichPostsWithSellerInfo 用 seller 模組的店名/logo 覆蓋貼文的賣家資訊。
// social-service 從 identity 模組取 nickname，但賣家的顯示名稱應該用 seller 模組的 store_name。
func (h *Holder) enrichPostsWithSellerInfo(ctx context.Context, posts []PostListItemData) {
	seen := make(map[int64]struct{})
	var ids []int64
	for _, p := range posts {
		sellerID, err := strconv.ParseInt(p.SellerID, 10, 64)
		if err != nil || sellerID == 0 {
			continue
		}
		if _, ok := seen[sellerID]; !ok {
			seen[sellerID] = struct{}{}
			ids = append(ids, sellerID)
		}
	}
	if len(ids) == 0 {
		return
	}

	batchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	resp, err := h.Seller.GetSellerBatch(batchCtx, ids)
	if err != nil || resp == nil || resp.GetCode() != 0 {
		return
	}

	type sellerInfoView struct {
		name      string
		logoKey   string
		storeCode string
	}
	sellerMap := make(map[int64]sellerInfoView, len(resp.Data))
	for _, s := range resp.Data {
		storeID, parseErr := strconv.ParseInt(s.GetStoreId(), 10, 64)
		if parseErr != nil || storeID <= 0 {
			continue
		}
		sellerMap[storeID] = sellerInfoView{name: s.Name, logoKey: s.LogoKey, storeCode: s.StoreCode}
	}

	for i := range posts {
		sellerID, err := strconv.ParseInt(posts[i].SellerID, 10, 64)
		if err != nil {
			continue
		}
		if info, ok := sellerMap[sellerID]; ok {
			if info.name != "" {
				posts[i].SellerName = info.name
			}
			// 49 §6.1 GW組：seller 回 objectKey，logoKey 與 logoUrl 成對回填，
			// 避免快照 key 空時 URL 有值、key 空的不對稱。
			if info.logoKey != "" {
				posts[i].SellerLogoKey = info.logoKey
				posts[i].SellerLogoUrl = buildURL(h.MediaBaseURL, info.logoKey)
			}
			// canonical 店面定址碼（#2098 階段三）。social-service 不存 store_code，
			// 這支 batch RPC 本來就為了店名/logo 呼叫，順手帶回來零額外成本。
			posts[i].StoreCode = info.storeCode
		}
	}
}
