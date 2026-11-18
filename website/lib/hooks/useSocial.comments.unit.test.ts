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
const identity = vi.hoisted(() => ({
	current: { accountId: "a" as string | null },
}));

vi.mock("@/lib/auth/context", () => ({
	useAuth: () => ({ identity: { nickname: "我", avatarKey: "me.png" } }),
}));
vi.mock("@/lib/comments/repo", async (orig) => ({
	...(await orig<typeof import("@/lib/comments/repo")>()),
	...repo,
}));
vi.mock("./useQueryIdentity", () => ({
	useQueryIdentity: () => identity.current,
}));

import type { Comment } from "@/lib/comments/repo";
import { usePostComments } from "./useSocial";

function comment(id: string, over: Partial<Comment> = {}): Comment {
	return {
		id,
		postId: "p1",
		isMine: true,
		userName: "我",
		content: `內容${id}`,
		createdAt: 0,
		etag: `e-${id}`,
		state: "visible",
		viewerCanEdit: true,
		viewerCanDelete: true,
		viewerCanReply: true,
		...over,
	};
}

function page(records: Comment[], nextPageToken = "") {
	return { records, nextPageToken, pageSize: 20 };
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

function mount() {
	return renderHook(() => usePostComments("p1"), { wrapper });
}

beforeEach(() => {
	client = new QueryClient({
		defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } },
	});
	identity.current = { accountId: "a" };
	for (const fn of Object.values(repo)) fn.mockReset();
	repo.fetchPostComments.mockResolvedValue(page([comment("c1")]));
	repo.deleteComment.mockResolvedValue(undefined);
	vi.spyOn(crypto, "randomUUID").mockReturnValue(
		"11111111-1111-4111-8111-111111111111",
	);
});
afterEach(() => {
	cleanup();
	vi.restoreAllMocks();
});

const REQUEST_KEY = "tmp-11111111-1111-4111-8111-111111111111";

