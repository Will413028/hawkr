/**
 * Orders + Payments HTTP repository.
 *
 * Endpoints (matching hawkr_buyer order_repository.dart):
 *   GET  /app/orders?status=&page=&page_size=
 *   GET  /app/orders/{order_no}
 *   POST /app/orders                            create
 *   POST /app/orders/{order_no}/cancellations
 *   POST /app/orders/{order_no}/refund          { orderItemId, reason }
 *   POST /app/orders/{order_no}/refund/method   { method, requestNo }
 *   POST /app/payments                          create payment
 *   GET  /app/payments/{trade_no}               poll status
 *   POST /app/payments/{trade_no}/status_syncs  force gateway sync
 */

import { authedFetch } from "../api/client";
import type { RepoFetcher } from "../api/delegated";
import {
	OrderStatus as CodeMetaOrderStatus,
	type SellerCancelReasonValue,
} from "../codemeta/enums";
import type {
	CreateOrderInput,
	CreateOrderResult,
	Order,
	OrderListResult,
	PaymentClientType,
} from "./types";

interface ServerListShape {
	records?: unknown[];
	list?: unknown[];
	rows?: unknown[];
	items?: unknown[];
	page?: string | number;
	pageSize?: string | number;
	page_size?: string | number;
	currentPage?: string | number;
	current_page?: string | number;
	total?: string | number;
	totalCount?: string | number;
	total_count?: string | number;
	totalElements?: string | number;
	total_elements?: string | number;
	count?: string | number;
	pagination?: Record<string, unknown>;
	pageInfo?: Record<string, unknown>;
	page_info?: Record<string, unknown>;
	meta?: Record<string, unknown>;
}

/**
 * Map backend `status` (numeric string from codemeta `OrderStatus`) to
 * the semantic name web call sites compare against (e.g. "preparing",
 * "shipping"). Without this every `order.status === "preparing"` check
 * silently fails because backend ships `"20"` not `"preparing"`.
 *
 * Statuses kept distinct (used to drive badge tone + filter chips):
 *   - "shipping_abnormal" — 配送異常 → urgent red, surfaces in 售後處理
 *   - "waiting_pickup"    — 待取貨 (7-11) → warning amber
 *   - "partially_refunded" / "refunded" — kept separate so 部分退款 reads
 *     differently from 已退款
 *
 * `tabForOrder()` in SellerOrdersPage maps each of these back into the
 * 配送中 / 已完成 tab buckets.
 *
 * String inputs already in semantic-name form pass through unchanged so
 * test fixtures / mock data don't need to migrate.
 */
const STATUS_NAME_BY_VALUE: Record<string, string> = {
	[CodeMetaOrderStatus.PENDING_PAYMENT]: "pending_payment",
	[CodeMetaOrderStatus.PAID]: "paid",
	[CodeMetaOrderStatus.PENDING_SHIPMENT]: "preparing",
	[CodeMetaOrderStatus.IN_TRANSIT]: "shipping",
	[CodeMetaOrderStatus.WAITING_PICKUP]: "waiting_pickup",
	[CodeMetaOrderStatus.SHIPPING_ABNORMAL]: "shipping_abnormal",
	[CodeMetaOrderStatus.DELIVERED]: "delivered",
	[CodeMetaOrderStatus.COMPLETED]: "completed",
	[CodeMetaOrderStatus.PARTIALLY_REFUNDED]: "partially_refunded",
	[CodeMetaOrderStatus.FULLY_REFUNDED]: "refunded",
	[CodeMetaOrderStatus.CANCELLED]: "cancelled",
};

function normalizeStatus(raw: unknown): string {
	const code = String(raw ?? "");
	if (!code) return "pending_payment";
	return STATUS_NAME_BY_VALUE[code] ?? code;
}

/**
 * Reverse of `STATUS_NAME_BY_VALUE` — map the frontend's semantic
 * status name back to the codemeta numeric code that the backend
 * expects on `GET /app/orders?status=`. Without this the buyer
 * sidebar's tabs (`pending_payment`, `preparing`, …) were being
 * sent verbatim, so the backend SQL `WHERE status = 'pending_payment'`
 * matched nothing and the sidebar showed 「沒有符合的訂單」for every
 * tab except 「全部」.
 *
 * Already-numeric values pass through (e.g. seller tabs that already
 * pass `"20"` for any reason still work).
 */
const STATUS_VALUE_BY_NAME: Record<string, string> = Object.fromEntries(
	Object.entries(STATUS_NAME_BY_VALUE).map(([code, name]) => [name, code]),
);

export function denormalizeStatus(name: string): string {
	// CSV-safe: each comma-separated token is mapped independently.
	// `OrdersClient` passes codemeta-numeric CSVs (e.g. "10,20") that
	// fall through unchanged; single semantic names (e.g.
	// "pending_payment") get mapped to a single code. Mixed input
	// (e.g. "preparing,paid") also works.
	if (!name.includes(",")) return STATUS_VALUE_BY_NAME[name] ?? name;
	return name
		.split(",")
		.map((token) => {
			const trimmed = token.trim();
			return STATUS_VALUE_BY_NAME[trimmed] ?? trimmed;
		})
		.join(",");
}

// Backend ships order items under wildly inconsistent keys depending on
// service (orderItems / orderItemEntityList / itemList / products / …)
// and per-item fields (image vs skuImage vs spuPic vs pic vs goodsImg).
// Mirrors `OrderSummary.fromMap` + `OrderItem.fromMap` in the Flutter
// seller app so the same payload renders identically on both clients.
const ITEMS_KEY_ALIASES = [
	"items",
	"orderItemEntityList",
	"orderItems",
	"orderItemList",
	"orderItemVos",
	"itemList",
	"orderItemEntities",
	"orderItemDtos",
	"orderItemRspList",
	"orderItemResps",
	"products",
	"productList",
	"orderProducts",
	"goods",
	"goodsList",
	"details",
	"detailList",
	"orderDetails",
	"orderDetailList",
	"skuList",
	"skus",
] as const;

