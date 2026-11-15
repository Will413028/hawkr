import { describe, expect, it, vi } from "vitest";
import { page, userEvent } from "vitest/browser";
import { renderWithIntl } from "@/tests/browser/render";
import Button from "./Button";

describe("Button", () => {
	it("calls onClick once when the primary button is clicked", async () => {
		const onClick = vi.fn();
		renderWithIntl(
			<Button color="primary" size="lg" onClick={onClick}>
				加入購物車
			</Button>,
		);
		await userEvent.click(page.getByRole("button", { name: "加入購物車" }));
		expect(onClick).toHaveBeenCalledTimes(1);
	});

	it("is disabled when disabled is set", async () => {
		renderWithIntl(
			<Button color="primary" size="lg" disabled>
				暫無庫存
			</Button>,
		);
		await expect.element(page.getByRole("button")).toBeDisabled();
	});

	it("is disabled and aria-busy while loading", async () => {
		renderWithIntl(
			<Button color="primary" size="lg" isLoading loadingLabel="加入中…">
				加入購物車
			</Button>,
		);
		const button = page.getByRole("button");
		await expect.element(button).toBeDisabled();
		await expect.element(button).toHaveAttribute("aria-busy", "true");
	});

	it("keeps working under the dark theme", async () => {
		const onClick = vi.fn();
		renderWithIntl(
			<Button color="danger" size="lg" onClick={onClick}>
				移除商品
			</Button>,
		);
		document.documentElement.dataset.theme = "dark";
		await expect
			.element(document.documentElement)
			.toHaveAttribute("data-theme", "dark");
		await userEvent.click(page.getByRole("button", { name: "移除商品" }));
		expect(onClick).toHaveBeenCalledTimes(1);
	});
});
