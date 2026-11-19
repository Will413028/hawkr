/** @vitest-environment jsdom */

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { SellerForumComment } from "@/lib/forum/repo";

const repo = vi.hoisted(() => ({
	fetchSellerForum: vi.fn(),
	fetchSellerForumReplies: vi.fn(),
}));

vi.mock("@/lib/auth/context", () => ({ useAuth: () => ({ identity: null }) }));
vi.mock("@/lib/forum/repo", async (orig) => ({
	...(await orig<typeof import("@/lib/forum/repo")>()),
	...repo,
}));
vi.mock("./useQueryIdentity", () => ({
	useQueryIdentity: () => ({ accountId: null }),
}));

import { useForumReplies, useSellerForum } from "./useSellerForum";

function entry(id: string, over: Partial<SellerForumComment> = {}) {
	return {
		id,
		storeId: "7",
		isMine: false,
		userName: "u",
		content: id,
		createdAt: 0,
		state: "visible" as const,
		viewerCanEdit: false,
		viewerCanDelete: false,
		viewerCanReply: true,
		...over,
	};
}

function page(records: SellerForumComment[], nextPageToken = "") {
	return { records, nextPageToken, pageSize: 20 };
}

let client: QueryClient;
function wrapper({ children }: { children: React.ReactNode }) {
	return React.createElement(QueryClientProvider, { client }, children);
}

beforeEach(() => {
	client = new QueryClient({
		defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } },
	});
	repo.fetchSellerForum.mockReset();
	repo.fetchSellerForumReplies.mockReset();
});
afterEach(cleanup);

describe("forum top-level entries page with a cursor and a refetch keeps the loaded pages", () => {
	function twoPages() {
		repo.fetchSellerForum.mockImplementation(
			async (_store: string, opts: { pageToken?: string }) =>
				opts.pageToken === "t1" ? page([entry("b")]) : page([entry("a")], "t1"),
		);
	}

	it("loadMore sends the next token and appends the page", async () => {
		twoPages();
		const { result } = renderHook(() => useSellerForum("7"), { wrapper });
		await waitFor(() => expect(result.current.hasMore).toBe(true));
		await act(async () => {
			await result.current.loadMore();
		});
		expect(repo.fetchSellerForum).toHaveBeenLastCalledWith(
			"7",
			expect.objectContaining({ pageToken: "t1", viewer: true }),
		);
		await waitFor(() =>
			expect(result.current.comments.map((c) => c.id)).toEqual(["a", "b"]),
		);
		expect(result.current.hasMore).toBe(false);
	});

	it("refresh refetches every loaded page instead of dropping back to the first", async () => {
		twoPages();
		const { result } = renderHook(() => useSellerForum("7"), { wrapper });
		await waitFor(() => expect(result.current.hasMore).toBe(true));
		await act(async () => {
			await result.current.loadMore();
		});
		await waitFor(() => expect(result.current.comments).toHaveLength(2));
		repo.fetchSellerForum.mockClear();
		await act(async () => {
			await result.current.refresh();
		});
		expect(repo.fetchSellerForum.mock.calls.map((c) => c[1].pageToken)).toEqual(
			["", "t1"],
		);
		await waitFor(() =>
			expect(result.current.comments.map((c) => c.id)).toEqual(["a", "b"]),
		);
	});

	it("seeds the first page from the server render without an initial loading state", () => {
		repo.fetchSellerForum.mockReturnValue(new Promise(() => undefined));
		const { result } = renderHook(
			() => useSellerForum("7", page([entry("seed")])),
			{ wrapper },
		);
		expect(result.current.loading).toBe(false);
		expect(result.current.comments.map((c) => c.id)).toEqual(["seed"]);
	});
});

describe("forum replies page with a cursor", () => {
	it("stays idle until enabled, then pages and appends", async () => {
		repo.fetchSellerForumReplies.mockImplementation(
			async (_store: string, _thread: string, opts: { pageToken?: string }) =>
				opts.pageToken === "r2"
					? page([entry("r3", { parentId: "top" })])
					: page([entry("r1", { parentId: "top" })], "r2"),
		);
		const { result, rerender } = renderHook(
			({ enabled }) => useForumReplies("7", "top", "樓主", enabled),
			{ wrapper, initialProps: { enabled: false } },
		);
		expect(repo.fetchSellerForumReplies).not.toHaveBeenCalled();
		rerender({ enabled: true });
		await waitFor(() => expect(result.current.items).toHaveLength(1));
		expect(result.current.hasMore).toBe(true);
		await act(async () => {
			await result.current.loadMore();
		});
		expect(repo.fetchSellerForumReplies).toHaveBeenLastCalledWith(
			"7",
			"top",
			expect.objectContaining({ pageToken: "r2", parentName: "樓主" }),
		);
		await waitFor(() =>
			expect(result.current.items.map((r) => r.id)).toEqual(["r1", "r3"]),
		);
		expect(result.current.hasMore).toBe(false);
	});

	it("records a loadMore failure without rejecting and keeps the loaded replies", async () => {
		repo.fetchSellerForumReplies.mockResolvedValueOnce(
			page([entry("r1", { parentId: "top" })], "r2"),
		);
		const { result } = renderHook(
			() => useForumReplies("7", "top", undefined, true),
			{ wrapper },
		);
		await waitFor(() => expect(result.current.hasMore).toBe(true));
		const err = new Error("boom");
		repo.fetchSellerForumReplies.mockRejectedValueOnce(err);
		await act(async () => {
			await result.current.loadMore();
		});
		await waitFor(() => expect(result.current.loadMoreError).toBe(err));
		expect(result.current.items).toHaveLength(1);
		expect(result.current.hasMore).toBe(true);
	});
});
