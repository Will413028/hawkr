import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";
import { expect, fn, userEvent, waitFor, within } from "storybook/test";
import { DescriptionColorPicker } from "@/features/seller/components/DescriptionColorPicker";

const meta = {
	title: "Web/Seller Center/DescriptionColorPicker",
	component: DescriptionColorPicker,
	args: { value: "#b8441f", onApply: fn(), onOpen: fn() },
	parameters: {
		docs: {
			description: {
				component:
					"商品描述文字色盤。色域、色相、HEX／RGB 與品牌常用色只更新本地草稿；按「套用顏色」才呼叫 onApply，取消與 Escape 不變更內容。onOpen 在指標焦點移入前保存文字選取；全螢幕編輯器透過 portalContainer 保留原生 dialog 的 top layer。僅儲存不透明 #rrggbb，無圖片上傳或交易請求。",
			},
		},
	},
	render: (args) => {
		const [value, setValue] = useState(args.value);
		return (
			<div style={{ minHeight: 600, padding: 16 }}>
				<DescriptionColorPicker
					{...args}
					value={value}
					onApply={(hex) => {
						setValue(hex);
						args.onApply(hex);
					}}
				/>
			</div>
		);
	},
} satisfies Meta<typeof DescriptionColorPicker>;
export default meta;
type Story = StoryObj<typeof meta>;

async function waitForPalette(canvasElement: HTMLElement) {
	// Wait for the portal's measured position and initial keyboard focus.
	return waitFor(() => {
		const panel = within(canvasElement.ownerDocument.body).getByRole("dialog", {
			name: "文字顏色",
		});
		expect(panel).toBeVisible();
		expect(within(panel).getByRole("slider", { name: "色相" })).toBeVisible();
		expect(
			within(panel).getByRole("slider", { name: "飽和度與亮度" }),
		).toHaveFocus();
		return panel;
	});
}

