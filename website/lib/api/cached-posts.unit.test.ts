import { beforeEach, describe, expect, it, vi } from "vitest";

const cache = vi.hoisted(() => ({
	cacheTag: vi.fn(),
	cacheLife: vi.fn(),
}));
const posts = vi.hoisted(() => ({
	fetchPostsPage: vi.fn(),
	fetchTrendingPosts: vi.fn(),
	fetchTrendingPostTags: vi.fn(),
}));

vi.mock("next/cache", () => cache);
vi.mock("@sentry/nextjs", () => ({
	startSpan: (_options: unknown, fn: () => unknown) => fn(),
}));
vi.mock("@/lib/posts/repo", () => posts);

import { PUBLIC_CACHE_TAGS, sellerTag } from "@/lib/seo/tags";
import {
	getCachedPublicPosts,
	getCachedStorePosts,
	getCachedTrendingPosts,
} from "./cached";

const PAGE = { records: [], nextPageToken: "t", pageSize: 12 };

beforeEach(() => {
	cache.cacheTag.mockReset();
	cache.cacheLife.mockReset();
	posts.fetchPostsPage.mockReset().mockResolvedValue(PAGE);
	posts.fetchTrendingPosts.mockReset().mockResolvedValue([]);
});

describe("cached post readers are anonymous and tagged", () => {
	it("the public feed first page carries posts:list and reads without a viewer", async () => {
		await getCachedPublicPosts(12);
		expect(cache.cacheTag).toHaveBeenCalledWith(PUBLIC_CACHE_TAGS.posts);
		const [options] = posts.fetchPostsPage.mock.calls[0] as [
			Record<string, unknown>,
		];
		expect(options).toMatchObject({
			pageSize: 12,
			strict: true,
			locale: "zh-TW",
		});
		expect(options.viewer).toBeUndefined();
		expect(options.signal).toBeInstanceOf(AbortSignal);
	});

	it("a store's first page carries posts:list and seller:<id>", async () => {
		await getCachedStorePosts("42", 12);
		expect(cache.cacheTag).toHaveBeenCalledWith(
			PUBLIC_CACHE_TAGS.posts,
			sellerTag("42"),
		);
		const [options] = posts.fetchPostsPage.mock.calls[0] as [
			Record<string, unknown>,
		];
		expect(options).toMatchObject({
			storeId: "42",
			strict: true,
			locale: "zh-TW",
		});
		expect(options.viewer).toBeUndefined();
	});

	it("the weekly topic is tagged posts:list and read strictly", async () => {
		await getCachedTrendingPosts(5);
		expect(cache.cacheTag).toHaveBeenCalledWith(PUBLIC_CACHE_TAGS.posts);
		expect(posts.fetchTrendingPosts).toHaveBeenCalledWith(
			5,
			expect.objectContaining({ strict: true, locale: "zh-TW" }),
		);
	});
});
