/**
 * 代行工作區 → 平台異常處置的交接參數（產品需求 §6.7）。
 *
 * 代行工作區的訂單抽屜底部有一顆「前往平台異常處置」，它是 §6.7 明訂會「離開代行
 * 工作區並重新驗治理權限」的那條路：`hawkr-website` 只組一條跨 host 連結，
 * 治理權限在**這裡**（`/admin` 掛 admin session）重驗。56 號 §2.2 Out-of-scope #5
 * 把「異常訂單處置」歸給 29 號既有 admin 入口，所以落點就是異常監控頁，**不新增
 * 任何端點**。
 *
 * ⚠️ 鍵名是跨 repo 契約。上游組連結的地方是
 * `hawkr-website/features/delegated-operations/state/session.ts` 的
 * `delegatedAnomalyUrl`，兩邊各有一支測試釘住這四個鍵；改名要同波，否則連結照樣
 * 200、頁面照樣渲染，只是帶過來的訂單完全沒有落點，**畫面上看不出來**。
 */

/** 異常監控頁的五個 tab。與 `page.tsx` 的 `TabKey` 同一組值。 */
export const ANOMALY_TAB_KEYS = [
	"shipping",
	"orders",
	"payments",
	"refunds",
	"grants",
] as const;

export type AnomalyTabKey = (typeof ANOMALY_TAB_KEYS)[number];

export interface AnomalyHandoff {
	/** 未帶或不是合法 tab 時為 `null`，由頁面沿用自己的預設，不得猜。 */
	tab: AnomalyTabKey | null;
	/** 目標店家（`sellers.store_id` surrogate）。收斂範圍用。 */
	storeId: string | null;
	/** 帶過來的那一筆訂單編號。用於當頁標示與過濾。 */
	orderNo: string | null;
	/** 訂單當下的 OrderStatus value。異常訂單查詢未帶 status 時後端預設只回 35。 */
	status: string | null;
}

export const EMPTY_ANOMALY_HANDOFF: AnomalyHandoff = {
	tab: null,
	storeId: null,
	orderNo: null,
	status: null,
};

/**
 * `?tab=&store_id=&order_no=&status=` → 交接參數。
 *
 * 全部欄位一律 fail-soft 成 `null`：這是一條「帶著上下文換頁」的連結，不是授權
 * 判定。參數壞掉的正確行為是照常開一個乾淨的異常監控頁，不是報錯把人擋在外面
 * ——真正的授權在 `/admin` 的 admin session，那一關與這些參數無關。
 *
 * `status` 只收純數字：它會被原樣送進 `GET /api/v1/admin/orders/abnormal` 的
 * `status`（CodeMeta OrderStatus value，逗號分隔的多值在本交接情境用不到）。
 */
export function parseAnomalyHandoff(
	params: URLSearchParams | null | undefined,
): AnomalyHandoff {
	if (!params) return EMPTY_ANOMALY_HANDOFF;
	const tab = params.get("tab");
	const status = clean(params.get("status"));
	return {
		tab: (ANOMALY_TAB_KEYS as readonly string[]).includes(tab ?? "")
			? (tab as AnomalyTabKey)
			: null,
		storeId: clean(params.get("store_id")),
		orderNo: clean(params.get("order_no")),
		status: status !== null && /^\d+$/.test(status) ? status : null,
	};
}

function clean(value: string | null): string | null {
	const trimmed = (value ?? "").trim();
	return trimmed === "" ? null : trimmed;
}
