package handler

import (
	"strconv"

	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/go-pkg/httputil"
	"github.com/Will413028/hawkr/go-pkg/pagination"
	pb "github.com/Will413028/hawkr/go-pkg/proto/order"
	"github.com/gin-gonic/gin"
)

// ==================== DTO ====================

// OrderDetailResponse 訂單詳情回應
type OrderDetailResponse struct {
	OrderNo             string `json:"orderNo,omitempty"`
	CreateTime          string `json:"createTime,omitempty"`
	PayTime             string `json:"payTime,omitempty"`
	ShipTime            string `json:"shipTime,omitempty"`
	Status              string `json:"status,omitempty"`
	TotalOriginalAmount string `json:"totalOriginalAmount,omitempty"`
	TotalActualAmount   string `json:"totalActualAmount,omitempty"`
	ShipmentNo          string `json:"shipmentNo,omitempty"`
	TransactionID       string `json:"transactionId,omitempty"`
	BuyerID             string `json:"buyerId,omitempty"`
	StoreID             string `json:"storeId,omitempty"`
	DeliveryTemp        string `json:"deliveryTemp,omitempty"`
	ShippingMethod      string `json:"shippingMethod,omitempty"`
	ShippingFee         string `json:"shippingFee,omitempty"`
	ShippingFeePayer    string `json:"shippingFeePayer,omitempty"`
	RecipientName       string `json:"recipientName,omitempty"`
	RecipientPhone      string `json:"recipientPhone,omitempty"`
	ShippingAddress     string `json:"shippingAddress,omitempty"`
	ShippingNote        string `json:"shippingNote,omitempty"`
	OrderRemark         string `json:"orderRemark,omitempty"`
	PickupStoreName     string `json:"pickupStoreName,omitempty"`
	ShippingArea        string `json:"shippingArea,omitempty"`
	CouponInstanceId    string `json:"couponInstanceId,omitempty"`
	DiscountAmount      string `json:"discountAmount,omitempty"`
	ShippingStatus      string `json:"shippingStatus,omitempty"`
	ShippingStatusDesc  string `json:"shippingStatusDesc,omitempty"`
	LabelKey            string `json:"labelKey,omitempty"` // 託運單 PDF objectKey（SSOT；49 §6.1）
	LabelUrl            string `json:"labelUrl,omitempty"` // 託運單 PDF 顯示 URL（Gateway 組）
	LabelExpireAt       string `json:"labelExpireAt,omitempty"`
	PartnerId           string `json:"partnerId,omitempty"`
	Odno                string `json:"odno,omitempty"`
	ValidationNo        string `json:"validationNo,omitempty"`
	PaymentMethod       string `json:"paymentMethod,omitempty"`
	AtmBankCode         string `json:"atmBankCode,omitempty"`
	AtmVirtualAccount   string `json:"atmVirtualAccount,omitempty"`
	AtmExpireDate       string `json:"atmExpireDate,omitempty"`
	SellerName          string `json:"sellerName,omitempty"`
	// 賣家 canonical 短碼（sellers.store_code）；storeId 是內部代理鍵，賣家頁
	// canonical 網址走 storeCode。與 sellerName 同源自 order-service 的
	// batchGetSellerMeta，聚合失敗時不輸出。
	StoreCode          string                    `json:"storeCode,omitempty"`
	SellerLogoKey      string                    `json:"logoKey,omitempty"` // 商家 logo objectKey（SSOT；49 §6.1）
	SellerLogoUrl      string                    `json:"logoUrl,omitempty"` // 商家 logo 顯示 URL（Gateway 組）
	SellerCity         string                    `json:"sellerCity,omitempty"`
	SellerDistrict     string                    `json:"sellerDistrict,omitempty"`
	BuyerNickname      string                    `json:"buyerNickname,omitempty"`
	BuyerAvatarKey     string                    `json:"avatarKey,omitempty"` // 買家頭像 objectKey（SSOT；49 §6.1）
	BuyerAvatarUrl     string                    `json:"avatarUrl,omitempty"` // 買家頭像顯示 URL（Gateway 組）
	CancelReason       string                    `json:"cancelReason,omitempty"`
	CancelRemark       string                    `json:"cancelRemark,omitempty"`
	CancelByRole       string                    `json:"cancelByRole,omitempty"`
	ReviewDeadline     string                    `json:"reviewDeadline,omitempty"`
	Review             *OrderReviewResponse      `json:"review"` // 訂單評論快照（訂單層級單一，取最近一筆商品評論；未評論為 null）
	AutoCompleteAt     string                    `json:"autoCompleteAt,omitempty"`
	ProcessingDeadline string                    `json:"processingDeadline,omitempty"`
	PaymentDeadline    string                    `json:"paymentDeadline,omitempty"` // 付款期限（ISO 8601 UTC，僅 status=0 時有值）
	ProviderStatus     string                    `json:"providerStatus,omitempty"`
	ProviderMessage    string                    `json:"providerMessage,omitempty"`
	PaymentMerID       string                    `json:"paymentMerId,omitempty"`      // 訂單對應 PayUni MerID（給 Google Pay 重試 tokenization）
	ShipmentPhotoKeys  []string                  `json:"shipmentPhotoKeys,omitempty"` // 出貨照片 objectKey（SSOT，前端 buildImageUrl 組顯示；49 §6.1）
	ShipmentPhotoUrls  []string                  `json:"shipmentPhotoUrls,omitempty"` // 出貨照片顯示 URL（Gateway 組）
	ReceiptPhotoKeys   []string                  `json:"receiptPhotoKeys,omitempty"`  // 收貨照片 objectKey（SSOT）
	ReceiptPhotoUrls   []string                  `json:"receiptPhotoUrls,omitempty"`  // 收貨照片顯示 URL（Gateway 組）
	Items              []OrderItemDetailResponse `json:"items,omitempty"`
	RefundHistory      []RefundRequestResponse   `json:"refundHistory,omitempty"`
	IsReviewed         bool                      `json:"isReviewed"`
	CanReview          bool                      `json:"canReview"`
	NeedRefundPage     bool                      `json:"needRefundPage"`
	// 再次付款可用付款方式（依 totalActualAmount 算）。僅買家 + status=pending_payment 時非空（03 §4.2.1）；前端據此 render（禁用 + 理由）
	AvailablePaymentMethods []AvailablePaymentMethodResponse `json:"availablePaymentMethods,omitempty"`
}

