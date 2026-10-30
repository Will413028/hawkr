import { afterEach, describe, expect, it, vi } from "vitest";
import type { ApiEnvelope } from "@/lib/api/client";
import type { RepoFetcher } from "@/lib/api/delegated";
import {
	DelegatedRuntimeTransportError,
	enterDelegatedRuntime,
} from "@/lib/api/delegated-runtime";
import { createMediaUploadRepo, uploadImageFile } from "@/lib/media/upload";
import { createOrderRepo, fetchOrders } from "@/lib/orders/repo";
import {
	createSellerProductRepo,
	fetchSellerProducts,
} from "@/lib/products/seller-repo";
import {
	createSellerPostRepo,
	fetchMyStore,
	fetchSellerPosts,
} from "@/lib/seller/repo";

/**
 * 賣家資料層的注入接縫（56 號 §4.5 L1053／L1056 上線阻斷項）。
 *
 * **雙向都要驗，只驗一邊等於沒驗**：
 * ① 代行執行期呼叫**未切換**的賣家資料層 ⇒ 必須當場 throw、不得發出請求
 *    （否則管理員本人剛好也是賣家時，`/app/*` 會用他自己的身分**成功**，
 *     操作到他自己的店、零稽核零告警）
 * ② 賣家自己的 `/seller-center` ⇒ 同一批函式必須照常送出請求、一個都不能 throw
 * ③ 工廠注入的 fetcher ⇒ 在代行執行期照常運作（護欄擋的是傳輸層，不是資料層）
 */

const release: (() => void)[] = [];

function stubBrowserFetch(envelope: unknown = { code: 0, data: null }) {
	const urls: string[] = [];
	const fetchSpy = vi.fn(async (url: string) => {
		urls.push(String(url));
		return new Response(JSON.stringify(envelope), {
			status: 200,
			headers: { "content-type": "application/json" },
		});
	});
	vi.stubGlobal("window", {
		location: { pathname: "/seller-center" },
	});
	vi.stubGlobal("fetch", fetchSpy);
	return { fetchSpy, urls };
}

function enterDelegated() {
	release.push(enterDelegatedRuntime());
}

afterEach(() => {
	while (release.length > 0) release.pop()?.();
	vi.unstubAllGlobals();
});

/** 賣家路徑的具名匯出（元件今天用的就是這些）。 */
const sellerEntryPoints: ReadonlyArray<[string, () => Promise<unknown>]> = [
	["fetchSellerProducts", () => fetchSellerProducts({ page: 1, pageSize: 20 })],
	["fetchMyStore", () => fetchMyStore()],
	["fetchSellerPosts", () => fetchSellerPosts(undefined, { pageSize: 50 })],
	["fetchOrders", () => fetchOrders({ view: "seller" })],
	[
		"uploadImageFile",
		() =>
			uploadImageFile(
				new File([new Uint8Array([1])], "a.png", { type: "image/png" }),
				undefined,
				{ compress: false },
			),
	],
];

describe("① 代行執行期：未切換的賣家資料層必須壞掉", () => {
	for (const [name, call] of sellerEntryPoints) {
		it(`${name} throws and never reaches the network`, async () => {
			const { fetchSpy } = stubBrowserFetch();
			enterDelegated();
			await expect(call()).rejects.toBeInstanceOf(
				DelegatedRuntimeTransportError,
			);
			expect(fetchSpy).not.toHaveBeenCalled();
		});
	}
});

describe("② 賣家路徑：同一批函式一個都不能 throw", () => {
	it("讀取類全部送得出請求，且打的是 /app/*", async () => {
		const { fetchSpy, urls } = stubBrowserFetch({
			code: 0,
			data: { records: [] },
		});
		// fetchMyStore 走 Connect（`/api/connect/...`），不在這一組的 REST 斷言裡。
		const restEntryPoints = sellerEntryPoints.filter(
			([name]) => name !== "fetchMyStore",
		);
		for (const [, call] of restEntryPoints.slice(0, 3)) {
			await expect(call()).resolves.toBeDefined();
		}
		expect(fetchSpy).toHaveBeenCalledTimes(3);
		expect(urls.every((url) => url.includes("/app/"))).toBe(true);
		expect(urls.some((url) => url.includes("/adm/"))).toBe(false);
	});

	it("媒體上傳走到建立工作階段那一步（回應缺欄才失敗，不是被護欄擋下）", async () => {
		const { urls } = stubBrowserFetch({ code: 0, data: {} });
		const error = await uploadImageFile(
			new File([new Uint8Array([1])], "a.png", { type: "image/png" }),
			undefined,
			{ compress: false },
		).catch((e) => e);
		expect(error).not.toBeInstanceOf(DelegatedRuntimeTransportError);
		expect(String((error as Error).message)).toContain("建立上傳工作階段失敗");
		expect(urls[0]).toContain("/app/media/upload_sessions");
	});
});

describe("③ 注入的 fetcher：代行執行期照常運作", () => {
	function recordingFetcher() {
		const seen: Array<{ path: string; method: string }> = [];
		const fetcher = (async <T>(
			path: string,
			opts: { method?: string } = {},
		) => {
			seen.push({ path, method: opts.method ?? "GET" });
			return { code: 0, data: {} as T } satisfies ApiEnvelope<T>;
		}) as RepoFetcher;
		return { seen, fetcher };
	}

	it("四支資源工廠都吃得到注入的 root，且不經過賣家傳輸層", async () => {
		const { fetchSpy } = stubBrowserFetch();
		enterDelegated();
		const { seen, fetcher } = recordingFetcher();

		await createSellerProductRepo(fetcher, {
			root: "/products",
		}).fetchSellerProducts({ page: 2, pageSize: 10 });
		await createSellerPostRepo(fetcher, {
			root: "/posts",
			listRoot: "/posts",
		}).fetchSellerPosts();
		await createOrderRepo(fetcher, { root: "/orders" }).fetchOrders({});
		await createMediaUploadRepo(fetcher, { root: "/media/upload_sessions" })
			.uploadImageFile(
				new File([new Uint8Array([1])], "a.png", { type: "image/png" }),
				undefined,
				{ compress: false },
			)
			.catch(() => undefined); // 缺 uploadUrl，只驗它打到了注入的路徑

		expect(seen.map((call) => call.path)).toEqual([
			"/products?page=2&page_size=10",
			"/posts",
			"/orders?page=1&page_size=20",
			"/media/upload_sessions",
		]);
		expect(fetchSpy).not.toHaveBeenCalled();
	});

	it("工廠不帶 root 時等於賣家路徑（預設值就是既有端點）", async () => {
		const { seen, fetcher } = recordingFetcher();
		await createSellerProductRepo(fetcher).fetchSellerProducts();
		await createOrderRepo(fetcher).fetchOrderStatusCounts();
		expect(seen.map((call) => call.path)).toEqual([
			"/app/products?page=1&page_size=20",
			"/app/orders/status_counts",
		]);
	});
});
