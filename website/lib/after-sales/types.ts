/**
 * Shared after-sale (售後 / 退換貨) domain types for BOTH buyer and seller
 * surfaces. The seller order-list lookup historically lived in
 * `lib/seller/after-sales.ts`; that module is now a thin re-export of this
 * package so existing seller imports keep compiling.
 *
 * Wire endpoints (see docs/planning/AFTER_SALES_UI_PLAN.md + OpenAPI):
 *   Buyer  : GET/POST /app/after_sales, GET /app/after_sales/{caseNo},
 *            POST /app/after_sales/{caseNo}/{cancel,supplement,appeal}
 *   Seller : GET /app/seller/after_sales(+/{caseNo}),
 *            POST /app/seller/after_sales/{caseNo}/{resolve,confirm_receipt}
 *
 * The backend mixes camelCase and snake_case across views, so every shape is
 * normalized defensively (see status.ts `normalizeCase` / repo `normalize*`).
 */

/** codemeta `AfterSaleCaseStatus` numeric codes (string on the wire). */
export const AFTER_SALE_STATUS = {
	PENDING: "0",
	RETURN_PENDING: "20",
	RETURN_SHIPPING: "25",
	/** 退貨已送達超商，等待物流取件（前端依 storePickupDeadlineAt 倒數 72h）。 */
	RETURN_AT_STORE: "26",
	/** 退貨已送達賣家，等待賣家確認（逾時依 autoConfirmAt 自動確認）。 */
	RETURN_DELIVERED: "27",
	/** 退貨物流失敗（returnProviderMessage 為 PayUni 原文）。 */
	RETURN_FAILED: "28",
	RETURN_RECEIVED: "30",
	/** 待寄回逾時自動關案（買家逾 7 天未寄回退貨商品，退款資格解除）。 */
	AUTO_COMPLETED_RETURN_PENDING_TIMEOUT: "33",
	REFUNDING: "40",
	CLOSED: "50",
	REJECTED: "60",
	CANCELLED: "70",
} as const;
export type AfterSaleStatus =
	(typeof AFTER_SALE_STATUS)[keyof typeof AFTER_SALE_STATUS];

/**
 * `refundTaskStatus` (PayUni WorkerTaskStatus) on the case detail. The
 * crucial one is `AWAITING_BUYER_REFUND_PAGE` ("5") — that, not a boolean,
 * is what tells the buyer to fill in ATM refund bank details.
 */
export const REFUND_TASK_STATUS = {
	PENDING: "0",
	AWAITING_BUYER_REFUND_PAGE: "5",
	PROCESSING: "10",
	AWAITING_WEBHOOK: "15",
	DONE: "20",
	FAILED: "30",
	MANUAL_INTERVENTION: "40",
} as const;

/** Status group a case falls into — drives the seller 售後處理 filter chips
 * and the buyer history tabs. */
export type AfterSaleGroup =
	| "all"
	| "pending"
	| "returning"
	| "refunding"
	| "closed";

/** Tone ladder shared with the seller-center status palette
 * (success / warning / danger / info / muted). */
export type AfterSaleTone = "success" | "warning" | "danger" | "info" | "muted";

/**
 * Base case shape — the list row used by both the seller case table and the
 * buyer history list. `reason` is a codemeta `RefundReason` code ("5001"…);
 * `reasonText` is the resolved display label when the backend sends free text.
 */
export interface AfterSaleCase {
	caseNo: string;
	orderNo: string;
	/** 買家營運鍵（buyer surrogate）。wire `buyerId`；列表摘要不帶此欄，僅詳情有值。 */
	buyerId: string;
	buyerName: string;
	/** Buyer avatar objectKey (wire `avatarKey`, 49 §6.1 SSOT) — a key, never a
	 * composed URL; renderers resolve it through buildImageUrl. */
	avatarKey?: string;
	sellerName?: string;
	status: string;
	/** Appeal lifecycle state (codemeta AfterSaleAppealState: "0" NONE /
	 * "10" APPEALED / "20" INVESTIGATING / "30" RESOLVED). The list summary
	 * carries it (NOT NULL DEFAULT "0"), so list/card rows can fold「申訴仲裁中」
	 * over the raw status. */
	appealState?: string;
	/** codemeta RefundReason code, e.g. "5001". */
	reason: string;
	/** Free-text reason / description from the buyer. */
	description: string;
	/** Max amount the seller may refund for this case. */
	refundableAmount: string;
	/** Amount actually resolved/refunded once a decision is made. */
	refundAmount?: string;
	returnShipmentNo: string;
	returnShippingMethod: string;
	createdAt: string;
	closedAt?: string;
	/** Seller process deadline (ISO) — drives the countdown in the table. */
	processDeadline?: string;
	/** 超商待取逾時截止 — only set when status=26 RETURN_AT_STORE. 72h countdown. */
	storePickupDeadlineAt?: string;
	/** 待寄回逾時截止 — only set when status=20 RETURN_PENDING. 7-day countdown;
	 * past the deadline the case auto-closes into status=33 and refund eligibility
	 * is revoked. */
	shipBackDeadlineAt?: string;
	/** 溫層代碼 (codemeta DeliveryTemp "3201"/"3202"/"3203"). */
	temperatureCode?: string;
	/** Return/shipping address text (seller-supplied for home return). */
	shippingAddress?: string;
}

/** A line item attached to the case (order snapshot). */
export interface AfterSaleItem {
	orderItemId: string;
	name: string;
	image?: string;
	skuName?: string;
	quantity: number;
	price: string;
}

/** Buyer- or seller-supplied photo evidence. `url` is a CDN preview URL. */
export interface AfterSaleEvidence {
	url: string;
	/** Optional caption / origin marker. */
	label?: string;
}

