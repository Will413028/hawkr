import { describe, expect, it } from "vitest";
import { page, userEvent } from "vitest/browser";
import { renderWithIntl } from "@/tests/browser/render";
import { Select } from "./Input";

const options = (
	<>
		<option value="home">宅配</option>
		<option value="store">超商取貨</option>
	</>
);

describe("Select", () => {
	it("chooses a delivery method", async () => {
		renderWithIntl(
			<Select
				label="配送方式"
				placeholder="請選擇配送方式"
				defaultValue=""
				isRequired
			>
				{options}
			</Select>,
		);
		const select = page.getByRole("combobox", { name: /配送方式/ });
		await userEvent.selectOptions(select, "store");
		await expect.element(select).toHaveValue("store");
	});

	it("marks itself invalid and describes the error", async () => {
		renderWithIntl(
			<Select
				label="配送方式"
				placeholder="請選擇配送方式"
				defaultValue=""
				isRequired
				isInvalid
				errorMessage="請選擇配送方式"
			>
				{options}
			</Select>,
		);
		const select = page.getByRole("combobox");
		await expect.element(select).toHaveAttribute("aria-invalid", "true");
		await expect.element(select).toHaveAccessibleDescription("請選擇配送方式");
	});

	it("is disabled when disabled", async () => {
		renderWithIntl(
			<Select
				label="配送方式"
				placeholder="請選擇配送方式"
				isRequired
				disabled
				defaultValue="store"
				description="此商品的配送方式已固定"
			>
				{options}
			</Select>,
		);
		await expect.element(page.getByRole("combobox")).toBeDisabled();
	});
});
