/**
 * 買家出價 → 賣家接受 → 買家確認 的可操作 demo。
 *
 * 與 `features/procurement-rfq-prototype` 的關係：那份是 11 個畫面的靜態設計
 * 預覽（零 API 呼叫，畫面上自陳「前端預覽不會送出任何回覆」）。本 demo 只取
 * 其中三個真正構成商務閉環的步驟，並且**買家那半吃真資料**——真 request
 * slots、真候選名單、真 Pricebook 參考價，出價也真的落成 confirmed snapshot；
 * 賣家那半仍是 local state，但它顯示的是買家這次真的定稿的那張詢價單，
 * 所以整條流程是連貫的。
 *
 * ⛔ 硬界線：本 demo 不呼叫 `POST /quote-batches`。那支會建立
 * `procurement_quote_invites` 並經 outbox 發 `procurement.quote.invite.created`
 * 到 RabbitMQ；PLATFORM lane 是真的會通知賣家的。demo 停在 confirmed snapshot，
 * 那一步只凍結買家自己的條件，不對任何賣家產生訊息。
 */

/** 三個步驟就是這條線的全部：買家出價、賣家決定、買家確認。 */
export type DemoStep = "buyer-offer" | "seller-inbox" | "buyer-confirm";

/** 賣家對「買家已定稿條件」的處置。原型 seller-response 的三個選項。 */
export type SellerDecision = "pending" | "accepted" | "disputed" | "declined";

/**
 * 買家這次的需求事實，全部來自 `GET /requests/{id}/snapshot` 的 slots。
 *
 * 值一律原樣搬運，demo 不補值也不推測——缺的就顯示為未填，讓 demo 誠實反映
 * 真實資料狀態。
 */
export interface DemoRequirement {
	requestId: string;
	revisionId: string;
	item: string | null;
	quantity: number | null;
	unit: string | null;
	packaging: string | null;
	deliveryCity: string | null;
	deliveryDate: string | null;
	/** 買家在釐清輪填的「單價預算」。這就是原型 rfq-compose 說的「理想價」。 */
	budgetPerUnit: number | null;
}

/**
 * 收件對象。取自最新一輪 CANDIDATE_SEARCH 的候選，只留買家已勾選的那些
 * ——沒勾選的不是收件人，放進來會讓「要發給誰」這件事失真。
 */
export interface DemoCandidate {
	candidateId: string;
	displayName: string;
	lane: string;
	/** 平台賣家才有；外部業者一律缺席，不補空字串。 */
	sellerName: string | null;
	matchedFactCount: number;
	unverifiableFactCount: number;
}

/**
 * Pricebook 行情，來自 `POST /rfq-price-previews` 的 referencePriceRange
 * ＋ marketPriceBasis。只有 `PRICEBOOK_GUIDED` 且 mapping 可靠時才有值。
 */
export interface DemoMarketReference {
	lower: number;
	midpoint: number;
	upper: number;
	/** 金額是「每多少單位」的價格。GRAM 是每 1000 公克，不講會差 1000 倍。 */
	basisQuantity: string;
	basisUnit: string;
	regionName: string;
	sourceDataAsOf: string | null;
	freshnessStatus: string;
}

/**
 * 詢價總額區間，伺服器依「本次單價範圍 × 需求數量」算出。
 *
 * 刻意不由前端自己乘：單價的基準是 basisQuantity/basisUnit，GRAM 品項前端
 * 自己乘會錯 1000 倍（gateway swagger 明寫這條）。數量沒填時後端回 null。
 */
export interface DemoQuoteTotalRange {
	lower: number;
	upper: number;
	quantity: string;
	quantityUnit: string;
}

/**
 * 一次價格試算的身分。confirm 時要原樣回送這三個值做 CAS，缺一不可。
 *
 * ⚠️ 這裡的 `payloadHash` 是「這一份試算」的 hash，跟 confirmed snapshot
 * 自己的 `payloadHash` 同名但不同義，不得互相代入。
 */
export interface DemoPreviewIdentity {
	pricePreviewId: string;
	version: string;
	payloadHash: string;
}

/** 買家定稿的出價。理想價與可接受區間都會原樣交給賣家。 */
export interface DemoBuyerOffer {
	idealUnitPrice: number;
	minUnitPrice: number;
	maxUnitPrice: number;
	/** 伺服器回寫的 confirmed snapshot 身分——這才是「買家定稿」的證據。 */
	confirmedSnapshotId: string;
	/** confirmed snapshot 自己的 payload hash（不是 preview 那個）。 */
	snapshotPayloadHash: string;
	confirmedAt: string;
}

/** 賣家的回覆。異議時可帶一個調整後的單價，婉拒時只有理由。 */
export interface DemoSellerResponse {
	decision: SellerDecision;
	/** 只有 disputed 才有值：賣家提出的替代單價。 */
	counterUnitPrice: number | null;
	note: string;
	respondedAt: string | null;
}

/** demo 一次載入所需的全部真資料。 */
export interface DemoContext {
	requirement: DemoRequirement;
	/** 買家已勾選的收件對象；沒有候選搜尋紀錄時為空陣列。 */
	candidates: DemoCandidate[];
	/** 找不到 CANDIDATE_SEARCH run 時的原因，用來誠實說明為何名單是空的。 */
	candidatesUnavailableReason: string | null;
	market: DemoMarketReference | null;
	quoteTotalRange: DemoQuoteTotalRange | null;
	preview: DemoPreviewIdentity | null;
	/** 行情不可用時的 typed 原因，原樣搬運不改寫。 */
	marketUnavailableReason: string | null;
}
