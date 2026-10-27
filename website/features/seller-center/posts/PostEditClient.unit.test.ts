// @vitest-environment jsdom

import { cleanup, render, screen, waitFor } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClientError } from "@/lib/api/client";
import type { MyPostDetail } from "@/lib/api/openapi-types";

const fetchSellerPostDetail = vi.fn();
const composeProps = vi.fn((_props: unknown) => null);

vi.mock("@/lib/seller/repo", () => ({
	sellerPostRepo: {
		fetchSellerPostDetail: (id: string) => fetchSellerPostDetail(id),
	},
}));
vi.mock("./PostComposeClient", () => ({
	default: (props: unknown) => {
		composeProps(props);
		return React.createElement("div", { "data-testid": "compose" });
	},
}));
vi.mock("@/components/shared/BackLink", () => ({
	BackLink: ({ children }: { children: React.ReactNode }) =>
		React.createElement("a", null, children),
}));
vi.mock("@/components/shared/AppIcon", () => ({ default: () => null }));
vi.mock("@/components/shared/SellerPageShell", () => ({
	SellerPageShell: ({ children }: { children: React.ReactNode }) =>
		React.createElement("div", null, children),
}));

import PostEditClient from "./PostEditClient";

function detail(overrides: Partial<MyPostDetail> = {}): MyPostDetail {
	return {
		id: "77",
		sellerId: "7",
		sellerName: "店",
		logoKey: "",
		content: "被平台下架的內容",
		imageKeys: ["img-key"],
		tags: [],
		products: [],
		commentCount: 0,
		likeCount: 0,
		shareCount: 0,
		viewCount: 0,
		isLiked: false,
		createdAt: 1,
		publishStatus: "4402",
		hiddenAt: 1760000000,
		hiddenReason: "違規",
		...overrides,
	};
}

afterEach(() => {
	cleanup();
	fetchSellerPostDetail.mockReset();
	composeProps.mockClear();
});

describe("PostEditClient", () => {
	it("hydrates the editor from the owner read, also for a platform-hidden post", async () => {
		fetchSellerPostDetail.mockResolvedValue(detail());
		render(React.createElement(PostEditClient, { id: "77" }));

		expect(await screen.findByTestId("compose")).toBeTruthy();
		expect(fetchSellerPostDetail).toHaveBeenCalledWith("77");
		expect(composeProps).toHaveBeenCalledWith({
			edit: {
				id: "77",
				content: "被平台下架的內容",
				imageKey: "img-key",
				productId: undefined,
			},
		});
	});

	it("is refresh-safe: a fresh mount reads the post by the URL id alone", async () => {
		fetchSellerPostDetail.mockResolvedValue(detail({ publishStatus: "4401" }));
		const first = render(React.createElement(PostEditClient, { id: "77" }));
		await screen.findByTestId("compose");
		first.unmount();

		render(React.createElement(PostEditClient, { id: "77" }));
		await screen.findByTestId("compose");
		expect(fetchSellerPostDetail).toHaveBeenCalledTimes(2);
	});

	it("shows not-found for a post that is not the store's", async () => {
		fetchSellerPostDetail.mockRejectedValue(
			new ApiClientError("not found", 200, 404),
		);
		render(React.createElement(PostEditClient, { id: "99" }));

		await waitFor(() =>
			expect(screen.getAllByText("找不到這則動態").length).toBeGreaterThan(0),
		);
		expect(screen.queryByTestId("compose")).toBeNull();
	});

	it("shows a load failure for anything else", async () => {
		fetchSellerPostDetail.mockRejectedValue(
			new ApiClientError("HTTP 503", 503, 503),
		);
		render(React.createElement(PostEditClient, { id: "77" }));

		expect(await screen.findByText("載入動態失敗")).toBeTruthy();
	});
});
