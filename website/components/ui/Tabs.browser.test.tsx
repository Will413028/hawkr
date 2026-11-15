import { describe, expect, it } from "vitest";
import { page, userEvent } from "vitest/browser";
import { renderWithIntl } from "@/tests/browser/render";
import { Tabs } from "./Tabs";

const items = [
	{ key: "pending", label: "待處理", content: "待處理訂單" },
	{ key: "shipping", label: "配送中", content: "配送中的訂單" },
	{ key: "done", label: "已完成", content: "已完成訂單" },
];

describe("Tabs", () => {
	it("shows the clicked tab's panel", async () => {
		renderWithIntl(<Tabs aria-label="訂單狀態" items={items} />);
		await userEvent.click(page.getByRole("tab", { name: "配送中" }));
		await expect
			.poll(() => page.getByRole("tabpanel").element().textContent)
			.toContain("配送中的訂單");
	});

	it("moves focus to the next tab with ArrowRight", async () => {
		renderWithIntl(<Tabs aria-label="訂單狀態" items={items} />);
		(
			page.getByRole("tab", { name: "待處理" }).element() as HTMLElement
		).focus();
		await userEvent.keyboard("{ArrowRight}");
		await expect
			.element(page.getByRole("tab", { name: "配送中" }))
			.toHaveFocus();
	});

	it("shows the clicked tab's panel under the dark theme", async () => {
		renderWithIntl(<Tabs aria-label="訂單狀態" items={items} />, {
			theme: "dark",
		});
		await expect
			.element(document.documentElement)
			.toHaveAttribute("data-theme", "dark");
		await userEvent.click(page.getByRole("tab", { name: "配送中" }));
		await expect
			.poll(() => page.getByRole("tabpanel").element().textContent)
			.toContain("配送中的訂單");
	});
});
