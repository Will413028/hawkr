import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { page, userEvent } from "vitest/browser";
import { renderWithIntl } from "@/tests/browser/render";
import { Dropdown } from "./Dropdown";

type Props = React.ComponentProps<typeof Dropdown>;

const baseProps = {
	value: undefined,
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
} satisfies Partial<Props>;

function Controlled(props: Partial<Props>) {
	const merged = { ...baseProps, ...props } as Props;
	const [value, setValue] = useState(merged.value);
	return (
		<Dropdown
			{...merged}
			value={value}
			onChange={(next) => {
				setValue(next);
				merged.onChange?.(next);
			}}
		/>
	);
}

const trigger = () => page.getByRole("button", { name: "產地", exact: true });

describe("Dropdown", () => {
	it("selects the first item with Home and Enter and closes the menu", async () => {
		const onChange = vi.fn();
		renderWithIntl(<Controlled onChange={onChange} />, { frameMaxWidth: 420 });
		await userEvent.click(trigger());
		await userEvent.keyboard("{Home}{Enter}");
		await expect
			.poll(() => trigger().element().textContent)
			.toContain("花蓮縣");
		expect(onChange).toHaveBeenCalledWith("hualien");
		await expect.element(trigger()).toHaveAttribute("aria-expanded", "false");
	});

	it("filters by search text and selects the matching option", async () => {
		const onChange = vi.fn();
		renderWithIntl(<Controlled searchable onChange={onChange} />, {
			frameMaxWidth: 420,
		});
		await userEvent.click(trigger());
		await userEvent.type(
			page.getByRole("combobox", { name: "搜尋選項", exact: true }),
			"臺東",
		);
		await userEvent.click(page.getByRole("option", { name: /臺東縣/ }));
		await expect
			.poll(() => trigger().element().textContent)
			.toContain("臺東縣");
		expect(onChange).toHaveBeenCalledWith("taitung");
	});

	it("shows the empty state when nothing matches and selects nothing", async () => {
		const onChange = vi.fn();
		renderWithIntl(<Controlled searchable onChange={onChange} />, {
			frameMaxWidth: 420,
		});
		await userEvent.click(trigger());
		await userEvent.type(
			page.getByRole("combobox", { name: "搜尋選項", exact: true }),
			"不存在的產地",
		);
		await expect
			.element(page.getByText("找不到符合的選項", { exact: true }))
			.toBeVisible();
		await userEvent.keyboard("{Escape}");
		expect(onChange).not.toHaveBeenCalled();
	});

	it("describes the trigger with the error message under the dark theme", async () => {
		renderWithIntl(<Controlled isInvalid errorMessage="請選擇商品產地" />, {
			frameMaxWidth: 420,
			theme: "dark",
		});
		await expect
			.element(document.documentElement)
			.toHaveAttribute("data-theme", "dark");
		await expect
			.element(trigger())
			.toHaveAccessibleDescription("請選擇商品產地");
	});

	it("is disabled and does not report a change when disabled", async () => {
		const onChange = vi.fn();
		renderWithIntl(
			<Controlled disabled value="hualien" onChange={onChange} />,
			{ frameMaxWidth: 420 },
		);
		await expect.element(trigger()).toBeDisabled();
		expect(onChange).not.toHaveBeenCalled();
	});
});
