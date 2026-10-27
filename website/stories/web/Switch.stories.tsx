import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useEffect, useState } from "react";
import { expect, fn, userEvent } from "storybook/test";
import { Switch } from "@/components/ui/Switch";

const meta = {
	title: "Web/Forms/Switch",
	component: Switch,
	args: { checked: false, ariaLabel: "商品上架通知", onCheckedChange: fn() },
	render: function Controlled(args) {
		const [checked, setChecked] = useState(args.checked);
		useEffect(() => setChecked(args.checked), [args.checked]);
		return (
			<div className="catalog-row">
				<span>商品上架通知</span>
				<Switch
					{...args}
					checked={checked}
					onCheckedChange={(checked) => {
						args.onCheckedChange(checked);
						setChecked(checked);
					}}
				/>
			</div>
		);
	},
} satisfies Meta<typeof Switch>;
export default meta;
type Story = StoryObj<typeof meta>;
export const Default: Story = {
	play: async ({ canvas, args }) => {
		await userEvent.click(canvas.getByRole("switch"));
		await expect(canvas.getByRole("switch")).toBeChecked();
		await expect(args.onCheckedChange).toHaveBeenCalledWith(true);
	},
};
export const Enabled: Story = { args: { checked: true } };
export const Disabled: Story = { args: { disabled: true } };
export const Busy: Story = { args: { busy: true, checked: true } };
