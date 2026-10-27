/** @vitest-environment jsdom */

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fetchProducts } from "@/lib/api";
import {
	fetchCanonicalFeedPage,
	fetchCanonicalMarketRecords,
} from "@/lib/api/main-tab-pages";
import type { PostListItem } from "@/lib/api/openapi-types";
import { fetchPublicPostsTyped } from "@/lib/api/posts";
import { mapProduct, type ProductRecord } from "@/lib/market/listFilters";
import type { MarketListSnapshot } from "@/lib/market/listSnapshot";
import { queryKeys } from "@/lib/query/keys";
import { useMainTabFreshnessStore } from "@/lib/state/main-tab-freshness-store";
import { useFeedPosts } from "./useFeedPosts";
import {
	marketProductsListQueryKey,
	useMarketProducts,
} from "./useMarketProducts";

vi.mock("@/lib/api", () => ({ fetchProducts: vi.fn() }));
vi.mock("@/lib/api/posts", () => ({ fetchPublicPostsTyped: vi.fn() }));
vi.mock("@/lib/api/main-tab-pages", () => ({
	fetchCanonicalFeedPage: vi.fn(),
	fetchCanonicalMarketRecords: vi.fn(),
}));
const identity = vi.hoisted(() => ({ accountId: null as string | null }));
vi.mock("./useQueryIdentity", () => ({ useQueryIdentity: () => identity }));

const clients: QueryClient[] = [];
const originalScrollY = Object.getOwnPropertyDescriptor(window, "scrollY");
let now = 1_000_000;

function makeClient() {
	const client = new QueryClient({
		defaultOptions: {
			queries: { retry: false, refetchOnWindowFocus: false },
		},
	});
	clients.push(client);
	const wrapper = ({ children }: { children: React.ReactNode }) =>
		React.createElement(QueryClientProvider, { client }, children);
	return { client, wrapper };
}

function deferred<T>() {
	let resolve!: (value: T) => void;
	let reject!: (reason: Error) => void;
	const promise = new Promise<T>((resolvePromise, rejectPromise) => {
		resolve = resolvePromise;
		reject = rejectPromise;
	});
	return { promise, resolve, reject };
}

function post(id: string, likeCount = 1): PostListItem {
	return {
		id,
		sellerId: "seller",
		storeCode: "2685914682",
		sellerName: "Seller",
		logoKey: "",
		content: id,
		imageKeys: [],
		product: null,
		commentCount: 0,
		likeCount,
		shareCount: 0,
		viewCount: 0,
		isLiked: false,
		createdAt: 0,
		tags: [],
	};
}

function record(id: string, stock = "12"): ProductRecord {
	return {
		id,
		name: `商品 ${id}`,
		imageKey: `public/product/${id}/photo.png`,
		priceMin: "100",
		stock,
		seller: { name: "測試賣家", storeId: "seller-1", storeCode: "fresh" },
	};
}

function feedPage(records: PostListItem[], page = 1) {
	return { records, page, pageSize: 1, total: 3 };
}

function seedFeed(
	client: QueryClient,
	scope: { sellerId?: string; hashtag?: string },
	updatedAt = now - 16_000,
) {
	const key = queryKeys.feed.posts({
		sellerId: scope.sellerId ?? "",
		hashtag: scope.hashtag ?? "",
		pageSize: 1,
	});
	client.setQueryData(
		key,
		{
			pages: [
				{ records: [post("1")], page: 1, pageSize: 1, hasMore: true },
				{ records: [post("2")], page: 2, pageSize: 1, hasMore: true },
			],
			pageParams: [1, 2],
		},
		{ updatedAt },
	);
	return key;
}

function seedMarket(
	client: QueryClient,
	params: Parameters<typeof marketProductsListQueryKey>[0],
	updatedAt = now - 31_000,
	accountId: string | null = null,
) {
	const key = marketProductsListQueryKey(params, 1, accountId);
	client.setQueryData(
		key,
		{
			pages: [
				{ products: [mapProduct(record("1"), 0)], page: 1, hasMore: true },
				{ products: [mapProduct(record("2"), 1)], page: 2, hasMore: true },
			],
			pageParams: [1, 2],
		},
		{ updatedAt },
	);
	return key;
}

