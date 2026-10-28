/**
 * Order wire types — match openapi `/app/orders` shapes.
 *
 * The gateway returns very loosely-typed envelopes here (`data: object`),
 * so the field set below mirrors the buyer Flutter app's `order_model.dart`
 * pragmatically. Numbers ship as strings; convert at render edge.
 */

export type OrderStatus =
	| "pending_payment"
	| "paid"
	| "preparing"
	| "shipping"
	| "delivered"
	| "completed"
	| "cancelled"
	| "refunded"
	| "after_sale"
	| string;

export interface OrderItem {
	orderItemId: number | string;
	productId: number | string;
	skuId: string;
	name: string;
	specName?: string;
	image?: string;
	price: string;
	quantity: number | string;
	subtotal?: string;
	reviewed?: boolean;
}

export interface OrderShipping {
	receiverName?: string;
	receiverPhone?: string;
	address?: string;
	city?: string;
	district?: string;
	method?: string;
	trackingNo?: string;
	deliveredAt?: number | null;
	/** PayUni 物流序號（HOME / B2C / C2C 通用） */
	shipTradeNo?: string;
	/** 黑貓宅配 PDF 下載 URL（generate_home_label 取得） */
	labelUrl?: string;
	/** 7-11 交貨便代碼（8 碼，print_cvs_label 取得） */
	odno?: string;
	/** 7-11 C2C 驗證碼（4 碼） */
	validationNo?: string;
	/** 物流商出貨狀態（92 = 已建單 / tcatCalled = 已呼叫黑貓 等） */
	shipmentStatus?: string;
	/** 物流單號 */
	shipmentNo?: string;
	/** 寄件人姓名（顯示用） */
	senderName?: string;
	/** 寄件人電話 */
	senderPhone?: string;
	/** 取件門市代碼（7-11） */
	storeCode?: string;
	/** 取件門市名稱（7-11） */
	storeName?: string;
	/** 託運單到期時間（黑貓 PDF 過期前須出貨）。wire `labelExpireAt`。 */
	labelExpireAt?: string;
	/** 賣家出貨建單照片 URL 陣列。wire `shipmentPhotoUrls`。 */
	shipmentPhotoUrls?: string[];
	/** 買家收貨/售後回傳照片 URL 陣列。wire `receiptPhotoUrls`。 */
	receiptPhotoUrls?: string[];
}

export interface OrderPayment {
	tradeNo?: string;
	method?: string;
	status?: string;
	paidAt?: number | null;
	amount?: string;
	/** Backend's `atmBankCode` — populated when paymentMethod = "2004" (ATM)
	 * and the gateway has minted a virtual account. */
	atmBankCode?: string;
	/** Backend's `atmVirtualAccount` — the 銀行虛擬帳號 buyer transfers to.
	 * App displays this as 繳費代碼. */
	atmVirtualAccount?: string;
	/** Backend's `atmExpireDate` — ISO/date string for 繳費截止日期. */
	atmExpireDate?: string;
}

export interface OrderReview {
	/** 評分 1–5 */
	rating: number;
	/** 評論文字（可空） */
	content?: string;
	/** 評論附圖 URL */
	imageUrls?: string[];
	/** 評論時間（ISO 8601 UTC） */
	createdAt?: string;
}

/** 付款方式可用性（再次付款依此 gating）。後端 OrderDetailView.availablePaymentMethods（§3.4.1） */
export interface AvailablePaymentMethod {
	paymentType: string; // PaymentMethod value（2001/2002/2003/2004）
	eligible: boolean;
	ineligibleReason: string; // PaymentMethodIneligibleReason value；eligible=true 時空字串
}

