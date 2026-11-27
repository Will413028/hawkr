package handler

import (
	"context"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/Will413028/hawkr/gateway/internal/handler/delegation"
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	commonpb "github.com/Will413028/hawkr/go-pkg/proto/common"
	orderpb "github.com/Will413028/hawkr/go-pkg/proto/order"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"github.com/Will413028/hawkr/go-pkg/twaddr"
	"github.com/gin-gonic/gin"
)

// DelegatedOrderBuyer carries the買家顯示名 only.
type DelegatedOrderBuyer struct {
	DisplayName string `json:"displayName"`
}

// DelegatedOrderRecipient is the masked recipient block of the **detail**. 遮罩由
// order-service 做，不在 Gateway：放 Gateway 等於完整收件資訊在每一次列表請求時跨
// gRPC 傳輸並落進 Gateway 記憶體與 log ＝ 把 deny-by-default 降級成「傳過來再遮掉」。
type DelegatedOrderRecipient struct {
	Name        string `json:"name"`
	MaskedPhone string `json:"maskedPhone"`
	City        string `json:"city"`
	District    string `json:"district"`
}

// DelegatedOrderListRecipient is the列表版本，比詳情少一個 maskedPhone。
//
// OrderSummaryView 沒有收件人電話欄位（賣家列表 RPC 從不回），所以列表拿不到它。
// 型別分離而不是與詳情共用後留空：共用會在每一列送出一個恆為空字串的 PII 欄位，
// 讀契約的人以為列表拿得到、前端據此寫出一條永遠不執行的分支——那正是 DelegatedOrderListItem
// 自己那段註解拒絕做的事（「與其填一個看起來像資料的空字串，不如不宣告」）。
//
// 反向也要成立：這裡不得為了補齊而去 OrderSummaryView 新增 recipient_phone。那會讓
// 完整號碼流進**所有**賣家列表消費者，為了一個列表上用不到的欄位擴大 PII 面。要看電話
// 走詳情，要看完整號碼走 recipient-contact（必落稽核）。
type DelegatedOrderListRecipient struct {
	Name     string `json:"name"`
	City     string `json:"city"`
	District string `json:"district"`
}

// DelegatedOrderListItem is one row of GET .../orders.
//
// ⚠️ 沒有 paymentMethod／updatedAt／buyer.maskedPhone／recipient.maskedPhone：
// GetSellerOrders 的 OrderSummaryView 沒有這些來源欄位（賣家列表 RPC 從不回任何電話）。
// 與其填一個看起來像資料的空字串，不如不宣告——deny-by-default 下也不得為了對齊範例而
// 新增一個上游根本沒有的 PII 欄位。
//
// FirstProductName 反過來是「上游一直有、我們一直沒轉出去」的那一類：搜尋比對範圍含
// order_items 的 product_name／sku_name（OrderRepository.SELLER_ORDER_FILTER），而列表
// 的商品欄只印得出件數 ⇒ 搜一個品名拿到四筆，四列全是「共 2 件」，畫面無法自我解釋
// 是哪一件命中。品名在 §8.1.5 的 order allow-list 內（「商品縮圖／名稱／件數」），且
// OrderSummaryView.items 不在 applyDelegatedProjection 的 clear 清單內
// （OrderGrpcService.java:863-885）⇒ 缺口只在這一層。
type DelegatedOrderListItem struct {
	OrderNo             string                      `json:"orderNo"`
	CreatedAt           string                      `json:"createdAt"`
	Status              string                      `json:"status"`
	StatusLabel         string                      `json:"statusLabel"`
	ItemCount           string                      `json:"itemCount"`
	FirstProductName    string                      `json:"firstProductName"`
	TotalAmount         string                      `json:"totalAmount"`
	ShippingStatus      string                      `json:"shippingStatus"`
	ShippingStatusLabel string                      `json:"shippingStatusLabel"`
	ShippingMethod      string                      `json:"shippingMethod"`
	DeliveryTemp        string                      `json:"deliveryTemp"`
	Buyer               DelegatedOrderBuyer         `json:"buyer"`
	Recipient           DelegatedOrderListRecipient `json:"recipient"`
}

