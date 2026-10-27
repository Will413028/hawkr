/**
 * 代行訂單列表的查詢條件（56 號 §6.1.10 的 Query 契約：`status`／`keyword`／
 * `date_from`／`date_to`）。
 *
 * 單獨一個模組是為了讓「清單、KPI 卡、零筆文案」三處讀同一份定義而**不必**各自
 * 依賴傳輸層 —— 展開成 wire 參數是 `repo.ts` 的事，這裡只有形狀與判準。
 */

/**
 * 除了 `status` 之外的查詢條件。
 *
 * ⚠️ **KPI 卡與清單必須吃同一份。** 五張卡各自是一次 `page_size=1` 的帶狀態列表、
 * 只讀 `total`；少餵任一個條件，卡片就會停在「全部 81 筆」而清單只剩搜尋命中的 3 筆
 * —— 兩個數字並排在同一個畫面上互相矛盾，操作者無從判斷哪一個才是真的。抽成具名
 * 型別是為了讓那件事變成**編譯期**的事：日後多一個條件而漏接 KPI，
 * `useDelegatedOrderStatusTotals` 的呼叫點會少一個必填欄位，`tsc` 當場擋下來。
 */
export interface DelegatedOrderSearchFilters {
	/**
	 * 模糊搜尋。比對範圍恰為訂單編號、收件人姓名、品項名稱
	 * （`api/proto/order/order.proto` 的 `GetSellerOrdersRequest.keyword`）。
	 *
	 * ⚠️ **不含收件人電話與完整地址**，那不是漏做：兩者都是列表投影看不到的欄位，拿
	 * 它們當條件等於造出一支 oracle —— 輸入一個值看有沒有命中，就能在完全不揭露該值
	 * 的前提下確認它存在，而且不落 `ORDER_PII_REVEAL` 稽核（56 號 §8.1.5）。搜尋框的
	 * placeholder 因此也只能寫得出這三者。
	 */
	keyword: string;
	/** 台北曆日 `YYYY-MM-DD`（`<input type="date">` 的原值）；空字串＝不篩選。 */
	dateFrom: string;
	/** 同上。送出前才由 `taipeiDayEndUtc` 展開成該日終點。 */
	dateTo: string;
}

/** 列表與 KPI 共用的完整條件集。 */
export interface DelegatedOrderFilters extends DelegatedOrderSearchFilters {
	/** CodeMeta `OrderStatus` 數字碼；空字串＝全部。 */
	status: string;
}

export const EMPTY_DELEGATED_ORDER_FILTERS: DelegatedOrderFilters = {
	status: "",
	keyword: "",
	dateFrom: "",
	dateTo: "",
};

/**
 * 搜尋草稿 → 要寫進網址的值（`null` ＝ 把這個參數從網址移除）。
 *
 * ⚠️ **必須 trim。** `"   "` 是 truthy，直接寫進去會得到 `?keyword=%20%20%20`。
 * 傳輸層那一側早就 trim 過（`repo.ts` 的 `if (trimmed) query.keyword = trimmed`），
 * 所以後端收到的條件與「沒搜尋」完全相同 —— 但 react-query 的鍵吃的是**未 trim 的**
 * 字串，於是那幾個空白換出一組全新的鍵，白打 6 支（清單 ＋ 五張 KPI）內容一模一樣的
 * 請求，順帶把 `page` 重置、留下一個髒的可分享網址。
 *
 * 判準與 `hasDelegatedOrderFilter` 的 `keyword.trim()` 是同一個：**trim 後為空＝沒搜**。
 */
export function keywordUrlValue(draft: string): string | null {
	return draft.trim() || null;
}

/** 任一條件成立就代表「使用者篩過」——零筆時的文案據此分流。 */
export function hasDelegatedOrderFilter(
	filters: DelegatedOrderFilters,
): boolean {
	return Boolean(
		filters.status ||
			filters.keyword.trim() ||
			filters.dateFrom ||
			filters.dateTo,
	);
}
