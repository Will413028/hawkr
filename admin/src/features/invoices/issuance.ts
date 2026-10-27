import type {
	InvoiceLine,
	ManualIssuanceRequest,
} from "@/features/invoices/types";
import {
	INVOICE_CARRIER_TYPE_LABEL,
	INVOICE_LINE_TYPE_LABEL,
	INVOICE_TAX_CATEGORY_LABEL,
	InvoiceCarrierType,
	InvoiceDocumentKind,
	InvoiceIssuerType,
	InvoiceLineType,
	InvoiceRecipientType,
	InvoiceTaxCategory,
} from "@/lib/codemeta";

/**
 * A11 手動開立發票（端點 57）的純函式層。
 *
 * 把矩陣、金額合計與送出前檢查抽出頁面的理由：**這一頁的每一條規則都是「做錯不會
 * 報錯、只會開出一張錯的發票」**，而發票開出去只能作廢或折讓、收不回來。純函式才
 * 測得動；混在 JSX 裡的條件式沒有任何東西守得住。
 *
 * ⚠️ 本檔一個碼值中文都不寫：label 全部查 `@/lib/codemeta` 的生成物。唯二的例外是
 * `SOURCE_KIND_LABEL`——`ORDER_SNAPSHOT` / `MANUAL_LINES` **不是 CodeMeta enum**
 * （實查 `src/lib/codemeta/generated.ts` 零命中），它們是端點 57 自己的值域，定義在
 * proto 與 `services/invoice-service/internal/usecase/admin_issuance.go` 的
 * `issuanceSourceOrderSnapshot` / `issuanceSourceManualLines` 兩個常數。
 */

/** `source.kind` 的兩個值（端點 57 專屬，非 CodeMeta）。 */
export const SOURCE_KIND_ORDER_SNAPSHOT = "ORDER_SNAPSHOT";
export const SOURCE_KIND_MANUAL_LINES = "MANUAL_LINES";

/** `source.kind` 的顯示文字。不是碼表鏡像（它沒有碼表），是這一頁自己的 UI 文案。 */
export const SOURCE_KIND_LABEL: Record<string, string> = {
	[SOURCE_KIND_ORDER_SNAPSHOT]: "沿用訂單快照",
	[SOURCE_KIND_MANUAL_LINES]: "手動輸入品項",
};

/** 品項行數上限（`admin_issuance.go` 的 `manualIssuanceMaxLines`）。 */
export const MAX_LINES = 50;
/** 理由字數上限（`admin_job.go` 的 `adminReasonMaxRunes`）。 */
export const MAX_REASON_RUNES = 500;

/**
 * 合法組合矩陣，逐列對應 `admin_issuance.go` 的 `allowedIssuanceCombos`。
 *
 * 🔴 **正面列舉**，與後端同一個方向。反過來寫（列禁止組合）會讓日後新增的
 * issuer／recipient 值自動變成允許，而那是一個沒有人會察覺的放寬。
 *
 * ⚠️ 這份是「提早給回饋」，不是授權來源：矩陣最後一列（訂單賣家 ≠ issuer.storeId）
 * 前端判斷不了（要載入訂單才知道賣家是誰）⇒ 9346 仍必須處理。
 */
export const ALLOWED_COMBOS: ReadonlyArray<{
	issuer: string;
	recipient: string;
	source: string;
	purpose: string;
}> = [
	{
		issuer: InvoiceIssuerType.SELLER,
		recipient: InvoiceRecipientType.BUYER,
		source: SOURCE_KIND_ORDER_SNAPSHOT,
		purpose: "補開一筆漏開的訂單發票",
	},
	{
		issuer: InvoiceIssuerType.SELLER,
		recipient: InvoiceRecipientType.EXTERNAL,
		source: SOURCE_KIND_MANUAL_LINES,
		purpose: "賣家對非會員買家開票（線下交易）",
	},
	{
		issuer: InvoiceIssuerType.PLATFORM,
		recipient: InvoiceRecipientType.SELLER,
		source: SOURCE_KIND_MANUAL_LINES,
		purpose: "平台服務費",
	},
	{
		issuer: InvoiceIssuerType.PLATFORM,
		recipient: InvoiceRecipientType.EXTERNAL,
		source: SOURCE_KIND_MANUAL_LINES,
		purpose: "平台對外部對象開票",
	},
];

export function isAllowedCombo(
	issuer: string,
	recipient: string,
	source: string,
): boolean {
	return ALLOWED_COMBOS.some(
		(c) =>
			c.issuer === issuer && c.recipient === recipient && c.source === source,
	);
}