function pickFirst<T = unknown>(
	source: Record<string, unknown>,
	keys: readonly string[],
): T | undefined {
	for (const k of keys) {
		const v = source[k];
		if (v !== undefined && v !== null && v !== "") return v as T;
	}
	return undefined;
}

function asRecord(value: unknown): Record<string, unknown> | undefined {
	if (!value || typeof value !== "object" || Array.isArray(value))
		return undefined;
	return value as Record<string, unknown>;
}

function pickFirstRecord(
	source: Record<string, unknown>,
	keys: readonly string[],
): Record<string, unknown> | undefined {
	for (const k of keys) {
		const record = asRecord(source[k]);
		if (record) return record;
	}
	return undefined;
}

function pickFirstArray(
	source: Record<string, unknown>,
	keys: readonly string[],
): unknown[] | undefined {
	for (const k of keys) {
		const value = source[k];
		if (Array.isArray(value)) return value;
	}
	return undefined;
}

function numberOrFallback(value: unknown, fallback: number): number {
	const numeric = Number(value);
	return Number.isFinite(numeric) ? numeric : fallback;
}

function normalizeTimestamp(value: unknown): number | null | undefined {
	if (value === null) return null;
	if (value === undefined || value === "") return undefined;
	if (typeof value === "number") return value;
	if (typeof value === "string") {
		const numeric = Number(value);
		if (!Number.isNaN(numeric)) return numeric;
		const parsed = Date.parse(value);
		if (!Number.isNaN(parsed)) return parsed;
	}
	return undefined;
}

/** Coerce a loosely-typed value into a clean `string[]` (drops nullish/empty
 * entries). Returns `undefined` when there's nothing usable so the field stays
 * absent rather than an empty array. */
function normalizeStringArray(value: unknown): string[] | undefined {
	if (!Array.isArray(value)) return undefined;
	const out = value
		.filter((v): v is string => typeof v === "string" && v.trim() !== "")
		.map((v) => v.trim());
	return out.length > 0 ? out : undefined;
}

function extractItemsList(raw: Record<string, unknown>): unknown[] {
	for (const k of ITEMS_KEY_ALIASES) {
		const v = raw[k];
		if (Array.isArray(v) && v.length > 0) return v;
	}
	const nested = (raw.order ?? raw.orderInfo) as
		| Record<string, unknown>
		| undefined;
	if (nested) {
		for (const k of ITEMS_KEY_ALIASES) {
			const v = nested[k];
			if (Array.isArray(v) && v.length > 0) return v;
		}
	}
	return [];
}

function normalizeOrderItem(
	it: Record<string, unknown>,
): Order["items"][number] {
	// §231: the order item carries a flat `imageKey` (objectKey SSOT) + `imageUrl`
	// (Gateway-composed) pair — `OrderItemDetailResponse` / `OrderItemViewResponse`
	// (order.go:101-102, 204-205). The key is the only thing read; every consumer
	// feeds it through buildImageUrl/resolveCdnUrl, which accept either form.
	const image = pickFirst<string>(it, ["imageKey"]);

	const name = String(
		pickFirst(it, [
			"productName",
			"skuName",
			"name",
			"goodsName",
			"title",
			"spuName",
		]) ?? "",
	);

	// 成交「單價」快照。訂單明細/列表回 `unitPrice`、結帳預覽回 `price`，
	// 兩者都是單價。嚴禁退回 `actualPayAmount` / `realAmount` / `payAmount` /
	// `totalPrice` 這類「整列已付金額（單價 × 數量）」欄位 — UI 會再乘一次數量
	// 算小計，用整列金額當單價會重複相乘（例：1,000 的單價被當成整列 10,000，
	// 顯示成 10,000 × 10 = 100,000）。
	const price = String(
		pickFirst(it, ["unitPrice", "price", "salePrice", "originalPrice"]) ?? "0",
	);

	const quantity =
		(pickFirst(it, [
			"skuQuantity",
			"quantity",
			"count",
			"num",
			"qty",
			"buyNum",
		]) as string | number | undefined) ?? 0;

	const productId =
		(pickFirst(it, ["spuId", "productId", "goodsId", "spu_id", "spu"]) as
			| string
			| number
			| undefined) ?? "";

	const skuId = String(
		pickFirst(it, ["skuId", "sku_id", "sku", "skuCode", "skuNo"]) ?? "",
	);

	return {
		orderItemId: (it.orderItemId ?? it.id ?? "") as string | number,
		productId,
		skuId,
		name,
		specName: pickFirst<string>(it, ["specName", "spec", "skuSpec", "variant"]),
		image,
		price,
		quantity,
		subtotal: pickFirst<string>(it, ["subtotal", "totalAmount", "lineTotal"]),
		reviewed: it.reviewed === true,
	};
}

