/**
 * 計算字串的「顯示長度」，對齊後端 PayUni 物流欄位約束：
 *   - 半形 / 英數（code point <= 0x7F）算 1
 *   - 全形 / 中文（其餘 code point）算 2
 *
 * 逐 code point 計算（用 `for...of` 走訪而非 `.length`），避免 emoji /
 * 補充平面字元被當成兩個 UTF-16 code unit 重複計數。後端以同一規則
 * （半形 1、全形 2）驗證收件人 / 寄件人姓名與地址長度，前端先擋可避免
 * 送出後才被 PayUni 退件（如 SHIP02080）。
 */
export function displayLength(input: string): number {
	let total = 0;
	for (const ch of input) {
		const cp = ch.codePointAt(0) ?? 0;
		total += cp <= 0x7f ? 1 : 2;
	}
	return total;
}

/** 顯示長度規則說明文案，附在錯誤訊息後讓使用者理解計數方式。 */
const DISPLAY_LENGTH_HINT = "（中文 1 字算 2、英文 1 字算 1）";

export interface DisplayLengthOptions {
	/** 欄位中文名稱，用於錯誤訊息（如「收件人姓名」）。 */
	label: string;
	/** 顯示長度下限（含）。 */
	min: number;
	/** 顯示長度上限（含）。 */
	max: number;
}

/**
 * 純函式版本（非 zod）— 給以 useState 管理表單、未走 zod resolver 的元件
 * （如賣家物流設定、超商取貨 picker）在送出前自行檢查用；也供 zod
 * 物件層 `.superRefine` 內部複用，避免在欄位上包 refine 改變型別推導
 * （zod v4 包 refine 後物件型別會塌成 `never`，破壞 react-hook-form 的
 * `setValue` 型別）。
 *
 * @returns 通過回 `null`，否則回繁中錯誤訊息字串。
 */
export function checkDisplayLength(
	value: string,
	{ label, min, max }: DisplayLengthOptions,
): string | null {
	const trimmed = value.trim();
	const len = displayLength(trimmed);
	if (len < min) {
		return `${label}至少需 ${min} 個字${DISPLAY_LENGTH_HINT}`;
	}
	if (len > max) {
		return `${label}最多 ${max} 個字${DISPLAY_LENGTH_HINT}`;
	}
	return null;
}
