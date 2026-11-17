/** @vitest-environment jsdom */

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const repo = vi.hoisted(() => ({
	fetchPostComments: vi.fn(),
	deleteComment: vi.fn(),
	createComment: vi.fn(),
	setCommentLiked: vi.fn(),
	fetchCommentReplies: vi.fn(),
}));

vi.mock("@/lib/auth/context", () => ({ useAuth: () => ({ identity: null }) }));
vi.mock("@/lib/comments/repo", async (orig) => ({
	...(await orig<typeof import("@/lib/comments/repo")>()),
	...repo,
}));
vi.mock("./useQueryIdentity", () => ({
	useQueryIdentity: () => ({ accountId: null }),
}));

import { usePostComments } from "./useSocial";

const comment = {
	id: "c1",
	postId: "p1",
	isMine: true,
	userName: "我",
	content: "舊",
	createdAt: 0,
	etag: "e1",
	state: "visible" as const,
	viewerCanEdit: true,
	viewerCanDelete: true,
	viewerCanReply: true,
};

function wrapper() {
	const client = new QueryClient({
		defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } },
	});
	return ({ children }: { children: React.ReactNode }) =>
		React.createElement(QueryClientProvider, { client }, children);
}

beforeEach(() => {
	for (const fn of Object.values(repo)) fn.mockReset();
	repo.fetchPostComments.mockResolvedValue({
		records: [comment],
		nextPageToken: "",
		pageSize: 20,
	});
	repo.deleteComment.mockResolvedValue(undefined);
});
afterEach(cleanup);

describe("comment edit keeps the cached etag current", () => {
	it("delete after an edit sends the etag the edit returned, not the loaded one", async () => {
		const { result } = renderHook(() => usePostComments("p1"), {
			wrapper: wrapper(),
		});
		await waitFor(() => expect(result.current.comments).toHaveLength(1));
		act(() => result.current.edited({ id: "c1", content: "新", etag: "e2" }));
		await waitFor(() => expect(result.current.comments[0].etag).toBe("e2"));
		expect(result.current.comments[0].content).toBe("新");
		await act(async () => {
			await result.current.remove("c1");
		});
		expect(repo.deleteComment).toHaveBeenCalledWith("c1", "e2");
	});
});
