/** @vitest-environment jsdom */

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { SellerForumComment } from "@/lib/forum/repo";

const repo = vi.hoisted(() => ({
	fetchSellerForum: vi.fn(),
	fetchSellerForumReplies: vi.fn(),
	createSellerForumComment: vi.fn(),
	replySellerForumComment: vi.fn(),
	updateSellerForumComment: vi.fn(),
	deleteSellerForumComment: vi.fn(),
}));
const identity = vi.hoisted(() => ({
	current: { accountId: "a" as string | null },
}));

vi.mock("@/lib/auth/context", () => ({
	useAuth: () => ({ identity: { nickname: "我", avatarKey: "me.png" } }),
}));
vi.mock("@/lib/forum/repo", async (orig) => ({
	...(await orig<typeof import("@/lib/forum/repo")>()),
	...repo,
}));
vi.mock("./useQueryIdentity", () => ({
	useQueryIdentity: () => identity.current,
}));

import { useForumReplies, useSellerForum } from "./useSellerForum";

function entry(id: string, over: Partial<SellerForumComment> = {}) {
	return {
		id,
		storeId: "7",
		isMine: false,
		userName: `u-${id}`,
		content: id,
		createdAt: 0,
		etag: `e-${id}`,
		state: "visible" as const,
		viewerCanEdit: true,
		viewerCanDelete: true,
		viewerCanReply: true,
		...over,
	};
}

function page(records: SellerForumComment[], nextPageToken = "") {
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

const never = () => new Promise<never>(() => undefined);

let client: QueryClient;
function wrapper({ children }: { children: React.ReactNode }) {
	return React.createElement(QueryClientProvider, { client }, children);
}

const REQUEST_KEY = "tmp-33333333-3333-4333-8333-333333333333";

beforeEach(() => {
	client = new QueryClient({
		defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } },
	});
	identity.current = { accountId: "a" };
	for (const fn of Object.values(repo)) fn.mockReset();
	vi.spyOn(crypto, "randomUUID").mockReturnValue(
		"33333333-3333-4333-8333-333333333333",
	);
});
afterEach(() => {
	cleanup();
	vi.restoreAllMocks();
});

/** 頂層留言 top（兩則回覆，其中 r1 載入）與 lonely（沒有回覆），回覆串展開。 */
async function mountForum(
	opts: { top?: Partial<SellerForumComment>; topToken?: string } = {},
) {
	repo.fetchSellerForum.mockResolvedValue(
		page(
			[
				entry("top", { replyCount: 2, ...opts.top }),
				entry("lonely", { replyCount: 0 }),
			],
			opts.topToken ?? "",
		),
	);
	repo.fetchSellerForumReplies.mockResolvedValue(
		page([
			entry("r1", { parentId: "top", replyToName: "u-top" }),
			entry("r2", { parentId: "top" }),
		]),
	);
	const hook = renderHook(
		() => ({
			forum: useSellerForum("7"),
			thread: useForumReplies("7", "top", "u-top", true),
		}),
		{ wrapper },
	);
	await waitFor(() =>
		expect(hook.result.current.forum.comments).toHaveLength(2),
	);
	await waitFor(() => expect(hook.result.current.thread.items).toHaveLength(2));
	return hook;
}

const ids = (items: { id: string }[]) => items.map((i) => i.id);

