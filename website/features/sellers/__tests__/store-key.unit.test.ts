import { beforeEach, describe, expect, it, vi } from "vitest";

/**
 * 網址段是 store_code，商品、評價、貼文的讀取要用 `GetStore` 回的 `store.id`。
 * 以前把網址段直接當 id 傳給 `/public/sellers/{id}`，這些測試在有人改回去時會失敗。
 */

const STORE_ID = "7487768729113814280";
const STORE_CODE = "2662192214";

const cached = vi.hoisted(() => ({
	getCachedStore: vi.fn(),
	getCachedSellerReviews: vi.fn(),
	getCachedSellerReviewStats: vi.fn(),
	getCachedStorePosts: vi.fn(),
	getCachedSellerStorefrontProducts: vi.fn(),
	getCachedSellerProducts: vi.fn(),
}));
vi.mock("@/lib/api/cached", () => cached);
vi.mock("@/features/feed/components/InfiniteFeed", () => ({
	default: () => null,
}));
vi.mock("@/features/sellers/reviews/ReviewsClient", () => ({
	default: () => null,
}));
vi.mock("@/features/sellers/storefront/SellerProductsClient", () => ({
	default: () => null,
}));

const store = { storeId: STORE_ID, storeCode: STORE_CODE, name: "店" };
const params = Promise.resolve({ locale: "zh-TW", id: STORE_CODE });

type Element = { props: Record<string, unknown> };
function findProp(node: unknown, prop: string): Element | undefined {
	if (!node || typeof node !== "object") return undefined;
	const el = node as Element;
	if (el.props && prop in el.props) return el;
	if (!el.props) return undefined;
	const kids = el.props.children;
	for (const child of Array.isArray(kids) ? kids : [kids]) {
		const hit = findProp(child, prop);
		if (hit) return hit;
	}
	return undefined;
}

beforeEach(() => {
	for (const fn of Object.values(cached)) fn.mockReset();
	cached.getCachedStore.mockResolvedValue(store);
	cached.getCachedSellerReviews.mockResolvedValue({
		reviews: [],
		nextPageToken: "",
	});
	cached.getCachedSellerReviewStats.mockResolvedValue(null);
	cached.getCachedStorePosts.mockResolvedValue({
		records: [],
		nextPageToken: "",
		pageSize: 12,
	});
	cached.getCachedSellerStorefrontProducts.mockResolvedValue({
		records: [{ id: "1" }],
		total: 1,
		totalPages: 1,
	});
});

describe("public storefront reads use the GetStore store id, never the URL store code", () => {
	it("reviews tab", async () => {
		const { default: Page } = await import(
			"@/features/sellers/reviews/SellerReviewsPage"
		);
		const tree = await Page({ params });
		expect(cached.getCachedStore).toHaveBeenCalledWith(STORE_CODE);
		expect(cached.getCachedSellerReviews).toHaveBeenCalledWith(STORE_ID);
		expect(cached.getCachedSellerReviewStats).toHaveBeenCalledWith(STORE_ID);
		expect(cached.getCachedSellerReviews).not.toHaveBeenCalledWith(STORE_CODE);
		expect(findProp(tree, "initialReviews")?.props.sellerId).toBe(STORE_ID);
	});

	it("posts tab", async () => {
		cached.getCachedStorePosts.mockResolvedValue({
			records: [{ id: "p" }],
			nextPageToken: "",
			pageSize: 12,
		});
		const { default: Page } = await import(
			"@/features/sellers/posts/SellerPostsPage"
		);
		const tree = await Page({ params });
		expect(cached.getCachedStorePosts).toHaveBeenCalledWith(
			STORE_ID,
			expect.anything(),
		);
		expect(findProp(tree, "initialPosts")?.props.sellerId).toBe(STORE_ID);
	});

	it("storefront products", async () => {
		const { default: Page } = await import(
			"@/app/[locale]/(public)/sellers/[id]/(storefront)/page"
		);
		const tree = await Page({ params });
		const props = findProp(tree, "sellerId")?.props;
		expect(props?.sellerId).toBe(STORE_ID);

		const { default: Storefront } = await import(
			"@/features/sellers/storefront/StorefrontPage"
		);
		await Storefront({ sellerId: props?.sellerId as string });
		expect(cached.getCachedSellerStorefrontProducts).toHaveBeenCalledWith(
			STORE_ID,
			1,
			12,
		);
	});

	it("a missing store is 404 for the reviews tab and reads nothing by id", async () => {
		cached.getCachedStore.mockResolvedValue(null);
		const { default: Page } = await import(
			"@/features/sellers/reviews/SellerReviewsPage"
		);
		await expect(Page({ params })).rejects.toThrow();
		expect(cached.getCachedSellerReviews).not.toHaveBeenCalled();
	});
});
