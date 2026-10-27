import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";
import { expect, fn, userEvent, waitFor, within } from "storybook/test";
import { DescriptionEditorCanvas } from "@/features/seller/components/DescriptionEditor";
import type { MediaUploadRepo } from "@/lib/media/upload";
import type { ProductDescription } from "@/lib/products/seller-repo";
import "@/styles/features/seller-center.css";

const document: ProductDescription = {
	version: "1.0",
	blocks: [
		{
			id: "color-first",
			type: "text",
			runs: [{ text: "選取這段文字調整顏色" }],
		},
		{
			id: "color-second",
			type: "text",
			runs: [{ text: "其餘內容維持原來的樣式" }],
		},
	],
};

const meta = {
	title: "Web/Seller Center/DescriptionEditorColor",
	component: DescriptionEditorCanvas,
	parameters: {
		layout: "padded",
		docs: {
			description: {
				component:
					"正式商品描述編輯器的色盤整合：保留文字選取、取消不變更、套用後可以復原；全螢幕與窄版共用同一份文件。範例只更新本地 state，不連接商品交易。色碼以既有 runs.color 的 #rrggbb 格式儲存。",
			},
		},
	},
	args: {
		value: document,
		onChange: fn<(next: ProductDescription) => void>(),
		uploadImageFile: fn<MediaUploadRepo["uploadImageFile"]>(async () => {
			throw new Error("此範例不會上傳圖片");
		}),
	},
	render: function Controlled(args) {
		const [value, setValue] = useState(args.value);
		return (
			<div style={{ width: "100%", maxWidth: 1040, marginInline: "auto" }}>
				<DescriptionEditorCanvas
					{...args}
					value={value}
					onChange={(next) => {
						setValue(next);
						args.onChange(next);
					}}
				/>
			</div>
		);
	},
} satisfies Meta<typeof DescriptionEditorCanvas>;
export default meta;
type Story = StoryObj<typeof meta>;

async function setup(canvasElement: HTMLElement) {
	const dialog = within(canvasElement).getByRole("dialog", {
		name: "商品描述編輯器",
	});
	await waitFor(
		() => {
			expect(dialog).toHaveAttribute("data-ready", "true");
			expect(dialog).toHaveAttribute("aria-busy", "false");
			expect(dialog).toBeVisible();
			expect(within(dialog).getByText("選取這段文字調整顏色")).toBeVisible();
		},
		{ timeout: 15_000 },
	);
	return dialog;
}

function selectText(element: HTMLElement) {
	element.focus();
	const doc = element.ownerDocument;
	const range = doc.createRange();
	range.selectNodeContents(element);
	doc.getSelection()?.removeAllRanges();
	doc.getSelection()?.addRange(range);
	doc.dispatchEvent(new Event("selectionchange"));
}

async function waitForPalette(dialog: HTMLElement) {
	// The portal mounts before positioning and focus are ready. Expanded mode
	// also animates its parent dialog; existence alone does not mean visible.
	return waitFor(() => {
		const panel = within(dialog).getByRole("dialog", { name: "文字顏色" });
		expect(panel).toBeVisible();
		expect(within(panel).getByRole("slider", { name: "色相" })).toBeVisible();
		expect(
			within(panel).getByRole("slider", { name: "飽和度與亮度" }),
		).toHaveFocus();
		return panel;
	});
}

async function openPalette(dialog: HTMLElement) {
	const ui = within(dialog);
	if (
		ui
			.getByRole("button", { name: /更多格式/ })
			.getAttribute("aria-expanded") !== "true"
	) {
		await userEvent.click(ui.getByRole("button", { name: /更多格式/ }));
	}
	await userEvent.click(ui.getByRole("button", { name: "文字顏色" }));
	return waitForPalette(dialog);
}

export const SelectedTextAndUndo: Story = {
	name: "選取文字 · 自訂色碼與復原",
	play: async ({ canvasElement, args }) => {
		const dialog = await setup(canvasElement);
		const ui = within(dialog);
		selectText(ui.getByText("選取這段文字調整顏色"));
		const picker = within(await openPalette(dialog));
		const input = picker.getByRole("textbox", { name: "HEX 色碼" });
		await userEvent.clear(input);
		await userEvent.type(input, "#563a72");
		await userEvent.click(picker.getByRole("button", { name: "套用顏色" }));
		await waitFor(() => {
			const next = args.onChange.mock.lastCall?.[0];
			expect(next?.blocks[0].runs).toEqual([
				{ text: "選取這段文字調整顏色", color: "#563a72" },
			]);
			expect(next?.blocks[1].runs).toEqual([
				{ text: "其餘內容維持原來的樣式" },
			]);
		});
		await waitFor(() =>
			expect(ui.getByText("選取這段文字調整顏色")).toHaveStyle({
				color: "rgb(86, 58, 114)",
			}),
		);
		await userEvent.click(ui.getByRole("button", { name: /^復原（/ }));
		await waitFor(() =>
			expect(
				args.onChange.mock.lastCall?.[0].blocks[0].runs?.[0].color,
			).toBeUndefined(),
		);
		await userEvent.click(ui.getByRole("button", { name: /^重做（/ }));
		await waitFor(() =>
			expect(args.onChange.mock.lastCall?.[0].blocks[0].runs?.[0].color).toBe(
				"#563a72",
			),
		);
		await expect(args.uploadImageFile).not.toHaveBeenCalled();
	},
};

