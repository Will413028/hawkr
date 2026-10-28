const SERVER_MONEY = /^(?:0|[1-9]\d{0,11})(?:\.\d{1,2})?$/;

function isServerMoney(value: unknown): value is string {
	return typeof value === "string" && SERVER_MONEY.test(value);
}

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
