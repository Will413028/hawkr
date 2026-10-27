/** @vitest-environment jsdom */

import { cleanup, render, screen } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import HomeMerchantPrizeGlyph from "./HomeMerchantPrizeGlyph";
import { HomeMerchantPrizeIconBoundary } from "./HomeMerchantPrizeIcon";

function BrokenPrizeGlyph(): React.ReactNode {
	throw new Error("fixture prize glyph failed");
}

afterEach(() => {
	cleanup();
	vi.restoreAllMocks();
});

describe("home merchant prize icon", () => {
	it("keeps the original decorative trophy contract", () => {
		const { container } = render(React.createElement(HomeMerchantPrizeGlyph));
		const icon = container.querySelector('svg[data-icon-name="trophy"]');

		expect(icon).toBeInstanceOf(SVGSVGElement);
		expect(icon?.getAttribute("width")).toBe("11");
		expect(icon?.getAttribute("height")).toBe("11");
		expect(icon?.getAttribute("aria-hidden")).toBe("true");
		expect(icon?.getAttribute("aria-label")).toBeNull();
		expect(icon?.getAttribute("class")).toContain("shrink-0");
		expect(icon?.outerHTML).toContain("currentColor");
	});

	it("contains a failed icon without swallowing authoritative award content", () => {
		vi.spyOn(console, "error").mockImplementation(() => {});
		render(
			React.createElement(
				"article",
				null,
				React.createElement("a", { href: "/sellers/fixture-farm" }, "查看商家"),
				React.createElement(
					"span",
					{ title: "2026 冠軍梨" },
					React.createElement(
						HomeMerchantPrizeIconBoundary,
						null,
						React.createElement(BrokenPrizeGlyph),
					),
					React.createElement("span", null, "2026 冠軍梨"),
				),
			),
		);

		expect(
			screen.getByRole("link", { name: "查看商家" }).getAttribute("href"),
		).toBe("/sellers/fixture-farm");
		expect(screen.getByText("2026 冠軍梨")).toBeTruthy();
		const reservation = document.querySelector(
			'span[aria-hidden="true"][style="width: 11px; height: 11px;"]',
		);
		expect(reservation).toBeTruthy();
	});
});
