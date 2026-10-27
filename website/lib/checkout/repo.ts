/**
 * Checkout HTTP repository.
 *
 *   POST /app/checkout/validate     — 結帳即時驗證
 *     Validates the buyer's current selection (skuId+productId+qty) and
 *     returns the cross-seller delivery-temp intersection plus the shipping
 *     options that are valid for the whole basket.
 *
 *   POST /app/checkout/shipping_fee — 運費精算 (used on checkout page, not cart)
 *   POST /app/checkout/preview      — 結帳預覽，取得 checkoutId 與精算金額
 *   POST /app/checkout/confirm      — 以 checkoutId 確認下單
 *
 * The cart page calls `validateCheckoutItems()` so the shipping-method picker
 * mirrors what the buyer will actually see at /checkout — no UI surprise when
 * a method the cart showed turns out to be unavailable for cold-chain items.
 */

import { authedFetch } from "../api/client";

export interface CheckoutItemInput {
	skuId: string;
	productId: string | number | undefined;
	quantity: number | string;
}

export interface CheckoutPreviewRequest {
	items: CheckoutItemInput[];
	shippingMethod: string;
	deliveryTemp: string;
	receiverZipCode?: string;
	storeId?: string;
	storeName?: string;
	couponTemplateId?: string;
	/**
	 * 是否為和單（多賣家代理商代收）。default false=單賣家走 seller mer；
	 * true=後端取 env PAYUNI_AGENT_MER_ID。目前 Phase 1 單賣家 cart 一律 false。
	 */
	consolidated?: boolean;
}

export interface CheckoutPreviewResult {
	checkoutId: string;
	subtotal: number;
	/**
	 * Null when the backend preview response omits every shipping-fee
	 * field. The previous `number` default of 0 made "field missing"
	 * indistinguishable from "free shipping", so the UI always showed
	 * 「免運」even when it should have fallen back to the carrier estimate
	 * from `/app/checkout/shipping_fee` or the option's `fee`.
	 */
	shippingFee: number | null;
	discountAmount: number;
	totalPayAmount: number;
	/**
	 * 本次交易付款對應的 PayUni MerID（單賣家=seller.mer / 和單=代理商 mer）。
	 * Google Pay 在 `loadPaymentData()` 開 sheet 前必須用此值填入
	 * `tokenizationSpec.parameters.gatewayMerchantId`。
	 */
	paymentMerId: string;
	/**
	 * 後端依 totalActualAmount 算出的各付款方式可用性（20 設計書 §3.4.1）。
	 * 前端據此 render（禁用 + 理由），不自行用金額判斷。空清單 = 零元（無需付款）。
	 */
	availablePaymentMethods: AvailablePaymentMethod[];
	/**
	 * 結帳頁的發票區（58 §6.2.4 order ②、58A §B01）。
	 *
	 * 🔴 `null` 與「空物件」是兩件事：後端拿不到發票服務時**不回這個 key**，
	 * 此時整個發票區不渲染。渲染一張空卡片會讓買家以為系統壞了。
	 */
	invoice: CheckoutInvoicePreview | null;
	raw: Record<string, unknown>;
}

/** 結帳頁發票區的資料（58A §B01）。逐欄取自後端，前端不做任何推導。 */
export interface CheckoutInvoicePreview {
	/**
	 * InvoiceFulfillmentMode 四值之一。
	 * 🔴 四值都必須有分支——缺一即落 invalid 而擋付款（58A §B01）。
	 */
	invoiceMode: string;
	/** 由 order-service 給（I-50：它是唯一寫入者）；confirm 時原樣回送 */
	snapshotVersion: string;
	/** ISO 8601 UTC；**前端不得自算 TTL** */
	previewExpiresAt: string;
	amount: CheckoutInvoiceAmount | null;
	disclosure: CheckoutInvoiceDisclosure | null;
	defaultProfile: CheckoutInvoiceProfile | null;
	intakeLimit: CheckoutInvoiceIntakeLimit | null;
}

/**
 * 發票金額拆解。
 * ⚠️ **不得**拿 `totalPayAmount` 充當 `invoiceAmount`——兩者在 Phase 1 相等是業務決策
 * 不是恆等式（58A §B01 逐字）。
 */
export interface CheckoutInvoiceAmount {
	productAmount: string;
	shippingAmount: string;
	/** 正數；畫面顯示為 −NT$ X */
	discountAmount: string;
	invoiceAmount: string;
	taxCategory: string;
}

