/**
 * Seller-side shipment operations (Phase 3 of the migration plan §6.2 Flow D).
 *
 * Mirrors `hawkr_seller/lib/features/order/data/order_repository.dart`:
 *   - generateHomeLabel()  POST /app/shipments/{order_no}/generate_home_label
 *   - callTcat()           POST /app/shipments/{order_no}/call_tcat
 *   - printCvsLabel()      POST /app/shipments/{order_no}/print_cvs_label
 *   - confirmShipment()    POST /app/shipments/{order_no}/confirm
 *   - fetchShippingBusinessDays()  GET /app/shipping_business_days
 *   - queryLogistics()     POST /app/logistics
 *
 * The button-state state machine + per-order cooldown / odno override helpers
 * live alongside the repo because the UI imports both — keeping them in one
 * file means callers don't have to wire two modules together for every action.
 *
 * Backend business codes we care about (carried in the message prefix
 * "[5xxx] …", parsed by shipmentBusinessCode):
 *   5001  物流單不存在 — terminal, contact support
 *   5046  Shipment created < 30s ago — transient, cooldown handles
 *   5047  PayUni call in flight — transient, cooldown handles
 *   5048  Backend has a user-facing message, show as-is
 */

import { ApiClientError, authedFetch } from "@/lib/api/client";
import { normalizeShippingMethod, ShippingMethod } from "@/lib/codemeta/enums";

/* ===== Code-meta enums (mirrors Flutter codemeta) ===== */

// 配送方式的值域**只有一個來源**：`lib/codemeta/generated.ts` 的 `ShippingMethod`
// （CodeMeta SSOT 生成，含 3001～3005）。本檔曾自帶一份手寫的 `SHIPPING_METHOD`
// 三值副本，於是後端加 3004／3005 時本檔完全不會知道。

/**
 * Carrier shipment-status values — sourced from the seller App's authoritative
 * `ShippingStatus` enum (`lib/core/codemeta/code_meta.dart`). The previous web
 * values (93/94/95…) were guessed and wrong: the real `TCAT_CALLED` is "210",
 * not "93", so the "已叫件 → 下載 PDF" branch never matched and an already-called
 * order fell through to "呼叫黑貓" again. We only branch on the ones the button
 * state machine actually checks; the rest are documented for reference.
 */
export const SHIPMENT_STATUS = {
	/** 尚未取得出貨單編號（= 尚未取號）— callTcat 在此狀態會回 5011 */
	UNPROCESSED: "91",
	/** 已取號、待商店出貨/呼叫黑貓 */
	PENDING_SHIPMENT: "21",
	/** 配送中 */
	IN_TRANSIT: "31",
	/** callTcat 成功，等待黑貓司機取件 */
	TCAT_CALLED: "210",
} as const;

export const LGS_TYPE = {
	B2C: "B2C", // 超商大宗寄倉
	C2C: "C2C", // 超商店到店
	HOME: "HOME", // 黑貓宅配
	C2B: "C2B", // 超商退貨便
} as const;
export type LgsType = (typeof LGS_TYPE)[keyof typeof LGS_TYPE];

/**
 * 配送方式（CodeMeta `ShippingMethod`）→ PayUni 配送軸（`LgsType`）。
 *
 * 🔴 **認不得就回 `null`，不得 fallback 成 `C2C`**。舊寫法
 * `isTaqbin ? HOME : C2C` 對 `3003`／`3004`／`3005` 回的是一個**合法值**，
 * 於是進度條照畫、狀態照查、測試全綠，畫面卻是別人家物流的形狀。
 *
 * ⚠️ 這條對映本身是過渡物：正解是後端回 normalized 配送值（65 的 31 值），
 * 前端不再持有 PayUni 的軸。
 */
export function lgsTypeForShippingMethod(
	method: string | undefined,
): LgsType | null {
	switch (normalizeShippingMethod(method)) {
		case ShippingMethod.TAQBIN:
			return LGS_TYPE.HOME;
		case ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP:
			return LGS_TYPE.C2C;
		default:
			return null;
	}
}

/* ===== API response types ===== */

export interface BusinessDay {
	/** yyyy-MM-dd */
	date: string;
	/** 一 / 二 / … / 六 */
	dayOfWeek: string;
}

export interface CallTcatResult {
	callTime: string;
	shipmentNo: string;
}

export interface CvsLabelResult {
	shipmentNo: string;
	partnerId: string;
	odno: string;
	validationNo: string;
	webhookReceived: boolean;
}

export interface HomeLabelResult {
	labelUrl: string;
	shipmentNo: string;
}

