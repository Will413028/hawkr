/**
 * 代行商品頁四張 KPI 卡的取值（56 號 B15）。
 *
 * 一張卡＝一次帶 `status` 的列表，只讀它的 `total`。這裡唯一的職責是把「那四個
 * total 還不知道」與「已經知道了」分開，因為**這一頁曾把「不知道」渲染成 `0`**：
 *
 * 2026-08-16 dev 驗收（session `OPS-1KXUSQJXJ7C3K`）實測，代行商品頁畫出
 * 「總商品數 15 / 啟用 0 / 未啟用 0 / 草稿 0」，而同一店同一時刻 DB 為
 * 4004=1／4003=10／4001=4，清單就在同一個畫面上列出那 15 筆。成因是 gateway 對
 * product-service 的斷路器當時處於 half-open：四支 `page_size=1` 探詢全部拿到
 * 9280／502（gateway log latency ~0.02s，product-service 完全沒收到請求），
 * `Promise.all` 整組 reject ⇒ 舊實作的 `?? 0` 把四個失敗一起講成「這家店一件上架
 * 商品都沒有」。**那是一句話，不是一個空值。**
 *
 * ⇒ 未知一律回 `null`，由呼叫端渲染破折號。這與同專案兩處既有裁決同源：
 * `DelegatedDashboardPage.statValue()`（「`0` 是一個宣稱」）與賣家訂單頁的
 * `formatNullableCount()`（「dashboard 尚未載入時為 null，避免閃出誤導的 0」）。
 *
 * ⚠️ `total` 刻意**不以清單的 `total` 兜底**。清單那個數字是「當前篩選」的總數
 * （`reloadProducts` 帶 `status: statusFilter`），拿它當「總商品數」在有篩選時
 * 直接是錯的；而它在無篩選時剛好等於正解，於是這個 bug 只在點過卡片之後才現形。
 * 更關鍵的是四格混用兩個來源會畫出「總數 15、其餘 0」這種自相矛盾的一列 ——
 * 四格同生共死，要嘛都是真值、要嘛都是破折號。
 */

/** 四支探詢回來的 total。任一支失敗 ⇒ 整組視為未知（呼叫端傳 `null`）。 */
export interface DelegatedProductTotals {
	all: number;
	active: number;
	off: number;
	draft: number;
}

/** 卡片要顯示的四個值。`null` ＝ 尚未知道（載入中或該次查詢失敗）。 */
export interface DelegatedProductKpi {
	total: number | null;
	active: number | null;
	off: number | null;
	/** 代行不渲染庫存預警卡（缺安全庫存門檻來源），恆為 `null`。 */
	lowStock: number | null;
	draft: number | null;
}

/**
 * 把「四個 total 或 null」映成四張卡的值。
 *
 * @param totals 四支探詢全部成功時的 total；未知（載入中／失敗）傳 `null`。
 */
export function delegatedProductKpi(
	totals: DelegatedProductTotals | null,
): DelegatedProductKpi {
	return {
		total: totals?.all ?? null,
		active: totals?.active ?? null,
		off: totals?.off ?? null,
		lowStock: null,
		draft: totals?.draft ?? null,
	};
}
