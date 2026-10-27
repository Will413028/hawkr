"use client";

import { ApiClientError } from "./api-error";
import { AuthService } from "./auth";
import { environmentService } from "./environment";

interface RequestOptions extends RequestInit {
	headers?: HeadersInit;
	/**
	 * 這一支請求的逾時（毫秒）。**預設仍是 10 秒**——只有明確傳值的呼叫點才會拿到
	 * 別的值，既有呼叫點的行為完全不變。
	 *
	 * 為什麼要有它（58A 陷阱 T10）：發票的端點 44（ADM 重新測試連線）與端點 66
	 * （恢復連線）在請求路徑上**同步等 AMEGO**，gateway 那一側的 deadline 以分鐘計。
	 * 寫死 10 秒的話它們每次都會在 10 秒被前端 abort，**而後端仍在跑、結果照樣落庫** ⇒
	 * 症狀是「按了說逾時，重新整理卻看到結果已經變了」，比單純失敗更難查。
	 */
	timeoutMs?: number;
}

/** 沒有指定 `timeoutMs` 時的逾時。除了同步等第三方的那幾支，一律用這個值。 */
const DEFAULT_TIMEOUT_MS = 10000;

export async function request<T>(
	endpoint: string,
	options: RequestOptions = {},
): Promise<T | null> {
	const headers: HeadersInit = {
		"Content-Type": "application/json",
		...(options.headers || {}),
	};

	// `timeoutMs` 是本層自己的參數，不屬於 `RequestInit` ⇒ 解構掉，不要往 fetch 送。
	const { timeoutMs = DEFAULT_TIMEOUT_MS, ...init } = options;
	const config: RequestInit = {
		...init,
		headers,
	};

	const controller = new AbortController();
	const timeoutId = setTimeout(() => controller.abort(), timeoutMs);
	config.signal = controller.signal;

	try {
		const baseUrl = environmentService.getGatewayUrl();
		const response = await fetch(`${baseUrl}${endpoint}`, config);
		clearTimeout(timeoutId);

		// 後台呼叫的 REST（/admin、/app/media）只在沒有身分時回 401（非管理員是 403）：session 已經失效，
		// 與 Connect 的 unauthenticated 一樣登出並回到 Access。
		if (response.status === 401) {
			console.warn("API returned 401 for:", endpoint);
			AuthService.sessionExpired();
			throw new Error("需要重新登入或權限不足");
		}

		if (response.status === 404) {
			console.warn("API returned 404 for:", endpoint);
			// 404 可能是路由不存在，也可能是後端業務上的「資源找不到」（例如帳號
			// 已被刪除）— 兩者都回真實 message，不要一律顯示「API 端點不存在」
			// 蓋掉實際原因。
			const data = await response.json().catch(() => null);
			throw new ApiClientError(
				data?.message || "API 端點不存在",
				data?.code,
				data?.traceId,
				data?.data,
			);
		}

		const data = await response.json();

		if (data && typeof data.code === "number") {
			if (data.code === 401) {
				console.warn("API returned code 401 for:", endpoint);
				throw new Error("需要重新登入或權限不足");
			}
			if (data.code !== 0) {
				// data 一定要帶上：有一批業務碼的 HTTP status 恆為 200 而 code !== 0
				// （58 §6.1.0 (13) 的 preserveData 家族），它們要傳給前端的東西全在
				// envelope 的 data 裡。這裡丟掉的話，呼叫端 catch 到的例外沒有任何
				// 可用資訊，畫面上的症狀是「按了沒反應」而不是一則錯誤訊息。
				throw new ApiClientError(
					data.message || `API Error Code: ${data.code}`,
					data.code,
					data.traceId,
					data.data,
				);
			}
		}

		if (!response.ok) {
			throw new ApiClientError(
				data.message || "API Request Failed",
				data.code,
				data.traceId,
				data.data,
			);
		}

		return data as T;
	} catch (error: unknown) {
		clearTimeout(timeoutId);
		if (error instanceof Error && error.name === "AbortError") {
			console.error("API Request Timed Out");
			throw new Error("請求超時，請檢查網路連線");
		}
		console.error("API Error:", error);
		throw error;
	}
}

export const api = {
	get: <T>(url: string) => request<T>(url, { method: "GET" }),
	post: <T>(url: string, body: unknown) =>
		request<T>(url, { method: "POST", body: JSON.stringify(body) }),
	put: <T>(url: string, body: unknown) =>
		request<T>(url, { method: "PUT", body: JSON.stringify(body) }),
	patch: <T>(url: string, body: unknown) =>
		request<T>(url, { method: "PATCH", body: JSON.stringify(body) }),
	delete: <T>(url: string) => request<T>(url, { method: "DELETE" }),
};
