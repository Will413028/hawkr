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

		if (response.status === 401) {
			console.warn("API returned 401 for:", endpoint);
			AuthService.sessionExpired();
			throw new Error("需要重新登入或權限不足");
		}

		if (response.status === 404) {
			console.warn("API returned 404 for:", endpoint);
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
