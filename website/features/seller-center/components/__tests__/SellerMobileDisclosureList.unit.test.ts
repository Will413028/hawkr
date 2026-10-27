// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { SellerMobileDisclosureList } from "../SellerMobileDisclosureList";

vi.mock("@/components/shared/AppIcon", () => ({
	default: () => null,
}));

afterEach(() => {
	cleanup();
	vi.restoreAllMocks();
});

describe("SellerMobileDisclosureList deferred details", () => {
	it("does not mount collapsed details and keeps only the expanded row mounted", () => {
		const items = [
			{ id: "first", title: "第一筆" },
			{ id: "second", title: "第二筆" },
		];
		const renderDetails = vi.fn((item: (typeof items)[number]) =>
			React.createElement(
				"div",
				{ "data-testid": `detail-${item.id}` },
				item.title,
			),
		);

		render(
			React.createElement(SellerMobileDisclosureList<(typeof items)[number]>, {
				items,
				getKey: (item: (typeof items)[number]) => item.id,
				getSummary: (item: (typeof items)[number]) => ({
					label: item.title,
					leading: null,
					title: item.title,
				}),
				renderDetails,
				ariaLabel: "測試清單",
				deferDetailsUntilExpanded: true,
			}),
		);

		expect(renderDetails).not.toHaveBeenCalled();
		expect(screen.queryByTestId("detail-first")).toBeNull();
		expect(screen.queryByTestId("detail-second")).toBeNull();

		fireEvent.click(screen.getByRole("button", { name: "展開第一筆" }));

		expect(renderDetails).toHaveBeenCalled();
		expect(
			renderDetails.mock.calls.every(([item]) => item.id === "first"),
		).toBe(true);
		expect(screen.getByTestId("detail-first")).not.toBeNull();
		expect(screen.queryByTestId("detail-second")).toBeNull();
		renderDetails.mockClear();

		fireEvent.click(screen.getByRole("button", { name: "展開第二筆" }));

		expect(renderDetails).toHaveBeenCalled();
		expect(
			renderDetails.mock.calls.every(([item]) => item.id === "second"),
		).toBe(true);
		expect(screen.queryByTestId("detail-first")).toBeNull();
		expect(screen.getByTestId("detail-second")).not.toBeNull();
	});
});