export function normalizeOrder(raw: Record<string, unknown>): Order {
	const itemsRaw = extractItemsList(raw) as Record<string, unknown>[];
	const items = itemsRaw.map(normalizeOrderItem);
	// Backend sometimes flattens receiver/shipping fields onto the order
	// root instead of nesting under `shipping`. Treat both shapes the
	// same so we always run the receiver-field fallback chain.
	const shipping =
		pickFirstRecord(raw, [
			"shipping",
			"shippingInfo",
			"shipping_info",
			"delivery",
			"deliveryInfo",
			"logistics",
			"logisticsInfo",
		]) ?? {};
	const hasShipping =
		Object.keys(shipping).length > 0 ||
		pickFirst(raw, [
			"receiverName",
			"receiver_name",
			"recipientName",
			"recipient_name",
			"consigneeName",
			"consignee_name",
			"shippingAddress",
			"shipping_address",
			"receiverAddress",
			"receiver_address",
			"recipientAddress",
			"recipient_address",
			"detailAddress",
			"detail_address",
			"recipientPhone",
			"recipient_phone",
			"receiverPhone",
			"receiver_phone",
			"buyerPhone",
			"buyer_phone",
			"shippingMethod",
			"shipMethod",
			"shipping_method",
			"deliveryType",
			"delivery_type",
			"pickupStoreName",
			"pickup_store_name",
			"storeName",
			"store_name",
			"storeId",
			"store_id",
			"shipmentPhotoKeys",
			"receiptPhotoKeys",
		]) !== undefined;
	const payment = pickFirstRecord(raw, [
		"payment",
		"paymentInfo",
		"payment_info",
		"payInfo",
		"pay_info",
	]);
	const paymentTradeNo =
		pickFirst<string>(payment ?? {}, [
			"tradeNo",
			"trade_no",
			"transactionId",
			"transaction_id",
			"payNo",
			"pay_no",
		]) ??
		pickFirst<string>(raw, [
			"tradeNo",
			"trade_no",
			"transactionId",
			"transaction_id",
			"payNo",
			"pay_no",
		]);
	const paymentMethod =
		pickFirst<string>(payment ?? {}, [
			"method",
			"paymentMethod",
			"payment_method",
			"paymentType",
			"payment_type",
			"payMethod",
			"pay_method",
		]) ??
		pickFirst<string>(raw, [
			"paymentMethod",
			"payment_method",
			"paymentType",
			"payment_type",
			"payMethod",
			"pay_method",
			"payType",
			"pay_type",
		]);
	const paymentStatus =
		pickFirst<string>(payment ?? {}, [
			"status",
			"paymentStatus",
			"payment_status",
			"payStatus",
			"pay_status",
		]) ??
		pickFirst<string>(raw, [
			"paymentStatus",
			"payment_status",
			"payStatus",
			"pay_status",
		]);
	const paymentPaidAt = normalizeTimestamp(
		pickFirst(payment ?? {}, [
			"paidAt",
			"paid_at",
			"payTime",
			"pay_time",
			"paymentTime",
			"payment_time",
		]) ??
			pickFirst(raw, [
				"paidAt",
				"paid_at",
				"payTime",
				"pay_time",
				"paymentTime",
				"payment_time",
			]),
	);
	const paymentAmount =
		pickFirst<string>(payment ?? {}, [
			"amount",
			"paymentAmount",
			"payment_amount",
			"payAmount",
			"pay_amount",
		]) ??
		pickFirst<string>(raw, [
			"paymentAmount",
			"payment_amount",
			"payAmount",
			"pay_amount",
			"totalActualAmount",
			"total_actual_amount",
		]);
	const atmBankCode = pickFirst<string>(raw, ["atmBankCode", "atm_bank_code"]);
	const atmVirtualAccount = pickFirst<string>(raw, [
		"atmVirtualAccount",
		"atm_virtual_account",
	]);
	const atmExpireDate = pickFirst<string>(raw, [
		"atmExpireDate",
		"atm_expire_date",
	]);
	const hasPayment = Boolean(
		payment ||
			paymentTradeNo ||
			paymentMethod ||
			paymentStatus ||
			paymentPaidAt !== undefined ||
			paymentAmount ||
			atmVirtualAccount ||
			atmBankCode ||
			atmExpireDate,
	);
	// Buyer-side identity — mirrors OrderSummary.fromMap (lines 552-607) in
	// the seller App. Names come from a long alias chain (商城後端蓋過幾次
	// schema 了).
	const buyerName = (pickFirst<string>(raw, [
		"buyerNickname",
		"buyer_nickname",
		"buyerName",
		"buyer_name",
		"customerName",
		"customer_name",
		"userName",
		"user_name",
		"nickname",
		"nickName",
		"nick_name",
		"realName",
		"real_name",
	]) ?? "") as string;
	// §231: seller-view order list/detail carry the flat pair `avatarKey`
	// (objectKey SSOT) + `avatarUrl` (order.go:47-48, 162-163). Consumers pass
	// this through buildImageUrl, so only the key is read.
	const buyerAvatar = (pickFirst<string>(raw, ["avatarKey"]) ?? "") as string;
	// 買家營運鍵。`/app/orders` 與 `/app/orders/{order_no}` 都只發 `buyerId`
	// （OrderSummaryResponse.BuyerID / OrderDetailResponse.BuyerID），單一來源。
	const buyerId = String(pickFirst(raw, ["buyerId"]) ?? "");
	const sellerObj =
		asRecord(raw.seller) ??
		asRecord(raw.store) ??
		asRecord(raw.shop) ??
		asRecord(raw.merchant) ??
		asRecord(raw.sellerInfo) ??
		asRecord(raw.seller_info);
	// 賣家營運鍵（store surrogate）。同上，兩支端點都只發 `storeId`。
	const storeId = String(pickFirst(raw, ["storeId"]) ?? "");
	// 賣家店面 canonical 短碼。與 sellerName/logo 同源自 order-service 的
	// batchGetSellerMeta；後端聚合失敗時不送，此處保持 undefined，由呼叫端決定
	// 不渲染連結（不回退 storeId — storeId 路徑會多吃一次 308）。
	const storeCode =
		typeof raw.storeCode === "string"
			? raw.storeCode.trim() || undefined
			: undefined;
	const sellerName =
		pickFirst<string>(raw, [
			"sellerName",
			"seller_name",
			"storeName",
			"store_name",
			"shopName",
			"shop_name",
			"merchantName",
			"merchant_name",
		]) ??
		pickFirst<string>(sellerObj ?? {}, [
			"name",
			"sellerName",
			"seller_name",
			"storeName",
			"store_name",
			"shopName",
			"shop_name",
		]);
	// B-238: order list/detail expose one canonical seller-logo URL. Do not
	// recover deprecated sellerAvatarUrl/seller_logo aliases or nested fields.
	const sellerLogo =
		typeof raw.logoUrl === "string"
			? raw.logoUrl.trim() || undefined
			: undefined;

	const reviewRecord = pickFirstRecord(raw, ["review"]);
	const review = reviewRecord
		? {
				rating: Number(
					pickFirst<string | number>(reviewRecord, ["rating"]) ?? 0,
				),
				content: pickFirst<string>(reviewRecord, ["content"]) ?? "",
				imageUrls: Array.isArray(reviewRecord.imageUrls)
					? (reviewRecord.imageUrls as string[])
					: [],
				createdAt: pickFirst<string>(reviewRecord, ["createdAt", "created_at"]),
			}
		: null;

	return {
		orderNo: String(raw.orderNo ?? raw.order_no ?? ""),
		storeId,
		storeCode,
		sellerName,
		sellerLogo,
		buyerName: buyerName || undefined,
		buyerAvatar: buyerAvatar || undefined,
		buyerId: buyerId || undefined,
		status: normalizeStatus(raw.status),
		totalActualAmount: String(
			pickFirst(raw, [
				"totalActualAmount",
				"total_actual_amount",
				"totalAmount",
				"total_amount",
				"payAmount",
				"pay_amount",
				"paymentAmount",
				"payment_amount",
				"serverPayAmount",
				"orderAmount",
			]) ?? "0",
		),
		totalOriginalAmount: pickFirst<string>(raw, [
			"totalOriginalAmount",
			"total_original_amount",
			"originalAmount",
			"original_amount",
		]),
		shippingFee: (pickFirst(raw, [
			"freightAmount",
			"freight_amount",
			"shippingFee",
			"shipping_fee",
			"deliveryFee",
			"delivery_fee",
			"postage",
			"freight",
			"fare",
			"shipping",
			"logisticsFee",
		]) ?? undefined) as string | undefined,
		discount: (pickFirst(raw, [
			"discountAmount",
			"discount_amount",
			"couponAmount",
			"coupon_amount",
			"promotionAmount",
			"promotion_amount",
			"reduceAmount",
			"reduce_amount",
			"discountTotal",
			"discount_total",
			"discount",
		]) ?? undefined) as string | undefined,
		platformFee: (pickFirst(raw, [
			"platformFee",
			"platform_fee",
			"commission",
			"commissionAmount",
			"commission_amount",
		]) ?? undefined) as string | undefined,
		sellerNetAmount: (pickFirst(raw, [
			"sellerNetAmount",
			"seller_net_amount",
			"sellerSettleAmount",
			"seller_settle_amount",
			"merchantAmount",
			"merchant_amount",
		]) ?? undefined) as string | undefined,
		createdAt:
			normalizeTimestamp(
				pickFirst(raw, [
					"createdAt",
					"created_at",
					"createTime",
					"create_time",
				]),
			) ?? 0,
		paidAt: normalizeTimestamp(
			pickFirst(raw, [
				"paidAt",
				"paid_at",
				"payTime",
				"pay_time",
				"paymentTime",
				"payment_time",
			]),
		),
		paymentDeadline: normalizeTimestamp(
			pickFirst(raw, [
				"paymentDeadline",
				"payment_deadline",
				"payDeadline",
				"pay_deadline",
				"paymentExpireAt",
				"payment_expire_at",
				"payExpireAt",
				"pay_expire_at",
			]) ??
				pickFirst(payment ?? {}, [
					"paymentDeadline",
					"payment_deadline",
					"expireAt",
					"expire_at",
				]),
		),
		// 出貨/取消期限 — App OrderSummary 用 `autoCancelAt`，後端實際送
		// `processingDeadline`（OrderSummaryResponse / OrderDetailResponse 皆有）。
		// 同一條 fallback 鏈對齊 App order_model.dart。
		autoCancelAt: normalizeTimestamp(
			pickFirst(raw, [
				"autoCancelAt",
				"auto_cancel_at",
				"processingDeadline",
				"processing_deadline",
			]),
		),
		autoCompleteAt: normalizeTimestamp(
			pickFirst(raw, [
				"autoCompleteAt",
				"auto_complete_at",
				"completionDeadline",
				"completion_deadline",
			]),
		),
		countdownPausedAt: normalizeTimestamp(
			pickFirst(raw, [
				"countdownPausedAt",
				"countdown_paused_at",
				"autoCompletePausedAt",
				"auto_complete_paused_at",
			]),
		),
		cancelReason: pickFirst<string>(raw, [
			"cancelReason",
			"cancel_reason",
			"cancellationReason",
			"cancellation_reason",
		]),
		cancelByRole: pickFirst<string>(raw, [
			"cancelByRole",
			"cancel_by_role",
			"cancelRole",
			"cancel_role",
			"cancelledByRole",
			"cancelled_by_role",
		]),
		cancelRemark: pickFirst<string>(raw, [
			"cancelRemark",
			"cancel_remark",
			"cancelNote",
			"cancel_note",
			"cancellationRemark",
			"cancellation_remark",
		]),
		items,
		shipping: hasShipping
			? {
					receiverName: (pickFirst<string>(shipping, [
						"receiverName",
						"receiver_name",
						"recipientName",
						"recipient_name",
						"consigneeName",
						"consignee_name",
						"name",
					]) ??
						pickFirst<string>(raw, [
							"consigneeName",
							"consignee_name",
							"receiverName",
							"receiver_name",
							"recipientName",
							"recipient_name",
						])) as string | undefined,
					receiverPhone: (pickFirst<string>(shipping, [
						"receiverPhone",
						"receiver_phone",
						"phone",
						"mobile",
						"tel",
						"contactPhone",
						"contact_phone",
					]) ??
						pickFirst<string>(raw, [
							"recipientPhone",
							"recipient_phone",
							"phone",
							"mobile",
							"buyerPhone",
							"buyer_phone",
							"receiverPhone",
							"receiver_phone",
						])) as string | undefined,
					address:
						pickFirst<string>(shipping, [
							"address",
							"shippingAddress",
							"shipping_address",
							"receiverAddress",
							"receiver_address",
							"recipientAddress",
							"recipient_address",
							"detailAddress",
							"detail_address",
							"fullAddress",
							"full_address",
						]) ??
						pickFirst<string>(raw, [
							"shippingAddress",
							"shipping_address",
							"receiverAddress",
							"receiver_address",
							"recipientAddress",
							"recipient_address",
							"detailAddress",
							"detail_address",
							"fullAddress",
							"full_address",
						]),
					city:
						pickFirst<string>(shipping, ["city", "county", "countyName"]) ??
						pickFirst<string>(raw, ["city", "county", "countyName"]),
					district:
						pickFirst<string>(shipping, [
							"district",
							"districtName",
							"township",
							"area",
						]) ??
						pickFirst<string>(raw, [
							"district",
							"districtName",
							"township",
							"area",
						]),
					method:
						pickFirst<string>(shipping, [
							"method",
							"shippingMethod",
							"shipping_method",
							"shipMethod",
							"ship_method",
							"deliveryType",
							"delivery_type",
						]) ??
						pickFirst<string>(raw, [
							"shippingMethod",
							"shipping_method",
							"shipMethod",
							"ship_method",
							"deliveryType",
							"delivery_type",
						]),
					trackingNo:
						pickFirst<string>(shipping, [
							"trackingNo",
							"tracking_no",
							"shipmentNo",
							"shipment_no",
						]) ??
						pickFirst<string>(raw, [
							"trackingNo",
							"tracking_no",
							"shipmentNo",
							"shipment_no",
						]),
					deliveredAt: normalizeTimestamp(
						pickFirst(shipping, [
							"deliveredAt",
							"delivered_at",
							"receiptTime",
							"receipt_time",
						]) ??
							pickFirst(raw, [
								"deliveredAt",
								"delivered_at",
								"receiptTime",
								"receipt_time",
							]),
					),
					shipTradeNo:
						pickFirst<string>(shipping, ["shipTradeNo", "ship_trade_no"]) ??
						pickFirst<string>(raw, ["shipTradeNo", "ship_trade_no"]),
					labelUrl:
						pickFirst<string>(shipping, ["labelUrl", "label_url"]) ??
						pickFirst<string>(raw, ["labelUrl", "label_url"]),
					odno:
						pickFirst<string>(shipping, ["odno", "ODNO"]) ??
						pickFirst<string>(raw, ["odno", "ODNO"]),
					validationNo:
						pickFirst<string>(shipping, ["validationNo", "validation_no"]) ??
						pickFirst<string>(raw, ["validationNo", "validation_no"]),
					shipmentStatus:
						pickFirst<string>(shipping, [
							"shipmentStatus",
							"shipment_status",
							"shippingStatus",
							"shipping_status",
						]) ??
						pickFirst<string>(raw, [
							"shipmentStatus",
							"shipment_status",
							"shippingStatus",
							"shipping_status",
						]),
					shipmentNo:
						pickFirst<string>(shipping, ["shipmentNo", "shipment_no"]) ??
						pickFirst<string>(raw, ["shipmentNo", "shipment_no"]),
					senderName:
						pickFirst<string>(shipping, ["senderName", "sender_name"]) ??
						pickFirst<string>(raw, ["senderName", "sender_name"]),
					senderPhone:
						pickFirst<string>(shipping, ["senderPhone", "sender_phone"]) ??
						pickFirst<string>(raw, ["senderPhone", "sender_phone"]),
					storeCode:
						pickFirst<string>(shipping, [
							"storeCode",
							"store_code",
							"storeId",
							"store_id",
						]) ??
						pickFirst<string>(raw, [
							"storeCode",
							"store_code",
							"storeId",
							"store_id",
						]),
					storeName:
						pickFirst<string>(shipping, [
							"storeName",
							"store_name",
							"pickupStoreName",
							"pickup_store_name",
						]) ??
						pickFirst<string>(raw, [
							"storeName",
							"store_name",
							"pickupStoreName",
							"pickup_store_name",
						]),
					deliveryTemp:
						pickFirst<string>(shipping, ["deliveryTemp", "delivery_temp"]) ??
						pickFirst<string>(raw, ["deliveryTemp", "delivery_temp"]),
					labelExpireAt:
						pickFirst<string>(shipping, ["labelExpireAt", "label_expire_at"]) ??
						pickFirst<string>(raw, ["labelExpireAt", "label_expire_at"]),
					// §231: `shipmentPhotoKeys` / `receiptPhotoKeys` are the objectKey
					// SSOT; the `*Urls` twins are the Gateway-composed display URLs
					// (order.go:75-78). Consumers run these through buildImageUrl.
					shipmentPhotoUrls: normalizeStringArray(
						pickFirst(shipping, ["shipmentPhotoKeys"]) ??
							pickFirst(raw, ["shipmentPhotoKeys"]),
					),
					receiptPhotoUrls: normalizeStringArray(
						pickFirst(shipping, ["receiptPhotoKeys"]) ??
							pickFirst(raw, ["receiptPhotoKeys"]),
					),
				}
			: undefined,
		payment: hasPayment
			? {
					tradeNo: paymentTradeNo,
					method: paymentMethod,
					status: paymentStatus,
					paidAt: paymentPaidAt,
					amount: paymentAmount,
					atmBankCode,
					atmVirtualAccount,
					atmExpireDate,
				}
			: undefined,
		// `canCancel` **不 normalize**：後端根本不送這一欄（見 types.ts 的說明）。
		// 讀一個永遠不存在的欄位並不會報錯，只會讓每一顆掛在它身上的按鈕靜默消失。
		canConfirm: raw.canConfirm === true,
		canReview: raw.canReview === true,
		canRefund: raw.canRefund === true,
		isReviewed: raw.isReviewed === true,
		review,
		paymentMerId:
			typeof raw.paymentMerId === "string" ? raw.paymentMerId : undefined,
		availablePaymentMethods: Array.isArray(raw.availablePaymentMethods)
			? raw.availablePaymentMethods.map((m) => {
					const o = (m ?? {}) as Record<string, unknown>;
					return {
						paymentType: String(o.paymentType ?? ""),
						eligible: Boolean(o.eligible),
						ineligibleReason: String(o.ineligibleReason ?? ""),
					};
				})
			: undefined,
	};
}