/**
 * 法定告知。
 * ⚠️ `required` 自 2026-08-31（DEC-20260831-210934）起**只決定要不要顯示**，
 * 不再是付款前置條件——沒有 checkbox、不擋付款。
 */
export interface CheckoutInvoiceDisclosure {
	required: boolean;
	termsVersion: string;
	title: string;
	body: string;
}

/** 買家的預設常用發票資料。`displayValue` 是**遮罩後**的字串，前端永遠拿不到明碼。 */
export interface CheckoutInvoiceProfile {
	profileId: string;
	carrierType: string;
	displayValue: string;
	buyerTaxId?: string;
	buyerName?: string;
	isDefault: boolean;
	label?: string;
}

/** 賣家待審期間的接單額度；`blocked=true` ⇒ 顯示不可下單 banner（9326）。 */
export interface CheckoutInvoiceIntakeLimit {
	orderCountUsed: string;
	orderCountLimit: string;
	amountUsed: string;
	amountLimit: string;
	blocked: boolean;
}

/**
 * 從後端 data 解析 `invoice`。缺這個 key ⇒ 回 `null`（不渲染發票區）。
 *
 * 🔴 只認 camelCase：Gateway 的 JSON 契約規定 response 一律 camel（API 設計規範 §2），
 * 這裡再吃一次 snake 等於默許契約漂移，而漂移的那一天不會有任何訊號。
 */
export function parseCheckoutInvoicePreview(
	data: Record<string, unknown>,
): CheckoutInvoicePreview | null {
	const raw = data["invoice"];
	if (!raw || typeof raw !== "object") return null;
	const o = raw as Record<string, unknown>;
	const str = (k: string) => (typeof o[k] === "string" ? (o[k] as string) : "");
	const sub = <T>(
		k: string,
		pick: (v: Record<string, unknown>) => T,
	): T | null => {
		const v = o[k];
		return v && typeof v === "object"
			? pick(v as Record<string, unknown>)
			: null;
	};
	const s = (v: Record<string, unknown>, k: string) =>
		typeof v[k] === "string" ? (v[k] as string) : "";
	return {
		invoiceMode: str("invoiceMode"),
		snapshotVersion: str("snapshotVersion"),
		previewExpiresAt: str("previewExpiresAt"),
		amount: sub("amount", (v) => ({
			productAmount: s(v, "productAmount"),
			shippingAmount: s(v, "shippingAmount"),
			discountAmount: s(v, "discountAmount"),
			invoiceAmount: s(v, "invoiceAmount"),
			taxCategory: s(v, "taxCategory"),
		})),
		disclosure: sub("disclosure", (v) => ({
			required: v["required"] === true,
			termsVersion: s(v, "termsVersion"),
			title: s(v, "title"),
			body: s(v, "body"),
		})),
		defaultProfile: sub("defaultProfile", (v) => ({
			profileId: s(v, "profileId"),
			carrierType: s(v, "carrierType"),
			displayValue: s(v, "displayValue"),
			buyerTaxId: s(v, "buyerTaxId"),
			buyerName: s(v, "buyerName"),
			isDefault: v["isDefault"] === true,
			label: s(v, "label"),
		})),
		intakeLimit: sub("intakeLimit", (v) => ({
			orderCountUsed: s(v, "orderCountUsed"),
			orderCountLimit: s(v, "orderCountLimit"),
			amountUsed: s(v, "amountUsed"),
			amountLimit: s(v, "amountLimit"),
			blocked: v["blocked"] === true,
		})),
	};
}

/** 付款方式可用性（結帳預覽 + 訂單詳情再次付款共用）。後端 availablePaymentMethods 元素 */
export interface AvailablePaymentMethod {
	paymentType: string; // PaymentMethod value（2001/2002/2003/2004）
	eligible: boolean; // 金額是否落在該方式合法區間
	ineligibleReason: string; // PaymentMethodIneligibleReason value；eligible=true 時空字串
}

/** 從後端 data 解析 availablePaymentMethods（支援 camel/snake；缺漏回空陣列） */
export function parseAvailablePaymentMethods(
	data: Record<string, unknown>,
): AvailablePaymentMethod[] {
	const raw =
		(data["availablePaymentMethods"] as unknown[] | undefined) ??
		(data["available_payment_methods"] as unknown[] | undefined);
	if (!Array.isArray(raw)) return [];
	return raw.map((m) => {
		const o = (m ?? {}) as Record<string, unknown>;
		return {
			paymentType: String(o["paymentType"] ?? o["payment_type"] ?? ""),
			eligible: Boolean(o["eligible"]),
			ineligibleReason: String(
				o["ineligibleReason"] ?? o["ineligible_reason"] ?? "",
			),
		};
	});
}

