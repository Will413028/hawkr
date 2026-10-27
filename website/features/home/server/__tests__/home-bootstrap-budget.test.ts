import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { GET } from "@/features/home/server/home-tab-bootstrap-route";
import {
	__setEnvForTest,
	type HomeBootstrapBudget,
	homeBootstrapBudget,
	homeSnapshotDeadlineMs,
} from "@/lib/config/env";
import { __setFlavorForTest, type Flavor } from "@/lib/config/flavor";

vi.mock("server-only", () => ({}));

const mocks = vi.hoisted(() => ({
	fetchMarketplace: vi.fn(),
	fetchProducts: vi.fn(),
	fetchSellers: vi.fn(),
	fetchPublicPostsTyped: vi.fn(),
	fetchCategoryTree: vi.fn(),
	getCachedMarketplace: vi.fn(),
	getCachedProductsFirstPage: vi.fn(),
	getCachedPublicPosts: vi.fn(),
	getCachedRecommendedSellers: vi.fn(),
	getCachedCategoryTree: vi.fn(),
}));

// The aggregates under test are the cache bodies; run them inline so the test
// exercises the route's own budget logic rather than Next's cache machinery.
vi.mock("next/cache", () => ({
	unstable_cache: <T>(fn: T) => fn,
}));

vi.mock("@/lib/api", () => ({
	fetchMarketplace: mocks.fetchMarketplace,
	fetchProducts: mocks.fetchProducts,
	fetchSellers: mocks.fetchSellers,
	fetchCategoryTree: mocks.fetchCategoryTree,
}));

vi.mock("@/lib/api/posts", () => ({
	fetchPublicPostsTyped: mocks.fetchPublicPostsTyped,
}));

vi.mock("@/lib/api/cached", () => ({
	CACHE_TTL: { COMMERCE_LIVE: 300, DISCOVERY: 900 },
	timedFetch: <T>(_label: string, fn: () => Promise<T>) => fn(),
	// Pass-through: coalescing is about how many upstream reads one key opens, and
	// these cases assert budget/degradation for a single read. The real helper has
	// its own coverage in lib/api/cached-single-flight.unit.test.ts.
	coalesceRead: <T>(_key: string[], run: () => Promise<T>) => run(),
	getCachedMarketplace: mocks.getCachedMarketplace,
	getCachedProductsFirstPage: mocks.getCachedProductsFirstPage,
	getCachedPublicPosts: mocks.getCachedPublicPosts,
	getCachedRecommendedSellers: mocks.getCachedRecommendedSellers,
	getCachedCategoryTree: mocks.getCachedCategoryTree,
}));

const MARKETPLACE = { hot_products: [], recommended_sellers: [] };
const PRODUCTS = { data: { records: [{ id: "9001" }] } };
const POSTS = { page: 1, pageSize: 8, total: 1, records: [{ id: "8001" }] };
const SELLERS = [{ store_id: 7001 }];
const CATEGORY_TREE = [
	{ id: "5", name: "海鮮", children: [{ id: "206", name: "魚類" }] },
];
const CATEGORY_GROUPS = [{ id: "5", name: "海鮮", descendantIds: ["206"] }];

/** An upstream that accepted the connection and never answers. */
function stalled(): Promise<never> {
	return new Promise<never>(() => undefined);
}

function primaryResolves(): void {
	mocks.fetchMarketplace.mockResolvedValue(MARKETPLACE);
	mocks.fetchProducts.mockResolvedValue(PRODUCTS);
	mocks.fetchPublicPostsTyped.mockResolvedValue(POSTS);
	mocks.fetchSellers.mockResolvedValue(SELLERS);
	mocks.fetchCategoryTree.mockResolvedValue(CATEGORY_TREE);
}

function cacheResolves(): void {
	mocks.getCachedMarketplace.mockResolvedValue(MARKETPLACE);
	mocks.getCachedProductsFirstPage.mockResolvedValue(PRODUCTS);
	mocks.getCachedPublicPosts.mockResolvedValue(POSTS);
	mocks.getCachedRecommendedSellers.mockResolvedValue(SELLERS);
	mocks.getCachedCategoryTree.mockResolvedValue(CATEGORY_TREE);
}

function allStalled(fns: Array<{ mockImplementation: unknown }>): void {
	for (const fn of fns) {
		(
			fn as { mockImplementation: (impl: () => Promise<never>) => void }
		).mockImplementation(stalled);
	}
}