describe("posting to the forum", () => {
	it("appends the optimistic entry at the end only when the last page is loaded", async () => {
		const { result } = await mountForum();
		repo.createSellerForumComment.mockReturnValue(never());
		act(() => {
			void result.current.forum.post("新留言");
		});
		await waitFor(() => expect(result.current.forum.comments).toHaveLength(3));
		expect(result.current.forum.comments[2]).toMatchObject({
			id: REQUEST_KEY,
			content: "新留言",
			optimistic: true,
			viewerCanEdit: false,
			viewerCanDelete: false,
			viewerCanReply: false,
		});
		expect(result.current.forum.submitted).toEqual([]);
	});

	it("shows it from the mutation state with a stable key when more pages are unloaded", async () => {
		const { result } = await mountForum({ topToken: "t1" });
		const create = deferred<SellerForumComment>();
		repo.createSellerForumComment.mockReturnValue(create.promise);
		act(() => {
			void result.current.forum.post("新留言");
		});
		await waitFor(() => expect(result.current.forum.submitted).toHaveLength(1));
		expect(ids(result.current.forum.comments)).toEqual(["top", "lonely"]);
		expect(result.current.forum.submitted[0].key).toBe(REQUEST_KEY);
		await act(async () => {
			create.resolve(entry("real", { content: "新留言" }));
			await create.promise;
		});
		await waitFor(() =>
			expect(result.current.forum.submitted[0]?.entry.id).toBe("real"),
		);
		expect(result.current.forum.submitted[0].key).toBe(REQUEST_KEY);
	});

	it("cancels an in-flight refetch before inserting so it cannot wash the entry out", async () => {
		const { result } = await mountForum();
		const slow = deferred<ReturnType<typeof page>>();
		repo.fetchSellerForum.mockReturnValue(slow.promise);
		act(() => {
			void result.current.forum.refresh();
		});
		repo.createSellerForumComment.mockReturnValue(never());
		act(() => {
			void result.current.forum.post("新留言");
		});
		await waitFor(() => expect(result.current.forum.comments).toHaveLength(3));
		await act(async () => {
			slow.resolve(page([entry("top", { replyCount: 2 }), entry("lonely")]));
			await slow.promise;
		});
		await waitFor(() => expect(client.isFetching()).toBe(0));
		await act(async () => {
			await new Promise((resolve) => setTimeout(resolve, 0));
		});
		expect(ids(result.current.forum.comments)).toEqual([
			"top",
			"lonely",
			REQUEST_KEY,
		]);
	});

	it("scopes the submitted entries to the viewer identity", async () => {
		const { result, rerender } = await mountForum({ topToken: "t1" });
		repo.createSellerForumComment.mockReturnValue(never());
		act(() => {
			void result.current.forum.post("新留言");
		});
		await waitFor(() => expect(result.current.forum.submitted).toHaveLength(1));
		identity.current = { accountId: "b" };
		rerender();
		expect(result.current.forum.submitted).toEqual([]);
	});

	it("on failure removes only that entry and keeps edits made to other rows meanwhile", async () => {
		const { result } = await mountForum();
		const create = deferred<SellerForumComment>();
		repo.createSellerForumComment.mockReturnValue(create.promise);
		repo.fetchSellerForum.mockReturnValue(never());
		let posted!: Promise<void>;
		act(() => {
			posted = result.current.forum.post("新留言").catch(() => undefined);
		});
		await waitFor(() => expect(result.current.forum.comments).toHaveLength(3));
		repo.updateSellerForumComment.mockResolvedValue(
			entry("lonely", { content: "改過", etag: "e9" }),
		);
		await act(async () => {
			await result.current.forum.update(
				result.current.forum.comments[1],
				"改過",
			);
		});
		await act(async () => {
			create.reject(new Error("boom"));
			await posted;
		});
		await waitFor(() => expect(result.current.forum.comments).toHaveLength(2));
		expect(result.current.forum.comments[1]).toMatchObject({
			id: "lonely",
			content: "改過",
			etag: "e9",
		});
	});

	it("refetches the forum when the submit settles", async () => {
		const { result } = await mountForum();
		repo.createSellerForumComment.mockResolvedValue(entry("real"));
		repo.fetchSellerForum.mockClear();
		await act(async () => {
			await result.current.forum.post("新留言");
		});
		await waitFor(() => expect(repo.fetchSellerForum).toHaveBeenCalled());
	});
});

