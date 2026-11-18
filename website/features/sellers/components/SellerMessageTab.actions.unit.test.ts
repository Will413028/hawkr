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
	update: vi.fn(),
	remove: vi.fn(),
}));
const blockBuyerByForumEntry = vi.hoisted(() => vi.fn());

vi.mock("@/lib/auth/context", () => ({
	useAuth: () => ({ status: "signedIn", identity: { nickname: "我" } }),
}));
vi.mock("@/lib/seller/block-buyer", () => ({ blockBuyerByForumEntry }));
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
		loadMoreReplies: vi.fn(),
		post: vi.fn(),
		reply: vi.fn(),
		update: forum.update,
		remove: forum.remove,
	}),
}));

import { IntlTestWrapper } from "@/tests/mocks/intl-test-wrapper";
import SellerMessageTab from "./SellerMessageTab";

function entry(id: string, over: Record<string, unknown> = {}) {
	return {
		id,
		storeId: "7",
		isMine: false,
		userName: `user-${id}`,
		authorKind: "buyer",
		authorId: `buyer-${id}`,
		content: `body-${id}`,
		createdAt: 0,
		etag: `e-${id}`,
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
	forum.update.mockReset();
	forum.remove.mockReset();
	blockBuyerByForumEntry.mockReset();
	vi.restoreAllMocks();
});

describe("forum entry actions follow the server flags", () => {
	it("edit and delete show only where the flags say, on top-level entries and replies", () => {
		forum.list = [
			entry("a", { viewerCanEdit: true, viewerCanDelete: true }),
			entry("b"),
			entry("c", {
				replyCount: 1,
				replies: [entry("r", { viewerCanEdit: true, viewerCanDelete: true })],
			}),
		];
		view();
		expect(screen.queryAllByText("編輯")).toHaveLength(1);
		expect(screen.queryAllByText("刪除")).toHaveLength(1);
		fireEvent.click(screen.getByText("1 則回覆 ▼"));
		expect(screen.queryAllByText("編輯")).toHaveLength(2);
		expect(screen.queryAllByText("刪除")).toHaveLength(2);
	});

	it("a placeholder row offers no actions even with flags set", () => {
		forum.list = [
			entry("d", {
				state: "deleted",
				content: "",
				viewerCanEdit: true,
				viewerCanDelete: true,
			}),
		];
		view();
		expect(screen.queryByText("編輯")).toBeNull();
		expect(screen.queryByText("刪除")).toBeNull();
	});

	it("deleting asks first and removes by id; declining removes nothing", async () => {
		forum.list = [entry("a", { viewerCanDelete: true })];
		forum.remove.mockResolvedValue(undefined);
		const confirm = vi.spyOn(window, "confirm").mockReturnValue(false);
		view();
		fireEvent.click(screen.getByText("刪除"));
		expect(confirm).toHaveBeenCalledWith("刪除這則留言？");
		expect(forum.remove).not.toHaveBeenCalled();
		confirm.mockReturnValue(true);
		fireEvent.click(screen.getByText("刪除"));
		await waitFor(() => expect(forum.remove).toHaveBeenCalledWith("a"));
	});

	it("editing saves the trimmed text by id", async () => {
		forum.list = [entry("a", { viewerCanEdit: true })];
		forum.update.mockResolvedValue(undefined);
		view();
		fireEvent.click(screen.getByText("編輯"));
		fireEvent.change(screen.getByDisplayValue("body-a"), {
			target: { value: "  new text " },
		});
		fireEvent.click(screen.getByText("儲存"));
		await waitFor(() =>
			expect(forum.update).toHaveBeenCalledWith("a", "new text"),
		);
	});
});

describe("block this person (storefront forum)", () => {
	it("is offered on a buyer's entry the owner can delete, not on own, store-authored or undeletable entries", () => {
		forum.list = [
			entry("a", { viewerCanDelete: true }),
			entry("b", { viewerCanDelete: true, isMine: true }),
			entry("c", { viewerCanDelete: true, authorKind: "store" }),
			entry("d"),
		];
		view();
		const rows = screen
			.queryAllByText("封鎖此人")
			.map((el) => el.closest("li")?.textContent ?? "");
		expect(rows).toHaveLength(1);
		expect(rows[0]).toContain("user-a");
	});

	it("asks, then blocks by forum entry id and reports it", async () => {
		forum.list = [entry("a", { viewerCanDelete: true })];
		blockBuyerByForumEntry.mockResolvedValue({ changed: true });
		const confirm = vi.spyOn(window, "confirm").mockReturnValue(true);
		view();
		fireEvent.click(screen.getByText("封鎖此人"));
		expect(String(confirm.mock.calls[0]?.[0])).toContain("封鎖「user-a」？");
		expect(blockBuyerByForumEntry).toHaveBeenCalledExactlyOnceWith("a");
		expect(await screen.findByText("已封鎖「user-a」")).toBeTruthy();
	});

	it("declining blocks nobody, and a failure shows inline", async () => {
		forum.list = [entry("a", { viewerCanDelete: true })];
		const confirm = vi.spyOn(window, "confirm").mockReturnValue(false);
		view();
		fireEvent.click(screen.getByText("封鎖此人"));
		expect(blockBuyerByForumEntry).not.toHaveBeenCalled();
		confirm.mockReturnValue(true);
		blockBuyerByForumEntry.mockRejectedValue(new Error("raw backend text"));
		fireEvent.click(screen.getByText("封鎖此人"));
		const box = await screen.findByRole("alert");
		expect(box.textContent).not.toContain("raw backend text");
		expect(box.textContent).not.toBe("");
	});
});