export interface LogisticsInfo {
	status: string;
	message: string;
	shipTradeNo: string;
	odno: string;
	goodsType: number; // 1 常溫 / 2 冷凍 / 3 冷藏
	lgsType: string;
	shipType: number; // 1 7-ELEVEN / 2 黑貓
	serviceType: number;
	shipAmt: number;
	consignee: string;
	consigneeMobile: string;
	shipStatus: number;
	shipStatusDesc: string;
	shipStatusTime: string;
	progressPercent: number;
	storeID?: string;
	storeName?: string;
	validationNo?: string;
	fileNo?: string;
	consigneeAddress?: string;
	isFinalStatus: boolean;
	isErrorStatus: boolean;
}

/* ===== HTTP calls ===== */

/** GET /app/shipping_business_days — 明日起 7 個工作天（排週日 + 國定假日） */
export async function fetchShippingBusinessDays(): Promise<BusinessDay[]> {
	const env = await authedFetch<{ businessDays?: unknown[] }>(
		"/app/shipping_business_days",
		{ throwOnHttpError: false },
	);
	const list = env.data?.businessDays ?? [];
	return list
		.filter(
			(x): x is Record<string, unknown> => typeof x === "object" && x != null,
		)
		.map((e) => ({
			date: String(e.date ?? ""),
			dayOfWeek: String(e.dayOfWeek ?? ""),
		}));
}

/**
 * POST /app/shipments/{order_no}/generate_home_label
 * @param shipDate yyyyMMdd（如 "20260512"）
 * @param stagedTokens 選填，出貨照片的 stagedToken（走 media/upload_sessions）
 */
export async function generateHomeLabel(
	orderNo: string,
	args: { shipDate: string; stagedTokens?: string[] },
): Promise<HomeLabelResult> {
	const body: Record<string, unknown> = { shipDate: args.shipDate };
	if (args.stagedTokens?.length) body.stagedTokens = args.stagedTokens;
	const env = await authedFetch<Record<string, unknown>>(
		`/app/shipments/${encodeURIComponent(orderNo)}/generate_home_label`,
		{ method: "POST", json: body },
	);
	const data = env.data ?? {};
	return {
		labelUrl: String(data.labelUrl ?? data.label_url ?? ""),
		shipmentNo: String(data.shipmentNo ?? ""),
	};
}

/** POST /app/shipments/{order_no}/call_tcat — 黑貓叫件 */
export async function callTcat(
	orderNo: string,
	args: { isContact: "Y" | "N"; isTrolley: "Y" | "N"; memo?: string },
): Promise<CallTcatResult> {
	const body: Record<string, unknown> = {
		isContact: args.isContact,
		isTrolley: args.isTrolley,
	};
	if (args.memo?.trim()) body.memo = args.memo.trim();
	const env = await authedFetch<Record<string, unknown>>(
		`/app/shipments/${encodeURIComponent(orderNo)}/call_tcat`,
		{ method: "POST", json: body },
	);
	const data = env.data ?? {};
	return {
		callTime: String(data.callTime ?? ""),
		shipmentNo: String(data.shipmentNo ?? ""),
	};
}

/**
 * POST /app/shipments/{order_no}/print_cvs_label — 7-11 取號
 *
 * Backend uses `shipDate` = tomorrow yyyyMMdd. Returns odno + validationNo
 * (synchronous from PayUni when `webhookReceived = true`; otherwise the
 * gateway will keep retrying and the seller should refresh later).
 */
export async function printCvsLabel(
	orderNo: string,
	args?: { stagedTokens?: string[] },
): Promise<CvsLabelResult> {
	const tomorrow = new Date(Date.now() + 86400_000);
	const yyyy = tomorrow.getFullYear();
	const mm = String(tomorrow.getMonth() + 1).padStart(2, "0");
	const dd = String(tomorrow.getDate()).padStart(2, "0");
	// 後端規格 PrintCvsLabelReq.shipDate 要 YYYYMMDD（無連字號）—— 與宅配
	// generate_home_label 一致。先前誤送 "YYYY-MM-DD" 會讓 PayUni 取號失敗，
	// 鎖殘留後續重試一律噴 [5047] 物流商呼叫進行中。
	const body: Record<string, unknown> = { shipDate: `${yyyy}${mm}${dd}` };
	if (args?.stagedTokens?.length) body.stagedTokens = args.stagedTokens;

	const env = await authedFetch<Record<string, unknown>>(
		`/app/shipments/${encodeURIComponent(orderNo)}/print_cvs_label`,
		{ method: "POST", json: body },
	);
	const data = env.data ?? {};
	return {
		shipmentNo: String(data.shipmentNo ?? ""),
		partnerId: String(data.partnerId ?? ""),
		odno: String(data.odno ?? ""),
		validationNo: String(data.validationNo ?? ""),
		webhookReceived: data.webhookReceived === true,
	};
}