// OrderItemDetailResponse 訂單明細項目
type OrderItemDetailResponse struct {
	ID              string `json:"id,omitempty"`
	ProductID       string `json:"productId,omitempty"`   // 商品 ID（快照）
	ProductName     string `json:"productName,omitempty"` // 商品名稱（快照）
	SkuID           string `json:"skuId,omitempty"`
	SkuName         string `json:"skuName,omitempty"` // 規格名稱
	Quantity        string `json:"quantity,omitempty"`
	UnitPrice       string `json:"unitPrice,omitempty"` // 實際成交單價快照（無特價=listPrice，有特價=當時 sale_price）
	ListPrice       string `json:"listPrice,omitempty"` // 原價快照（給前端 strike-through 顯示）
	ActualPayAmount string `json:"actualPayAmount,omitempty"`
	RefundStatus    string `json:"refundStatus,omitempty"` // 退款狀態（0=正常, 10=退款中, 20=已退款, 21=退款駁回）
	ShareDiscount   string `json:"shareDiscount,omitempty"`
	ImageKey        string `json:"imageKey,omitempty"` // 商品圖片 objectKey（SSOT；49 §6.1）
	ImageUrl        string `json:"imageUrl,omitempty"` // 商品圖片顯示 URL（Gateway 組）
}

// OrderReviewResponse 訂單評論快照（訂單層級單一，取該訂單最近一筆商品評論；凍結快照，來源 review.created MQ 事件）
type OrderReviewResponse struct {
	Rating    string   `json:"rating,omitempty"`    // 評分 1-5（數字字串）
	Content   string   `json:"content,omitempty"`   // 評論文字（可空）
	ImageKeys []string `json:"imageKeys,omitempty"` // 評論附圖 objectKey（SSOT，前端 buildImageUrl 組顯示；49 §6.1）
	ImageUrls []string `json:"imageUrls,omitempty"` // 評論附圖顯示 URL（Gateway 組）
	CreatedAt string   `json:"createdAt,omitempty"` // 評論建立時間（ISO 8601 UTC）
}

// RefundRequestResponse 退款申請紀錄
type RefundRequestResponse struct {
	RequestNo    string `json:"requestNo,omitempty"`
	OrderItemID  string `json:"orderItemId,omitempty"`
	Status       string `json:"status,omitempty"`
	RefundAmount string `json:"refundAmount,omitempty"`
	Reason       string `json:"reason,omitempty"`
	CreateTime   string `json:"createTime,omitempty"`
}

// OrderSummaryResponse 訂單摘要（列表用）
// OrderStatusCountResponse 單一狀態的訂單筆數（數字皆 string）
type OrderStatusCountResponse struct {
	Status string `json:"status"`
	Count  string `json:"count"`
}

// MyOrderStatusCountsResponse 買家訂單各狀態筆數 rollup（total 為 counts 加總）
type MyOrderStatusCountsResponse struct {
	Counts []OrderStatusCountResponse `json:"counts"`
	Total  string                     `json:"total"`
}

