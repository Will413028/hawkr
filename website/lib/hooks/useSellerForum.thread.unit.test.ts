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

describe("thread page reply seeding", () => {
	it("one GetForumThread read serves the first reply page", async () => {
		repo.fetchSellerForumThread.mockResolvedValue({
			top: page([entry(T, { replyCount: 2 })]),
			replies: page([entry("r1"), entry("r2")], "next"),
		});
		const { result } = renderHook(
			() => {
				const forum = useSellerForum("7", undefined, T);
				return {
					forum,
					// The page expands the thread's replies once its top entry has rendered.
					replies: useForumReplies("7", T, "u", forum.comments.length > 0),
				};
			},
			{ wrapper },
		);
		await waitFor(() =>
			expect(result.current.replies.items.map((e) => e.id)).toEqual([
				"r1",
				"r2",
			]),
		);
		expect(result.current.replies.hasMore).toBe(true);
		expect(repo.fetchSellerForumReplies).not.toHaveBeenCalled();
	});

	it("server-rendered replies seed the reply query without any read", async () => {
		const { result } = renderHook(
			() => ({
				forum: useSellerForum(
					"7",
					page([entry(T)]),
					T,
					page([entry("r1")], "next"),
				),
				replies: useForumReplies("7", T, "u", true),
			}),
			{ wrapper },
		);
		expect(result.current.replies.items.map((e) => e.id)).toEqual(["r1"]);
		expect(repo.fetchSellerForumReplies).not.toHaveBeenCalled();
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
