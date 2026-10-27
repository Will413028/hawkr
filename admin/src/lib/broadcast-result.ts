/**
 * 系統公告送出後的實際結果（`POST /admin/notifications/broadcast` 的 `data`）。
 *
 * 「鍵不存在」與「值是 0」是兩件完全不同的事，而兩種常見寫法各自蓋掉一邊：
 *   - `Number(raw) || 0` 把「後端沒回報」壓成 0，變成一個假的空推播警報
 *   - `count || "未回報"` 把真正的 0 藏起來，而 0 正是「收件人枚舉整個壞掉」
 *     唯一看得出來的訊號
 * 所以三種狀態要分開表達，讀取端才不會把其中兩種混為一談。
 */
export type BroadcastOutcome =
	| { kind: "delivered"; count: number; broadcastId: string }
	| { kind: "empty"; broadcastId: string }
	| { kind: "unreported"; broadcastId: string };

/**
 * `recipientCount` 走 wire 是字串（10 檔 §4：數字對外一律字串）。這裡也收
 * 數字型別 —— 型別漂移時該顯示的仍是真實人數，不該退化成「未回報」而製造
 * 假警報；wire 型別本身由 tests/notification-template-envelope-contract.test.ts
 * 釘住，不靠這支容忍度來守。
 */
function parseCount(raw: unknown): number | null {
	if (typeof raw === "string") {
		return /^\d+$/.test(raw) ? Number(raw) : null;
	}
	if (typeof raw === "number") {
		return Number.isInteger(raw) && raw >= 0 ? raw : null;
	}
	return null;
}

export function readBroadcastOutcome(data: unknown): BroadcastOutcome {
	const record = (data ?? {}) as Record<string, unknown>;
	const broadcastId =
		typeof record.broadcastId === "string" ? record.broadcastId : "";
	const count = parseCount(record.recipientCount);

	if (count === null) return { kind: "unreported", broadcastId };
	if (count === 0) return { kind: "empty", broadcastId };
	return { kind: "delivered", count, broadcastId };
}
