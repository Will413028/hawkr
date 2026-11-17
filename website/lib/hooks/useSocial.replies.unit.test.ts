/** @vitest-environment jsdom */

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const repo = vi.hoisted(() => ({ fetchCommentReplies: vi.fn() }));

vi.mock("@/lib/auth/context", () => ({ useAuth: () => ({ identity: null }) }));
vi.mock("@/lib/comments/repo", async (orig) => ({
	...(await orig<typeof import("@/lib/comments/repo")>()),
	...repo,
}));
vi.mock("./useQueryIdentity", () => ({
	useQueryIdentity: () => ({ accountId: null }),
}));

import { useCommentReplies } from "./useSocial";

function reply(id: string) {
	return {
		id,
		postId: "p",
		isMine: false,
		userName: "u",
		content: id,
		createdAt: 0,
		state: "visible" as const,
		viewerCanEdit: false,
		viewerCanDelete: false,
		viewerCanReply: true,
	};
}

function wrapper() {
	const client = new QueryClient({
		defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } },
	});
	return ({ children }: { children: React.ReactNode }) =>
		React.createElement(QueryClientProvider, { client }, children);
}

beforeEach(() => repo.fetchCommentReplies.mockReset());
afterEach(cleanup);

describe("comment replies paging", () => {
	it("offers more only with a next token, fetches with it and appends", async () => {
		repo.fetchCommentReplies
			.mockResolvedValueOnce({
				records: [reply("r1")],
				nextPageToken: "tok",
				pageSize: 20,
			})
			.mockResolvedValueOnce({
				records: [reply("r2")],
				nextPageToken: "",
				pageSize: 20,
			});
		const { result } = renderHook(() => useCommentReplies("p", "c"), {
			wrapper: wrapper(),
		});
		await act(async () => {
			await result.current.expand();
		});
		await waitFor(() => expect(result.current.items).toHaveLength(1));
		expect(result.current.hasMore).toBe(true);
		await act(async () => {
			await result.current.loadMore();
		});
		expect(repo.fetchCommentReplies).toHaveBeenLastCalledWith(
			"p",
			"c",
			expect.objectContaining({ pageToken: "tok" }),
		);
		expect(result.current.items.map((r) => r.id)).toEqual(["r1", "r2"]);
		expect(result.current.hasMore).toBe(false);
	});

	it("does not fetch more when there is no next token", async () => {
		repo.fetchCommentReplies.mockResolvedValue({
			records: [reply("r1")],
			nextPageToken: "",
			pageSize: 20,
		});
		const { result } = renderHook(() => useCommentReplies("p", "c"), {
			wrapper: wrapper(),
		});
		await act(async () => {
			await result.current.expand();
		});
		await waitFor(() => expect(result.current.items).toHaveLength(1));
		expect(result.current.hasMore).toBe(false);
		await act(async () => {
			await result.current.loadMore();
		});
		expect(repo.fetchCommentReplies).toHaveBeenCalledTimes(1);
	});
});
