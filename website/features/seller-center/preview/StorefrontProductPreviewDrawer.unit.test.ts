/** @vitest-environment jsdom */

import { cleanup, render, screen, waitFor } from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiClientError } from "@/lib/api/client";
import type { PublicStoreView } from "@/lib/seller/public-store";

const mocks = vi.hoisted(() => {
	const state = {
		calls: 0,
		impl: (): Promise<unknown> => Promise.resolve(null),
	};
	return {
		state,
		fetchEdit: () => {
			state.calls += 1;
			return state.impl();
		},
	};
});
vi.mock("@/lib/products/seller-repo", () => ({
	fetchSellerProductEdit: mocks.fetchEdit,
}));
vi.mock("@/lib/reviews/repo", () => ({
	fetchProductReviews: vi.fn().mockResolvedValue(null),
}));
vi.mock("@/components/shared/Drawer", () => ({
	Drawer: (props: { open: boolean; children: React.ReactNode }) =>
		props.open ? React.createElement("div", null, props.children) : null,
}));
vi.mock("@/features/product/ProductDetailLayout", () => ({
	default: () => null,
}));
vi.mock("@/features/product/components/ProductSellerIntro", () => ({
	default: () => null,
}));
vi.mock("@/features/product/deriveDetailLayoutProps", () => ({
	deriveDetailLayoutProps: () => ({}),
}));
vi.mock("@/lib/seller/storefront-preview", () => ({
	previewDetailProduct: () => ({ id: "9", name: "x" }),
	previewStoreStats: () => ({ sellerStats: null, followerStats: null }),
}));

import StorefrontProductPreviewDrawer from "./StorefrontProductPreviewDrawer";

const store = { storeId: "700", storeCode: "123456" } as PublicStoreView;

function drawer(s: PublicStoreView) {
	return React.createElement(StorefrontProductPreviewDrawer, {
		productId: "9",
		store: s,
		onClose: () => undefined,
	});
}

beforeEach(() => {
	mocks.state.calls = 0;
});
afterEach(cleanup);

describe("StorefrontProductPreviewDrawer", () => {
	it("shows unavailable for a missing product", async () => {
		mocks.state.impl = () => Promise.resolve(null);
		render(drawer(store));
		await screen.findByText("目前無法預覽這項商品");
	});

	it("shows an error with retry when the read fails", async () => {
		mocks.state.impl = () => Promise.reject(new ApiClientError("boom", 503));
		render(drawer(store));
		await screen.findByText("商品資料載入失敗");
		expect(screen.getByText("重新載入")).toBeTruthy();
		expect(screen.queryByText("目前無法預覽這項商品")).toBeNull();
	});

	it("does not refetch when the store object is replaced with the same id", async () => {
		mocks.state.impl = () => Promise.resolve({ id: "9" });
		const view = render(drawer(store));
		await waitFor(() => expect(mocks.state.calls).toBe(1));
		view.rerender(drawer({ ...store }));
		await new Promise((r) => setTimeout(r, 20));
		expect(mocks.state.calls).toBe(1);
	});
});
