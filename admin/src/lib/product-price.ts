/**
 * 商品價格區間顯示 — ADM 唯一入口。
 *
 * 後端契約：價格欄位的 `"0"` 就是「無價格」的表達（入站 `parsePrice("0") → null`、
 * 出站 `formatAmount(null) → "0"`，雙向一致）。所以 `"0"` 必須當成缺席處理，
 * 不能印成 `NT$ 0` —— 那是在告訴管理員「這件商品標價零元」。
 *
 * ⚠️ falsy 判斷（`if (!min && !max)`）擋不住這件事：`"0"` 是非空字串，truthy。
 * 判準只能是「parse 完之後是不是正數」。同一份語意在網頁是
 * `hawkr-website/lib/products/pricing.ts` 的 `num()`。
 */

/** 解析成正數金額；`"0"`／負數／非數字／缺鍵一律回 null（＝無價格）。 */
function positiveAmount(raw?: string | number | null): number | null {
	if (raw == null) return null;
	const n = typeof raw === "number" ? raw : Number(raw);
	if (!Number.isFinite(n) || n <= 0) return null;
	return n;
}

/**
 * `NT$ 100`／`NT$ 100 - NT$ 250`／兩端皆無價格時回 `--`。
 *
 * 幣別前綴與 products/[id]、anomalies、disputes 一致用 `NT$ `：裸 `$` 讀者無從判斷
 * 是新台幣還是美元，而這一欄是商家的定價。
 */
export function formatProductPriceRange(
	min?: string | number | null,
	max?: string | number | null,
): string {
	const minN = positiveAmount(min);
	const maxN = positiveAmount(max);
	if (minN === null && maxN === null) return "--";
	if (minN === null || maxN === null) {
		return `NT$ ${(minN ?? maxN)?.toLocaleString()}`;
	}
	if (minN === maxN) return `NT$ ${minN.toLocaleString()}`;
	return `NT$ ${minN.toLocaleString()} - NT$ ${maxN.toLocaleString()}`;
}
