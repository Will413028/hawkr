// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
	getCachedProductReviews: vi.fn(),
	refresh: vi.fn(),
}));
vi.mock("@/lib/api/cached", () => ({
	getCachedProductReviews: mocks.getCachedProductReviews,
}));
vi.mock("@/lib/navigation/use-observed-router", () => ({
	useObservedRouter: () => ({ refresh: mocks.refresh }),
}));

import ProductReviewsUnavailable from "./components/ProductReviewsUnavailable";
import ReviewsAsync from "./ReviewsAsync";

afterEach(() => {
	cleanup();
	vi.clearAllMocks();
});

describe("ReviewsAsync read failure", () => {
	it("renders the unavailable block (not null, not the empty state) when the read fails", async () => {
		mocks.getCachedProductReviews.mockRejectedValue(new Error("down"));
		const el = (await ReviewsAsync({ productId: "p1" })) as React.ReactElement;
		expect(el).not.toBeNull();
		expect(el.type).toBe(ProductReviewsUnavailable);
	});

	it("the unavailable block offers a retry that refreshes the route", () => {
		render(React.createElement(ProductReviewsUnavailable));
		expect(screen.getByRole("alert").textContent).toContain("無法載入");
		fireEvent.click(screen.getByRole("button", { name: "重新載入" }));
		expect(mocks.refresh).toHaveBeenCalledTimes(1);
	});
});