export interface CheckoutConfirmRequest extends CheckoutPreviewRequest {
	checkoutId: string;
	recipientName?: string;
	recipientPhone?: string;
	shippingAddress?: string;
	shippingNote?: string;
	orderRemark?: string;
	/**
	 * 買家的發票選擇（58 §7.1.2 步驟 1，**選填**）。
	 *
	 * 🔴 **整包省略是合法且常見的**：已上架的買家 App 完全不知道這個欄位存在，
	 * 後端未帶時走 `USE_BUYER_DEFAULT`（由 invoice-service 在開票時解析）。
	 * 任何把它變成必填的改動都會立刻打爛線上結帳。
	 */
	invoice?: CheckoutConfirmInvoice;
}

/**
 * confirm 的發票區塊。四個欄位**各自只填自己的**——後端的 `chk_ois_*` 是靠
 * 「哪幾欄是空的」判形狀，多填一欄會落 9306（三者互斥）。
 *
 * ⚠️ 沒有 `carrierType`／`value`：手機條碼與自然人憑證走「先存成常用資料 →
 * 送 profileId」，理由見 58 §5821 第 4 項（order_invoice_snapshots 是零 PII 表）。
 */
export interface CheckoutConfirmInvoice {
	profileId?: string;
	buyerTaxId?: string;
	buyerTitle?: string;
	donationCode?: string;
	notifyEmail?: string;
}

export interface CheckoutConfirmResult {
	orderNos: string[];
	totalPayAmount: number;
	raw: Record<string, unknown>;
}

export interface AvailableShippingOption {
	shippingMethod: string;
	label: string;
	fee: number;
	feePayer?: string;
	freeShippingEligible: boolean;
	freeShippingThreshold?: number;
	deliveryTemp?: string;
}

export interface CheckoutValidationError {
	errorCode: string;
	errorMessage: string;
}

/**
 * row 級不可購買明細（後端 issue #1127）。reason ∈
 * {DELISTED, OUT_OF_STOCK, INSUFFICIENT_STOCK, LOOKUP_FAILED}（CartItemUnavailableReason）。
 * Validate / Preview / Confirm 三 endpoint 共用。Validate 階段非空時 valid=false；
 * Preview / Confirm 階段非空時 envelope code=4001 + dialog 提示。
 */
export interface UnavailableItem {
	skuId: string;
	skuName: string;
	reason: string;
	requestedQty: string;
	availableQty: string;
}

export interface CheckoutValidateResult {
	valid: boolean;
	deliveryTempIntersection: string[];
	availableShippingOptions: AvailableShippingOption[];
	errors: CheckoutValidationError[];
	unavailableItems: UnavailableItem[];
}

interface AvailableShippingOptionWire {
	deliveryTemp?: string;
	fee?: string;
	feePayer?: string;
	freeShippingEligible?: boolean;
	freeShippingThreshold?: string;
	label?: string;
	shippingMethod?: string;
}

interface UnavailableItemWire {
	skuId?: string;
	skuName?: string;
	reason?: string;
	requestedQty?: string;
	availableQty?: string;
}

interface ValidateWireData {
	availableShippingOptions?: AvailableShippingOptionWire[];
	deliveryTempIntersection?: string[];
	errors?: Array<{ errorCode?: string; errorMessage?: string }>;
	unavailableItems?: UnavailableItemWire[];
	valid?: boolean;
}

function mapUnavailableItems(
	wires: UnavailableItemWire[] | undefined,
): UnavailableItem[] {
	if (!wires || wires.length === 0) return [];
	return wires.map((u) => ({
		skuId: String(u.skuId ?? ""),
		skuName: String(u.skuName ?? ""),
		reason: String(u.reason ?? ""),
		requestedQty: String(u.requestedQty ?? ""),
		availableQty: String(u.availableQty ?? ""),
	}));
}

function toNum(v: unknown): number | undefined {
	if (v == null || v === "") return undefined;
	const n = Number(v);
	return Number.isFinite(n) ? n : undefined;
}

function toRecord(value: unknown): Record<string, unknown> {
	return value && typeof value === "object"
		? (value as Record<string, unknown>)
		: {};
}

function pickString(data: Record<string, unknown>, keys: string[]): string {
	for (const key of keys) {
		const value = data[key];
		if (value != null && String(value) !== "") return String(value);
	}
	return "";
}

