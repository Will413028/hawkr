import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";
import { expect, userEvent, waitFor, within } from "storybook/test";
import Button from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";

const meta = {
	title: "Web/Overlays/Dialog",
	component: Dialog,
	render: function Example() {
		const [open, setOpen] = useState(false);
		return (
			<>
				<Button onClick={() => setOpen(true)}>查看配送說明</Button>
				<Dialog
					open={open}
					onClose={() => setOpen(false)}
					title="配送說明"
					description="預覽用的靜態內容，不會修改訂單。"
				>
					<p>請依商品標示的保存方式收貨。</p>
				</Dialog>
			</>
		);
	},
} satisfies Meta<typeof Dialog>;
export default meta;
type Story = StoryObj<typeof meta>;
export const OpenAndDismiss: Story = {
	args: { open: false, onClose: () => {}, children: null },
	play: async ({ canvas, canvasElement }) => {
		await userEvent.click(canvas.getByRole("button", { name: "查看配送說明" }));
		const body = within(canvasElement.ownerDocument.body);
		await waitFor(() =>
			expect(body.getByRole("dialog", { name: "配送說明" })).toBeVisible(),
		);
		await userEvent.keyboard("{Escape}");
		await waitFor(() =>
			expect(body.queryByRole("dialog")).not.toBeInTheDocument(),
		);
	},
};
