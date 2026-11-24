import type { PreviewRfqPriceResponse } from "@hawkr/api/gen/hawkr/procurement/v1alpha1/procurement_pb";
import type { ProcurementFailure } from "./errors";

/** 買家預期的單價範圍，金額是新台幣元的整數，以十進位字串承載。 */
export interface RfqBuyerRequestedRange {
	lowerBoundAmount: string;
	upperBoundAmount: string;
	/** 單價的計價基準：每 basisQuantity 個 basisUnit，由伺服器依需求的單位填。 */
	basisQuantity: string;
	basisUnit: string;
}

/** 伺服器依單價範圍與需求數量算出的詢價總額區間；client 不自己乘，計價基準因單位而異。 */
export interface RfqQuoteTotalRange {
	lowerBoundAmount: string;
	upperBoundAmount: string;
	quantity: string;
	quantityUnit: string;
}

/** 純計算的價格試算：不寫入任何資料，所以沒有識別碼可以引用。 */
export interface RfqPricePreview {
	buyerRequestedRange: RfqBuyerRequestedRange;
	/** 需求沒有填數量時是 null；不回 0，也不把單價當總額。 */
	quoteTotalRange: RfqQuoteTotalRange | null;
}

export function pricePreviewFromProto(
	res: PreviewRfqPriceResponse,
): RfqPricePreview | null {
	const range = res.buyerRequestedRange;
	if (!range) return null;
	const total = res.quoteTotalRange;
	return {
		buyerRequestedRange: {
			lowerBoundAmount: String(range.lowerBoundAmount),
			upperBoundAmount: String(range.upperBoundAmount),
			basisQuantity: range.basisQuantity > 0 ? String(range.basisQuantity) : "",
			basisUnit: range.basisUnit,
		},
		quoteTotalRange: total
			? {
					lowerBoundAmount: String(total.lowerBoundAmount),
					upperBoundAmount: String(total.upperBoundAmount),
					quantity: String(total.quantity),
					quantityUnit: total.quantityUnit,
				}
			: null,
	};
}

export interface RfqPricePreviewCommand {
	expectedRevisionId: string;
	buyerRequestedRange: {
		lowerBoundAmount: string;
		upperBoundAmount: string;
	};
}

/** 給供應商的補充說明字數上限（客戶端先擋；契約上限是 500 字）。 */
export const BUYER_NOTE_MAX_LENGTH = 200;

export type RfqCommandFailure = ProcurementFailure;

export type RfqCommandResult<T> =
	| { ok: true; data: T }
	| { ok: false; error: RfqCommandFailure };

export function validateBuyerDesiredRange(
	lower: string,
	upper: string,
): string | null {
	const normalizedLower = lower.trim();
	const normalizedUpper = upper.trim();
	if (!normalizedLower || !normalizedUpper) {
		return "請完整填寫希望下限與上限。";
	}
	if (!/^[1-9]\d{0,11}$/.test(normalizedLower)) {
		return "希望下限須為大於 0 的新台幣整數。";
	}
	if (!/^[1-9]\d{0,11}$/.test(normalizedUpper)) {
		return "希望上限須為大於 0 的新台幣整數。";
	}
	if (BigInt(normalizedLower) > BigInt(normalizedUpper)) {
		return "希望上限不可低於下限。";
	}
	return null;
}

/** 伺服器是否給了完整的計價基準；缺了就只能檢視，不能往下發布。 */
export function hasPriceBasis(preview: RfqPricePreview): boolean {
	const { basisQuantity, basisUnit } = preview.buyerRequestedRange;
	return basisQuantity !== "" && basisUnit !== "";
}