function pickNumber(data: Record<string, unknown>, keys: string[]): number {
	for (const key of keys) {
		const value = toNum(data[key]);
		if (value != null) return value;
	}
	return 0;
}

function pickNumberOrNull(
	data: Record<string, unknown>,
	keys: string[],
): number | null {
	for (const key of keys) {
		const value = toNum(data[key]);
		if (value != null) return value;
	}
	return null;
}

function normalizeItems(items: CheckoutItemInput[]) {
	return items
		.filter((i) => i.skuId && i.productId != null && Number(i.quantity) > 0)
		.map((i) => {
			const quantity = Number(i.quantity);
			return {
				skuId: String(i.skuId),
				productId: String(i.productId),
				quantity: String(quantity),
			};
		});
}

/** preview 與 confirm 共用的 body。 */
function checkoutPayload(body: CheckoutPreviewRequest) {
	const receiverZipCode = body.receiverZipCode?.trim();
	return {
		items: normalizeItems(body.items),
		shippingMethod: body.shippingMethod,
		deliveryTemp: body.deliveryTemp,
		...(receiverZipCode ? { receiverZipCode } : {}),
		...(body.storeId ? { storeId: body.storeId } : {}),
		...(body.storeName ? { storeName: body.storeName } : {}),
		...(body.couponTemplateId
			? { couponTemplateId: body.couponTemplateId }
			: {}),
		...(body.consolidated ? { consolidated: true } : {}),
	};
}

export interface ShippingFeeRequest {
	items: CheckoutItemInput[];
	shippingMethod: string;
	deliveryTemp: string;
	receiverZipCode: string;
}

export interface ShippingFeeResult {
	shippingFee: number;
	freeShippingEligible: boolean;
}

interface ShippingFeeWireData {
	shippingFee?: string;
	freeShippingEligible?: boolean;
}

/**
 * POST /app/checkout/shipping_fee — precise fee for home delivery once the
 * buyer has picked an address. The validate endpoint returns an estimate;
 * shipping_fee re-runs the carrier's weight/volume/zone math with the real
 * zip code so the buyer sees the actual amount before swiping to pay.
 */
export async function calculateShippingFee(
	body: ShippingFeeRequest,
): Promise<ShippingFeeResult> {
	const env = await authedFetch<ShippingFeeWireData>(
		"/app/checkout/shipping_fee",
		{
			method: "POST",
			json: {
				items: normalizeItems(body.items),
				shippingMethod: body.shippingMethod,
				deliveryTemp: body.deliveryTemp,
				receiverZipCode: body.receiverZipCode,
			},
		},
	);
	const data = env.data ?? {};
	return {
		shippingFee: toNum(data.shippingFee) ?? 0,
		freeShippingEligible: Boolean(data.freeShippingEligible),
	};
}

export async function validateCheckoutItems(
	items: CheckoutItemInput[],
): Promise<CheckoutValidateResult> {
	const env = await authedFetch<ValidateWireData>("/app/checkout/validate", {
		method: "POST",
		json: {
			items: normalizeItems(items),
		},
	});
	const data = env.data ?? {};
	return {
		valid: Boolean(data.valid),
		deliveryTempIntersection: data.deliveryTempIntersection ?? [],
		availableShippingOptions: (data.availableShippingOptions ?? []).map(
			(o): AvailableShippingOption => ({
				shippingMethod: String(o.shippingMethod ?? ""),
				label: String(o.label ?? ""),
				fee: toNum(o.fee) ?? 0,
				feePayer: o.feePayer ? String(o.feePayer) : undefined,
				freeShippingEligible: Boolean(o.freeShippingEligible),
				freeShippingThreshold: toNum(o.freeShippingThreshold),
				deliveryTemp: o.deliveryTemp ? String(o.deliveryTemp) : undefined,
			}),
		),
		errors: (data.errors ?? []).map((e) => ({
			errorCode: String(e.errorCode ?? ""),
			errorMessage: String(e.errorMessage ?? ""),
		})),
		unavailableItems: mapUnavailableItems(data.unavailableItems),
	};
}

