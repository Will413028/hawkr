/**
 * 「結束結果不明」封鎖旗標的持久化（56 號 §3.1.3 F8／§3.1.4）。
 *
 * 結束代行的回應不在封閉列舉內時（逾時、502、`ended` 沒被證明），工作區必須停掉
 * 所有寫入，設計書對這個狀態的要求是「寫入被**永久** latch 封鎖」。而伺服器在
 * 結束未獲證明時刻意**不清 credential、不終止工作階段** —— 於是封鎖若只活在
 * provider 的一個 React state 裡，操作者遇到結束失敗最自然的動作（重新整理、貼
 * 網址重開）就會把它解除，工作階段仍然活著、寫入全部恢復。整個 fail-closed 保證
 * 活不過一次 document load。
 *
 * 存在 `sessionStorage` 而不是後端欄位：`end_unknown` 是前端由 DTO 推導出來的
 * 畫面狀態，後端沒有、也不得回傳它（§7.5.1），封鎖狀態的權威在前端（§3.0 Q3）。
 *
 * **存的是工作階段 id 而不是布林**，解除條件因此是機械的、不需要任何額外的清除
 * 時機：
 * - 換一個工作階段（新的 sessionId）→ 值對不上 → 不封鎖。這是操作者的出路：返回
 *   ADM、撤銷這個工作階段、再建一個新的。
 * - 結束已被證明（成功回應）→ 明確清掉，不留死值。
 * - 分頁關閉 → `sessionStorage` 自然消失；工作階段最長 30 分鐘，殘值不會長命。
 *
 * ⚠️ 存布林會做出「永遠清不掉」的封鎖：下一個工作階段一進來就是阻擋畫面，而畫面
 * 上完全看不出成因（它說的是上一個工作階段的結束結果）。
 *
 * ⚠️ 已知殘留：`sessionStorage` 是 per-tab，而 credential cookie 是跨分頁共享的
 * ⇒ 操作者若刻意另開一個分頁貼上工作區網址，那個分頁沒有封鎖。要連這條一起關
 * 得改由伺服器種 marker cookie，但結束請求連不上 BFF（最常見的「結果不明」成因）
 * 時伺服器根本沒有機會種它 —— 兩者守的不是同一段，這裡先守活得下來的那一段。
 */

export const DELEGATED_END_PENDING_STORAGE_KEY = "hawkr:adm:end-pending";

type StorageLike = Pick<Storage, "getItem" | "setItem" | "removeItem">;

/** SSR 沒有 sessionStorage；Safari 私密模式與封鎖第三方儲存時存取會直接 throw。 */
function browserSessionStore(): StorageLike | null {
	if (typeof window === "undefined") return null;
	try {
		return window.sessionStorage;
	} catch {
		return null;
	}
}

export function readEndPendingSessionId(
	store: StorageLike | null = browserSessionStore(),
): string | null {
	if (!store) return null;
	try {
		const raw = store.getItem(DELEGATED_END_PENDING_STORAGE_KEY);
		return raw !== null && raw.length > 0 ? raw : null;
	} catch {
		return null;
	}
}

export function writeEndPendingSessionId(
	sessionId: string,
	store: StorageLike | null = browserSessionStore(),
): void {
	if (!store || sessionId.length === 0) return;
	try {
		store.setItem(DELEGATED_END_PENDING_STORAGE_KEY, sessionId);
	} catch {
		// 寫不進去只代表這一輪的封鎖無法跨 document 存活，記憶體那一半仍然成立；
		// 為了寫 log 而讓工作區炸開沒有意義。
	}
}

export function clearEndPendingSessionId(
	store: StorageLike | null = browserSessionStore(),
): void {
	if (!store) return;
	try {
		store.removeItem(DELEGATED_END_PENDING_STORAGE_KEY);
	} catch {
		// 同上。
	}
}

/**
 * 此刻要不要封鎖寫入。
 *
 * `currentSessionId` 為 null（還沒拿到 summary，或工作階段已經查不到）時一律不
 * 封鎖 —— 那些情況下 `resolveDelegatedSessionState` 本來就會把畫面切成阻擋態或
 * 「正在確認…」，不需要、也不該由這裡再蓋一層看不懂的理由。
 */
export function isEndLatched(input: {
	/** 本次 document 內按過結束且尚未被證明成功。 */
	blockedInMemory: boolean;
	/** 從 `sessionStorage` 讀回來的工作階段 id。 */
	pendingSessionId: string | null;
	/** 當前 summary 的工作階段 id。 */
	currentSessionId: string | null;
}): boolean {
	if (input.blockedInMemory) return true;
	return (
		input.pendingSessionId !== null &&
		input.pendingSessionId === input.currentSessionId
	);
}
