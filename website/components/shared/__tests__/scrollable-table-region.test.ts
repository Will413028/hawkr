/** @vitest-environment jsdom */

import { cleanup, render } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ScrollableTableRegion } from "@/components/shared/ScrollableTableRegion";

function mockWidths(scrollWidth: number, clientWidth: number) {
	vi.spyOn(HTMLElement.prototype, "scrollWidth", "get").mockReturnValue(
		scrollWidth,
	);
	vi.spyOn(HTMLElement.prototype, "clientWidth", "get").mockReturnValue(
		clientWidth,
	);
}

function region() {
	return React.createElement(ScrollableTableRegion, {
		label: "訂單表格",
		children: React.createElement("table"),
	});
}

describe("ScrollableTableRegion", () => {
	afterEach(() => {
		cleanup();
		vi.restoreAllMocks();
	});

	it("is neither focusable nor a region when it does not overflow", () => {
		mockWidths(300, 300);
		const { container } = render(region());
		const el = container.firstElementChild as HTMLElement;
		expect(el.hasAttribute("tabindex")).toBe(false);
		expect(el.hasAttribute("role")).toBe(false);
		expect(el.hasAttribute("aria-label")).toBe(false);
	});

	it("is a focusable named region when it overflows horizontally", () => {
		mockWidths(900, 300);
		const { container } = render(region());
		const el = container.firstElementChild as HTMLElement;
		expect(el.getAttribute("tabindex")).toBe("0");
		expect(el.getAttribute("role")).toBe("region");
		expect(el.getAttribute("aria-label")).toBe("訂單表格");
	});
});
