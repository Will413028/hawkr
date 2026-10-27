import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect, userEvent } from "storybook/test";
import { Select } from "@/components/ui/Input";

const meta = {
	title: "Web/Forms/Select",
	component: Select,
	args: {
		label: "配送溫層",
		placeholder: "請選擇溫層",
		defaultValue: "",
		isRequired: true,
		children: (
			<>
				<option value="ambient">常溫</option>
				<option value="chilled">冷藏</option>
				<option value="frozen">冷凍</option>
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
export const ChooseTemperature: Story = {
	play: async ({ canvas }) => {
		const select = canvas.getByRole("combobox", { name: /配送溫層/ });
		await userEvent.selectOptions(select, "chilled");
		await expect(select).toHaveValue("chilled");
	},
};
export const Invalid: Story = {
	args: { isInvalid: true, errorMessage: "請選擇配送溫層" },
	play: async ({ canvas }) => {
		await expect(canvas.getByRole("combobox")).toHaveAttribute(
			"aria-invalid",
			"true",
		);
		await expect(canvas.getByRole("combobox")).toHaveAccessibleDescription(
			"請選擇配送溫層",
		);
	},
};
export const Disabled: Story = {
	args: {
		disabled: true,
		defaultValue: "frozen",
		description: "此商品的溫層已固定",
	},
	play: async ({ canvas }) => {
		await expect(canvas.getByRole("combobox")).toBeDisabled();
	},
};