export async function previewCheckout(
	body: CheckoutPreviewRequest,
): Promise<CheckoutPreviewResult> {
	const env = await authedFetch<Record<string, unknown>>(
		"/app/checkout/preview",
		{
			method: "POST",
			json: checkoutPayload(body),
		},
	);
	const data = toRecord(env.data);
	return {
		checkoutId: pickString(data, ["checkoutId", "checkout_id", "id"]),
		subtotal: pickNumber(data, [
			"subtotal",
			"itemSubtotal",
			"itemsSubtotal",
			"totalOriginalAmount",
			"total_original_amount",
		]),
		shippingFee: pickNumberOrNull(data, [
			"totalShippingFee",
			"total_shipping_fee",
			"shippingFee",
			"shipping_fee",
			"freight",
			"deliveryFee",
		]),
		discountAmount: pickNumber(data, [
			"totalDiscount",
			"total_discount",
			"discountAmount",
			"discount_amount",
			"couponDiscount",
			"coupon_discount",
		]),
		totalPayAmount: pickNumber(data, [
			"totalActualAmount",
			"total_actual_amount",
			"totalPayAmount",
			"total_pay_amount",
			"payAmount",
			"amount",
			"grandTotal",
			"total",
		]),
		paymentMerId: pickString(data, ["paymentMerId", "payment_mer_id"]),
		availablePaymentMethods: parseAvailablePaymentMethods(data),
		invoice: parseCheckoutInvoicePreview(data),
		raw: data,
	};
}

export async function confirmCheckout(
	body: CheckoutConfirmRequest,
): Promise<CheckoutConfirmResult> {
	// Payload schema mirrors the buyer APP's confirm body exactly
	// (lib/features/cart/presentation/widgets/checkout_payment_sheet.dart:282).
	// Notable parity points:
	//   - shippingNote and orderRemark are ALWAYS sent (even as empty
	//     strings) — APP omits the `if` guard for them. The web's
	//     conditional spread was hypothesised as one of the variables
	//     that could trip backend dedupe (resource conflict / 409),
	//     so we now match APP's "always present, possibly empty"
	//     contract verbatim.
	//   - recipientName / recipientPhone are required by the OpenAPI confirm
	//     schema, so they are always sent. The checkout UI supplies them for
	//     both home delivery and store pickup.
	//   - shippingAddress remains conditional because it is not required for
	//     store pickup.
	const wirePayload = {
		...checkoutPayload(body),
		checkoutId: body.checkoutId,
		shippingNote: body.shippingNote ?? "",
		orderRemark: body.orderRemark ?? "",
		recipientName: body.recipientName ?? "",
		recipientPhone: body.recipientPhone ?? "",
		...(body.shippingAddress ? { shippingAddress: body.shippingAddress } : {}),
		// 🔴 買家的發票選擇（58 §7.1.2 步驟 1）。**這一行曾經漏掉**：
		// 上面那段「mirrors the buyer APP's confirm body exactly」是照當時的 App body 抄的，
		// 而 `invoice` 是之後才加進 CheckoutConfirmRequest 的欄位 ⇒ 逐欄重組時被丟掉。
		// 症狀非常隱蔽：畫面顯示已套用、buildConfirmPayload 也確實組進了 body，
		// 但送出的 wire payload 沒有它 ⇒ 後端一律走 USE_BUYER_DEFAULT。
		// TS 不會叫（wirePayload 是無型別標註的新物件，少一欄不算錯），
		// 而 USE_BUYER_DEFAULT 是合法值 ⇒ DB CHECK 與後端驗證也都不會有意見。
		// 2026-09-01 dev 實測：web 選統編／捐贈碼／常用資料三種全部沒帶到，
		// 而同日 App 送的同樣選擇都正確落庫。
		...(body.invoice ? { invoice: body.invoice } : {}),
	};
	if (process.env.NODE_ENV !== "production") {
		console.warn(
			"[checkout] confirm wire payload:",
			JSON.stringify(wirePayload, null, 2),
		);
	}
	const env = await authedFetch<Record<string, unknown>>(
		"/app/checkout/confirm",
		{
			method: "POST",
			json: wirePayload,
		},
	);
	const data = toRecord(env.data);
	const rawOrderNos =
		data.orderNos ??
		data.order_nos ??
		data.orderNumbers ??
		data.order_no ??
		data.orderNo;
	const orderNos = Array.isArray(rawOrderNos)
		? rawOrderNos.map(String).filter(Boolean)
		: rawOrderNos != null && String(rawOrderNos) !== ""
			? [String(rawOrderNos)]
			: [];
	return {
		orderNos,
		totalPayAmount: pickNumber(data, [
			"totalPayAmount",
			"total_pay_amount",
			"payAmount",
			"amount",
			"grandTotal",
			"total",
		]),
		raw: data,
	};
}