/**
 * POST /app/shipments/{order_no}/confirm — 出貨確認
 * 7-11 流程必走，建單照片可附帶 stagedTokens（黑貓不需要）。
 */
export async function confirmShipment(
	orderNo: string,
	args?: { stagedTokens?: string[] },
): Promise<void> {
	const body: Record<string, unknown> = {};
	if (args?.stagedTokens?.length) body.stagedTokens = args.stagedTokens;
	await authedFetch(`/app/shipments/${encodeURIComponent(orderNo)}/confirm`, {
		method: "POST",
		json: body,
	});
}

/**
 * GET /app/shipments/{orderNo}/tracking — 賣家訂單物流追蹤
 *
 * 後端即時向 PayUni 同步當前狀態，回 ShipmentDetailView（單點，PayUni 無多筆軌跡）。
 * Ownership 由後端驗 seller_account_id。`isReturn=true` 查退貨單（預設正向單）。
 *
 * 賣家進度條/狀態判定沿用 PayUni 物流碼：取 `providerStatus`（PayUni 原始碼）當 shipStatus；
 * 為空時（PayUni 尚未回填）以我方 `statusLabel` 顯示狀態文案、進度回退 0。
 */
export async function queryLogistics(
	orderNo: string,
	opts?: { isReturn?: boolean },
): Promise<LogisticsInfo | null> {
	const qs = opts?.isReturn ? "?is_return=true" : "";
	const env = await authedFetch<Record<string, unknown>>(
		`/app/shipments/${encodeURIComponent(orderNo)}/tracking${qs}`,
		{ method: "GET", throwOnHttpError: false, throwOnEnvelopeError: false },
	);
	if (env.code !== 0 || !env.data) return null;
	const d = env.data;
	const providerStatus = String(d.providerStatus ?? "");
	const shipStatus = Number(providerStatus) || 0;
	return {
		status: String(d.status ?? ""),
		message: "",
		shipTradeNo: String(d.shipTradeNo ?? ""),
		odno: "",
		goodsType: 0,
		lgsType: "",
		shipType: 0,
		serviceType: 0,
		shipAmt: 0,
		consignee: String(d.recipientName ?? ""),
		consigneeMobile: "",
		shipStatus,
		shipStatusDesc: String(d.providerMessage || d.statusLabel || ""),
		shipStatusTime: String(d.statusTime ?? ""),
		progressPercent: 0,
		storeName: (d.storeName as string) || undefined,
		isFinalStatus: isFinalShipStatus(shipStatus),
		isErrorStatus: isErrorShipStatus(shipStatus),
	};
}

/* ===== Logistics status code helpers ===== */

const SHIP_STATUS_TEXT: Record<number, string> = {
	0: "初始狀態",
	1: "已取號",
	2: "已列印",
	3: "已出貨",
	10: "配送中",
	11: "到店",
	12: "取貨完成",
	13: "退貨中",
	14: "退貨完成",
	20: "逾期未取",
	21: "遺失",
	30: "已收件",
	31: "配送中",
	32: "配達完成",
	33: "無法投遞",
	34: "退回中",
	35: "退回完成",
	81: "門市關轉",
	99: "異常",
};

export function shipStatusLabel(code: number): string {
	return SHIP_STATUS_TEXT[code] ?? `狀態碼 ${code}`;
}

export function isFinalShipStatus(code: number): boolean {
	return [12, 14, 20, 21, 32, 35].includes(code);
}

export function isErrorShipStatus(code: number): boolean {
	return [20, 21, 33, 99].includes(code);
}

/**
 * PayUni 原始 shipStatus → 進度百分比。
 *
 * 🔴 **`lgsType` 認不得就回 `null`（＝進度未知），不得 fallback 到超商那張表**。
 * 舊寫法是 `if (HOME) {宅配表} else {超商表}`，於是 `B2C`／`C2B`／
 * 空字串／後端新增的任何軸全部走超商百分比 —— 型別對、不拋例外、測試全綠，
 * 只有畫面上的進度條是錯的。
 *
 * ⚠️ `B2C`（超商大宗寄倉）與 `C2B`（超商退貨便）**刻意不列 case**：它們的進度
 * 對應表尚未定義（尚未定義），現在猜一個等於製造第二個靜默錯誤。
 */
export function progressPercentFor(
	code: number,
	lgsType: string,
): number | null {
	switch (lgsType) {
		case LGS_TYPE.HOME:
			return homeProgressPercent(code);
		case LGS_TYPE.C2C:
			return cvsC2cProgressPercent(code);
		default:
			// 未知配送軸：進度未知。呼叫端顯示「處理中」＋原碼，不畫假進度。
			return null;
	}
}