// DelegatedOrderItem is one line of the detail's items[].
type DelegatedOrderItem struct {
	ProductID       string `json:"productId"`
	ProductName     string `json:"productName"`
	SkuID           string `json:"skuId"`
	SkuName         string `json:"skuName"`
	Quantity        string `json:"quantity"`
	UnitPrice       string `json:"unitPrice"`
	ListPrice       string `json:"listPrice"`
	ActualPayAmount string `json:"actualPayAmount"`
	ImageKey        string `json:"imageKey"`
}

// DelegatedOrderTimelineEntry is one point of the status timeline. The existing DTO
// keeps these as separate scalar columns; the delegated DTO collapses them into an
// array because that is what the workspace renders.
//
// OccurredAt 允許為空：訂單**當下**的狀態一定發生過，但 orders 表只為建立／付款／出貨
// 三個里程碑與三個結案節點（送達／完成／取消）留了時間欄。待出貨／待取貨／物流異常／
// 部分退款／全額退款沒有專屬時間欄，此時送空字串由前端不渲染時間，比補一個 updated_at
// 充數誠實——那一欄會被任何後續寫入推走（見 terminalStatusTime 的實測數字）。
type DelegatedOrderTimelineEntry struct {
	Status      string `json:"status"`
	StatusLabel string `json:"statusLabel"`
	OccurredAt  string `json:"occurredAt"`
}

// DelegatedOrderDetail is GET .../orders/{orderNo}.
//
// ⚠️ ItemsSubtotal／CancelReason／CancelRemark／CancelByRole 四欄逐項對照 §8.1.5 的
// order allow-list：
//   - `ItemsSubtotal` ＝ 上游 `total_original_amount`，屬「金額」。少了它，畫面上的
//     運費／折扣／訂單金額三個數字湊不出彼此的關係，操作者無從判斷金額是否合理。
//   - 取消三欄是「訂單時間軸」終態節點的內容本身：時間軸說得出「已取消」，說不出
//     「為什麼」，而代行者要協助賣家正是需要那個為什麼。三欄皆非 PII、非金流明細；
//     `cancel_remark` 的寫入者只有系統排程與賣家（買家自願取消時 `cancel_reason` 與
//     `cancel_remark` 一律留 NULL，見 CodeMeta `OrderCancelReason` 的說明），所以它是
//     賣家自己的內容，不是買家自由文字。
//
// 反向仍成立：`shipmentNo`／`labelKey`／`odno`／`validationNo`／`transactionId` 那一組
// 履約與金流欄位一個都不加（admin_delegated_contract_test.go 以欄位名為錨守住）。
type DelegatedOrderDetail struct {
	OrderNo             string                        `json:"orderNo"`
	CreatedAt           string                        `json:"createdAt"`
	Status              string                        `json:"status"`
	StatusLabel         string                        `json:"statusLabel"`
	ShippingStatus      string                        `json:"shippingStatus"`
	ShippingStatusLabel string                        `json:"shippingStatusLabel"`
	PaymentMethod       string                        `json:"paymentMethod"`
	ShippingMethod      string                        `json:"shippingMethod"`
	DeliveryTemp        string                        `json:"deliveryTemp"`
	ItemCount           string                        `json:"itemCount"`
	ItemsSubtotal       string                        `json:"itemsSubtotal"`
	TotalAmount         string                        `json:"totalAmount"`
	ShippingFee         string                        `json:"shippingFee"`
	DiscountAmount      string                        `json:"discountAmount"`
	CancelReason        string                        `json:"cancelReason"`
	CancelRemark        string                        `json:"cancelRemark"`
	CancelByRole        string                        `json:"cancelByRole"`
	Buyer               DelegatedOrderBuyer           `json:"buyer"`
	Recipient           DelegatedOrderRecipient       `json:"recipient"`
	Items               []DelegatedOrderItem          `json:"items"`
	Timeline            []DelegatedOrderTimelineEntry `json:"timeline"`
	OrderRemark         string                        `json:"orderRemark"`
	ShippingNote        string                        `json:"shippingNote"`
}

// DelegatedRecipientContact is the ONLY response that carries unmasked recipient data.
type DelegatedRecipientContact struct {
	Name        string `json:"name"`
	Phone       string `json:"phone"`
	FullAddress string `json:"fullAddress"`
	ZipCode     string `json:"zipCode"`
}

