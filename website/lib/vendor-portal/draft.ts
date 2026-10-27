/**
 * 站外業者回價**草稿**的欄位模型（#2538 wire 契約，17 欄）。
 *
 * ⛔ 草稿不是正式提交。v6.0 §5.2 的 assurance 梯子把兩件事分開：儲存草稿只需要
 * `INVITE_ONLY`（業者點邀請信進來就有），建立不可變的正式回價版本需要
 * `GOOGLE_VERIFIED`（另一條線，尚未實作）。所以本模組刻意**不做**正式提交那層
 * 驗證——金額恆等式（`422 QUOTE_AMOUNT_INCONSISTENT`）、品相必填
 * （`422 QUOTE_QUALITY_REQUIRED`）、`validUntil` 必須是未來，全部是提交那條線
 * 的事，後端在草稿端點也不驗。草稿的意義就是「還沒想清楚也能先存」，把提交的
 * 驗證搬到草稿上會直接毀掉這個用途。
 *
 * 金額一律 decimal string 不用 number：JS 的 number 是 binary64，`19.99` 存進去
 * 就已經不是 `19.99`，而這些字串最後會變成買家看到的成交價。
 */
import {
	ProcurementSellerResponseType,
	ProcurementTaxInclusionBasis,
} from "@/lib/codemeta/generated";
import { taipeiParts } from "@/lib/date/taipei";

export interface VendorOfferedQuality {
	grade: string;
	inspectionNotes: string;
}

export interface VendorUnitBasis {
	quantity: string;
	unit: string;
}

/** 契約的 17 欄，逐欄照抄；全部 string。 */
export interface VendorResponseDraft {
	responseType: string;
	offeredQuality: VendorOfferedQuality;
	unitBasis: VendorUnitBasis;
	offeredQuantity: string;
	offeredQuantityUnit: string;
	conversionBasis: string;
	currency: string;
	goodsAmount: string;
	shippingAmount: string;
	taxAmount: string;
	taxInclusionBasis: string;
	totalAmount: string;
	leadTimeDays: string;
	promisedDeliveryAt: string;
	validUntil: string;
	termsNote: string;
	deviationDisclosure: string;
}

/**
 * 還沒有草稿時的起始值。三個有預設值的欄位都取自 CodeMeta 生成的值域常數，不自己
 * 寫字面值——值域改名時應該是編譯失敗，不是靜默送出一個後端不認得的字串。
 */
export function emptyVendorResponseDraft(): VendorResponseDraft {
	return {
		responseType: ProcurementSellerResponseType.MANUAL_QUOTE,
		offeredQuality: { grade: "", inspectionNotes: "" },
		unitBasis: { quantity: "", unit: "" },
		offeredQuantity: "",
		offeredQuantityUnit: "",
		conversionBasis: "",
		currency: "TWD",
		goodsAmount: "",
		shippingAmount: "",
		taxAmount: "",
		taxInclusionBasis: ProcurementTaxInclusionBasis.TAX_EXCLUDED,
		totalAmount: "",
		leadTimeDays: "",
		promisedDeliveryAt: "",
		validUntil: "",
		termsNote: "",
		deviationDisclosure: "",
	};
}

/**
 * 有限數字也收：契約寫明全部是 string，但真收到 number 時把它丟掉等於吃掉業者
 * 填過的金額，比原樣轉成字串更糟。其餘型別（null／物件／undefined）一律空字串
 * ——受控欄位吃到 `undefined` 會在使用者第一次輸入時從 uncontrolled 跳成
 * controlled，React 會警告而且游標位置會亂掉。
 */
function asText(value: unknown): string {
	if (typeof value === "string") return value;
	if (typeof value === "number" && Number.isFinite(value)) return String(value);
	return "";
}

function asRecord(value: unknown): Record<string, unknown> {
	return typeof value === "object" && value !== null
		? (value as Record<string, unknown>)
		: {};
}

