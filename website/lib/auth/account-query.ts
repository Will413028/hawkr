import { skipToken } from "@tanstack/react-query";
import type { AuthStatus } from "./session-state";

/**
 * 公開頁的帳號查詢只在 `signedIn` 才送：其他狀態（`unknown`、`guest`、`unavailable`）都是 `skipToken`，
 * 查詢維持 `isPending`、不送請求。消費端以 `isPending` 判斷「還沒有資料」，並先看 `status` 決定訪客畫面；
 * 不以 `enabled:` 或自己的布林擋帳號查詢。`status` 取自 `useAccountStatus()`：受保護區內伺服器已確認
 * `signedIn`（`ZoneIdentityProvider`），所以區內查詢不被根層可能過期的答案擋住，真正被擋的只有公開頁。
 */
export function accountQueryFn<TFn extends (...args: never[]) => unknown>(
	status: AuthStatus,
	fn: TFn,
): TFn | typeof skipToken {
	return status === "signedIn" ? fn : skipToken;
}

/**
 * 公開頁帳號查詢的「載入中」：`unknown` 等伺服器的答案（中性等待，不是訪客）；`guest`、`unavailable` 沒有東西要載，
 * 由消費端依 `status` 畫訪客或暫時不可用；`signedIn` 才看查詢自己的 `isPending`。
 */
export function accountQueryPending(
	status: AuthStatus,
	queryIsPending: boolean,
): boolean {
	if (status === "unknown") return true;
	return status === "signedIn" && queryIsPending;
}
