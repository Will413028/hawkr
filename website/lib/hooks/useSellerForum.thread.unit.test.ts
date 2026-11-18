/** @vitest-environment jsdom */

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { SellerForumComment } from "@/lib/forum/repo";
import { queryKeys } from "@/lib/query/keys";

const repo = vi.hoisted(() => ({
	fetchSellerForum: vi.fn(),
	fetchSellerForumReplies: vi.fn(),
	fetchSellerForumThread: vi.fn(),
	createSellerForumComment: vi.fn(),
	replySellerForumComment: vi.fn(),
	deleteSellerForumComment: vi.fn(),
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

const T = "0a1b2c3d-0000-4000-8000-00000000abcd";
const ID = { accountId: null, activeRole: undefined } as never;

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
		defaultOptions: {
			queries: { retry: false, refetchOnWindowFocus: false, staleTime: 30_000 },
		},
	});
	for (const f of Object.values(repo)) f.mockReset();
});
afterEach(cleanup);

function deferred<T>() {
	let resolve!: (v: T) => void;
	const promise = new Promise<T>((r) => {
		resolve = r;
	});
	return { promise, resolve };
}

describe("thread page SSR data is anonymous and is replaced by the viewer read", () => {
	it("shows the seeded replies at once, then the viewer read replaces them", async () => {
		repo.fetchSellerForumReplies.mockResolvedValue(
			page([entry("r1", { viewerCanEdit: true })]),
		);
		const { result } = renderHook(
			() => useForumReplies("7", T, "u", true, page([entry("r1")], "next")),
			{ wrapper },
		);
		expect(result.current.items.map((e) => e.viewerCanEdit)).toEqual([false]);
		await waitFor(() =>
			expect(result.current.items.map((e) => e.viewerCanEdit)).toEqual([true]),
		);
		expect(repo.fetchSellerForumReplies).toHaveBeenCalledTimes(1);
	});

	it("re-reads the seeded thread top with the viewer client on mount", async () => {
		repo.fetchSellerForumThread.mockResolvedValue({
			top: page([entry(T, { viewerCanReply: true })]),
			replies: page([]),
		});
		const { result } = renderHook(
			() => useSellerForum("7", page([entry(T, { viewerCanReply: false })]), T),
			{ wrapper },
		);
		expect(result.current.comments[0]?.viewerCanReply).toBe(false);
		await waitFor(() =>
			expect(result.current.comments[0]?.viewerCanReply).toBe(true),
		);
		expect(repo.fetchSellerForumThread).toHaveBeenCalledTimes(1);
		expect(repo.fetchSellerForumThread.mock.calls[0]?.[2]).toMatchObject({
			viewer: true,
		});
	});

	it("a thread-top refetch landing after a reply delete cannot bring the reply back", async () => {
		repo.fetchSellerForumReplies.mockResolvedValueOnce(
			page([entry("r1", { parentId: T }), entry("r2", { parentId: T })]),
		);
		const staleTop = deferred<unknown>();
		repo.fetchSellerForumThread.mockReturnValueOnce(staleTop.promise);
		repo.deleteSellerForumComment.mockResolvedValue(undefined);
		const { result } = renderHook(
			() => {
				const forum = useSellerForum(
					"7",
					page([entry(T, { replyCount: 2 })]),
					T,
				);
				return {
					forum,
					replies: useForumReplies(
						"7",
						T,
						"u",
						true,
						page([entry("r1", { parentId: T }), entry("r2", { parentId: T })]),
					),
				};
			},
			{ wrapper },
		);
		await waitFor(() =>
			expect(repo.fetchSellerForumReplies).toHaveBeenCalledTimes(1),
		);
		repo.fetchSellerForumReplies.mockResolvedValue(
			page([entry("r2", { parentId: T })]),
		);
		await act(async () => {
			await result.current.forum.remove(
				result.current.replies.items.find((e) => e.id === "r1") as never,
			);
		});
		await act(async () => {
			staleTop.resolve({
				top: page([entry(T, { replyCount: 2 })]),
				replies: page([
					entry("r1", { parentId: T }),
					entry("r2", { parentId: T }),
				]),
			});
			await staleTop.promise;
		});
		await waitFor(() =>
			expect(result.current.replies.items.map((e) => e.id)).toEqual(["r2"]),
		);
	});
});

describe("thread and list views invalidate each other", () => {
	it("a mutation on the thread page marks the store's list queries stale", async () => {
		repo.fetchSellerForumThread.mockResolvedValue({
			top: page([entry(T)]),
			replies: page([]),
		});
		repo.replySellerForumComment.mockResolvedValue(entry("new"));
		const listKey = queryKeys.seller.forum(ID, "7", { pageSize: 20 });
		client.setQueryData(listKey, {
			pages: [page([entry("a")])],
			pageParams: [""],
		});
		const { result } = renderHook(() => useSellerForum("7", undefined, T), {
			wrapper,
		});
		await waitFor(() => expect(result.current.comments).toHaveLength(1));
		await act(async () => {
			await result.current.reply(result.current.comments[0], "hi");
		});
		await waitFor(() =>
			expect(client.getQueryState(listKey)?.isInvalidated).toBe(true),
		);
	});

	it("a mutation on the list page marks the store's thread queries stale", async () => {
		repo.fetchSellerForum.mockResolvedValue(page([entry("a")]));
		repo.createSellerForumComment.mockResolvedValue(entry("new"));
		const threadKey = queryKeys.seller.forumThread(ID, "7", T);
		client.setQueryData(threadKey, {
			pages: [page([entry(T)])],
			pageParams: [""],
		});
		const { result } = renderHook(() => useSellerForum("7"), { wrapper });
		await waitFor(() => expect(result.current.comments).toHaveLength(1));
		await act(async () => {
			await result.current.post("hello");
		});
		await waitFor(() =>
			expect(client.getQueryState(threadKey)?.isInvalidated).toBe(true),
		);
	});
});