/**
 * 把後端回聲（或任何殘缺的既有草稿）補成完整 17 欄。
 *
 * 缺欄一律補**空字串**而不是補預設值：`responseType` 例外——它是列舉，空字串在
 * `<select>` 上沒有對應選項會顯示成空白列。列舉缺席時退回 `emptyVendorResponseDraft()`
 * 的值，其餘欄位不猜。
 */
export function normalizeVendorResponseDraft(
	raw: unknown,
): VendorResponseDraft {
	const source = asRecord(raw);
	const fallback = emptyVendorResponseDraft();
	const quality = asRecord(source.offeredQuality);
	const basis = asRecord(source.unitBasis);
	return {
		responseType: asText(source.responseType) || fallback.responseType,
		offeredQuality: {
			grade: asText(quality.grade),
			inspectionNotes: asText(quality.inspectionNotes),
		},
		unitBasis: {
			quantity: asText(basis.quantity),
			unit: asText(basis.unit),
		},
		offeredQuantity: asText(source.offeredQuantity),
		offeredQuantityUnit: asText(source.offeredQuantityUnit),
		conversionBasis: asText(source.conversionBasis),
		currency: asText(source.currency) || fallback.currency,
		goodsAmount: asText(source.goodsAmount),
		shippingAmount: asText(source.shippingAmount),
		taxAmount: asText(source.taxAmount),
		taxInclusionBasis:
			asText(source.taxInclusionBasis) || fallback.taxInclusionBasis,
		totalAmount: asText(source.totalAmount),
		leadTimeDays: asText(source.leadTimeDays),
		promisedDeliveryAt: asText(source.promisedDeliveryAt),
		validUntil: asText(source.validUntil),
		termsNote: asText(source.termsNote),
		deviationDisclosure: asText(source.deviationDisclosure),
	};
}

const DECIMAL = /^-?\d+(?:\.\d+)?$/;

function fractionDigits(value: string): number {
	const dot = value.indexOf(".");
	return dot === -1 ? 0 : value.length - dot - 1;
}

/** 依 `scale` 位小數把 decimal string 轉成整數放大值，避免 binary64 的 0.1+0.2。 */
function scaleDecimal(value: string, scale: number): bigint {
	const negative = value.startsWith("-");
	const body = negative ? value.slice(1) : value;
	const [integer, fraction = ""] = body.split(".");
	const digits = BigInt(`${integer}${fraction.padEnd(scale, "0")}`);
	return negative ? -digits : digits;
}

/**
 * 金額恆等式的**提示**結果。`unknown` 是「四欄裡有還沒填或不是合法 decimal 的」
 * ——那時什麼都不該說：空字串到底是「0」還是「還沒填」只有業者知道，替他猜一個
 * 再據此喊不相等，就是把草稿當提交在驗。
 */
export type DraftAmountBalance = "unknown" | "balanced" | "unbalanced";

/**
 * ⛔ 這是提示不是驗證。回 `"unbalanced"` 時畫面可以說「加起來不等於總金額」，
 * 但**不得**擋住儲存——恆等式是正式提交（`422 QUOTE_AMOUNT_INCONSISTENT`）才驗的。
 */
export function checkDraftAmountBalance(
	draft: VendorResponseDraft,
): DraftAmountBalance {
	const parts = [
		draft.goodsAmount,
		draft.shippingAmount,
		draft.taxAmount,
		draft.totalAmount,
	].map((value) => value.trim());
	if (!parts.every((value) => DECIMAL.test(value))) return "unknown";

	const scale = Math.max(...parts.map(fractionDigits));
	const [goods, shipping, tax, total] = parts.map((value) =>
		scaleDecimal(value, scale),
	);
	return goods + shipping + tax === total ? "balanced" : "unbalanced";
}

