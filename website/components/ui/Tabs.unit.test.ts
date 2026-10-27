/** @vitest-environment jsdom */

import { cleanup, render, screen } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it } from "vitest";
import { Tabs } from "./Tabs";

afterEach(cleanup);

const items = [
	{ key: "a", label: "A", content: React.createElement("p", null, "panel-a") },
	{ key: "b", label: "B", content: React.createElement("p", null, "panel-b") },
];

describe("Tabs keepMounted", () => {
	it("預設只渲染 active panel（既有行為不得改變）", () => {
		render(React.createElement(Tabs, { items, defaultSelectedKey: "a" }));
		expect(screen.queryByText("panel-a")).not.toBeNull();
		expect(screen.queryByText("panel-b")).toBeNull();
	});

	it("keepMounted 時兩個 panel 都在 DOM 裡，非 active 的帶 hidden", () => {
		render(
			React.createElement(Tabs, {
				items,
				defaultSelectedKey: "a",
				keepMounted: true,
			}),
		);
		const panelA = screen.getByText("panel-a").closest('[role="tabpanel"]');
		const panelB = screen.getByText("panel-b").closest('[role="tabpanel"]');
		expect(panelA?.hasAttribute("hidden")).toBe(false);
		expect(panelB?.hasAttribute("hidden")).toBe(true);
	});

	it("keepMounted 時每個 panel 仍與自己的 tab button aria 對得起來", () => {
		render(
			React.createElement(Tabs, {
				items,
				defaultSelectedKey: "a",
				keepMounted: true,
			}),
		);
		for (const name of ["A", "B"]) {
			const tab = screen.getByRole("tab", { name, hidden: true });
			const panelId = tab.getAttribute("aria-controls");
			expect(panelId).toBeTruthy();
			const panel = document.getElementById(panelId as string);
			expect(panel?.getAttribute("aria-labelledby")).toBe(tab.id);
		}
	});
});