describe("comment edit keeps the cached etag current", () => {
	it("delete after an edit sends the etag the edit returned, not the loaded one", async () => {
		const { result } = mount();
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

describe("post comments page with a cursor and a refetch keeps the loaded pages", () => {
	function twoPages() {
		repo.fetchPostComments.mockImplementation(
			async (_postId: string, opts: { pageToken?: string }) =>
				opts.pageToken === "t1"
					? page([comment("c2")])
					: page([comment("c1")], "t1"),
		);
	}

	it("loadMore sends the next token and appends the page", async () => {
		twoPages();
		const { result } = mount();
		await waitFor(() => expect(result.current.hasMore).toBe(true));
		await act(async () => {
			await result.current.loadMore();
		});
		expect(repo.fetchPostComments).toHaveBeenLastCalledWith(
			"p1",
			expect.objectContaining({ pageToken: "t1" }),
		);
		await waitFor(() =>
			expect(result.current.comments.map((c) => c.id)).toEqual(["c1", "c2"]),
		);
		expect(result.current.hasMore).toBe(false);
	});

	it("refresh refetches every loaded page instead of dropping back to the first", async () => {
		twoPages();
		const { result } = mount();
		await waitFor(() => expect(result.current.hasMore).toBe(true));
		await act(async () => {
			await result.current.loadMore();
		});
		repo.fetchPostComments.mockClear();
		await act(async () => {
			await result.current.refresh();
		});
		const tokens = repo.fetchPostComments.mock.calls.map(
			(call) => call[1].pageToken,
		);
		expect(tokens).toEqual(["", "t1"]);
		await waitFor(() =>
			expect(result.current.comments.map((c) => c.id)).toEqual(["c1", "c2"]),
		);
	});
});

describe("submitting a comment", () => {
	it("inserts the optimistic row at the end only when the last page is loaded", async () => {
		const create = deferred<Comment>();
		repo.createComment.mockReturnValue(create.promise);
		const { result } = mount();
		await waitFor(() => expect(result.current.comments).toHaveLength(1));
		act(() => {
			void result.current.post("新留言");
		});
		await waitFor(() => expect(result.current.comments).toHaveLength(2));
		expect(result.current.comments[1]).toMatchObject({
			id: REQUEST_KEY,
			content: "新留言",
			optimistic: true,
			viewerCanEdit: false,
			viewerCanDelete: false,
			viewerCanReply: false,
		});
		expect(result.current.submitted).toEqual([]);
		create.resolve(comment("real"));
	});

	it("does not insert into the list while more pages are unloaded and shows it from the mutation state", async () => {
		repo.fetchPostComments.mockResolvedValue(page([comment("c1")], "t1"));
		const create = deferred<Comment>();
		repo.createComment.mockReturnValue(create.promise);
		const { result } = mount();
		await waitFor(() => expect(result.current.hasMore).toBe(true));
		act(() => {
			void result.current.post("新留言");
		});
		await waitFor(() => expect(result.current.submitted).toHaveLength(1));
		expect(result.current.comments.map((c) => c.id)).toEqual(["c1"]);
		expect(result.current.submitted[0].key).toBe(REQUEST_KEY);
		expect(result.current.submitted[0].comment).toMatchObject({
			content: "新留言",
			optimistic: true,
		});
		create.resolve(comment("real"));
	});

	it("keeps the same key from pending to success and drops it once the page that holds it is loaded", async () => {
		repo.fetchPostComments.mockResolvedValue(page([comment("c1")], "t1"));
		const create = deferred<Comment>();
		repo.createComment.mockReturnValue(create.promise);
		const { result } = mount();
		await waitFor(() => expect(result.current.hasMore).toBe(true));
		act(() => {
			void result.current.post("新留言");
		});
		await waitFor(() => expect(result.current.submitted).toHaveLength(1));
		repo.fetchPostComments.mockResolvedValue(page([comment("c1")], "t1"));
		await act(async () => {
			create.resolve(comment("real", { content: "新留言" }));
			await create.promise;
		});
		await waitFor(() =>
			expect(result.current.submitted[0]?.comment.id).toBe("real"),
		);
		expect(result.current.submitted[0].key).toBe(REQUEST_KEY);
		repo.fetchPostComments.mockResolvedValue(
			page([comment("c1"), comment("real")], "t1"),
		);
		await act(async () => {
			await result.current.refresh();
		});
		await waitFor(() => expect(result.current.submitted).toEqual([]));
	});

	it("scopes the submitted rows to the viewer identity", async () => {
		repo.fetchPostComments.mockResolvedValue(page([comment("c1")], "t1"));
		repo.createComment.mockReturnValue(new Promise(() => undefined));
		const { result, rerender } = mount();
		await waitFor(() => expect(result.current.hasMore).toBe(true));
		act(() => {
			void result.current.post("新留言");
		});
		await waitFor(() => expect(result.current.submitted).toHaveLength(1));
		identity.current = { accountId: "b" };
		rerender();
		expect(result.current.submitted).toEqual([]);
	});

	it("cancels an in-flight refetch before inserting so it cannot wash the row out", async () => {
		const { result } = mount();
		await waitFor(() => expect(result.current.comments).toHaveLength(1));
		const slow = deferred<ReturnType<typeof page>>();
		repo.fetchPostComments.mockReturnValue(slow.promise);
		act(() => {
			void result.current.refresh();
		});
		repo.createComment.mockReturnValue(new Promise(() => undefined));
		act(() => {
			void result.current.post("新留言");
		});
		await waitFor(() => expect(result.current.comments).toHaveLength(2));
		await act(async () => {
			slow.resolve(page([comment("c1")]));
			await slow.promise;
		});
		await waitFor(() => expect(client.isFetching()).toBe(0));
		await act(async () => {
			await new Promise((resolve) => setTimeout(resolve, 0));
		});
		expect(result.current.comments.map((c) => c.id)).toEqual([
			"c1",
			REQUEST_KEY,
		]);
	});

	it("on failure removes only that optimistic row and keeps edits made to other rows meanwhile", async () => {
		repo.fetchPostComments.mockResolvedValueOnce(
			page([comment("c1"), comment("c2")]),
		);
		const create = deferred<Comment>();
		repo.createComment.mockReturnValue(create.promise);
		const { result } = mount();
		await waitFor(() => expect(result.current.comments).toHaveLength(2));
		repo.fetchPostComments.mockReturnValue(new Promise(() => undefined));
		let posted!: Promise<void>;
		act(() => {
			posted = result.current.post("新留言").catch(() => undefined);
		});
		await waitFor(() => expect(result.current.comments).toHaveLength(3));
		act(() => result.current.edited({ id: "c1", content: "改過", etag: "e9" }));
		await act(async () => {
			create.reject(new Error("boom"));
			await posted;
		});
		await waitFor(() => expect(result.current.comments).toHaveLength(2));
		expect(result.current.comments[0]).toMatchObject({
			id: "c1",
			content: "改過",
			etag: "e9",
		});
	});

	it("refetches the list when the submit settles", async () => {
		repo.createComment.mockResolvedValue(comment("real"));
		const { result } = mount();
		await waitFor(() => expect(result.current.comments).toHaveLength(1));
		repo.fetchPostComments.mockClear();
		repo.fetchPostComments.mockResolvedValue(
			page([comment("c1"), comment("real", { content: "伺服器版" })]),
		);
		await act(async () => {
			await result.current.post("新留言");
		});
		await waitFor(() => expect(repo.fetchPostComments).toHaveBeenCalled());
		await waitFor(() =>
			expect(result.current.comments[1].content).toBe("伺服器版"),
		);
	});
});

describe("a submission stops being shown once it has landed or been deleted", () => {
	it("post then delete: the row is gone and does not come back from the successful mutation", async () => {
		repo.createComment.mockResolvedValue(
			comment("real", { content: "新留言" }),
		);
		const { result } = mount();
		await waitFor(() => expect(result.current.comments).toHaveLength(1));
		repo.fetchPostComments.mockResolvedValue(
			page([comment("c1"), comment("real")]),
		);
		await act(async () => {
			await result.current.post("新留言");
		});
		await waitFor(() =>
			expect(result.current.comments.map((c) => c.id)).toContain("real"),
		);
		repo.fetchPostComments.mockResolvedValue(page([comment("c1")]));
		await act(async () => {
			await result.current.remove("real");
		});
		await waitFor(() =>
			expect(result.current.comments.map((c) => c.id)).toEqual(["c1"]),
		);
		await act(async () => {
			await result.current.refresh();
		});
		expect(result.current.submitted).toEqual([]);
		expect(result.current.comments.map((c) => c.id)).toEqual(["c1"]);
	});
});

describe("a pending load-more survives submitting", () => {
	it("posting while the next page is loading keeps that page", async () => {
		const more = deferred<ReturnType<typeof page>>();
		repo.fetchPostComments.mockImplementation(
			(_postId: string, opts: { pageToken?: string }) =>
				opts.pageToken === "t1"
					? more.promise
					: Promise.resolve(page([comment("c1")], "t1")),
		);
		repo.createComment.mockResolvedValue(comment("real"));
		const { result } = mount();
		await waitFor(() => expect(result.current.hasMore).toBe(true));
		act(() => {
			void result.current.loadMore();
		});
		await waitFor(() => expect(result.current.loadingMore).toBe(true));
		await act(async () => {
			await result.current.post("新留言");
		});
		await act(async () => {
			more.resolve(page([comment("c2")]));
			await more.promise;
		});
		await waitFor(() =>
			expect(result.current.comments.map((c) => c.id)).toContain("c2"),
		);
	});
});
