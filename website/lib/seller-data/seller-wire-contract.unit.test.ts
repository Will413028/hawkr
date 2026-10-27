import { afterEach, describe, expect, it, vi } from "vitest";

/**
 * 賣家路徑的線上契約（本次工廠化重構的回歸網）。
 *
 * 這次改的是**全賣家共用的資料層**，弄壞的症狀是賣家中心整片壞掉，不是代行不能用。
 * 所以逐條釘住「賣家按下去之後實際送出的請求」：method、路徑、body ——
 * 與重構前 `git show HEAD:lib/{products/seller-repo,seller/repo}.ts` 逐字相同。
 *
 * 覆蓋 §13 要求的賣家流程：建商品／編商品／上下架／發貼文。改店面已經是 Connect（`SellerAppService.UpdateMyStore`），
 * 它的請求形狀由 `lib/seller/repo.unit.test.ts` 釘住。
 */

const calls: Array<{
	method: string;
	path: string;
	query: string;
	body: unknown;
}> = [];

function stubBrowserFetch(payloads: unknown[]) {
	let index = 0;
	const fetchSpy = vi.fn(async (url: string, init: RequestInit) => {
		const parsed = new URL(url, "https://local.invalid");
		calls.push({
			method: String(init.method ?? "GET"),
			path: parsed.pathname,
			query: parsed.search,
			body: init.body ? JSON.parse(String(init.body)) : undefined,
		});
		const payload = payloads[index] ?? { code: 0, data: {} };
		index += 1;
		return new Response(JSON.stringify(payload), {
			status: 200,
			headers: { "content-type": "application/json" },
		});
	});
	vi.stubGlobal("window", {
		location: { pathname: "/seller-center" },
		Clerk: { loaded: true, session: { getToken: async () => "test-token" } },
	});
	vi.stubGlobal("fetch", fetchSpy);
	return fetchSpy;
}

afterEach(() => {
	calls.length = 0;
	vi.unstubAllGlobals();
});

describe("賣家 REST 流程的請求形狀不得改變", () => {
	it("建商品 / 編商品 / 上下架", async () => {
		stubBrowserFetch([{ code: 0, data: { id: 7 } }, { code: 0 }, { code: 0 }]);
		const { createProduct, updateProduct, updateProductStatus } = await import(
			"@/lib/products/seller-repo"
		);

		await createProduct({ name: "高山蜜梨", idempotencyKey: "k-1" });
		await updateProduct("2698", {
			name: "高山蜜梨禮盒",
			idempotencyKey: "k-2",
		});
		await updateProductStatus("2698", "4004");

		expect(calls).toEqual([
			{
				method: "POST",
				path: "/api/v1/app/products",
				query: "",
				body: { idempotencyKey: "k-1", name: "高山蜜梨" },
			},
			{
				method: "PUT",
				path: "/api/v1/app/products/2698",
				query: "",
				body: { idempotencyKey: "k-2", name: "高山蜜梨禮盒" },
			},
			{
				method: "PUT",
				path: "/api/v1/app/products/2698/status",
				query: "",
				body: { status: "4004" },
			},
		]);
	});

	it("發貼文 ＋ 讀自己的貼文列表", async () => {
		stubBrowserFetch([
			{ code: 0, data: { id: 9 } },
			{ code: 0, data: [] },
		]);
		const { createSellerPost, fetchSellerPosts } = await import(
			"@/lib/seller/repo"
		);

		await createSellerPost({ content: "今天採收", stagedToken: "stg_x" });
		await fetchSellerPosts(undefined, { pageSize: 50 });

		expect(calls).toEqual([
			{
				method: "POST",
				path: "/api/v1/app/sellers/posts",
				query: "",
				body: { content: "今天採收", stagedToken: "stg_x" },
			},
			{
				method: "GET",
				path: "/api/v1/app/sellers/me/posts",
				query: "?page_size=50",
				body: undefined,
			},
		]);
	});
});