export interface FetchOrdersParams {
	status?: string;
	page?: number;
	pageSize?: number;
	buyerId?: string;
	/**
	 * Order perspective. Omit (default) for the BUYER view — the account's own
	 * purchases. Pass "seller" for the SELLER view — orders placed against this
	 * account's store (i.e. the fulfilment queue). Backed by `/app/orders`'s
	 * `view` query param (OpenAPI: 空值=買家, seller=賣家). Without it, the
	 * seller order page wrongly shows the account's purchases.
	 */
	view?: "seller";
	/** Forwarded from the TanStack queryFn so a status-tab flip aborts the
	 *  in-flight request instead of leaking it. */
	signal?: AbortSignal;
}

/**
 * 訂單資料層的介面（56 號 §4.5 L1053；設計書 L1053 漏列了這一支）。
 *
 * ⚠️ 代行的訂單端點**沒有 `view` query**（店家由路徑上的 storeId 定界），而代行綁的
 * 就是本檔的 `createOrderRepo`（見 `lib/seller-data/delegated-repos.ts` 的 `orders:`），
 * 它在 `fetchOrdersWith` 裡照樣會 `qs.set("view", view)`。
 *
 * 也就是說：**代行實作並沒有忽略 `view`**——一旦有呼叫端在代行下傳入它，請求會帶著
 * 一個不在 BFF 白名單（`page` / `page_size` / `status`，見
 * `app/(delegated)/adm/api/orders/[storeId]/route.ts`）內的 key 送出，被
 * `buildQuery` 判 `INVALID_QUERY` 回 400，一列都不轉送。
 *
 * 今天不會發生，是因為代行工作區掛載的頁面都到不了 `scope.repos.orders`
 * （代行訂單面走的是另一支 `features/delegated-operations/orders/repo.ts` 的
 * `createDelegatedOrderRepo`，那支才真的不送 `view`，且刻意不實作本介面）。
 * 這是「目前不可達」而不是「已經處理好」——`lib/hooks/useOrders.ts` 有三處把 `view`
 * 直送 `scope.repos.orders` 且沒有 mode 守門，任何一處被代行頁面接上就會現形。
 *
 * `normalizeOrder` 兩邊共用。
 */
