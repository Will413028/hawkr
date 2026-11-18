/** @vitest-environment jsdom */

import { cleanup, render, screen } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

const comments = vi.hoisted(() => ({ list: [] as unknown[] }));

vi.mock("@/lib/auth/context", () => ({
	useAuth: () => ({ status: "signedIn", identity: { nickname: "我" } }),
}));
vi.mock("@/lib/hooks/useSocial", () => ({
	usePostComments: () => ({
		comments: comments.list,
		submitted: [],
		loading: false,
		loadingMore: false,
		hasMore: false,
		error: null,
		loadMoreError: null,
		refresh: vi.fn(),
		loadMore: vi.fn(),
		post: vi.fn(),
		remove: vi.fn(),
		edited: vi.fn(),
		like: vi.fn(),
	}),
	usePostViewerState: () => ({ data: { canComment: true } }),
	useCommentReplies: () => ({
		submitted: [],
		reply: vi.fn(),
		removed: vi.fn(),
		items: [],
		open: false,
		loading: false,
		expand: vi.fn(),
		collapse: vi.fn(),
		refresh: vi.fn(),
	}),
}));

import { IntlTestWrapper } from "@/tests/mocks/intl-test-wrapper";
import PostComments from "../PostComments";

function comment(state: "visible" | "deleted" | "hidden") {
	return {
		id: `c-${state}`,
		postId: "p",
		isMine: true,
		userName: "u",
		content: "x",
		likeCount: 0,
		replyCount: 0,
		createdAt: 0,
		state,
		viewerCanEdit: true,
		viewerCanDelete: true,
		viewerCanReply: true,
	};
}

afterEach(cleanup);

describe("placeholder comments carry no actions", () => {
	it("a visible comment offers like, reply, edit and delete", () => {
		comments.list = [comment("visible")];
		render(
			React.createElement(
				IntlTestWrapper,
				null,
				React.createElement(PostComments, { postId: "p" }),
			),
		);
		expect(screen.queryByText("回覆")).not.toBeNull();
		expect(screen.queryByText("編輯")).not.toBeNull();
		expect(screen.queryByText("刪除")).not.toBeNull();
		expect(screen.queryByText("按讚")).not.toBeNull();
	});

	it.each(["deleted", "hidden"] as const)(
		"a %s comment shows none of them",
		(state) => {
			comments.list = [comment(state)];
			render(
				React.createElement(
					IntlTestWrapper,
					null,
					React.createElement(PostComments, { postId: "p" }),
				),
			);
			expect(screen.queryByText("回覆")).toBeNull();
			expect(screen.queryByText("編輯")).toBeNull();
			expect(screen.queryByText("刪除")).toBeNull();
			expect(screen.queryByText("按讚")).toBeNull();
		},
	);
});

describe("deleted placeholder text", () => {
	it("a deleted comment says it was deleted and a hidden one says it was hidden", () => {
		comments.list = [comment("deleted")];
		render(
			React.createElement(
				IntlTestWrapper,
				null,
				React.createElement(PostComments, { postId: "p" }),
			),
		);
		expect(screen.queryByText("此留言已刪除")).not.toBeNull();
		expect(screen.queryByText("x")).toBeNull();
		expect(screen.queryByText("u")).toBeNull();
		cleanup();
		comments.list = [comment("hidden")];
		render(
			React.createElement(
				IntlTestWrapper,
				null,
				React.createElement(PostComments, { postId: "p" }),
			),
		);
		expect(screen.queryByText("此留言已刪除")).toBeNull();
		expect(screen.queryByText("此留言已被隱藏")).not.toBeNull();
	});
});
