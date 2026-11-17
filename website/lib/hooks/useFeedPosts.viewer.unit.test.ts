/** @vitest-environment jsdom */

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, renderHook, waitFor } from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { PostListItem } from "@/lib/api/openapi-types";

const auth = vi.hoisted(() => ({ status: "guest" as string }));
const repo = vi.hoisted(() => ({ fetchPostsPage: vi.fn() }));
const canonical = vi.hoisted(() => ({
	fetchCanonicalFeedPage: vi.fn(),
	fetchCanonicalMarketRecords: vi.fn(),
}));

vi.mock("@/lib/auth/context", () => ({
	useAuth: () => ({ status: auth.status }),
}));
vi.mock("@/lib/posts/repo", () => repo);
vi.mock("@/lib/api/main-tab-pages", () => canonical);
vi.mock("./useQueryIdentity", () => ({
	useQueryIdentity: () => ({ accountId: null }),
}));

import { useFeedPosts } from "./useFeedPosts";

function post(id: string): PostListItem {
	return {
		id,
		sellerId: "7",
		storeCode: "shop7",
		sellerName: "店",
		logoKey: "",
		content: id,
		imageKeys: [],
		product: null,
		commentCount: 0,
		likeCount: 0,
		shareCount: 0,
		viewCount: 0,
		isLiked: false,
		createdAt: 0,
		tags: [],
	};
}

function wrapper() {
	const client = new QueryClient({
		defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } },
	});
	return ({ children }: { children: React.ReactNode }) =>
		React.createElement(QueryClientProvider, { client }, children);
}

beforeEach(() => {
	auth.status = "guest";
	repo.fetchPostsPage.mockReset();
	canonical.fetchCanonicalFeedPage.mockReset();
});
afterEach(cleanup);

describe("feed reads depend on the viewer", () => {
	it("a guest keeps the anonymous server seed and does not refetch it", async () => {
		const { result } = renderHook(
			() =>
				useFeedPosts({
					initialPosts: [post("seed")],
					initialNextPageToken: "t2",
					pageSize: 12,
				}),
			{ wrapper: wrapper() },
		);
		expect(result.current.posts.map((p) => p.id)).toEqual(["seed"]);
		expect(result.current.initialLoading).toBe(false);
		expect(repo.fetchPostsPage).not.toHaveBeenCalled();
	});

	it("a signed-in viewer drops the anonymous seed and reads with their own credentials", async () => {
		auth.status = "signedIn";
		repo.fetchPostsPage.mockResolvedValue({
			records: [post("mine")],
			nextPageToken: "",
			pageSize: 12,
		});
		const { result } = renderHook(
			() =>
				useFeedPosts({
					initialPosts: [post("anonymous-seed-with-blocked-store")],
					initialNextPageToken: "t2",
					pageSize: 12,
				}),
			{ wrapper: wrapper() },
		);
		// The shared anonymous seed is not trusted for a signed-in viewer.
		expect(result.current.posts).toEqual([]);
		await waitFor(() =>
			expect(result.current.posts.map((p) => p.id)).toEqual(["mine"]),
		);
		expect(repo.fetchPostsPage).toHaveBeenCalledWith(
			expect.objectContaining({ viewer: true, pageToken: "" }),
		);
		// The anonymous same-origin canonical route is bypassed.
		expect(canonical.fetchCanonicalFeedPage).not.toHaveBeenCalled();
	});

	it("a guest next page continues from the seed's page_token", async () => {
		canonical.fetchCanonicalFeedPage.mockResolvedValue({
			records: [post("page2")],
			nextPageToken: "",
			pageSize: 12,
		});
		const { result } = renderHook(
			() =>
				useFeedPosts({
					initialPosts: [post("seed")],
					initialNextPageToken: "t2",
					pageSize: 12,
				}),
			{ wrapper: wrapper() },
		);
		await result.current.loadMore();
		await waitFor(() =>
			expect(result.current.posts.map((p) => p.id)).toEqual(["seed", "page2"]),
		);
		expect(canonical.fetchCanonicalFeedPage).toHaveBeenCalledWith(
			"t2",
			expect.anything(),
		);
		expect(result.current.done).toBe(true);
	});

	it("a refetch of a seeded list replays page one from the empty token, not the seed's next token", async () => {
		canonical.fetchCanonicalFeedPage.mockResolvedValue({
			records: [post("fresh1")],
			nextPageToken: "t2",
			pageSize: 12,
		});
		const client = new QueryClient({
			defaultOptions: { queries: { retry: false } },
		});
		const { result } = renderHook(
			() =>
				useFeedPosts({
					initialPosts: [post("seed")],
					initialNextPageToken: "t2",
					pageSize: 12,
				}),
			{
				wrapper: ({ children }: { children: React.ReactNode }) =>
					React.createElement(QueryClientProvider, { client }, children),
			},
		);
		expect(result.current.posts.map((p) => p.id)).toEqual(["seed"]);
		await client.refetchQueries();
		await waitFor(() =>
			expect(result.current.posts.map((p) => p.id)).toEqual(["fresh1"]),
		);
		expect(canonical.fetchCanonicalFeedPage).toHaveBeenCalledWith(
			"",
			expect.anything(),
		);
		expect(canonical.fetchCanonicalFeedPage).not.toHaveBeenCalledWith(
			"t2",
			expect.anything(),
		);
	});
});
