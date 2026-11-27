import { screen, within } from "@testing-library/dom";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { userEvent } from "vitest/browser";
import { expectAbsent, waitFor } from "@/tests/browser/dom";
import { renderWithIntl } from "@/tests/browser/render";
import { DescriptionColorPicker } from "./DescriptionColorPicker";

function Controlled({
	onApply,
	onOpen,
}: {
	onApply: (hex: string) => void;
	onOpen: () => void;
}) {
	const [value, setValue] = useState("#b8441f");
	return (
		<div style={{ minHeight: 600, padding: 16 }}>
			<DescriptionColorPicker
				value={value}
				onOpen={onOpen}
				onApply={(hex) => {
					setValue(hex);
					onApply(hex);
				}}
			/>
		</div>
	);
}

async function waitForPalette() {
	const panel = await screen.findByRole(
		"dialog",
		{ name: "文字顏色" },
		{ timeout: 5000 },
	);
	await expect.element(panel).toBeVisible();
	await expect
		.element(within(panel).getByRole("slider", { name: "色相" }))
		.toBeVisible();
	await expect
		.element(within(panel).getByRole("slider", { name: "飽和度與亮度" }))
		.toHaveFocus();
	return panel;
}

describe("DescriptionColorPicker", () => {
	it("adjusts the area and hue by keyboard", async () => {
		const onApply = vi.fn();
		const onOpen = vi.fn();
		const { container } = renderWithIntl(
			<Controlled onApply={onApply} onOpen={onOpen} />,
		);
		await userEvent.click(
			within(container).getByRole("button", { name: "文字顏色" }),
		);
		const panel = within(await waitForPalette());
		const area = panel.getByRole("slider", { name: "飽和度與亮度" });
		const before = Number(area.getAttribute("aria-valuenow"));
		await userEvent.keyboard("{ArrowRight}");
		await waitFor(() =>
			expect(Number(area.getAttribute("aria-valuenow"))).toBe(before + 1),
		);
		const brightness = Number(
			/亮度 (\d+)%/.exec(area.getAttribute("aria-valuetext") ?? "")?.[1],
		);
		await userEvent.keyboard("{Shift>}{ArrowUp}{/Shift}");
		await waitFor(() =>
			expect(area.getAttribute("aria-valuetext")).toContain(
				`亮度 ${brightness + 10}%`,
			),
		);
		const hue = panel.getByRole("slider", { name: "色相" });
		hue.focus();
		await userEvent.keyboard("{End}");
		await expect.element(hue).toHaveValue("359");
		expect(onOpen).toHaveBeenCalledTimes(1);
		expect(onApply).not.toHaveBeenCalled();
		await userEvent.click(panel.getByRole("button", { name: "套用顏色" }));
		await waitFor(() => expect(onApply).toHaveBeenCalledTimes(1));
		expect(onApply).toHaveBeenCalledWith(
			expect.stringMatching(/^#[\da-f]{6}$/),
		);
	});

	it("validates HEX and RGB input before applying", async () => {
		const onApply = vi.fn();
		const { container } = renderWithIntl(
			<Controlled onApply={onApply} onOpen={vi.fn()} />,
		);
		await userEvent.click(
			within(container).getByRole("button", { name: "文字顏色" }),
		);
		const panel = within(await waitForPalette());
		const input = panel.getByRole("textbox", { name: "HEX 色碼" });
		await userEvent.clear(input);
		await userEvent.type(input, "#12345678");
		await expect.element(input).toHaveAttribute("aria-invalid", "true");
		await expect
			.element(panel.getByRole("button", { name: "套用顏色" }))
			.toBeDisabled();
		expect(onApply).not.toHaveBeenCalled();
		await userEvent.clear(input);
		await userEvent.type(input, "#6194d0");
		await userEvent.selectOptions(
			panel.getByRole("combobox", { name: "色碼格式" }),
			"rgb",
		);
		const red = panel.getByRole("textbox", { name: "紅色 R" });
		const green = panel.getByRole("textbox", { name: "綠色 G" });
		const blue = panel.getByRole("textbox", { name: "藍色 B" });
		await expect.element(red).toHaveValue("97");
		await expect.element(green).toHaveValue("148");
		await expect.element(blue).toHaveValue("208");
		await userEvent.clear(red);
		await userEvent.type(red, "1000");
		await expect.element(red).toHaveAttribute("aria-invalid", "true");
		await expect
			.element(panel.getByRole("button", { name: "套用顏色" }))
			.toBeDisabled();
		for (const [field, next] of [
			[red, "24"],
			[green, "128"],
			[blue, "255"],
		] as const) {
			await userEvent.clear(field);
			await userEvent.type(field, next);
		}
		await userEvent.click(panel.getByRole("button", { name: "套用顏色" }));
		await waitFor(() => expect(onApply).toHaveBeenCalledTimes(1));
		expect(onApply).toHaveBeenCalledWith("#1880ff");
	});

	it("keeps the original color on cancel and Escape", async () => {
		const onApply = vi.fn();
		const onOpen = vi.fn();
		const { container } = renderWithIntl(
			<Controlled onApply={onApply} onOpen={onOpen} />,
		);
		const trigger = within(container).getByRole("button", { name: "文字顏色" });
		trigger.focus();
		await userEvent.keyboard("{Enter}");
		let panel = within(await waitForPalette());
		panel.getByRole("button", { name: "關閉色盤" }).focus();
		await userEvent.tab({ shift: true });
		await expect
			.element(panel.getByRole("button", { name: "套用顏色" }))
			.toHaveFocus();
		await userEvent.tab();
		await expect
			.element(panel.getByRole("button", { name: "關閉色盤" }))
			.toHaveFocus();
		await userEvent.click(panel.getByRole("button", { name: "深藍 #1e5fab" }));
		await userEvent.keyboard("{Escape}");
		await expectAbsent(() =>
			screen.queryByRole("dialog", { name: "文字顏色" }),
		);
		await expect.element(trigger).toHaveFocus();
		expect(onApply).not.toHaveBeenCalled();
		await userEvent.keyboard(" ");
		panel = within(await waitForPalette());
		await expect
			.element(panel.getByRole("textbox", { name: "HEX 色碼" }))
			.toHaveValue("#b8441f");
		await userEvent.click(panel.getByRole("button", { name: "柿橙 #ea6021" }));
		await userEvent.click(panel.getByRole("button", { name: "取消" }));
		await expect.element(trigger).toHaveFocus();
		expect(onOpen).toHaveBeenCalledTimes(2);
		expect(onApply).not.toHaveBeenCalled();
	});

	it("shows brand colors on a narrow dark layout and ignores a disabled trigger", async () => {
		const onApply = vi.fn();
		const onOpen = vi.fn();
		const { container } = renderWithIntl(
			<div
				style={{
					width: "100%",
					maxWidth: 320,
					minHeight: 600,
					padding: 8,
					display: "flex",
					alignItems: "flex-start",
					gap: 12,
				}}
			>
				<DescriptionColorPicker
					value="#b8441f"
					onApply={onApply}
					onOpen={onOpen}
				/>
				<DescriptionColorPicker
					value="#b8441f"
					onApply={onApply}
					onOpen={onOpen}
					disabled
				/>
			</div>,
			{ theme: "dark" },
		);
		const [enabled, disabled] = within(container).getAllByRole("button", {
			name: "文字顏色",
		});
		await expect.element(disabled).toBeDisabled();
		await userEvent.click(disabled, { force: true });
		expect(onOpen).not.toHaveBeenCalled();
		await userEvent.click(enabled);
		const element = await waitForPalette();
		const panel = within(element);
		await userEvent.click(panel.getByRole("button", { name: "深藍 #1e5fab" }));
		await expect
			.element(panel.getByRole("button", { name: "深藍 #1e5fab" }))
			.toHaveAttribute("aria-pressed", "true");
		expect(onApply).not.toHaveBeenCalled();
		const bounds = element.getBoundingClientRect();
		expect(bounds.left).toBeGreaterThanOrEqual(0);
		expect(bounds.right).toBeLessThanOrEqual(
			document.documentElement.clientWidth,
		);
	});
});
