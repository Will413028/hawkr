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
	replies: {} as Record<
		string,
		{ items: unknown[]; hasMore?: boolean; loadMoreError?: unknown }
	>,
	loadMore: vi.fn(),
}));

vi.mock("@/lib/auth/context", () => ({
	useAuth: () => ({ status: "signedIn", identity: { nickname: "我" } }),
}));
vi.mock("@/lib/hooks/useSellerForum", () => ({
	useSellerForum: () => ({
		comments: forum.list,
		submitted: [],
		loading: false,
		refreshing: false,
		loadingMore: false,
		hasMore: false,
		error: null,
		loadMoreError: null,
		refresh: vi.fn(),
		loadMore: vi.fn(),
		post: vi.fn(),
		reply: vi.fn(),
		update: vi.fn(),
		remove: vi.fn(),
	}),
	useForumReplies: (_store: string, threadId: string) => ({
		items: forum.replies[threadId]?.items ?? [],
		submitted: [],
		loading: false,
		hasMore: forum.replies[threadId]?.hasMore ?? false,
		loadingMore: false,
		error: null,
		loadMoreError: forum.replies[threadId]?.loadMoreError ?? null,
		loadMore: forum.loadMore,
		refresh: vi.fn(),
	}),
}));

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

function view() {
	render(
		React.createElement(
			IntlTestWrapper,
			null,
			React.createElement(SellerMessageTab, { sellerId: "7", routeId: "7" }),
		),
	);
}

afterEach(() => {
	cleanup();
	forum.loadMore.mockReset();
	forum.replies = {};
});

describe("forum placeholder", () => {
	it("a deleted entry shows the approved text, no author and no actions, replies still render", () => {
		forum.list = [
			entry("d", {
				state: "deleted",
				content: "",
				userName: "匿名",
				replyCount: 1,
			}),
		];
		forum.replies = { d: { items: [entry("r", { content: "reply-body" })] } };
		view();
		expect(screen.queryByText("此留言已刪除")).not.toBeNull();
		expect(screen.queryByText("匿名")).toBeNull();
		expect(screen.queryByText("回覆")).toBeNull();
		fireEvent.click(screen.getByText("1 則回覆 ▼"));
		expect(screen.queryByText("reply-body")).not.toBeNull();
	});
});

describe("forum hidden placeholder", () => {
	it("a hidden entry shows the hidden text, not the deleted text, with no author and no actions", () => {
		forum.list = [
			entry("h", { state: "hidden", content: "", userName: "匿名" }),
		];
		view();
		expect(screen.queryByText("此留言已被隱藏")).not.toBeNull();
		expect(screen.queryByText("此留言已刪除")).toBeNull();
		expect(screen.queryByText("匿名")).toBeNull();
		expect(screen.queryByText("回覆")).toBeNull();
	});
});

describe("forum replies load more", () => {
	const top = () => [entry("top", { replyCount: 30 })];

	it("shows only when the thread has another page and loads it", () => {
		forum.list = top();
		forum.replies = { top: { items: [entry("r1")], hasMore: false } };
		view();
		fireEvent.click(screen.getByText("30 則回覆 ▼"));
		expect(screen.queryByText("載入更多留言")).toBeNull();
		cleanup();
		forum.replies = { top: { items: [entry("r1")], hasMore: true } };
		view();
		fireEvent.click(screen.getByText("30 則回覆 ▼"));
		fireEvent.click(screen.getByText("載入更多留言"));
		expect(forum.loadMore).toHaveBeenCalledTimes(1);
	});

	it("shows a load-more failure inline as an alert and keeps the button", async () => {
		forum.list = top();
		forum.replies = {
			top: {
				items: [entry("r1")],
				hasMore: true,
				loadMoreError: new Error("boom"),
			},
		};
		view();
		fireEvent.click(screen.getByText("30 則回覆 ▼"));
		await waitFor(() => expect(screen.getByRole("alert")).toBeTruthy());
		expect(screen.getByRole("alert").textContent).not.toContain("boom");
		expect(screen.getByText("載入更多留言")).toBeTruthy();
	});
});
