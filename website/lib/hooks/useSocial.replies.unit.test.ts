/** @vitest-environment jsdom */

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const repo = vi.hoisted(() => ({
	fetchCommentReplies: vi.fn(),
	fetchPostComments: vi.fn(),
	createComment: vi.fn(),
}));

vi.mock("@/lib/auth/context", () => ({
	useAuth: () => ({ identity: { nickname: "我", avatarKey: "me.png" } }),
}));
vi.mock("@/lib/comments/repo", async (orig) => ({
	...(await orig<typeof import("@/lib/comments/repo")>()),
	...repo,
}));
vi.mock("./useQueryIdentity", () => ({
	useQueryIdentity: () => ({ accountId: null }),
}));

import type { Comment } from "@/lib/comments/repo";
import { useCommentReplies, usePostComments } from "./useSocial";

function reply(id: string, over: Partial<Comment> = {}): Comment {
	return {
		id,
		postId: "p",
		isMine: false,
		userName: "u",
		content: id,
		createdAt: 0,
		state: "visible",
		viewerCanEdit: false,
		viewerCanDelete: false,
		viewerCanReply: true,
		...over,
	};
}

function deferred<T>() {
	let resolve!: (value: T) => void;
	let reject!: (error: unknown) => void;
	const promise = new Promise<T>((res, rej) => {
		resolve = res;
		reject = rej;
	});
	return { promise, resolve, reject };
}

let client: QueryClient;
function wrapper({ children }: { children: React.ReactNode }) {
	return React.createElement(QueryClientProvider, { client }, children);
}

beforeEach(() => {
	client = new QueryClient({
		defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } },
	});
	for (const fn of Object.values(repo)) fn.mockReset();
	vi.spyOn(crypto, "randomUUID").mockReturnValue(
		"22222222-2222-4222-8222-222222222222",
	);
});
afterEach(() => {
	cleanup();
	vi.restoreAllMocks();
});

const REQUEST_KEY = "tmp-22222222-2222-4222-8222-222222222222";

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
			wrapper,
		});
		act(() => result.current.expand());
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
		await waitFor(() =>
			expect(result.current.items.map((r) => r.id)).toEqual(["r1", "r2"]),
		);
		expect(result.current.hasMore).toBe(false);
	});

	it("does not fetch more when there is no next token", async () => {
		repo.fetchCommentReplies.mockResolvedValue({
			records: [reply("r1")],
			nextPageToken: "",
			pageSize: 20,
		});
		const { result } = renderHook(() => useCommentReplies("p", "c"), {
			wrapper,
		});
		act(() => result.current.expand());
		await waitFor(() => expect(result.current.items).toHaveLength(1));
		expect(result.current.hasMore).toBe(false);
		await act(async () => {
			await result.current.loadMore();
		});
		expect(repo.fetchCommentReplies).toHaveBeenCalledTimes(1);
	});

	it("ignores a second same-tick loadMore call", async () => {
		repo.fetchCommentReplies.mockResolvedValueOnce({
			records: [reply("r1")],
			nextPageToken: "tok",
			pageSize: 20,
		});
		const { result } = renderHook(() => useCommentReplies("p", "c"), {
			wrapper,
		});
		act(() => result.current.expand());
		await waitFor(() => expect(result.current.hasMore).toBe(true));
		let resolve!: (v: unknown) => void;
		repo.fetchCommentReplies.mockReturnValueOnce(
			new Promise((r) => {
				resolve = r;
			}),
		);
		let a!: Promise<void>;
		let b!: Promise<void>;
		act(() => {
			a = result.current.loadMore();
			b = result.current.loadMore();
		});
		resolve({ records: [reply("r2")], nextPageToken: "", pageSize: 20 });
		await act(async () => {
			await Promise.all([a, b]);
		});
		expect(repo.fetchCommentReplies).toHaveBeenCalledTimes(2);
	});

	it("records a loadMore failure without rejecting and allows retry", async () => {
		repo.fetchCommentReplies.mockResolvedValueOnce({
			records: [reply("r1")],
			nextPageToken: "tok",
			pageSize: 20,
		});
		const { result } = renderHook(() => useCommentReplies("p", "c"), {
			wrapper,
		});
		act(() => result.current.expand());
		await waitFor(() => expect(result.current.hasMore).toBe(true));
		const err = new Error("boom");
		repo.fetchCommentReplies.mockRejectedValueOnce(err);
		await act(async () => {
			await result.current.loadMore();
		});
		await waitFor(() => expect(result.current.loadMoreError).toBe(err));
		expect(result.current.hasMore).toBe(true);
		repo.fetchCommentReplies.mockResolvedValueOnce({
			records: [reply("r2")],
			nextPageToken: "",
			pageSize: 20,
		});
		await act(async () => {
			await result.current.loadMore();
		});
		await waitFor(() => expect(result.current.loadMoreError).toBeNull());
		await waitFor(() => expect(result.current.items).toHaveLength(2));
	});
});

function page(records: Comment[], nextPageToken = "") {
	return { records, nextPageToken, pageSize: 20 };
}

