/** @vitest-environment jsdom */

import { cleanup, render, screen, waitFor } from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({ fetchSellerReviews: vi.fn() }));
const rejectLater = () =>
	new Promise<never>((_, reject) =>
		setTimeout(() => reject(new Error("down")), 0),
	);
vi.mock("@/lib/api/index", () => ({
	fetchSellerReviews: mocks.fetchSellerReviews,
}));
vi.mock("@/lib/hooks/useUrlPageParam", () => ({
	useUrlPageParam: () => [2, vi.fn()],
}));
vi.mock("./ReviewList", () => ({
	default: () => React.createElement("div", null, "review-list"),
}));

import ReviewsClient from "./ReviewsClient";

function mount(strict?: boolean) {
	return render(
		React.createElement(ReviewsClient, {
			sellerId: "700",
			statsCard: null,
			initialReviews: [],
			initialTotalPages: 3,
			strict,
		}),
	);
}

beforeEach(() => {
	mocks.fetchSellerReviews.mockReset();
});
afterEach(cleanup);

describe("ReviewsClient later pages", () => {
	it("strict mode shows the error state when a page fetch fails", async () => {
		mocks.fetchSellerReviews.mockImplementation(() => rejectLater());
		mount(true);
		await waitFor(() =>
			expect(screen.getByRole("alert").textContent).toBe(
				"評價暫時載入不到，請稍後再試。",
			),
		);
		expect(mocks.fetchSellerReviews).toHaveBeenCalledWith("700", 2, true);
		expect(screen.queryByText("review-list")).toBeNull();
	});

	it("public mode keeps the list when a page fetch throws", async () => {
		mocks.fetchSellerReviews.mockImplementation(() => rejectLater());
		mount();
		await waitFor(() =>
			expect(mocks.fetchSellerReviews).toHaveBeenCalledWith("700", 2, false),
		);
		expect(screen.queryByRole("alert")).toBeNull();
		expect(screen.getByText("review-list")).toBeTruthy();
	});
});
