const MAX_SLOT_ANSWER_RUNES = 200;

const NUMERIC_LIMITS: Record<string, { label: string; max: number }> = {
	quantity: { label: "數量", max: 1_000_000 },
	budget_per_unit: { label: "單價預算", max: 10_000_000 },
};

/**
 * /answers 不經 LLM 抽取；quantity／budget_per_unit 會在後端直接 strconv.Atoi。
 * 因此「30 箱」在 /turns 可理解，在問題卡 /answers 卻是 422。前端在欄位旁先
 * 說清楚，仍以後端驗證為最後權威。
 */
export function validateFreeTextAnswer(
	slotKey: string,
	value: string,
): string | null {
	const answer = value.trim();
	if (!answer) return "請先輸入答案。";
	if (Array.from(answer).length > MAX_SLOT_ANSWER_RUNES) {
		return `單一條件最多 ${MAX_SLOT_ANSWER_RUNES} 字。`;
	}

	const numeric = NUMERIC_LIMITS[slotKey];
	if (!numeric) return null;
	if (!/^\d+$/.test(answer)) {
		return `${numeric.label}請只輸入數字，例如 50。`;
	}
	const parsed = Number(answer);
	if (!Number.isSafeInteger(parsed) || parsed < 1 || parsed > numeric.max) {
		return `${numeric.label}需介於 1 與 ${numeric.max.toLocaleString("zh-TW")} 之間。`;
	}
	return null;
}