type OrderSummaryResponse struct {
	BuyerNickname      string `json:"buyerNickname,omitempty"`
	ShippingStatusDesc string `json:"shippingStatusDesc,omitempty"`
	Status             string `json:"status,omitempty"`
	TotalActualAmount  string `json:"totalActualAmount,omitempty"`
	ItemCount          string `json:"itemCount,omitempty"`
	StoreID            string `json:"storeId,omitempty"`
	SellerName         string `json:"sellerName,omitempty"`
	// 賣家 canonical 短碼；見 OrderDetailResponse.StoreCode。
	StoreCode          string                  `json:"storeCode,omitempty"`
	SellerLogoKey      string                  `json:"logoKey,omitempty"` // 商家 logo objectKey（SSOT；49 §6.1）
	SellerLogoUrl      string                  `json:"logoUrl,omitempty"` // 商家 logo 顯示 URL（Gateway 組）
	SellerCity         string                  `json:"sellerCity,omitempty"`
	SellerDistrict     string                  `json:"sellerDistrict,omitempty"`
	ShippingMethod     string                  `json:"shippingMethod,omitempty"`
	DeliveryTemp       string                  `json:"deliveryTemp,omitempty"`
	PickupStoreName    string                  `json:"pickupStoreName,omitempty"`
	ShippingStatus     string                  `json:"shippingStatus,omitempty"`
	BuyerID            string                  `json:"buyerId,omitempty"`
	StatusTime         string                  `json:"statusTime,omitempty"`
	StatusSummary      string                  `json:"statusSummary,omitempty"`
	RecipientName      string                  `json:"recipientName,omitempty"`
	CreateTime         string                  `json:"createTime,omitempty"`
	ShippingAddress    string                  `json:"shippingAddress,omitempty"`
	Odno               string                  `json:"odno,omitempty"`
	BuyerAvatarKey     string                  `json:"avatarKey,omitempty"` // 買家頭像 objectKey（SSOT；49 §6.1）
	BuyerAvatarUrl     string                  `json:"avatarUrl,omitempty"` // 買家頭像顯示 URL（Gateway 組）
	LabelKey           string                  `json:"labelKey,omitempty"`  // 託運單 PDF objectKey（SSOT；49 §6.1）
	LabelUrl           string                  `json:"labelUrl,omitempty"`  // 託運單 PDF 顯示 URL（Gateway 組）
	LabelExpireAt      string                  `json:"labelExpireAt,omitempty"`
	PartnerId          string                  `json:"partnerId,omitempty"`
	OrderNo            string                  `json:"orderNo,omitempty"`
	ValidationNo       string                  `json:"validationNo,omitempty"`
	ReviewDeadline     string                  `json:"reviewDeadline,omitempty"`
	AutoCompleteAt     string                  `json:"autoCompleteAt,omitempty"`
	ProcessingDeadline string                  `json:"processingDeadline,omitempty"`
	PaymentDeadline    string                  `json:"paymentDeadline,omitempty"` // 付款期限（ISO 8601 UTC，僅 status=0 時有值）
	CancelReason       string                  `json:"cancelReason,omitempty"`
	CancelRemark       string                  `json:"cancelRemark,omitempty"`
	CancelByRole       string                  `json:"cancelByRole,omitempty"`
	ProviderMessage    string                  `json:"providerMessage,omitempty"`
	ProviderStatus     string                  `json:"providerStatus,omitempty"`
	Items              []OrderItemViewResponse `json:"items,omitempty"`
	ShipmentPhotoKeys  []string                `json:"shipmentPhotoKeys,omitempty"` // 出貨照片 objectKey（SSOT）
	ShipmentPhotoUrls  []string                `json:"shipmentPhotoUrls,omitempty"` // 出貨照片顯示 URL（Gateway 組）
	IsReviewed         bool                    `json:"isReviewed"`
	CanReview          bool                    `json:"canReview"`
	NeedRefundPage     bool                    `json:"needRefundPage"`
}

// AdminOrderSummaryResponse Admin 訂單列表單筆（/admin/orders）。
// 在共用摘要之上補 admin 交易頁欄位；App 端列表 DTO（OrderSummaryResponse）不揭露。
type AdminOrderSummaryResponse struct {
	OrderSummaryResponse
	PaymentMethod string `json:"paymentMethod,omitempty"` // 付款方式（PaymentMethod value；合法值以 CodeMeta 業務類型碼.md 為準，2001-2004；未付款為空字串）
}

// OrderItemViewResponse 訂單摘要明細項目
type OrderItemViewResponse struct {
	ID          string `json:"id,omitempty"`
	ProductID   string `json:"productId,omitempty"`   // 商品 ID（快照）
	ProductName string `json:"productName,omitempty"` // 商品名稱（快照）
	SkuID       string `json:"skuId,omitempty"`
	SkuName     string `json:"skuName,omitempty"` // 規格名稱
	Quantity    string `json:"quantity,omitempty"`
	UnitPrice   string `json:"unitPrice,omitempty"` // 實際成交單價快照（無特價=listPrice，有特價=當時 sale_price）
	ListPrice   string `json:"listPrice,omitempty"` // 原價快照（給前端 strike-through 顯示用）
	ImageKey    string `json:"imageKey,omitempty"`  // 商品圖片 objectKey（SSOT；49 §6.1）
	ImageUrl    string `json:"imageUrl,omitempty"`  // 商品圖片顯示 URL（Gateway 組）
}

// AdminOrderDetailResponse 管理端訂單詳情（含資金流水 + §29 provider 對帳明細）
type AdminOrderDetailResponse struct {
	OrderInfo       *OrderDetailResponse           `json:"orderInfo,omitempty"`
	ProviderContext *AdminOrderProviderContextResp `json:"providerContext,omitempty"`
	Logs            []FinancialLogResponse         `json:"logs,omitempty"`
}

// AdminOrderProviderContextResp §29 Admin 對帳 provider 資訊
type AdminOrderProviderContextResp struct {
	ProviderSource   string `json:"providerSource"` // CodeMeta ProviderSource：PAYMENT / PAYMENT_REFUND / SHIPPING；"" = 無第三方環節可對帳
	ProviderStatus   string `json:"providerStatus"`
	ProviderMessage  string `json:"providerMessage"`
	ProviderTradeNo  string `json:"providerTradeNo,omitempty"`  // Payment 相關
	ProviderRefundNo string `json:"providerRefundNo,omitempty"` // Refund 相關
	ShipTradeNo      string `json:"shipTradeNo,omitempty"`      // Shipping 相關
}

// FinancialLogResponse 資金流水紀錄
type FinancialLogResponse struct {
	Type          string `json:"type,omitempty"`
	Amount        string `json:"amount,omitempty"`
	ExternalTxnID string `json:"externalTxnId,omitempty"`
	CreateTime    string `json:"createTime,omitempty"`
}

// CreateOrderRequest 建立訂單請求
type CreateOrderRequest struct {
	StoreID             string                   `json:"storeId" binding:"required"`
	TotalOriginalAmount string                   `json:"totalOriginalAmount" binding:"required"`
	TotalActualAmount   string                   `json:"totalActualAmount" binding:"required"`
	CheckoutID          string                   `json:"checkoutId"`
	DeliveryTemp        string                   `json:"deliveryTemp"`
	ShippingMethod      string                   `json:"shippingMethod"`
	ShippingFee         string                   `json:"shippingFee"`
	ShippingFeePayer    string                   `json:"shippingFeePayer"`
	Items               []CreateOrderItemRequest `json:"items" binding:"required"`
}

