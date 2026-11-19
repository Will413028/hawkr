/** @vitest-environment jsdom */

import {
	cleanup,
	fireEvent,
	render,
	screen,
	waitFor,
} from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const state = vi.hoisted(() => ({
	comments: [] as unknown[],
	submitted: [] as unknown[],
	replyItems: [] as unknown[],
	replySubmitted: [] as unknown[],
	post: vi.fn(),
	remove: vi.fn(),
	reply: vi.fn(),
	removed: vi.fn(),
	expand: vi.fn(),
}));
const repo = vi.hoisted(() => ({
	updateComment: vi.fn(),
	deleteComment: vi.fn(),
	setCommentLiked: vi.fn(),
}));

vi.mock("@/lib/auth/context", () => ({
	useAuth: () => ({ status: "signedIn", identity: { nickname: "我" } }),
}));
vi.mock("@/lib/comments/repo", () => repo);
vi.mock("@/lib/hooks/useSocial", () => ({
	usePostComments: () => ({
		comments: state.comments,
		submitted: state.submitted,
		loading: false,
		loadingMore: false,
		hasMore: false,
		error: null,
		loadMoreError: null,
		refresh: vi.fn(),
		loadMore: vi.fn(),
		post: state.post,
		remove: state.remove,
		edited: vi.fn(),
		like: vi.fn(),
	}),
	usePostViewerState: () => ({ data: { canComment: true } }),
	useCommentReplies: () => ({
		items: state.replyItems,
		submitted: state.replySubmitted,
		open: true,
		loading: false,
		hasMore: false,
		loadingMore: false,
		loadMoreError: null,
		loadMore: vi.fn(),
		expand: state.expand,
		collapse: vi.fn(),
		refresh: vi.fn(),
		reply: state.reply,
		removed: state.removed,
	}),
}));

import { IntlTestWrapper } from "@/tests/mocks/intl-test-wrapper";
import PostComments from "../PostComments";

function comment(id: string, over: Record<string, unknown> = {}) {
	return {
		id,
		postId: "p",
		isMine: false,
		userName: `user-${id}`,
		content: `body-${id}`,
		likeCount: 0,
		replyCount: 0,
		createdAt: 0,
		etag: `e-${id}`,
		state: "visible",
		viewerCanEdit: false,
		viewerCanDelete: false,
		viewerCanReply: false,
		...over,
	};
}

function view() {
	return render(
		React.createElement(
			IntlTestWrapper,
			null,
			React.createElement(PostComments, { postId: "p" }),
		),
	);
}

const optimistic = (id: string) =>
	comment(id, {
		optimistic: true,
		isMine: true,
		isLiked: false,
		viewerCanEdit: false,
		viewerCanDelete: false,
		viewerCanReply: false,
	});

beforeEach(() => {
	state.comments = [];
	state.submitted = [];
	state.replyItems = [];
	state.replySubmitted = [];
	for (const fn of [
		state.post,
		state.remove,
		state.reply,
		state.removed,
		state.expand,
		...Object.values(repo),
	]) {
		fn.mockReset();
	}
});
afterEach(() => {
	cleanup();
	vi.restoreAllMocks();
});

describe("optimistic rows carry no actions", () => {
	it("a comment in the list, one from the submitted state and optimistic replies offer no like, reply, edit or delete", () => {
		state.comments = [optimistic("tmp-a"), comment("top", { replyCount: 2 })];
		state.submitted = [{ key: "tmp-b", comment: optimistic("real-b") }];
		state.replyItems = [optimistic("tmp-r")];
		state.replySubmitted = [{ key: "tmp-s", comment: optimistic("real-s") }];
		view();
		for (const id of ["tmp-a", "real-b", "tmp-r", "real-s"]) {
			const row = screen.getByText(`body-${id}`).closest("li");
			expect(row?.querySelector("button")).toBeNull();
		}
		expect(screen.getByText("body-top").closest("li")).not.toBeNull();
	});

	it("shows submitted comments between the composer and the list, and submitted replies after the thread", () => {
		state.comments = [comment("top", { replyCount: 1 })];
		state.replyItems = [comment("r1")];
		state.submitted = [{ key: "tmp-b", comment: optimistic("real-b") }];
		state.replySubmitted = [{ key: "tmp-s", comment: optimistic("real-s") }];
		const { container } = view();
		const text = container.textContent ?? "";
		expect(text.indexOf("送出")).toBeLessThan(text.indexOf("body-real-b"));
		expect(text.indexOf("body-real-b")).toBeLessThan(text.indexOf("body-top"));
		expect(text.indexOf("body-r1")).toBeLessThan(text.indexOf("body-real-s"));
	});
});

