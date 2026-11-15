import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { page, userEvent } from "vitest/browser";
import { renderWithIntl } from "@/tests/browser/render";
import { DatePicker } from "./DatePicker";

type Props = React.ComponentProps<typeof DatePicker>;

const baseProps = {
	label: "希望到貨日期",
	value: "2026-09-18",
	minDate: "2026-09-12",
	maxDate: "2026-10-31",
	locale: "zh-TW",
} satisfies Partial<Props>;

function Controlled(props: Partial<Props>) {
	const merged = { ...baseProps, ...props } as Props;
	const [value, setValue] = useState(merged.value);
	return (
		<DatePicker
			{...merged}
			value={value}
			onChange={(next) => {
				setValue(next);
				merged.onChange?.(next);
			}}
		/>
	);
}

describe("DatePicker", () => {
	it("selects a date from the calendar and reports an ISO date", async () => {
		const onChange = vi.fn();
		renderWithIntl(<Controlled onChange={onChange} />, { frameMaxWidth: 420 });
		await userEvent.click(
			page.getByRole("button", { name: "開啟日期選擇", exact: true }),
		);
		await userEvent.click(
			page.getByRole("gridcell", { name: /2026年9月20日/ }),
		);
		await expect.element(page.getByRole("textbox")).toHaveValue("2026/09/20");
		expect(onChange).toHaveBeenCalledWith("2026-09-20");
	});

	it("describes the input with the error message when invalid", async () => {
		renderWithIntl(
			<Controlled isInvalid errorMessage="請重新選擇可配送的日期" />,
			{ frameMaxWidth: 420 },
		);
		await expect
			.element(page.getByRole("textbox"))
			.toHaveAccessibleDescription("請重新選擇可配送的日期");
	});

	it("disables the input and the calendar button when disabled", async () => {
		const onChange = vi.fn();
		renderWithIntl(<Controlled disabled onChange={onChange} />, {
			frameMaxWidth: 420,
		});
		await expect.element(page.getByRole("textbox")).toBeDisabled();
		await expect
			.element(page.getByRole("button", { name: "開啟日期選擇", exact: true }))
			.toBeDisabled();
		expect(onChange).not.toHaveBeenCalled();
	});

	it("keeps the error description under the dark theme", async () => {
		renderWithIntl(
			<Controlled isInvalid errorMessage="請重新選擇可配送的日期" />,
			{ frameMaxWidth: 420, theme: "dark" },
		);
		await expect
			.element(document.documentElement)
			.toHaveAttribute("data-theme", "dark");
		await expect
			.element(page.getByRole("textbox"))
			.toHaveAccessibleDescription("請重新選擇可配送的日期");
	});
});
