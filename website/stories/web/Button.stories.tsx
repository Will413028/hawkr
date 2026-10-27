import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect, fn, userEvent, within } from "storybook/test";
import Button from "@/components/ui/Button";

const meta = {
	title: "Web/Actions/Button",
	component: Button,
	args: { children: "加入購物車", color: "primary", size: "lg", onClick: fn() },
	parameters: {
		docs: {
			description: {
				component:
					"正式站共用 Button。主要觸控操作採 size=lg（48px）；Web md 為 44px。等待狀態沿用品牌 loader 與固定幾何，禁止自行重畫按鈕。",
			},
		},
	},
} satisfies Meta<typeof Button>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Primary: Story = {
	play: async ({ canvasElement, args }) => {
		await userEvent.click(
			within(canvasElement).getByRole("button", { name: "加入購物車" }),
		);
		await expect(args.onClick).toHaveBeenCalledOnce();
	},
};
export const Secondary: Story = {
	args: { variant: "bordered", children: "繼續選購" },
};
export const Quiet: Story = {
	args: { variant: "light", children: "稍後再說" },
};
export const Disabled: Story = {
	args: { disabled: true, children: "暫無庫存" },
	play: async ({ canvasElement }) => {
		await expect(within(canvasElement).getByRole("button")).toBeDisabled();
	},
};
export const Loading: Story = {
	args: { isLoading: true, loadingLabel: "加入中…" },
	play: async ({ canvasElement }) => {
		await expect(within(canvasElement).getByRole("button")).toBeDisabled();
		await expect(within(canvasElement).getByRole("button")).toHaveAttribute(
			"aria-busy",
			"true",
		);
	},
};
export const Destructive: Story = {
	args: { color: "danger", children: "移除商品" },
};

export const DarkDestructive: Story = {
	args: { color: "danger", children: "移除商品" },
	globals: { theme: "dark" },
	play: async ({ canvasElement, canvas, args }) => {
		await expect(canvasElement.ownerDocument.documentElement).toHaveAttribute(
			"data-theme",
			"dark",
		);
		await userEvent.click(canvas.getByRole("button", { name: "移除商品" }));
		await expect(args.onClick).toHaveBeenCalledOnce();
	},
};
export const CompactLargeText: Story = {
	globals: { textScale: "1.4" },
	args: { fullWidth: true, children: "儲存商品並繼續編輯" },
	decorators: [
		(Story) => (
			<div style={{ width: 288, maxWidth: "100%" }}>
				<Story />
			</div>
		),
	],
};