export interface OrderRepo {
	fetchOrders(params?: FetchOrdersParams): Promise<OrderListResult>;
	fetchOrder(orderNo: string, opts?: { view?: "seller" }): Promise<Order>;
	fetchOrderStatusCounts(params?: {
		signal?: AbortSignal;
	}): Promise<OrderStatusCountsResult>;
}

export interface OrderRepoOptions {
	/** 資源根。賣家／買家 `/app/orders`；代行的 BFF 路徑另由代行工廠決定。 */
	root?: string;
}

export function createOrderRepo(
	fetcher: RepoFetcher,
	options: OrderRepoOptions = {},
): OrderRepo {
	const root = options.root ?? "/app/orders";
	return {
		fetchOrders: (params) => fetchOrdersWith(fetcher, root, params),
		fetchOrder: (orderNo, opts) => fetchOrderWith(fetcher, root, orderNo, opts),
		fetchOrderStatusCounts: (params) =>
			fetchOrderStatusCountsWith(fetcher, root, params),
	};
}

/** 買家與賣家兩條路徑共用的預設實例（同一支 `/app/orders`，由 JWT 定界）。 */
export const orderRepo = createOrderRepo(authedFetch);

async function fetchOrdersWith(
	fetcher: RepoFetcher,
	root: string,
	{
		status,
		page = 1,
		pageSize = 20,
		buyerId,
		view,
		signal,
	}: FetchOrdersParams = {},
): Promise<OrderListResult> {
	const qs = new URLSearchParams({
		page: String(page),
		page_size: String(pageSize),
	});
	if (status) qs.set("status", denormalizeStatus(status));
	if (view) qs.set("view", view);
	// query key 用 buyer_id、參數名用 buyerId（44 號身分分離：買家業務鍵是
	// buyers.buyer_id，不是登入 principal 的 account_id；命名統一來自 dev 的 aa83fb4e）。
	// fetcher/root 是代行資料層的注入點：代行下要走 /adm/api 的 BFF，不能寫死
	// authedFetch 與 /app/orders。兩者正交，都要保留。
	if (buyerId) qs.set("buyer_id", buyerId);
	const env = await fetcher<ServerListShape | unknown[]>(`${root}?${qs}`, {
		signal,
	});
	const data = env.data;
	const dataRecord = asRecord(data);
	const pagination = dataRecord
		? pickFirstRecord(dataRecord, [
				"pagination",
				"pageInfo",
				"page_info",
				"meta",
			])
		: undefined;
	const rawList = Array.isArray(data)
		? data
		: dataRecord
			? (pickFirstArray(dataRecord, ["records", "list", "rows", "items"]) ?? [])
			: [];
	const list = rawList as Record<string, unknown>[];
	const resolvedPage = dataRecord
		? (pickFirst(dataRecord, ["page", "currentPage", "current_page"]) ??
			(pagination
				? pickFirst(pagination, ["page", "currentPage", "current_page"])
				: undefined))
		: undefined;
	const resolvedPageSize = dataRecord
		? (pickFirst(dataRecord, ["pageSize", "page_size", "size", "limit"]) ??
			(pagination
				? pickFirst(pagination, ["pageSize", "page_size", "size", "limit"])
				: undefined))
		: undefined;
	const resolvedTotal = dataRecord
		? (pickFirst(dataRecord, [
				"total",
				"totalCount",
				"total_count",
				"totalElements",
				"total_elements",
				"count",
			]) ??
			(pagination
				? pickFirst(pagination, [
						"total",
						"totalCount",
						"total_count",
						"totalElements",
						"total_elements",
						"count",
					])
				: undefined))
		: undefined;
	return {
		records: list.map(normalizeOrder),
		page: numberOrFallback(resolvedPage, page),
		pageSize: numberOrFallback(resolvedPageSize, pageSize),
		total: numberOrFallback(resolvedTotal, list.length),
	};
}