/** 這個 issuer 之下，矩陣允許哪些 recipient。用來把不合法的卡片標成不可選。 */
export function allowedRecipients(issuer: string): string[] {
	return [
		...new Set(
			ALLOWED_COMBOS.filter((c) => c.issuer === issuer).map((c) => c.recipient),
		),
	];
}

/** 這組 issuer × recipient 之下，矩陣允許哪些 source。 */
export function allowedSources(issuer: string, recipient: string): string[] {
	return [
		...new Set(
			ALLOWED_COMBOS.filter(
				(c) => c.issuer === issuer && c.recipient === recipient,
			).map((c) => c.source),
		),
	];
}

/**
 * `taxCategory` 的下拉選項。
 *
 * 🔴 **wire 值是 CodeMeta 的 Code（`TAXABLE` / `ZERO_RATED` / `TAX_EXEMPT`），
 * 而 label 表的鍵是 Value（`"1"` / `"2"` / `"3"`）。** 這是本頁最容易寫錯的一處：
 * `InvoiceTaxCategory` 是唯一 Code ≠ Value 的發票碼表，而後端收的是 **Code**
 * （`admin_issuance.go` 的 `codemeta.InvoiceTaxCategoryFromCode`）。
 * 送 Value 過去會落 `ErrIssuanceLineTaxInvalid` ⇒ 整張表單被拒，而錯誤訊息只說
 * 「tax_category 不合法」。
 */
export const TAX_CATEGORY_OPTIONS: ReadonlyArray<[string, string]> = (
	Object.keys(InvoiceTaxCategory) as Array<keyof typeof InvoiceTaxCategory>
).map((code) => [
	code,
	INVOICE_TAX_CATEGORY_LABEL[InvoiceTaxCategory[code]] ?? code,
]);

/** `lineType` 的下拉選項（Code ＝ Value，直接用生成的 label 表）。 */
export const LINE_TYPE_OPTIONS: ReadonlyArray<[string, string]> = Object.values(
	InvoiceLineType,
).map((value) => [value, INVOICE_LINE_TYPE_LABEL[value] ?? value]);

/**
 * A11 的載具子集：三值。
 *
 * `TAX_ID` **刻意不在其中**——統編走 recipient 的 `taxId` 欄，兩邊都收會與
 * 「統編／載具／捐贈碼三者互斥」自打架。留空 ＝ 無載具。
 */
export const A11_CARRIER_TYPES: ReadonlyArray<[string, string]> = [
	InvoiceCarrierType.MOBILE_BARCODE,
	InvoiceCarrierType.CITIZEN_CERT,
	InvoiceCarrierType.DONATION,
].map((value) => [value, INVOICE_CARRIER_TYPE_LABEL[value] ?? value]);

/**
 * 冪等鍵。格式受 gateway `middleware.IdempotencyKeyPattern` 強制
 * （`^[A-Za-z0-9._:-]{8,128}$`），不符即 400 —— UUID 的字元集（hex ＋ `-`）落在裡面。
 *
 * ⚠️ **這一把不是後端的去重鍵。** 端點 57 的去重走 `provider_request_no`
 * ＝ 請求內容的 sha256（`admin_issuance.go` 的 `manualProviderRequestNo`，該函式的
 * 註解逐字寫明「鍵只由請求內容決定、不含 client 的 Idempotency-Key」）。
 * ⇒ 同一份內容重送必然回 9347，換不換這把鍵都一樣；這把鍵只落進稽核列的
 * `request_id`。**因此「失敗不換鍵」在本頁是稽核可追溯性的要求，不是去重的要求。**
 */
export function createInvoiceIdempotencyKey(): string {
	const uuid =
		typeof crypto !== "undefined" && typeof crypto.randomUUID === "function"
			? crypto.randomUUID()
			: `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 12)}`;
	return `inv-manual-${uuid}`;
}

/** 表單裡的一行品項。四個數值欄一律 string（`type="text" inputMode="numeric"`）。 */
export interface LineDraft {
	/** React key。與 wire 無關，不送出。 */
	key: string;
	lineType: string;
	name: string;
	quantity: string;
	unitPrice: string;
	taxCategory: string;
}

/** 嚴格整數解析：空字串、小數、前後空白、`1e3` 一律回 null（不當成 0）。 */
export function parseStrictInt(raw: string): number | null {
	if (!/^-?\d{1,15}$/.test(raw)) return null;
	const n = Number(raw);
	return Number.isSafeInteger(n) ? n : null;
}

