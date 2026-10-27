import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";
import { expect, fn, userEvent, waitFor, within } from "storybook/test";
import Button from "@/components/ui/Button";
import { Sheet } from "@/components/ui/Dialog";
import { DrawerSkeleton } from "@/components/ui/DrawerSkeleton";

const meta = {
	title: "Web/Overlays/Sheet",
	component: Sheet,
	args: {
		open: false,
		onClose: fn(),
		title: "商品匯入明細",
		description: "元件展示用資料，不會建立或修改商品。",
		children: <p>核對品名、規格與圖片後，再回到商品清單繼續操作。</p>,
	},
	render: function Controlled(args) {
		const [open, setOpen] = useState(args.open);
		return (
			<>
				<Button onClick={() => setOpen(true)}>查看明細</Button>
				<Sheet
					{...args}
					open={open}
					onClose={() => {
						setOpen(false);
						args.onClose();
					}}
				/>
			</>
		);
	},
} satisfies Meta<typeof Sheet>;
export default meta;
type Story = StoryObj<typeof meta>;
export const BottomSheet: Story = {
	play: async ({ canvas, canvasElement, args }) => {
		const trigger = canvas.getByRole("button", { name: "查看明細" });
		await userEvent.click(trigger);
		const page = within(canvasElement.ownerDocument.body);
		await waitFor(() =>
			expect(page.getByRole("dialog", { name: "商品匯入明細" })).toBeVisible(),
		);
		await userEvent.keyboard("{Escape}");
		await waitFor(() =>
			expect(page.queryByRole("dialog")).not.toBeInTheDocument(),
		);
		await expect(args.onClose).toHaveBeenCalledOnce();
	},
};
export const RightSheet: Story = {
	args: { side: "right" },
	play: BottomSheet.play,
};
export const LoadingDetails: Story = {
	args: { side: "right", children: <DrawerSkeleton rows={3} /> },
};