/** 黑貓宅配（HOME）的 PayUni shipStatus → 進度百分比。 */
function homeProgressPercent(code: number): number {
	switch (code) {
		case 0:
			return 0;
		case 1:
		case 2:
			return 15;
		case 30:
			return 30;
		case 31:
			return 60;
		case 32:
			return 100;
		case 33:
		case 34:
			return 80;
		case 35:
			return 100;
		default:
			return 50;
	}
}

/** 7-11 店到店（C2C）的 PayUni shipStatus → 進度百分比。 */
function cvsC2cProgressPercent(code: number): number {
	switch (code) {
		case 0:
			return 0;
		case 1:
			return 10;
		case 2:
			return 20;
		case 3:
			return 35;
		case 10:
			return 50;
		case 11:
			return 80;
		case 12:
			return 100;
		case 13:
			return 85;
		case 14:
			return 100;
		case 20:
		case 21:
		case 99:
			return 100;
		default:
			return 50;
	}
}

/* ===== Error / business code helpers (mirrors shipment_error_dispatch.dart) ===== */

const BIZ_CODE_PREFIX = /^\[(\d+)\]/;

/**
 * Pull the business code out of `[5xxx] message` prefixes that the gateway
 * embeds in the message string. Falls back to the HTTP status / envelope code.
 */
export function shipmentBusinessCode(err: unknown): number {
	if (!(err instanceof Error)) return 0;
	const m = err.message.match(BIZ_CODE_PREFIX);
	if (m) {
		const parsed = Number.parseInt(m[1] ?? "", 10);
		if (Number.isFinite(parsed)) return parsed;
	}
	if (err instanceof ApiClientError && typeof err.code === "number") {
		return err.code;
	}
	return 0;
}

/**
 * Strip the `[5xxx]` prefix and unwrap PayUni's nested error envelope so the
 * seller sees the *actual* actionable reason rather than the engineering wrapper.
 *
 * 後端依 06-Shipping 設計書 §2373 直接透傳 PayUni `ErrorMsg`，格式常是巢狀的
 * 「呼叫黑貓失敗：處理失敗 (執行失敗 -> E072-<真因>。)」。真因夾在最後一個
 * `-> ` 之後、可能帶 `E0xx-` 代碼前綴。舊版用 `replace(/\(...\)/)` 整段刪括號，
 * 反而把唯一可行動的真因（如「寄件地址不在簽約可收貨範圍，請聯繫簽約站所」）
 * 一併刪掉。改為「抽出真因」而非「刪括號」。本身已乾淨的訊息（如「尚未取號，
 * 請先產生 PDF 後再呼叫黑貓」無 `-> ` pattern）原樣透傳。
 */
export function friendlyShipmentMessage(err: unknown): string {
	const fallback = "操作失敗，請稍後再試";
	if (!(err instanceof Error)) return fallback;
	const raw = err.message.trim().replace(BIZ_CODE_PREFIX, "").trim();

	const arrowIdx = raw.lastIndexOf("-> ");
	if (arrowIdx !== -1) {
		// 取最後一個 "-> " 之後的真因，去尾端括號 / 句號，去 "E072-" 之類代碼前綴。
		let reason = raw.slice(arrowIdx + 3);
		reason = reason.replace(/[）)。.\s]+$/u, "").trim();
		reason = reason.replace(/^[A-Za-z]\d{2,}\s*[-:：]\s*/u, "").trim();
		// 保留動作前綴（首個全形冒號前，如「呼叫黑貓失敗：」），讓賣家知道是哪個動作。
		const colonIdx = raw.indexOf("：");
		const prefix = colonIdx !== -1 ? raw.slice(0, colonIdx + 1) : "";
		const composed = (prefix + reason).trim();
		if (reason) return composed || fallback;
	}

	return raw || fallback;
}

export interface ShipmentErrorOutcome {
	/** 是否已被處理（true 代表 caller 不必再 fallback toast） */
	handled: boolean;
	/** 顯示給賣家的訊息 */
	message: string;
	/** UI 應該用哪種 toast：error 紅 / warning 黃 / info 藍 */
	tone: "error" | "warning" | "info";
	/**
	 * 對應的業務碼。caller 拿到 `5046 / 5047` 可以選擇 reload list（後端會
	 * 自己補同步），其他碼通常維持目前畫面就好。
	 */
	bizCode: number;
	/**
	 * 是否應該重新整理訂單（換成下一步按鈕）。僅「狀態已往前 / 重複動作 /
	 * 物流商處理中」等 stale-state 情境為 true（5003 / 5012 / 5013 / 5047）；
	 * 硬失敗（叫件失敗 / 未取號 / 未開通…）維持畫面並顯示真因，不 reload，
	 * 以免重整把錯誤橫幅蓋掉。caller 應改判 `shouldReload`，而非反推 `bizCode === 409`
	 * ——後端對所有物流業務錯誤統一回 HTTP 409，409 本身無法區分情境。
	 */
	shouldReload: boolean;
}

