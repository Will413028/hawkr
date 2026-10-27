/** @vitest-environment jsdom */

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
	act,
	cleanup,
	fireEvent,
	render,
	renderHook,
	screen,
} from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import HomeCompactSearchExperience from "./HomeCompactSearchExperience";
import {
	homeSearchQueryKeys,
	useHomeSearchQueries,
} from "./useHomeSearchQueries";

vi.mock("@/lib/search/repo", () => ({ fetchPopularHashtags: async () => [] }));
vi.mock("@/features/procurement-assistant/availability", () => ({
	HAWKR_AGENT_ENABLED: false,
}));
vi.mock("@/features/procurement-assistant/AssistantEntryLink", () => ({
	default: () => null,
}));
vi.mock("@/components/shared/AppImage", () => ({ default: () => null }));

const clients: QueryClient[] = [];
let requests: ReturnType<typeof vi.fn<typeof fetch>>;
function createClient() {
	const client = new QueryClient({
		defaultOptions: { queries: { retry: false } },
	});
	clients.push(client);
	return client;
}
function provider(client: QueryClient) {
	return ({ children }: { children: React.ReactNode }) =>
		React.createElement(QueryClientProvider, { client }, children);
}
function response(body: unknown) {
	return Promise.resolve(new Response(JSON.stringify(body), { status: 200 }));
}
function queryResponse(input: RequestInfo | URL) {
	const url = new URL(String(input), "https://hawkr.test");
	const q = url.searchParams.get("q");
	return response(
		url.pathname.endsWith("suggest")
			? { suggestions: [{ keyword: `${q}建議` }] }
			: url.pathname.endsWith("products")
				? { records: [{ id: q, name: `${q}商品` }], total: 1 }
				: {},
	);
}
function keywordCalls() {
	return requests.mock.calls.filter(([url]) =>
		String(url).includes("/search/"),
	);
}
async function advance(ms: number) {
	await act(async () => {
		await vi.advanceTimersByTimeAsync(ms);
	});
}
function panel(open = true, query = "芒果") {
	return React.createElement(HomeCompactSearchExperience, {
		open,
		query,
		requestQuery: query,
		coords: { top: 100, left: 20, width: 350, maxHeight: 600 },
		panelRef: React.createRef<HTMLDivElement>(),
		onDismiss: vi.fn(),
		onNavigate: vi.fn(),
	});
}
beforeEach(() => {
	vi.useFakeTimers();
	vi.setSystemTime(new Date("2026-09-07T12:00:00Z"));
	requests = vi.fn<typeof fetch>().mockImplementation(queryResponse);
	vi.stubGlobal("fetch", requests);
});
afterEach(() => {
	cleanup();
	for (const client of clients.splice(0)) client.clear();
	vi.useRealTimers();
	vi.unstubAllGlobals();
});

