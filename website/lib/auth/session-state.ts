import type { Identity } from "./storage";

/**
 * 這個請求的登入狀態，由伺服器的 DAL（`lib/auth/dal.ts`）決定，序列化後交給 `AuthProvider`。
 *
 * - `signedIn`：單體的 GetSession 回了帳號，連同頁首顯示用的暱稱與頭像。
 * - `guest`：沒有 session cookie，或單體確定說沒有登入（401）。
 * - `unavailable`：單體暫時答不出來（5xx、網路、逾時）。不是登出：不能據此清掉任何登入狀態（B-199）。
 */
export type SessionState =
	| { status: "signedIn"; identity: Identity }
	| { status: "guest" }
	| { status: "unavailable" };

/** `AuthProvider` 對外的狀態：伺服器的答案還沒到之前是 `unknown`，之後與 `SessionState` 同值。 */
export type AuthStatus = "unknown" | SessionState["status"];

/** `unavailable` 之後瀏覽器端重問 GetSession 的間隔（毫秒）；一直答不出來就停在 `unavailable`，不當成登出。 */
export const SESSION_RETRY_BACKOFF_MS = [1500, 3000, 6000] as const;
