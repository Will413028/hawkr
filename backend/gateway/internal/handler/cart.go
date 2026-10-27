package handler

import (
	"errors"
	"strconv"

	"github.com/Will413028/hawkr/gateway/internal/buyerid"
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/go-pkg/identity"
	"github.com/Will413028/hawkr/go-pkg/mediaurl"
	pb "github.com/Will413028/hawkr/go-pkg/proto/cart"
	"github.com/gin-gonic/gin"
)

// ==================== DTO ====================

// AddToCartRequest 加入購物車請求
type AddToCartRequest struct {
	GuestID  string `json:"guestId"`
	SkuID    string `json:"skuId" binding:"required"`
	Quantity string `json:"quantity" binding:"required"`
}

// SetItemQuantityRequest 設定購物車商品數量請求
type SetItemQuantityRequest struct {
	GuestID  string `json:"guestId"`
	Quantity string `json:"quantity" binding:"required"`
}

// MergeCartResponseData 合併購物車回應 data（§5.3）
type MergeCartResponseData struct {
	RemovedSelfOwnedCount string `json:"removedSelfOwnedCount"` // 合併過程中靜默移除的自家商品品項數（distinct SKU count，數字字串）
}

// MergeCartRequest 合併購物車請求
type MergeCartRequest struct {
	GuestID string `json:"guestId" binding:"required"`
}

// CartModifyResponse 購物車修改回應
type CartModifyResponse struct {
	TotalItems string `json:"totalItems"`
}

// CartListResponse 購物車列表回應
type CartListResponse struct {
	EstimatedTotalAmount string              `json:"estimatedTotalAmount"`
	TotalItems           string              `json:"totalItems"`
	Groups               []CartGroupResponse `json:"groups"`
}

// CartGroupResponse 購物車賣家分組
type CartGroupResponse struct {
	StoreId        string             `json:"storeId"`
	SellerName     string             `json:"sellerName"`
	LogoKey        string             `json:"logoKey"`        // 賣家 Logo objectKey（SSOT；49 §6.1）
	LogoUrl        string             `json:"logoUrl"`        // 賣家 Logo 顯示 URL（Gateway 組）
	SellerCity     string             `json:"sellerCity"`     // 賣家縣市（如「桃園市」）
	SellerDistrict string             `json:"sellerDistrict"` // 賣家區域（如「桃園區」）
	StoreCode      string             `json:"storeCode"`      // 賣家短碼
	Items          []CartItemResponse `json:"items"`
}

// CartItemResponse 購物車商品項目
type CartItemResponse struct {
	SkuId             string   `json:"skuId"`
	Quantity          string   `json:"quantity"`
	Name              string   `json:"name"`
	Price             string   `json:"price"`               // 加入時實際成交單價快照（特價優先）
	ListPrice         string   `json:"listPrice,omitempty"` // 加入時原價快照（給前端 strike-through 顯示用，等於 Price 時表示加入時無特價）
	Status            string   `json:"status"`
	ProductId         string   `json:"productId"`
	SpecName          string   `json:"specName"`
	ImageKey          string   `json:"imageKey"`                    // 商品主圖 objectKey（SSOT；49 §6.1）
	ImageUrl          string   `json:"imageUrl"`                    // 商品主圖顯示 URL（Gateway 組）
	StockQty          string   `json:"stockQty"`                    // 即時庫存數量
	DeliveryTemps     []string `json:"deliveryTemps"`               // 配送溫層（3201=常溫, 3202=冷藏, 3203=冷凍）
	Purchasable       bool     `json:"purchasable"`                 // 本筆 row 是否可購買（前端據此反灰 / disable checkbox）
	UnavailableReason string   `json:"unavailableReason,omitempty"` // 不可購買原因（CartItemUnavailableReason value：DELISTED / OUT_OF_STOCK / INSUFFICIENT_STOCK / LOOKUP_FAILED）
}

// ==================== 共用工具 ====================

// getOptionalAccountID 取得可選的 accountId（OptionalJWTAuth 場景）
// 回傳 0 表示訪客
func getOptionalAccountID(c *gin.Context) int64 {
	accountIDRaw, exists := c.Get("accountId")
	if !exists {
		return 0
	}
	accountID, ok := accountIDRaw.(int64)
	if !ok {
		return 0
	}
	return accountID
}

