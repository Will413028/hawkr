package handler

import (
	"github.com/Will413028/hawkr/gateway/internal/response"
	pb "github.com/Will413028/hawkr/go-pkg/proto/pricebook"
	"github.com/gin-gonic/gin"
)

// CreateMarketWatchTargetRequest is the seller HTTP request body.
type CreateMarketWatchTargetRequest struct {
	CropCode   string  `json:"cropCode"`
	MarketCode *string `json:"marketCode,omitempty"`
}

// GetMarketOverview returns an account-scoped official-price snapshot.
// @Summary 取得市場行情雷達
// @Description Seller CAP_MARKETING；省略數字 query 使用預設，非法值回 validation error；回應 private, no-store。
// @Tags 賣家市場行情 (Seller Pricebook)
// @Produce json
// @Security BearerAuth
// @Param scope query string false "all 或 watchlist，預設 all"
// @Param window_days query string false "7、14 或 30，預設 7"
// @Param page query string false "正整數，預設 1"
// @Param page_size query string false "1 至 100，預設 20"
// @Param query query string false "品項名稱或代碼"
// @Param market_code query string false "限 scope=all；省略代表全部市場"
// @Success 200 {object} object{code=int,message=string,data=pb.GetMarketOverviewResponse}
// @Failure 400 {object} object{code=int,message=string,traceId=string}
// @Failure 401 {object} object{code=int,message=string,traceId=string}
// @Failure 403 {object} object{code=int,message=string,traceId=string}
// @Router /app/sellers/pricebook/overview [get]
func (h *Holder) GetMarketOverview(c *gin.Context) {
	if _, ok := getAccountID(c); !ok {
		response.Unauthorized(c)
		return
	}
	values := c.Request.URL.Query()
	for _, key := range []string{"scope", "window_days", "page", "page_size", "query", "market_code"} {
		if len(values[key]) > 1 {
			response.BadRequest(c, "query parameter must appear once: "+key)
			return
		}
	}
	optional := func(key string) *string {
		v, present := values[key]
		if !present {
			return nil
		}
		value := ""
		if len(v) > 0 {
			value = v[0]
		}
		return &value
	}
	req := &pb.GetMarketOverviewRequest{Scope: values.Get("scope"), WindowDays: optional("window_days"), Page: optional("page"), PageSize: optional("page_size"), Query: values.Get("query"), MarketCode: optional("market_code")}
	result, err := h.Pricebook.GetMarketOverview(c.Request.Context(), req)
	if err != nil {
		response.GrpcErr(c, "pricebook", err)
		return
	}
	response.OKProto(c, result)
}

// CreateMarketWatchTarget creates or returns the exact saved target.
// @Summary 新增市場行情追蹤
// @Description Seller CAP_MARKETING；省略市場代表全市場；帳號最多 20 筆，重複新增回原 ID。
// @Tags 賣家市場行情 (Seller Pricebook)
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateMarketWatchTargetRequest true "追蹤品項與可選市場"
// @Success 200 {object} object{code=int,message=string,data=pb.CreateMarketWatchTargetResponse}
// @Failure 400 {object} object{code=int,message=string,traceId=string}
// @Failure 401 {object} object{code=int,message=string,traceId=string}
// @Failure 403 {object} object{code=int,message=string,traceId=string}
// @Failure 429 {object} object{code=int,message=string,traceId=string}
// @Router /app/sellers/pricebook/watch-targets [post]
func (h *Holder) CreateMarketWatchTarget(c *gin.Context) {
	if _, ok := getAccountID(c); !ok {
		response.Unauthorized(c)
		return
	}
	var body CreateMarketWatchTargetRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "invalid watch target body")
		return
	}
	result, err := h.Pricebook.CreateMarketWatchTarget(c.Request.Context(), &pb.CreateMarketWatchTargetRequest{CropCode: body.CropCode, MarketCode: body.MarketCode})
	if err != nil {
		response.GrpcErr(c, "pricebook", err)
		return
	}
	response.OKProto(c, result)
}

// DeleteMarketWatchTarget removes only this account's target, idempotently.
// @Summary 刪除市場行情追蹤
// @Description Seller CAP_MARKETING；硬刪除自己的追蹤。不存在或屬於他人皆回相同成功結果。
// @Tags 賣家市場行情 (Seller Pricebook)
// @Produce json
// @Security BearerAuth
// @Param watch_id path string true "追蹤 ID"
// @Success 200 {object} object{code=int,message=string,data=pb.DeleteMarketWatchTargetResponse}
// @Failure 400 {object} object{code=int,message=string,traceId=string}
// @Failure 401 {object} object{code=int,message=string,traceId=string}
// @Failure 403 {object} object{code=int,message=string,traceId=string}
// @Router /app/sellers/pricebook/watch-targets/{watch_id} [delete]
func (h *Holder) DeleteMarketWatchTarget(c *gin.Context) {
	if _, ok := getAccountID(c); !ok {
		response.Unauthorized(c)
		return
	}
	result, err := h.Pricebook.DeleteMarketWatchTarget(c.Request.Context(), &pb.DeleteMarketWatchTargetRequest{WatchId: c.Param("watch_id")})
	if err != nil {
		response.GrpcErr(c, "pricebook", err)
		return
	}
	response.OKProto(c, result)
}