/**
 * 一行的 `netAmount` ＝ quantity × unitPrice。
 *
 * 🔴 這個欄位**必須送**：58A §A11 的 request 範例沒有列它，但 Gateway DTO
 * （`invoice_dto.go` 的 `InvoiceLine`）有，而 invoice-service 的 `buildManualLines`
 * 逐行斷言 `net_amount == quantity × unit_price`（`chk_invline_amount`）。
 * 不送 ⇒ net 是 0 ⇒ 除非金額本來就是 0，否則整張表單被拒。
 *
 * ⚠️ 這不是「前端算稅」：稅額與應稅／零稅率／免稅銷售額的拆分一律由後端做
 * （58A §A11「權限與邊界」逐字），本函式只是一行之內的乘法。
 */
export function lineNetAmount(line: LineDraft): number | null {
	const qty = parseStrictInt(line.quantity);
	const unit = parseStrictInt(line.unitPrice);
	if (qty === null || unit === null) return null;
	return qty * unit;
}

export interface LineTotals {
	product: number;
	shipping: number;
	discount: number;
	total: number;
	/** 有任何一行的數值欄還解析不出來（合計此時只是部分和，畫面要說明）。 */
	incomplete: boolean;
}

/**
 * 即時合計。**只是輸入值的回顯，不是發票金額**：表頭三欄與稅額由後端依 58 §5A.6
 * 計算（含尾差的最大餘數處理），前端算第二份就是製造第二個真相。
 */
export function summariseLines(lines: LineDraft[]): LineTotals {
	const totals: LineTotals = {
		product: 0,
		shipping: 0,
		discount: 0,
		total: 0,
		incomplete: false,
	};
	for (const line of lines) {
		const net = lineNetAmount(line);
		if (net === null) {
			totals.incomplete = true;
			continue;
		}
		if (line.lineType === InvoiceLineType.PRODUCT) totals.product += net;
		else if (line.lineType === InvoiceLineType.SHIPPING) totals.shipping += net;
		else if (line.lineType === InvoiceLineType.DISCOUNT) totals.discount += net;
		totals.total += net;
	}
	return totals;
}

export interface IssuanceDraft {
	issuerKind: string;
	issuerStoreId: string;
	recipientKind: string;
	recipientBuyerId: string;
	recipientStoreId: string;
	recipientName: string;
	recipientTaxId: string;
	recipientEmail: string;
	carrierType: string;
	carrierValue: string;
	sourceKind: string;
	orderNo: string;
	lines: LineDraft[];
	documentKind: string;
	reason: string;
}

/**
 * 送出前的阻擋清單。回空陣列才可以送。
 *
 * ⚠️ 這是**第一道**不是唯一一道：後端每一條都會再擋一次（`admin_issuance.go`）。
 * 前端擋的價值在於「不要讓人填完整張表才被拒」，不在於代替後端判定。
 */
