package handler

import (
	"strconv"
	"strings"

	"github.com/Will413028/hawkr/gateway/internal/response"
	sellerpb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"github.com/gin-gonic/gin"
)

// AddressSuggestionItem 地址搜尋結果
type AddressSuggestionItem struct {
	FullAddress string  `json:"fullAddress"`
	County      string  `json:"county"`
	District    string  `json:"district"`
	Detail      string  `json:"detail"`
	Lat         float64 `json:"lat"`
	Lng         float64 `json:"lng"`
}

// AddressValidationResult 地址驗證結果
type AddressValidationResult struct {
	FullAddress string `json:"fullAddress,omitempty"`
	County      string `json:"county,omitempty"`
	District    string `json:"district,omitempty"`
	Detail      string `json:"detail,omitempty"`
	Lat         string `json:"lat,omitempty"`
	Lng         string `json:"lng,omitempty"`
	Valid       bool   `json:"valid"`
}

// AddressSuggestions 地址模糊搜尋
// @Summary      地址模糊搜尋
// @Description  輸入縣市、鄉鎮區、路名關鍵字，回傳賣家地址候選清單
// @Tags         地址搜尋 (Address Geocode)
// @Produce      json
// @Param        county   query  string  true   "縣市，例如 台北市"
// @Param        district query  string  true   "鄉鎮市區，例如 中正區"
// @Param        query    query  string  true   "路名+門牌關鍵字，例如 中華路一段25"
// @Param        limit    query  string  false  "最多回傳筆數，預設 8，上限 20"
// @Success      200  {object}  object{code=int,message=string,data=[]AddressSuggestionItem}
// @Failure      400  {object}  object{code=int,message=string,traceId=string}
// @Failure      503  {object}  object{code=int,message=string,traceId=string}
// @Security     BearerAuth
// @Router       /app/address/suggestions [get]
//
// 地址查詢由 seller 模組處理；這裡只做 REST→gRPC 轉發。
func (h *Holder) AddressSuggestions(c *gin.Context) {
	county := strings.TrimSpace(c.Query("county"))
	district := strings.TrimSpace(c.Query("district"))
	query := strings.TrimSpace(c.Query("query"))
	if county == "" || district == "" || query == "" {
		response.BadRequest(c, "county, district, query are required")
		return
	}

	limit := 8
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 20 {
		limit = 20
	}

	var items []AddressSuggestionItem
	switch c.GetInt("loginRole") {
	case 2:
		resp, err := h.Seller.SuggestAddress(c.Request.Context(), county, district, query)
		if err != nil {
			response.GrpcErr(c, "seller", err)
			return
		}
		items = toSellerSuggestionItems(resp.Data)
	default:
		response.BadRequest(c, "active role required")
		return
	}

	if len(items) > limit {
		items = items[:limit]
	}
	response.OK(c, items)
}

func toSellerSuggestionItems(data []*sellerpb.AddressSuggestionData) []AddressSuggestionItem {
	items := make([]AddressSuggestionItem, 0, len(data))
	for _, d := range data {
		items = append(items, AddressSuggestionItem{
			FullAddress: d.FullAddress, County: d.County, District: d.District, Detail: d.Detail,
		})
	}
	return items
}

// AddressValidate 地址嚴格驗證
// @Summary      地址嚴格驗證
// @Description  輸入完整地址，驗證賣家地址並回傳座標
// @Tags         地址搜尋 (Address Geocode)
// @Produce      json
// @Param        address  query  string  true  "完整地址，例如 臺北市中正區中華路一段1號"
// @Success      200  {object}  object{code=int,message=string,data=AddressValidationResult}
// @Failure      400  {object}  object{code=int,message=string,traceId=string}
// @Failure      503  {object}  object{code=int,message=string,traceId=string}
// @Security     BearerAuth
// @Router       /app/address/validate [get]
func (h *Holder) AddressValidate(c *gin.Context) {
	address := strings.TrimSpace(c.Query("address"))
	if address == "" {
		response.BadRequest(c, "address is required")
		return
	}

	var result AddressValidationResult
	switch c.GetInt("loginRole") {
	case 2:
		resp, err := h.Seller.ValidateAddress(c.Request.Context(), address)
		if err != nil {
			response.GrpcErr(c, "seller", err)
			return
		}
		result = toSellerValidationResult(resp.Data)
	default:
		response.BadRequest(c, "active role required")
		return
	}

	response.OK(c, result)
}

func toSellerValidationResult(d *sellerpb.AddressValidationData) AddressValidationResult {
	if d == nil {
		return AddressValidationResult{}
	}
	return AddressValidationResult{
		FullAddress: d.FullAddress, County: d.County, District: d.District, Detail: d.Detail,
		Lat: d.Lat, Lng: d.Lng, Valid: d.Valid,
	}
}