/**
 * 把物流操作的錯誤分類成「給賣家看的友善結果」。
 *
 * ⚠️ 關鍵前提：Gateway 對**所有**物流業務錯誤統一回 HTTP 409 + envelope `[5xxx]`
 * 前綴（`GrpcErr` 全站慣例 → 未登錄業務碼落 FailedPrecondition → 409）。因此
 * 「該不該 reload」「要顯示什麼訊息」**必須看真正的業務碼**，不能用 HTTP 409 本身
 * 一概而論——否則像 5044（叫件失敗，PayUni E072 真因）會被當成「狀態已變更，重整」
 * 而把真因吞掉。reload 決策與 message 決策在此**解耦**：
 *   - stale-state（5003 已取號 / 5012 已叫過 / 5013 併發 / 5047 物流商處理中）
 *     → `shouldReload: true`，重整換成下一步按鈕；
 *   - 待建檔（5002 物流單尚未取得 PayUni ShipTradeNo）
 *     → 告知稍候刷新，不再顯示「無法操作」這種像是使用者順序錯的原始訊息；
 *   - 硬失敗（5044 叫件失敗 / 5011 未取號 / 5004 未開通 / 5001 / 5048…）
 *     → `shouldReload: false`，維持畫面並把 PayUni 真因（§2373 透傳）顯示給賣家。
 *
 * Drop-in replacement for Flutter's `handleShipmentApiError(...)`.
 */
export function classifyShipmentError(
	err: unknown,
	orderNo?: string,
): ShipmentErrorOutcome {
	const bizCode = shipmentBusinessCode(err);

	switch (bizCode) {
		case 5001:
			return {
				handled: true,
				bizCode,
				tone: "error",
				shouldReload: false,
				message: "此筆訂單異常（找不到物流資料），請聯絡客服處理",
			};
		case 5002:
			if (orderNo) markShipmentCreated(orderNo);
			return {
				handled: true,
				bizCode,
				tone: "warning",
				shouldReload: false,
				message:
					"物流單還在後端建檔中，請稍候刷新後再預約取號；若持續失敗請聯絡客服",
			};
		// stale-state：狀態已往前 / 重複動作 / 併發。重整即可換成正確的下一步
		// 按鈕（已叫件 → 下載 PDF），訊息直接透傳後端說明（如「已呼叫過黑貓，
		// 不可重複呼叫」），比舊版籠統的「可能已處理過」精準。
		case 5003:
		case 5012:
		case 5013:
			return {
				handled: true,
				bizCode,
				tone: "warning",
				shouldReload: true,
				message: friendlyShipmentMessage(err),
			};
		case 5046:
		case 5047: {
			if (orderNo) {
				markShipmentCreated(orderNo);
				incrementShipmentRetryCount(orderNo);
			}
			// 5047 物流商處理中：後端會自行補同步，重整可看到最新狀態；
			// 5046 建立未滿 30 秒：只是太快，維持畫面等冷卻，不需重整。
			const shouldReload = bizCode === 5047;
			if (orderNo && isShipmentBackendStuck(orderNo)) {
				return {
					handled: true,
					bizCode,
					tone: "warning",
					shouldReload,
					message: "物流商通訊異常處理中，請稍後（數分鐘）再試或聯絡客服",
				};
			}
			const remaining = orderNo
				? shipmentCooldownRemainingSeconds(orderNo)
				: 30;
			if (bizCode === 5046) {
				return {
					handled: true,
					bizCode,
					tone: "info",
					shouldReload: false,
					message:
						remaining > 0
							? `物流單剛建立，請等 ${remaining} 秒後再操作`
							: "物流單剛建立，請稍候再操作",
				};
			}
			return {
				handled: true,
				bizCode,
				tone: "info",
				shouldReload,
				message:
					remaining > 0
						? `系統處理中（PayUni 通訊中），請等 ${remaining} 秒後再試`
						: "系統處理中，請稍候再試",
			};
		}
		// 黑貓叫件失敗（5044）：PayUni 真因（E072 寄件地址不在簽約可收貨範圍 /
		// 假日不收件 / 配額等）直接透傳給賣家，讓賣家知道要改寄件地址或改時段。
		// E072 是多因通用碼，不再猜測成「假日」——以 PayUni 實際回傳的文字為準。
		case 5044:
			return {
				handled: true,
				bizCode,
				tone: "error",
				shouldReload: false,
				message: friendlyShipmentMessage(err),
			};
		case 5048:
			return {
				handled: true,
				bizCode,
				tone: "error",
				shouldReload: false,
				message: friendlyShipmentMessage(err),
			};
		default:
			// 其他物流業務碼（5004 未開通 / 5011 未取號 / 日期驗證…）後端訊息本身
			// 即可行動，交給 caller 補上動作前綴後顯示。
			return {
				handled: false,
				bizCode,
				tone: "error",
				shouldReload: false,
				message: friendlyShipmentMessage(err),
			};
	}
}