function mountThread() {
	return renderHook(
		() => ({
			list: usePostComments("p"),
			thread: useCommentReplies("p", "c1", "樓主"),
		}),
		{ wrapper },
	);
}

async function openThread(
	parents: Comment[] = [
		reply("c1", { replyCount: 1 }),
		reply("c2", { replyCount: 3 }),
	],
	replyPage = page([reply("r1", { parentId: "c1" })]),
) {
	repo.fetchPostComments.mockResolvedValue(page(parents));
	repo.fetchCommentReplies.mockResolvedValue(replyPage);
	const hook = mountThread();
	await waitFor(() =>
		expect(hook.result.current.list.comments).toHaveLength(2),
	);
	act(() => hook.result.current.thread.expand());
	await waitFor(() => expect(hook.result.current.thread.items).toHaveLength(1));
	return hook;
}

const countOf = (comments: Comment[], id: string) =>
	comments.find((c) => c.id === id)?.replyCount;

describe("submitting a reply", () => {
	it("appends the optimistic reply at the bottom and bumps the parent's count when the last page is loaded", async () => {
		const { result } = await openThread();
		repo.createComment.mockReturnValue(new Promise(() => undefined));
		act(() => {
			void result.current.thread.reply("新回覆", { replyToId: "c1" });
		});
		await waitFor(() => expect(result.current.thread.items).toHaveLength(2));
		expect(result.current.thread.items[1]).toMatchObject({
			id: REQUEST_KEY,
			content: "新回覆",
			optimistic: true,
			replyToName: "樓主",
			viewerCanEdit: false,
			viewerCanDelete: false,
			viewerCanReply: false,
		});
		expect(countOf(result.current.list.comments, "c1")).toBe(2);
		expect(result.current.thread.submitted).toEqual([]);
	});

	it("shows it from the mutation state at the bottom when the thread has unloaded pages", async () => {
		const { result } = await openThread(
			undefined,
			page([reply("r1", { parentId: "c1" })], "more"),
		);
		repo.createComment.mockReturnValue(new Promise(() => undefined));
		act(() => {
			void result.current.thread.reply("新回覆", { replyToId: "c1" });
		});
		await waitFor(() =>
			expect(result.current.thread.submitted).toHaveLength(1),
		);
		expect(result.current.thread.items.map((r) => r.id)).toEqual(["r1"]);
		expect(result.current.thread.submitted[0].key).toBe(REQUEST_KEY);
		expect(result.current.thread.submitted[0].comment.optimistic).toBe(true);
	});

	it("on failure removes only that reply and only that parent's count, keeping other rows' changes", async () => {
		const { result } = await openThread();
		const create = deferred<Comment>();
		repo.createComment.mockReturnValue(create.promise);
		repo.fetchPostComments.mockReturnValue(new Promise(() => undefined));
		repo.fetchCommentReplies.mockReturnValue(new Promise(() => undefined));
		let sent!: Promise<void>;
		act(() => {
			sent = result.current.thread
				.reply("新回覆", { replyToId: "c1" })
				.catch(() => undefined);
		});
		await waitFor(() => expect(result.current.thread.items).toHaveLength(2));
		act(() =>
			result.current.list.edited({ id: "c2", content: "別列改過", etag: "e9" }),
		);
		await act(async () => {
			create.reject(new Error("boom"));
			await sent;
		});
		await waitFor(() => expect(result.current.thread.items).toHaveLength(1));
		expect(countOf(result.current.list.comments, "c1")).toBe(1);
		expect(result.current.list.comments[1]).toMatchObject({
			id: "c2",
			content: "別列改過",
			etag: "e9",
			replyCount: 3,
		});
	});

	it("refetches the thread and the comments when the submit settles", async () => {
		const { result } = await openThread();
		repo.createComment.mockResolvedValue(reply("rn", { parentId: "c1" }));
		repo.fetchCommentReplies.mockClear();
		repo.fetchPostComments.mockClear();
		await act(async () => {
			await result.current.thread.reply("新回覆", { replyToId: "c1" });
		});
		await waitFor(() => expect(repo.fetchCommentReplies).toHaveBeenCalled());
		await waitFor(() => expect(repo.fetchPostComments).toHaveBeenCalled());
	});
});

describe("removing a reply", () => {
	it("lowers the parent's count and drops the parent placeholder once its last reply is gone", async () => {
		const { result } = await openThread([
			reply("c1", { replyCount: 1, state: "deleted" }),
			reply("c2", { replyCount: 3 }),
		]);
		repo.fetchPostComments.mockReturnValue(new Promise(() => undefined));
		act(() => result.current.thread.removed("r1"));
		await waitFor(() =>
			expect(result.current.list.comments.map((c) => c.id)).toEqual(["c2"]),
		);
	});

	it("keeps a live parent and only lowers its count", async () => {
		const { result } = await openThread();
		repo.fetchPostComments.mockReturnValue(new Promise(() => undefined));
		act(() => result.current.thread.removed("r1"));
		await waitFor(() =>
			expect(countOf(result.current.list.comments, "c1")).toBe(0),
		);
		expect(result.current.list.comments.map((c) => c.id)).toEqual(["c1", "c2"]);
	});
});
