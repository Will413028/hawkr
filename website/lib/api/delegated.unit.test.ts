import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClientError } from "./client";
import { delegatedFetch } from "./delegated";

/**
 * 代行工作區的傳輸契約（56 號 §4.5 L1052）。
 *
 * 這支測試守的是四件「壞掉的時候不會有人發現」的事：URL 沒有被補上 `/app`、非 GET
 * 帶得到 BFF 的 CSRF header、業務碼失敗會 throw（而不是把 `{code:9270}` 當資料回
 * 去畫面）、以及被 middleware 導向時錯誤訊息指得到原因。
 */

function jsonResponse(body: unknown, init: { status?: number } = {}): Response {
	return new Response(JSON.stringify(body), {
		status: init.status ?? 200,
		headers: { "content-type": "application/json" },
	});
}

function stubBrowser(response: Response | (() => Response)) {
	const fetchSpy = vi.fn(async () =>
		typeof response === "function" ? response() : response,
	);
	vi.stubGlobal("window", { location: { pathname: "/adm/1/products" } });
	vi.stubGlobal("fetch", fetchSpy);
	return fetchSpy;
}

function lastCall(fetchSpy: ReturnType<typeof vi.fn>) {
	const [url, init] = fetchSpy.mock.calls.at(-1) as [string, RequestInit];
	return { url, init, headers: new Headers(init.headers) };
}

afterEach(() => {
	vi.unstubAllGlobals();
});

describe("delegatedFetch", () => {
	it("打同源 /adm/api 且不補 /app（不得重用 client.ts 的 buildUrl）", async () => {
		const fetchSpy = stubBrowser(
			jsonResponse({ code: 0, data: { total: "3" } }),
		);
		const env = await delegatedFetch<{ total: string }>("/products", {
			query: { page: 1, page_size: 20, keyword: undefined },
		});

		const { url, init } = lastCall(fetchSpy);
		expect(url).toBe("/adm/api/products?page=1&page_size=20");
		expect(url).not.toContain("/app/");
		expect(init.credentials).toBe("same-origin");
		expect(init.cache).toBe("no-store");
		// 3xx 一律不跟隨：跟隨會拿到一個 HTTP 200 的登入 HTML。
		expect(init.redirect).toBe("manual");
		expect(env.data.total).toBe("3");
	});

	it("非 GET 帶 BFF 的 CSRF header 與 Idempotency-Key", async () => {
		const fetchSpy = stubBrowser(jsonResponse({ code: 0, data: {} }));
		await delegatedFetch("/products/9/publish", {
			method: "POST",
			json: { expectedVersion: "12", operationReason: "協助上架" },
			idempotencyKey: "adm-9-publish-1", // gitleaks:allow（測試值）
		});

		const { url, init, headers } = lastCall(fetchSpy);
		expect(url).toBe("/adm/api/products/9/publish");
		expect(headers.get("X-Requested-With")).toBe("XMLHttpRequest");
		expect(headers.get("Idempotency-Key")).toBe("adm-9-publish-1");
		expect(headers.get("Content-Type")).toBe("application/json");
		expect(JSON.parse(String(init.body))).toEqual({
			expectedVersion: "12",
			operationReason: "協助上架",
		});
	});

	it("GET 不帶 X-Requested-With，也不帶 Idempotency-Key", async () => {
		const fetchSpy = stubBrowser(jsonResponse({ code: 0, data: null }));
		await delegatedFetch("/orders");
		const { headers } = lastCall(fetchSpy);
		expect(headers.get("X-Requested-With")).toBeNull();
		expect(headers.get("Idempotency-Key")).toBeNull();
	});

	it("業務碼非 0 一律 throw，且 code 讀得到（終態 9270/9271/9272 靠它判定）", async () => {
		stubBrowser(jsonResponse({ code: 9270, traceId: "t-1" }, { status: 401 }));
		const error = await delegatedFetch("/products").catch((e) => e);
		expect(error).toBeInstanceOf(ApiClientError);
		expect((error as ApiClientError).code).toBe(9270);
		expect((error as ApiClientError).status).toBe(401);
	});

	it("throwOnHttpError:false 時把 envelope 交回呼叫端（repo 的 404 分支靠它）", async () => {
		stubBrowser(jsonResponse({ code: 0, data: null }, { status: 404 }));
		const env = await delegatedFetch("/products/404", {
			throwOnHttpError: false,
		});
		expect(env.data).toBeNull();
	});

	it("被導向時錯誤訊息指得到 middleware，而不是 JSON parse 失敗", async () => {
		stubBrowser(() => new Response(null, { status: 307 }));
		const error = await delegatedFetch("/session/current").catch((e) => e);
		expect(error).toBeInstanceOf(ApiClientError);
		expect((error as Error).message).toContain("middleware");
	});

	it("伺服器側直接擋下：憑證是 Path=/adm 的 cookie，SSR 沒有它", async () => {
		const fetchSpy = vi.fn();
		vi.stubGlobal("fetch", fetchSpy);
		await expect(delegatedFetch("/products")).rejects.toThrow(/瀏覽器/);
		expect(fetchSpy).not.toHaveBeenCalled();
	});
});
