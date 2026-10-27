import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect, userEvent } from "storybook/test";
import { Textarea } from "@/components/ui/Input";

const meta = {
	title: "Web/Forms/Textarea",
	component: Textarea,
	args: {
		label: "商品說明",
		placeholder: "分享產地、風味與保存方式",
		minRows: 3,
	},
	decorators: [
		(Story) => (
			<div className="catalog-field">
				<Story />
			</div>
		),
	],
} satisfies Meta<typeof Textarea>;
export default meta;
type Story = StoryObj<typeof meta>;
export const Default: Story = {
	play: async ({ canvas }) => {
		await userEvent.type(canvas.getByRole("textbox"), "來自花蓮的當季收成");
		await expect(canvas.getByRole("textbox")).toHaveValue("來自花蓮的當季收成");
	},
};
export const Invalid: Story = {
	args: { errorMessage: "請補上商品說明", isInvalid: true },
};
export const Disabled: Story = {
	args: { disabled: true, defaultValue: "商品審核中" },
};
