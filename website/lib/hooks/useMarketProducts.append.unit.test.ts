/** @vitest-environment jsdom */

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fetchProducts } from "@/lib/api";
import type { ApiEnvelope } from "@/lib/api/client";
import { fetchCanonicalMarketRecords } from "@/lib/api/main-tab-pages";
import { mapProduct, type ProductRecord } from "@/lib/market/listFilters";
import type { MarketProductItem } from "@/lib/market/types";
import { useMarketProducts } from "./useMarketProducts";

vi.mock("@/lib/api", () => ({ fetchProducts: vi.fn() }));
vi.mock("@/lib/api/main-tab-pages", () => ({
	fetchCanonicalMarketRecords: vi.fn(),
}));
vi.mock("./useQueryIdentity", () => ({
	useQueryIdentity: () => ({ accountId: null }),
}));
// Keep the data hook, queue, mapper and TanStack cache real. Only automatic
// tab scheduling is disabled; explicit refresh() still executes the real hook.
vi.mock("@/lib/hooks/useMainTabActivationRefresh", () => ({
	useMainTabActivationRefresh: () => ({ refreshing: false }),
}));

const clients: QueryClient[] = [];
const originalScrollY = Object.getOwnPropertyDescriptor(window, "scrollY");

function record(
	id: number,
	overrides: Partial<ProductRecord> = {},
): ProductRecord {
	return {
		id: String(id),
		name: `商品 ${id}`,
		imageKey: `public/product/${id}/photo.png`,
		priceMin: "100",
		stock: "12",
		seller: { name: "測試賣家", storeId: "seller-1", storeCode: "fresh" },
		hashtags: ["當季"],
		...overrides,
	};
}

function deferred<T>() {
	let resolve!: (value: T) => void;
	const promise = new Promise<T>((resolvePromise) => {
		resolve = resolvePromise;
	});
	return { promise, resolve };
}

function pageEnvelope(
	records: ProductRecord[],
): ApiEnvelope<{ records: ProductRecord[] }> {
	return { code: 0, data: { records } };
}

function mountProducts(
	initialProducts: MarketProductItem[],
	pageSize: number,
	enableActivationRefresh = false,
) {
	const client = new QueryClient({
		defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } },
	});
	clients.push(client);
	const fetchParams = { keyword: "" };
	const wrapper = ({ children }: { children: React.ReactNode }) =>
		React.createElement(QueryClientProvider, { client }, children);
	return renderHook(
		() =>
			useMarketProducts({
				initialProducts,
				initialDone: false,
				pageSize,
				fetchParams,
				enableActivationRefresh,
			}),
		{ wrapper },
	);
}

function expectSameItems(
	actual: MarketProductItem[],
	expected: MarketProductItem[],
) {
	for (const [index, product] of expected.entries()) {
		expect(actual[index]).toBe(product);
	}
}

beforeEach(() => {
	vi.resetAllMocks();
	Object.defineProperty(window, "scrollY", { configurable: true, value: 0 });
});

afterEach(() => {
	cleanup();
	for (const client of clients.splice(0)) client.clear();
	vi.restoreAllMocks();
	if (originalScrollY)
		Object.defineProperty(window, "scrollY", originalScrollY);
});

describe.each([
	false,
	true,
])("useMarketProducts append identity (canonical=%s)", (enableActivationRefresh) => {
	it.each([
		1, 24, 100,
	])("retains %i existing product references while loading and appending", async (pageSize) => {
		const initial = Array.from({ length: pageSize }, (_, index) =>
			mapProduct(record(index + 1), index),
		);
		const response = deferred<ProductRecord[]>();
		if (enableActivationRefresh) {
			vi.mocked(fetchCanonicalMarketRecords).mockReturnValueOnce(
				response.promise,
			);
		} else {
			vi.mocked(fetchProducts).mockImplementationOnce(async () =>
				pageEnvelope(await response.promise),
			);
		}
		const { result, rerender } = mountProducts(
			initial,
			pageSize,
			enableActivationRefresh,
		);
		const seeded = result.current.products;
		expectSameItems(seeded, initial);
		expect(fetchProducts).not.toHaveBeenCalled();
		expect(fetchCanonicalMarketRecords).not.toHaveBeenCalled();

		let append!: Promise<void>;
		act(() => {
			append = result.current.loadMore();
		});
		await waitFor(() => expect(result.current.loading).toBe(true));
		expect(result.current.products).toBe(seeded);
		expectSameItems(result.current.products, initial);
		rerender();
		expect(result.current.products).toBe(seeded);

		if (enableActivationRefresh) {
			expect(fetchCanonicalMarketRecords).toHaveBeenCalledWith(
				2,
				expect.any(AbortSignal),
			);
			expect(fetchProducts).not.toHaveBeenCalled();
		} else {
			expect(fetchProducts).toHaveBeenCalledWith(
				"",
				2,
				pageSize,
				undefined,
				undefined,
				expect.objectContaining({ signal: expect.any(AbortSignal) }),
			);
			expect(fetchCanonicalMarketRecords).not.toHaveBeenCalled();
		}
		const appended = Array.from({ length: pageSize }, (_, index) =>
			record(pageSize + index + 1),
		);
		await act(async () => {
			response.resolve(appended);
			await append;
		});
		await waitFor(() =>
			expect(result.current.products).toHaveLength(pageSize * 2),
		);
		expect(result.current.loading).toBe(false);
		expect(result.current.products).not.toBe(seeded);
		expectSameItems(result.current.products, initial);
		expect(result.current.products.slice(pageSize)).toEqual(
			appended.map((item, index) => mapProduct(item, pageSize + index)),
		);
		expect(result.current.page).toBe(2);
		expect(result.current.done).toBe(false);
	});
});