function focus() {
	act(() => window.dispatchEvent(new Event("focus")));
}

async function settleMount() {
	await act(async () => {
		await new Promise((resolve) => setTimeout(resolve, 10));
	});
}

beforeEach(() => {
	vi.resetAllMocks();
	identity.accountId = null;
	now = 1_000_000;
	vi.spyOn(Date, "now").mockImplementation(() => now);
	vi.spyOn(document, "visibilityState", "get").mockReturnValue("visible");
	vi.spyOn(navigator, "onLine", "get").mockReturnValue(true);
	Object.defineProperty(window, "scrollY", { configurable: true, value: 500 });
	vi.spyOn(window, "scrollTo").mockImplementation(() => {});
	useMainTabFreshnessStore.getState().resetForTests();
});

afterEach(() => {
	cleanup();
	for (const client of clients.splice(0)) client.clear();
	useMainTabFreshnessStore.getState().resetForTests();
	vi.restoreAllMocks();
	if (originalScrollY)
		Object.defineProperty(window, "scrollY", originalScrollY);
});

describe("scoped feed activation", () => {
	it.each([
		{ sellerId: "seller-a" },
		{ hashtag: "當季" },
	])("keeps cached pages, refreshes only stale head and reuses a fresh revisit (%j)", async (scope) => {
		const { client, wrapper } = makeClient();
		const key = seedFeed(client, scope);
		const response = deferred<ReturnType<typeof feedPage>>();
		vi.mocked(fetchPublicPostsTyped).mockReturnValueOnce(response.promise);
		const mount = () =>
			renderHook(
				() => useFeedPosts({ initialPosts: [], pageSize: 1, ...scope }),
				{ wrapper },
			);
		const first = mount();
		expect(first.result.current.posts.map((item) => item.id)).toEqual([
			"1",
			"2",
		]);
		expect(first.result.current.initialLoading).toBe(false);
		await waitFor(() => expect(fetchPublicPostsTyped).toHaveBeenCalledTimes(1));
		for (let index = 0; index < 8; index++) focus();
		expect(fetchPublicPostsTyped).toHaveBeenCalledTimes(1);
		expect(fetchPublicPostsTyped).toHaveBeenCalledWith(
			expect.objectContaining({
				page: 1,
				pageSize: 1,
				sellerId: scope.sellerId,
				tag: scope.hashtag,
				signal: expect.any(AbortSignal),
			}),
		);
		await act(async () => response.resolve(feedPage([post("1", 9)])));
		await waitFor(() =>
			expect(first.result.current.posts[0]?.likeCount).toBe(9),
		);
		expect(first.result.current.page).toBe(2);
		expect(client.getQueryData(key)).toMatchObject({ pageParams: [1, 2] });
		expect(window.scrollTo).not.toHaveBeenCalled();
		expect(fetchCanonicalFeedPage).not.toHaveBeenCalled();
		first.unmount();
		const second = mount();
		await settleMount();
		expect(second.result.current.posts).toHaveLength(2);
		expect(fetchPublicPostsTyped).toHaveBeenCalledTimes(1);
	});

	it("reconciles the loaded tail after five minutes without an extra head request or polling", async () => {
		const { client, wrapper } = makeClient();
		seedFeed(client, { hashtag: "當季" }, now - 301_000);
		vi.mocked(fetchPublicPostsTyped).mockImplementation(async ({ page } = {}) =>
			feedPage([post(String(page), 9)], page),
		);
		const { result } = renderHook(
			() => useFeedPosts({ initialPosts: [], pageSize: 1, hashtag: "當季" }),
			{ wrapper },
		);
		await waitFor(() => expect(result.current.posts[1]?.likeCount).toBe(9));
		expect(
			vi.mocked(fetchPublicPostsTyped).mock.calls.map(([args]) => args?.page),
		).toEqual([1, 2]);
		expect(result.current.page).toBe(2);
		now += 600_000;
		await settleMount();
		expect(fetchPublicPostsTyped).toHaveBeenCalledTimes(2);
	});

	it("aborts a superseded scope and ignores its late response", async () => {
		const { client, wrapper } = makeClient();
		const oldKey = seedFeed(client, { sellerId: "a" });
		seedFeed(client, { sellerId: "b" }, now);
		const response = deferred<ReturnType<typeof feedPage>>();
		vi.mocked(fetchPublicPostsTyped).mockReturnValueOnce(response.promise);
		const { result, rerender } = renderHook(
			({ sellerId }) =>
				useFeedPosts({ initialPosts: [], pageSize: 1, sellerId }),
			{ wrapper, initialProps: { sellerId: "a" } },
		);
		await waitFor(() => expect(fetchPublicPostsTyped).toHaveBeenCalledTimes(1));
		const signal = vi.mocked(fetchPublicPostsTyped).mock.calls[0]?.[0]?.signal;
		rerender({ sellerId: "b" });
		expect(signal?.aborted).toBe(true);
		await act(async () => response.resolve(feedPage([post("late", 99)])));
		expect(result.current.posts.map((item) => item.id)).toEqual(["1", "2"]);
		expect(client.getQueryData(oldKey)).toMatchObject({
			pages: [
				{ records: [{ id: "1", likeCount: 1 }] },
				{ records: [{ id: "2" }] },
			],
		});
		expect(result.current.hasPendingFresh).toBe(false);
	});
});

