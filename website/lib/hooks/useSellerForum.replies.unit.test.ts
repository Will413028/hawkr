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

import { useSellerForum } from "./useSellerForum";

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

function wrapper() {
	const client = new QueryClient({
		defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } },
	});
	return ({ children }: { children: React.ReactNode }) =>
		React.createElement(QueryClientProvider, { client }, children);
}

beforeEach(() => {
	repo.fetchSellerForum.mockReset().mockResolvedValue({
		records: [entry("top", { replyCount: 45 })],
		nextPageToken: "",
		pageSize: 20,
	});
	repo.fetchSellerForumReplies.mockReset().mockResolvedValue({
		records: [entry("r1"), entry("r2")],
		nextPageToken: "more",
		pageSize: 20,
	});
});
afterEach(cleanup);

describe("loading replies keeps the server's reply count", () => {
	it("a first page shorter than the total does not shrink replyCount", async () => {
		const { result } = renderHook(() => useSellerForum("7"), {
			wrapper: wrapper(),
		});
		await waitFor(() => expect(result.current.comments).toHaveLength(1));
		await act(async () => {
			await result.current.loadReplies("top");
		});
		expect(repo.fetchSellerForumReplies).toHaveBeenCalledWith(
			"7",
			"top",
			expect.anything(),
		);
		await waitFor(() =>
			expect(result.current.comments[0].replies).toHaveLength(2),
		);
		expect(result.current.comments[0].replyCount).toBe(45);
	});
});

describe("forum replies paging", () => {
	it("loads the next reply page with the token and appends it", async () => {
		const { result } = renderHook(() => useSellerForum("7"), {
			wrapper: wrapper(),
		});
		await waitFor(() => expect(result.current.comments).toHaveLength(1));
		await act(async () => {
			await result.current.loadReplies("top");
		});
		await waitFor(() =>
			expect(result.current.comments[0].repliesNextPageToken).toBe("more"),
		);
		repo.fetchSellerForumReplies.mockResolvedValueOnce({
			records: [entry("r3")],
			nextPageToken: "",
			pageSize: 20,
		});
		await act(async () => {
			await result.current.loadMoreReplies("top");
		});
		expect(repo.fetchSellerForumReplies).toHaveBeenLastCalledWith(
			"7",
			"top",
			expect.objectContaining({ pageToken: "more" }),
		);
		await waitFor(() =>
			expect(result.current.comments[0].replies).toHaveLength(3),
		);
		const top = result.current.comments[0];
		expect(top.replies?.map((r) => r.id)).toEqual(["r1", "r2", "r3"]);
		expect(top.repliesNextPageToken).toBe("");
		await act(async () => {
			await result.current.loadMoreReplies("top");
		});
		expect(repo.fetchSellerForumReplies).toHaveBeenCalledTimes(2);
	});
});
