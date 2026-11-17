// @vitest-environment jsdom

import { cleanup, render, screen } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { PostListItem } from "@/lib/api/openapi-types";
import { PostPublishStatus } from "@/lib/codemeta/enums";
import { IntlTestWrapper } from "@/tests/mocks/intl-test-wrapper";

const postComments = vi.fn(
	(_props: { postId: string | number; [key: string]: unknown }) => null,
);

vi.mock("@/features/feed/components/PostComments", () => ({
	default: (props: { postId: string | number; [key: string]: unknown }) => {
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
		React.createElement(
			IntlTestWrapper,
			null,
			React.createElement(SellerPostDetailModal, {
				open: true,
				post: p,
				storePublic,
				onClose: () => {},
			}),
		),
	);
}

afterEach(() => {
	cleanup();
	postComments.mockClear();
});

const STORE_NOT_PUBLIC = "店家目前不公開，買家看不到這些留言。";
const UNPUBLISHED = "這則動態已下架，買家看不到這些留言。";
const TAKEDOWN = "這則動態已被平台下架，買家看不到留言。";

describe("SellerPostDetailModal comments", () => {
	it("a store that cannot go public still loads the thread and explains why buyers cannot see it", async () => {
		renderModal(post(), false);
		expect(await screen.findByText(STORE_NOT_PUBLIC)).toBeTruthy();
		expect(postComments).toHaveBeenCalledWith(
			expect.objectContaining({ postId: "42" }),
		);
	});

	it("an owner-unpublished post still loads the thread and says it is unpublished", async () => {
		renderModal(post({ publishStatus: PostPublishStatus.UNPUBLISHED }), true);
		expect(await screen.findByText(UNPUBLISHED)).toBeTruthy();
		expect(postComments).toHaveBeenCalledWith(
			expect.objectContaining({ postId: "42" }),
		);
	});

	it("a platform takedown (measure id) still loads the thread and says so", async () => {
		renderModal(post({ takedownMeasureId: "m-1" }), true);
		expect(await screen.findByText(TAKEDOWN)).toBeTruthy();
		expect(postComments).toHaveBeenCalledWith(
			expect.objectContaining({ postId: "42" }),
		);
	});

	it("a takedown of a post in a non-public store shows the takedown line only", async () => {
		renderModal(post({ takedownMeasureId: "m-1" }), false);
		expect(await screen.findByText(TAKEDOWN)).toBeTruthy();
		expect(screen.queryByText(STORE_NOT_PUBLIC)).toBeNull();
	});

	it("a public published post loads the thread with no notice", async () => {
		renderModal(post(), true);
		expect(await screen.findByTestId("post-comments")).toBeTruthy();
		expect(screen.queryByText(STORE_NOT_PUBLIC)).toBeNull();
		expect(screen.queryByText(UNPUBLISHED)).toBeNull();
		expect(screen.queryByText(TAKEDOWN)).toBeNull();
	});

	it("hands the server's viewer state to the thread and offers blocking", async () => {
		const viewer = {
			canComment: true,
			actor: { kind: "store" as const, id: "7", name: "測試店家" },
		};
		renderModal(post({ viewer }), true);
		await screen.findByTestId("post-comments");
		expect(postComments).toHaveBeenCalledWith(
			expect.objectContaining({ viewer, allowBlockAuthor: true }),
		);
	});
});
