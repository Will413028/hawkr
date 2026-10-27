/** @vitest-environment jsdom */

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
	act,
	cleanup,
	fireEvent,
	render,
	screen,
	waitFor,
} from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import SearchForm from "./SearchForm";
import SearchResultsClient from "./SearchResultsClient";

const mocks = vi.hoisted(() => ({
	products: vi.fn(),
	push: vi.fn(),
}));
vi.mock("@/lib/search/repo", () => ({
	searchProducts: mocks.products,
}));
vi.mock("@/lib/navigation/use-observed-router", () => ({
	useObservedRouter: () => ({ push: mocks.push }),
}));
vi.mock("@/features/procurement-assistant/availability", () => ({
	HAWKR_AGENT_ENABLED: false,
}));
vi.mock("@/features/procurement-assistant/AssistantEntryLink", () => ({
	default: () => null,
}));
vi.mock("@/features/market/components/MarketProductCard", () => ({
	default: ({
		product,
		imageLoading,
		priority,
	}: {
		product: { id: string; name: string };
		imageLoading?: string;
		priority?: boolean;
	}) =>
		React.createElement(
			"article",
			{
				"data-testid": `product-${product.id}`,
				"data-loading": imageLoading,
				"data-priority": priority,
			},
			product.name,
		),
}));
vi.mock("@/components/shared/AppImage", () => ({
	default: ({
		alt,
		loading,
		fetchPriority,
	}: {
		alt: string;
		loading?: string;
		fetchPriority?: string;
	}) =>
		React.createElement("span", {
			"data-testid": `image-${alt}`,
			"data-loading": loading,
			"data-fetch-priority": fetchPriority,
		}),
}));
vi.mock("@/components/shared/Link", () => ({
	default: ({ children, href }: { children: React.ReactNode; href: string }) =>
		React.createElement("a", { href }, children),
}));
vi.mock("@/components/ui/Button", () => ({
	default: ({
		children,
		isLoading: _loading,
		loadingLabel: _label,
		...props
	}: React.ButtonHTMLAttributes<HTMLButtonElement> & {
		isLoading?: boolean;
		loadingLabel?: string;
	}) => React.createElement("button", { type: "button", ...props }, children),
}));
vi.mock("@/components/ui", () => ({
	Button: ({
		children,
		...props
	}: React.ButtonHTMLAttributes<HTMLButtonElement>) =>
		React.createElement("button", { type: "button", ...props }, children),
}));

function deferred<T>() {
	let resolve!: (value: T) => void;
	let reject!: (reason: Error) => void;
	const promise = new Promise<T>((done, fail) => {
		resolve = done;
		reject = fail;
	});
	return { promise, resolve, reject };
}

type ProductResponse = {
	records: Array<{ id: string; name: string }>;
	total: number;
};
function submitSearch(input: HTMLElement) {
	const form = input.closest("form");
	if (!form) throw new Error("Search form missing");
	fireEvent.submit(form);
}
const clients: QueryClient[] = [];
function renderResults(query = "芒果", sort = "推薦") {
	const client = new QueryClient({
		defaultOptions: { queries: { retry: false, gcTime: Infinity } },
	});
	clients.push(client);
	const element = (nextQuery: string, nextSort: string) =>
		React.createElement(
			QueryClientProvider,
			{ client },
			React.createElement(SearchResultsClient, {
				query: nextQuery,
				sort: nextSort,
			}),
		);
	const result = render(element(query, sort));
	return {
		...result,
		client,
		update: (nextQuery: string, nextSort = sort) =>
			result.rerender(element(nextQuery, nextSort)),
	};
}
beforeEach(() => {
	vi.clearAllMocks();
	mocks.products.mockResolvedValue({ records: [], total: 0 });
});
afterEach(() => {
	cleanup();
	for (const client of clients.splice(0)) client.clear();
});