describe("home search cache reuse", () => {
	it("shares an in-flight request between popovers and does not cancel the remaining observer", async () => {
		const client = createClient();
		const wrapper = provider(client);
		const first = renderHook(() => useHomeSearchQueries(true, "芒果"), {
			wrapper,
		});
		const second = renderHook(() => useHomeSearchQueries(true, "芒果"), {
			wrapper,
		});
		await advance(100);
		first.unmount();
		await advance(170);
		expect(keywordCalls()).toHaveLength(2);
		expect(second.result.current.products.data?.records[0].name).toBe(
			"芒果商品",
		);
	});

	it("shows a first-load failure as retryable instead of claiming no products", async () => {
		requests.mockImplementation((input) =>
			String(input).includes("/search/")
				? Promise.resolve(new Response("unavailable", { status: 502 }))
				: queryResponse(input),
		);
		render(panel(), { wrapper: provider(createClient()) });
		await advance(270);
		expect(screen.getByText("商品結果暫時載入失敗")).toBeTruthy();
		expect(screen.queryByText("目前沒有相關商品")).toBeNull();
		expect(screen.getAllByRole("button", { name: "重新載入" })).toHaveLength(2);
	});

	it("reopens fresh results synchronously without fetching or showing a skeleton", async () => {
		const client = createClient();
		const wrapper = provider(client);
		const view = render(panel(), { wrapper });
		await advance(270);
		expect(screen.getByText("芒果商品")).toBeTruthy();
		expect(keywordCalls()).toHaveLength(2);
		view.rerender(panel(false));
		expect(screen.queryByRole("dialog")).toBeNull();
		view.rerender(panel());
		expect(screen.getByText("芒果商品")).toBeTruthy();
		expect(screen.getByText("芒果建議")).toBeTruthy();
		expect(document.querySelector("[role=dialog] .animate-pulse")).toBeNull();
		await advance(1_000);
		expect(keywordCalls()).toHaveLength(2);
		view.unmount();
		render(panel(), { wrapper });
		expect(screen.getByText("芒果商品")).toBeTruthy();
		await advance(270);
		expect(keywordCalls()).toHaveLength(2);
	});

	it("debounces a new keyword without relabeling another keyword's cached data", async () => {
		const client = createClient();
		const view = renderHook(
			({ open, query }) => useHomeSearchQueries(open, query),
			{
				initialProps: { open: true, query: "芒果" },
				wrapper: provider(client),
			},
		);
		await advance(270);
		expect(view.result.current.products.data?.records[0].name).toBe("芒果商品");
		view.rerender({ open: true, query: "鳳" });
		expect(view.result.current.products.data).toBeUndefined();
		await advance(100);
		view.rerender({ open: true, query: "鳳梨" });
		await advance(249);
		expect(keywordCalls()).toHaveLength(2);
		expect(view.result.current.products.data).toBeUndefined();
		await advance(21);
		expect(keywordCalls()).toHaveLength(4);
		expect(view.result.current.products.data?.records[0].name).toBe("鳳梨商品");
	});

	it("keeps the popover's related-products payload separate from full 16-result searches", async () => {
		const client = createClient();
		const fullKey = ["search", "products", "芒果", "推薦"];
		client.setQueryData(fullKey, {
			records: [{ id: "full-only", name: "完整搜尋" }],
			total: 16,
		});
		const view = renderHook(() => useHomeSearchQueries(true, " 芒果 "), {
			wrapper: provider(client),
		});
		expect(view.result.current.products.data).toBeUndefined();
		await advance(270);
		expect(view.result.current.products.data?.records[0].name).toBe("芒果商品");
		const productCall = requests.mock.calls.find(([url]) =>
			String(url).includes("/products"),
		);
		const url = new URL(String(productCall?.[0]), "https://hawkr.test");
		expect(url.searchParams.get("page")).toBe("1");
		expect(url.searchParams.get("page_size")).toBe("6");
		expect(client.getQueryData(fullKey)).toEqual({
			records: [{ id: "full-only", name: "完整搜尋" }],
			total: 16,
		});
	});

	it("aborts both searches on close and ignores a transport that resolves late", async () => {
		const client = createClient();
		const pending: Array<{
			signal: AbortSignal;
			resolve: (value: Response) => void;
		}> = [];
		requests.mockImplementation(
			(_url, init) =>
				new Promise<Response>((resolve) => {
					pending.push({ signal: init?.signal as AbortSignal, resolve });
				}),
		);
		const view = renderHook(
			({ open, query }) => useHomeSearchQueries(open, query),
			{
				initialProps: { open: true, query: "芒果" },
				wrapper: provider(client),
			},
		);
		await advance(270);
		expect(pending).toHaveLength(2);
		view.rerender({ open: false, query: "芒果" });
		expect(pending.every((request) => request.signal.aborted)).toBe(true);
		requests.mockImplementation(queryResponse);
		view.rerender({ open: true, query: "鳳梨" });
		await advance(270);
		await act(async () => {
			for (const request of pending)
				request.resolve(
					new Response(
						JSON.stringify({
							records: [{ id: "old", name: "過期芒果" }],
							total: 1,
							suggestions: [{ keyword: "過期芒果建議" }],
						}),
					),
				);
		});
		expect(view.result.current.products.data?.records[0].name).toBe("鳳梨商品");
		expect(
			client.getQueryData(homeSearchQueryKeys.products("芒果")),
		).toBeUndefined();
		expect(
			client.getQueryData(homeSearchQueryKeys.suggestions("芒果")),
		).toBeUndefined();
	});

	it("shows cached products and suggestions during stale refetch failure, with independent retries", async () => {
		const client = createClient();
		const updatedAt = Date.now() - 31_000;
		client.setQueryData(
			homeSearchQueryKeys.products("芒果"),
			{ records: [{ id: "old", name: "上次芒果" }], total: 1 },
			{ updatedAt },
		);
		client.setQueryData(
			homeSearchQueryKeys.suggestions("芒果"),
			[{ keyword: "上次建議" }],
			{ updatedAt },
		);
		requests.mockImplementation((input) =>
			String(input).includes("/search/")
				? Promise.resolve(new Response("unavailable", { status: 503 }))
				: queryResponse(input),
		);
		render(panel(), { wrapper: provider(client) });
		expect(screen.getByText("上次芒果")).toBeTruthy();
		await advance(270);
		expect(screen.getByText("上次芒果")).toBeTruthy();
		expect(screen.getByText("上次建議")).toBeTruthy();
		expect(screen.getByText(/商品結果更新失敗/)).toBeTruthy();
		expect(screen.getByText(/關鍵字建議更新失敗/)).toBeTruthy();
		requests.mockImplementation(queryResponse);
		fireEvent.click(screen.getByRole("button", { name: "重新載入商品" }));
		await advance(270);
		expect(screen.getByText("芒果商品")).toBeTruthy();
		expect(screen.getByText("上次建議")).toBeTruthy();
		expect(keywordCalls()).toHaveLength(3);
	});

	it("collects inactive results after five minutes and requests them again", async () => {
		const client = createClient();
		const view = renderHook(({ open }) => useHomeSearchQueries(open, "芒果"), {
			initialProps: { open: true },
			wrapper: provider(client),
		});
		await advance(270);
		view.rerender({ open: false });
		await advance(5 * 60_000 + 1);
		expect(
			client.getQueryData(homeSearchQueryKeys.products("芒果")),
		).toBeUndefined();
		view.rerender({ open: true });
		expect(view.result.current.products.data).toBeUndefined();
		await advance(270);
		expect(keywordCalls()).toHaveLength(4);
	});
});
