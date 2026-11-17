/** @vitest-environment jsdom */

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

const forum = vi.hoisted(() => ({
	list: [] as unknown[],
	loadMoreReplies: vi.fn(),
}));

vi.mock("@/lib/auth/context", () => ({
	useAuth: () => ({ status: "signedIn", identity: { nickname: "我" } }),
}));
vi.mock("@/lib/hooks/useSellerForum", () => ({
	useSellerForum: () => ({
		comments: forum.list,
		loading: false,
		refreshing: false,
		loadingMore: false,
		hasMore: false,
		error: null,
		refresh: vi.fn(),
		loadMore: vi.fn(),
		loadReplies: vi.fn(),
		loadMoreReplies: forum.loadMoreReplies,
		post: vi.fn(),
		reply: vi.fn(),
		update: vi.fn(),
		remove: vi.fn(),
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
	forum.loadMoreReplies.mockReset();
});

describe("forum placeholder", () => {
	it("a deleted entry shows the approved text, no author and no actions, replies still render", () => {
		forum.list = [
			entry("d", {
				state: "deleted",
				content: "",
				userName: "匿名",
				replyCount: 1,
				replies: [entry("r", { content: "reply-body" })],
			}),
		];
		view();
		expect(screen.queryByText("此留言已刪除")).not.toBeNull();
		expect(screen.queryByText("匿名")).toBeNull();
		expect(screen.queryByText("回覆")).toBeNull();
		fireEvent.click(screen.getByText("1 則回覆 ▼"));
		expect(screen.queryByText("reply-body")).not.toBeNull();
	});
});

describe("forum replies load more", () => {
	const withReplies = (token: string) => [
		entry("top", {
			replyCount: 30,
			replies: [entry("r1")],
			repliesNextPageToken: token,
		}),
	];

	it("shows only with a next token and loads the next page", () => {
		forum.list = withReplies("");
		view();
		fireEvent.click(screen.getByText("30 則回覆 ▼"));
		expect(screen.queryByText("載入更多留言")).toBeNull();
		cleanup();
		forum.list = withReplies("tok");
		view();
		fireEvent.click(screen.getByText("30 則回覆 ▼"));
		fireEvent.click(screen.getByText("載入更多留言"));
		expect(forum.loadMoreReplies).toHaveBeenCalledWith("top");
	});
});
