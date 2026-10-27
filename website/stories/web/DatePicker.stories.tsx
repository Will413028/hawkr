import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";
import { expect, fn, userEvent, within } from "storybook/test";
import { DatePicker } from "@/components/ui/DatePicker";

const meta = {
	title: "Web/Forms/DatePicker",
	component: DatePicker,
	args: {
		label: "希望到貨日期",
		value: "2026-09-18",
		minDate: "2026-09-12",
		maxDate: "2026-10-31",
		onChange: fn(),
		locale: "zh-TW",
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
			<DatePicker
				{...args}
				value={value}
				onChange={(next) => {
					setValue(next);
					args.onChange?.(next);
				}}
			/>
		);
	},
	parameters: {
		docs: {
			description: {
				component:
					"正式站日期／日期時間欄位，展示固定日期範圍，不查詢配送時段。顯示格式與輸出的 YYYY-MM-DD 分開處理。",
			},
		},
	},
} satisfies Meta<typeof DatePicker>;
export default meta;
type Story = StoryObj<typeof meta>;
export const CalendarSelection: Story = {
	play: async ({ canvas, canvasElement, args }) => {
		await userEvent.click(canvas.getByRole("button", { name: "開啟日期選擇" }));
		const page = within(canvasElement.ownerDocument.body);
		const date = page.getByRole("gridcell", { name: /2026年9月20日/ });
		await userEvent.click(date);
		await expect(canvas.getByRole("textbox")).toHaveValue("2026/09/20");
		await expect(args.onChange).toHaveBeenCalledWith("2026-09-20");
	},
};
export const Invalid: Story = {
	args: { isInvalid: true, errorMessage: "請重新選擇可配送的日期" },
	play: async ({ canvas }) => {
		await expect(canvas.getByRole("textbox")).toHaveAccessibleDescription(
			"請重新選擇可配送的日期",
		);
	},
};
export const Disabled: Story = {
	args: { disabled: true },
	play: async ({ canvas, args }) => {
		await expect(canvas.getByRole("textbox")).toBeDisabled();
		await expect(
			canvas.getByRole("button", { name: "開啟日期選擇" }),
		).toBeDisabled();
		await expect(args.onChange).not.toHaveBeenCalled();
	},
};
export const DateTime: Story = {
	args: {
		granularity: "date-time",
		label: "取貨時間",
		value: "2026-09-18T14:30",
	},
};
export const MinguoYear: Story = {
	args: { yearMode: "minguo", description: "畫面顯示民國年，輸出仍是西元日期" },
};

export const DarkInvalid: Story = {
	args: { isInvalid: true, errorMessage: "請重新選擇可配送的日期" },
	globals: { theme: "dark" },
	play: async ({ canvasElement, canvas }) => {
		await expect(canvasElement.ownerDocument.documentElement).toHaveAttribute(
			"data-theme",
			"dark",
		);
		await expect(canvas.getByRole("textbox")).toHaveAccessibleDescription(
			"請重新選擇可配送的日期",
		);
	},
};