// DelegatedRecipientContactData wraps it with the audit row id, which is what makes
// this disclosure traceable at all.
type DelegatedRecipientContactData struct {
	AuditLogID string                    `json:"auditLogId"`
	Recipient  DelegatedRecipientContact `json:"recipient"`
}

// ListDelegatedOrders handles GET .../orders.
//
// keyword 的比對範圍恰為 order_no、recipient_name 與 order_items 的 product_name／
// sku_name，由 order-service 實作。**不得**擴充到 recipient_phone 或完整
// shipping_address：判準是「列表投影看得到的才可以搜」——姓名在代行投影裡未遮罩、
// 列表本來就顯示得出來，搜尋只是省去翻頁；而電話列表 RPC 從不回傳、地址只給到縣市區
// ⇒ 拿它們當條件等於造出一支 oracle，輸入一個值看有沒有命中，就能在完全不揭露該值的
// 前提下確認它存在，而且不會觸發 ORDER_PII_REVEAL 稽核（§8.1.5／A16）。
//
// @Summary      代行訂單列表
// @Description  §6.1.10 的唯讀訂單列表，capability = orders.read。keyword 為模糊搜尋，比對範圍恰為訂單編號、收件人姓名與品項名稱（商品／規格）——刻意不含收件人電話與完整地址，那兩者列表投影本來就看不到，拿來當搜尋條件等於在不揭露值的前提下確認它存在。每一列帶 firstProductName（第一件品項名，product_name 為空時退回 sku_name），與 itemCount 一起讓畫面說得出 keyword 命中的是哪一件。date_from／date_to 依建單時間篩選。
// @Tags         代行營運 (Delegated Operations)
// @Accept       json
// @Produce      json
// @Param        X-HK-Operation-Session  header  string  true   "代行工作階段憑證"
// @Param        store_id                path    string  true   "目標店家 store_id"
// @Param        status                  query   string  false  "訂單狀態碼，支援逗號分隔多值（如 20,30）"
// @Param        keyword                 query   string  false  "模糊搜尋，比對訂單編號／收件人姓名／品項名稱，上限 255 字"
// @Param        date_from               query   string  false  "建單時間下界，ISO 8601 UTC（2026-08-01T00:00:00Z）"
// @Param        date_to                 query   string  false  "建單時間上界，ISO 8601 UTC；早於 date_from 回 9260"
// @Param        page                    query   string  false  "頁碼，預設 1"
// @Param        page_size               query   string  false  "每頁筆數，1–50，預設 20（越界回 9260，不靜默夾值）"
// @Success      200  {object}  object{code=int,message=string,data=object{records=[]handler.DelegatedOrderListItem,total=string,page=string,pageSize=string}}
// @Failure      400  {object}  object{code=int,message=string,traceId=string}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      403  {object}  object{code=int,message=string,traceId=string}
// @Failure      424  {object}  object{code=int,message=string,traceId=string}
// @Router       /admin/sellers/{store_id}/orders [get]
func (h *Holder) ListDelegatedOrders(c *gin.Context) {
	req, ok := resolveDelegatedRequest(c, false)
	if !ok {
		return
	}
	page, pageSize, ok := delegatedPaging(c)
	if !ok {
		return
	}
	keyword, ok := delegatedKeyword(c)
	if !ok {
		return
	}
	dateFrom, dateTo, ok := delegatedDateRange(c)
	if !ok {
		return
	}
	if h.Order == nil {
		denyDelegated(c, upstreamDenial())
		return
	}

	orch := h.orchestrator()
	resp, err := h.Order.GetSellerOrdersForDelegation(c.Request.Context(), &orderpb.GetSellerOrdersRequest{
		StoreId:      req.StoreID,
		StatusFilter: c.Query("status"),
		Keyword:      keyword,
		DateFrom:     dateFrom,
		DateTo:       dateTo,
		Page: &orderpb.PageRequest{
			Page: strconv.FormatInt(int64(page), 10),
			Size: strconv.FormatInt(int64(pageSize), 10),
		},
		Delegation: orch.Context(delegation.Spec{Session: req.Session}, authz.AdminCapOrdersRead, ""),
	}, h.Delegation.GrpcDeadlineDomainReadMs)
	if err != nil {
		denyDelegated(c, denialFromGrpcErr(err))
		return
	}
	if resp.GetCode() != 0 {
		if resp.GetCode() == bizcode.DelegationResourceNotOwned {
			orch.RecordReadDenial(c.Request.Context(), req.readDenial("order", "", resp.GetCode()))
		}
		denyDelegated(c, delegationDenialFromCode(resp.GetCode(), resp.GetMessage()))
		return
	}

	records := make([]DelegatedOrderListItem, 0, len(resp.GetOrders()))
	for _, o := range resp.GetOrders() {
		city, district := splitMaskedAddress(o.GetShippingAddress())
		records = append(records, DelegatedOrderListItem{
			OrderNo:             o.GetOrderNo(),
			CreatedAt:           normalizeOrderTime(o.GetCreateTime()),
			Status:              o.GetStatus(),
			StatusLabel:         codemeta.OrderStatusLabel(o.GetStatus()),
			ItemCount:           o.GetItemCount(),
			FirstProductName:    firstDelegatedProductName(o.GetItems()),
			TotalAmount:         o.GetTotalActualAmount(),
			ShippingStatus:      o.GetShippingStatus(),
			ShippingStatusLabel: o.GetShippingStatusDesc(),
			ShippingMethod:      o.GetShippingMethod(),
			DeliveryTemp:        o.GetDeliveryTemp(),
			Buyer:               DelegatedOrderBuyer{DisplayName: o.GetBuyerNickname()},
			Recipient: DelegatedOrderListRecipient{
				Name:     o.GetRecipientName(),
				City:     city,
				District: district,
			},
		})
	}
	meta := resp.GetMeta()
	response.Page(c, records, meta.GetTotalItems(), meta.GetCurrentPage(), strconv.FormatInt(int64(pageSize), 10))
}

