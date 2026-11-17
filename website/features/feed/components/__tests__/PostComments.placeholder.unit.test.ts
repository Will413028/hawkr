/** @vitest-environment jsdom */

import { cleanup, render, screen } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

const comments = vi.hoisted(() => ({ list: [] as unknown[] }));

vi.mock("@/lib/auth/context", () => ({
	useAuth: () => ({ status: "signedIn", identity: { nickname: "我" } }),
}));
vi.mock("@/components/shared/useSellerIdentity", () => ({
	useSellerIdentity: () => ({ avatar: null, storeName: "" }),
}));
vi.mock("@/lib/hooks/useSocial", () => ({
	usePostComments: () => ({
		comments: comments.list,
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
	useCommentReplies: () => ({
		items: [],
		open: false,
		loading: false,
		expand: vi.fn(),
		collapse: vi.fn(),
		refresh: vi.fn(),
	}),
}));

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
			React.createElement(PostComments, { postId: "p", canModerate: true }),
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
				React.createElement(PostComments, { postId: "p", canModerate: true }),
			);
			expect(screen.queryByText("回覆")).toBeNull();
			expect(screen.queryByText("編輯")).toBeNull();
			expect(screen.queryByText("刪除")).toBeNull();
			expect(screen.queryByText("按讚")).toBeNull();
		},
	);
});
