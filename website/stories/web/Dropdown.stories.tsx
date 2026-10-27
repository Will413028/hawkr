import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";
import { expect, fn, userEvent, waitFor, within } from "storybook/test";
import { Dropdown } from "@/components/ui/Dropdown";

const meta = {
	title: "Web/Forms/Dropdown",
	component: Dropdown,
	args: {
		value: undefined,
		onChange: fn(),
		label: "產地",
		"aria-label": "產地",
		placeholder: "請選擇產地",
		items: [
			{ value: "hualien", label: "花蓮縣", description: "東部產區" },
			{ value: "taitung", label: "臺東縣", description: "東部產區" },
			{
				value: "nantou",
				label: "南投縣",
				disabled: true,
				description: "暫不可選",
			},
		],
	},
	decorators: [
		(Story) => (
			<div className="catalog-field">
				<Story />
			</div>
		),
	],
	render: function Controlled(args) {
		const [value, setValue] = useState(args.value);
		return (
			<Dropdown
				{...args}
				value={value}
				onChange={(next) => {
					setValue(next);
					args.onChange(next);
				}}
			/>
		);
	},
	parameters: {
		docs: {
			description: {
				component:
					"正式站共用 Dropdown：portal 選單、鍵盤選取與可選搜尋。對照 App 的 AppDropdown；此處選項為固定展示資料。",
			},
		},
	},
} satisfies Meta<typeof Dropdown>;
export default meta;
type Story = StoryObj<typeof meta>;
export const KeyboardSelection: Story = {
	play: async ({ canvas, args }) => {
		const trigger = canvas.getByRole("button", { name: "產地" });
		await userEvent.click(trigger);
		await userEvent.keyboard("{Home}{Enter}");
		await expect(trigger).toHaveTextContent("花蓮縣");
		await expect(args.onChange).toHaveBeenCalledWith("hualien");
		await expect(trigger).toHaveAttribute("aria-expanded", "false");
	},
};
export const SearchAndSelect: Story = {
	args: { searchable: true },
	play: async ({ canvas, canvasElement, args }) => {
		await userEvent.click(canvas.getByRole("button", { name: "產地" }));
		const page = within(canvasElement.ownerDocument.body);
		await userEvent.type(
			page.getByRole("combobox", { name: "搜尋選項" }),
			"臺東",
		);
		await userEvent.click(page.getByRole("option", { name: /臺東縣/ }));
		await expect(
			canvas.getByRole("button", { name: "產地" }),
		).toHaveTextContent("臺東縣");
		await expect(args.onChange).toHaveBeenCalledWith("taitung");
	},
};
export const NoSearchResults: Story = {
	args: { searchable: true },
	play: async ({ canvas, canvasElement, args }) => {
		await userEvent.click(canvas.getByRole("button", { name: "產地" }));
		const page = within(canvasElement.ownerDocument.body);
		await userEvent.type(
			page.getByRole("combobox", { name: "搜尋選項" }),
			"不存在的產地",
		);
		// The menu enters with an opacity transition; wait for the visible
		// result rather than racing its first animation frame in a full run.
		await waitFor(() =>
			expect(page.getByText("找不到符合的選項")).toBeVisible(),
		);
		await userEvent.keyboard("{Escape}");
		await expect(args.onChange).not.toHaveBeenCalled();
	},
};
export const Invalid: Story = {
	args: { isInvalid: true, errorMessage: "請選擇商品產地" },
};

export const DarkInvalid: Story = {
	args: { isInvalid: true, errorMessage: "請選擇商品產地" },
	globals: { theme: "dark" },
	play: async ({ canvasElement, canvas }) => {
		await expect(canvasElement.ownerDocument.documentElement).toHaveAttribute(
			"data-theme",
			"dark",
		);
		await expect(
			canvas.getByRole("button", { name: "產地" }),
		).toHaveAccessibleDescription("請選擇商品產地");
	},
};
export const Disabled: Story = {
	args: { disabled: true, value: "hualien" },
	play: async ({ canvas, args }) => {
		await expect(canvas.getByRole("button", { name: "產地" })).toBeDisabled();
		await expect(args.onChange).not.toHaveBeenCalled();
	},
};
