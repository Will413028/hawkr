// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, waitFor } from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { MyPostDetail, PostListItem } from "@/lib/api/openapi-types";

const fetchSellerPosts = vi.fn();
const fetchSellerPostDetail = vi.fn();
const modalProps = vi.fn((_props: unknown) => null);

vi.mock("@/lib/seller-data/scope", () => ({
	useSellerDataScope: () => scope,
}));
vi.mock("@/lib/hooks/useSellerSelf", () => ({
	useSellerSelf: () => ({ data: null, loading: false }),
}));
vi.mock("@/components/shared/useSellerIdentity", () => ({
	useSellerIdentity: () => ({ isSuspended: false }),
}));
vi.mock("./SellerPostDetailModal", () => ({
	default: (props: { post: PostListItem | null }) => {
		modalProps(props);
		return null;
	},
}));
vi.mock("@/components/shared/SellerPageShell", () => ({
	SellerPageShell: ({ children }: { children: React.ReactNode }) =>
		React.createElement("div", null, children),
}));
vi.mock("@/components/shared/AppImage", () => ({ default: () => null }));
vi.mock("@/components/shared/AppIcon", () => ({ default: () => null }));
vi.mock("@/components/ui/Button", () => ({ default: () => null }));
vi.mock("@/features/feed/components/PostMetricIcons", () => ({
	PostImageIcon: () => null,
}));
vi.mock("@/features/seller-center", () => ({
	SellerEmptyState: () => null,
	SellerPostsSkeleton: () => null,
}));
vi.mock(
	"@/features/seller-center/components/SellerMobileDisclosureList",
	() => ({
		SellerMobileDisclosureIcon: () => null,
		SellerMobileDisclosureList: () => null,
	}),
);
vi.mock("@/features/seller-center/components/SellerPageSkeletons", () => ({
	SellerMobileDisclosureSkeleton: () => null,
}));
vi.mock("./useSellerPostsViewport", () => ({
	useSellerPostsViewport: () => "desktop",
}));

const scope = {
	mode: "seller",
	ownerStoreId: "7",
	keys: { posts: (id: string) => ["posts", id] },
	canRead: () => true,
	repos: {
		posts: {
			fetchSellerPosts: (...args: unknown[]) => fetchSellerPosts(...args),
			fetchSellerPostDetail: (id: string) => fetchSellerPostDetail(id),
		},
	},
};

import SellerPostsClient from "./SellerPostsClient";

function listItem(id: string): PostListItem {
	return {
		id,
		sellerId: "7",
		storeCode: "",
		sellerName: "店",
		logoKey: "",
		content: `貼文 ${id}`,
		imageKeys: [],
		tags: [],
		product: null,
		commentCount: 0,
		likeCount: 0,
		shareCount: 0,
		viewCount: 0,
		isLiked: false,
		createdAt: 1,
	};
}

function mount() {
	const client = new QueryClient({
		defaultOptions: { queries: { retry: false } },
	});
	return render(
		React.createElement(
			QueryClientProvider,
			{ client },
			React.createElement(SellerPostsClient),
		),
	);
}

beforeEach(() => {
	window.history.pushState({}, "", "/seller-center/posts?post=900");
});

afterEach(() => {
	cleanup();
	fetchSellerPosts.mockReset();
	fetchSellerPostDetail.mockReset();
	modalProps.mockClear();
});

describe("SellerPostsClient deep link", () => {
	it("opens a post that is not on the loaded page through the owner read, with its owner-only state", async () => {
		fetchSellerPosts.mockResolvedValue([listItem("1"), listItem("2")]);
		const hidden: MyPostDetail = {
			...listItem("900"),
			products: [],
			publishStatus: "4402",
			hiddenAt: 1760000000,
			hiddenReason: "違規",
		};
		fetchSellerPostDetail.mockResolvedValue(hidden);

		mount();

		await waitFor(() => {
			const shown = modalProps.mock.calls
				.map(([props]) => (props as { post: PostListItem | null }).post)
				.find((post) => post?.id === "900");
			expect(shown).toMatchObject({
				id: "900",
				publishStatus: "4402",
				hiddenAt: 1760000000,
				hiddenReason: "違規",
			});
		});
		expect(fetchSellerPostDetail).toHaveBeenCalledWith("900");
	});

	it("does not read the detail when the post is on the loaded page", async () => {
		fetchSellerPosts.mockResolvedValue([listItem("900")]);

		mount();

		await waitFor(() => {
			const shown = modalProps.mock.calls
				.map(([props]) => (props as { post: PostListItem | null }).post)
				.find((post) => post?.id === "900");
			expect(shown).toBeTruthy();
		});
		expect(fetchSellerPostDetail).not.toHaveBeenCalled();
	});

	it("opens nothing when the owner read fails", async () => {
		fetchSellerPosts.mockResolvedValue([listItem("1")]);
		fetchSellerPostDetail.mockRejectedValue(new Error("404"));

		mount();

		await waitFor(() => expect(fetchSellerPostDetail).toHaveBeenCalled());
		expect(
			modalProps.mock.calls.some(
				([props]) => (props as { post: PostListItem | null }).post !== null,
			),
		).toBe(false);
	});
});
