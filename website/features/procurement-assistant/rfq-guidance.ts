const SERVER_MONEY = /^(?:0|[1-9]\d{0,11})(?:\.\d{1,2})?$/;

function isServerMoney(value: unknown): value is string {
	return typeof value === "string" && SERVER_MONEY.test(value);
}

export const RFQ_TEMPERATURE_UNKNOWN_COPY = "成交溫度計：未知";

export const RFQ_TEMPERATURE_BAND_LABEL: Record<
	"COLD" | "WARM" | "HOT",
	string
> = {
	COLD: "冷",
	WARM: "溫",
	HOT: "熱",
};

export const RFQ_TEMPERATURE_UNKNOWN_DETAIL: Record<string, string> = {
	BUYER_RANGE_NOT_SET: "調整希望詢價區間以查看出價位置。",
	MAPPING_MISSING: "沒有可靠的市場行情映射，無法判斷出價位置。",
	RANGE_OUTSIDE_REFERENCE:
		"你的希望區間完全落在市場參考區間之外，不判斷出價位置。",
	QUANTILES_NOT_MONOTONIC: "市場行情資料異常，暫不判斷出價位置。",
};

export const RFQ_TEMPERATURE_UNKNOWN_FALLBACK =
	"目前尚無足夠資料判斷出價位置。";

export function validateBuyerDesiredRange(
	lower: string,
	upper: string,
): string | null {
	const normalizedLower = lower.trim();
	const normalizedUpper = upper.trim();
	if (!normalizedLower && !normalizedUpper) return null;
	if (
		(normalizedLower &&
			(!isServerMoney(normalizedLower) || Number(normalizedLower) <= 0)) ||
		(normalizedUpper &&
			(!isServerMoney(normalizedUpper) || Number(normalizedUpper) <= 0))
	) {
		return "請輸入大於 0 的新台幣金額，最多保留兩位小數。";
	}
	if (
		normalizedLower &&
		normalizedUpper &&
		Number(normalizedLower) > Number(normalizedUpper)
	) {
		return "希望上限不可低於下限。";
	}
	return null;
}