/**
 * RFC3339 判定，與平台賣家 App 同一條規則
 * （`hawkr_seller` `lib/features/procurement/presentation/seller_quote_validation.dart`
 * 的 `isRfc3339()`，#200 已上線）。
 *
 * ⛔ 下面這條 regex 與平台側**逐字同款**，不是自己寫一套寬鬆版：草稿的用途就是
 * 之後正式提交的預填，兩邊格式一旦不同，業者存的草稿永遠變不成合法提交，而正式
 * 提交還沒開放 ⇒ 這個錯要等到很久以後才會被發現，中間存下來的草稿全都得重填。
 *
 * 位移接受 `Z` 或 `±hh:mm` 兩種，所以前端不必先決定「要不要轉成 UTC」。
 * （regex 之外還多一道解析檢查，比平台側嚴一點點，理由寫在函式裡。）
 */
const RFC3339 =
	/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$/;

export function isRfc3339(raw: string): boolean {
	const value = raw.trim();
	if (!RFC3339.test(value)) return false;
	// ⚠️ 這一行比平台側**嚴**，不是「對齊」。實測 Dart 的 `DateTime.tryParse`
	// 對 `2026-13-45T00:00:00Z` 會靜默進位成 2027-02-14 而不是回 null，所以
	// 賣家 App 的 `isRfc3339` 其實放行它；V8 的 ISO 解析器則會擋掉月份 > 12
	// （日期進位如 `2026-02-31` 兩邊都放行）。
	//
	// 刻意留著這個差異：會出事的方向是前端**比平台鬆**——那會讓草稿存得進去、
	// 正式提交卻被擋。前端嚴一點只會對「選擇器根本產不出來」的值多提示一次，
	// 而那種值只可能來自手工造的舊草稿，提示業者重挑正是想要的行為。
	return !Number.isNaN(new Date(value).getTime());
}

function pad2(value: number): string {
	return String(value).padStart(2, "0");
}

/**
 * 台北固定位移。整頁的時間都以台北牆上時間呈現（左欄的到期時間、儲存提示都是），
 * 這兩欄沒有理由改用瀏覽器所在時區——那會讓人在國外開同一張草稿看到不同的鐘點，
 * 而且會引進 `getFullYear` 家族（`tests/date-formatting-taipei.test.ts` 擋的那族，
 * 輸出跟著 runtime 時區跑）。台灣無日光節約，位移是常數。
 */
const TAIPEI_OFFSET = "+08:00";

const TAIPEI_INPUT = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})(?::(\d{2}))?$/;

/**
 * `<input type="datetime-local">` 的值（無時區的牆上時間）當成**台北時間**，
 * 補上 `+08:00` 變成 RFC3339。
 *
 * 不是選擇器產生的形狀、或組出來的字串過不了 `isRfc3339`（例如月份 > 12），一律
 * 原樣回傳，不猜——猜錯會把業者填的東西悄悄改成另一個時刻，比留著錯字更糟。
 * 選擇器本來就產不出這兩種值，這條是給手工造的舊草稿用的。
 */
export function taipeiInputToRfc3339(local: string): string {
	const value = local.trim();
	if (!value) return "";
	const parts = TAIPEI_INPUT.exec(value);
	if (!parts) return value;
	const [, year, month, day, hour, minute, second = "00"] = parts;
	const candidate = `${year}-${month}-${day}T${hour}:${minute}:${second}${TAIPEI_OFFSET}`;
	return isRfc3339(candidate) ? candidate : value;
}

/**
 * 反向：RFC3339 instant 轉成選擇器顯示得了的**台北**牆上時間。
 *
 * 走 `taipeiParts()` 而不是 `getFullYear` 家族，輸出因此與 runtime 時區脫鉤
 * （同 `lib/date/taipei.ts` 的理由，#374／#391）。
 *
 * 不是 RFC3339 就回空字串——讓選擇器保持空白。**呼叫端不得因此把欄位清掉**：
 * 那個舊值仍然留在草稿裡，畫面另外原樣顯示它，由業者自己決定要不要換掉。
 */
export function rfc3339ToTaipeiInput(value: string): string {
	if (!isRfc3339(value)) return "";
	const parts = taipeiParts(value.trim());
	if (!parts) return "";
	return `${String(parts.year).padStart(4, "0")}-${pad2(parts.month)}-${pad2(parts.day)}T${pad2(parts.hour)}:${pad2(parts.minute)}`;
}
