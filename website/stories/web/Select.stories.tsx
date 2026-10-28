import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect, userEvent } from "storybook/test";
import { Select } from "@/components/ui/Input";

const meta = {
	title: "Web/Forms/Select",
	component: Select,
	args: {
		label: "配送方式",
		placeholder: "請選擇配送方式",
		defaultValue: "",
		isRequired: true,
		children: (
			<>
				<option value="home">宅配</option>
				<option value="store">超商取貨</option>
			</>
		),
	},
	decorators: [
		(Story) => (
			<div className="catalog-field">
				<Story />
			</div>
		),
	],
	parameters: {
		docs: {
			description: {
				component:
					"正式站原生 Select，適合少量固定選項。搜尋與較長選單使用 Dropdown。",
			},
		},
	},
} satisfies Meta<typeof Select>;
export default meta;
type Story = StoryObj<typeof meta>;
export const ChooseDeliveryMethod: Story = {
	play: async ({ canvas }) => {
		const select = canvas.getByRole("combobox", { name: /配送方式/ });
		await userEvent.selectOptions(select, "store");
		await expect(select).toHaveValue("store");
	},
};
export const Invalid: Story = {
	args: { isInvalid: true, errorMessage: "請選擇配送方式" },
	play: async ({ canvas }) => {
		await expect(canvas.getByRole("combobox")).toHaveAttribute(
			"aria-invalid",
			"true",
		);
		await expect(canvas.getByRole("combobox")).toHaveAccessibleDescription(
			"請選擇配送方式",
		);
	},
};
export const Disabled: Story = {
	args: {
		disabled: true,
		defaultValue: "store",
		description: "此商品的配送方式已固定",
	},
	play: async ({ canvas }) => {
		await expect(canvas.getByRole("combobox")).toBeDisabled();
	},
};