export const AreaAndHue: Story = {
	name: "色域與色相 · 鍵盤調色",
	play: async ({ canvas, canvasElement, args, globals }) => {
		await userEvent.click(canvas.getByRole("button", { name: "文字顏色" }));
		const panel = within(await waitForPalette(canvasElement));
		const area = panel.getByRole("slider", { name: "飽和度與亮度" });
		const before = Number(area.getAttribute("aria-valuenow"));
		await userEvent.keyboard("{ArrowRight}");
		await expect(Number(area.getAttribute("aria-valuenow"))).toBe(before + 1);
		const brightness = Number(
			/亮度 (\d+)%/.exec(area.getAttribute("aria-valuetext") ?? "")?.[1],
		);
		await userEvent.keyboard("{Shift>}{ArrowUp}{/Shift}");
		await expect(area.getAttribute("aria-valuetext")).toContain(
			`亮度 ${brightness + 10}%`,
		);
		const hue = panel.getByRole("slider", { name: "色相" });
		// Range keys require native browser defaults; Testing Library tries to
		// setSelectionRange on input[type=range], which the browser disallows.
		const hasNativeBrowser = "__vitest_browser__" in globalThis;
		if (globals["storybook/test-provided"])
			await expect(hasNativeBrowser).toBe(true);
		if (hasNativeBrowser) {
			const { userEvent: nativeUserEvent } = await import("vitest/browser");
			hue.focus();
			await nativeUserEvent.keyboard("{End}");
			await expect(hue).toHaveValue("359");
		}
		await expect(args.onOpen).toHaveBeenCalledTimes(1);
		await expect(args.onApply).not.toHaveBeenCalled();
		await userEvent.click(panel.getByRole("button", { name: "套用顏色" }));
		await expect(args.onApply).toHaveBeenCalledTimes(1);
		await expect(args.onApply).toHaveBeenCalledWith(
			expect.stringMatching(/^#[\da-f]{6}$/),
		);
	},
};

export const HexAndRgbValidation: Story = {
	name: "HEX／RGB · 驗證與套用",
	play: async ({ canvas, canvasElement, args }) => {
		await userEvent.click(canvas.getByRole("button", { name: "文字顏色" }));
		const panel = within(await waitForPalette(canvasElement));
		const input = panel.getByRole("textbox", { name: "HEX 色碼" });
		await userEvent.clear(input);
		await userEvent.type(input, "#12345678");
		await expect(input).toHaveAttribute("aria-invalid", "true");
		await expect(
			panel.getByRole("button", { name: "套用顏色" }),
		).toBeDisabled();
		await expect(args.onApply).not.toHaveBeenCalled();
		await userEvent.clear(input);
		await userEvent.type(input, "#6194d0");
		await userEvent.selectOptions(
			panel.getByRole("combobox", { name: "色碼格式" }),
			"rgb",
		);
		const red = panel.getByRole("textbox", { name: "紅色 R" });
		const green = panel.getByRole("textbox", { name: "綠色 G" });
		const blue = panel.getByRole("textbox", { name: "藍色 B" });
		await expect(red).toHaveValue("97");
		await expect(green).toHaveValue("148");
		await expect(blue).toHaveValue("208");
		await userEvent.clear(red);
		await userEvent.type(red, "1000");
		await expect(red).toHaveAttribute("aria-invalid", "true");
		await expect(
			panel.getByRole("button", { name: "套用顏色" }),
		).toBeDisabled();
		for (const [field, next] of [
			[red, "24"],
			[green, "128"],
			[blue, "255"],
		] as const) {
			await userEvent.clear(field);
			await userEvent.type(field, next);
		}
		await userEvent.click(panel.getByRole("button", { name: "套用顏色" }));
		await expect(args.onApply).toHaveBeenCalledTimes(1);
		await expect(args.onApply).toHaveBeenCalledWith("#1880ff");
	},
};

export const CancelAndEscape: Story = {
	name: "取消與 Escape · 保留原色",
	play: async ({ canvas, canvasElement, args }) => {
		const body = within(canvasElement.ownerDocument.body);
		const trigger = canvas.getByRole("button", { name: "文字顏色" });
		trigger.focus();
		await userEvent.keyboard("{Enter}");
		let panel = within(await waitForPalette(canvasElement));
		panel.getByRole("button", { name: "關閉色盤" }).focus();
		await userEvent.tab({ shift: true });
		await expect(panel.getByRole("button", { name: "套用顏色" })).toHaveFocus();
		await userEvent.tab();
		await expect(panel.getByRole("button", { name: "關閉色盤" })).toHaveFocus();
		await userEvent.click(
			panel.getByRole("button", { name: "深藍 #1e5fab" }),
		);
		await userEvent.keyboard("{Escape}");
		await expect(
			body.queryByRole("dialog", { name: "文字顏色" }),
		).not.toBeInTheDocument();
		await expect(trigger).toHaveFocus();
		await expect(args.onApply).not.toHaveBeenCalled();
		await userEvent.keyboard(" ");
		panel = within(await waitForPalette(canvasElement));
		await expect(panel.getByRole("textbox", { name: "HEX 色碼" })).toHaveValue(
			"#b8441f",
		);
		await userEvent.click(panel.getByRole("button", { name: "柿橙 #ea6021" }));
		await userEvent.click(panel.getByRole("button", { name: "取消" }));
		await expect(trigger).toHaveFocus();
		await expect(args.onOpen).toHaveBeenCalledTimes(2);
		await expect(args.onApply).not.toHaveBeenCalled();
	},
};

export const NarrowDarkAndDisabled: Story = {
	name: "窄版深色 · 品牌色與停用",
	globals: { theme: "dark" },
	render: (args) => (
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
			<DescriptionColorPicker {...args} />
			<DescriptionColorPicker {...args} disabled />
		</div>
	),
	play: async ({ canvas, canvasElement, args }) => {
		const [enabled, disabled] = canvas.getAllByRole("button", {
			name: "文字顏色",
		});
		await expect(disabled).toBeDisabled();
		await userEvent.click(disabled);
		await expect(args.onOpen).not.toHaveBeenCalled();
		await userEvent.click(enabled);
		const element = await waitForPalette(canvasElement);
		const panel = within(element);
		await userEvent.click(
			panel.getByRole("button", { name: "深藍 #1e5fab" }),
		);
		await expect(
			panel.getByRole("button", { name: "深藍 #1e5fab" }),
		).toHaveAttribute("aria-pressed", "true");
		await expect(args.onApply).not.toHaveBeenCalled();
		const bounds = element.getBoundingClientRect();
		await expect(bounds.left).toBeGreaterThanOrEqual(0);
		await expect(bounds.right).toBeLessThanOrEqual(
			canvasElement.ownerDocument.documentElement.clientWidth,
		);
	},
};
