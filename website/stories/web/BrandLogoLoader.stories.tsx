import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import BrandLogoLoader from "@/components/shared/BrandLogoLoader";

const meta = {
	title: "Web/Feedback/BrandLogoLoader",
	component: BrandLogoLoader,
	args: { size: 64, tone: "brand" },
	parameters: {
		docs: {
			description: {
				component:
					"正式站的四葉商標載入動畫。brand 放中性底；實心品牌色底必須指定 on-light 或 on-dark。按鈕內使用 mono 繼承可讀前景色。",
			},
		},
	},
} satisfies Meta<typeof BrandLogoLoader>;
export default meta;
type Story = StoryObj<typeof meta>;
export const Default: Story = {};
export const OnPrimaryStrong: Story = {
	args: { tone: "on-dark" },
	decorators: [
		(Story) => (
			<div
				style={{
					background: "var(--color-brand-primary-strong)",
					padding: 24,
					display: "inline-flex",
				}}
			>
				<Story />
			</div>
		),
	],
};