export const CancelAndContinueEditing: Story = {
	name: "取消選色 · 保留文件與選取",
	play: async ({ canvasElement, args }) => {
		const dialog = await setup(canvasElement);
		const ui = within(dialog);
		selectText(ui.getByText("選取這段文字調整顏色"));
		let picker = within(await openPalette(dialog));
		const input = picker.getByRole("textbox", { name: "HEX 色碼" });
		await userEvent.clear(input);
		await userEvent.type(input, "#563a72");
		await userEvent.click(picker.getByRole("button", { name: "取消" }));
		await expect(
			ui.queryByRole("dialog", { name: "文字顏色" }),
		).not.toBeInTheDocument();
		await expect(ui.getByRole("button", { name: "文字顏色" })).toHaveFocus();
		await expect(
			args.onChange.mock.lastCall?.[0].blocks[0].runs?.[0].color,
		).toBeUndefined();
		picker = within(await openPalette(dialog));
		await expect(
			picker.getByRole("textbox", { name: "HEX 色碼" }),
		).not.toHaveValue("#563a72");
		await userEvent.keyboard("{Escape}");
		await userEvent.click(ui.getByRole("button", { name: /^粗體/ }));
		await waitFor(() => {
			expect(args.onChange.mock.lastCall?.[0].blocks[0].runs).toEqual([
				{ text: "選取這段文字調整顏色", bold: true },
			]);
			expect(dialog.ownerDocument.getSelection()?.toString()).toBe(
				"選取這段文字調整顏色",
			);
			expect(
				dialog.ownerDocument.defaultView?.getComputedStyle(
					ui.getByText("選取這段文字調整顏色"),
				).fontWeight,
			).toMatch(/^(bold|[6-9]00)$/);
		});
	},
};

export const ExpandedPalette: Story = {
	name: "放大編輯 · 色盤鍵盤與關閉",
	play: async ({ canvasElement, args }) => {
		const dialog = await setup(canvasElement);
		const ui = within(dialog);
		await userEvent.click(ui.getByRole("button", { name: "放大編輯" }));
		await waitFor(() => {
			const collapse = ui.getByRole("button", { name: "縮小編輯區" });
			expect(collapse).toHaveAttribute("aria-expanded", "true");
			expect(collapse).toBeVisible();
		});
		selectText(ui.getByText("選取這段文字調整顏色"));
		let picker = within(await openPalette(dialog));
		const input = picker.getByRole("textbox", { name: "HEX 色碼" });
		await userEvent.clear(input);
		await userEvent.type(input, "#b8441f");
		await userEvent.click(picker.getByRole("button", { name: "套用顏色" }));
		await waitFor(() =>
			expect(args.onChange.mock.lastCall?.[0].blocks[0].runs?.[0].color).toBe(
				"#b8441f",
			),
		);
		selectText(ui.getByText("選取這段文字調整顏色"));
		picker = within(await openPalette(dialog));
		await expect(picker.getByRole("textbox", { name: "HEX 色碼" })).toHaveValue(
			"#b8441f",
		);
		await userEvent.keyboard("{Escape}");
		await expect(
			ui.queryByRole("dialog", { name: "文字顏色" }),
		).not.toBeInTheDocument();
		await expect(ui.getByRole("button", { name: "縮小編輯區" })).toBeVisible();
		await userEvent.click(ui.getByRole("button", { name: "縮小編輯區" }));
		await waitFor(() => {
			const expand = ui.getByRole("button", { name: "放大編輯" });
			expect(expand).toHaveAttribute("aria-expanded", "false");
			expect(expand).toBeVisible();
		});
	},
};

