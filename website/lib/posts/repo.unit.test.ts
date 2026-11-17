import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import {
	PostProductSchema,
	PostSchema,
} from "@hawkr/api/gen/hawkr/social/v1alpha1/social_pb";
import { beforeEach, describe, expect, it, vi } from "vitest";

const publicRpc = vi.hoisted(() => ({
	listPosts: vi.fn(),
	suggestTrendingPosts: vi.fn(),
	suggestTags: vi.fn(),
	suggestTrendingTags: vi.fn(),
}));
const viewerRpc = vi.hoisted(() => ({
	listPosts: vi.fn(),
	getPost: vi.fn(),
	recordPostView: vi.fn(),
}));

vi.mock("../connect/social", () => ({
	socialPublicClient: () => publicRpc,
	socialViewerClient: () => viewerRpc,
	socialAppClient: () => ({}),
}));

import {
	FALLBACK_POST_IMAGE,
	fetchPostsPage,
	fetchTrendingPosts,
	postToDetail,
	postToListItem,
	recordPostView,
} from "./repo";

beforeEach(() => {
	for (const fn of [...Object.values(publicRpc), ...Object.values(viewerRpc)]) {
		fn.mockReset();
	}
});

function wirePost(id: string) {
	return create(PostSchema, {
		id,
		body: "內文",
		store: { id: "7", storeCode: "shop7", name: "店" },
		hashtags: [{ name: "tea", display: "Tea" }],
		likeCount: BigInt(4),
		commentCount: BigInt(2),
		viewCount: BigInt(9),
		shareCount: BigInt(1),
		viewerLiked: true,
		publishTime: timestampFromDate(new Date("2026-11-01T00:00:00Z")),
		etag: "e1",
	});
}

describe("post mapping", () => {
	it("maps counts from int64 and the viewer's like", () => {
		expect(postToListItem(wirePost("p-1"))).toMatchObject({
			id: "p-1",
			sellerId: "7",
			storeCode: "shop7",
			sellerName: "店",
			content: "內文",
			tags: ["Tea"],
			likeCount: 4,
			commentCount: 2,
			viewCount: 9,
			shareCount: 1,
			isLiked: true,
			createdAt: Date.parse("2026-11-01T00:00:00Z") / 1000,
			imageKeys: [FALLBACK_POST_IMAGE],
		});
	});

	it("the card and the detail show the server's product summary", () => {
		const post = wirePost("p-1");
		post.productId = "88";
		post.product = create(PostProductSchema, {
			id: "88",
			name: "烏龍茶",
			minPrice: BigInt(1200),
			maxPrice: BigInt(1500),
			available: true,
			image: { uriTemplate: "https://media.example.test/p/88/{preset}.jpg" },
		});
		const product = {
			id: "88",
			name: "烏龍茶",
			price: "NT$ 1,200 – 1,500",
			imageKey: "https://media.example.test/p/88/thumb.jpg",
		};
		expect(postToListItem(post).product).toEqual(product);
		expect(postToDetail(post)).toMatchObject({
			products: [product],
			productId: "88",
		});
	});

	it("an unavailable product (summary unset) leaves the card without a product but keeps the id", () => {
		const post = wirePost("p-1");
		post.productId = "88";
		expect(postToListItem(post).product).toBeNull();
		expect(postToDetail(post)).toMatchObject({
			products: [],
			productId: "88",
		});
	});
});

describe("feed pages (page_token, no totals)", () => {
	it("anonymous reads use the public client and pass the token through", async () => {
		publicRpc.listPosts.mockResolvedValue({
			posts: [wirePost("p-1")],
			nextPageToken: "next",
		});
		const page = await fetchPostsPage({ pageToken: "cur", pageSize: 5 });
		expect(publicRpc.listPosts).toHaveBeenCalledWith(
			{ pageSize: 5, pageToken: "cur" },
			{ signal: undefined },
		);
		expect(viewerRpc.listPosts).not.toHaveBeenCalled();
		expect(page.nextPageToken).toBe("next");
		expect(page.records.map((r) => r.id)).toEqual(["p-1"]);
		expect("total" in page).toBe(false);
	});

	it("viewer reads use the cookie-carrying client, never the shared one", async () => {
		viewerRpc.listPosts.mockResolvedValue({ posts: [], nextPageToken: "" });
		await fetchPostsPage({ viewer: true });
		expect(viewerRpc.listPosts).toHaveBeenCalledOnce();
		expect(publicRpc.listPosts).not.toHaveBeenCalled();
	});

	it("filters by store or by tag, and refuses both together", async () => {
		publicRpc.listPosts.mockResolvedValue({ posts: [], nextPageToken: "" });
		await fetchPostsPage({ storeId: "7" });
		await fetchPostsPage({ tag: "#Tea" });
		expect(publicRpc.listPosts.mock.calls.map(([req]) => req)).toEqual([
			{ pageSize: 12, pageToken: "", storeId: "7" },
			{ pageSize: 12, pageToken: "", hashtag: "Tea" },
		]);
		await expect(
			fetchPostsPage({ storeId: "7", tag: "Tea" }),
		).rejects.toThrow();
	});

	it("strict reads throw; the default degrades to an empty last page", async () => {
		publicRpc.listPosts.mockRejectedValue(new Error("down"));
		await expect(fetchPostsPage({ strict: true })).rejects.toThrow("down");
		expect(await fetchPostsPage()).toEqual({
			records: [],
			nextPageToken: "",
			pageSize: 12,
		});
	});
});

describe("single post page and weekly topic", () => {
	it("records a view for the opened post", async () => {
		viewerRpc.recordPostView.mockResolvedValue({});
		await recordPostView("p-1");
		expect(viewerRpc.recordPostView).toHaveBeenCalledExactlyOnceWith({
			postId: "p-1",
		});
	});

	it("a failed view beacon never breaks the page", async () => {
		viewerRpc.recordPostView.mockRejectedValue(new Error("429"));
		await expect(recordPostView("p-1")).resolves.toBeUndefined();
	});

	it("the weekly topic is the server's ranking, not a client re-sort", async () => {
		publicRpc.suggestTrendingPosts.mockResolvedValue({
			posts: [wirePost("low"), wirePost("high")],
		});
		const posts = await fetchTrendingPosts(5);
		expect(publicRpc.suggestTrendingPosts).toHaveBeenCalledWith(
			{ maxResults: 5 },
			{ signal: undefined },
		);
		expect(posts.map((p) => p.id)).toEqual(["low", "high"]);
	});
});