describe("replying in a forum thread", () => {
	it("appends at the bottom of the thread and bumps the thread's count; failure rolls back only those", async () => {
		const { result } = await mountForum();
		const create = deferred<SellerForumComment>();
		repo.replySellerForumComment.mockReturnValue(create.promise);
		repo.fetchSellerForum.mockReturnValue(never());
		repo.fetchSellerForumReplies.mockReturnValue(never());
		let sent!: Promise<void>;
		act(() => {
			sent = result.current.forum
				.reply(result.current.thread.items[1], "新回覆")
				.catch(() => undefined);
		});
		await waitFor(() => expect(result.current.thread.items).toHaveLength(3));
		expect(result.current.thread.items[2]).toMatchObject({
			id: REQUEST_KEY,
			parentId: "top",
			replyToId: "r2",
			replyToName: "u-r2",
			optimistic: true,
			viewerCanEdit: false,
			viewerCanDelete: false,
		});
		expect(repo.replySellerForumComment).toHaveBeenCalledWith(
			"7",
			"top",
			"r2",
			"新回覆",
			"33333333-3333-4333-8333-333333333333",
		);
		expect(result.current.forum.comments[0].replyCount).toBe(3);
		repo.updateSellerForumComment.mockResolvedValue(
			entry("lonely", { content: "改過", etag: "e9" }),
		);
		await act(async () => {
			await result.current.forum.update(
				result.current.forum.comments[1],
				"改過",
			);
		});
		await act(async () => {
			create.reject(new Error("boom"));
			await sent;
		});
		await waitFor(() => expect(result.current.thread.items).toHaveLength(2));
		expect(result.current.forum.comments[0].replyCount).toBe(2);
		expect(result.current.forum.comments[1]).toMatchObject({
			content: "改過",
			etag: "e9",
		});
	});

	it("shows it at the bottom of the thread from the mutation state when the thread has unloaded pages", async () => {
		const { result } = await mountForum();
		repo.fetchSellerForumReplies.mockResolvedValue(
			page([entry("r1", { parentId: "top" })], "more"),
		);
		await act(async () => {
			await result.current.thread.refresh();
		});
		await waitFor(() => expect(result.current.thread.hasMore).toBe(true));
		repo.replySellerForumComment.mockReturnValue(never());
		act(() => {
			void result.current.forum.reply(
				result.current.forum.comments[0],
				"新回覆",
			);
		});
		await waitFor(() =>
			expect(result.current.thread.submitted).toHaveLength(1),
		);
		expect(ids(result.current.thread.items)).toEqual(["r1"]);
		expect(result.current.thread.submitted[0].key).toBe(REQUEST_KEY);
	});
});

describe("editing an entry", () => {
	it("keeps the client-only reply name and takes the server's new etag", async () => {
		const { result } = await mountForum();
		repo.updateSellerForumComment.mockResolvedValue(
			entry("r1", { content: "edited", parentId: "top", etag: "e2" }),
		);
		repo.fetchSellerForumReplies.mockReturnValue(never());
		await act(async () => {
			await result.current.forum.update(
				result.current.thread.items[0],
				"edited",
			);
		});
		await waitFor(() => expect(result.current.thread.items[0].etag).toBe("e2"));
		expect(result.current.thread.items[0]).toMatchObject({
			content: "edited",
			replyToName: "u-top",
		});
		expect(repo.updateSellerForumComment).toHaveBeenCalledWith(
			"r1",
			"edited",
			"e-r1",
		);
	});

	it("on failure restores only that row's content and keeps another row's newer etag", async () => {
		const { result } = await mountForum();
		const failing = deferred<SellerForumComment>();
		repo.updateSellerForumComment.mockReturnValueOnce(failing.promise);
		repo.fetchSellerForum.mockReturnValue(never());
		let first!: Promise<void>;
		act(() => {
			first = result.current.forum
				.update(result.current.forum.comments[0], "會失敗")
				.catch(() => undefined);
		});
		await waitFor(() =>
			expect(result.current.forum.comments[0].content).toBe("會失敗"),
		);
		repo.updateSellerForumComment.mockResolvedValueOnce(
			entry("lonely", { content: "成功", etag: "e9" }),
		);
		await act(async () => {
			await result.current.forum.update(
				result.current.forum.comments[1],
				"成功",
			);
		});
		await act(async () => {
			failing.reject(new Error("boom"));
			await first;
		});
		await waitFor(() =>
			expect(result.current.forum.comments[0].content).toBe("top"),
		);
		expect(result.current.forum.comments[1]).toMatchObject({
			content: "成功",
			etag: "e9",
		});
	});
});

