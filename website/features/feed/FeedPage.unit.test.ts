import { beforeEach, describe, expect, it, vi } from "vitest";

const cached = vi.hoisted(() => ({
	getCachedPublicPosts: vi.fn(),
	getCachedStorePosts: vi.fn(),
	getCachedStore: vi.fn(),
}));
vi.mock("@/lib/api/cached", () => cached);
vi.mock("@/features/feed/FeedParamController", () => ({
	default: () => null,
}));
vi.mock("@/features/feed/components/InfiniteFeed", () => ({
	default: () => null,
}));

import SellerPostsPage from "@/features/sellers/posts/SellerPostsPage";
import FeedPage from "./FeedPage";

type Element = { props: Record<string, unknown>; type: unknown };

function find(
	node: unknown,
	match: (el: Element) => boolean,
): Element | undefined {
	if (!node || typeof node !== "object") return undefined;
	if (Array.isArray(node)) {
		for (const child of node) {
			const hit = find(child, match);
			if (hit) return hit;
		}
		return undefined;
	}
	const el = node as Element;
	if (el.props && match(el)) return el;
	return el.props ? find(el.props.children, match) : undefined;
}

const PAGE = {
	records: [{ id: "p-1", sellerId: "7", sellerName: "店", logoKey: "" }],
	nextPageToken: "tok",
	pageSize: 12,
};

beforeEach(() => {
	cached.getCachedPublicPosts.mockReset().mockResolvedValue(PAGE);
	cached.getCachedStorePosts.mockReset().mockResolvedValue(PAGE);
	cached.getCachedStore
		.mockReset()
		.mockResolvedValue({ storeId: "7", storeCode: "shop-code" });
});

describe("/feed and the store posts tab seed from tagged anonymous entries", () => {
	it("/feed reads its first page through getCachedPublicPosts and hands on the token", async () => {
		const tree = await FeedPage();
		expect(cached.getCachedPublicPosts).toHaveBeenCalledExactlyOnceWith(12);
		const controller = find(tree, (el) => "initialNextPageToken" in el.props);
		expect(controller?.props.initialNextPageToken).toBe("tok");
		expect(controller?.props.initialSeedFailed).toBe(false);
	});

	it("/feed marks the seed failed (not empty) when the cached read throws", async () => {
		cached.getCachedPublicPosts.mockRejectedValue(new Error("down"));
		const tree = await FeedPage();
		const controller = find(tree, (el) => "initialSeedFailed" in el.props);
		expect(controller?.props.initialSeedFailed).toBe(true);
	});

	it("the store posts tab reads page one through getCachedStorePosts for that store", async () => {
		const tree = await SellerPostsPage({
			params: Promise.resolve({ locale: "zh-TW", id: "shop-code" }),
		});
		expect(cached.getCachedStore).toHaveBeenCalledWith("shop-code");
		expect(cached.getCachedStorePosts).toHaveBeenCalledExactlyOnceWith("7", 12);
		const feed = find(tree, (el) => "initialPostsMatchQuery" in el.props);
		expect(feed?.props.initialNextPageToken).toBe("tok");
		expect(feed?.props.sellerId).toBe("7");
	});

	it("the store posts tab reads nothing when the seller payload has no store id", async () => {
		cached.getCachedStore.mockResolvedValue({ storeCode: "shop-code" });
		await SellerPostsPage({
			params: Promise.resolve({ locale: "zh-TW", id: "shop-code" }),
		});
		expect(cached.getCachedStorePosts).not.toHaveBeenCalled();
	});
});
