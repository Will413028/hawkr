/** @vitest-environment jsdom */

import {
	cleanup,
	fireEvent,
	render,
	screen,
	waitFor,
} from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

const forum = vi.hoisted(() => ({
	list: [] as unknown[],
	error: null as unknown,
	replies: {} as Record<string, unknown[]>,
	hookArgs: [] as unknown[][],
	repliesAsked: [] as string[],
	post: vi.fn(),
	reply: vi.fn(),
	remove: vi.fn(),
	loading: false,
	replace: vi.fn(),
}));
vi.mock("@/lib/navigation/use-observed-router", () => ({
	useObservedRouter: () => ({ replace: forum.replace }),
}));

vi.mock("@/lib/auth/context", () => ({
	useAuth: () => ({ status: "signedIn", identity: { nickname: "我" } }),
}));
vi.mock("@/lib/hooks/useSellerForum", () => ({
	useSellerForum: (...args: unknown[]) => {
		forum.hookArgs.push(args);
		return {
			comments: forum.list,
			submitted: [],
			loading: forum.loading,
			refreshing: false,
			loadingMore: false,
			hasMore: false,
			error: forum.error,
			loadMoreError: null,
			refresh: vi.fn(),
			loadMore: vi.fn(),
			post: forum.post,
			reply: forum.reply,
			update: vi.fn(),
			remove: forum.remove,
		};
	},
	useForumReplies: (
		_store: string,
		threadId: string,
		_name: string,
		enabled: boolean,
	) => {
		if (enabled) forum.repliesAsked.push(threadId);
		return {
			items: enabled ? (forum.replies[threadId] ?? []) : [],
			submitted: [],
			loading: false,
			hasMore: false,
			loadingMore: false,
			error: null,
			loadMoreError: null,
			loadMore: vi.fn(),
			refresh: vi.fn(),
		};
	},
}));

import { Code, ConnectError } from "@connectrpc/connect";
import { IntlTestWrapper } from "@/tests/mocks/intl-test-wrapper";
import SellerMessageTab from "./SellerMessageTab";

function entry(id: string, over: Record<string, unknown> = {}) {
	return {
		id,
		storeId: "7",
		isMine: false,
		userName: "alice",
		content: `body-${id}`,
		createdAt: 0,
		state: "visible",
		viewerCanEdit: false,
		viewerCanDelete: false,
		viewerCanReply: true,
		...over,
	};
}

function view(threadId = "t1") {
	render(
		React.createElement(
			IntlTestWrapper,
			null,
			React.createElement(SellerMessageTab, {
				sellerId: "7",
				routeId: "7",
				threadId,
			}),
		),
	);
}

afterEach(() => {
	cleanup();
	forum.error = null;
	forum.hookArgs = [];
	forum.repliesAsked = [];
	forum.post.mockReset();
	forum.reply.mockReset();
	forum.remove.mockReset();
	forum.replace.mockReset();
	forum.loading = false;
});

describe("single thread page", () => {
	it("asks the hook for that thread, shows its top entry and its replies expanded", () => {
		forum.list = [entry("t1", { replyCount: 1 })];
		forum.replies = { t1: [entry("r1", { content: "reply-body" })] };
		view();
		expect(forum.hookArgs[0]?.[2]).toBe("t1");
		expect(forum.repliesAsked).toContain("t1");
		expect(screen.queryByText("body-t1")).not.toBeNull();
		expect(screen.queryByText("reply-body")).not.toBeNull();
	});

	it("the composer replies to the thread and never posts a top-level entry", async () => {
		forum.list = [entry("t1")];
		view();
		const box = screen.getByPlaceholderText("回覆 alice…");
		fireEvent.change(box, { target: { value: "hello" } });
		fireEvent.click(
			document.querySelector("button[type=submit]") as HTMLButtonElement,
		);
		await waitFor(() => expect(forum.reply).toHaveBeenCalledTimes(1));
		expect(forum.reply.mock.calls[0]?.[0]).toMatchObject({ id: "t1" });
		expect(forum.reply.mock.calls[0]?.[1]).toBe("hello");
		expect(forum.post).not.toHaveBeenCalled();
	});

	it("a placeholder top entry has no composer", () => {
		forum.list = [
			entry("t1", {
				state: "deleted",
				content: "",
				viewerCanReply: false,
				replyCount: 1,
			}),
		];
		view();
		expect(screen.queryByPlaceholderText("留言給這個賣家…")).toBeNull();
		expect(document.querySelector("textarea")).toBeNull();
	});

	it("a viewer the server does not allow to reply (viewerCanReply false) sees no composer", () => {
		forum.list = [entry("t1", { viewerCanReply: false })];
		view();
		expect(screen.queryByText("body-t1")).not.toBeNull();
		expect(document.querySelector("textarea")).toBeNull();
	});

	it("a NotFound after a refetch replaces the stale thread with the error state", () => {
		forum.list = [entry("t1")];
		forum.error = new ConnectError("gone", Code.NotFound);
		view();
		expect(screen.queryByText("body-t1")).toBeNull();
		expect(document.querySelector("textarea")).toBeNull();
		expect(screen.queryByText("暫無留言")).toBeNull();
		expect(screen.queryByText("重新載入")).not.toBeNull();
	});

	it("an empty thread with no error (query not running) is not the store-wide empty state or an error", () => {
		forum.list = [];
		view();
		expect(screen.queryByText("暫無留言")).toBeNull();
		expect(screen.queryByText("重新載入")).toBeNull();
		expect(screen.queryByText("載入中…")).not.toBeNull();
	});

	it("a non-NotFound error keeps the generic error state", () => {
		forum.list = [];
		forum.error = new ConnectError("boom", Code.Unavailable);
		view();
		expect(screen.queryByText("重新載入")).not.toBeNull();
	});

	it("deleting your own top entry with no replies goes back to the store messages tab", async () => {
		forum.list = [entry("t1", { viewerCanDelete: true, replyCount: 0 })];
		forum.remove.mockResolvedValue(undefined);
		vi.spyOn(window, "confirm").mockReturnValue(true);
		view();
		fireEvent.click(screen.getByText("刪除"));
		await waitFor(() => expect(forum.remove).toHaveBeenCalledTimes(1));
		await waitFor(() =>
			expect(forum.replace).toHaveBeenCalledWith("/sellers/7/messages"),
		);
	});

	it("deleting a reply stays on the thread page", async () => {
		forum.list = [entry("t1", { replyCount: 1 })];
		forum.replies = {
			t1: [entry("r1", { parentId: "t1", viewerCanDelete: true })],
		};
		forum.remove.mockResolvedValue(undefined);
		vi.spyOn(window, "confirm").mockReturnValue(true);
		view();
		fireEvent.click(screen.getByText("刪除"));
		await waitFor(() => expect(forum.remove).toHaveBeenCalledTimes(1));
		expect(forum.replace).not.toHaveBeenCalled();
	});
});
