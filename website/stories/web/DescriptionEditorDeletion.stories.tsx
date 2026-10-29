import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";
import { expect, fn, waitFor, within } from "storybook/test";
import { DescriptionEditorCanvas } from "@/features/seller/components/DescriptionEditor";
import type { MediaUploadRepo } from "@/lib/media/upload";
import type { ProductDescription } from "@/lib/products/seller-repo";
import "@/styles/features/seller-center.css";
import { startDescriptionImageFixtures } from "../../.storybook/description-image-fixtures";

type NativeUserEvent = typeof import("vitest/browser").userEvent;

function paragraphs(...texts: string[]): ProductDescription {
	return {
		version: "1.0",
		blocks: texts.map((text, index) => ({
			id: `deletion-paragraph-${index}`,
			type: "text",
			runs: [{ text }],
		})),
	};
}

function documentText(value: ProductDescription): string[] {
	return value.blocks
		.filter((block) => block.type === "text")
		.map(
			(block) =>
				block.runs?.map((run) => run.text).join("") ?? block.content ?? "",
		)
		.filter(Boolean);
}

const meta = {
	title: "Web/Seller Center/DescriptionEditor Deletion",
	component: DescriptionEditorCanvas,
	args: {
		value: paragraphs("第一段保留", "第二段保留"),
		onChange: fn<(value: ProductDescription) => void>(),
		uploadImageFile: fn<MediaUploadRepo["uploadImageFile"]>(async () => {
			throw new Error("此回歸故事不建立媒體上傳交易");
		}),
	},
	parameters: {
		docs: {
			description: {
				component:
					"刪除與復原的原生瀏覽器回歸。MCP test-run 使用 Vitest Browser 的 CDP 鍵盤，保留真正的 beforeinput、段落合併與選取刪除行為；一般 Storybook 預覽只載入固定文件，方便手動重現。測試同時比對可見文字與 onChange 文件，並檢查瀏覽器錯誤。所有文件為本地 state，上傳方法為 stub。\n\n實際 import：`@/features/seller/components/DescriptionEditor`。",
			},
		},
	},
	render: function Controlled(args) {
		const [value, setValue] = useState(args.value);
		return (
			<div style={{ maxWidth: 1040, marginInline: "auto" }}>
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

async function nativeDeletionTest(
	canvasElement: HTMLElement,
	globals: Record<string, unknown>,
	test: (dialog: HTMLElement, user: NativeUserEvent) => Promise<void>,
) {
	const dialog = within(canvasElement).getByRole("dialog", {
		name: "商品描述編輯器",
	});
	await waitFor(() => expect(dialog).toHaveAttribute("data-ready", "true"), {
		timeout: 15_000,
	});
	// Ordinary previews remain interactive fixtures. Never replace native editing
	// with synthetic defaults: those cannot reproduce browser block deletion.
	const hasNativeBrowser = "__vitest_browser__" in globalThis;
	// The addon sets this key for every MCP test run (compose-initial-globals).
	// Fail loudly if that runner ever stops supplying a native browser driver.
	if (globals["storybook/test-provided"]) {
		await expect(hasNativeBrowser).toBe(true);
	}
	if (!hasNativeBrowser) return;
	const { userEvent } = await import("vitest/browser");
	await expect(userEvent).not.toBeNull();
	const errors: string[] = [];
	const view = dialog.ownerDocument.defaultView;
	if (!view) throw new Error("缺少編輯器瀏覽器視窗");
	const onError = (event: ErrorEvent) => errors.push(event.message);
	const onRejection = (event: PromiseRejectionEvent) =>
		errors.push(String(event.reason));
	view.addEventListener("error", onError);
	view.addEventListener("unhandledrejection", onRejection);
	try {
		await userEvent.unhover(dialog);
		await test(dialog, userEvent);
		await expect(errors).toEqual([]);
	} finally {
		view.removeEventListener("error", onError);
		view.removeEventListener("unhandledrejection", onRejection);
	}
}

function editableText(dialog: HTMLElement, text: string): HTMLElement {
	const editable = Array.from(
		dialog.querySelectorAll<HTMLElement>(
			'.ce-paragraph[contenteditable="true"], .ce-header[contenteditable="true"], .cdx-list__item',
		),
	).find((candidate) => candidate.textContent === text);
	if (!editable) throw new Error(`找不到可編輯文字：${text}`);
	return editable;
}

function setCaret(editable: HTMLElement, at: "start" | "end") {
	editable.focus();
	const range = editable.ownerDocument.createRange();
	range.selectNodeContents(editable);
	range.collapse(at === "start");
	const selection = editable.ownerDocument.getSelection();
	selection?.removeAllRanges();
	selection?.addRange(range);
}

function selectTextRange(
	start: HTMLElement,
	startOffset: number,
	end: HTMLElement,
	endOffset: number,
) {
	start.focus();
	const point = (element: HTMLElement, offset: number) => {
		const walker = element.ownerDocument.createTreeWalker(
			element,
			NodeFilter.SHOW_TEXT,
		);
		let remaining = offset;
		for (let node = walker.nextNode(); node; node = walker.nextNode()) {
			const length = node.textContent?.length ?? 0;
			if (remaining <= length) return { node, offset: remaining };
			remaining -= length;
		}
		throw new Error("選取範圍超過段落文字");
	};
	const from = point(start, startOffset);
	const to = point(end, endOffset);
	const range = start.ownerDocument.createRange();
	range.setStart(from.node, from.offset);
	range.setEnd(to.node, to.offset);
	const selection = start.ownerDocument.getSelection();
	selection?.removeAllRanges();
	selection?.addRange(range);
}

async function deleteFromBlockMenu(
	dialog: HTMLElement,
	target: HTMLElement,
	user: NativeUserEvent,
) {
	const block = target.closest<HTMLElement>(".ce-block");
	if (!block) throw new Error("找不到要刪除的內容區塊");
	await user.hover(block);
	// Editor.js supplies these controls without a stable accessible name.
	const settings = dialog.querySelector<HTMLElement>(
		".ce-toolbar__settings-btn",
	);
	if (!settings) throw new Error("找不到區塊調整按鈕");
	await user.click(settings);
	const item = await waitFor(() => {
		const candidate = dialog.querySelector<HTMLElement>(
			'.ce-popover-item[data-item-name="delete"]',
		);
		expect(candidate).toBeVisible();
		if (!candidate) throw new Error("找不到刪除區塊選項");
		return candidate;
	});
	await user.click(item);
}

async function undo(dialog: HTMLElement, user: NativeUserEvent) {
	await user.click(within(dialog).getByRole("button", { name: /^復原（/ }));
}

async function redo(dialog: HTMLElement, user: NativeUserEvent) {
	await user.click(within(dialog).getByRole("button", { name: /^重做（/ }));
}

export const BackspaceAtParagraphStart: Story = {
	name: "段落開頭 · Backspace 合併與復原",
	args: {
		value: {
			version: "1.0",
			blocks: [
				{
					id: "formatted-first",
					type: "text",
					runs: [{ text: "第一段保留", bold: true, color: "#682e0e" }],
				},
				{ id: "plain-second", type: "text", runs: [{ text: "第二段保留" }] },
			],
		},
	},
	play: async ({ canvasElement, args, globals }) =>
		nativeDeletionTest(canvasElement, globals, async (dialog, user) => {
			const second = editableText(dialog, "第二段保留");
			await user.click(second);
			setCaret(second, "start");
			await user.keyboard("{Backspace}");
			await waitFor(() => {
				expect(
					documentText(args.onChange.mock.lastCall?.[0] ?? args.value),
				).toEqual(["第一段保留第二段保留"]);
				expect(args.onChange.mock.lastCall?.[0].blocks[0].runs).toEqual(
					expect.arrayContaining([
						{ text: "第一段保留", bold: true, color: "#682e0e" },
					]),
				);
				expect(editableText(dialog, "第一段保留第二段保留")).toBeVisible();
			});
			await undo(dialog, user);
			await waitFor(() => {
				expect(
					documentText(args.onChange.mock.lastCall?.[0] ?? args.value),
				).toEqual(["第一段保留", "第二段保留"]);
				expect(editableText(dialog, "第二段保留")).toBeVisible();
			});
		}),
};

export const DeleteAtParagraphEnd: Story = {
	name: "段落結尾 · Delete 合併與復原",
	play: async ({ canvasElement, args, globals }) =>
		nativeDeletionTest(canvasElement, globals, async (dialog, user) => {
			const first = editableText(dialog, "第一段保留");
			await user.click(first);
			setCaret(first, "end");
			await user.keyboard("{Delete}");
			await waitFor(() => {
				expect(
					documentText(args.onChange.mock.lastCall?.[0] ?? args.value),
				).toEqual(["第一段保留第二段保留"]);
				expect(editableText(dialog, "第一段保留第二段保留")).toBeVisible();
			});
			await undo(dialog, user);
			await waitFor(() => {
				expect(
					documentText(args.onChange.mock.lastCall?.[0] ?? args.value),
				).toEqual(["第一段保留", "第二段保留"]);
				expect(editableText(dialog, "第一段保留")).toBeVisible();
			});
		}),
};

export const DeleteAcrossParagraphs: Story = {
	name: "跨段選取 · 刪除中間內容",
	args: {
		value: {
			version: "1.0",
			blocks: [
				{
					id: "range-first",
					type: "text",
					runs: [
						{ text: "前段保留", bold: true, color: "#682e0e" },
						{ text: "刪掉" },
					],
				},
				{
					id: "range-middle",
					type: "text",
					runs: [{ text: "整段刪掉", underline: true }],
				},
				{
					id: "range-last",
					type: "text",
					runs: [{ text: "刪掉" }, { text: "後段保留", italic: true }],
				},
			],
		},
	},
	play: async ({ canvasElement, args, globals }) =>
		nativeDeletionTest(canvasElement, globals, async (dialog, user) => {
			const first = editableText(dialog, "前段保留刪掉");
			await user.click(first);
			selectTextRange(first, 4, editableText(dialog, "刪掉後段保留"), 2);
			await user.keyboard("{Backspace}");
			await waitFor(() => {
				expect(
					documentText(args.onChange.mock.lastCall?.[0] ?? args.value),
				).toEqual(["前段保留後段保留"]);
				expect(args.onChange.mock.lastCall?.[0].blocks[0].runs).toEqual(
					expect.arrayContaining([
						{ text: "前段保留", bold: true, color: "#682e0e" },
						{ text: "後段保留", italic: true },
					]),
				);
				expect(editableText(dialog, "前段保留後段保留")).toBeVisible();
			});
			await undo(dialog, user);
			await waitFor(() => {
				expect(
					documentText(args.onChange.mock.lastCall?.[0] ?? args.value),
				).toEqual(["前段保留刪掉", "整段刪掉", "刪掉後段保留"]);
				expect(editableText(dialog, "整段刪掉")).toBeVisible();
			});
		}),
};

const paragraphToListValue: ProductDescription = {
	version: "1.0",
	blocks: [
		{
			id: "paragraph-range-start",
			type: "text",
			runs: [
				{ text: "段落前端", bold: true, color: "#682e0e" },
				{ text: "刪除起點" },
			],
			style: { textAlign: "center" },
		},
		{
			id: "list-range-end",
			type: "list",
			listJson: JSON.stringify({
				ordered: true,
				textAlign: "right",
				items: [
					{ runs: [{ text: "刪除終點" }, { text: "清單尾端", italic: true }] },
					{ runs: [{ text: "保留的後續項目", underline: true }] },
				],
			}),
		},
	],
};

export const DeleteFromParagraphIntoList: Story = {
	name: "跨段刪除 · 內文到清單保留後續項目",
	args: { value: paragraphToListValue },
	play: async ({ canvasElement, args, globals }) =>
		nativeDeletionTest(canvasElement, globals, async (dialog, user) => {
			const start = editableText(dialog, "段落前端刪除起點");
			const end = editableText(dialog, "刪除終點清單尾端");
			await user.click(start);
			selectTextRange(start, 4, end, 4);
			await user.keyboard("{Delete}");
			await waitFor(() => {
				const value = args.onChange.mock.lastCall?.[0] ?? args.value;
				expect(documentText(value)).toEqual(["段落前端清單尾端"]);
				expect(value.blocks[0].runs).toEqual([
					{ text: "段落前端", bold: true, color: "#682e0e" },
					{ text: "清單尾端", italic: true },
				]);
				expect(value.blocks[0].style?.textAlign).toBe("center");
				expect(
					JSON.parse(
						value.blocks.find((block) => block.type === "list")?.listJson ??
							"null",
					),
				).toEqual({
					ordered: true,
					textAlign: "right",
					items: [{ runs: [{ text: "保留的後續項目", underline: true }] }],
				});
				expect(editableText(dialog, "段落前端清單尾端")).toBeVisible();
				expect(editableText(dialog, "保留的後續項目")).toBeVisible();
			});
			await undo(dialog, user);
			await waitFor(() => {
				const value = args.onChange.mock.lastCall?.[0] ?? args.value;
				expect(documentText(value)).toEqual(["段落前端刪除起點"]);
				expect(
					JSON.parse(
						value.blocks.find((block) => block.type === "list")?.listJson ??
							"null",
					),
				).toEqual(
					JSON.parse(paragraphToListValue.blocks[1].listJson ?? "null"),
				);
				expect(editableText(dialog, "刪除終點清單尾端")).toBeVisible();
			});
		}),
};

const listToHeadingValue: ProductDescription = {
	version: "1.0",
	blocks: [
		{
			id: "list-range-start",
			type: "list",
			listJson: JSON.stringify({
				ordered: false,
				textAlign: "center",
				items: [
					{ runs: [{ text: "前方項目保留", underline: true }] },
					{ runs: [{ text: "清單前端", bold: true }, { text: "刪除起點" }] },
				],
			}),
		},
		{
			id: "heading-range-end",
			type: "text",
			runs: [
				{ text: "刪除終點" },
				{ text: "標題尾端", color: "#682e0e", italic: true },
			],
			style: { fontSize: "26px", fontWeight: 700, textAlign: "center" },
		},
	],
};

export const DeleteFromListIntoHeading: Story = {
	name: "跨段刪除 · 清單到標題保留前方項目",
	args: { value: listToHeadingValue },
	play: async ({ canvasElement, args, globals }) =>
		nativeDeletionTest(canvasElement, globals, async (dialog, user) => {
			const start = editableText(dialog, "清單前端刪除起點");
			const end = editableText(dialog, "刪除終點標題尾端");
			await user.click(start);
			selectTextRange(start, 4, end, 4);
			await user.keyboard("{Backspace}");
			await waitFor(() => {
				const value = args.onChange.mock.lastCall?.[0] ?? args.value;
				expect(value.blocks.map((block) => block.type)).toEqual(["list"]);
				expect(JSON.parse(value.blocks[0].listJson ?? "null")).toEqual({
					ordered: false,
					textAlign: "center",
					items: [
						{ runs: [{ text: "前方項目保留", underline: true }] },
						{
							runs: [
								{ text: "清單前端", bold: true },
								{ text: "標題尾端", color: "#682e0e", italic: true },
							],
						},
					],
				});
				expect(editableText(dialog, "前方項目保留")).toBeVisible();
				expect(editableText(dialog, "清單前端標題尾端")).toBeVisible();
			});
			await undo(dialog, user);
			await waitFor(() => {
				const value = args.onChange.mock.lastCall?.[0] ?? args.value;
				expect(documentText(value)).toEqual(["刪除終點標題尾端"]);
				expect(
					value.blocks.find((block) => block.type === "text")?.style,
				).toEqual({ fontSize: "26px", fontWeight: 700, textAlign: "center" });
				expect(
					JSON.parse(
						value.blocks.find((block) => block.type === "list")?.listJson ??
							"null",
					),
				).toEqual(JSON.parse(listToHeadingValue.blocks[0].listJson ?? "null"));
				expect(editableText(dialog, "刪除終點標題尾端")).toBeVisible();
			});
		}),
};

export const DeleteHoveredBlock: Story = {
	name: "區塊選單 · 刪除滑鼠指向的段落",
	args: {
		value: {
			version: "1.0",
			blocks: [
				{
					id: "deletion-paragraph-0",
					type: "text",
					runs: [{ text: "第一段保留" }],
					style: { textAlign: "center" },
				},
				{
					id: "deletion-paragraph-1",
					type: "text",
					runs: [{ text: "第二段保留" }],
				},
			],
		},
	},
	play: async ({ canvasElement, args, globals }) =>
		nativeDeletionTest(canvasElement, globals, async (dialog, user) => {
			await expect(editableText(dialog, "第一段保留")).toHaveStyle({
				textAlign: "center",
			});
			await user.click(editableText(dialog, "第二段保留"));
			await deleteFromBlockMenu(
				dialog,
				editableText(dialog, "第一段保留"),
				user,
			);
			await waitFor(() => {
				expect(
					documentText(args.onChange.mock.lastCall?.[0] ?? args.value),
				).toEqual(["第二段保留"]);
				expect(
					within(dialog).queryByText("第一段保留"),
				).not.toBeInTheDocument();
				expect(editableText(dialog, "第二段保留")).toBeVisible();
			});
			await undo(dialog, user);
			await waitFor(() => {
				expect(
					documentText(args.onChange.mock.lastCall?.[0] ?? args.value),
				).toEqual(["第一段保留", "第二段保留"]);
				expect(editableText(dialog, "第一段保留")).toBeVisible();
				expect(editableText(dialog, "第一段保留")).toHaveStyle({
					textAlign: "center",
				});
				expect(
					(args.onChange.mock.lastCall?.[0] ?? args.value).blocks.find(
						(block) => block.id === "deletion-paragraph-0",
					)?.style?.textAlign,
				).toBe("center");
			});
		}),
};

export const DeleteLastBlockAndRepeatHistory: Story = {
	name: "最後一段 · 刪除與反覆復原重做",
	args: { value: paragraphs("最後一段也可以找回") },
	play: async ({ canvasElement, args, globals }) =>
		nativeDeletionTest(canvasElement, globals, async (dialog, user) => {
			const text = "最後一段也可以找回";
			await user.click(editableText(dialog, text));
			await deleteFromBlockMenu(dialog, editableText(dialog, text), user);
			await waitFor(() => {
				expect(args.onChange.mock.lastCall?.[0].blocks).toEqual([]);
				expect(within(dialog).queryByText(text)).not.toBeInTheDocument();
			});
			for (let cycle = 0; cycle < 3; cycle += 1) {
				await undo(dialog, user);
				await waitFor(() => {
					expect(
						documentText(args.onChange.mock.lastCall?.[0] ?? args.value),
					).toEqual([text]);
					expect(editableText(dialog, text)).toBeVisible();
				});
				await redo(dialog, user);
				await waitFor(() => {
					expect(args.onChange.mock.lastCall?.[0].blocks).toEqual([]);
					expect(within(dialog).queryByText(text)).not.toBeInTheDocument();
				});
			}
		}),
};

export const ExitEmptyListItem: Story = {
	name: "空白清單項目 · Enter 回到內文",
	args: {
		value: {
			version: "1.0",
			blocks: [
				{
					id: "list-with-empty-tail",
					type: "list",
					listJson: JSON.stringify({
						ordered: false,
						items: [{ runs: [{ text: "保留的清單項目" }] }, { runs: [] }],
					}),
				},
			],
		},
	},
	play: async ({ canvasElement, args, globals }) =>
		nativeDeletionTest(canvasElement, globals, async (dialog, user) => {
			const item = dialog.querySelector<HTMLElement>(".cdx-list li:last-child");
			if (!item) throw new Error("找不到空白清單項目");
			await user.click(item);
			setCaret(item, "end");
			await user.keyboard("{Enter}離開清單後的文字");
			await waitFor(() => {
				const value = args.onChange.mock.lastCall?.[0] ?? args.value;
				expect(
					JSON.parse(
						value.blocks.find((block) => block.type === "list")?.listJson ??
							"null",
					),
				).toEqual({
					ordered: false,
					items: [{ runs: [{ text: "保留的清單項目" }] }],
				});
				expect(documentText(value)).toEqual(["離開清單後的文字"]);
				expect(editableText(dialog, "離開清單後的文字")).toBeVisible();
			});
		}),
};

// 讓段落的存檔一直失敗，直到 Editor.js 自己的 onChange 觸發的那次也失敗為止。
// 每次按鍵會觸發兩次存檔：下一個畫面的即時存檔（連同它的重試在幾十毫秒內），
// 以及 Editor.js 批次約 400ms 後的 onChange。兩段都失敗才是「最後一次存檔失敗」；
// 和上一次失敗隔超過 150ms 的那次就是第二段。Saver 會吞掉錯誤並回傳 undefined。
function failSavesThroughOnChange(editable: HTMLElement) {
	const descriptor = Object.getOwnPropertyDescriptor(
		Element.prototype,
		"innerHTML",
	);
	if (!descriptor?.get || !descriptor.set)
		throw new Error("缺少 innerHTML 存取子");
	const { get, set } = descriptor;
	let lastFailureAt: number | undefined;
	let onChangeFailed = false;
	Object.defineProperty(editable, "innerHTML", {
		configurable: true,
		get() {
			if (!onChangeFailed && new Error().stack?.includes("save")) {
				const now = performance.now();
				if (lastFailureAt !== undefined && now - lastFailureAt > 150)
					onChangeFailed = true;
				lastFailureAt = now;
				throw new Error("模擬存檔失敗");
			}
			return get.call(this);
		},
		set(value: string) {
			set.call(this, value);
		},
	});
	return () => onChangeFailed;
}

export const LastSaveFailureRetries: Story = {
	name: "最後一次存檔失敗 · 補存後 onChange 跟上",
	args: { value: paragraphs("第一段") },
	play: async ({ canvasElement, args, globals }) =>
		nativeDeletionTest(canvasElement, globals, async (dialog, user) => {
			const paragraph = editableText(dialog, "第一段");
			await user.click(paragraph);
			setCaret(paragraph, "end");
			await user.keyboard("補");
			await waitFor(() =>
				expect(
					documentText(args.onChange.mock.lastCall?.[0] ?? args.value),
				).toEqual(["第一段補"]),
			);
			const saveFailed = failSavesThroughOnChange(paragraph);
			await user.keyboard("存");
			// 失敗後不再有輸入；沒有補存的話 onChange 會一直停在「第一段補」。
			await waitFor(
				() => {
					expect(saveFailed()).toBe(true);
					expect(
						documentText(args.onChange.mock.lastCall?.[0] ?? args.value),
					).toEqual(["第一段補存"]);
				},
				{ timeout: 5_000 },
			);
		}),
};

export const BackspaceAtHeadingAfterList: Story = {
	name: "清單與標題交界 · Backspace 保留內容",
	args: {
		value: {
			version: "1.0",
			blocks: [
				{
					id: "list-before-heading",
					type: "list",
					listJson: JSON.stringify({
						ordered: false,
						items: [{ runs: [{ text: "保存方式依包裝標示" }] }],
					}),
				},
				{
					id: "heading-after-list",
					type: "text",
					runs: [{ text: "## 保存與料理", color: "#682e0e" }],
					style: { fontSize: "26px", fontWeight: 700 },
				},
			],
		},
	},
	play: async ({ canvasElement, args, globals }) =>
		nativeDeletionTest(canvasElement, globals, async (dialog, user) => {
			const heading = editableText(dialog, "## 保存與料理");
			await user.click(heading);
			setCaret(heading, "start");
			await user.keyboard("{Backspace}");
			await waitFor(() => {
				const value = args.onChange.mock.lastCall?.[0] ?? args.value;
				expect(documentText(value)).toEqual(["## 保存與料理"]);
				expect(
					JSON.parse(
						value.blocks.find((block) => block.type === "list")?.listJson ??
							"null",
					),
				).toEqual({
					ordered: false,
					items: [{ runs: [{ text: "保存方式依包裝標示" }] }],
				});
				expect(editableText(dialog, "## 保存與料理")).toBeVisible();
				expect(within(dialog).getByText("保存方式依包裝標示")).toBeVisible();
			});
		}),
};

const localImage = `data:image/svg+xml,${encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" width="120" height="80"><rect width="120" height="80" fill="#e1dbce"/><circle cx="60" cy="40" r="20" fill="#334c0f"/></svg>')}`;
const tableJson = JSON.stringify({
	withHeadings: true,
	rows: [
		[{ runs: [{ text: "欄位" }] }, { runs: [{ text: "內容" }] }],
		[{ runs: [{ text: "規格" }] }, { runs: [{ text: "展示文字" }] }],
	],
});

export const DeleteBetweenImageAndTable: Story = {
	name: "圖片與表格之間 · 刪字保留資料",
	beforeEach: startDescriptionImageFixtures,
	args: {
		value: {
			version: "1.0",
			blocks: [
				{
					id: "preserved-image",
					type: "image",
					assetId: "story-image",
					imageKey: "public/storybook/deletion-fixture.svg",
					imageUrl: localImage,
					alt: "本地測試圖片",
				},
				{
					id: "text-between-media",
					type: "text",
					runs: [{ text: "刪除這段文字" }],
				},
				{ id: "preserved-table", type: "table", tableJson },
				{
					id: "text-after-table",
					type: "text",
					runs: [{ text: "表格後的文字保留" }],
				},
			],
		},
	},
	play: async ({ canvasElement, args, globals }) =>
		nativeDeletionTest(canvasElement, globals, async (dialog, user) => {
			const ui = within(dialog);
			const checkPhase = async (phase: string, assertion: () => void) => {
				try {
					await waitFor(assertion);
				} catch (cause) {
					throw new Error(
						`Media-adjacent deletion failed: ${JSON.stringify(
							{
								phase,
								actual: documentText(
									args.onChange.mock.lastCall?.[0] ?? args.value,
								),
								undo: ui
									.queryByRole("button", { name: /^復原（/ })
									?.getAttribute("title"),
							},
							null,
							2,
						)}`,
						{ cause },
					);
				}
			};
			const target = editableText(dialog, "刪除這段文字");
			await user.click(target);
			await checkPhase("initialHistoryEmpty", () => {
				expect(
					ui.getByRole("button", { name: "目前沒有可復原的動作" }),
				).toBeDisabled();
				expect(
					documentText(args.onChange.mock.lastCall?.[0] ?? args.value),
				).toEqual(["刪除這段文字", "表格後的文字保留"]);
			});
			selectTextRange(target, 0, target, "刪除這段文字".length);
			// A real held key lets the input save observe Chromium's temporary
			// <br> before Paragraph's keyup cleanup. Both saves are one action.
			await user.keyboard("{Backspace>}");
			await new Promise<void>((resolve) =>
				requestAnimationFrame(() => resolve()),
			);
			await user.keyboard("{/Backspace}");
			const assertMedia = () => {
				const value = args.onChange.mock.lastCall?.[0] ?? args.value;
				expect(value.blocks.filter((block) => block.type === "image")).toEqual([
					expect.objectContaining({
						assetId: "story-image",
						imageKey: "public/storybook/deletion-fixture.svg",
						alt: "本地測試圖片",
					}),
				]);
				expect(
					JSON.parse(
						value.blocks.find((block) => block.type === "table")?.tableJson ??
							"null",
					),
				).toEqual(JSON.parse(tableJson));
				expect(within(dialog).getByText("展示文字")).toBeVisible();
				expect(
					within(dialog).getByRole("img", { name: "本地測試圖片" }),
				).toBeVisible();
			};
			await checkPhase("deletedTextSaved", () => {
				expect(
					documentText(args.onChange.mock.lastCall?.[0] ?? args.value),
				).toEqual(["表格後的文字保留"]);
				expect(
					ui.getByRole("button", {
						name: "復原（⌘/Ctrl+Z，還可復原 1 步）",
					}),
				).toBeEnabled();
				assertMedia();
			});
			await undo(dialog, user);
			await checkPhase("undoRestoredText", () => {
				expect(
					documentText(args.onChange.mock.lastCall?.[0] ?? args.value),
				).toEqual(["刪除這段文字", "表格後的文字保留"]);
				expect(
					ui.getByRole("button", { name: "目前沒有可復原的動作" }),
				).toBeDisabled();
				expect(editableText(dialog, "刪除這段文字")).toBeVisible();
				assertMedia();
			});
			await expect(args.uploadImageFile).not.toHaveBeenCalled();
		}),
};

export const RapidDeleteThenImmediateUndo: Story = {
	name: "快速刪除 · 立即復原與重做",
	args: { value: paragraphs("快速刪除仍可完整復原") },
	play: async ({ canvasElement, args, globals }) =>
		nativeDeletionTest(canvasElement, globals, async (dialog, user) => {
			const text = "快速刪除仍可完整復原";
			const target = editableText(dialog, text);
			await user.click(target);
			selectTextRange(target, text.length - 2, target, text.length);
			// One native key sequence: no wait for Editor.js's delayed onChange.
			await user.keyboard("{Backspace}{Control>}z{/Control}");
			await waitFor(() => {
				expect(
					documentText(args.onChange.mock.lastCall?.[0] ?? args.value),
				).toEqual([text]);
				expect(editableText(dialog, text)).toBeVisible();
			});
			for (let cycle = 0; cycle < 2; cycle += 1) {
				await redo(dialog, user);
				await waitFor(() => {
					expect(
						documentText(args.onChange.mock.lastCall?.[0] ?? args.value),
					).toEqual([text.slice(0, -2)]);
					expect(editableText(dialog, text.slice(0, -2))).toBeVisible();
				});
				await undo(dialog, user);
				await waitFor(() => {
					expect(
						documentText(args.onChange.mock.lastCall?.[0] ?? args.value),
					).toEqual([text]);
					expect(editableText(dialog, text)).toBeVisible();
				});
			}
		}),
};

export const RapidUndoThenTyping: Story = {
	name: "復原後立刻輸入 · 保留新舊文字",
	args: { value: paragraphs("原本內容不能遺失") },
	play: async ({ canvasElement, args, globals }) =>
		nativeDeletionTest(canvasElement, globals, async (dialog, user) => {
			const original = "原本內容不能遺失";
			const insertion = "新增";
			const target = editableText(dialog, original);
			await user.click(target);
			selectTextRange(target, original.length - 2, target, original.length);
			// The browser continues typing immediately after Undo, while the
			// asynchronous snapshot apply may still be rebuilding the paragraph.
			await user.keyboard(`{Backspace}{Control>}z{/Control}${insertion}`);
			await waitFor(() => {
				const text = documentText(
					args.onChange.mock.lastCall?.[0] ?? args.value,
				).join("");
				expect(text.split(insertion)).toHaveLength(2);
				expect(text.replace(insertion, "")).toBe(original);
				expect(editableText(dialog, text)).toBeVisible();
			});
			await undo(dialog, user);
			await waitFor(() => {
				expect(
					documentText(args.onChange.mock.lastCall?.[0] ?? args.value),
				).toEqual([original]);
				expect(editableText(dialog, original)).toBeVisible();
			});
		}),
};