// CreateOrderItemRequest 建立訂單明細項目
type CreateOrderItemRequest struct {
	SkuID     string `json:"skuId" binding:"required"`
	SkuName   string `json:"skuName" binding:"required"`
	Quantity  string `json:"quantity" binding:"required"`
	UnitPrice string `json:"unitPrice" binding:"required"` // 實際成交單價（無特價=listPrice）
	ListPrice string `json:"listPrice,omitempty"`          // 原價快照（無特價可省略，後端視為 = unitPrice）
}

// CreateOrderResponse 建立訂單回應
type CreateOrderResponse struct {
	OrderNo string `json:"orderNo"`
}

// SuccessResponse 通用成功回應
type SuccessResponse struct {
	Success bool `json:"success"`
}

// ==================== 轉換函式 ====================

func convertOrderDetail(d *pb.OrderDetailView, mediaBase string) *OrderDetailResponse {
	if d == nil {
		return nil
	}
	r := &OrderDetailResponse{
		OrderNo:                 d.OrderNo,
		CreateTime:              d.CreateTime,
		PayTime:                 d.PayTime,
		ShipTime:                d.ShipTime,
		Status:                  d.Status,
		TotalOriginalAmount:     d.TotalOriginalAmount,
		TotalActualAmount:       d.TotalActualAmount,
		ShipmentNo:              d.ShipmentNo,
		TransactionID:           d.TransactionId,
		BuyerID:                 d.BuyerId,
		StoreID:                 d.StoreId,
		DeliveryTemp:            d.DeliveryTemp,
		ShippingMethod:          d.ShippingMethod,
		ShippingFee:             d.ShippingFee,
		ShippingFeePayer:        d.ShippingFeePayer,
		RecipientName:           d.RecipientName,
		RecipientPhone:          d.RecipientPhone,
		ShippingAddress:         d.ShippingAddress,
		ShippingNote:            d.ShippingNote,
		OrderRemark:             d.OrderRemark,
		PickupStoreName:         d.PickupStoreName,
		ShippingArea:            d.ShippingArea,
		CouponInstanceId:        d.CouponInstanceId,
		DiscountAmount:          d.DiscountAmount,
		ShippingStatus:          d.ShippingStatus,
		ShippingStatusDesc:      d.ShippingStatusDesc,
		LabelKey:                d.LabelKey,
		LabelUrl:                buildURL(mediaBase, d.LabelKey),
		LabelExpireAt:           d.LabelExpireAt,
		PartnerId:               d.PartnerId,
		Odno:                    d.Odno,
		ValidationNo:            d.ValidationNo,
		PaymentMethod:           d.PaymentMethod,
		AtmBankCode:             d.AtmBankCode,
		AtmVirtualAccount:       d.AtmVirtualAccount,
		AtmExpireDate:           d.AtmExpireDate,
		SellerName:              d.SellerName,
		StoreCode:               d.StoreCode,
		SellerLogoKey:           d.SellerAvatarKey,
		SellerLogoUrl:           buildURL(mediaBase, d.SellerAvatarKey),
		SellerCity:              d.SellerCity,
		SellerDistrict:          d.SellerDistrict,
		BuyerNickname:           d.BuyerNickname,
		BuyerAvatarKey:          d.BuyerAvatarKey,
		BuyerAvatarUrl:          buildURL(mediaBase, d.BuyerAvatarKey),
		CancelReason:            d.CancelReason,
		CancelRemark:            d.CancelRemark,
		CancelByRole:            d.GetCancelByRole(),
		ShipmentPhotoKeys:       d.ShipmentPhotoKeys,
		ShipmentPhotoUrls:       buildURLs(mediaBase, d.ShipmentPhotoKeys),
		ReceiptPhotoKeys:        d.ReceiptPhotoKeys,
		ReceiptPhotoUrls:        buildURLs(mediaBase, d.ReceiptPhotoKeys),
		IsReviewed:              d.IsReviewed,
		ReviewDeadline:          d.ReviewDeadline,
		AutoCompleteAt:          d.AutoCompleteAt,
		ProcessingDeadline:      d.GetProcessingDeadline(),
		PaymentDeadline:         d.GetPaymentDeadline(),
		CanReview:               d.CanReview,
		NeedRefundPage:          d.GetNeedRefundPage(),
		ProviderStatus:          d.GetProviderStatus(),
		ProviderMessage:         d.GetProviderMessage(),
		PaymentMerID:            d.GetPaymentMerId(),
		AvailablePaymentMethods: convertAvailablePaymentMethods(d.GetAvailablePaymentMethods()),
	}
	if d.Review != nil {
		r.Review = &OrderReviewResponse{
			Rating:    d.Review.Rating,
			Content:   d.Review.Content,
			ImageKeys: d.Review.ImageKeys,
			ImageUrls: buildURLs(mediaBase, d.Review.ImageKeys),
			CreatedAt: d.Review.CreatedAt,
		}
	}
	for _, item := range d.Items {
		itemResp := OrderItemDetailResponse{
			ID:              item.Id,
			ProductID:       item.ProductId,
			ProductName:     item.ProductName,
			SkuID:           item.SkuId,
			SkuName:         item.SkuName,
			Quantity:        item.Quantity,
			UnitPrice:       item.UnitPrice,
			ListPrice:       item.ListPrice,
			ActualPayAmount: item.ActualPayAmount,
			RefundStatus:    item.RefundStatus,
			ShareDiscount:   item.ShareDiscount,
			ImageKey:        item.ImageKey,
			ImageUrl:        buildURL(mediaBase, item.ImageKey),
		}
		r.Items = append(r.Items, itemResp)
	}
	for _, ref := range d.RefundHistory {
		r.RefundHistory = append(r.RefundHistory, RefundRequestResponse{
			RequestNo:    ref.RequestNo,
			OrderItemID:  ref.OrderItemId,
			Status:       ref.Status,
			RefundAmount: ref.RefundAmount,
			Reason:       ref.Reason,
			CreateTime:   ref.CreateTime,
		})
	}
	return r
}