export const fetchOrders: OrderRepo["fetchOrders"] = (params) =>
	orderRepo.fetchOrders(params);

/** One raw `OrderStatus` bucket → its order count, plus the grand total. */
export interface OrderStatusCountsResult {
	/** codemeta `OrderStatus` numeric code (e.g. "50") → count. Backend omits
	 *  zero-count buckets, so absent keys mean 0. */
	countsByStatus: Record<string, number>;
	/** Total orders across every status (i.e. the "全部" bucket). */
	total: number;
}

interface StatusCountsWire {
	counts?: Array<{ status?: string | number; count?: string | number }>;
	total?: string | number;
}

/**
 * Buyer-only per-status order rollup. `GET /app/orders/status_counts` returns
 * every non-zero `OrderStatus` bucket's count (numeric strings) plus the total
 * in a single JWT-scoped request. This replaces the old N parallel
 * `page_size=1` probes (one per status tab) that `useOrderStatusCounts` used to
 * fire — see Sentry N+1 DEV-3. The endpoint takes no query params at all: the
 * handler reads only `view` (to 400-reject the seller view) and derives the
 * buyer from the JWT session, so the request is sent bare.
 */
export const fetchOrderStatusCounts: OrderRepo["fetchOrderStatusCounts"] = (
	params,
) => orderRepo.fetchOrderStatusCounts(params);