/** 黑貓營業所查詢頁——賣家可自行就近找站所或預約取件 */
export const TCAT_BRANCH_LOOKUP_URL =
	"https://www.t-cat.com.tw/consign/info.aspx";

export interface ShipmentErrorGuidance {
	/** 可能原因（讓賣家知道這不是系統壞掉，而是有具體成因） */
	cause: string;
	/** 建議的處理步驟（依序） */
	steps: string[];
	/** 相關外部連結（如黑貓官網） */
	links: { label: string; href: string }[];
}

/**
 * 針對特定物流錯誤補上「為什麼會這樣 + 可以怎麼做」的引導，讓賣家不會只看到
 * 一句後端真因就卡住。目前涵蓋黑貓 E072「寄件地址不在簽約可收貨範圍，請聯繫
 * 簽約站所」——這通常代表寄件地址有誤或不在宅配收件範圍。寄件地址於金流開通時
 * 鎖定（見 ShippingClient「地址已鎖定，如需變更請聯繫 PayUni 客服」），故修改
 * 地址須自行聯繫 PayUni 客服；急件則可就近找黑貓營業所／用黑貓官網 App 預約取件。
 * 其他錯誤回 null（不額外引導）。
 *
 * 以 PayUni 透傳的真因文字（friendlyShipmentMessage 後）判定，而非單看 bizCode：
 * E072 是多因通用碼（假日不收件 / 配額 / 地址…），只有文字確實提到「簽約 /
 * 收貨範圍」才是地址問題，避免對假日等其他成因顯示錯誤引導。
 */
export function shipmentErrorGuidance(
	outcome: Pick<ShipmentErrorOutcome, "message">,
): ShipmentErrorGuidance | null {
	const text = outcome.message ?? "";
	const isAddressOutOfRange = /簽約|可收貨範圍|收貨範圍|收件範圍|站所/.test(
		text,
	);
	if (isAddressOutOfRange) {
		return {
			cause:
				"通常代表商店的寄件地址有誤，或該地址不在黑貓宅配的簽約收件範圍內。",
			steps: [
				"寄件地址於開通時鎖定，如需確認或修改，請自行聯繫 PayUni 客服協助。",
				"若無法即時修改、又想盡快出貨，可直接聯繫鄰近的黑貓營業所安排取件。",
				"也可透過黑貓官網或手機 App 自行預約取件。",
			],
			links: [{ label: "查詢黑貓營業所", href: TCAT_BRANCH_LOOKUP_URL }],
		};
	}
	return null;
}

/* ===== 5046 / 5047 cooldown + odno override (in-memory, per-tab) ===== */

const SHIPMENT_COOLDOWN_MS = 30_000;
const SHIPMENT_BACKEND_STUCK_THRESHOLD = 3;

const shipmentCreatedAt = new Map<string, number>();
const shipmentRetryCount = new Map<string, number>();
const localOdnoOverride = new Map<
	string,
	{ odno: string; validationNo: string }
>();

export function isUnderShipmentCooldown(orderNo: string): boolean {
	const t = shipmentCreatedAt.get(orderNo);
	if (!t) return false;
	return Date.now() - t < SHIPMENT_COOLDOWN_MS;
}

export function shipmentCooldownRemainingSeconds(orderNo: string): number {
	const t = shipmentCreatedAt.get(orderNo);
	if (!t) return 0;
	const remaining = Math.ceil((SHIPMENT_COOLDOWN_MS - (Date.now() - t)) / 1000);
	return remaining > 0 ? remaining : 0;
}

/** Start the 30s cooldown after a successful generate_home_label / print_cvs_label. */
export function markShipmentCreated(orderNo: string): void {
	shipmentCreatedAt.set(orderNo, Date.now());
}

export function incrementShipmentRetryCount(orderNo: string): void {
	shipmentRetryCount.set(orderNo, (shipmentRetryCount.get(orderNo) ?? 0) + 1);
}

export function resetShipmentRetryCount(orderNo: string): void {
	shipmentRetryCount.delete(orderNo);
}

export function isShipmentBackendStuck(orderNo: string): boolean {
	return (
		(shipmentRetryCount.get(orderNo) ?? 0) >= SHIPMENT_BACKEND_STUCK_THRESHOLD
	);
}

/**
 * Frontend pre-check before firing generate_home_label / print_cvs_label.
 * Returns `null` if it's OK to proceed, or a message to show as a toast.
 */