// resolveCartBuyerID 回傳購物車擁有者的現任 buyer_id。兩個非 buyer_id 的分支：
//   - accountID == 0（訪客，OptionalJWTAuth 未帶身份）→ 回 0，由呼叫端配 guestID 判斷
//   - ErrNoCurrentBuyer（帳號沒有現任買家 stint）→ grandfather 回原 accountID，
//     不 403（購物車是登入前就能用的功能，不該因為身份解析失敗整條斷掉）
func (h *Holder) resolveCartBuyerID(c *gin.Context, accountID int64) (int64, bool) {
	if accountID == 0 {
		return 0, true
	}
	resolved, err := h.BuyerID.CurrentBuyerID(c.Request.Context(), identity.AccountID(accountID))
	if err != nil {
		if errors.Is(err, buyerid.ErrNoCurrentBuyer) {
			return accountID, true
		}
		response.GrpcErr(c, "buyer", err)
		return 0, false
	}
	return int64(resolved), true
}

// ==================== 對外 API ====================

// AddToCart 加入購物車
// @Summary      加入購物車
// @Description  將指定 SKU 加入購物車，支援訪客（需提供 guestId）及登入會員
// @Tags         購物車 (Cart)
// @Accept       json
// @Produce      json
// @Param        request  body  AddToCartRequest  true  "加入購物車請求（訪客需提供 guestId）"
// @Success      200  {object}  object{code=int,data=CartModifyResponse}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /public/carts/items [post]
func (h *Holder) AddToCart(c *gin.Context) {
	accountID := getOptionalAccountID(c)
	buyerID, ok := h.resolveCartBuyerID(c, accountID)
	if !ok {
		return
	}

	var req struct {
		GuestID  string `json:"guestId"`
		SkuID    string `json:"skuId" binding:"required"`
		Quantity string `json:"quantity" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	// 訪客必須提供 guest_id
	if buyerID == 0 && req.GuestID == "" {
		response.BadRequest(c, "guest_id is required for guest user")
		return
	}

	grpcReq := &pb.AddItemRequest{
		GuestId:  req.GuestID,
		SkuId:    req.SkuID,
		Quantity: req.Quantity,
	}

	resp, err := h.Cart.AddItem(c.Request.Context(), buyerID, grpcReq)
	if err != nil {
		response.GrpcErr(c, "cart", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, gin.H{
		"totalItems": resp.TotalItems,
	})
}

// GetCartList 查詢購物車
// @Summary      查詢購物車內容
// @Description  取得購物車商品列表，依賣家分組顯示。支援訪客（需提供 guest_id）及登入會員
// @Tags         購物車 (Cart)
// @Accept       json
// @Produce      json
// @Param        guest_id  query  string  false  "訪客 ID（訪客必填）"
// @Success      200  {object}  object{code=int,data=CartListResponse}
// @Failure      400  {object}  object{code=int,message=string}
// @Router       /public/carts [get]
func (h *Holder) GetCartList(c *gin.Context) {
	accountID := getOptionalAccountID(c)
	buyerID, ok := h.resolveCartBuyerID(c, accountID)
	if !ok {
		return
	}
	guestID := c.Query("guest_id")

	// 訪客必須提供 guest_id
	if buyerID == 0 && guestID == "" {
		response.BadRequest(c, "guest_id is required for guest user")
		return
	}

	grpcReq := &pb.GetCartRequest{
		GuestId: guestID,
	}

	resp, err := h.Cart.GetCart(c.Request.Context(), buyerID, grpcReq)
	if err != nil {
		response.GrpcErr(c, "cart", err)
		return
	}

	// 轉換 gRPC 回應為前端友善格式（49 §6.6：Gateway 由 objectKey 組顯示 URL）
	base := h.MediaBaseURL
	groups := make([]CartGroupResponse, 0, len(resp.Groups))
	for _, g := range resp.Groups {
		items := make([]CartItemResponse, 0, len(g.Items))
		for _, item := range g.Items {
			items = append(items, CartItemResponse{
				SkuId:             item.SkuId,
				Quantity:          item.Quantity,
				Name:              item.Name,
				Price:             item.Price,
				ListPrice:         item.ListPrice,
				Status:            item.Status,
				ProductId:         item.ProductId,
				SpecName:          item.SpecName,
				ImageKey:          item.ImageKey,
				ImageUrl:          mediaurl.BuildURL(base, item.ImageKey),
				StockQty:          item.StockQty,
				DeliveryTemps:     item.DeliveryTemps,
				Purchasable:       item.Purchasable,
				UnavailableReason: item.UnavailableReason,
			})
		}
		groups = append(groups, CartGroupResponse{
			StoreId:        g.StoreId,
			SellerName:     g.SellerName,
			LogoKey:        g.LogoKey,
			LogoUrl:        mediaurl.BuildURL(base, g.LogoKey),
			SellerCity:     g.SellerCity,
			SellerDistrict: g.SellerDistrict,
			StoreCode:      g.StoreCode,
			Items:          items,
		})
	}

	response.OK(c, CartListResponse{
		Groups:               groups,
		EstimatedTotalAmount: resp.EstimatedTotalAmount,
		TotalItems:           resp.TotalItems,
	})
}

// SetItemQuantity 設定商品數量（0 = 刪除）
// @Summary      設定購物車商品數量
// @Description  更新購物車中指定 SKU 的數量，設為 0 等同刪除該商品。支援訪客（需提供 guestId）及登入會員
// @Tags         購物車 (Cart)
// @Accept       json
// @Produce      json
// @Param        sku_id   path  string  true   "SKU ID"
// @Param        request  body  SetItemQuantityRequest  true  "設定數量請求（訪客需提供 guestId）"
// @Success      200  {object}  object{code=int,data=CartModifyResponse}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /public/carts/items/{sku_id}/quantity [put]
func (h *Holder) SetItemQuantity(c *gin.Context) {
	accountID := getOptionalAccountID(c)
	buyerID, ok := h.resolveCartBuyerID(c, accountID)
	if !ok {
		return
	}
	skuID := c.Param("sku_id")

	var req struct {
		GuestID  string `json:"guestId"`
		Quantity string `json:"quantity" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	// 訪客必須提供 guest_id
	if buyerID == 0 && req.GuestID == "" {
		response.BadRequest(c, "guest_id is required for guest user")
		return
	}

	grpcReq := &pb.SetItemQuantityRequest{
		GuestId:  req.GuestID,
		SkuId:    skuID,
		Quantity: req.Quantity,
	}

	resp, err := h.Cart.SetItemQuantity(c.Request.Context(), buyerID, grpcReq)
	if err != nil {
		response.GrpcErr(c, "cart", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, gin.H{
		"totalItems": resp.TotalItems,
	})
}

// MergeCart 合併購物車（登入後，訪客車 → 會員車）
// @Summary      合併購物車
// @Description  登入後將訪客購物車合併至會員購物車，需提供訪客 guestId。同帳號上架的商品在合併時靜默移除，移除數量回傳於 removedSelfOwnedCount（§5.3）
// @Tags         購物車 (Cart)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  MergeCartRequest  true  "合併購物車請求"
// @Success      200  {object}  object{code=int,message=string,data=MergeCartResponseData}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Router       /app/carts/merge [post]
func (h *Holder) MergeCart(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok || accountID == 0 {
		response.Unauthorized(c)
		return
	}
	buyerID, ok := h.resolveCartBuyerID(c, accountID)
	if !ok {
		return
	}

	var req struct {
		GuestID string `json:"guestId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "guest_id is required")
		return
	}

	grpcReq := &pb.MergeCartRequest{
		GuestId: req.GuestID,
	}

	resp, err := h.Cart.MergeCart(c.Request.Context(), buyerID, grpcReq)
	if err != nil {
		response.GrpcErr(c, "cart", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, "merge cart failed", nil)
		return
	}
	response.OK(c, MergeCartResponseData{
		RemovedSelfOwnedCount: resp.RemovedSelfOwnedCount,
	})
}

// DeleteCartItem 刪除購物車商品（SetItemQuantity quantity=0 的快捷方式）
// @Summary      刪除購物車商品
// @Description  刪除購物車中指定 SKU 的商品（等同將數量設為 0）。支援訪客（需提供 guest_id）及登入會員
// @Tags         購物車 (Cart)
// @Accept       json
// @Produce      json
// @Param        sku_id    path   string  true   "SKU ID"
// @Param        guest_id  query  string  false  "訪客 ID（訪客必填）"
// @Success      200  {object}  object{code=int,data=CartModifyResponse}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /public/carts/items/{sku_id} [delete]
func (h *Holder) DeleteCartItem(c *gin.Context) {
	accountID := getOptionalAccountID(c)
	buyerID, ok := h.resolveCartBuyerID(c, accountID)
	if !ok {
		return
	}
	skuID := c.Param("sku_id")

	guestID := c.Query("guest_id")

	// 訪客必須提供 guest_id
	if buyerID == 0 && guestID == "" {
		response.BadRequest(c, "guest_id is required for guest user")
		return
	}

	grpcReq := &pb.SetItemQuantityRequest{
		GuestId:  guestID,
		SkuId:    skuID,
		Quantity: strconv.Itoa(0),
	}

	resp, err := h.Cart.SetItemQuantity(c.Request.Context(), buyerID, grpcReq)
	if err != nil {
		response.GrpcErr(c, "cart", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, gin.H{
		"totalItems": resp.TotalItems,
	})
}