async function fetchOrderStatusCountsWith(
	fetcher: RepoFetcher,
	root: string,
	params: { signal?: AbortSignal } = {},
): Promise<OrderStatusCountsResult> {
	const { signal } = params;
	const env = await fetcher<StatusCountsWire>(`${root}/status_counts`, {
		signal,
	});
	const wire = env.data ?? {};
	const countsByStatus: Record<string, number> = {};
	for (const row of wire.counts ?? []) {
		const code = String(row.status ?? "");
		if (!code) continue;
		countsByStatus[code] = numberOrFallback(row.count, 0);
	}
	return { countsByStatus, total: numberOrFallback(wire.total, 0) };
}

/**
 * Fetch a single order. Defaults to the BUYER view (the account's own
 * purchase). Pass `view: "seller"` for the seller-center detail page —
 * without it `/app/orders/{order_no}` is scoped to the buyer and the backend
 * returns 無權操作 when a seller opens an order placed against their store.
 * Mirrors the same `view` switch already used by `fetchOrders`.
 */
export const fetchOrder: OrderRepo["fetchOrder"] = (orderNo, opts) =>
	orderRepo.fetchOrder(orderNo, opts);

async function fetchOrderWith(
	fetcher: RepoFetcher,
	root: string,
	orderNo: string,
	opts?: { view?: "seller" },
): Promise<Order> {
	const qs = opts?.view ? `?view=${opts.view}` : "";
	const env = await fetcher<Record<string, unknown>>(
		`${root}/${encodeURIComponent(orderNo)}${qs}`,
	);
	return normalizeOrder(env.data ?? {});
}

export async function createOrder(
	input: CreateOrderInput,
): Promise<CreateOrderResult> {
	const env = await authedFetch<Record<string, unknown>>("/app/orders", {
		method: "POST",
		json: input,
	});
	const data = env.data ?? {};
	return {
		orderNo: String(data.orderNo ?? data.order_no ?? ""),
		tradeNo: data.tradeNo as string | undefined,
		redirectUrl: data.redirectUrl as string | undefined,
		uppFormData: data.uppFormData as Record<string, string> | undefined,
	};
}

