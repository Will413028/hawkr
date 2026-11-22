// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

const state = vi.hoisted(() => ({
	value: {} as Record<string, unknown>,
	loadMore: vi.fn(),
}));
vi.mock("@/lib/hooks/useMyReviews", () => ({
	useMyReviews: () => ({ ...state.value, loadMore: state.loadMore }),
}));
vi.mock("@/components/shared/AppImage", () => ({ default: () => null }));
vi.mock("@/components/shared/Link", () => ({
	default: ({ children }: { children: React.ReactNode }) =>
		React.createElement("a", null, children),
}));
vi.mock("@/components/ui/Stars", () => ({ Stars: () => null }));
vi.mock("@/features/account/components/AccountEmptyIcon", () => ({
	AccountEmptyStateIcon: () => null,
}));

import ReviewedList from "./ReviewedList";

const review = {
	id: "r1",
	rating: 5,
	content: "good",
	skuName: "sku",
	createdAt: 0,
	imageUrls: [],
	orderNo: "",
	productImageUrl: "",
};

afterEach(cleanup);

describe("ReviewedList next page failure", () => {
	it("shows an inline error and a retry that calls loadMore when the next page fails", () => {
		state.loadMore.mockClear();
		state.value = {
			reviews: [review],
			loading: false,
			loadingMore: false,
			error: new Error("x"),
			hasMore: true,
		};
		render(React.createElement(ReviewedList));
		expect(screen.getByRole("alert").textContent).toContain("載入更多評價失敗");
		fireEvent.click(screen.getByRole("button", { name: "重試" }));
		expect(state.loadMore).toHaveBeenCalledTimes(1);
	});

	it("shows no error when the next page is fine", () => {
		state.value = {
			reviews: [review],
			loading: false,
			loadingMore: false,
			error: null,
			hasMore: true,
		};
		render(React.createElement(ReviewedList));
		expect(screen.queryByRole("alert")).toBeNull();
		expect(screen.getByRole("button", { name: "載更多" })).toBeTruthy();
	});
});
