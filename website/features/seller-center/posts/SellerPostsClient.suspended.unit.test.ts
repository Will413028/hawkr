// @vitest-environment jsdom

import { SellerCapability } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { PostListItem } from "@/lib/api/openapi-types";
import { IntlTestWrapper } from "@/tests/mocks/intl-test-wrapper";

const state = vi.hoisted(() => ({
	suspended: false,
	mode: "seller",
	self: null as unknown,
}));
const fetchSellerPosts = vi.fn();

vi.mock("@/lib/seller-data/scope", () => ({
	useSellerDataScope: () => ({
		mode: state.mode,
		ownerStoreId: "7",
		keys: { posts: (id: string) => ["posts", id] },
		canRead: () => true,
		repos: {
			posts: {
				fetchSellerPosts: (...args: unknown[]) => fetchSellerPosts(...args),
				fetchSellerPostDetail: vi.fn(),
			},
		},
	}),
}));
vi.mock("@/lib/hooks/useSellerSelf", () => ({
	useSellerSelf: () => ({ data: state.self, loading: false }),
}));
vi.mock("@/components/shared/useSellerIdentity", () => ({
	useSellerIdentity: () => ({ isSuspended: state.suspended }),
}));
vi.mock("./SellerPostDetailModal", () => ({ default: () => null }));
vi.mock("@/components/shared/SellerPageShell", () => ({
	SellerPageShell: ({
		actions,
		children,
	}: {
		actions?: React.ReactNode;
		children: React.ReactNode;
	}) => React.createElement("div", null, actions, children),
}));
vi.mock("@/components/shared/AppImage", () => ({ default: () => null }));
vi.mock("@/components/shared/AppIcon", () => ({ default: () => null }));
vi.mock("@/components/ui/Button", () => ({
	default: (props: {
		children?: React.ReactNode;
		disabled?: boolean;
		href?: string;
	}) =>
		React.createElement(
			"button",
			{ type: "button", disabled: props.disabled, "data-href": props.href },
			props.children,
		),
}));
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
vi.mock("./PostPublishControl", () => ({
	default: (props: { suspended?: boolean }) =>
		React.createElement(
			"span",
			{
				"data-testid": "publish-control",
				"data-locked": String(props.suspended),
			},
			"publish",
		),
}));

import SellerPostsClient from "./SellerPostsClient";

const NOTICE = "店家目前受平台措施限制，動態只能下架或刪除。";

function post(id: string): PostListItem {
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
		publishStatus: "4402",
	};
}

function mount() {
	const client = new QueryClient({
		defaultOptions: { queries: { retry: false } },
	});
	return render(
		React.createElement(
			IntlTestWrapper,
			null,
			React.createElement(
				QueryClientProvider,
				{ client },
				React.createElement(SellerPostsClient),
			),
		),
	);
}

function disabledOf(name: string): boolean[] {
	return screen
		.getAllByText(name)
		.map((el) => (el.closest("button") as HTMLButtonElement).disabled);
}

afterEach(() => {
	cleanup();
	fetchSellerPosts.mockReset();
	state.suspended = false;
	state.mode = "seller";
	state.self = null;
});

describe("seller posts list while the store is restricted", () => {
	it("shows the notice, disables publish and edit, and keeps delete", async () => {
		state.suspended = true;
		fetchSellerPosts.mockResolvedValue([post("1")]);
		mount();
		await waitFor(() => screen.getByText("刪除"));

		expect(screen.getByText(NOTICE)).toBeTruthy();
		expect(disabledOf("發布動態")).toEqual([true]);
		expect(disabledOf("編輯")).toEqual([true]);
		expect(disabledOf("刪除")).toEqual([false]);
	});

	it("a store whose MARKETING capability is revoked but is not suspended is locked too", async () => {
		state.self = {
			store: { id: "7" },
			capabilities: [
				{ capability: SellerCapability.MARKETING, active: false, missing: [] },
			],
		};
		fetchSellerPosts.mockResolvedValue([post("1")]);
		mount();
		await waitFor(() => screen.getByText("刪除"));

		expect(screen.getByText(NOTICE)).toBeTruthy();
		expect(disabledOf("發布動態")).toEqual([true]);
		expect(disabledOf("編輯")).toEqual([true]);
		expect(disabledOf("刪除")).toEqual([false]);
		expect(screen.getByTestId("publish-control").dataset.locked).toBe("true");
	});

	it("an active store gets no notice and enabled entries", async () => {
		fetchSellerPosts.mockResolvedValue([post("1")]);
		mount();
		await waitFor(() => screen.getByText("刪除"));

		expect(screen.queryByText(NOTICE)).toBeNull();
		expect(disabledOf("發布動態")).toEqual([false]);
		expect(disabledOf("編輯")).toEqual([false]);
	});

	it("the delegated workspace is not the store itself: no notice, no lock", async () => {
		state.suspended = true;
		state.mode = "delegated";
		fetchSellerPosts.mockResolvedValue([post("1")]);
		mount();
		await waitFor(() => screen.getByText("發布動態"));

		expect(screen.queryByText(NOTICE)).toBeNull();
		expect(disabledOf("發布動態")).toEqual([false]);
	});
});