describe("home bootstrap request budget", () => {
	let budget: HomeBootstrapBudget;

	beforeEach(() => {
		vi.useFakeTimers();
		for (const fn of Object.values(mocks)) fn.mockReset();
		budget = homeBootstrapBudget();
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	it("answers straight from the primary aggregates when they are healthy", async () => {
		primaryResolves();
		const startedAt = Date.now();

		const response = await GET();
		const body = (await response.json()) as Record<string, unknown>;

		expect(response.status).toBe(200);
		expect(Date.now() - startedAt).toBe(0);
		expect(body.schemaVersion).toBe("home.v1");
		// The tree is rolled up server-side: the browser gets the lv1 index only.
		expect(body.categoryGroups).toEqual(CATEGORY_GROUPS);
		expect(body.degraded).toBeUndefined();
		expect(response.headers.get("x-bootstrap-degraded")).toBeNull();
		expect(mocks.getCachedMarketplace).not.toHaveBeenCalled();
		expect(mocks.getCachedProductsFirstPage).not.toHaveBeenCalled();
	});

	it("falls back after the primary deadline and answers inside the shared budget", async () => {
		allStalled([
			mocks.fetchMarketplace,
			mocks.fetchProducts,
			mocks.fetchPublicPostsTyped,
			mocks.fetchSellers,
			mocks.fetchCategoryTree,
		]);
		cacheResolves();
		const startedAt = Date.now();

		const pending = GET();
		await vi.advanceTimersByTimeAsync(budget.totalMs);
		const response = await pending;
		const body = (await response.json()) as Record<string, unknown>;
		const elapsedMs = Date.now() - startedAt;

		expect(response.status).toBe(200);
		// The cache answers immediately, so the whole request costs one budget —
		// never the primary budget plus a second full one.
		expect(elapsedMs).toBe(budget.totalMs);
		expect(elapsedMs).toBeLessThanOrEqual(
			budget.totalMs + budget.fallbackFloorMs,
		);
		expect(body.products).toEqual(PRODUCTS);
		expect(body.degraded).toBeUndefined();
	});

	it("stops at the budget cap instead of stacking a second full wait", async () => {
		allStalled([
			mocks.fetchMarketplace,
			mocks.fetchProducts,
			mocks.fetchPublicPostsTyped,
			mocks.fetchSellers,
			mocks.fetchCategoryTree,
			mocks.getCachedMarketplace,
			mocks.getCachedProductsFirstPage,
			mocks.getCachedPublicPosts,
			mocks.getCachedRecommendedSellers,
			mocks.getCachedCategoryTree,
		]);
		const startedAt = Date.now();

		const pending = GET();
		await vi.advanceTimersByTimeAsync(budget.totalMs);
		await vi.advanceTimersByTimeAsync(budget.fallbackFloorMs);
		const response = await pending;
		const body = (await response.json()) as {
			error: string;
			unavailable: string[];
		};
		const elapsedMs = Date.now() - startedAt;

		expect(elapsedMs).toBe(budget.totalMs + budget.fallbackFloorMs);
		expect(elapsedMs).toBeLessThan(budget.totalMs * 2);
		// No source could supply the commerce wave, so the answer is an outage —
		// not an empty catalogue the caller would paint over real content.
		expect(response.status).toBe(503);
		expect(response.headers.get("retry-after")).toBe("1");
		expect(body.error).toBe("bootstrap_unavailable");
		// Only the commerce surface makes an outage; posts are labelled, not fatal.
		expect(body.unavailable.sort()).toEqual(["marketplace", "products"]);
	});

	it("keeps a posts-only outage a labelled 200 so commerce still refreshes", async () => {
		primaryResolves();
		cacheResolves();
		allStalled([mocks.fetchPublicPostsTyped, mocks.getCachedPublicPosts]);

		const pending = GET();
		await vi.advanceTimersByTimeAsync(budget.totalMs);
		await vi.advanceTimersByTimeAsync(budget.fallbackFloorMs);
		const response = await pending;
		const body = (await response.json()) as Record<string, unknown>;

		// A sick social service must not stop the price/stock wave: that is the
		// whole reason the visitor reloads Home.
		expect(response.status).toBe(200);
		expect(body.products).toEqual(PRODUCTS);
		expect(body.marketplace).toEqual(MARKETPLACE);
		expect(body.degraded).toEqual(["posts"]);
		expect(response.headers.get("x-bootstrap-degraded")).toBe("posts");
	});

	it("labels a discovery-only outage instead of returning silent empty arrays", async () => {
		primaryResolves();
		mocks.getCachedPublicPosts.mockResolvedValue(POSTS);
		mocks.getCachedCategoryTree.mockResolvedValue(CATEGORY_TREE);
		allStalled([mocks.fetchSellers, mocks.getCachedRecommendedSellers]);

		const pending = GET();
		await vi.advanceTimersByTimeAsync(budget.totalMs);
		await vi.advanceTimersByTimeAsync(budget.fallbackFloorMs);
		const response = await pending;
		const body = (await response.json()) as Record<string, unknown>;

		expect(response.status).toBe(200);
		expect(body.products).toEqual(PRODUCTS);
		expect(body.recommendedSellers).toEqual([]);
		// The healthy half of the same wave still answers from cache.
		expect(body.posts).toEqual(POSTS);
		expect(body.categoryGroups).toEqual(CATEGORY_GROUPS);
		expect(body.degraded).toEqual(["recommendedSellers"]);
		expect(response.headers.get("x-bootstrap-degraded")).toBe(
			"recommendedSellers",
		);
	});
});

describe("home bootstrap budget sizing", () => {
	const FLAVORS: Flavor[] = ["dev", "prod"];
	/**
	 * Longest a visitor may wait for a home refresh before the browser gives up.
	 * An independent UX requirement, not a copy of an implementation constant:
	 * growing the prod budget past it must fail here rather than quietly make a
	 * visitor stare longer.
	 */
	const PROD_VISITOR_WAIT_CEILING_MS = 4_500;

	afterEach(() => {
		__setFlavorForTest(null);
		__setEnvForTest(null);
	});

	for (const flavor of FLAVORS) {
		it(`keeps the ${flavor} worst case inside the browser's abandon deadline`, () => {
			__setFlavorForTest(flavor);
			__setEnvForTest(null);
			const budget = homeBootstrapBudget();

			// features/home/HomePage.tsx aborts the refresh at homeSnapshotDeadlineMs();
			// a response produced after that can reach no visitor, so any budget past
			// it is dead time. Imported, never re-typed: a copied literal would keep
			// passing after someone shortened the deadline it is meant to track.
			expect(budget.totalMs + budget.fallbackFloorMs).toBeLessThanOrEqual(
				homeSnapshotDeadlineMs(),
			);
		});
	}

	it("keeps the production wait inside the visitor-facing ceiling", () => {
		__setFlavorForTest("prod");
		__setEnvForTest(null);

		expect(homeSnapshotDeadlineMs()).toBeLessThanOrEqual(
			PROD_VISITOR_WAIT_CEILING_MS,
		);
	});
});
