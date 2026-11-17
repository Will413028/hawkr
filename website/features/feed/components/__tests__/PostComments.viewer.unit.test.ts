/** @vitest-environment jsdom */

import {
	act,
	cleanup,
	fireEvent,
	render,
	screen,
} from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { PostViewerState } from "@/lib/api/openapi-types";

const state = vi.hoisted(() => ({
	comments: [] as unknown[],
	fetched: undefined as unknown,
	viewerEnabled: [] as boolean[],
	viewerError: null as unknown,
}));
const refetchViewer = vi.hoisted(() => vi.fn());
const blockBuyerByComment = vi.hoisted(() => vi.fn());

vi.mock("@/lib/auth/context", () => ({
	useAuth: () => ({
		status: "signedIn",
		identity: { nickname: "會員小明", avatarKey: "m.png" },
	}),
}));
vi.mock("@/lib/seller/block-buyer", () => ({ blockBuyerByComment }));
vi.mock("@/lib/hooks/useSocial", () => ({
	usePostComments: () => ({
		comments: state.comments,
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
	usePostViewerState: (_postId: string, enabled: boolean) => {
		state.viewerEnabled.push(enabled);
		return {
			data: state.fetched,
			isError: state.viewerError !== null,
			error: state.viewerError,
			isFetching: false,
			refetch: refetchViewer,
		};
	},
	useCommentReplies: () => ({
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

function comment(id: string, flags: Record<string, boolean> = {}) {
	return {
		id,
		postId: "p",
		isMine: false,
		userName: `買家${id}`,
		content: `內容${id}`,
		likeCount: 0,
		replyCount: 0,
		createdAt: 0,
		state: "visible",
		viewerCanEdit: false,
		viewerCanDelete: false,
		viewerCanReply: false,
		...flags,
	};
}

const STORE: PostViewerState = {
	canComment: true,
	actor: { kind: "store", id: "7", name: "小店" },
};
const BUYER: PostViewerState = { canComment: true };

function mount(props: Record<string, unknown> = {}) {
	return render(
		React.createElement(
			IntlTestWrapper,
			null,
			React.createElement(PostComments, { postId: "p", ...props }),
		),
	);
}

beforeEach(() => {
	state.comments = [];
	state.fetched = undefined;
	state.viewerEnabled = [];
	state.viewerError = null;
	refetchViewer.mockReset();
	blockBuyerByComment.mockReset();
});
afterEach(cleanup);

describe("row actions follow the server's viewer_can_* flags", () => {
	it("delete, edit and reply each appear only on the rows whose flag is set", () => {
		state.comments = [
			comment("1", { viewerCanDelete: true }),
			comment("2", { viewerCanEdit: true }),
			comment("3", { viewerCanReply: true }),
			comment("4"),
			// 自己寫的留言，伺服器卻沒給任何權限：畫面不自己補（不能因為 isMine 就顯示）。
			comment("5", { isMine: true }),
		];
		mount({ viewer: BUYER });
		const rows = (label: string) =>
			screen
				.queryAllByText(label)
				.map((el) => el.closest("li")?.getAttribute("data-comment-anchor"));
		expect(rows("刪除")).toEqual(["1"]);
		expect(rows("編輯")).toEqual(["2"]);
		expect(rows("回覆")).toEqual(["3"]);
	});

	it("the same flags give the same actions on the public page and in the seller center", () => {
		state.comments = [comment("1", { viewerCanDelete: true })];
		mount({ viewer: STORE });
		expect(screen.queryAllByText("刪除")).toHaveLength(1);
		cleanup();
		mount({ viewer: STORE, allowBlockAuthor: true });
		expect(screen.queryAllByText("刪除")).toHaveLength(1);
	});
});

describe("composer", () => {
	it("is hidden when the server says the viewer cannot comment", () => {
		mount({ viewer: { canComment: false, actor: STORE.actor } });
		expect(screen.queryByPlaceholderText(/留言/)).toBeNull();
	});

	it("is hidden until the viewer state is known", () => {
		mount();
		expect(screen.queryByPlaceholderText(/留言/)).toBeNull();
	});

	it("uses the fetched viewer state when the post did not carry one", () => {
		state.fetched = STORE;
		mount();
		expect(screen.getByPlaceholderText("以「小店」身分留言…")).toBeTruthy();
		expect(state.viewerEnabled.at(-1)).toBe(true);
	});

	it("shows an error with a retry when the viewer state cannot be read, and retry refetches", () => {
		state.viewerError = new Error("boom");
		mount();
		expect(screen.getByRole("alert")).toBeTruthy();
		expect(screen.queryByPlaceholderText(/留言/)).toBeNull();
		fireEvent.click(screen.getByText("重試"));
		expect(refetchViewer).toHaveBeenCalledOnce();
	});

	it("shows no error while the viewer state is merely loading", () => {
		mount();
		expect(screen.queryByRole("alert")).toBeNull();
	});

	it("does not fetch the viewer state when the post already carries it", () => {
		mount({ viewer: BUYER });
		expect(state.viewerEnabled.at(-1)).toBe(false);
	});

	it("a store actor comments as the store", () => {
		mount({ viewer: STORE });
		expect(screen.getByPlaceholderText("以「小店」身分留言…")).toBeTruthy();
	});

	it("a buyer actor keeps the buyer placeholder", () => {
		mount({ viewer: { canComment: true, actor: { kind: "buyer", id: "9" } } });
		expect(screen.getByPlaceholderText("寫下你的留言…")).toBeTruthy();
	});

	it("the screen does not decide the identity: a store actor on the seller screen and a buyer actor there differ", () => {
		mount({ viewer: BUYER, allowBlockAuthor: true });
		expect(screen.getByPlaceholderText("寫下你的留言…")).toBeTruthy();
	});
});

describe("reply composer", () => {
	it("a store actor replies as the store", () => {
		state.comments = [comment("1", { viewerCanReply: true })];
		mount({ viewer: STORE });
		fireEvent.click(screen.getByText("回覆"));
		expect(screen.getByPlaceholderText("以「小店」身分回覆…")).toBeTruthy();
	});

	it("otherwise the placeholder names who is being replied to", () => {
		state.comments = [comment("1", { viewerCanReply: true })];
		mount({ viewer: BUYER });
		fireEvent.click(screen.getByText("回覆"));
		expect(screen.getByPlaceholderText("回覆 買家1…")).toBeTruthy();
	});
});

describe("block this person (seller center)", () => {
	it("is offered on another author's deletable comment, not on one's own or on a public page", () => {
		state.comments = [
			comment("1", { viewerCanDelete: true }),
			comment("2", { viewerCanDelete: true, isMine: true }),
			comment("3"),
		];
		mount({ viewer: STORE, allowBlockAuthor: true });
		const rows = screen
			.queryAllByText("封鎖此人")
			.map((el) => el.closest("li")?.getAttribute("data-comment-anchor"));
		expect(rows).toEqual(["1"]);
		cleanup();
		mount({ viewer: STORE });
		expect(screen.queryByText("封鎖此人")).toBeNull();
	});

	it("asks with the approved title and description, then blocks by comment id", async () => {
		state.comments = [comment("1", { viewerCanDelete: true })];
		blockBuyerByComment.mockResolvedValue({ changed: true });
		const confirm = vi.spyOn(window, "confirm").mockReturnValue(true);
		mount({ viewer: STORE, allowBlockAuthor: true });
		fireEvent.click(screen.getByText("封鎖此人"));
		expect(confirm).toHaveBeenCalledOnce();
		const text = String(confirm.mock.calls[0]?.[0]);
		expect(text).toContain("封鎖「買家1」？");
		expect(text).toContain(
			"封鎖後，對方將無法在你的貼文與討論區留言、按讚，也無法傳送訊息給你；對方仍看得到你公開的內容，評價不受影響。你可以隨時在「封鎖名單」解除封鎖。",
		);
		expect(blockBuyerByComment).toHaveBeenCalledExactlyOnceWith("1");
		expect(await screen.findByText("已封鎖「買家1」")).toBeTruthy();
		confirm.mockRestore();
	});

	it("declining the confirmation blocks nobody", () => {
		state.comments = [comment("1", { viewerCanDelete: true })];
		const confirm = vi.spyOn(window, "confirm").mockReturnValue(false);
		mount({ viewer: STORE, allowBlockAuthor: true });
		fireEvent.click(screen.getByText("封鎖此人"));
		expect(blockBuyerByComment).not.toHaveBeenCalled();
		confirm.mockRestore();
	});

	it("shows a failed block inline, not through window.alert, and not the raw message", async () => {
		state.comments = [comment("1", { viewerCanDelete: true })];
		blockBuyerByComment.mockRejectedValue(new Error("raw backend text"));
		const confirm = vi.spyOn(window, "confirm").mockReturnValue(true);
		const alert = vi.spyOn(window, "alert").mockImplementation(() => {});
		mount({ viewer: STORE, allowBlockAuthor: true });
		fireEvent.click(screen.getByText("封鎖此人"));
		const box = await screen.findByRole("alert");
		expect(box.textContent).not.toContain("raw backend text");
		expect(box.textContent).not.toBe("");
		expect(alert).not.toHaveBeenCalled();
		confirm.mockRestore();
		alert.mockRestore();
	});

	it("a second click while the block is in flight sends no second request", async () => {
		state.comments = [comment("1", { viewerCanDelete: true })];
		let resolve: (v: unknown) => void = () => {};
		blockBuyerByComment.mockReturnValue(
			new Promise((r) => {
				resolve = r;
			}),
		);
		const confirm = vi.spyOn(window, "confirm").mockReturnValue(true);
		mount({ viewer: STORE, allowBlockAuthor: true });
		const button = screen.getByText("封鎖此人") as HTMLButtonElement;
		fireEvent.click(button);
		fireEvent.click(button);
		expect(blockBuyerByComment).toHaveBeenCalledTimes(1);
		expect((screen.getByText("封鎖此人") as HTMLButtonElement).disabled).toBe(
			true,
		);
		resolve({ changed: true });
		await screen.findByText("已封鎖「買家1」");
		confirm.mockRestore();
	});

	it("two clicks in the same tick send one request even before the button re-renders disabled", async () => {
		state.comments = [comment("1", { viewerCanDelete: true })];
		let resolve: (v: unknown) => void = () => {};
		blockBuyerByComment.mockReturnValue(
			new Promise((r) => {
				resolve = r;
			}),
		);
		const confirm = vi.spyOn(window, "confirm").mockReturnValue(true);
		mount({ viewer: STORE, allowBlockAuthor: true });
		const button = screen.getByText("封鎖此人") as HTMLButtonElement;
		act(() => {
			button.click();
			button.click();
		});
		expect(blockBuyerByComment).toHaveBeenCalledTimes(1);
		resolve({ changed: true });
		await screen.findByText("已封鎖「買家1」");
		confirm.mockRestore();
	});
});
