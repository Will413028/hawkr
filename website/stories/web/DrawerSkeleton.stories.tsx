import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect } from "storybook/test";
import { DrawerSkeleton } from "@/components/ui/DrawerSkeleton";

const meta = {
	title: "Web/Feedback/DrawerSkeleton",
	component: DrawerSkeleton,
	args: { rows: 2 },
	decorators: [
		(Story) => (
			<div className="w-full max-w-3xl">
				<Story />
			</div>
		),
	],
	parameters: {
		docs: {
			description: {
				component:
					"正式抽屜載入骨架。依最終內容選擇列數，保留載入前後的幾何；支援 reduced motion，不包含網路請求。",
			},
		},
	},
} satisfies Meta<typeof DrawerSkeleton>;
export default meta;
type Story = StoryObj<typeof meta>;
export const ProductDetails: Story = {
	play: async ({ canvas }) => {
		await expect(canvas.getByRole("status")).toHaveAttribute(
			"aria-busy",
			"true",
		);
		await expect(canvas.getByRole("status")).toHaveTextContent("載入中");
	},
};
export const OrderDetails: Story = { args: { rows: 3 } };

export const NarrowDrawer: Story = {
	decorators: [
		(Story) => (
			<div style={{ maxWidth: 288, width: "100%" }}>
				<Story />
			</div>
		),
	],
};
