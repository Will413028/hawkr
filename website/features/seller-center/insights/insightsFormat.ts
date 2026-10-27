/** Shared formatting helpers for the pricebook insights feature — was
 *  duplicated verbatim across InsightsClient/MarketComparisonPanel/
 *  SevenDayTable; extracted to a single module so the three stay in sync. */
export function formatPrice(v: number | null | undefined): string {
	if (v == null) return "—";
	return `NT$ ${v.toLocaleString("zh-TW", { maximumFractionDigits: 1 })}`;
}

export function formatVolume(v: number | undefined): string {
	if (v == null) return "—";
	return `${v.toLocaleString("zh-TW", { maximumFractionDigits: 1 })} kg`;
}

/** 後端的單位代碼 → 中文標籤。
 *
 * 🔴 這張表是**顯示**用的，不是換算用的。換算一律在後端完成
 *    （pricebook 的 canonical_price），前端拿到什麼渲染什麼——同樣的原則
 *    採購助手那側早就寫死在註解裡了：「前端自己乘會錯」。
 *
 * 未知單位刻意回「單位待確認」而不是猜或留白：後端送來一個我們沒見過的單位
 * 時，畫面該說「我不確定這是什麼單位」，而不是把它當成公斤。 */
export function formatUnitLabel(unit: string | null | undefined): string {
	switch (unit) {
		case "kg":
			return "每公斤";
		case "catty":
			return "每台斤";
		case "100kg":
			return "每百公斤";
		default:
			return "單位待確認";
	}
}

/** 同一張表的「新臺幣／X」寫法，給表格 caption 與圖表說明用。 */
export function formatCurrencyUnitLabel(
	unit: string | null | undefined,
): string {
	switch (unit) {
		case "kg":
			return "新臺幣／公斤";
		case "catty":
			return "新臺幣／台斤";
		case "100kg":
			return "新臺幣／百公斤";
		default:
			return "單位待確認";
	}
}

/** 價格層級 → 中文標籤。
 *
 * 🔴 這不是可選的裝飾。毛豬回的是活體拍賣均價，不是豬肉分切零售價，兩者
 *    水平差數倍——顯示價格卻不說它是哪一個層級，就是在讓賣家誤讀。
 *    取捨是「用這個數字，但一定標清楚」。 */
export function formatPriceLayerLabel(
	layer: string | null | undefined,
): string | null {
	switch (layer) {
		case "wholesale_auction":
			return "批發拍賣";
		case "farmgate":
			return "產地價";
		case "live_animal":
			return "活體拍賣";
		case "retail":
			return "零售價";
		case "wholesale_bulk":
			return "躉售價";
		default:
			return null;
	}
}
