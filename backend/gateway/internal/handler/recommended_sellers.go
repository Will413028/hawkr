package handler

import (
	"github.com/Will413028/hawkr/gateway/internal/response"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GetRecommendedSellers 取得推薦商家列表
// @Summary      取得推薦商家列表
// @Description  依近 30 天營收排名回傳推薦商家，不暴露營收金額，僅回傳排序後的商家資訊
// @Tags         商家 (Seller)
// @Accept       json
// @Produce      json
// @Param        limit  query  int  false  "回傳筆數（預設 10，上限 50）"  default(10)
// @Success      200  {object}  object{code=int,data=object{records=[]object{storeId=string,storeCode=string,storeName=string,logoKey=string,logoUrl=string,coverKey=string,coverUrl=string,description=string,rating=number,city=string,district=string,followers=int,certified=bool}}}
// @Failure      500  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /public/sellers/recommended [get]
func (h *Holder) GetRecommendedSellers(c *gin.Context) {
	limit := parseIntQuery(c, "limit", 10)
	if limit > maxPageSize {
		limit = maxPageSize
	}

	resp, err := h.Seller.GetRecommendedSellers(c.Request.Context(), int32(limit))
	if err != nil {
		pkglogger.Warn(c.Request.Context(), "GetRecommendedSellers gRPC failed", zap.Error(err))
		response.GrpcErr(c, "seller", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	records := make([]gin.H, 0, len(resp.Sellers))
	for _, s := range resp.Sellers {
		records = append(records, gin.H{
			"storeId":     s.StoreId,
			"storeCode":   s.StoreCode, // 賣家頁 canonical 連結用；缺席時前端會 fallback 回 store_id
			"storeName":   s.Name,
			"logoKey":     s.LogoKey,                            // 商家 logo objectKey（SSOT；49 §6.1）
			"logoUrl":     buildURL(h.MediaBaseURL, s.LogoKey),  // 顯示 URL（Gateway 組）
			"coverKey":    s.CoverKey,                           // 商家封面 objectKey（SSOT；49 §6.1）
			"coverUrl":    buildURL(h.MediaBaseURL, s.CoverKey), // 顯示 URL（Gateway 組）
			"description": s.Description,
			"rating":      s.Rating,
			"city":        s.City,
			"district":    s.District,
			"followers":   s.Followers,
			"certified":   true, // service 已過濾只回傳 approved 賣家
		})
	}

	response.OK(c, gin.H{"records": records})
}
