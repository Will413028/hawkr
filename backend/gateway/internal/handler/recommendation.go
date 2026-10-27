package handler

import (
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/go-pkg/identity"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/mediaurl"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RecommendationsResponse 商品推薦 response（typed DTO；取代既有 gin.H 動態組裝）。
//
// 設計備註（39 §5.5b.5）：
//   - JSON tags 保留 snake_case，避免破壞既有對外契約；docs/開發規範/API 設計規範.md
//     規範 public namespace 應 camelCase，此處為已知技術債，待獨立 contract 變更議題對齊。
//   - PersonalizedRecommend RPC 持續以 accountID=0（匿名模式）呼叫，本次不引入 AI
//     personalization 議題。但 isRestockSubscribed 改從 JWT context 取出 buyer accountID，
//     在 Gateway 層獨立做 aggregation。
type RecommendationsResponse struct {
	ModelVersion string                  `json:"modelVersion"`
	Sections     []RecommendationSection `json:"sections"`
}

// RecommendationSection 推薦區塊（同分類 / 同主題的商品集合）
type RecommendationSection struct {
	Title    string                  `json:"title"`
	Type     string                  `json:"type"`
	Products []RecommendationProduct `json:"products"`
}

// RecommendationProduct 推薦區塊內單一商品 item
type RecommendationProduct struct {
	ProductID string `json:"productId"`
	Name      string `json:"name"`
	// imageKey = 商品圖 objectKey（proto 值）；imageUrl = Gateway 組的顯示 URL（49 §6.1）。
	ImageKey   string  `json:"imageKey"`
	ImageURL   string  `json:"imageUrl"`
	PriceMin   int64   `json:"priceMin"`
	PriceMax   int64   `json:"priceMax"`
	Rating     float64 `json:"rating"`
	SellerName string  `json:"sellerName"`
	// logoKey = 賣家 logo objectKey（S9 cascade）；logoUrl = Gateway 組的顯示 URL（49 §6.1）。
	SellerLogoKey string  `json:"logoKey"`
	SellerLogo    string  `json:"logoUrl"`
	Score         float64 `json:"score"`
	Reason        string  `json:"reason"`
	// 當前登入買家對此商品是否有 ACTIVE 補貨通知訂閱（issue #1005 / B-203，39 §5.5b）
	IsRestockSubscribed bool `json:"isRestockSubscribed"`
}

// GetRecommendations returns product recommendations (public, no auth required).
// @Summary      取得商品推薦
// @Description  取得商品推薦列表（公開 API，不需登入；isRestockSubscribed 僅在登入時聚合）
// @Tags         推薦 (Recommendation)
// @Produce      json
// @Success      200  {object}  object{code=int,data=RecommendationsResponse}
// @Failure      500  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router /public/recommendations [get]
func (h *Holder) GetRecommendations(c *gin.Context) {
	resp, err := h.AI.PersonalizedRecommend(c.Request.Context(), 0)
	if err != nil {
		response.GrpcErr(c, "ai", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	data := RecommendationsResponse{
		ModelVersion: resp.Data.GetModelVersion(),
		Sections:     make([]RecommendationSection, 0, len(resp.Data.GetSections())),
	}

	for _, sec := range resp.Data.GetSections() {
		section := RecommendationSection{
			Title:    sec.GetTitle(),
			Type:     sec.GetType(),
			Products: make([]RecommendationProduct, 0, len(sec.GetProducts())),
		}
		for _, p := range sec.GetProducts() {
			section.Products = append(section.Products, RecommendationProduct{
				ProductID:     p.GetId(),
				Name:          p.GetName(),
				ImageKey:      p.GetImageKey(),
				ImageURL:      mediaurl.BuildURL(h.MediaBaseURL, p.GetImageKey()),
				PriceMin:      p.GetPriceMin(),
				PriceMax:      p.GetPriceMax(),
				Rating:        p.GetRating(),
				SellerName:    p.GetSellerName(),
				SellerLogoKey: p.GetSellerLogoKey(),
				SellerLogo:    mediaurl.BuildURL(h.MediaBaseURL, p.GetSellerLogoKey()),
				Score:         p.GetScore(),
				Reason:        p.GetReason(),
			})
		}
		data.Sections = append(data.Sections, section)
	}

	// isRestockSubscribed aggregation across all section products
	// (issue #1005 / B-203, 39 §5.5b)
	accountID := getOptionalAccountID(c)
	if accountID != 0 {
		// 收集所有 section product IDs + 建立 (section, product) 索引
		type loc struct {
			s, p int
		}
		idx := make(map[string][]loc)
		ids := make([]string, 0)
		for si := range data.Sections {
			for pi, prod := range data.Sections[si].Products {
				pidStr := prod.ProductID
				if _, ok := idx[pidStr]; !ok {
					ids = append(ids, pidStr)
				}
				idx[pidStr] = append(idx[pidStr], loc{s: si, p: pi})
			}
		}
		// buyer_id-first：這裡沒有其他地方已解析過 buyerID，自己做一次 soft-fail
		// 解析（同 product.go GetProductList 的理由：ErrNoCurrentBuyer 等同無
		// stint，略過聚合、不阻斷推薦回應）。
		if buyerID, berr := h.BuyerID.CurrentBuyerID(c.Request.Context(), identity.AccountID(accountID)); berr != nil {
			pkglogger.Debug(c.Request.Context(), "Failed to resolve buyer id for restock subscription aggregation", zap.Error(berr))
		} else {
			h.applyRestockSubscribed(c.Request.Context(), int64(buyerID), ids, func(pid string, subscribed bool) {
				for _, l := range idx[pid] {
					data.Sections[l.s].Products[l.p].IsRestockSubscribed = subscribed
				}
			})
		}
	}

	response.OK(c, data)
}