export const PartialSelection: Story = {
	name: "局部選取 · 僅變更選取的文字",
	play: async ({ canvasElement, args }) => {
		const dialog = await setup(canvasElement);
		const ui = within(dialog);
		const paragraph = ui.getByText("選取這段文字調整顏色");
		paragraph.focus();
		const text = paragraph.firstChild;
		if (!text) throw new Error("缺少待選取文字");
		const doc = paragraph.ownerDocument;
		const range = doc.createRange();
		range.setStart(text, 2);
		range.setEnd(text, 6);
		doc.getSelection()?.removeAllRanges();
		doc.getSelection()?.addRange(range);
		doc.dispatchEvent(new Event("selectionchange"));
		const picker = within(await openPalette(dialog));
		const input = picker.getByRole("textbox", { name: "HEX 色碼" });
		await userEvent.clear(input);
		await userEvent.type(input, "#682e0e");
		await userEvent.click(picker.getByRole("button", { name: "套用顏色" }));
		await waitFor(() =>
			expect(args.onChange.mock.lastCall?.[0].blocks[0].runs).toEqual([
				{ text: "選取" },
				{ text: "這段文字", color: "#682e0e" },
				{ text: "調整顏色" },
			]),
		);
		await expect(ui.getByText("這段文字")).toHaveStyle({
			color: "rgb(104, 46, 14)",
		});
	},
};

export const ColorThenInlineFormatting: Story = {
	name: "文字顏色之後 · 粗斜底線與逐步復原",
	parameters: {
		docs: {
			description: {
				story:
					"原生瀏覽器回歸：同一次編輯中先變色，再加粗體、斜體、底線，逐步復原時保留其他格式。自動操作使用 MCP 的 Vitest Browser；一般預覽保留可手動操作的本地範例。",
			},
		},
	},
	play: async ({ canvasElement, args, globals }) => {
		const dialog = await setup(canvasElement);
		const hasNativeBrowser = "__vitest_browser__" in globalThis;
		if (globals["storybook/test-provided"]) {
			await expect(hasNativeBrowser).toBe(true);
		}
		if (!hasNativeBrowser) return;
		const { userEvent: native } = await import("vitest/browser");
		await expect(native).not.toBeNull();
		const ui = within(dialog);
		const view = dialog.ownerDocument.defaultView;
		if (!view) throw new Error("缺少編輯器瀏覽器視窗");
		const errors: string[] = [];
		const onError = (event: ErrorEvent) => errors.push(event.message);
		const onRejection = (event: PromiseRejectionEvent) =>
			errors.push(String(event.reason));
		view.addEventListener("error", onError);
		view.addEventListener("unhandledrejection", onRejection);
		try {
			await native.unhover(dialog);
			const paragraph = ui.getByText("選取這段文字調整顏色");
			await native.click(paragraph);
			selectText(paragraph);
			await native.click(ui.getByRole("button", { name: /更多格式/ }));
			await native.click(ui.getByRole("button", { name: "文字顏色" }));
			const picker = within(await waitForPalette(dialog));
			await native.fill(
				picker.getByRole("textbox", { name: "HEX 色碼" }),
				"#682e0e",
			);
			await native.click(picker.getByRole("button", { name: "套用顏色" }));
			const text = "選取這段文字調整顏色";
			const color = "#682e0e";
			const states = [
				{ text },
				{ text, color },
				{ text, color, bold: true },
				{ text, color, bold: true, italic: true },
				{ text, color, bold: true, italic: true, underline: true },
			];
			const expectState = async (index: number, phase: string) => {
				try {
					await waitFor(() => {
						// History publishes its snapshot before the final busy-state
						// update. Check the current rendered text after that update,
						// rather than retaining a node from an intermediate render.
						expect(dialog).toHaveAttribute("aria-busy", "false");
						const next = args.onChange.mock.lastCall?.[0];
						expect(next?.blocks[0].runs).toEqual([states[index]]);
						expect(next?.blocks[1].runs).toEqual([
							{ text: "其餘內容維持原來的樣式" },
						]);
						expect(within(dialog).getByText(text)).toBeVisible();
					});
				} catch (cause) {
					throw new Error(
						`Formatting state failed: ${JSON.stringify(
							{
								phase,
								expected: [states[index]],
								actual: args.onChange.mock.lastCall?.[0].blocks[0].runs,
							},
							null,
							2,
						)}`,
						{ cause },
					);
				}
			};
			await expectState(1, "applyColor");
			const actions = [
				{ name: /^粗體/, phase: "Bold" },
				{ name: /^斜體/, phase: "Italic" },
				{ name: /^底線/, phase: "Underline" },
			];
			for (const [index, { name, phase }] of actions.entries()) {
				// No reselection: applying one format must keep the selection ready
				// for the next toolbar command in the same editing session.
				await native.click(ui.getByRole("button", { name }));
				await expectState(index + 2, `apply${phase}`);
			}
			for (let index = 3; index >= 0; index -= 1) {
				await native.click(ui.getByRole("button", { name: /^復原（/ }));
				await expectState(
					index,
					`undo${["Color", "Bold", "Italic", "Underline"][index]}`,
				);
			}
			await expect(args.uploadImageFile).not.toHaveBeenCalled();
			await expect(errors).toEqual([]);
		} finally {
			view.removeEventListener("error", onError);
			view.removeEventListener("unhandledrejection", onRejection);
		}
	},
};
