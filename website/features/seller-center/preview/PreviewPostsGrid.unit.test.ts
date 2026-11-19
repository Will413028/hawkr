/** @vitest-environment jsdom */

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import type { PostListItem } from "@/lib/api/openapi-types";

vi.mock("@/components/shared/AppImage", () => ({
	default: () => React.createElement("span"),
}));
vi.mock("@/components/shared/Avatar", () => ({ default: () => null }));
vi.mock("@/components/shared/Link", () => ({
	default: (props: React.AnchorHTMLAttributes<HTMLAnchorElement>) =>
		React.createElement("a", props),
}));

vi.mock("@/lib/auth/context", async (importOriginal) => ({
	...(await importOriginal<typeof import("@/lib/auth/context")>()),
	useAuth: () => ({
		status: "signedOut",
		identity: null,
		activeRole: null,
		hasSeller: false,
	}),
}));

import { PreviewPostsGrid } from "./PreviewPostsTab";

class ResizeObserverStub {
	observe() {}
	unobserve() {}
	disconnect() {}
}

beforeAll(() => {
	window.matchMedia = ((query: string) => ({
		matches: false,
		media: query,
		addEventListener: () => undefined,
		removeEventListener: () => undefined,
		addListener: () => undefined,
		removeListener: () => undefined,
		onchange: null,
		dispatchEvent: () => false,
	})) as typeof window.matchMedia;
	vi.stubGlobal("ResizeObserver", ResizeObserverStub);
});

function post(id: string): PostListItem {
	return {
		id,
		sellerId: "700",
		storeCode: "123456",
		sellerName: "預覽店",
		logoKey: "",
		content: `貼文內容${id}`,
		imageKeys: [],
		tags: [],
		commentCount: 3,
		likeCount: 5,
		shareCount: 0,
		viewCount: 0,
		isLiked: false,
		createdAt: 1_700_000_000,
	} as unknown as PostListItem;
}

function mount(posts: PostListItem[]) {
	const client = new QueryClient();
	return render(
		React.createElement(
			QueryClientProvider,
			{ client },
			React.createElement(PreviewPostsGrid, {
				posts,
				hasMore: false,
				loadingMore: false,
				onLoadMore: () => undefined,
			}),
		),
	);
}

afterEach(cleanup);

describe("preview posts are structurally read-only", () => {
	it("cards render no like control and the opened viewer renders no write controls", () => {
		const { container } = mount([post("1"), post("2")]);

		expect(screen.queryByLabelText("按讚")).toBeNull();
		expect(screen.queryByLabelText("取消按讚")).toBeNull();
		expect(container.querySelector(".hkui-feed-card__like")).not.toBeNull();
		expect(container.querySelector(".hkui-feed-card__like button")).toBeNull();

		const card = container.querySelector(".hkui-feed-card");
		expect(card).not.toBeNull();
		fireEvent.click(card as Element);

		const dialog = screen.getByRole("dialog");
		expect(dialog.textContent).toContain("貼文內容1");
		expect(screen.queryByLabelText("按讚")).toBeNull();
		expect(screen.queryByLabelText("分享")).toBeNull();
		expect(screen.queryByLabelText(/^留言 /)).toBeNull();
		expect(document.querySelector("textarea")).toBeNull();
		expect(document.querySelector("input")).toBeNull();
		expect(document.querySelector(".post-viewer__inline-comments")).toBeNull();
		for (const label of ["編輯", "刪除", "封鎖", "回覆"]) {
			expect(screen.queryByText(label)).toBeNull();
		}
	});

	it("shows the empty state when the store has no published posts", () => {
		mount([]);
		expect(screen.getByText("還沒有貼文")).toBeTruthy();
	});
});