export interface Order {
	orderNo: string;
	/** 賣家營運鍵（store surrogate）。wire `storeId`。 */
	storeId: string;
	/**
	 * 賣家店面 canonical 路徑短碼（`sellers.store_code`）。wire `storeCode`。
	 *
	 * `storeId` 是內部代理鍵，賣家頁 canonical 網址走 store_code；訂單的賣家
	 * 連結一律用這個值。order-service 為了 sellerName/logo 本來就會呼叫
	 * batchGetSellerMeta，值同源，聚合失敗時後端不送（→ undefined，不連結）。
	 */
	storeCode?: string;
	sellerName?: string;
	sellerLogo?: string;
	/** Buyer-side identity — mirrors the seller App's `OrderSummary.buyerName/
	 * buyerAvatar`. Surfaced in the seller orders table so sellers recognise
	 * who ordered without expanding the row. */
	buyerName?: string;
	buyerAvatar?: string;
	/** 買家營運鍵（buyer surrogate）。wire `buyerId`。 */
	buyerId?: string;
	status: OrderStatus;
	totalActualAmount: string;
	totalOriginalAmount?: string;
	shippingFee?: string;
	discount?: string;
	/** Platform commission (NT$ string). Set by backend per B-166 對帳明細
	 * endpoint; not present on every list payload. */
	platformFee?: string;
	/** What the seller actually receives after platform fee + shipping fee
	 * deductions. Surfaced on the detail page's fee breakdown. */
	sellerNetAmount?: string;
	createdAt: number;
	paidAt?: number | null;
	/** 付款期限（unix ms；僅 status === "pending_payment" 時有值）。
	 * 超過此時間訂單會被系統自動取消（cancelReason 9103 PAYMENT_TIMEOUT）。 */
	paymentDeadline?: number | null;
	/** 出貨/取消期限（unix ms）。後端 `processingDeadline`（App `autoCancelAt`）。
	 * status 10/20(待出貨) 時顯示為「出貨期限」，其餘顯示為「取消期限」。 */
	autoCancelAt?: number | null;
	/** 已送達後自動完成時間（unix ms）。倒數結束後後端會將訂單轉為 completed。 */
	autoCompleteAt?: number | null;
	/** 自動完成倒數暫停時間（unix ms）。售後/爭議等流程會暫停倒數。 */
	countdownPausedAt?: number | null;
	/** 取消原因 code（買家/賣家/系統共用）。 */
	cancelReason?: string;
	/** 取消發起角色，例如 BUYER / SELLER / SYSTEM。 */
	cancelByRole?: string;
	/** 取消備註或系統補充說明。 */
	cancelRemark?: string;
	items: OrderItem[];
	shipping?: OrderShipping;
	payment?: OrderPayment;
	/**
	 * ⚠️ **後端沒有這一欄**（2026-09-02 實查：`gateway/internal/handler/order.go` 的
	 * `OrderDetailResponse`／`OrderSummaryResponse` 逐欄都沒有 `canCancel`）。
	 * 留著純粹是為了讓「有人又把它當閘門」這件事看得見——**不要拿它 gate 任何按鈕**，
	 * 它恆為 `undefined` ⇒ 恆為 falsy ⇒ 那顆鍵永遠不渲染，而且不會有任何東西轉紅
	 * （#3018 就是這樣來的：文案寫著「或選擇取消」，而取消鍵從來沒出現過）。
	 * 買家能不能取消一律走 `canBuyerCancelOrder(order)`。
	 *
	 * @deprecated 後端未提供；改用 `canBuyerCancelOrder`。
	 */
	canCancel?: never;
	canConfirm?: boolean;
	canReview?: boolean;
	canRefund?: boolean;
	/** 任一商品已評論即 true（後端 isReviewed） */
	isReviewed?: boolean;
	/** 訂單評論快照（訂單層級單一，取最近一筆商品評論；未評論為 null）。後端 B-145。 */
	review?: OrderReview | null;
	/** 訂單對應 PayUni MerID（= orders.mer_id）。Google Pay 重試 tokenization 用。 */
	paymentMerId?: string;
	/** 再次付款可用付款方式（依 totalActualAmount 算；僅 pending_payment 時非空）。§3.4.1 */
	availablePaymentMethods?: AvailablePaymentMethod[];
}

export function canWriteOrderReview(order: Order): boolean {
	if (!order.canReview) return false;
	if (order.isReviewed || order.review) return false;
	return !order.items.some((item) => item.reviewed === true);
}

export interface OrderListResult {
	records: Order[];
	page: number;
	pageSize: number;
	total: number;
}

export interface CreateOrderItemInput {
	skuId: string;
	quantity: number;
	priceValue?: number;
}

/**
 * Where the order/payment originates. PayUni uses this to pick between
 * the web browser redirect URL and the mobile-app deep-link URL on
 * return. Backend defaults to "MOBILE_APP" when absent for legacy app
 * builds, so any web surface MUST set this to "WEB_BROWSER".
 */
export type PaymentClientType = "WEB_BROWSER" | "MOBILE_APP";

export interface CreateOrderInput {
	checkoutId: string;
	/** Backend `CreateOrderRequest.StoreID`, wire `storeId` (required). */
	storeId: string;
	items: CreateOrderItemInput[];
	totalActualAmount: number;
	totalOriginalAmount: number;
	addressId: number;
	shippingMethod?: string;
	paymentType?: string;
	couponCode?: string;
	usePoints?: number;
	note?: string;
	clientType?: PaymentClientType;
}

export interface CreateOrderResult {
	orderNo: string;
	tradeNo?: string;
	redirectUrl?: string;
	uppFormData?: Record<string, string>;
}

export const STATUS_LABEL: Record<string, string> = {
	pending_payment: "待付款",
	paid: "已付款",
	preparing: "待出貨",
	shipping: "運送中",
	waiting_pickup: "待取貨",
	shipping_abnormal: "配送異常",
	delivered: "已送達",
	completed: "已完成",
	partially_refunded: "部分退款",
	refunded: "已退款",
	cancelled: "已取消",
	after_sale: "退換貨中",
};
