/** @vitest-environment jsdom */

import {
	cleanup,
	fireEvent,
	render,
	screen,
	waitFor,
} from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({ fetchStoreReviewsAtPage: vi.fn() }));
const rejectLater = () =>
	new Promise<never>((_, reject) =>
		setTimeout(() => reject(new Error("down")), 0),
	);
vi.mock("@/lib/reviews/repo", () => ({
	fetchStoreReviewsAtPage: mocks.fetchStoreReviewsAtPage,
}));
vi.mock("@/lib/hooks/useUrlPageParam", () => ({
	useUrlPageParam: () => [2, vi.fn()],
}));
vi.mock("./ReviewList", () => ({
	default: ({ reviews }: { reviews: unknown[] }) =>
		React.createElement("div", null, `review-list:${reviews.length}`),
}));

import ReviewsClient from "./ReviewsClient";

function mount(extra: Record<string, unknown> = {}) {
	return render(
		React.createElement(ReviewsClient, {
			sellerId: "700",
			statsCard: null,
			initialReviews: [],
			initialTotalPages: 3,
			...extra,
		}),
	);
}

beforeEach(() => {
	mocks.fetchStoreReviewsAtPage.mockReset();
});
afterEach(cleanup);

describe("ReviewsClient later pages", () => {
	it("shows the error state, not an empty list, when a page 2+ read fails", async () => {
		mocks.fetchStoreReviewsAtPage.mockImplementation(() => rejectLater());
		mount();
		await waitFor(() =>
			expect(screen.getByRole("alert").textContent).toBe(
				"評價暫時載入不到，請稍後再試。",
			),
		);
		expect(mocks.fetchStoreReviewsAtPage).toHaveBeenCalledWith(
			"700",
			2,
			expect.anything(),
		);
		expect(screen.queryByText(/review-list/)).toBeNull();
	});

	it("retries a failed page from the error block", async () => {
		mocks.fetchStoreReviewsAtPage.mockImplementationOnce(() => rejectLater());
		mocks.fetchStoreReviewsAtPage.mockResolvedValueOnce({
			reviews: [],
			nextPageToken: "",
		});
		mount();
		await waitFor(() => screen.getByRole("alert"));
		fireEvent.click(screen.getByRole("button", { name: "重新載入" }));
		await waitFor(() => expect(screen.queryByRole("alert")).toBeNull());
		expect(mocks.fetchStoreReviewsAtPage).toHaveBeenCalledTimes(2);
		expect(screen.getByText("review-list:0")).toBeTruthy();
	});

	it("a successful empty page renders the list, not the error", async () => {
		mocks.fetchStoreReviewsAtPage.mockResolvedValue({
			reviews: [],
			nextPageToken: "",
		});
		mount();
		await waitFor(() => screen.getByText("review-list:0"));
		expect(screen.queryByRole("alert")).toBeNull();
	});

	it("wraps the retry button in the reading-control wrapper when given", async () => {
		mocks.fetchStoreReviewsAtPage.mockImplementation(() => rejectLater());
		mount({
			paginationWrapper: (n: React.ReactNode) =>
				React.createElement("div", { "data-wrap": "1" }, n),
		});
		const btn = await screen.findByRole("button", { name: "重新載入" });
		expect(btn.closest("[data-wrap]")).not.toBeNull();
	});
});
