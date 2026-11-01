import "server-only";

import { cache } from "react";
import { fetchSession, SESSION_UNAVAILABLE } from "@/lib/api/auth";
import { defaultLocale } from "@/lib/intl/locales";
import { sessionCookieHeader } from "./session-cookie.server";
import type { SessionState } from "./session-state";

/** GetSession 在伺服器端的總時限（含 body）；逾時是 `unavailable`，不是登出。 */
export const SESSION_READ_TIMEOUT_MS = 5_000;

/**
 * Data Access Layer：這個請求的登入狀態的唯一伺服器端來源。
 *
 * 讀 `__Host-hk_session` cookie，只呼叫單體的 GetSession（GET，唯讀，D6），回 `signedIn | guest | unavailable`。
 * 只有確定的答案才是 `guest`：沒有 cookie（沒有憑證，不必問單體）、401。
 * 其他一切（403、5xx、網路、逾時、讀 body 失敗、空帳號）都是 `unavailable`：暫時不可用不等於登出（B-199）。
 *
 * `cache()` 讓同一個請求的多個呼叫共用一次讀取。不得在 `'use cache'` 內呼叫：讀 cookie 是 request 資料，
 * 結果也因人而異。
 */
export const verifySession = cache(async (): Promise<SessionState> => {
	let cookie: string | null;
	try {
		cookie = await sessionCookieHeader();
	} catch (error) {
		// Next 用帶 `digest` 的錯誤告訴 renderer 這裡是動態的，必須原樣往上丟。
		if (error instanceof Error && "digest" in error) throw error;
		return { status: "unavailable" };
	}
	if (!cookie) return { status: "guest" };
	return classifySession(() =>
		fetchSession({
			cookie,
			locale: defaultLocale,
			signal: AbortSignal.timeout(SESSION_READ_TIMEOUT_MS),
		}),
	);
});

/** GetSession 的結果對應到 `SessionState`；讀取的任何例外（含逾時）都是 `unavailable`。 */
export async function classifySession(
	read: () => Promise<Awaited<ReturnType<typeof fetchSession>>>,
): Promise<SessionState> {
	try {
		const result = await read();
		if (result === SESSION_UNAVAILABLE) return { status: "unavailable" };
		if (result === null) return { status: "guest" };
		return { status: "signedIn", identity: result };
	} catch {
		return { status: "unavailable" };
	}
}
