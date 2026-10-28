import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect, fn, userEvent } from "storybook/test";
import { Card, CardBody, CardFooter, CardHeader } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";

const meta = {
	title: "Web/Data Display/Card",
	component: Card,
	decorators: [
		(Story) => (
			<div className="catalog-field">
				<Story />
			</div>
		),
	],
	render: (args) => (
		<Card {...args}>
			<CardHeader>
				<strong>花蓮小農香米</strong>
			</CardHeader>
			<CardBody>每日餐桌上的安心選擇。</CardBody>
			<CardFooter>
				<Chip variant="flat" color="primary">
					宅配
				</Chip>
			</CardFooter>
		</Card>
	),
} satisfies Meta<typeof Card>;
export default meta;
type Story = StoryObj<typeof meta>;
export const Default: Story = {};
export const Flat: Story = { args: { shadow: "none" } };
export const KeyboardPressable: Story = {
	args: { isPressable: true, "aria-label": "查看花蓮小農香米", onClick: fn() },
	play: async ({ canvas, args }) => {
		canvas.getByRole("button").focus();
		await userEvent.keyboard("{Enter}");
		await expect(args.onClick).toHaveBeenCalledOnce();
	},
};