func convertOrderSummaries(orders []*pb.OrderSummaryView, mediaBase string) []OrderSummaryResponse {
	result := make([]OrderSummaryResponse, 0, len(orders))
	for _, o := range orders {
		s := OrderSummaryResponse{
			OrderNo:            o.OrderNo,
			CreateTime:         o.CreateTime,
			Status:             o.Status,
			TotalActualAmount:  o.TotalActualAmount,
			ItemCount:          o.ItemCount,
			StoreID:            o.StoreId,
			SellerName:         o.SellerName,
			StoreCode:          o.StoreCode,
			SellerLogoKey:      o.SellerAvatarKey,
			SellerLogoUrl:      buildURL(mediaBase, o.SellerAvatarKey),
			SellerCity:         o.SellerCity,
			SellerDistrict:     o.SellerDistrict,
			ShippingMethod:     o.ShippingMethod,
			DeliveryTemp:       o.DeliveryTemp,
			PickupStoreName:    o.PickupStoreName,
			ShippingStatus:     o.ShippingStatus,
			ShippingStatusDesc: o.ShippingStatusDesc,
			StatusTime:         o.StatusTime,
			StatusSummary:      o.StatusSummary,
			RecipientName:      o.RecipientName,
			ShippingAddress:    o.ShippingAddress,
			BuyerID:            o.BuyerId,
			BuyerNickname:      o.BuyerNickname,
			BuyerAvatarKey:     o.BuyerAvatarKey,
			BuyerAvatarUrl:     buildURL(mediaBase, o.BuyerAvatarKey),
			LabelKey:           o.LabelKey,
			LabelUrl:           buildURL(mediaBase, o.LabelKey),
			LabelExpireAt:      o.LabelExpireAt,
			PartnerId:          o.PartnerId,
			Odno:               o.Odno,
			ValidationNo:       o.ValidationNo,
			ShipmentPhotoKeys:  o.ShipmentPhotoKeys,
			ShipmentPhotoUrls:  buildURLs(mediaBase, o.ShipmentPhotoKeys),
			IsReviewed:         o.IsReviewed,
			ReviewDeadline:     o.ReviewDeadline,
			AutoCompleteAt:     o.AutoCompleteAt,
			ProcessingDeadline: o.GetProcessingDeadline(),
			PaymentDeadline:    o.GetPaymentDeadline(),
			CancelReason:       o.GetCancelReason(),
			CancelRemark:       o.GetCancelRemark(),
			CancelByRole:       o.GetCancelByRole(),
			CanReview:          o.CanReview,
			NeedRefundPage:     o.GetNeedRefundPage(),
			ProviderStatus:     o.GetProviderStatus(),
			ProviderMessage:    o.GetProviderMessage(),
		}
		for _, item := range o.Items {
			s.Items = append(s.Items, OrderItemViewResponse{
				ID:          item.Id,
				ProductID:   item.ProductId,
				ProductName: item.ProductName,
				SkuID:       item.SkuId,
				SkuName:     item.SkuName,
				Quantity:    item.Quantity,
				UnitPrice:   item.UnitPrice,
				ListPrice:   item.ListPrice,
				ImageKey:    item.ImageKey,
				ImageUrl:    buildURL(mediaBase, item.ImageKey),
			})
		}
		result = append(result, s)
	}
	return result
}

// convertAdminOrderSummaries Admin 列表：共用摘要 + admin 專屬欄位（paymentMethod）
func convertAdminOrderSummaries(orders []*pb.OrderSummaryView, mediaBase string) []AdminOrderSummaryResponse {
	base := convertOrderSummaries(orders, mediaBase)
	result := make([]AdminOrderSummaryResponse, 0, len(orders))
	for i, o := range orders {
		result = append(result, AdminOrderSummaryResponse{
			OrderSummaryResponse: base[i],
			PaymentMethod:        o.GetPaymentMethod(),
		})
	}
	return result
}

func convertAdminOrderDetail(d *pb.AdminOrderDetailData, mediaBase string) *AdminOrderDetailResponse {
	if d == nil {
		return nil
	}
	r := &AdminOrderDetailResponse{
		OrderInfo: convertOrderDetail(d.OrderInfo, mediaBase),
	}
	for _, log := range d.Logs {
		r.Logs = append(r.Logs, FinancialLogResponse{
			Type:          log.Type,
			Amount:        log.Amount,
			ExternalTxnID: log.ExternalTxnId,
			CreateTime:    log.CreateTime,
		})
	}
	// §29 providerContext
	if pc := d.GetProviderContext(); pc != nil && pc.GetProviderSource() != "" {
		r.ProviderContext = &AdminOrderProviderContextResp{
			ProviderSource:   pc.GetProviderSource(),
			ProviderStatus:   pc.GetProviderStatus(),
			ProviderMessage:  pc.GetProviderMessage(),
			ProviderTradeNo:  pc.GetProviderTradeNo(),
			ProviderRefundNo: pc.GetProviderRefundNo(),
			ShipTradeNo:      pc.GetShipTradeNo(),
		}
	}
	return r
}

// ==================== 買家端 ====================

