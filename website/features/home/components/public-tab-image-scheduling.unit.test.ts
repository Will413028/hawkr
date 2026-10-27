/** @vitest-environment jsdom */

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import * as React from "react";
import { afterEach, expect, it, vi } from "vitest";
import type { PostListItem } from "@/lib/api/openapi-types";
import HomeCompactMarketStage from "./HomeCompactMarketStage";

vi.mock("@/components/shared/Link", () => ({
	default: (props: React.AnchorHTMLAttributes<HTMLAnchorElement>) =>
		React.createElement("a", props),
}));
vi.mock("@/components/shared/EmptyStatePanel", () => ({
	default: () => null,
}));
vi.mock("@/features/home/components/HomePostDetailViewer", () => ({
	default: () => null,
}));
vi.mock("@/features/home/components/HomePostDetailViewerLoader", () => ({
	prewarmHomePostDetailViewer: vi.fn(),
}));
vi.mock("@/lib/state/use-login-required-overlay", () => ({
	useLoginRequiredOverlay: () => vi.fn(),
}));
vi.mock("framer-motion", () => ({
	useReducedMotion: () => true,
	AnimatePresence: ({ children }: { children: React.ReactNode }) => children,
	motion: {
		div: ({
			children,
			className,
		}: {
			children: React.ReactNode;
			className?: string;
		}) => React.createElement("div", { className }, children),
	},
}));
// Probe the scheduling props at the real carousel's card boundary. No image
// needs to resolve before the carousel controls can select another story.
vi.mock("@/features/home/components/HomeStageFeedCard", () => ({
	default: ({
		post,
		imageLoading,
		imagePriority,
	}: {
		post: PostListItem;
		imageLoading?: string;
		imagePriority?: boolean;
	}) =>
		React.createElement(
			"article",
			{
				"data-post": post.id,
				"data-loading": imageLoading,
				"data-priority": Boolean(imagePriority),
			},
			post.content,
		),
}));

const posts: PostListItem[] = Array.from({ length: 5 }, (_, index) => ({
	id: String(index + 1),
	sellerId: "fixture-seller",
	storeCode: "fixture-store",
	sellerName: "測試商家",
	logoKey: "",
	content: `動態 ${index}`,
	imageKeys: [`/post-${index}.jpg`],
	tags: [],
	commentCount: 0,
	likeCount: 0,
	shareCount: 0,
	viewCount: 0,
	isLiked: false,
	createdAt: 0,
}));
afterEach(() => cleanup());

it("renders hidden home stories lazily and promotes the selected story without moving LCP priority", () => {
	const view = render(
		React.createElement(HomeCompactMarketStage, {
			posts: posts.slice(0, 5),
			products: [],
		}),
	);
	const cards = () =>
		Array.from(view.container.querySelectorAll("[data-post]"));
	expect(cards().map((card) => card.getAttribute("data-loading"))).toEqual([
		"eager",
		"lazy",
		"lazy",
		"lazy",
		"lazy",
	]);
	fireEvent.click(screen.getByRole("button", { name: "下一則" }));
	expect(cards()[1].getAttribute("data-loading")).toBe("eager");
	expect(cards()[1].getAttribute("data-priority")).toBe("false");
	expect(cards()[0].getAttribute("data-priority")).toBe("true");
});