describe("removing an entry follows the ListForumEntries contract", () => {
	it("deleting a reply lowers the thread's count and drops only that reply", async () => {
		const { result } = await mountForum();
		repo.deleteSellerForumComment.mockResolvedValue(undefined);
		repo.fetchSellerForum.mockReturnValue(never());
		repo.fetchSellerForumReplies.mockReturnValue(never());
		await act(async () => {
			await result.current.forum.remove(result.current.thread.items[0]);
		});
		expect(repo.deleteSellerForumComment).toHaveBeenCalledWith("r1", "e-r1");
		await waitFor(() =>
			expect(ids(result.current.thread.items)).toEqual(["r2"]),
		);
		expect(result.current.forum.comments[0].replyCount).toBe(1);
	});

	it("deleting the last reply of a deleted placeholder makes the placeholder disappear", async () => {
		const { result } = await mountForum({
			top: { state: "deleted", replyCount: 1, viewerCanDelete: false },
		});
		repo.deleteSellerForumComment.mockResolvedValue(undefined);
		repo.fetchSellerForum.mockReturnValue(never());
		repo.fetchSellerForumReplies.mockReturnValue(never());
		await act(async () => {
			await result.current.forum.remove(result.current.thread.items[0]);
		});
		await waitFor(() =>
			expect(ids(result.current.forum.comments)).toEqual(["lonely"]),
		);
	});

	it("deleting a top-level entry that has replies leaves a deleted placeholder", async () => {
		const { result } = await mountForum();
		repo.deleteSellerForumComment.mockResolvedValue(undefined);
		repo.fetchSellerForum.mockReturnValue(never());
		await act(async () => {
			await result.current.forum.remove(result.current.forum.comments[0]);
		});
		await waitFor(() =>
			expect(result.current.forum.comments[0].state).toBe("deleted"),
		);
		expect(result.current.forum.comments[0]).toMatchObject({
			id: "top",
			content: "",
			viewerCanEdit: false,
			viewerCanDelete: false,
			replyCount: 2,
		});
	});

	it("deleting a top-level entry without replies removes it", async () => {
		const { result } = await mountForum();
		repo.deleteSellerForumComment.mockResolvedValue(undefined);
		repo.fetchSellerForum.mockReturnValue(never());
		await act(async () => {
			await result.current.forum.remove(result.current.forum.comments[1]);
		});
		await waitFor(() =>
			expect(ids(result.current.forum.comments)).toEqual(["top"]),
		);
	});

	it("a failed delete restores only that row and keeps another row's edit", async () => {
		const { result } = await mountForum();
		const failing = deferred<void>();
		repo.deleteSellerForumComment.mockReturnValue(failing.promise);
		repo.fetchSellerForum.mockReturnValue(never());
		let removing!: Promise<void>;
		act(() => {
			removing = result.current.forum
				.remove(result.current.forum.comments[1])
				.catch(() => undefined);
		});
		await waitFor(() => expect(result.current.forum.comments).toHaveLength(1));
		repo.updateSellerForumComment.mockResolvedValue(
			entry("top", { content: "改過", etag: "e9", replyCount: 2 }),
		);
		await act(async () => {
			await result.current.forum.update(
				result.current.forum.comments[0],
				"改過",
			);
		});
		await act(async () => {
			failing.reject(new Error("boom"));
			await removing;
		});
		await waitFor(() => expect(result.current.forum.comments).toHaveLength(2));
		expect(result.current.forum.comments[1].id).toBe("lonely");
		expect(result.current.forum.comments[0]).toMatchObject({
			content: "改過",
			etag: "e9",
		});
	});

	it("a failed reply delete restores the reply and the thread's count", async () => {
		const { result } = await mountForum();
		const failing = deferred<void>();
		repo.deleteSellerForumComment.mockReturnValue(failing.promise);
		repo.fetchSellerForum.mockReturnValue(never());
		repo.fetchSellerForumReplies.mockReturnValue(never());
		let removing!: Promise<void>;
		act(() => {
			removing = result.current.forum
				.remove(result.current.thread.items[0])
				.catch(() => undefined);
		});
		await waitFor(() => expect(result.current.thread.items).toHaveLength(1));
		await act(async () => {
			failing.reject(new Error("boom"));
			await removing;
		});
		await waitFor(() => expect(result.current.thread.items).toHaveLength(2));
		expect(ids(result.current.thread.items)).toEqual(["r1", "r2"]);
		expect(result.current.forum.comments[0].replyCount).toBe(2);
	});
});

