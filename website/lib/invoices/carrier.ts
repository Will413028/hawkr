/**
 * 四種載具的顯示名稱與「形狀」驗證（58A §B02 的欄位表）。B02／B06／B08 共用。
 *
 * 🔴 三條規約：
 *   1. **中文名稱一律取自 CodeMeta**（§0.4 ⑤），本檔不手抄第二份。
 *   2. **display map 必須有 default 分支**（§0.6 C11）：後端先上線第五種載具時，
 *      落 default 顯示原始碼值雖然不好看，但畫面還在；沒有 default 就是白畫面，
 *      而測試用的是舊值域所以全綠。
 *   3. **只做形狀檢查，不做檢查碼**。正則與後端 `validateIdentityShape`
 *      （invoice-service `internal/usecase/checkout.go`）逐字相同，而後端的統編
 *      走 `domain.IsBusinessTaxID`——註解明寫「8 碼數字，**不驗檢查碼**」。前端
 *      比後端嚴會讓合法統編打不出發票，而那個失敗沒有任何錯誤訊息可解釋。
 */

import {
	INVOICE_CARRIER_TYPE_LABEL,
	InvoiceCarrierType,
	type InvoiceCarrierTypeValue,
} from "@/lib/codemeta/generated";

export const INVOICE_CARRIER_TYPES: readonly InvoiceCarrierTypeValue[] = [
	InvoiceCarrierType.MOBILE_BARCODE,
	InvoiceCarrierType.CITIZEN_CERT,
	InvoiceCarrierType.TAX_ID,
	InvoiceCarrierType.DONATION,
];

/**
 * `carrierType` → 中文名稱。參數是 `string` 而不是 union：wire 上的值可能比前端
 * 認得的多，收窄型別會讓 default 分支不可達。
 */
export function invoiceCarrierLabel(carrierType: string): string {
	return INVOICE_CARRIER_TYPE_LABEL[carrierType] ?? carrierType;
}

/** 這個載具型別是否認得（用來決定要不要讓使用者編輯它）。 */
export function isKnownCarrierType(
	carrierType: string,
): carrierType is InvoiceCarrierTypeValue {
	return carrierType in INVOICE_CARRIER_TYPE_LABEL;
}

/* ===== 形狀驗證（與後端逐字相同的三條正則） ===== */

const MOBILE_BARCODE_RE = /^\/[0-9A-Z+\-.]{7}$/;
const CITIZEN_CERT_RE = /^[A-Z]{2}[0-9]{14}$/;
const DONATION_RE = /^[0-9]{3,7}$/;
const TAX_ID_RE = /^[0-9]{8}$/;

/** 後端 `buyerNameMaxRunes`。以 code point 計數，不是顯示寬度。 */
export const BUYER_NAME_MAX_RUNES = 60;

/** 各型別的 `value` 欄位輸入上限（AMEGO 上限）。`TAX_ID` 不用 `value`。 */
export const CARRIER_VALUE_MAX_LENGTH: Record<string, number> = {
	[InvoiceCarrierType.MOBILE_BARCODE]: 8,
	[InvoiceCarrierType.CITIZEN_CERT]: 16,
	[InvoiceCarrierType.DONATION]: 7,
};

export const TAX_ID_MAX_LENGTH = 8;

export type CarrierShapeError =
	| "mobileBarcode"
	| "citizenCert"
	| "donation"
	| "taxId"
	| "buyerName";

/**
 * 驗 `value` 欄位的形狀。通過回 `null`，否則回一個 key——文案由畫面層決定，
 * 因為同一個錯誤在 Sheet（B02）與清單頁（B08）的擺放位置不同。
 *
 * `TAX_ID` 不使用 `value`（後端要求該欄必須為空），所以恆回 `null`。
 */
export function validateCarrierValue(
	carrierType: string,
	value: string,
): CarrierShapeError | null {
	switch (carrierType) {
		case InvoiceCarrierType.MOBILE_BARCODE:
			return MOBILE_BARCODE_RE.test(value) ? null : "mobileBarcode";
		case InvoiceCarrierType.CITIZEN_CERT:
			return CITIZEN_CERT_RE.test(value) ? null : "citizenCert";
		case InvoiceCarrierType.DONATION:
			return DONATION_RE.test(value) ? null : "donation";
		case InvoiceCarrierType.TAX_ID:
			return null;
		default:
			// 前端還不認得的型別：沒有正則可套，交給後端驗。不要在這裡回錯誤——
			// 那會讓一個後端已經支援的新載具在前端永遠送不出去。
			return null;
	}
}

/** 驗統一編號。8 碼數字，不驗檢查碼（與後端一致，見檔頭第 3 條）。 */
export function validateTaxId(taxId: string): CarrierShapeError | null {
	return TAX_ID_RE.test(taxId) ? null : "taxId";
}

/** 驗公司抬頭。1–60 個 code point，與後端 `utf8.RuneCountInString` 同一把尺。 */
export function validateBuyerName(buyerName: string): CarrierShapeError | null {
	const runes = [...buyerName.trim()].length;
	return runes >= 1 && runes <= BUYER_NAME_MAX_RUNES ? null : "buyerName";
}

/**
 * 這個型別的 `displayValue` 是不是遮罩過的。
 *
 * 後端 `preferenceDisplayValue` 只對兩種載具遮罩；`TAX_ID` 回統編、`DONATION`
 * 回捐贈碼，兩者本來就不是秘密。編輯表單靠這個決定「能不能把原值帶回欄位」——
 * 帶不回來時必須明說「請重新輸入」，否則使用者以為系統把資料弄丟了。
 *
 * 前端還不認得的型別一律當成遮罩（要求重新輸入）：那是壞掉時比較安全的一邊。
 */
export function carrierValueIsMasked(carrierType: string): boolean {
	return (
		carrierType !== InvoiceCarrierType.TAX_ID &&
		carrierType !== InvoiceCarrierType.DONATION
	);
}