func firstDelegatedProductName(items []*orderpb.OrderItemView) string {
	if len(items) == 0 {
		return ""
	}
	first := items[0]
	if name := strings.TrimSpace(first.GetProductName()); name != "" {
		return name
	}
	return strings.TrimSpace(first.GetSkuName())
}

// GetDelegatedOrder handles GET .../orders/{orderNo}. reveal_recipient_contact stays
// at its zero value here, and the zero value means masked.
func (h *Holder) GetDelegatedOrder(c *gin.Context) {
	req, ok := resolveDelegatedRequest(c, false)
	if !ok {
		return
	}
	orderNo := c.Param("order_no")
	orch := h.orchestrator()
	detail, denial := h.readDelegatedOrder(c.Request.Context(), req, orderNo, false,
		orch.Context(delegation.Spec{Session: req.Session}, authz.AdminCapOrdersRead, ""))
	if denial != nil {
		if denial.Code == bizcode.DelegationResourceNotOwned {
			orch.RecordReadDenial(c.Request.Context(), req.readDenial("order", orderNo, denial.Code))
		}
		denyDelegated(c, denial)
		return
	}
	response.OK(c, delegatedOrderDetailDTO(detail))
}

// RevealDelegatedRecipientContact handles GET .../orders/{orderNo}/recipient-contact.
//
// 這是唯一會回傳未遮罩收件資訊的端點，而且稽核 intent 必須先寫成功才揭露：intent
// 失敗 → 9279，不揭露。capability 就是 orders.read，沒有第二層獨立揭露授權
// （9278 已刪、是空號不得填補）。
//
// notify_seller 顯式寫 false：本端點沒有「通知賣家」勾選框，落到 DB 的 DEFAULT true
// 就會靜默通知。查看訂單是高頻讀取、通知是低頻推播，逐次推播會稀釋通知的鑑別力。
//
// 已知代價（不得美化）：本列 risk_level='high' 雖計入 UI 風險操作並出現在結束摘要與
// 稽核查詢，但那三個面都在平台側（稽核端點掛 requireAdmin()）⇒ 賣家本期看不到自己的
// 買家收件資訊被誰看過。
func (h *Holder) RevealDelegatedRecipientContact(c *gin.Context) {
	req, ok := resolveDelegatedRequest(c, false)
	if !ok {
		return
	}
	orderNo := c.Param("order_no")
	reason, ok := validateOperationReason(c, c.Query("operation_reason"))
	if !ok {
		return
	}

	orch := h.orchestrator()
	result, denial := orch.Orchestrate(c.Request.Context(), delegation.Spec{
		Session:          req.Session,
		Action:           "ORDER_PII_REVEAL",
		ResourceType:     "order",
		ReadCap:          authz.AdminCapOrdersRead,
		WriteCap:         authz.AdminCapOrdersRead,
		SnapshotFields:   nil,
		OperationReason:  reason,
		NotifySeller:     false,
		IdempotencyKey:   "",
		RequestID:        req.RequestID,
		CorrelationID:    req.CorrelationID,
		UserAgent:        req.UserAgent,
		ResourceID:       orderNo,
		ResourceName:     orderNo,
		SnapshotOverride: map[string]any{"revealedFields": []string{"phone", "fullAddress"}},
		Write: func(ctx context.Context, dctx *commonpb.DelegationContext) (*delegation.Resource, *delegation.Denial) {
			detail, denial := h.readDelegatedOrder(ctx, req, orderNo, true, dctx)
			if denial != nil {
				return nil, denial
			}
			full := detail.GetShippingAddress()
			return &delegation.Resource{
				ID:     orderNo,
				Name:   orderNo,
				Fields: map[string]any{},
				Payload: DelegatedRecipientContact{
					Name:        detail.GetRecipientName(),
					Phone:       detail.GetRecipientPhone(),
					FullAddress: full,
					ZipCode:     leadingZipCode(full),
				},
			}, nil
		},
	})
	if denial != nil {
		denyDelegated(c, denial)
		return
	}
	contact, _ := result.Resource.(DelegatedRecipientContact)
	response.OK(c, DelegatedRecipientContactData{
		AuditLogID: result.AuditLogID,
		Recipient:  contact,
	})
}

