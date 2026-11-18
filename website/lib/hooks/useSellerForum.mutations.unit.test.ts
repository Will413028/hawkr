/** @vitest-environment jsdom */

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { SellerForumComment } from "@/lib/forum/repo";

const repo = vi.hoisted(() => ({
	fetchSellerForum: vi.fn(),
	fetchSellerForumReplies: vi.fn(),
	updateSellerForumComment: vi.fn(),
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
		viewerCanEdit: true,
		viewerCanDelete: true,
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

const settle = () => act(async () => new Promise((r) => setTimeout(r, 20)));

async function loaded(topOver: Partial<SellerForumComment> = {}) {
	repo.fetchSellerForum.mockResolvedValue({
		records: [
			entry("top", { replyCount: 2, ...topOver }),
			entry("lonely", { replyCount: 0 }),
		],
		nextPageToken: "",
		pageSize: 20,
	});
	repo.fetchSellerForumReplies.mockResolvedValue({
		records: [
			entry("r1", { parentId: "top", rootId: "top", replyToName: "@top-user" }),
			entry("r2", { parentId: "top", rootId: "top" }),
		],
		nextPageToken: "more",
		pageSize: 20,
	});
	const hook = renderHook(() => useSellerForum("7"), { wrapper: wrapper() });
	await waitFor(() => expect(hook.result.current.comments).toHaveLength(2));
	await act(async () => {
		await hook.result.current.loadReplies("top");
	});
	await waitFor(() =>
		expect(hook.result.current.comments[0].replies).toHaveLength(2),
	);
	return hook;
}

beforeEach(() => {
	for (const fn of Object.values(repo)) fn.mockReset();
});
afterEach(cleanup);

describe("update keeps the client-only fields", () => {
	it("a top-level edit keeps loaded replies and their page token", async () => {
		const { result } = await loaded();
		repo.updateSellerForumComment.mockResolvedValue(
			entry("top", { content: "edited", replyCount: 2, etag: "e2" }),
		);
		await act(async () => {
			await result.current.update("top", "edited");
		});
		await settle();
		const top = result.current.comments[0];
		expect(top.content).toBe("edited");
		expect(top.etag).toBe("e2");
		expect(top.replies?.map((r) => r.id)).toEqual(["r1", "r2"]);
		expect(top.repliesNextPageToken).toBe("more");
	});

	it("a reply edit keeps rootId and replyToName", async () => {
		const { result } = await loaded();
		repo.updateSellerForumComment.mockResolvedValue(
			entry("r1", { content: "edited", parentId: "top" }),
		);
		await act(async () => {
			await result.current.update("r1", "edited");
		});
		await settle();
		const r1 = result.current.comments[0].replies?.[0];
		expect(r1?.content).toBe("edited");
		expect(r1?.rootId).toBe("top");
		expect(r1?.replyToName).toBe("@top-user");
	});
});

describe("remove follows the ListForumEntries contract", () => {
	it("deleting a reply lowers the parent's replyCount and drops only that reply", async () => {
		const { result } = await loaded();
		repo.deleteSellerForumComment.mockResolvedValue(undefined);
		await act(async () => {
			await result.current.remove("r1");
		});
		await settle();
		expect(result.current.comments[0].replyCount).toBe(1);
		expect(result.current.comments[0].replies?.map((r) => r.id)).toEqual([
			"r2",
		]);
	});

	it("deleting a top-level entry that has replies leaves a deleted placeholder with its replies", async () => {
		const { result } = await loaded();
		repo.deleteSellerForumComment.mockResolvedValue(undefined);
		await act(async () => {
			await result.current.remove("top");
		});
		await settle();
		const top = result.current.comments[0];
		expect(top.id).toBe("top");
		expect(top.state).toBe("deleted");
		expect(top.content).toBe("");
		expect(top.viewerCanEdit).toBe(false);
		expect(top.viewerCanDelete).toBe(false);
		expect(top.replyCount).toBe(2);
		expect(top.replies?.map((r) => r.id)).toEqual(["r1", "r2"]);
	});

	it("deleting a top-level entry without replies removes it", async () => {
		const { result } = await loaded();
		repo.deleteSellerForumComment.mockResolvedValue(undefined);
		await act(async () => {
			await result.current.remove("lonely");
		});
		await settle();
		expect(result.current.comments.map((c) => c.id)).toEqual(["top"]);
	});

	it("a failed delete restores the list", async () => {
		const { result } = await loaded();
		repo.deleteSellerForumComment.mockRejectedValue(new Error("boom"));
		await act(async () => {
			await result.current.remove("top").catch(() => undefined);
		});
		await settle();
		expect(result.current.comments[0].state).toBe("visible");
		expect(result.current.comments[0].replies).toHaveLength(2);
	});
});