describe("edit and delete failures", () => {
	it("an edit failure shows the localized error, not the raw message, and keeps the editor open", async () => {
		forum.list = [entry("a", { viewerCanEdit: true })];
		forum.update.mockRejectedValue(new Error("raw backend text"));
		view();
		fireEvent.click(screen.getByText("編輯"));
		fireEvent.change(screen.getByDisplayValue("body-a"), {
			target: { value: "changed" },
		});
		fireEvent.click(screen.getByText("儲存"));
		const box = await screen.findByRole("alert");
		expect(box.textContent).not.toContain("raw backend text");
		expect(box.textContent).not.toBe("");
		expect(screen.getByDisplayValue("changed")).toBeTruthy();
	});

	it("a delete failure shows the localized error, not the raw message", async () => {
		forum.list = [entry("a", { viewerCanDelete: true })];
		forum.remove.mockRejectedValue(new Error("raw backend text"));
		vi.spyOn(window, "confirm").mockReturnValue(true);
		view();
		fireEvent.click(screen.getByText("刪除"));
		const box = await screen.findByRole("alert");
		expect(box.textContent).not.toContain("raw backend text");
		expect(box.textContent).not.toBe("");
	});

	it("error boxes use the theme-aware danger tokens, no raw colors", async () => {
		forum.list = [entry("a", { viewerCanDelete: true })];
		forum.remove.mockRejectedValue(new Error("x"));
		vi.spyOn(window, "confirm").mockReturnValue(true);
		view();
		fireEvent.click(screen.getByText("刪除"));
		const cls = (await screen.findByRole("alert")).className;
		expect(cls).toContain("--hkui-danger-text");
		expect(cls).toContain("dark:");
		expect(cls).not.toMatch(/#[0-9a-f]{3,6}|rgba\(/i);
	});
});

describe("the inline editor", () => {
	it("focuses its textarea when it opens", () => {
		forum.list = [entry("a", { viewerCanEdit: true })];
		view();
		fireEvent.click(screen.getByText("編輯"));
		expect(document.activeElement).toBe(screen.getByDisplayValue("body-a"));
	});

	it("each row keeps its own editor: opening another does not discard an unsaved draft", () => {
		forum.list = [
			entry("a", { viewerCanEdit: true }),
			entry("b", { viewerCanEdit: true }),
		];
		view();
		fireEvent.click(screen.getAllByText("編輯")[0]);
		fireEvent.change(screen.getByDisplayValue("body-a"), {
			target: { value: "draft-a" },
		});
		fireEvent.click(screen.getByText("編輯"));
		expect(screen.getByDisplayValue("draft-a")).toBeTruthy();
		expect(screen.getByDisplayValue("body-b")).toBeTruthy();
	});

	it("a late success from a cancelled edit does not close the reopened editor", async () => {
		forum.list = [entry("a", { viewerCanEdit: true })];
		let resolve: () => void = () => undefined;
		forum.update.mockReturnValue(
			new Promise<void>((r) => {
				resolve = r;
			}),
		);
		view();
		fireEvent.click(screen.getByText("編輯"));
		fireEvent.change(screen.getByDisplayValue("body-a"), {
			target: { value: "first" },
		});
		fireEvent.click(screen.getByText("儲存"));
		fireEvent.click(screen.getByText("取消"));
		fireEvent.click(screen.getByText("編輯"));
		fireEvent.change(screen.getByDisplayValue("body-a"), {
			target: { value: "second" },
		});
		resolve();
		await waitFor(() => expect(forum.update).toHaveBeenCalledTimes(1));
		await new Promise((r) => setTimeout(r, 0));
		expect(screen.getByDisplayValue("second")).toBeTruthy();
	});
});

describe("after blocking someone", () => {
	it("their other entries no longer offer the block action; other authors still do", async () => {
		forum.list = [
			entry("a", { viewerCanDelete: true, authorId: "same" }),
			entry("b", { viewerCanDelete: true, authorId: "same" }),
			entry("c", { viewerCanDelete: true, authorId: "other" }),
		];
		blockBuyerByForumEntry.mockResolvedValue({ changed: true });
		vi.spyOn(window, "confirm").mockReturnValue(true);
		view();
		expect(screen.getAllByText("封鎖此人")).toHaveLength(3);
		fireEvent.click(screen.getAllByText("封鎖此人")[0]);
		await screen.findByText("已封鎖「user-a」");
		const rows = screen
			.getAllByText("封鎖此人")
			.map((el) => el.closest("li")?.textContent ?? "");
		expect(rows).toHaveLength(1);
		expect(rows[0]).toContain("user-c");
	});
});