func (h *Holder) readDelegatedOrder(ctx context.Context, req *delegatedRequest, orderNo string, reveal bool, dctx *commonpb.DelegationContext) (*orderpb.OrderDetailView, *delegation.Denial) {
	if h.Order == nil {
		return nil, upstreamDenial()
	}
	resp, err := h.Order.GetSellerOrderDetailForDelegation(ctx, &orderpb.GetSellerOrderDetailRequest{
		StoreId:                req.StoreID,
		OrderNo:                orderNo,
		Delegation:             dctx,
		RevealRecipientContact: reveal,
	}, h.Delegation.GrpcDeadlineDomainReadMs)
	if err != nil {
		return nil, denialFromGrpcErr(err)
	}
	if resp.GetCode() != 0 {
		return nil, delegationDenialFromCode(resp.GetCode(), resp.GetMessage())
	}
	return resp.GetData(), nil
}

func delegatedOrderDetailDTO(d *orderpb.OrderDetailView) DelegatedOrderDetail {
	items := make([]DelegatedOrderItem, 0, len(d.GetItems()))
	for _, it := range d.GetItems() {
		items = append(items, DelegatedOrderItem{
			ProductID:       it.GetProductId(),
			ProductName:     it.GetProductName(),
			SkuID:           it.GetSkuId(),
			SkuName:         it.GetSkuName(),
			Quantity:        it.GetQuantity(),
			UnitPrice:       it.GetUnitPrice(),
			ListPrice:       it.GetListPrice(),
			ActualPayAmount: it.GetActualPayAmount(),
			ImageKey:        it.GetImageKey(),
		})
	}
	city, district := splitMaskedAddress(d.GetShippingAddress())
	return DelegatedOrderDetail{
		OrderNo:             d.GetOrderNo(),
		CreatedAt:           normalizeOrderTime(d.GetCreateTime()),
		Status:              d.GetStatus(),
		StatusLabel:         codemeta.OrderStatusLabel(d.GetStatus()),
		ShippingStatus:      d.GetShippingStatus(),
		ShippingStatusLabel: d.GetShippingStatusDesc(),
		PaymentMethod:       d.GetPaymentMethod(),
		ShippingMethod:      d.GetShippingMethod(),
		DeliveryTemp:        d.GetDeliveryTemp(),
		ItemCount:           strconv.Itoa(len(items)),
		ItemsSubtotal:       d.GetTotalOriginalAmount(),
		TotalAmount:         d.GetTotalActualAmount(),
		ShippingFee:         d.GetShippingFee(),
		DiscountAmount:      d.GetDiscountAmount(),
		CancelReason:        d.GetCancelReason(),
		CancelRemark:        d.GetCancelRemark(),
		CancelByRole:        d.GetCancelByRole(),
		Buyer:               DelegatedOrderBuyer{DisplayName: d.GetBuyerNickname()},
		Recipient: DelegatedOrderRecipient{
			Name:        d.GetRecipientName(),
			MaskedPhone: d.GetRecipientPhone(),
			City:        city,
			District:    district,
		},
		Items:        items,
		Timeline:     buildOrderTimeline(d),
		OrderRemark:  d.GetOrderRemark(),
		ShippingNote: d.GetShippingNote(),
	}
}

