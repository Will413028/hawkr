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
		renderWithIntl(<Controlled onCheckedChange={onCheckedChange} />);
		await userEvent.click(page.getByRole("switch"));
		await expect.element(page.getByRole("switch")).toBeChecked();
		expect(onCheckedChange).toHaveBeenCalledWith(true);
	});
});
