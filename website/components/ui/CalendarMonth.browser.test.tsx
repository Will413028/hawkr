import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { page, userEvent } from "vitest/browser";
import { renderWithIntl } from "@/tests/browser/render";
import { CalendarMonth } from "./CalendarMonth";

const baseProps = {
	defaultMonth: new Date(2026, 8, 1),
	today: new Date(2026, 8, 12),
	selectedDate: new Date(2026, 8, 18),
	locale: "zh-TW",
	weekdayLabels: ["日", "一", "二", "三", "四", "五", "六"],
	showJumpPickers: false,
} satisfies Partial<React.ComponentProps<typeof CalendarMonth>>;

function Controlled(
	props: Partial<React.ComponentProps<typeof CalendarMonth>> & {
		onSelectDate?: (date: Date) => void;
	},
) {
	const [selectedDate, setSelectedDate] = useState(baseProps.selectedDate);
	return (
		<CalendarMonth
			{...baseProps}
			{...props}
			selectedDate={selectedDate}
			onSelectDate={(date) => {
				setSelectedDate(date);
				props.onSelectDate?.(date);
			}}
		/>
	);
}

describe("CalendarMonth", () => {
	it("selects the next day with ArrowRight and Enter", async () => {
		const onSelectDate = vi.fn();
		renderWithIntl(<Controlled onSelectDate={onSelectDate} />, {
			frameMaxWidth: 420,
		});
		await userEvent.click(
			page.getByRole("gridcell", { name: /2026年9月18日/ }),
		);
		await userEvent.keyboard("{ArrowRight}{Enter}");
		await expect
			.element(page.getByRole("gridcell", { name: /2026年9月19日/ }))
			.toHaveAttribute("aria-selected", "true");
		expect(onSelectDate).toHaveBeenLastCalledWith(new Date(2026, 8, 19));
	});

	it("disables dates outside minDate and maxDate", async () => {
		renderWithIntl(
			<Controlled
				minDate={new Date(2026, 8, 12)}
				maxDate={new Date(2026, 8, 25)}
			/>,
			{ frameMaxWidth: 420 },
		);
		await expect
			.element(page.getByRole("gridcell", { name: /2026年9月11日/ }))
			.toBeDisabled();
		await expect
			.element(page.getByRole("gridcell", { name: /2026年9月26日/ }))
			.toBeDisabled();
	});
});