func buildOrderTimeline(d *orderpb.OrderDetailView) []DelegatedOrderTimelineEntry {
	entries := make([]DelegatedOrderTimelineEntry, 0, 4)
	entry := func(status, at string) DelegatedOrderTimelineEntry {
		return DelegatedOrderTimelineEntry{
			Status:      status,
			StatusLabel: codemeta.OrderStatusLabel(status),
			OccurredAt:  at,
		}
	}

	current := d.GetStatus()
	reach := happyPathReach(current)
	for i, milestone := range []struct {
		status int
		at     string
	}{
		{codemeta.OrderStatusPendingPayment, d.GetCreateTime()},
		{codemeta.OrderStatusPaid, d.GetPayTime()},
		{codemeta.OrderStatusInTransit, d.GetShipTime()},
	} {
		if reach >= 0 && i > reach {
			break
		}
		at := normalizeOrderTime(milestone.at)
		if at == "" {
			break
		}
		entries = append(entries, entry(strconv.Itoa(milestone.status), at))
	}

	if current != "" && (len(entries) == 0 || entries[len(entries)-1].Status != current) {
		entries = append(entries, entry(current, terminalStatusTime(d, current)))
	}
	return entries
}

func terminalStatusTime(d *orderpb.OrderDetailView, status string) string {
	switch status {
	case strconv.Itoa(codemeta.OrderStatusDelivered):
		return normalizeOrderTime(d.GetDeliveredAt())
	case strconv.Itoa(codemeta.OrderStatusCompleted):
		return normalizeOrderTime(d.GetCompletedAt())
	case strconv.Itoa(codemeta.OrderStatusCancelled):
		return normalizeOrderTime(d.GetCancelledAt())
	default:
		return ""
	}
}

func happyPathReach(status string) int {
	switch status {
	case strconv.Itoa(codemeta.OrderStatusPendingPayment):
		return 0
	case strconv.Itoa(codemeta.OrderStatusPaid),
		strconv.Itoa(codemeta.OrderStatusPendingShipment):
		return 1
	case strconv.Itoa(codemeta.OrderStatusInTransit),
		strconv.Itoa(codemeta.OrderStatusWaitingPickup),
		strconv.Itoa(codemeta.OrderStatusDelivered),
		strconv.Itoa(codemeta.OrderStatusCompleted):
		return 2
	default:
		return -1
	}
}

func splitMaskedAddress(masked string) (city, district string) {
	trimmed := strings.TrimSpace(masked)
	if trimmed == "" {
		return "", ""
	}
	trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, leadingZipCode(trimmed)))
	for _, candidate := range twaddr.TaiwanCities {
		if strings.HasPrefix(trimmed, candidate) {
			return candidate, strings.TrimSpace(strings.TrimPrefix(trimmed, candidate))
		}
	}
	for _, alias := range []string{"臺北市", "臺中市", "臺南市", "臺東縣"} {
		if strings.HasPrefix(trimmed, alias) {
			canonical, _ := twaddr.CanonicalCity(alias)
			return canonical, strings.TrimSpace(strings.TrimPrefix(trimmed, alias))
		}
	}
	return "", trimmed
}

func leadingZipCode(addr string) string {
	trimmed := strings.TrimSpace(addr)
	end := 0
	for end < len(trimmed) && unicode.IsDigit(rune(trimmed[end])) {
		end++
	}
	if end < 3 || end > 5 {
		return ""
	}
	return trimmed[:end]
}

func normalizeOrderTime(v string) string {
	if v == "" || v == "0" {
		return ""
	}
	if n, err := strconv.ParseInt(v, 10, 64); err == nil {
		if n <= 0 {
			return ""
		}
		return timeutil.FormatISO8601(time.UnixMilli(n).UTC().Truncate(time.Second))
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, v); err == nil {
			return timeutil.FormatISO8601(t.UTC().Truncate(time.Second))
		}
	}
	return ""
}
