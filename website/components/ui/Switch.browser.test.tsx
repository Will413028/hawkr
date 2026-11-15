import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { page, userEvent } from "vitest/browser";
import { renderWithIntl } from "@/tests/browser/render";
import { Switch } from "./Switch";

function Controlled({
	onCheckedChange,
}: {
	onCheckedChange: (checked: boolean) => void;
}) {
	const [checked, setChecked] = useState(false);
	return (
		<div>
			<span>商品上架通知</span>
			<Switch
				ariaLabel="商品上架通知"
				checked={checked}
				onCheckedChange={(next) => {
					onCheckedChange(next);
					setChecked(next);
				}}
			/>
		</div>
	);
}

describe("Switch", () => {
	it("turns on when clicked and reports true", async () => {
		const onCheckedChange = vi.fn();
		const { container } = renderWithIntl(
			<Controlled onCheckedChange={onCheckedChange} />,
		);
		// role=switch 的 input 視覺上隱藏（react-aria），使用者點的是可見的軌道。
		await userEvent.click(
			page.elementLocator(
				container.querySelector("[data-slot=switch-control]") as HTMLElement,
			),
		);
		await expect.element(page.getByRole("switch")).toBeChecked();
		expect(onCheckedChange).toHaveBeenCalledWith(true);
	});
});