describe("filtered market activation", () => {
	it("drops freshness metadata with a removed query and checks a recreated stale key", async () => {
		const { client, wrapper } = makeClient();
		const params = { keyword: "蔬菜" };
		const key = seedMarket(client, params);
		vi.mocked(fetchProducts).mockResolvedValue({
			code: 0,
			data: { records: [record("1", "0")] },
		});
		const mount = () =>
			renderHook(
				() =>
					useMarketProducts({
						initialProducts: [],
						initialDone: false,
						pageSize: 1,
						fetchParams: params,
						seedInitialData: false,
						enableScopedActivationRefresh: true,
					}),
				{ wrapper },
			);
		const first = mount();
		await waitFor(() =>
			expect(first.result.current.products[0]?.stock).toBe(0),
		);
		expect(
			Object.keys(useMainTabFreshnessStore.getState().scopes),
		).toHaveLength(2);
		first.unmount();
		client.removeQueries({ queryKey: key, exact: true });
		expect(useMainTabFreshnessStore.getState().scopes).toEqual({});
		seedMarket(client, params);
		const second = mount();
		await waitFor(() =>
			expect(second.result.current.products[0]?.stock).toBe(0),
		);
		expect(fetchProducts).toHaveBeenCalledTimes(2);
	});

	it("ignores a response after the hidden document cancels it and after its query is removed", async () => {
		const { client, wrapper } = makeClient();
		const params = { keyword: "蔬菜" };
		const key = seedMarket(client, params);
		const response = deferred<{
			code: number;
			data: { records: ProductRecord[] };
		}>();
		vi.mocked(fetchProducts).mockReturnValueOnce(response.promise);
		const { result, unmount } = renderHook(
			() =>
				useMarketProducts({
					initialProducts: [],
					initialDone: false,
					pageSize: 1,
					fetchParams: params,
					seedInitialData: false,
					enableScopedActivationRefresh: true,
				}),
			{ wrapper },
		);
		await waitFor(() => expect(fetchProducts).toHaveBeenCalledTimes(1));
		const signal = vi.mocked(fetchProducts).mock.calls[0]?.[5]?.signal;
		vi.spyOn(document, "visibilityState", "get").mockReturnValue("hidden");
		act(() => document.dispatchEvent(new Event("visibilitychange")));
		expect(signal?.aborted).toBe(true);
		await act(async () =>
			response.resolve({ code: 0, data: { records: [record("late", "0")] } }),
		);
		expect(result.current.products.map((item) => item.id)).toEqual(["1", "2"]);
		expect(result.current.hasPendingFresh).toBe(false);
		const manual = deferred<{
			code: number;
			data: { records: ProductRecord[] };
		}>();
		vi.mocked(fetchProducts).mockReturnValueOnce(manual.promise);
		let refresh!: Promise<void>;
		act(() => {
			refresh = result.current.refresh();
		});
		await waitFor(() => expect(fetchProducts).toHaveBeenCalledTimes(2));
		unmount();
		client.removeQueries({ queryKey: key, exact: true });
		await act(async () => {
			manual.resolve({ code: 0, data: { records: [record("late", "0")] } });
			await expect(refresh).rejects.toMatchObject({ name: "AbortError" });
		});
		expect(client.getQueryData(key)).toBeUndefined();
		expect(useMainTabFreshnessStore.getState().scopes).toEqual({});
	});

	it("does not restore an item removed from a newer query cache", async () => {
		const { client, wrapper } = makeClient();
		const params = { keyword: "蔬菜" };
		const key = marketProductsListQueryKey(params, 1, null);
		client.setQueryData(
			key,
			{
				pages: [
					{ products: [mapProduct(record("2"), 0)], page: 1, hasMore: false },
				],
				pageParams: [1],
			},
			{ updatedAt: now },
		);
		const restoredSnapshot: MarketListSnapshot = {
			key: "/market/products?q=蔬菜",
			products: [mapProduct(record("removed"), 0), mapProduct(record("2"), 1)],
			page: 2,
			done: false,
			scrollY: 500,
			savedAt: now - 20_000,
		};
		const { result } = renderHook(
			() =>
				useMarketProducts({
					initialProducts: [],
					initialDone: false,
					pageSize: 1,
					fetchParams: params,
					seedInitialData: false,
					enableScopedActivationRefresh: true,
					restoredSnapshot,
				}),
			{ wrapper },
		);
		expect(result.current.products.map((item) => item.id)).toEqual(["2"]);
		await settleMount();
		expect(result.current.products.map((item) => item.id)).toEqual(["2"]);
		expect(fetchProducts).not.toHaveBeenCalled();
	});

	it("discards a followed-list snapshot when the viewer changes before the new request resolves", async () => {
		identity.accountId = "11";
		const { wrapper } = makeClient();
		const params = { keyword: "", followedProductOnly: true };
		const restoredSnapshot: MarketListSnapshot = {
			key: "/market/products?followed=1",
			products: [mapProduct(record("viewer-a"), 0)],
			page: 1,
			done: true,
			scrollY: 500,
			savedAt: now,
		};
		const response = deferred<{
			code: number;
			data: { records: ProductRecord[] };
		}>();
		vi.mocked(fetchProducts).mockReturnValueOnce(response.promise);
		const { result, rerender } = renderHook(
			() =>
				useMarketProducts({
					initialProducts: [],
					initialDone: false,
					pageSize: 1,
					fetchParams: params,
					seedInitialData: false,
					enableScopedActivationRefresh: true,
					restoredSnapshot,
				}),
			{ wrapper },
		);
		expect(result.current.products[0]?.id).toBe("viewer-a");
		identity.accountId = "22";
		rerender();
		expect(result.current.products).toEqual([]);
		expect(result.current.initialLoading).toBe(true);
		await act(async () =>
			response.resolve({ code: 0, data: { records: [record("viewer-b")] } }),
		);
		await waitFor(() =>
			expect(result.current.products[0]?.id).toBe("viewer-b"),
		);
	});

	it("keeps filters and viewer key isolated while refreshing commerce fields in place", async () => {
		identity.accountId = "11";
		const { client, wrapper } = makeClient();
		const params = {
			keyword: "蔬菜",
			sort: "price_asc",
			followedSellerOnly: true,
		};
		const oldKey = seedMarket(client, params, now - 31_000, "11");
		const response = deferred<{
			code: number;
			data: { records: ProductRecord[] };
		}>();
		vi.mocked(fetchProducts).mockReturnValueOnce(response.promise);
		const { result, rerender } = renderHook(
			() =>
				useMarketProducts({
					initialProducts: [],
					initialDone: false,
					pageSize: 1,
					fetchParams: params,
					seedInitialData: false,
					enableScopedActivationRefresh: true,
				}),
			{ wrapper },
		);
		expect(result.current.initialLoading).toBe(false);
		await waitFor(() => expect(fetchProducts).toHaveBeenCalledTimes(1));
		expect(fetchProducts).toHaveBeenCalledWith(
			"蔬菜",
			1,
			1,
			undefined,
			"price_asc",
			expect.objectContaining({ followedSellerOnly: true }),
		);
		const signal = vi.mocked(fetchProducts).mock.calls[0]?.[5]?.signal;
		identity.accountId = "22";
		seedMarket(client, params, now, "22");
		rerender();
		expect(signal?.aborted).toBe(true);
		await act(async () =>
			response.resolve({ code: 0, data: { records: [record("late", "0")] } }),
		);
		expect(result.current.products.map((item) => item.id)).toEqual(["1", "2"]);
		expect(client.getQueryData(oldKey)).toMatchObject({
			pages: [
				{ products: [{ id: "1", stock: 12 }] },
				{ products: [{ id: "2" }] },
			],
		});
		expect(fetchCanonicalMarketRecords).not.toHaveBeenCalled();
	});

	it("fetches a cold filter only once and immediately reuses its fresh result on remount", async () => {
		const { wrapper } = makeClient();
		const params = { keyword: "蘋果" };
		vi.mocked(fetchProducts).mockResolvedValue({
			code: 0,
			data: { records: [record("apple")] },
		});
		const mount = () =>
			renderHook(
				() =>
					useMarketProducts({
						initialProducts: [],
						initialDone: false,
						pageSize: 1,
						fetchParams: params,
						seedInitialData: false,
						enableScopedActivationRefresh: true,
					}),
				{ wrapper },
			);
		const first = mount();
		expect(first.result.current.initialLoading).toBe(true);
		await waitFor(() =>
			expect(first.result.current.products[0]?.id).toBe("apple"),
		);
		await settleMount();
		first.unmount();
		const second = mount();
		expect(second.result.current.initialLoading).toBe(false);
		expect(second.result.current.products[0]?.id).toBe("apple");
		await settleMount();
		expect(fetchProducts).toHaveBeenCalledTimes(1);
	});

	it("retains pages on a failed full refresh and retries its tail despite a successful head", async () => {
		const { client, wrapper } = makeClient();
		const params = { keyword: "蔬菜" };
		seedMarket(client, params, now - 301_000);
		let tailFailed = false;
		vi.mocked(fetchProducts).mockImplementation(async (_keyword, page) => {
			if (page === 2 && !tailFailed) {
				tailFailed = true;
				throw new Error("offline");
			}
			return { code: 0, data: { records: [record(String(page), "0")] } };
		});
		const mount = () =>
			renderHook(
				() =>
					useMarketProducts({
						initialProducts: [],
						initialDone: false,
						pageSize: 1,
						fetchParams: params,
						seedInitialData: false,
						enableScopedActivationRefresh: true,
					}),
				{ wrapper },
			);
		const first = mount();
		await waitFor(() =>
			expect(first.result.current.products[0]?.stock).toBe(0),
		);
		expect(first.result.current.products[1]?.stock).toBe(12);
		expect(first.result.current.page).toBe(2);
		expect(vi.mocked(fetchProducts).mock.calls.map((args) => args[1])).toEqual([
			1, 2, 1,
		]);
		for (let index = 0; index < 5; index++) focus();
		expect(fetchProducts).toHaveBeenCalledTimes(3);
		first.unmount();
		now += 11_000;
		const second = mount();
		await waitFor(() =>
			expect(second.result.current.products[1]?.stock).toBe(0),
		);
		expect(vi.mocked(fetchProducts).mock.calls.map((args) => args[1])).toEqual([
			1, 2, 1, 1, 2,
		]);
		expect(second.result.current.products).toHaveLength(2);
		expect(window.scrollTo).not.toHaveBeenCalled();
	});

	it("does not refresh a preview that did not opt in", async () => {
		const { client, wrapper } = makeClient();
		const params = { keyword: "preview" };
		seedMarket(client, params, now - 600_000);
		renderHook(
			() =>
				useMarketProducts({
					initialProducts: [],
					initialDone: false,
					pageSize: 1,
					fetchParams: params,
				}),
			{ wrapper },
		);
		await settleMount();
		focus();
		expect(fetchProducts).not.toHaveBeenCalled();
		expect(fetchCanonicalMarketRecords).not.toHaveBeenCalled();
	});
});
