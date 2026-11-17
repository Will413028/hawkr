/** @vitest-environment jsdom */

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

const state = vi.hoisted(() => ({
	hasMore: false,
	loadMore: vi.fn(),
	error: null as unknown,
}));

vi.mock("@/lib/auth/context", () => ({
	useAuth: () => ({ status: "signedIn", identity: { nickname: "我" } }),
}));
vi.mock("@/lib/hooks/useSocial", () => ({
	usePostComments: () => ({
		comments: [
			{
				id: "c1",
				postId: "p",
				isMine: false,
				userName: "u",
				content: "top",
				likeCount: 0,
				replyCount: 3,
				createdAt: 0,
				state: "visible",
				viewerCanEdit: false,
				viewerCanDelete: false,
				viewerCanReply: true,
			},
		],
		loading: false,
		loadingMore: false,
		hasMore: false,
		error: null,
		refresh: vi.fn(),
		loadMore: vi.fn(),
		post: vi.fn(),
		remove: vi.fn(),
		edited: vi.fn(),
		like: vi.fn(),
	}),
	usePostViewerState: () => ({ data: { canComment: true } }),
	useCommentReplies: () => ({
		items: [],
		open: true,
		loading: false,
		hasMore: state.hasMore,
		loadingMore: false,
		loadMoreError: state.error,
		loadMore: state.loadMore,
		expand: vi.fn(),
		collapse: vi.fn(),
		refresh: vi.fn(),
	}),
}));

import { IntlTestWrapper } from "@/tests/mocks/intl-test-wrapper";
import PostComments from "../PostComments";

function view() {
	render(
		React.createElement(
			IntlTestWrapper,
			null,
			React.createElement(PostComments, { postId: "p" }),
		),
	);
}

afterEach(() => {
	cleanup();
	state.loadMore.mockReset();
	state.error = null;
});

describe("reply load more", () => {
	it("shows the control only while a next page exists and calls loadMore", () => {
		state.hasMore = false;
		view();
		expect(screen.queryByText("載入更多")).toBeNull();
		cleanup();
		state.hasMore = true;
		view();
		fireEvent.click(screen.getByText("載入更多"));
		expect(state.loadMore).toHaveBeenCalledTimes(1);
	});

	it("shows the load-more failure inline as an alert and keeps the button", () => {
		state.hasMore = true;
		state.error = new Error("boom");
		view();
		expect(screen.getByRole("alert").textContent).toBeTruthy();
		fireEvent.click(screen.getByText("載入更多"));
		expect(state.loadMore).toHaveBeenCalledTimes(1);
	});
});