describe("search result continuity", () => {
	it("retains cached cards when a background update fails, then retries", async () => {
		mocks.products.mockResolvedValue({
			records: [{ id: "mango", name: "快取芒果" }],
			total: 1,
		});
		const view = renderResults();
		await screen.findByTestId("product-mango");
		mocks.products.mockRejectedValue(new Error("background product outage"));
		await act(async () => {
			await view.client.invalidateQueries();
		});
		expect(
			await screen.findByText("搜尋結果更新失敗，先顯示上次結果。"),
		).toBeTruthy();
		expect(screen.getByTestId("product-mango")).toBeTruthy();
		expect(document.querySelector(".route-product-grid-skeleton")).toBeNull();
		mocks.products.mockResolvedValue({
			records: [{ id: "new-mango", name: "更新芒果" }],
			total: 1,
		});
		fireEvent.click(screen.getByRole("button", { name: "重新載入搜尋結果" }));
		await screen.findByTestId("product-new-mango");
		expect(mocks.products).toHaveBeenCalledTimes(3);
	});

	it("returns to a fresh keyword cache without requesting the same results again", async () => {
		mocks.products.mockImplementation(({ q }: { q: string }) =>
			Promise.resolve({ records: [{ id: q, name: q }], total: 1 }),
		);
		const view = renderResults();
		await screen.findByTestId("product-芒果");
		view.update("鳳梨");
		await screen.findByTestId("product-鳳梨");
		view.update("芒果");
		expect(screen.getByTestId("product-芒果")).toBeTruthy();
		expect(document.querySelector(".route-product-grid-skeleton")).toBeNull();
		expect(mocks.products).toHaveBeenCalledTimes(2);
	});

	it("keeps a previously successful empty result distinct from a failed background update", async () => {
		const view = renderResults();
		await screen.findByText(/沒有找到「芒果」/);
		mocks.products.mockRejectedValue(new Error("background outage"));
		await act(async () => {
			await view.client.invalidateQueries({ queryKey: ["search", "products"] });
		});
		expect(
			await screen.findByText("搜尋結果更新失敗，先顯示上次結果。"),
		).toBeTruthy();
		expect(screen.getByText(/沒有找到「芒果」/)).toBeTruthy();
		expect(document.querySelector(".route-product-grid-skeleton")).toBeNull();
	});

	it("shows all result text while reserving eager requests for the first product row", async () => {
		mocks.products.mockResolvedValue({
			records: Array.from({ length: 16 }, (_, index) => ({
				id: String(index),
				name: `商品 ${index}`,
			})),
			total: 16,
		});
		renderResults();
		await screen.findByTestId("product-15");
		expect(screen.getByTestId("product-0").getAttribute("data-priority")).toBe(
			"true",
		);
		for (let index = 1; index < 4; index += 1)
			expect(
				screen.getByTestId(`product-${index}`).getAttribute("data-loading"),
			).toBe("eager");
		for (let index = 4; index < 16; index += 1)
			expect(
				screen.getByTestId(`product-${index}`).getAttribute("data-loading"),
			).toBe("lazy");
	});

	it("does not report zero products before the product response arrives", () => {
		mocks.products.mockReturnValue(new Promise(() => {}));
		renderResults();
		expect(screen.getByRole("status").textContent).toContain("搜尋商品");
		expect(screen.queryByText(/件商品/)).toBeNull();
		expect(screen.queryByText(/沒有找到/)).toBeNull();
		expect(document.querySelector(".route-product-grid-skeleton")).toBeTruthy();
	});

	it("shows authoritative product results once the product response arrives", async () => {
		const products = deferred<ProductResponse>();
		mocks.products.mockReturnValue(products.promise);
		renderResults();
		await act(async () => {
			products.resolve({
				records: [{ id: "fixture-mango", name: "測試芒果" }],
				total: 12,
			});
		});
		expect(await screen.findByTestId("product-fixture-mango")).toBeTruthy();
		expect(screen.getByText("12")).toBeTruthy();
		expect(document.querySelector(".route-product-grid-skeleton")).toBeNull();
	});

	it("shows zero results only after a successful empty product response", async () => {
		const products = deferred<ProductResponse>();
		mocks.products.mockReturnValue(products.promise);
		renderResults();
		expect(screen.queryByText(/沒有找到/)).toBeNull();
		await act(async () => {
			products.resolve({ records: [], total: 0 });
		});
		expect(await screen.findByText(/沒有找到「芒果」/)).toBeTruthy();
		expect(screen.getByText("0")).toBeTruthy();
	});

	it("keeps only the same keyword's products while a new sort is updating", async () => {
		const sorted = deferred<ProductResponse>();
		mocks.products
			.mockResolvedValueOnce({
				records: [{ id: "fixture-mango", name: "測試芒果" }],
				total: 12,
			})
			.mockReturnValue(sorted.promise);
		const view = renderResults();
		await screen.findByTestId("product-fixture-mango");
		view.update("芒果", "價格低到高");
		expect(screen.getByTestId("product-fixture-mango")).toBeTruthy();
		expect(screen.getByRole("status").textContent).toContain("更新排序");
		expect(document.querySelector(".route-product-grid-skeleton")).toBeNull();
		await waitFor(() => expect(mocks.products).toHaveBeenCalledTimes(2));
		view.update("鳳梨", "價格低到高");
		expect(screen.queryByTestId("product-fixture-mango")).toBeNull();
	});

	it("passes cancellation to the product search when a new keyword supersedes it", async () => {
		mocks.products.mockReturnValue(new Promise(() => {}));
		const view = renderResults();
		await waitFor(() => expect(mocks.products).toHaveBeenCalledTimes(1));
		const productOptions = mocks.products.mock.calls[0][0];
		expect(productOptions.strict).toBe(true);
		expect(productOptions.signal).toBeInstanceOf(AbortSignal);
		view.update("鳳梨");
		await waitFor(() => expect(productOptions.signal.aborted).toBe(true));
	});

	it("does not render an obsolete response if its transport resolves after cancellation", async () => {
		const old = deferred<ProductResponse>();
		const next = deferred<ProductResponse>();
		mocks.products.mockImplementation(({ q }: { q: string }) =>
			q === "芒果" ? old.promise : next.promise,
		);
		const view = renderResults();
		view.update("鳳梨");
		await act(async () => {
			next.resolve({
				records: [{ id: "fixture-pineapple", name: "測試鳳梨" }],
				total: 1,
			});
		});
		await screen.findByTestId("product-fixture-pineapple");
		await act(async () => {
			old.resolve({
				records: [{ id: "fixture-mango", name: "過期測試芒果" }],
				total: 1,
			});
		});
		expect(screen.queryByTestId("product-fixture-mango")).toBeNull();
		expect(screen.getByTestId("product-fixture-pineapple")).toBeTruthy();
	});

	it("shows a product failure as retryable error instead of empty results", async () => {
		mocks.products.mockRejectedValue(new Error("product outage"));
		renderResults();
		expect(
			await screen.findByRole("button", { name: "重新載入搜尋結果" }),
		).toBeTruthy();
		expect(screen.queryByText(/沒有找到/)).toBeNull();
		expect(screen.queryByText(/件商品/)).toBeNull();
	});
});