describe("useMarketProducts dedupe and authoritative refresh", () => {
	it("deduplicates repeated records across appended pages without cloning surviving products", async () => {
		const initial = [record(1), record(2), record(3)].map(mapProduct);
		vi.mocked(fetchProducts)
			.mockResolvedValueOnce(pageEnvelope([record(3), record(4), record(4)]))
			.mockResolvedValueOnce(pageEnvelope([record(4), record(5)]));
		const { result } = mountProducts(initial, 3);
		await act(async () => {
			await result.current.loadMore();
		});
		await waitFor(() =>
			expect(result.current.products.map((item) => item.id)).toEqual([
				"1",
				"2",
				"3",
				"4",
			]),
		);
		expectSameItems(result.current.products, initial);
		const previous = result.current.products;
		await act(async () => {
			await result.current.loadMore();
		});
		await waitFor(() =>
			expect(result.current.products.map((item) => item.id)).toEqual([
				"1",
				"2",
				"3",
				"4",
				"5",
			]),
		);
		expectSameItems(result.current.products, previous);
		expect(result.current.page).toBe(3);
		expect(result.current.done).toBe(true);
	});

	it("applies changed commerce fields after append and preserves buyer state until accepting fresh presentation", async () => {
		const initial = [
			mapProduct(
				record(1, { isFavorited: true, isRestockSubscribed: true }),
				0,
			),
			mapProduct(record(2), 1),
		];
		const changed = record(1, {
			name: "更新商品名稱",
			priceMin: "180",
			stock: "0",
			favoriteCount: "9",
			isFavorited: false,
			isRestockSubscribed: false,
		});
		vi.mocked(fetchProducts)
			.mockResolvedValueOnce(pageEnvelope([record(3), record(4)]))
			.mockResolvedValueOnce(pageEnvelope([record(5), changed]));
		const { result } = mountProducts(initial, 2);
		await act(async () => {
			await result.current.loadMore();
		});
		await waitFor(() => expect(result.current.products).toHaveLength(4));
		const beforeRefresh = result.current.products;
		Object.defineProperty(window, "scrollY", {
			configurable: true,
			value: 400,
		});
		await act(async () => {
			await result.current.refresh();
		});
		await waitFor(() => expect(result.current.products[0]?.stock).toBe(0));
		expect(result.current.products.map((item) => item.id)).toEqual([
			"1",
			"2",
			"3",
			"4",
		]);
		expect(result.current.products[0]).not.toBe(beforeRefresh[0]);
		expect(result.current.products[0]).toMatchObject({
			name: initial[0]?.name,
			priceValue: 180,
			stock: 0,
			favoriteCount: 9,
			isFavorited: true,
			isRestockSubscribed: true,
		});
		for (let index = 1; index < beforeRefresh.length; index++) {
			expect(result.current.products[index]).toBe(beforeRefresh[index]);
		}
		expect(result.current.hasPendingFresh).toBe(true);
		expect(result.current.pendingNewCount).toBe(1);
		const scrollTo = vi.spyOn(window, "scrollTo").mockImplementation(() => {});
		await act(async () => {
			result.current.applyPendingFresh();
		});
		await waitFor(() =>
			expect(result.current.products.map((item) => item.id)).toEqual([
				"5",
				"1",
			]),
		);
		expect(result.current.products[1]).toMatchObject({
			name: "更新商品名稱",
			priceValue: 180,
			stock: 0,
			isFavorited: true,
			isRestockSubscribed: true,
		});
		expect(result.current.hasPendingFresh).toBe(false);
		expect(result.current.pendingNewCount).toBe(0);
		expect(scrollTo).toHaveBeenCalledWith({ top: 0, behavior: "auto" });
	});
});
