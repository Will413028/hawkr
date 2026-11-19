/** @vitest-environment jsdom */

import { cleanup, render } from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import type { PostListItem } from "@/lib/api/openapi-types";

const postComments = vi.hoisted(() =>
	vi.fn((_props: Record<string, unknown>) => null),
);

vi.mock("@/features/feed/components/PostComments", () => ({
	default: (props: Record<string, unknown>) => {
		postComments(props);
		return null;
	},
}));
vi.mock("@/features/feed/components/CommentsBottomSheet", () => ({
	default: () => null,
}));
vi.mock("@/features/feed/components/PostViewTracker", () => ({
	default: () => null,
}));
vi.mock("@/components/shared/AuthGateWaitModal", () => ({
	default: () => null,
}));
vi.mock("@/components/shared/Avatar", () => ({ default: () => null }));
vi.mock("@/components/shared/AppIcon", () => ({ default: () => null }));
vi.mock("@/components/shared/Link", () => ({ default: () => null }));
vi.mock("@/lib/auth/context", () => ({
	useAuth: () => ({ status: "signedIn" }),
}));
vi.mock("@/lib/auth/useAuthGate", () => ({
	useAuthGate: () => ({ authGate: vi.fn(), authWaiting: false }),
}));
vi.mock("@/lib/browser/body-scroll-lock", () => ({
	acquireBodyScrollLock: () => () => {},
}));
vi.mock("@/lib/hooks/useSocial", () => ({
	usePostInteractions: () => ({
		liked: false,
		likeCount: 0,
		likeBusy: false,
		toggleLike: vi.fn(),
	}),
}));

import PostDetailViewer from "../PostDetailViewer";

beforeAll(() => {
	vi.stubGlobal(
		"ResizeObserver",
		class {
			observe() {}
			unobserve() {}
			disconnect() {}
		},
	);
	window.matchMedia = ((query: string) => ({
		matches: false,
		media: query,
		addEventListener: () => {},
		removeEventListener: () => {},
		addListener: () => {},
		removeListener: () => {},
		onchange: null,
		dispatchEvent: () => false,
	})) as typeof window.matchMedia;
});
afterEach(() => {
	cleanup();
	postComments.mockClear();
});

const viewer = {
	canComment: true,
	actor: { kind: "store" as const, id: "7", name: "小店" },
};

function post(): PostListItem {
	return {
		id: "42",
		sellerId: "7",
		storeCode: "",
		sellerName: "店",
		logoKey: "",
		content: "內容",
		imageKeys: ["a.jpg"],
		tags: [],
		product: null,
		commentCount: 0,
		likeCount: 0,
		shareCount: 0,
		viewCount: 0,
		isLiked: false,
		createdAt: 1,
		viewer,
	};
}

describe("PostDetailViewer", () => {
	it("hands the post's own viewer state to the inline comments so they do not re-read the post", () => {
		render(
			React.createElement(PostDetailViewer, {
				posts: [post()],
				initialIndex: 0,
				onClose: () => {},
			}),
		);
		expect(postComments).toHaveBeenCalledWith(
			expect.objectContaining({ postId: "42", viewer }),
		);
	});
});
