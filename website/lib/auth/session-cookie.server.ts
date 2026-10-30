import { cookies } from "next/headers";
import { SESSION_COOKIE_NAME } from "./session-cookie";

/**
 * 伺服器端（RSC、server action）取訪客的 session cookie，交給單體的 GET 讀取呼叫。
 *
 * D6（ADR 2026-10-30-identity-self-hosted-federated-login）：伺服器發出的請求沒有瀏覽器的
 * `Sec-Fetch-Site`／`Origin`，單體對 cookie 認證的非 GET 請求會拒絕，所以寫入一律由瀏覽器經同源反代送出，
 * 伺服器端只做讀取。呼叫端把這裡拿到的值明確交給 `apiClient` 的 `cookie` 選項或 `authedTransport` 的
 * `cookie` 選項，兩者都只在 GET／HEAD 送出它。本檔 import `next/headers`，只能由伺服器端的模組 import
 * （`"use server"` 檔、RSC）；共用的 `lib/api/client.ts`、`lib/connect/transport.ts` 在瀏覽器的圖裡，不能 import 它。
 */

/** 這個請求帶的 session cookie 的 `Cookie` header 值；沒有（訪客）或不在 request scope 內是 null。 */
export async function sessionCookieHeader(): Promise<string | null> {
	try {
		const value = (await cookies()).get(SESSION_COOKIE_NAME)?.value;
		return value ? `${SESSION_COOKIE_NAME}=${value}` : null;
	} catch (error) {
		// Next 用帶 `digest` 的錯誤把「這個頁面是動態的」告訴 renderer，必須原樣往上丟；
		// 其他錯誤是在 request scope 之外呼叫（腳本、單元測試），那裡沒有瀏覽器的 cookie。
		if (error instanceof Error && "digest" in error) throw error;
		return null;
	}
}