describe("a submission stops being shown once it has landed or been deleted", () => {
	it("post then delete: the entry is gone and does not come back from the successful mutation", async () => {
		const { result } = await mountForum();
		repo.createSellerForumComment.mockResolvedValue(entry("real"));
		repo.fetchSellerForum.mockResolvedValue(
			page([entry("top", { replyCount: 2 }), entry("lonely"), entry("real")]),
		);
		await act(async () => {
			await result.current.forum.post("新發言");
		});
		await waitFor(() =>
			expect(ids(result.current.forum.comments)).toContain("real"),
		);
		repo.deleteSellerForumComment.mockResolvedValue(undefined);
		repo.fetchSellerForum.mockResolvedValue(
			page([entry("top", { replyCount: 2 }), entry("lonely")]),
		);
		await act(async () => {
			await result.current.forum.remove(
				result.current.forum.comments.find((c) => c.id === "real") as never,
			);
		});
		await act(async () => {
			await result.current.forum.refresh();
		});
		await waitFor(() =>
			expect(ids(result.current.forum.comments)).toEqual(["top", "lonely"]),
		);
		expect(result.current.forum.submitted).toEqual([]);
	});
});

describe("an ordinary refetch in flight when a row is deleted", () => {
	it("cannot bring the deleted row back when its stale response lands later", async () => {
		const { result } = await mountForum();
		const staleRefetch = deferred<ReturnType<typeof page>>();
		repo.createSellerForumComment.mockResolvedValue(entry("real"));
		repo.fetchSellerForum.mockReturnValueOnce(staleRefetch.promise);
		repo.fetchSellerForum.mockResolvedValue(
			page([entry("top", { replyCount: 2 }), entry("lonely")]),
		);
		await act(async () => {
			await result.current.forum.post("新發言");
		});
		expect(repo.fetchSellerForum).toHaveBeenCalledTimes(2);
		await waitFor(() =>
			expect(ids(result.current.forum.comments)).toContain("real"),
		);
		repo.deleteSellerForumComment.mockResolvedValue(undefined);
		await act(async () => {
			await result.current.forum.remove(
				result.current.forum.comments.find((c) => c.id === "real") as never,
			);
		});
		await act(async () => {
			staleRefetch.resolve(
				page([entry("top", { replyCount: 2 }), entry("lonely"), entry("real")]),
			);
			await staleRefetch.promise;
		});
		await act(async () => {
			await result.current.forum.refresh();
		});
		await waitFor(() =>
			expect(ids(result.current.forum.comments)).toEqual(["top", "lonely"]),
		);
	});
});

