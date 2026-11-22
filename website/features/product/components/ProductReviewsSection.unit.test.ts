// @vitest-environment jsdom

import { cleanup, render, screen } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { IntlTestWrapper } from "@/tests/mocks/intl-test-wrapper";
import ProductReviewsSection from "./ProductReviewsSection";

vi.mock("@/components/shared/Avatar", () => ({
	default: () => null,
}));
vi.mock("@/features/report/ReportActionMenu", () => ({
	default: ({ target }: { target: { id: string } }) =>
		React.createElement(
			"button",
			{ type: "button", "data-testid": "report" },
			target.id,
		),
}));

afterEach(cleanup);

function view(
	reviews: Array<{ id: string; author?: string; isMine?: boolean }>,
) {
	return render(
		React.createElement(
			IntlTestWrapper,
			null,
			React.createElement(ProductReviewsSection, {
				reviews,
				reviewTotal: reviews.length,
				reportable: true,
			}),
		),
	);
}

describe("ProductReviewsSection", () => {
	it("shows the catalog anonymous name when the author has no name", () => {
		view([{ id: "1" }]);
		expect(screen.getByText("匿名會員")).toBeTruthy();
	});

	it("hides the report action on the viewer's own review only", () => {
		view([
			{ id: "1", author: "A", isMine: true },
			{ id: "2", author: "B" },
		]);
		const reports = screen.getAllByTestId("report");
		expect(reports.map((r) => r.textContent)).toEqual(["2"]);
	});
});