export function guardShipmentCooldown(orderNo: string): string | null {
	if (!isUnderShipmentCooldown(orderNo)) return null;
	if (isShipmentBackendStuck(orderNo)) {
		return "物流商通訊異常處理中，請稍後（數分鐘）再試或聯絡客服";
	}
	const remaining = shipmentCooldownRemainingSeconds(orderNo);
	return `物流單剛建立，請等 ${remaining} 秒後再試`;
}

/**
 * Workaround for B-147: print_cvs_label returns odno + validationNo
 * synchronously, but the order list endpoint is on a read replica that lags
 * behind, so the freshly-printed odno doesn't appear for a few seconds.
 * We stash the values client-side and prefer them when rendering, so the
 * button immediately flips to "出貨確認" rather than staying on "預約取號".
 */
export function setLocalOdnoOverride(
	orderNo: string,
	odno: string,
	validationNo: string,
): void {
	localOdnoOverride.set(orderNo, { odno, validationNo });
}

export function effectiveOdno(orderNo: string, backendOdno?: string): string {
	if (backendOdno && backendOdno.length > 0) return backendOdno;
	return localOdnoOverride.get(orderNo)?.odno ?? "";
}

export function effectiveValidationNo(
	orderNo: string,
	backendValidationNo?: string,
): string {
	if (backendValidationNo && backendValidationNo.length > 0) {
		return backendValidationNo;
	}
	return localOdnoOverride.get(orderNo)?.validationNo ?? "";
}

/* ===== Button state machine — shared by list row + detail page ===== */

export type ShipmentAction =
	| { kind: "tcat-reserve" }
	| { kind: "tcat-call" }
	| { kind: "tcat-pdf"; labelUrl: string }
	| { kind: "cvs-print" }
	| { kind: "cvs-confirm"; odno: string; validationNo: string }
	| { kind: "none" }
	/**
	 * 🔴 認不得的配送方式。**不是** `none`：`none` 是「這張單現在沒事可做」，
	 * 這個是「我不知道這張單該怎麼出貨」。兩者分開才擋得住舊行為——舊版把所有
	 * 非黑貓的方式一律當成 7-11，於是一張大宗寄倉／商家自送的單會畫出
	 * 「預約取號」按鈕，按下去真的打 `POST /app/shipments/{no}/print_cvs_label`。
	 */
	| { kind: "unsupported"; method: string };

/** 有對應出貨 API、可以真的按下去的動作種類。 */
export type RunnableShipmentActionKind = Exclude<
	ShipmentAction["kind"],
	"none" | "unsupported"
>;

/**
 * 這個動作可不可以真的執行。`none`（無事可做）與 `unsupported`（未知配送方式）
 * 都不可以——後者尤其不能，它代表前端不知道要打哪一支 API。
 */
export function isRunnableShipmentAction(
	action: ShipmentAction,
): action is Extract<ShipmentAction, { kind: RunnableShipmentActionKind }> {
	return action.kind !== "none" && action.kind !== "unsupported";
}

export interface ShipmentLike {
	method?: string;
	labelUrl?: string;
	odno?: string;
	validationNo?: string;
	shipmentStatus?: string;
	labelExpireAt?: string | number;
}

/**
 * 託運單／寄件代碼自取號起 4 天內有效（黑貓與 7-11 相同；對齊 App
 * order_expanded_section.dart 的 `_isLabelExpired`，文案「請於代碼產生後
 * 4 天內完成寄件，逾期代碼將失效」）。後端以 `labelExpireAt` 給絕對期限
 * —— ISO 字串或 unix 秒/毫秒都接受；沒帶此欄位的舊單視為未過期，避免誤殺。
 */
export function isLabelExpired(
	shipping: Pick<ShipmentLike, "labelExpireAt"> | undefined,
	now: number = Date.now(),
): boolean {
	const raw = shipping?.labelExpireAt;
	if (raw == null || raw === "") return false;
	let ms: number;
	if (typeof raw === "number") {
		ms = raw < 1e12 ? raw * 1000 : raw;
	} else if (/^\d+$/.test(raw)) {
		const n = Number(raw);
		ms = n < 1e12 ? n * 1000 : n;
	} else {
		ms = Date.parse(raw);
	}
	if (!Number.isFinite(ms)) return false;
	return now > ms;
}

/**
 * True when a shipping method is 黑貓宅配 (TAQBIN).
 *
 * ⚠️ 這是個**肯定**判斷（是不是黑貓），不是分流器。回 `false` 只代表「不是黑貓」，
 * **不代表「是超商」** —— 曾經被當成後者用，那正是 3003／3004／3005 被靜默畫成
 * 7-11 的單點根因。要分流請用 `nextShipmentAction`／`normalizeShippingMethod`。
 *
 * 別名（"TAQBIN"／"home"／"宅配"）的正規化統一在 `lib/codemeta/enums.ts`。
 */