export function issuanceBlockers(draft: IssuanceDraft): string[] {
	const out: string[] = [];

	if (
		!isAllowedCombo(draft.issuerKind, draft.recipientKind, draft.sourceKind)
	) {
		out.push("目前的「開票人 × 受票人 × 品項來源」不是合法組合。");
	}
	if (
		draft.issuerKind === InvoiceIssuerType.SELLER &&
		draft.issuerStoreId.trim() === ""
	) {
		out.push("開票人是賣家時必須填 store_id。");
	}
	if (
		draft.recipientKind === InvoiceRecipientType.BUYER &&
		draft.recipientBuyerId.trim() === ""
	) {
		out.push("受票人是買家時必須填 buyer_id。");
	}
	if (
		draft.recipientKind === InvoiceRecipientType.SELLER &&
		draft.recipientStoreId.trim() === ""
	) {
		out.push("受票人是賣家時必須填 store_id。");
	}
	if (
		draft.recipientKind === InvoiceRecipientType.EXTERNAL &&
		draft.recipientName.trim() === ""
	) {
		out.push("受票人是非平台會員時必須填抬頭。");
	}

	// 統編／載具／捐贈碼三者互斥（違反 → 9306）。捐贈碼走 carrierType=DONATION。
	const hasTaxId = draft.recipientTaxId.trim() !== "";
	const hasCarrier = draft.carrierType !== "";
	if (hasTaxId && hasCarrier) {
		out.push("統一編號與載具（含捐贈碼）三者互斥，只能擇一。");
	}
	if (hasCarrier && draft.carrierValue.trim() === "") {
		out.push("選了載具就必須填載具內容。");
	}
	if (
		draft.documentKind === InvoiceDocumentKind.B2B &&
		!/^\d{8}$/.test(draft.recipientTaxId.trim())
	) {
		out.push("三聯式（B2B）的受票人統一編號必須是 8 位數字。");
	}
	if (draft.documentKind !== InvoiceDocumentKind.B2B && hasTaxId) {
		out.push("二聯式與收據不得帶受票人統一編號。");
	}

	if (
		draft.sourceKind === SOURCE_KIND_ORDER_SNAPSHOT &&
		draft.orderNo.trim() === ""
	) {
		out.push("沿用訂單快照時必須填訂單編號。");
	}
	if (draft.sourceKind === SOURCE_KIND_MANUAL_LINES) {
		if (draft.lines.length === 0) out.push("請至少新增一行品項。");
		if (draft.lines.length > MAX_LINES) out.push(`品項最多 ${MAX_LINES} 行。`);
		draft.lines.forEach((line, i) => {
			const seq = i + 1;
			if (line.name.trim() === "") out.push(`第 ${seq} 列缺品名。`);
			const qty = parseStrictInt(line.quantity);
			if (qty === null || qty <= 0)
				out.push(`第 ${seq} 列的數量必須是正整數。`);
			const unit = parseStrictInt(line.unitPrice);
			if (unit === null) out.push(`第 ${seq} 列的單價必須是整數。`);
			const net = lineNetAmount(line);
			if (net !== null) {
				// chk_invline_sign：DISCOUNT 行必須為負，其餘必須非負。
				if (line.lineType === InvoiceLineType.DISCOUNT && net >= 0) {
					out.push(`第 ${seq} 列是折抵，金額必須是負數。`);
				}
				if (line.lineType !== InvoiceLineType.DISCOUNT && net < 0) {
					out.push(`第 ${seq} 列不是折抵，金額不得為負數。`);
				}
			}
		});
		const totals = summariseLines(draft.lines);
		if (!totals.incomplete && totals.total <= 0) {
			// 後端 `ErrIssuanceTotalNotPositive`：零元或負數總額的發票不是「金額 0 的
			// 發票」，是一張不該存在的發票。
			out.push("合計金額必須大於 0。");
		}
	}

	const reason = draft.reason.trim();
	if (reason === "") out.push("請填寫開立理由。");
	if ([...reason].length > MAX_REASON_RUNES) {
		out.push(`理由最多 ${MAX_REASON_RUNES} 字。`);
	}
	return out;
}

/**
 * B2B ＋ 多種稅別混用的警告。**不擋送出**：後端會 fail-closed 成 `AWAITING_MANUAL`
 * （理由碼 `B2B_MIXED_TAX_UNVERIFIED`），前端擋掉等於做了一個後端沒有承諾的規則。
 */
export function hasMixedTaxCategories(draft: IssuanceDraft): boolean {
	if (draft.documentKind !== InvoiceDocumentKind.B2B) return false;
	if (draft.sourceKind !== SOURCE_KIND_MANUAL_LINES) return false;
	return new Set(draft.lines.map((l) => l.taxCategory)).size > 1;
}

/** 把表單狀態折成端點 57 的 body。空字串欄位照送（後端一律以空字串當「沒填」）。 */
export function toIssuanceRequest(draft: IssuanceDraft): ManualIssuanceRequest {
	const lines: InvoiceLine[] =
		draft.sourceKind === SOURCE_KIND_MANUAL_LINES
			? draft.lines.map((line) => ({
					lineType: line.lineType,
					name: line.name.trim(),
					quantity: line.quantity.trim(),
					unitPrice: line.unitPrice.trim(),
					netAmount: String(lineNetAmount(line) ?? 0),
					taxCategory: line.taxCategory,
				}))
			: [];
	return {
		issuer: {
			kind: draft.issuerKind,
			storeId:
				draft.issuerKind === InvoiceIssuerType.SELLER
					? draft.issuerStoreId.trim()
					: "",
		},
		recipient: {
			kind: draft.recipientKind,
			buyerId:
				draft.recipientKind === InvoiceRecipientType.BUYER
					? draft.recipientBuyerId.trim()
					: "",
			storeId:
				draft.recipientKind === InvoiceRecipientType.SELLER
					? draft.recipientStoreId.trim()
					: "",
			name: draft.recipientName.trim(),
			taxId: draft.recipientTaxId.trim(),
			email: draft.recipientEmail.trim(),
			carrierType: draft.carrierType,
			carrierValue: draft.carrierValue.trim(),
		},
		source: {
			kind: draft.sourceKind,
			orderNo:
				draft.sourceKind === SOURCE_KIND_ORDER_SNAPSHOT
					? draft.orderNo.trim()
					: "",
			lines,
		},
		documentKind: draft.documentKind,
		reason: draft.reason.trim(),
		confirmed: true,
	};
}
