import type { Identity } from "./storage";

/**
 * 這個請求的登入狀態，由伺服器的 DAL（`lib/auth/dal.ts`）決定，序列化後交給 `AuthProvider`。
 *
 * - `signedIn`：單體的 GetSession 回了帳號。
 * - `guest`：沒有 session cookie，或單體確定說沒有登入（401）。`refusal` 是單體拒絕這個帳號（刪除或停權，
 *   403）：伺服器端唯讀（D6），結束 session 與顯示拒絕原因由瀏覽器端的 `AuthProvider` 接手。
 * - `unavailable`：單體暫時答不出來（5xx、網路、逾時）。不是登出：不能據此清掉任何登入狀態（B-199）。
 */
export type SessionState =
	| { status: "signedIn"; identity: Identity }
	| { status: "guest"; refusal?: SessionRefusal }
	| { status: "unavailable" };

export interface SessionRefusal {
	/** 單體附的使用者面訊息；沒有時由 client 用該 code 的預設文字。 */
	message: string | null;
}

/** `AuthProvider` 對外的狀態：伺服器的答案還沒到之前是 `unknown`，之後與 `SessionState` 同值。 */
export type AuthStatus = "unknown" | SessionState["status"];
