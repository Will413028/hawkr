// @vitest-environment jsdom

import { cleanup, render, screen, waitFor } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it } from "vitest";
import EmptyStatePanel from "./EmptyStatePanel";
import LazyAppIcon from "./LazyAppIcon";
import UploadLoadingIndicator from "./UploadLoadingIndicator";

afterEach(cleanup);

describe("UploadLoadingIndicator", () => {
	it("announces the upload as a named status and pulses while active", () => {
		render(React.createElement(UploadLoadingIndicator, { size: 22 }));
		const status = screen.getByRole("status", { name: "上傳中" });
		const glyph = status.querySelector("[data-icon-name='upload']");
		expect(glyph?.getAttribute("class")).toContain("animate-pulse");
		expect(glyph?.getAttribute("aria-hidden")).toBe("true");
	});

	it("holds still when inactive and keeps a custom name", () => {
		render(
			React.createElement(UploadLoadingIndicator, {
				active: false,
				ariaLabel: "上傳封面中",
			}),
		);
		const status = screen.getByRole("status", { name: "上傳封面中" });
		expect(
			status.querySelector("[data-icon-name='upload']")?.getAttribute("class"),
		).not.toContain("animate-pulse");
	});
});

describe("EmptyStatePanel", () => {
	it("renders the requested icon beside a heading of the requested level", () => {
		const { container } = render(
			React.createElement(EmptyStatePanel, {
				icon: "box",
				title: "目前沒有訂單",
				description: "買家下單後會自動出現在這裡。",
				titleAs: "h3",
			}),
		);
		expect(
			screen.getByRole("heading", { level: 3, name: "目前沒有訂單" }),
		).toBeDefined();
		expect(container.querySelector("[data-icon-name='box']")).not.toBeNull();
	});
});

describe("LazyAppIcon", () => {
	it("reserves the final box, then swaps in the glyph", async () => {
		const { container } = render(
			React.createElement(LazyAppIcon, { name: "cart", size: 24 }),
		);
		await waitFor(() =>
			expect(container.querySelector("[data-icon-name='cart']")).not.toBeNull(),
		);
		expect(container.querySelector("span[aria-hidden]")).toBeNull();
	});
});