export function isTaqbinMethod(method: string | undefined): boolean {
	return normalizeShippingMethod(method) === ShippingMethod.TAQBIN;
}

/**
 * True when a 黑貓 order has already been handed to the driver
 * (shipmentStatus = 210 TCAT_CALLED). The order's top-level status is still
 * preparing(20) at this point, so the order list groups it into 配送中 by this
 * predicate rather than letting it vanish from both 待出貨 and 配送中 — mirrors
 * the seller APP's `order_tab_mapper.dart`.
 */
export function isTcatCalled(shipping: ShipmentLike | undefined): boolean {
	if (!shipping) return false;
	return (
		isTaqbinMethod(shipping.method) &&
		shipping.shipmentStatus === SHIPMENT_STATUS.TCAT_CALLED
	);
}

/**
 * Mirrors `_buildShippingButtons(...)` in order_bottom_row.dart. Decides the
 * single primary CTA for a row/detail. Caller passes the orderNo so we can
 * fold in the odno override.
 *
 * @param orderStatus  current order status string ("preparing" | "shipping" | …)
 *                     – we hide actions once it's past preparing/shipping.
 */
export function nextShipmentAction(
	orderNo: string,
	shipping: ShipmentLike | undefined,
	orderStatus: string,
): ShipmentAction {
	if (!shipping) return { kind: "none" };
	// 出貨 CTA 只在訂單仍由賣家負責出貨時才有意義 —— status 10(已付款)/20(待出貨)。
	// 一旦進入配送中/已送達/完成/取消，殘留的 odno 或 labelUrl 不能再讓
	// 「出貨確認」「呼叫黑貓」按鈕復活（對齊 App order_list_screen.dart 只在
	// 10/20 顯示出貨按鈕）。黑貓已叫件的訂單 status 仍停在 preparing(20) 直到司機
	// 取件，因此其「下載 PDF」CTA 仍會通過此 gate。
	if (orderStatus !== "paid" && orderStatus !== "preparing") {
		return { kind: "none" };
	}
	// 🔴 provider 分流一律 switch + default。舊版是
	// `if (isTaqbinMethod) {黑貓} else {超商}` —— 那個 else 吞掉了 3003、
	// 3004 大宗寄倉、3005 商家自送與後端未來新增的每一個 provider。
	switch (normalizeShippingMethod(shipping.method)) {
		case ShippingMethod.TAQBIN:
			return taqbinAction(shipping);
		case ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP:
			return cvsAction(orderNo, shipping);
		default:
			// 未知配送方式：不猜、不隱藏，交給畫面顯示「尚未支援」＋原碼。
			return { kind: "unsupported", method: shipping.method ?? "" };
	}
}

/** 黑貓宅配的下一步 CTA。 */
function taqbinAction(shipping: ShipmentLike): ShipmentAction {
	const labelUrl = shipping.labelUrl ?? "";
	const ss = shipping.shipmentStatus ?? "";
	// 已叫件 → 只剩下載託運單 PDF。代碼 4 天內有效（對齊 App）：過期就
	// 不再提供下載 —— 點了也只會拿到失效的託運單。
	if (ss === SHIPMENT_STATUS.TCAT_CALLED) {
		if (labelUrl && !isLabelExpired(shipping)) {
			return { kind: "tcat-pdf", labelUrl };
		}
		return { kind: "none" };
	}
	// 尚未取號就不能呼叫黑貓：後端會回 5011「尚未取號，請先產生 PDF」。
	// 取號完成（= generateHomeLabel 成功）才會有出貨單編號 + 託運單，且狀態
	// 會離開「未處理(91)」。因此只有「託運單已產生 **且** 已取號」才顯示
	// 呼叫黑貓；否則一律先走「預約出貨」產生 PDF 並取號。
	//
	// 注意：不能只靠 labelUrl，後端清單可能在尚未取號時就帶回 labelUrl，
	// 導致誤顯示呼叫黑貓。狀態未知(空字串)時也偏向「預約出貨」——重跑
	// generateHomeLabel 在已取號時只會被後端擋下(已取號/5046)並可復原，
	// 比硬噴 5011 安全。
	const numbered =
		labelUrl !== "" && ss !== "" && ss !== SHIPMENT_STATUS.UNPROCESSED;
	if (numbered) return { kind: "tcat-call" };
	return { kind: "tcat-reserve" };
}

/** 7-11 店到店（C2C）的下一步 CTA。 */
function cvsAction(orderNo: string, shipping: ShipmentLike): ShipmentAction {
	const odno = effectiveOdno(orderNo, shipping.odno);
	if (odno) {
		const validationNo = effectiveValidationNo(orderNo, shipping.validationNo);
		return { kind: "cvs-confirm", odno, validationNo };
	}
	return { kind: "cvs-print" };
}
