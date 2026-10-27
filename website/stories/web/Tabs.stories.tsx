import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect, userEvent } from "storybook/test";
import { Tabs } from "@/components/ui/Tabs";

const meta = {
	title: "Web/Navigation/Tabs",
	component: Tabs,
	args: {
		"aria-label": "訂單狀態",
		items: [
			{ key: "pending", label: "待處理", content: "待處理訂單" },
			{ key: "shipping", label: "配送中", content: "配送中的訂單" },
			{ key: "done", label: "已完成", content: "已完成訂單" },
		],
	},
} satisfies Meta<typeof Tabs>;
export default meta;
type Story = StoryObj<typeof meta>;
export const Default: Story = {
	play: async ({ canvas }) => {
		await userEvent.click(canvas.getByRole("tab", { name: "配送中" }));
		await expect(canvas.getByRole("tabpanel")).toHaveTextContent(
			"配送中的訂單",
		);
	},
};
export const Keyboard: Story = {
	play: async ({ canvas }) => {
		canvas.getByRole("tab", { name: "待處理" }).focus();
		await userEvent.keyboard("{ArrowRight}");
		await expect(canvas.getByRole("tab", { name: "配送中" })).toHaveFocus();
	},
};
export const Underlined: Story = { args: { variant: "underlined" } };

export const DarkDefault: Story = {
	globals: { theme: "dark" },
	play: async ({ canvasElement, canvas }) => {
		await expect(canvasElement.ownerDocument.documentElement).toHaveAttribute(
			"data-theme",
			"dark",
		);
		await userEvent.click(canvas.getByRole("tab", { name: "配送中" }));
		await expect(canvas.getByRole("tabpanel")).toHaveTextContent(
			"配送中的訂單",
		);
	},
};
