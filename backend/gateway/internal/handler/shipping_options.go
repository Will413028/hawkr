package handler

import (
	"github.com/Will413028/hawkr/gateway/internal/response"
	pb "github.com/Will413028/hawkr/go-pkg/proto/product"
	"github.com/gin-gonic/gin"
)

// CalculateShippingOptionsRequest 運費計算請求（JSON camelCase）
type CalculateShippingOptionsRequest struct {
	Length       string `json:"length" binding:"required"`     // 長（cm）
	Width        string `json:"width" binding:"required"`      // 寬（cm）
	Height       string `json:"height" binding:"required"`     // 高（cm）
	Weight       string `json:"weight" binding:"required"`     // 重量值
	WeightUnit   string `json:"weightUnit" binding:"required"` // 重量單位：KG, CATTY, GRAM
	SenderArea   string `json:"senderArea,omitempty"`          // 寄件區域（選填，由 Product Service 決定預設值）
	ReceiverArea string `json:"receiverArea,omitempty"`        // 收件區域（選填，由 Product Service 決定預設值）
}

// AvailableTemperatureItem 可配送溫層
type AvailableTemperatureItem struct {
	Temperature string `json:"temperature"` // 溫層 Enum code
	DisplayName string `json:"displayName"` // 溫層顯示名
}

// ShippingLimitsItem 配送限制
type ShippingLimitsItem struct {
	MaxDimensionSum string `json:"maxDimensionSum"` // 材積上限 L+W+H (cm)
	MaxLongestSide  string `json:"maxLongestSide"`  // 最長邊上限 (cm)，"0" 表示無限制
	MaxWeightGrams  string `json:"maxWeightGrams"`  // 重量上限 (g)
}

// ShippingOptionItem 運送方式選項（method × temperature 組合）
type ShippingOptionItem struct {
	ShippingMethod string             `json:"shippingMethod"` // 配送方式 Enum code
	DisplayName    string             `json:"displayName"`    // 配送方式顯示名
	Temperature    string             `json:"temperature"`    // 溫層 Enum code
	Fee            string             `json:"fee"`            // 運費（TWD）
	EstimatedDays  string             `json:"estimatedDays"`  // 預計到貨天數（暫空）
	Limits         ShippingLimitsItem `json:"limits"`         // 尺寸/重量限制
}

// ShippingCalculationDataItem 運費計算結果
type ShippingCalculationDataItem struct {
	VolumeSize            string                     `json:"volumeSize"`
	WeightGrams           string                     `json:"weightGrams"`
	DimensionSum          string                     `json:"dimensionSum"`
	LongestSide           string                     `json:"longestSide"`
	AvailableTemperatures []AvailableTemperatureItem `json:"availableTemperatures"`
	ShippingOptions       []ShippingOptionItem       `json:"shippingOptions"`
}

// CalculateShippingOptions 計算可用配送選項與運費
// @Summary      計算可用配送選項與運費
// @Description  依據商品包裝尺寸與重量，計算所有可用的配送選項與對應運費
// @Tags         物流 (Shipping)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  CalculateShippingOptionsRequest  true  "運費計算請求"
// @Success      200  {object}  object{code=int,data=ShippingCalculationDataItem}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Router       /app/shipping_quotes [post]
func (h *Holder) CalculateShippingOptions(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}
	storeID, ok := h.resolveStoreIDFor(c, accountID)
	if !ok {
		return
	}

	var req CalculateShippingOptionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	// 區域預設值由 Product Service 決定，Gateway 僅透傳
	grpcReq := &pb.CalculateShippingOptionsRequest{
		StoreId:      int64(storeID),
		Length:       req.Length,
		Width:        req.Width,
		Height:       req.Height,
		Weight:       req.Weight,
		WeightUnit:   req.WeightUnit,
		SenderArea:   req.SenderArea,
		ReceiverArea: req.ReceiverArea,
	}

	resp, err := h.Product.CalculateShippingOptions(c.Request.Context(), grpcReq)
	if err != nil {
		response.GrpcErr(c, "product", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	data := resp.Data

	temps := make([]AvailableTemperatureItem, 0, len(data.AvailableTemperatures))
	for _, t := range data.AvailableTemperatures {
		temps = append(temps, AvailableTemperatureItem{
			Temperature: t.Temperature,
			DisplayName: t.DisplayName,
		})
	}

	options := make([]ShippingOptionItem, 0, len(data.ShippingOptions))
	for _, o := range data.ShippingOptions {
		item := ShippingOptionItem{
			ShippingMethod: o.ShippingMethod,
			DisplayName:    o.DisplayName,
			Temperature:    o.Temperature,
			Fee:            o.Fee,
			EstimatedDays:  o.EstimatedDays,
		}
		if o.Limits != nil {
			item.Limits = ShippingLimitsItem{
				MaxDimensionSum: o.Limits.MaxDimensionSum,
				MaxLongestSide:  o.Limits.MaxLongestSide,
				MaxWeightGrams:  o.Limits.MaxWeightGrams,
			}
		}
		options = append(options, item)
	}

	response.OK(c, ShippingCalculationDataItem{
		AvailableTemperatures: temps,
		ShippingOptions:       options,
		VolumeSize:            data.VolumeSize,
		WeightGrams:           data.WeightGrams,
		DimensionSum:          data.DimensionSum,
		LongestSide:           data.LongestSide,
	})
}
