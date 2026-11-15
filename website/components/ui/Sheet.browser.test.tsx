import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { page, userEvent } from "vitest/browser";
import { renderWithIntl } from "@/tests/browser/render";
import Button from "./Button";
import { Sheet } from "./Dialog";

function Example({
	side,
	onClose,
}: {
	side?: React.ComponentProps<typeof Sheet>["side"];
	onClose: () => void;
}) {
	const [open, setOpen] = useState(false);
	return (
		<>
			<Button onClick={() => setOpen(true)}>查看明細</Button>
			<Sheet
				side={side}
				open={open}
				onClose={() => {
					setOpen(false);
					onClose();
				}}
				title="商品匯入明細"
				description="元件展示用資料，不會建立或修改商品。"
			>
				<p>核對品名、規格與圖片後，再回到商品清單繼續操作。</p>
			</Sheet>
		</>
	);
}

describe("Sheet", () => {
	it.each([
		["bottom", undefined],
		["right", "right"],
	] as const)(
		"opens as a %s sheet and closes on Escape",
		async (_name, side) => {
			const onClose = vi.fn();
			renderWithIntl(<Example side={side} onClose={onClose} />);
			await userEvent.click(
				page.getByRole("button", { name: "查看明細", exact: true }),
			);
			await expect
				.element(
					page.getByRole("dialog", { name: "商品匯入明細", exact: true }),
				)
				.toBeVisible();
			await userEvent.keyboard("{Escape}");
			await expect.element(page.getByRole("dialog")).not.toBeInTheDocument();
			expect(onClose).toHaveBeenCalledTimes(1);
		},
	);
});
