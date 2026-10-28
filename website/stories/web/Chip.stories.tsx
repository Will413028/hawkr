import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { Chip } from "@/components/ui/Chip";

const meta = {
	title: "Web/Data Display/Chip",
	component: Chip,
	args: { children: "宅配", color: "primary", variant: "flat" },
} satisfies Meta<typeof Chip>;
export default meta;
type Story = StoryObj<typeof meta>;
export const Default: Story = {};
export const Statuses: Story = {
	render: () => (
		<div className="catalog-row">
			<Chip color="success" variant="dot">
				已完成
			</Chip>
			<Chip color="warning" variant="dot">
				待確認
			</Chip>
			<Chip color="danger" variant="dot">
				配送異常
			</Chip>
			<Chip variant="bordered">尚未上架</Chip>
		</div>
	),
};
