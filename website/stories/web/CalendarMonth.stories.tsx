import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";
import { expect, fn, userEvent } from "storybook/test";
import { CalendarMonth } from "@/components/ui/CalendarMonth";

const meta = {
	title: "Web/Forms/CalendarMonth",
	component: CalendarMonth,
	args: {
		defaultMonth: new Date(2026, 8, 1),
		today: new Date(2026, 8, 12),
		selectedDate: new Date(2026, 8, 18),
		locale: "zh-TW",
		weekdayLabels: ["日", "一", "二", "三", "四", "五", "六"],
		onSelectDate: fn(),
		showJumpPickers: false,
	},
	decorators: [
		(Story) => (
			<div className="catalog-field">
				<Story />
			</div>
		),
	],
	render: function Controlled(args) {
		const [selectedDate, setSelectedDate] = useState(args.selectedDate);
		return (
			<CalendarMonth
				{...args}
				selectedDate={selectedDate}
				onSelectDate={(date) => {
					setSelectedDate(date);
					args.onSelectDate?.(date);
				}}
			/>
		);
	},
	parameters: {
		docs: {
			description: {
				component:
					"DatePicker 使用的正式月曆 grid；固定 today 與月份，便於比較選取、今日、不可選與跨月狀態。",
			},
		},
	},
} satisfies Meta<typeof CalendarMonth>;
export default meta;
type Story = StoryObj<typeof meta>;
export const KeyboardDateSelection: Story = {
	play: async ({ canvas, args }) => {
		const selected = canvas.getByRole("gridcell", { name: /2026年9月18日/ });
		await userEvent.click(selected);
		await userEvent.keyboard("{ArrowRight}{Enter}");
		await expect(
			canvas.getByRole("gridcell", { name: /2026年9月19日/ }),
		).toHaveAttribute("aria-selected", "true");
		await expect(args.onSelectDate).toHaveBeenLastCalledWith(
			new Date(2026, 8, 19),
		);
	},
};
export const DateBounds: Story = {
	args: { minDate: new Date(2026, 8, 12), maxDate: new Date(2026, 8, 25) },
	play: async ({ canvas }) => {
		await expect(
			canvas.getByRole("gridcell", { name: /2026年9月11日/ }),
		).toBeDisabled();
		await expect(
			canvas.getByRole("gridcell", { name: /2026年9月26日/ }),
		).toBeDisabled();
	},
};
export const MinguoJumpPickers: Story = {
	args: {
		yearDisplayMode: "minguo",
		showJumpPickers: true,
		yearRange: { min: 2024, max: 2028 },
		showTaiwanYearFlag: true,
	},
};
