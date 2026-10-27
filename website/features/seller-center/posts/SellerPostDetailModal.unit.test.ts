// @vitest-environment jsdom

import { cleanup, render, screen } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { PostListItem } from "@/lib/api/openapi-types";
import { PostPublishStatus } from "@/lib/codemeta/enums";

const postComments = vi.fn((_props: { postId: string | number }) => null);

vi.mock("@/features/feed/components/PostComments", () => ({
	default: (props: { postId: string | number }) => {
		postComments(props);
		return React.createElement("div", { "data-testid": "post-comments" });
	},
}));
vi.mock("@/components/shared/AppImage", () => ({ default: () => null }));
vi.mock("@/components/shared/Avatar", () => ({ default: () => null }));
vi.mock("@/components/shared/AppIcon", () => ({ default: () => null }));
vi.mock("@/features/feed/components/HashtagText", () => ({
	default: () => null,
}));
vi.mock("@/lib/browser/body-scroll-lock", () => ({
	acquireBodyScrollLock: () => () => {},
}));

import SellerPostDetailModal from "./SellerPostDetailModal";

function post(overrides: Partial<PostListItem> = {}): PostListItem {
	return {
		id: "42",
		sellerId: "7",
		storeCode: "",
		sellerName: "測試店家",
		logoKey: "",
		content: "內容",
		imageKeys: [],
		tags: [],
		product: null,
		commentCount: 0,
		likeCount: 0,
		shareCount: 0,
		viewCount: 0,
		isLiked: false,
		createdAt: 0,
		publishStatus: PostPublishStatus.PUBLISHED,
		...overrides,
	};
}

function renderModal(
	p: PostListItem,
	storePublic: boolean | undefined,
): ReturnType<typeof render> {
	return render(
		React.createElement(SellerPostDetailModal, {
			open: true,
			post: p,
			storePublic,
			onClose: () => {},
		}),
	);
}

afterEach(() => {
	cleanup();
	postComments.mockClear();
});

describe("SellerPostDetailModal comments", () => {
	it("a store that cannot go public explains it and never mounts the public thread", async () => {
		renderModal(post(), false);
		expect(
			await screen.findByText("店家公開後，買家的留言會顯示在這裡。"),
		).toBeTruthy();
		expect(postComments).not.toHaveBeenCalled();
	});

	for (const publishStatus of [
		PostPublishStatus.DRAFT,
		PostPublishStatus.UNPUBLISHED,
	]) {
		it(`an unpublished post (${publishStatus}) explains it needs publishing`, async () => {
			renderModal(post({ publishStatus }), true);
			expect(
				await screen.findByText("動態發布後，買家的留言會顯示在這裡。"),
			).toBeTruthy();
			expect(postComments).not.toHaveBeenCalled();
		});
	}

	it("a platform-hidden post says buyers cannot see comments", async () => {
		renderModal(post({ hiddenAt: 1760000000 }), true);
		expect(
			await screen.findByText("這則動態已被平台下架，買家看不到留言。"),
		).toBeTruthy();
		expect(postComments).not.toHaveBeenCalled();
	});

	it("a draft that is also platform-hidden shows the platform-hidden copy", async () => {
		renderModal(
			post({ publishStatus: PostPublishStatus.DRAFT, hiddenAt: 1760000000 }),
			true,
		);
		expect(
			await screen.findByText("這則動態已被平台下架，買家看不到留言。"),
		).toBeTruthy();
		expect(postComments).not.toHaveBeenCalled();
	});

	it("a platform-hidden post of a non-public store shows the platform-hidden copy", async () => {
		renderModal(post({ hiddenAt: 1760000000 }), false);
		expect(
			await screen.findByText("這則動態已被平台下架，買家看不到留言。"),
		).toBeTruthy();
		expect(postComments).not.toHaveBeenCalled();
	});

	it("a public post mounts the public thread as before", async () => {
		renderModal(post(), true);
		expect(await screen.findByTestId("post-comments")).toBeTruthy();
		expect(postComments).toHaveBeenCalledWith(
			expect.objectContaining({ postId: "42" }),
		);
	});
});
