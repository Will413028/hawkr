package handler

import (
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/gin-gonic/gin"
)

type SellerCitySummaryItem struct {
	CityCode     string `json:"cityCode"`
	CityName     string `json:"cityName"`
	ProductCount string `json:"productCount"`
	// SellerCount 是「至少有一件公開上架商品」的商家數，不是該縣市的核准商家數。
	SellerCount string `json:"sellerCount"`
	// PreviewStoreIDs 取自同一批商家（最多 6 筆），供畫面預覽用。
	PreviewStoreIDs []string `json:"previewStoreIds"`
}

type SellerCitiesSummaryResponse struct {
	Records []SellerCitySummaryItem `json:"records"`
}

// GetSellerCitiesSummary returns complete seller-registration-city counts.
// @Summary 商家所在地完整聚合
// @Description 固定回傳台灣 22 縣市（含 0 家商家的縣市）。productCount 為該縣市商家的公開上架商品總數；sellerCount 為該縣市「至少有一件公開上架商品」的商家數，0 件在架的商家不計入，故可能小於該縣市的核准商家數。previewStoreIds 取自同一批有在架商品的商家（最多 6 筆，deterministic），僅供畫面預覽，不得作完整篩選。productCount/sellerCount 均不受商品分頁或 preview 6 家限制；商品產地請使用 /public/origins/summary。
// @Tags 商品 (Product)
// @Produce json
// @Success 200 {object} response.Envelope{data=SellerCitiesSummaryResponse}
// @Failure 503 {object} response.Envelope
// @Router /public/seller-cities/summary [get]
func (h *Holder) GetSellerCitiesSummary(c *gin.Context) {
	result, err := h.Product.GetPublicSellerCitiesSummary(c.Request.Context())
	if err != nil {
		response.GrpcErr(c, "product", err)
		return
	}
	records := make([]SellerCitySummaryItem, 0, len(result.Records))
	for _, row := range result.Records {
		records = append(records, SellerCitySummaryItem{
			CityCode: row.CityCode, CityName: row.CityName,
			ProductCount: row.ProductCount, SellerCount: row.SellerCount,
			PreviewStoreIDs: row.PreviewStoreIds,
		})
	}
	c.Header("Cache-Control", "public, max-age=300, stale-while-revalidate=3600")
	response.OK(c, SellerCitiesSummaryResponse{Records: records})
}