describe("a pending load-more survives submitting", () => {
	async function startLoadMore() {
		const hook = await mountForum({ topToken: "t1" });
		const more = deferred<ReturnType<typeof page>>();
		repo.fetchSellerForum.mockReturnValueOnce(more.promise);
		repo.fetchSellerForum.mockImplementation(never);
		act(() => {
			void hook.result.current.forum.loadMore();
		});
		await waitFor(() =>
			expect(hook.result.current.forum.loadingMore).toBe(true),
		);
		return { ...hook, more };
	}

	it("posting while the next page is loading keeps that page and the new item", async () => {
		const { result, more } = await startLoadMore();
		repo.createSellerForumComment.mockResolvedValue(entry("real"));
		let posting!: Promise<unknown>;
		act(() => {
			posting = result.current.forum.post("新發言");
		});
		await act(async () => {
			more.resolve(page([entry("p2")]));
			await more.promise;
			await posting;
		});
		await waitFor(() =>
			expect(ids(result.current.forum.comments)).toContain("p2"),
		);
		const shown = [
			...ids(result.current.forum.comments),
			...result.current.forum.submitted.map((s) => s.entry.id),
		];
		expect(shown.some((id) => id === "real" || id.startsWith("tmp-"))).toBe(
			true,
		);
	});

	it("deleting while the next page is loading: the row stays gone after the page arrives", async () => {
		const { result, more } = await startLoadMore();
		repo.deleteSellerForumComment.mockResolvedValue(undefined);
		let removing!: Promise<unknown>;
		act(() => {
			removing = result.current.forum.remove(
				result.current.forum.comments.find((c) => c.id === "lonely") as never,
			);
		});
		await act(async () => {
			more.resolve(page([entry("p2")]));
			await more.promise;
			await removing;
		});
		await waitFor(() =>
			expect(ids(result.current.forum.comments)).toContain("p2"),
		);
		expect(ids(result.current.forum.comments)).not.toContain("lonely");
	});

	it("replying while the next page is loading: the reply count stays bumped once the page arrives", async () => {
		const { result, more } = await startLoadMore();
		const sent = deferred<ReturnType<typeof entry>>();
		repo.replySellerForumComment.mockReturnValue(sent.promise);
		let replying!: Promise<unknown>;
		act(() => {
			replying = result.current.forum.reply(
				result.current.forum.comments.find((c) => c.id === "top") as never,
				"回",
			);
		});
		await act(async () => {
			await new Promise((r) => setTimeout(r, 0));
		});
		await act(async () => {
			more.resolve(page([entry("p2")]));
			await more.promise;
		});
		await waitFor(() =>
			expect(ids(result.current.forum.comments)).toContain("p2"),
		);
		await waitFor(() =>
			expect(
				result.current.forum.comments.find((c) => c.id === "top")?.replyCount,
			).toBe(3),
		);
		await act(async () => {
			sent.resolve(entry("rn", { parentId: "top" }));
			await replying;
		});
		expect(
			result.current.forum.comments.find((c) => c.id === "top")?.replyCount,
		).toBe(3);
	});
});

describe("replying does not refetch the parent list", () => {
	it("settling a reply refetches the thread only", async () => {
		const { result } = await mountForum();
		repo.replySellerForumComment.mockResolvedValue(
			entry("rn", { parentId: "top" }),
		);
		repo.fetchSellerForum.mockClear();
		await act(async () => {
			await result.current.forum.reply(result.current.forum.comments[0], "回");
		});
		await waitFor(() =>
			expect(repo.fetchSellerForumReplies).toHaveBeenCalledTimes(2),
		);
		expect(repo.fetchSellerForum).not.toHaveBeenCalled();
	});

	it("settling a reply delete refetches the thread only", async () => {
		const { result } = await mountForum();
		repo.deleteSellerForumComment.mockResolvedValue(undefined);
		repo.fetchSellerForum.mockClear();
		await act(async () => {
			await result.current.forum.remove(result.current.thread.items[0]);
		});
		await waitFor(() =>
			expect(repo.fetchSellerForumReplies).toHaveBeenCalledTimes(2),
		);
		expect(repo.fetchSellerForum).not.toHaveBeenCalled();
	});
});
