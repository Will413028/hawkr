import * as React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { SellerProductListItem } from "@/lib/products/seller-repo";
import type { PublicStoreView } from "@/lib/seller/public-store";
import {
	PREVIEW_MESSAGES_UNAVAILABLE,
	PREVIEW_REVIEWS_UNAVAILABLE,
} from "@/lib/seller/storefront-preview";

const mocks = vi.hoisted(() => ({
	useSellerProducts: vi.fn(),
	gridProps: vi.fn(),
	messageTab: vi.fn(),
	reviewsFetch: vi.fn(),
	statsFetch: vi.fn(),
}));

vi.mock("@/lib/hooks/useSellerProducts", () => ({
	useSellerProducts: mocks.useSellerProducts,
}));
vi.mock("@/features/sellers/storefront/StorefrontProductGrid", () => ({
	default: (props: Record<string, unknown>) => {
		mocks.gridProps(props);
		return null;
	},
}));
vi.mock("@/features/sellers/components/SellerMessageTab", () => ({
	default: () => {
		mocks.messageTab();
		return null;
	},
}));
vi.mock("@/lib/api/index", () => ({
	fetchSellerReviews: mocks.reviewsFetch,
	fetchSellerReviewStats: mocks.statsFetch,
}));
vi.mock("@/features/sellers/reviews/ReviewsClient", () => ({
	default: () => null,
}));
vi.mock("@tanstack/react-query", () => ({
	useQuery: (options: { queryFn: () => unknown }) => {
		void options.queryFn();
		return { data: undefined, isError: false };
	},
	useInfiniteQuery: () => ({ data: undefined, isError: false }),
}));

import PreviewMessagesTab from "./PreviewMessagesTab";
import PreviewProductsTab from "./PreviewProductsTab";
import PreviewReviewsTab from "./PreviewReviewsTab";
import { PreviewStoreProvider } from "./PreviewStoreContext";

const view: PublicStoreView = {
	storeId: "700",
	storeCode: "123456",
	name: "預覽店",
	description: "",
	prize: "",
	websiteUrl: "",
	logo_url: "",
	coverImage: "",
	city: "",
	district: "",
};

function renderTab(
	tab: () => React.ReactElement | null,
	canBePublic: boolean,
): string {
	return renderToStaticMarkup(
		React.createElement(
			PreviewStoreProvider,
			{ value: { view, canBePublic } },
			React.createElement(tab),
		),
	);
}

function product(
	id: string,
	status: string | number | undefined,
): SellerProductListItem {
	return { id, name: `商品${id}`, status, price_min: "100", price_max: "100" };
}

beforeEach(() => {
	for (const mock of Object.values(mocks)) mock.mockReset();
});

describe("preview reviews and messages tabs when the store cannot be public", () => {
	it("shows the reviews copy and makes no public reads", () => {
		const html = renderTab(PreviewReviewsTab, false);
		expect(html).toContain(PREVIEW_REVIEWS_UNAVAILABLE);
		expect(html).toContain("店家公開後，買家的評價會顯示在這裡。");
		expect(mocks.reviewsFetch).not.toHaveBeenCalled();
		expect(mocks.statsFetch).not.toHaveBeenCalled();
	});

	it("shows the messages copy and never mounts the forum", () => {
		const html = renderTab(PreviewMessagesTab, false);
		expect(html).toContain(PREVIEW_MESSAGES_UNAVAILABLE);
		expect(html).toContain("店家公開後，買家的迴響會顯示在這裡。");
		expect(mocks.messageTab).not.toHaveBeenCalled();
	});

	it("reads the public sources once the store can be public", () => {
		renderTab(PreviewReviewsTab, true);
		expect(mocks.reviewsFetch).toHaveBeenCalledWith("700", 1);
		expect(mocks.statsFetch).toHaveBeenCalledWith("700");
		const html = renderTab(PreviewMessagesTab, true);
		expect(mocks.messageTab).toHaveBeenCalledTimes(1);
		expect(html).not.toContain(PREVIEW_MESSAGES_UNAVAILABLE);
	});
});

describe("preview products tab", () => {
	it("asks for on-sale products and drops draft, off-shelf and status-less items", () => {
		mocks.useSellerProducts.mockReturnValue({
			data: {
				records: [
					product("1", "4004"),
					product("2", "4003"),
					product("3", "4001"),
					product("4", 4004),
					product("5", undefined),
				],
				total: 5,
				page: 1,
				pageSize: 12,
			},
			loading: false,
			error: null,
		});
		renderTab(PreviewProductsTab, true);

		expect(mocks.useSellerProducts).toHaveBeenCalledWith(
			expect.objectContaining({ status: "4004" }),
		);
		const props = mocks.gridProps.mock.calls[0][0] as {
			items: Array<{ id: string }>;
			cardPreview: boolean;
		};
		// 4004 以數字形式回來的列表項（舊路徑）同樣是上架中。
		expect(props.items.map((item) => item.id)).toEqual(["1", "4"]);
		expect(props.cardPreview).toBe(true);
	});
});