describe("edit and delete failures use the localized error on the row", () => {
	it("a comment edit failure shows no window.alert and no raw message, on that row only", async () => {
		const alert = vi.spyOn(window, "alert").mockImplementation(() => undefined);
		state.comments = [
			comment("a", { viewerCanEdit: true }),
			comment("b", { viewerCanEdit: true }),
		];
		repo.updateComment.mockRejectedValue(new Error("raw backend text"));
		view();
		fireEvent.click(screen.getAllByText("編輯")[0]);
		fireEvent.change(screen.getByDisplayValue("body-a"), {
			target: { value: "changed" },
		});
		fireEvent.click(screen.getByText("儲存"));
		const box = await screen.findByRole("alert");
		expect(alert).not.toHaveBeenCalled();
		expect(box.textContent).not.toContain("raw backend text");
		expect(box.textContent).not.toBe("");
		expect(box.closest("li")?.textContent).toContain("user-a");
		expect(box.closest("li")?.textContent).not.toContain("user-b");
		expect(screen.getByDisplayValue("changed")).toBeTruthy();
	});

	it("a comment delete failure shows the error on the row, not window.alert", async () => {
		const alert = vi.spyOn(window, "alert").mockImplementation(() => undefined);
		vi.spyOn(window, "confirm").mockReturnValue(true);
		state.comments = [comment("a", { viewerCanDelete: true })];
		state.remove.mockRejectedValue(new Error("raw backend text"));
		view();
		fireEvent.click(screen.getByText("刪除"));
		const box = await screen.findByRole("alert");
		expect(alert).not.toHaveBeenCalled();
		expect(box.textContent).not.toContain("raw backend text");
		expect(box.closest("li")?.textContent).toContain("user-a");
	});

	it("a reply edit and delete failure show the error on the reply row", async () => {
		const alert = vi.spyOn(window, "alert").mockImplementation(() => undefined);
		vi.spyOn(window, "confirm").mockReturnValue(true);
		state.comments = [comment("top", { replyCount: 1 })];
		state.replyItems = [
			comment("r1", { viewerCanEdit: true, viewerCanDelete: true }),
		];
		repo.updateComment.mockRejectedValue(new Error("raw backend text"));
		repo.deleteComment.mockRejectedValue(new Error("raw backend text"));
		view();
		fireEvent.click(screen.getByText("刪除"));
		await waitFor(() => expect(screen.getByRole("alert")).toBeTruthy());
		expect(screen.getByRole("alert").closest("li")?.textContent).toContain(
			"user-r1",
		);
		fireEvent.click(screen.getByText("編輯"));
		fireEvent.change(screen.getByDisplayValue("body-r1"), {
			target: { value: "changed" },
		});
		fireEvent.click(screen.getByText("儲存"));
		await waitFor(() => expect(repo.updateComment).toHaveBeenCalled());
		expect(alert).not.toHaveBeenCalled();
		for (const box of screen.getAllByRole("alert")) {
			expect(box.textContent).not.toContain("raw backend text");
		}
	});

	it("a submit failure is shown with the localized error instead of an unhandled rejection", async () => {
		state.post.mockRejectedValue(new Error("raw backend text"));
		view();
		fireEvent.change(screen.getByRole("textbox"), {
			target: { value: "新留言" },
		});
		fireEvent.click(screen.getByText("送出"));
		const box = await screen.findByRole("alert");
		expect(box.textContent).not.toContain("raw backend text");
		expect(box.textContent).not.toBe("");
	});
});

describe("reply composer", () => {
	it("sends through the hook, closes and opens the thread", async () => {
		state.comments = [comment("top", { viewerCanReply: true })];
		state.reply.mockResolvedValue(undefined);
		view();
		fireEvent.click(screen.getByText("回覆"));
		fireEvent.change(screen.getAllByRole("textbox")[1], {
			target: { value: "  回覆內容 " },
		});
		fireEvent.click(screen.getByText("回覆"));
		await waitFor(() =>
			expect(state.reply).toHaveBeenCalledWith(
				"回覆內容",
				expect.objectContaining({
					replyToId: "top",
					replyToName: "user-top",
				}),
			),
		);
		await waitFor(() => expect(screen.getAllByRole("textbox")).toHaveLength(1));
	});

	it("shows a failed send inside the composer and keeps the draft", async () => {
		state.comments = [comment("top", { viewerCanReply: true })];
		state.reply.mockRejectedValue(new Error("raw backend text"));
		view();
		fireEvent.click(screen.getByText("回覆"));
		fireEvent.change(screen.getAllByRole("textbox")[1], {
			target: { value: "回覆內容" },
		});
		fireEvent.click(screen.getByText("回覆"));
		const box = await screen.findByRole("alert");
		expect(box.textContent).not.toContain("raw backend text");
		expect(screen.getByDisplayValue("回覆內容")).toBeTruthy();
	});
});

describe("comment count header", () => {
	it("counts the submitted rows shown outside the list", () => {
		state.comments = [comment("c1")];
		state.submitted = [{ key: "tmp-x", comment: optimistic("real-x") }];
		view();
		expect(screen.getByText("留言（2）")).toBeTruthy();
	});
});