// CreateOrder 建立訂單
// @Summary      建立訂單
// @Description  買家建立新訂單，需提供店家 ID（store_id）、商品項目及金額
// @Tags         訂單 (Order)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  CreateOrderRequest  true  "建立訂單請求"
// @Success      200  {object}  object{code=int,data=CreateOrderResponse}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Failure      409  {object}  object{code=int,message=string}
// @Router       /app/orders [post]
func (h *Holder) CreateOrder(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok || accountID == 0 {
		response.Unauthorized(c)
		return
	}
	buyerID, ok := h.resolveBuyerIDFor(c, accountID)
	if !ok {
		return
	}

	var req struct {
		StoreID             string `json:"storeId" binding:"required"`
		TotalOriginalAmount string `json:"totalOriginalAmount" binding:"required"`
		TotalActualAmount   string `json:"totalActualAmount" binding:"required"`
		CheckoutID          string `json:"checkoutId"`
		DeliveryTemp        string `json:"deliveryTemp"`     // 配送溫層 Enum code
		ShippingMethod      string `json:"shippingMethod"`   // 運送方式 Enum code
		ShippingFee         string `json:"shippingFee"`      // 運費（新台幣元，數字字串）
		ShippingFeePayer    string `json:"shippingFeePayer"` // 運費負擔方 Enum code
		Items               []struct {
			SkuID     string `json:"skuId" binding:"required"`
			SkuName   string `json:"skuName" binding:"required"`
			Quantity  string `json:"quantity" binding:"required"`
			UnitPrice string `json:"unitPrice" binding:"required"`
			ListPrice string `json:"listPrice,omitempty"`
		} `json:"items" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	items := make([]*pb.CreateOrderItem, 0, len(req.Items))
	for _, item := range req.Items {
		// listPrice 缺省時退化為 unitPrice（無特價情境向下相容）
		listPrice := item.ListPrice
		if listPrice == "" {
			listPrice = item.UnitPrice
		}
		items = append(items, &pb.CreateOrderItem{
			SkuId:     item.SkuID,
			SkuName:   item.SkuName,
			Quantity:  item.Quantity,
			UnitPrice: item.UnitPrice,
			ListPrice: listPrice,
		})
	}

	resp, err := h.Order.CreateOrder(c.Request.Context(), &pb.CreateOrderRequest{
		BuyerId:             int64(buyerID),
		StoreId:             req.StoreID,
		Items:               items,
		TotalOriginalAmount: req.TotalOriginalAmount,
		TotalActualAmount:   req.TotalActualAmount,
		CheckoutId:          req.CheckoutID,
		DeliveryTemp:        req.DeliveryTemp,
		ShippingMethod:      req.ShippingMethod,
		ShippingFee:         req.ShippingFee,
		ShippingFeePayer:    req.ShippingFeePayer,
	})
	if err != nil {
		response.GrpcErr(c, "order", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, gin.H{
		"orderNo": resp.Data.GetOrderNo(),
	})
}

// CancelOrder 取消訂單
// @Summary      取消訂單
// @Description  買家取消指定訂單
// @Tags         訂單 (Order)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        order_no  path  string  true  "訂單編號"
// @Success      200  {object}  object{code=int,data=SuccessResponse}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Failure      409  {object}  object{code=int,message=string}
// @Router       /app/orders/{order_no}/cancellations [post]
func (h *Holder) CancelOrder(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok || accountID == 0 {
		response.Unauthorized(c)
		return
	}
	buyerID, ok := h.resolveBuyerIDFor(c, accountID)
	if !ok {
		return
	}

	orderNo := c.Param("order_no")
	if orderNo == "" {
		response.BadRequest(c, "order_no is required")
		return
	}

	resp, err := h.Order.CancelOrder(c.Request.Context(), &pb.CancelOrderRequest{
		BuyerId: int64(buyerID),
		OrderNo: orderNo,
	})
	if err != nil {
		response.GrpcErr(c, "order", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, map[string]bool{"success": resp.Success})
}

// GetMyOrders 查詢訂單列表
// 買家視角：預設，查詢自己購買的訂單
// 賣家視角：?view=seller，查詢自己銷售的訂單（可搭配 buyer_id 篩選指定買家）
// @Summary      查詢我的訂單列表
// @Description  查詢當前用戶的訂單列表。預設為買家視角；傳入 view=seller 切換為賣家視角，可搭配 buyer_id 篩選指定買家的訂單
// @Tags         訂單 (Order)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        status            query  string  false  "訂單狀態篩選，支援逗號分隔多值（如 10,20），空值=全部。合法值：0=待付款, 10=已付款, 20=待出貨, 30=配送中, 32=待取貨, 35=物流異常, 40=已送達, 50=已完成, 60=部分退款, 70=全額退款, 80=已取消"
// @Param        page              query  string  false  "頁碼（預設 1）"
// @Param        page_size         query  string  false  "每頁筆數（預設 10，最大 50）"
// @Param        view              query  string  false  "視角：空值=買家, seller=賣家"
// @Param        buyer_id          query  string  false  "篩選指定買家（搭配 view=seller 使用）"
// @Success      200  {object}  object{code=int,data=object{records=[]OrderSummaryResponse,total=string,page=string,pageSize=string}}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Router       /app/orders [get]
func (h *Holder) GetMyOrders(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok || accountID == 0 {
		response.Unauthorized(c)
		return
	}

	statusFilter := c.DefaultQuery("status", "")
	rawPage := httputil.ParsePageParam(c.Query("page"), 1)
	rawSize := httputil.ParsePageParam(c.Query("page_size"), defaultLimit)
	page, size, _ := pagination.NormalizePage(rawPage, rawSize, defaultLimit, maxPageSize)

	// 賣家視角：view=seller → 呼叫 GetSellerOrders gRPC（§5.1 角色分離，獨立 RPC 由 Gateway 分發）
	view := c.DefaultQuery("view", "")
	if view == "seller" {
		storeID, ok := h.resolveStoreID(c)
		if !ok {
			return
		}
		buyerIDFilter, ok := h.resolveOptionalBuyerFilterQuery(c, c.DefaultQuery("buyer_id", ""))
		if !ok {
			return
		}
		sellerResp, err := h.Order.GetSellerOrders(c.Request.Context(), &pb.GetSellerOrdersRequest{
			StoreId:      int64(storeID),
			StatusFilter: statusFilter,
			Page:         &pb.PageRequest{Page: strconv.Itoa(page), Size: strconv.Itoa(size)},
			BuyerId:      buyerIDFilter,
		})
		if err != nil {
			response.GrpcErr(c, "order", err)
			return
		}
		response.GrpcResult(c, sellerResp.Code, sellerResp.Message, response.PageData{
			Records:  convertOrderSummaries(sellerResp.Orders, h.MediaBaseURL),
			Total:    sellerResp.Meta.GetTotalItems(),
			Page:     sellerResp.Meta.GetCurrentPage(),
			PageSize: strconv.Itoa(size),
		})
		return
	}

	buyerID, ok := h.resolveBuyerIDFor(c, accountID)
	if !ok {
		return
	}
	resp, err := h.Order.GetMyOrders(c.Request.Context(), &pb.GetMyOrdersRequest{
		BuyerId:      int64(buyerID),
		StatusFilter: statusFilter,
		Page:         &pb.PageRequest{Page: strconv.Itoa(page), Size: strconv.Itoa(size)},
	})
	if err != nil {
		response.GrpcErr(c, "order", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, response.PageData{
		Records:  convertOrderSummaries(resp.Orders, h.MediaBaseURL),
		Total:    resp.Meta.GetTotalItems(),
		Page:     resp.Meta.GetCurrentPage(),
		PageSize: strconv.Itoa(size),
	})
}

// GetMyOrderStatusCounts 查詢買家訂單各狀態筆數 rollup
// 買家訂單頁各狀態 tab 筆數的唯一真實來源（取代前端多發 page_size=1 探測，見 03 §2.1）
// @Summary      查詢買家訂單各狀態筆數
// @Description  回傳當前買家各 OrderStatus 的原始狀態筆數（GROUP BY status），筆數為 0 的狀態省略；total 為 counts 加總。tab 分組加總由前端自行處理。賣家視角（view=seller/selling）一律 400 拒絕，賣家統計請走 Connect SellerAppService.GetMyDashboard。
// @Tags         訂單 (Order)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        view  query  string  false  "視角：空值/其他=買家；seller 或 selling=拒絕（賣家統計走 dashboard 四桶）"
// @Success      200  {object}  object{code=int,data=MyOrderStatusCountsResponse}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Router       /app/orders/status_counts [get]
func (h *Holder) GetMyOrderStatusCounts(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok || accountID == 0 {
		response.Unauthorized(c)
		return
	}

	// 賣家視角不走本 API：與 dashboard 四桶（含售後覆蓋）會形成兩套對不上的數字，違反單一真相源
	if authz.IsSellerView(c.Query("view")) {
		response.BadRequest(c, "賣家統計請走 Connect SellerAppService.GetMyDashboard（四桶狀態卡），本 API 僅回買家各狀態筆數")
		return
	}

	buyerID, ok := h.resolveBuyerIDFor(c, accountID)
	if !ok {
		return
	}

	resp, err := h.Order.GetMyOrderStatusCounts(c.Request.Context(), &pb.GetMyOrderStatusCountsRequest{
		BuyerId: int64(buyerID),
	})
	if err != nil {
		response.GrpcErr(c, "order", err)
		return
	}

	counts := make([]OrderStatusCountResponse, 0, len(resp.Counts))
	for _, sc := range resp.Counts {
		counts = append(counts, OrderStatusCountResponse{Status: sc.GetStatus(), Count: sc.GetCount()})
	}
	// total 由 order-service 計算（構造上恆 = counts 加總）；空字串兜 "0" 防 proto3 預設值外漏
	total := resp.GetTotal()
	if total == "" {
		total = "0"
	}

	response.GrpcResult(c, resp.Code, resp.Message, MyOrderStatusCountsResponse{
		Counts: counts,
		Total:  total,
	})
}

// GetOrder 查詢訂單詳情
// 買家視角：預設
// 賣家視角：?view=seller
// @Summary      查詢訂單詳情
// @Description  查詢指定訂單的詳細資訊。預設為買家視角；傳入 view=seller 切換為賣家視角
// @Tags         訂單 (Order)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        order_no  path   string  true   "訂單編號"
// @Param        view      query  string  false  "視角：空值=買家, seller=賣家"
// @Success      200  {object}  object{code=int,data=OrderDetailResponse}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/orders/{order_no} [get]
func (h *Holder) GetOrder(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok || accountID == 0 {
		response.Unauthorized(c)
		return
	}

	orderNo := c.Param("order_no")
	if orderNo == "" {
		response.BadRequest(c, "order_no is required")
		return
	}

	view := c.DefaultQuery("view", "")
	if view == "seller" {
		storeID, ok := h.resolveStoreID(c)
		if !ok {
			return
		}
		sellerResp, err := h.Order.GetSellerOrderDetail(c.Request.Context(), &pb.GetSellerOrderDetailRequest{
			StoreId: int64(storeID),
			OrderNo: orderNo,
		})
		if err != nil {
			response.GrpcErr(c, "order", err)
			return
		}
		response.GrpcResult(c, sellerResp.Code, sellerResp.Message, convertOrderDetail(sellerResp.Data, h.MediaBaseURL))
		return
	}

	buyerID, ok := h.resolveBuyerIDFor(c, accountID)
	if !ok {
		return
	}
	resp, err := h.Order.GetOrder(c.Request.Context(), &pb.GetOrderRequest{
		BuyerId: int64(buyerID),
		OrderNo: orderNo,
	})
	if err != nil {
		response.GrpcErr(c, "order", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, convertOrderDetail(resp.Data, h.MediaBaseURL))
}

// ConfirmReceiptRequest 確認收貨請求
type ConfirmReceiptRequest struct {
	StagedTokens []string `json:"stagedTokens,omitempty"` // 買家收貨照片 stagedToken（選填，由 media-service 預上傳取得）
}

// ConfirmReceipt 買家確認收貨
// @Summary      確認收貨
// @Description  買家對已送達訂單確認收貨，提前完成訂單。可選擇附上收貨照片（先呼叫 media-service 取得 stagedTokens）。若未主動確認，系統將在送達後 3 天自動完成。確認後系統將非同步進行撥款指示
// @Tags         訂單 (Order)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        order_no  path  string  true  "訂單編號"
// @Param        body      body  ConfirmReceiptRequest  false  "選填：收貨照片 stagedTokens"
// @Success      200  {object}  object{code=int,data=SuccessResponse}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      500  {object}  object{code=int,message=string}
// @Router       /app/orders/{order_no}/confirm_receipt [post]
func (h *Holder) ConfirmReceipt(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok || accountID == 0 {
		response.Unauthorized(c)
		return
	}
	buyerID, ok := h.resolveBuyerIDFor(c, accountID)
	if !ok {
		return
	}

	orderNo := c.Param("order_no")
	if orderNo == "" {
		response.BadRequest(c, "order_no is required")
		return
	}

	var req ConfirmReceiptRequest
	// body 為選填，忽略解析錯誤（允許空 body）
	_ = c.ShouldBindJSON(&req)

	resp, err := h.Order.ConfirmReceipt(c.Request.Context(), &pb.ConfirmReceiptRequest{
		BuyerId:      int64(buyerID),
		OrderNo:      orderNo,
		StagedTokens: req.StagedTokens,
	})
	if err != nil {
		response.GrpcErr(c, "order", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, map[string]bool{"success": resp.Success})
}

// ==================== 管理端 ====================

// SearchOrders 後台搜尋訂單
// @Summary      後台搜尋訂單
// @Description  管理員搜尋訂單，支援依訂單編號、買家 ID、賣家 ID、狀態篩選
// @Tags         管理員 (Admin)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        order_no           query  string  false  "訂單編號"
// @Param        buyer_id           query  string  false  "依買家過濾（現任 buyer_id 代理鍵前的原始 account_id，gateway 解析）"
// @Param        store_id           query  string  false  "依賣家過濾（現任 store_id 代理鍵前的原始 account_id，gateway 解析）"
// @Param        status             query  string  false  "訂單狀態篩選"
// @Param        page               query  string  false  "頁碼（預設 1）"
// @Param        page_size          query  string  false  "每頁筆數（預設 10，最大 50）"
// @Success      200  {object}  object{code=int,data=object{records=[]AdminOrderSummaryResponse,total=string,page=string,pageSize=string}}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Router       /admin/orders [get]
func (h *Holder) SearchOrders(c *gin.Context) {
	orderNo := c.DefaultQuery("order_no", "")
	status := c.DefaultQuery("status", "")
	rawPage := httputil.ParsePageParam(c.Query("page"), 1)
	rawSize := httputil.ParsePageParam(c.Query("page_size"), defaultLimit)
	page, size, _ := pagination.NormalizePage(rawPage, rawSize, defaultLimit, maxPageSize)

	buyerIDFilter, ok := h.resolveOptionalBuyerFilterQuery(c, c.DefaultQuery("buyer_id", ""))
	if !ok {
		return
	}
	storeIDFilter, ok := h.resolveOptionalSellerFilterQuery(c, c.DefaultQuery("store_id", ""))
	if !ok {
		return
	}

	resp, err := h.Order.SearchOrders(c.Request.Context(), &pb.SearchOrdersRequest{
		OrderNo: orderNo,
		BuyerId: buyerIDFilter,
		StoreId: storeIDFilter,
		Status:  status,
		Page:    &pb.PageRequest{Page: strconv.Itoa(page), Size: strconv.Itoa(size)},
	})
	if err != nil {
		response.GrpcErr(c, "order", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, response.PageData{
		Records:  convertAdminOrderSummaries(resp.Orders, h.MediaBaseURL),
		Total:    resp.Meta.GetTotalItems(),
		Page:     resp.Meta.GetCurrentPage(),
		PageSize: strconv.Itoa(size),
	})
}

// GetAdminOrderDetail 後台查詢訂單詳情（含資金流水）
// @Summary      後台查詢訂單詳情
// @Description  管理員查詢指定訂單的詳細資訊，包含資金流水
// @Tags         管理員 (Admin)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        order_no  path  string  true  "訂單編號"
// @Success      200  {object}  object{code=int,data=AdminOrderDetailResponse}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /admin/orders/{order_no} [get]
func (h *Holder) GetAdminOrderDetail(c *gin.Context) {
	orderNo := c.Param("order_no")
	if orderNo == "" {
		response.BadRequest(c, "order_no is required")
		return
	}

	resp, err := h.Order.GetAdminOrderDetail(c.Request.Context(), &pb.GetAdminOrderDetailRequest{
		OrderNo: orderNo,
	})
	if err != nil {
		response.GrpcErr(c, "order", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, convertAdminOrderDetail(resp.Data, h.MediaBaseURL))
}
