import { afterEach, describe, expect, it, vi } from "vitest";
import { apiClient, authedFetch } from "./client";
import {
	DelegatedRuntimeTransportError,
	enterDelegatedRuntime,
	isDelegatedRuntime,
} from "./delegated-runtime";

/**
 * 代行執行期的傳輸層護欄（56 號 L1051／L1056 的上線阻斷項）。
 *
 * 這支測試守的是「漏切的資料層會**當場壞掉**，不會靜默用管理員自己的賣家身分
 * 打 `/app/*` 成功」。⚠️ 兩條路都要驗：`apiClient.authed.*` 與 `authedFetch` 走
 * 同一個 dispatch，但 15 個生產檔案用的是前者 —— 只驗後者等於留一扇開著的門。
 */

const release: (() => void)[] = [];

afterEach(() => {
	while (release.length > 0) release.pop()?.();
	vi.unstubAllGlobals();
});

/**
 * 旗標刻意只在瀏覽器生效（模組級變數在伺服器上是跨請求共用的，由 SSR 設起來會
 * 讓同一個 isolate 內其他人的賣家請求一起 throw）。所以這裡得先假裝有 window。
 */
function enter() {
	vi.stubGlobal("window", { location: { pathname: "/adm/1/products" } });
	release.push(enterDelegatedRuntime());
}

describe("delegated runtime transport guard", () => {
	it("throws for apiClient.authed and never reaches the network", async () => {
		const fetchSpy = vi.fn();
		vi.stubGlobal("fetch", fetchSpy);
		enter();
		expect(isDelegatedRuntime()).toBe(true);
		await expect(apiClient.authed.get("/sellers/me")).rejects.toBeInstanceOf(
			DelegatedRuntimeTransportError,
		);
		await expect(authedFetch("/products")).rejects.toBeInstanceOf(
			DelegatedRuntimeTransportError,
		);
		expect(fetchSpy).not.toHaveBeenCalled();
	});

	it("leaves public reads alone — the workspace still renders public content", async () => {
		const fetchSpy = vi.fn(
			async () =>
				new Response(JSON.stringify({ code: 0, data: null }), {
					status: 200,
					headers: { "content-type": "application/json" },
				}),
		);
		vi.stubGlobal("fetch", fetchSpy);
		enter();
		await apiClient.public.get("/products/1");
		expect(fetchSpy).toHaveBeenCalledTimes(1);
	});

	it("does not guard when the workspace never mounted", async () => {
		const fetchSpy = vi.fn(
			async () =>
				new Response(JSON.stringify({ code: 0, data: null }), {
					status: 200,
					headers: { "content-type": "application/json" },
				}),
		);
		vi.stubGlobal("fetch", fetchSpy);
		enterDelegatedRuntime()();
		expect(isDelegatedRuntime()).toBe(false);
		await apiClient.authed.get("/sellers/me");
		expect(fetchSpy).toHaveBeenCalledTimes(1);
	});
});