/** A single seller decision in the case history. */
export interface AfterSaleResolution {
	/** codemeta AfterSaleResolutionType code ("1"…"6"). */
	resolutionType: string;
	refundAmount?: string;
	couponAmount?: string;
	couponMinSpend?: string;
	couponMessage?: string;
	/** codemeta AfterSaleRejectReason code. */
	rejectReason?: string;
	rejectMessage?: string;
	/** 驗貨扣款（resolutionType=6）扣款原因。 */
	deductionReason?: string;
	/** 驗貨扣款（resolutionType=6）驗貨備註。 */
	inspectionNote?: string;
	createdAt?: string;
}

/** Return logistics block — 7-11 store-to-store codes or 黑貓 home pickup. */
export interface AfterSaleReturnLogistics {
	/** codemeta ShippingMethod code ("3001" 黑貓 / "3002" 7-11). */
	method?: string;
	shipmentNo?: string;
	/** 7-11 three-segment codes. */
	partnerId?: string;
	odno?: string;
	validationNo?: string;
	/** 黑貓 / provider shipment status text. */
	shipmentStatus?: string;
}

/** Refund execution progress (original-route or ATM). */
export interface AfterSaleRefundProgress {
	refundTaskStatus?: string;
	estimatedRefundDate?: string;
	bankDate?: string;
	refundFailureMessage?: string;
	/** True when the buyer must submit bank/ATM details before refund runs. */
	needRefundPage?: boolean;
}

/** Full case detail — superset of the list row. */
export interface AfterSaleDetail extends AfterSaleCase {
	items: AfterSaleItem[];
	/** Buyer-submitted evidence — `evidence[]` entries that are NOT
	 * AfterSaleEvidenceType.INSPECTION_PHOTO ("8602"); 買家舉證 is "8601". */
	photos: AfterSaleEvidence[];
	/** Seller inspection photos — `evidence[]` entries tagged
	 * AfterSaleEvidenceType.INSPECTION_PHOTO ("8602"), uploaded at
	 * confirm-receipt to back an inspection deduction. */
	sellerPhotos: AfterSaleEvidence[];
	resolutions: AfterSaleResolution[];
	returnLogistics?: AfterSaleReturnLogistics;
	refundProgress?: AfterSaleRefundProgress;
	/** Buyer may file a platform appeal once, after the seller's decision. */
	canAppeal?: boolean;
	/** Appeal window deadline (ISO). */
	appealDeadline?: string;
	/** Whether the platform auto-escalated to arbitration (§5.10). */
	autoEscalated?: boolean;
	/** Seller auto-confirm-receipt deadline (only status=27 RETURN_DELIVERED). */
	autoConfirmAt?: string;
	/** Return delivered timestamp (status >= 27). */
	returnDeliveredAt?: string;
	/** PayUni return-logistics failure message (status=28 RETURN_FAILED). */
	returnProviderMessage?: string;
	returnProviderStatus?: string;
}

/* ---- Platform arbitration (申訴 / 仲裁) -------------------------------- */

/** A single arbitration timeline event. */
export interface AfterSaleArbitrationEvent {
	/** codemeta AfterSaleAppealEventType (APPEAL_SUBMITTED / DECISION_MADE). */
	eventType: string;
	label: string;
	description: string;
	timestamp: string;
}

/** The platform's arbitration decision (once made). */
export interface AfterSaleArbitrationDecision {
	/** codemeta AfterSalePlatformDecisionOutcome (FULL_REFUND/PARTIAL_REFUND/REJECT_APPEAL). */
	outcome: string;
	reason: string;
	/** Only set for PARTIAL_REFUND. */
	refundAmount?: string;
	decidedAt?: string;
}

/** GET /app/{seller/}after_sales/{caseNo}/arbitration. */
export interface AfterSaleArbitration {
	state: string;
	decision?: AfterSaleArbitrationDecision;
	timeline: AfterSaleArbitrationEvent[];
}

/** Buyer: POST /app/after_sales/{caseNo}/appeal. */
export interface AppealAfterSaleInput {
	/** Short reason category / summary. */
	reason: string;
	description?: string;
	/** Up to 5 staged tokens. */
	evidencePhotoTokens?: string[];
}

export interface AfterSaleListResult {
	records: AfterSaleCase[];
	total: number;
}

/* ---- Request inputs ---------------------------------------------------- */

/** Buyer: POST /app/after_sales. */
export interface CreateAfterSaleInput {
	orderNo: string;
	/** codemeta RefundReason code. */
	reason: string;
	description: string;
	/** Up to 3 staged tokens from `uploadImageFile`. */
	photoTokens?: string[];
}

/** Seller: POST /app/seller/after_sales/{caseNo}/resolve. */
export interface ResolveAfterSaleInput {
	caseNo: string;
	/** codemeta AfterSaleResolutionType code ("1"…"6"). */
	resolutionType: string;
	/** Required for type 1 — codemeta ShippingMethod ("3001"/"3002"). */
	returnShippingMethod?: string;
	/** Required for type 2/3. */
	refundAmount?: string;
	/** Required for type 4. */
	couponAmount?: string;
	couponMinSpend?: string;
	couponMessage?: string;
	/** Required for type 5 — codemeta AfterSaleRejectReason code. */
	rejectReason?: string;
	rejectMessage?: string;
}

/** Seller: POST /app/seller/after_sales/{caseNo}/confirm_receipt. */
export interface ConfirmReceiptInput {
	caseNo: string;
	actualRefundAmount: string;
	/** Required when actualRefundAmount < the resolved amount. */
	deductionReason?: string;
	inspectionNote?: string;
	/** Up to 3 staged tokens. */
	inspectionPhotoTokens?: string[];
}