/**
 * 買家能不能取消這張訂單。
 *
 * 🔴 判準是 **`status === "pending_payment"`**，逐字對應 order-service 的
 * `OrderService.cancelOrder`：它只接受 `PENDING_PAYMENT`，其餘一律
 * `InvalidStatusException("僅待付款訂單可取消")`。
 *
 * ⚠️ **不要改回讀後端的 `canCancel`**：gateway 的訂單 DTO 沒有那一欄（實查
 * `handler/order.go`），讀它恆得 `undefined`。#3018 的實際症狀就是這個——
 * 訂單頁的文案寫著「或選擇取消」，而那顆鍵因為閘門恆為 falsy 從來沒有出現過，
 * 全程零錯誤、零測試轉紅。哪天後端真的補了那一欄，改這裡一個地方就好。
 */
export function canBuyerCancelOrder(
	order: Pick<Order, "status"> | null | undefined,
): boolean {
	return order?.status === "pending_payment";
}

export async function cancelOrder(orderNo: string): Promise<boolean> {
	const env = await authedFetch<{ success?: boolean }>(
		`/app/orders/${encodeURIComponent(orderNo)}/cancellations`,
		{ method: "POST" },
	);
	return env.data?.success === true || env.code === 0;
}

export interface SellerCancelResult {
	orderNo: string;
	refundAmount: string;
	/** Buyer must visit the PayUni ATM refund page to receive the refund. */
	needRefundPage: boolean;
}

/**
 * Seller cancels a paid-but-unshipped order (待出貨). Mirrors the seller APP's
 * `order_repository.cancelShipment()` → POST /app/orders/{order_no}/seller_cancellations.
 * Backend only accepts status 10(已付款)/20(待出貨) and auto-starts the refund.
 */
export async function cancelSellerOrder(
	orderNo: string,
	args: { reason: SellerCancelReasonValue; remark?: string },
): Promise<SellerCancelResult> {
	const body: Record<string, unknown> = { reason: args.reason };
	if (args.remark?.trim()) body.remark = args.remark.trim();
	const env = await authedFetch<Record<string, unknown>>(
		`/app/orders/${encodeURIComponent(orderNo)}/seller_cancellations`,
		{ method: "POST", json: body },
	);
	const data = env.data ?? {};
	return {
		orderNo: String(data.orderNo ?? orderNo),
		refundAmount: String(data.refundAmount ?? ""),
		needRefundPage: Boolean(data.needRefundPage),
	};
}

/**
 * Buyer confirms receipt of a delivered order. Endpoint may not be deployed
 * yet on the backend — caller treats a 404 the same as success and refreshes
 * (status flips automatically when logistics says "delivered").
 */
export async function confirmOrderReceipt(orderNo: string): Promise<boolean> {
	const env = await authedFetch<{ success?: boolean }>(
		`/app/orders/${encodeURIComponent(orderNo)}/confirm_receipt`,
		{ method: "POST", throwOnHttpError: false },
	);
	return env.data?.success === true || env.code === 0;
}

/* ===== Payments ===== */

export interface CreatePaymentInput {
	orderNo: string;
	amount: number;
	productName: string;
	paymentType: string; // CREDIT | ATM | LINEPAY | …
	userEmail?: string;
	bankType?: string;
	expireDays?: number;
	merId?: string;
	clientType?: PaymentClientType;
	/** Native Apple Pay token (base64-encoded `PKPaymentToken` JSON). When
	 * supplied the backend processes the payment synchronously without
	 * issuing a PayUni redirect — same contract as the App. */
	applePayToken?: string;
	/** Native Google Pay token (base64-encoded
	 * `PaymentData.paymentMethodData.tokenizationData.token`). */
	googlePayToken?: string;
}

export interface CreatePaymentResult {
	tradeNo: string;
	redirectUrl?: string;
	uppFormData?: Record<string, string>;
	status?: string;
}

export async function createPayment(
	input: CreatePaymentInput,
): Promise<CreatePaymentResult> {
	const env = await authedFetch<Record<string, unknown>>("/app/payments", {
		method: "POST",
		json: input,
	});
	const data = env.data ?? {};
	return {
		tradeNo: String(
			pickFirst(data, [
				"tradeNo",
				"trade_no",
				"paymentTradeNo",
				"payment_trade_no",
				"providerTradeNo",
				"provider_trade_no",
			]) ?? "",
		),
		redirectUrl: pickFirst<string>(data, [
			"redirectUrl",
			"redirect_url",
			"paymentUrl",
			"payment_url",
		]),
		uppFormData: pickFirst<Record<string, string>>(data, [
			"uppFormData",
			"upp_form_data",
		]),
		status: pickFirst<string>(data, [
			"status",
			"paymentStatus",
			"payment_status",
		]),
	};
}

export async function fetchPayment(tradeNo: string): Promise<{
	status: string;
	paidAt?: number | null;
	amount?: string;
}> {
	const env = await authedFetch<Record<string, unknown>>(
		`/app/payments/${encodeURIComponent(tradeNo)}`,
	);
	const data = env.data ?? {};
	return {
		status: String(data.status ?? "unknown"),
		paidAt: data.paidAt as number | null | undefined,
		amount: data.amount as string | undefined,
	};
}

export async function syncPayment(
	tradeNo: string,
): Promise<{ status: string }> {
	const env = await authedFetch<Record<string, unknown>>(
		`/app/payments/${encodeURIComponent(tradeNo)}/status_syncs`,
		{ method: "POST" },
	);
	return { status: String(env.data?.status ?? "unknown") };
}