describe("search draft continuity", () => {
	it("preserves text typed after submission when that URL finally commits", () => {
		const view = render(
			React.createElement(SearchForm, { query: "", sort: "推薦" }),
		);
		const input = screen.getByRole("textbox", {
			name: "搜尋商品",
		}) as HTMLInputElement;
		fireEvent.change(input, { target: { value: "芒" } });
		submitSearch(input);
		fireEvent.change(input, { target: { value: "芒果" } });
		view.rerender(
			React.createElement(SearchForm, { query: "芒", sort: "推薦" }),
		);
		expect(input.value).toBe("芒果");
		expect(mocks.push).toHaveBeenCalledWith("/search?q=%E8%8A%92", {
			scroll: false,
		});
		view.rerender(
			React.createElement(SearchForm, { query: "鳳梨", sort: "推薦" }),
		);
		expect(input.value).toBe("鳳梨");
	});

	it("normalizes the input after an unchanged submitted draft commits", () => {
		const view = render(
			React.createElement(SearchForm, { query: "", sort: "推薦" }),
		);
		const input = screen.getByRole("textbox", {
			name: "搜尋商品",
		}) as HTMLInputElement;
		fireEvent.change(input, { target: { value: " 芒果 " } });
		submitSearch(input);
		view.rerender(
			React.createElement(SearchForm, { query: "芒果", sort: "推薦" }),
		);
		expect(input.value).toBe("芒果");
	});

	it("preserves the latest draft through more than one delayed submitted query", () => {
		const view = render(
			React.createElement(SearchForm, { query: "", sort: "推薦" }),
		);
		const input = screen.getByRole("textbox", {
			name: "搜尋商品",
		}) as HTMLInputElement;
		fireEvent.change(input, { target: { value: "芒" } });
		submitSearch(input);
		fireEvent.change(input, { target: { value: "芒果" } });
		submitSearch(input);
		fireEvent.change(input, { target: { value: "芒果箱" } });
		view.rerender(
			React.createElement(SearchForm, { query: "芒", sort: "推薦" }),
		);
		expect(input.value).toBe("芒果箱");
		view.rerender(
			React.createElement(SearchForm, { query: "芒果", sort: "推薦" }),
		);
		expect(input.value).toBe("芒果箱");
		view.rerender(
			React.createElement(SearchForm, { query: "芒", sort: "推薦" }),
		);
		expect(input.value).toBe("芒");
	});
});
