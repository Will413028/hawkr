import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";
import { expect, fn, userEvent, waitFor, within } from "storybook/test";
import { DescriptionEditorCanvas } from "@/features/seller/components/DescriptionEditor";
import type { MediaUploadRepo } from "@/lib/media/upload";
import type { ProductDescription } from "@/lib/products/seller-repo";
import "@/styles/features/seller-center.css";

const productDescription: ProductDescription = {
	version: "1.0",
	blocks: [
		{
			id: "product-information",
			type: "text",
			runs: [{ text: "商品資訊" }],
			style: { fontSize: "26px", fontWeight: 700 },
		},
		{
			id: "product-introduction",
			type: "text",
			runs: [
				{ text: "用一段話介紹商品特色，" },
				{ text: "讓買家快速找到重點", bold: true },
				{ text: "。這裡的文字僅供編輯器展示。" },
			],
		},
		{
			id: "product-specifications",
			type: "list",
			listJson: JSON.stringify({
				ordered: false,
				items: [
					{ runs: [{ text: "規格：依商品包裝上的正式標示填寫。" }] },
					{ runs: [{ text: "產地：填入可核對的來源資訊。" }] },
					{ runs: [{ text: "成分與過敏原：請保留完整內容。" }] },
				],
			}),
		},
		{
			id: "storage-information",
			type: "text",
			runs: [{ text: "保存與料理" }],
			style: { fontSize: "20px", fontWeight: 700 },
		},
		{
			id: "storage-description",
			type: "text",
			runs: [{ text: "依包裝標示填寫保存條件、有效日期與料理建議。" }],
		},
	],
};

const importedDescription: ProductDescription = {
	version: "1.0",
	blocks: [
		{
			id: "imported-heading",
			type: "text",
			runs: [{ text: "## 商品資訊", color: "#682e0e" }],
			style: { fontSize: "26px", fontWeight: 700 },
		},
		{
			id: "imported-list",
			type: "list",
			listJson: JSON.stringify({
				ordered: false,
				items: [
					{ runs: [{ text: "- 標稱規格合計：445g（265g＋180g）" }] },
					{ runs: [{ text: "• - 保存條件：冷凍 −18°C" }] },
					{ runs: [{ text: "- 實際效期依各包正式標示" }] },
					{ runs: [{ text: "" }] },
				],
			}),
		},
		{
			id: "imported-storage",
			type: "text",
			runs: [{ text: "## 保存與料理" }],
		},
		{
			id: "imported-preserved-text",
			type: "text",
			runs: [{ text: "請依各包正式解凍及料理指引安排火候。", bold: true }],
		},
	],
};

const editableDescription: ProductDescription = {
	version: "1.0",
	blocks: [
		{
			id: "editable-introduction",
			type: "text",
			runs: [{ text: "選取文字後即可調整格式" }],
		},
	],
};

const meta = {
	title: "Web/Seller Center/DescriptionEditor",
	component: DescriptionEditorCanvas,
	args: {
		value: productDescription,
		onChange: fn<(next: ProductDescription) => void>(),
		uploadImageFile: fn<MediaUploadRepo["uploadImageFile"]>(async () => {
			throw new Error("元件庫展示不會上傳圖片，請在正式商品表單操作。");
		}),
	},
	argTypes: {
		value: {
			control: "object",
			description: "與正式商品表單相同的 1.0 描述文件。",
		},
		onChange: { description: "內容變更時回傳完整文件；故事只更新本地 state。" },
		uploadImageFile: {
			control: false,
			description:
				"由正式賣家資料層注入的圖片上傳方法；故事使用拒絕網路操作的 stub。",
		},
	},
	parameters: {
		layout: "padded",
		docs: {
			description: {
				component:
					"正式商品描述編輯器的獨立 Canvas，包含文字格式、清單、格式整理、復原／重做與閱讀預覽。正式 DescriptionEditor 由 seller scope 注入上傳方法；此處所有內容均為固定展示資料，只保留於本地 state，不讀寫商品、不建立媒體上傳交易。\n\n實際使用：`import { DescriptionEditorCanvas } from '@/features/seller/components/DescriptionEditor'`。此專案不是 hawkr-web npm 套件。閱讀預覽呈現目前文件的文字排版，不代表商品詳情頁的完整佈局。",
			},
		},
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

async function waitForEditor(canvasElement: HTMLElement) {
	const dialog = within(canvasElement).getByRole("dialog", {
		name: "商品描述編輯器",
	});
	await waitFor(
		() => {
			expect(dialog).toHaveAttribute("data-ready", "true");
			expect(dialog).toHaveAttribute("aria-busy", "false");
			expect(dialog).toBeVisible();
		},
		{ timeout: 15_000 },
	);
	return dialog;
}

function paragraphEditor(dialog: HTMLElement) {
	// Editor.js owns these contenteditable nodes; they have no native textbox role.
	const editable = dialog.querySelector<HTMLElement>(
		'.ce-paragraph[contenteditable="true"]',
	);
	if (!editable) throw new Error("編輯器沒有可輸入的段落");
	return editable;
}

function selectContents(editable: HTMLElement) {
	editable.focus();
	const doc = editable.ownerDocument;
	const selection = doc.getSelection();
	const range = doc.createRange();
	range.selectNodeContents(editable);
	selection?.removeAllRanges();
	selection?.addRange(range);
	doc.dispatchEvent(new Event("selectionchange"));
}

function textBlocks(description: ProductDescription | undefined) {
	return description?.blocks
		.filter((block) => block.type === "text")
		.map(
			(block) => block.runs?.map((run) => run.text).join("") ?? block.content,
		)
		.join("\n");
}

export const ProductDocument: Story = {
	name: "商品描述 · 日常編輯",
	play: async ({ canvasElement, args }) => {
		const dialog = await waitForEditor(canvasElement);
		await expect(
			within(dialog).getByRole("toolbar", { name: "文字樣式" }),
		).toBeVisible();
		await expect(within(dialog).getByText("商品資訊")).toBeVisible();
		await expect(args.uploadImageFile).not.toHaveBeenCalled();
	},
};

export const ImportedFormatting: Story = {
	name: "匯入內容 · 待整理格式",
	args: { value: importedDescription },
	parameters: {
		docs: {
			description: {
				story:
					"重現文字標題殘留 ##、清單內重複符號與空白項目，供新舊編輯體驗對照。內容不會在載入時自行改寫；按整理格式才套用。",
			},
		},
	},
	play: async ({ canvasElement }) => {
		const dialog = await waitForEditor(canvasElement);
		// Readiness and painted content are separate: query the live node on retry.
		await waitFor(() => {
			expect(within(dialog).getByText("## 商品資訊")).toBeVisible();
			expect(
				within(dialog).getByRole("button", { name: "整理格式" }),
			).toBeEnabled();
		});
	},
};

export const Empty: Story = {
	name: "空白內容 · 開始輸入",
	args: { value: { version: "1.0", blocks: [] } },
	play: async ({ canvasElement, args }) => {
		const dialog = await waitForEditor(canvasElement);
		const paragraph = paragraphEditor(dialog);
		await userEvent.click(paragraph);
		await userEvent.type(paragraph, "介紹商品的特色與來源");
		await waitFor(() =>
			expect(textBlocks(args.onChange.mock.lastCall?.[0])).toContain(
				"介紹商品的特色與來源",
			),
		);
		await expect(paragraph).toHaveTextContent("介紹商品的特色與來源");
		await expect(args.uploadImageFile).not.toHaveBeenCalled();
	},
};

export const Mobile: Story = {
	name: "窄版 · 375px 編輯區",
	args: { value: productDescription },
	decorators: [
		(Story) => (
			<div style={{ width: 375, maxWidth: "100%", marginInline: "auto" }}>
				<Story />
			</div>
		),
	],
	parameters: {
		docs: {
			description: {
				story:
					"固定 375px 容器；裝置 viewport、觸控與虛擬鍵盤仍需另以瀏覽器或實機驗證。",
			},
		},
	},
	play: async ({ canvasElement }) => {
		const dialog = await waitForEditor(canvasElement);
		await expect(dialog.getBoundingClientRect().width).toBeLessThanOrEqual(375);
		const editor = within(dialog);
		await userEvent.click(editor.getByRole("button", { name: "更多格式" }));
		await expect(editor.getByRole("button", { name: "置中" })).toBeVisible();
		await userEvent.click(editor.getByRole("button", { name: "更多格式" }));
		await userEvent.click(editor.getByRole("button", { name: "放大編輯" }));
		await expect(
			editor.getByRole("button", { name: "縮小編輯區" }),
		).toHaveAttribute("aria-expanded", "true");
		await userEvent.click(editor.getByRole("button", { name: "縮小編輯區" }));
		await waitFor(() =>
			expect(editor.getByRole("button", { name: "放大編輯" })).toBeVisible(),
		);
	},
};

export const KeyboardFormattingAndUndo: Story = {
	name: "鍵盤 · 選取、粗體與復原",
	args: { value: editableDescription },
	play: async ({ canvasElement, args }) => {
		const dialog = await waitForEditor(canvasElement);
		const editor = within(dialog);
		const paragraph = paragraphEditor(dialog);
		await userEvent.click(paragraph);
		selectContents(paragraph);
		const boldButton = editor.getByRole("button", { name: "粗體（⌘B）" });
		boldButton.focus();
		await userEvent.keyboard("{Enter}");
		await waitFor(() => {
			expect(args.onChange.mock.lastCall?.[0].blocks[0].runs).toEqual([
				{ text: "選取文字後即可調整格式", bold: true },
			]);
		});
		await expect(boldButton).toHaveAttribute("aria-pressed", "true");
		await userEvent.keyboard("{Control>}z{/Control}");
		await waitFor(() => {
			expect(args.onChange.mock.lastCall?.[0].blocks[0].runs).toEqual([
				{ text: "選取文字後即可調整格式" },
			]);
		});
		await userEvent.keyboard("{Control>}{Shift>}z{/Shift}{/Control}");
		await waitFor(() => {
			expect(args.onChange.mock.lastCall?.[0].blocks[0].runs).toEqual([
				{ text: "選取文字後即可調整格式", bold: true },
			]);
		});
	},
};

export const MarkdownPaste: Story = {
	name: "貼上 · 標題與清單",
	args: { value: { version: "1.0", blocks: [] } },
	play: async ({ canvasElement, args }) => {
		const dialog = await waitForEditor(canvasElement);
		await userEvent.click(paragraphEditor(dialog));
		await userEvent.paste(
			"## 商品資訊\n\n- 規格：445g\n- 保存條件：冷凍 −18°C\n\n請依包裝標示保存。",
		);
		await waitFor(() => {
			const description = args.onChange.mock.lastCall?.[0];
			expect(textBlocks(description)).toContain("商品資訊");
			expect(textBlocks(description)).not.toContain("##");
			expect(description?.blocks).toEqual(
				expect.arrayContaining([expect.objectContaining({ type: "list" })]),
			);
		});
		await waitFor(() => {
			expect(within(dialog).getByText("商品資訊")).toBeVisible();
			expect(within(dialog).getByText("規格：445g")).toBeVisible();
		});
		await expect(args.uploadImageFile).not.toHaveBeenCalled();
	},
};

export const MultiParagraphListAndUndo: Story = {
	name: "多段選取 · 合併清單與復原",
	args: {
		value: {
			version: "1.0",
			blocks: ["商品規格", "來源資訊", "保存方式"].map((text, index) => ({
				id: `selected-paragraph-${index}`,
				type: "text" as const,
				runs: [{ text }],
			})),
		},
	},
	play: async ({ canvasElement, args }) => {
		const dialog = await waitForEditor(canvasElement);
		const editor = within(dialog);
		const paragraphs = dialog.querySelectorAll<HTMLElement>(
			'.ce-paragraph[contenteditable="true"]',
		);
		await expect(paragraphs).toHaveLength(3);
		await userEvent.click(paragraphs[0]);
		const doc = dialog.ownerDocument;
		const selection = doc.getSelection();
		const range = doc.createRange();
		range.setStart(paragraphs[0], 0);
		range.setEnd(paragraphs[2], paragraphs[2].childNodes.length);
		selection?.removeAllRanges();
		selection?.addRange(range);
		doc.dispatchEvent(new Event("selectionchange"));
		await userEvent.click(
			editor.getByRole("button", { name: /^套用項目符號/ }),
		);
		await waitFor(() => {
			const description = args.onChange.mock.lastCall?.[0];
			expect(description?.blocks).toHaveLength(1);
			const list = description?.blocks[0];
			expect(list?.type).toBe("list");
			expect(JSON.parse(list?.listJson ?? "null")).toEqual({
				ordered: false,
				items: ["商品規格", "來源資訊", "保存方式"].map((text) => ({
					runs: [{ text }],
				})),
			});
		});
		await userEvent.click(editor.getByRole("button", { name: /^復原（/ }));
		await waitFor(() => {
			expect(textBlocks(args.onChange.mock.lastCall?.[0])).toBe(
				"商品規格\n來源資訊\n保存方式",
			);
			const restoredParagraphs = Array.from(
				dialog.querySelectorAll('.ce-paragraph[contenteditable="true"]'),
			)
				.map((paragraph) => paragraph.textContent?.trim())
				.filter(Boolean);
			// Editor.js may retain an empty insertion paragraph after the list.
			expect(restoredParagraphs).toEqual(["商品規格", "來源資訊", "保存方式"]);
		});
	},
};

export const HeadingAndAlignment: Story = {
	name: "段落樣式 · 標題與對齊",
	args: { value: editableDescription },
	play: async ({ canvasElement, args }) => {
		const dialog = await waitForEditor(canvasElement);
		const editor = within(dialog);
		const paragraph = paragraphEditor(dialog);
		await userEvent.click(paragraph);
		selectContents(paragraph);
		await userEvent.click(editor.getByRole("button", { name: "大標（⌘⇧H）" }));
		await waitFor(() => {
			expect(args.onChange.mock.lastCall?.[0].blocks[0].style?.fontSize).toBe(
				"26px",
			);
			expect(textBlocks(args.onChange.mock.lastCall?.[0])).toBe(
				"選取文字後即可調整格式",
			);
		});
		await waitFor(() =>
			expect(
				editor.getByRole("button", { name: "大標（⌘⇧H）" }),
			).toHaveAttribute("aria-pressed", "true"),
		);
		await userEvent.click(editor.getByRole("button", { name: "更多格式" }));
		await userEvent.click(editor.getByRole("button", { name: "置中" }));
		await waitFor(() =>
			expect(args.onChange.mock.lastCall?.[0].blocks[0].style?.textAlign).toBe(
				"center",
			),
		);
		await waitFor(() =>
			expect(editor.getByRole("button", { name: "置中" })).toHaveAttribute(
				"aria-pressed",
				"true",
			),
		);
		await userEvent.click(editor.getByRole("button", { name: "小標" }));
		await waitFor(() =>
			expect(args.onChange.mock.lastCall?.[0].blocks[0].style?.fontSize).toBe(
				"20px",
			),
		);
		await userEvent.click(editor.getByRole("button", { name: "內文" }));
		await waitFor(() => {
			const description = args.onChange.mock.lastCall?.[0];
			expect(description?.blocks[0].style?.fontSize).toBeUndefined();
			expect(description?.blocks[0].style?.textAlign).toBe("center");
			expect(textBlocks(description)).toBe("選取文字後即可調整格式");
		});
		await waitFor(() =>
			expect(editor.getByRole("button", { name: "內文" })).toHaveAttribute(
				"aria-pressed",
				"true",
			),
		);
	},
};

export const ListAfterEmptyParagraph: Story = {
	name: "空段落之後 · 清單選取與對齊",
	args: {
		value: {
			version: "1.0",
			blocks: [
				{ id: "leading-empty-paragraph", type: "text", runs: [] },
				{
					id: "selected-centered-paragraph",
					type: "text",
					runs: [{ text: "這一段改成清單" }],
					style: { textAlign: "center" },
				},
				{
					id: "untouched-following-paragraph",
					type: "text",
					runs: [{ text: "這一段維持內文" }],
				},
			],
		},
	},
	parameters: {
		docs: {
			description: {
				story:
					"空段落仍可編輯，但不會出現在儲存資料中；操作必須持續作用於使用者選取的段落。切換清單種類應保留內容、對齊與相鄰段落。",
			},
		},
	},
	play: async ({ canvasElement, args }) => {
		const dialog = await waitForEditor(canvasElement);
		const editor = within(dialog);
		const paragraphs = dialog.querySelectorAll<HTMLElement>(
			'.ce-paragraph[contenteditable="true"]',
		);
		await expect(paragraphs).toHaveLength(3);
		await expect(paragraphs[0].textContent?.trim()).toBe("");
		const selected = editor.getByText("這一段改成清單");
		await userEvent.click(selected);
		selectContents(selected);
		await userEvent.click(
			editor.getByRole("button", { name: /^套用項目符號/ }),
		);
		await waitFor(() => {
			const description = args.onChange.mock.lastCall?.[0];
			expect(description?.blocks.map((block) => block.type)).toEqual([
				"list",
				"text",
			]);
			expect(JSON.parse(description?.blocks[0].listJson ?? "null")).toEqual({
				ordered: false,
				items: [{ runs: [{ text: "這一段改成清單" }] }],
				textAlign: "center",
			});
			expect(textBlocks(description)).toBe("這一段維持內文");
		});
		await waitFor(() =>
			expect(editor.getByText("這一段改成清單")).toBeVisible(),
		);
		await expect(editor.getByText("這一段維持內文")).toBeVisible();
		await userEvent.click(editor.getByText("這一段改成清單"));
		await userEvent.click(
			editor.getByRole("button", { name: /^套用編號清單/ }),
		);
		await waitFor(() => {
			const description = args.onChange.mock.lastCall?.[0];
			expect(description?.blocks.map((block) => block.type)).toEqual([
				"list",
				"text",
			]);
			expect(JSON.parse(description?.blocks[0].listJson ?? "null")).toEqual({
				ordered: true,
				items: [{ runs: [{ text: "這一段改成清單" }] }],
				textAlign: "center",
			});
			expect(textBlocks(description)).toBe("這一段維持內文");
		});
	},
};

export const CleanupUndoAndPreview: Story = {
	name: "整理格式 · 復原與閱讀預覽",
	args: { value: importedDescription },
	play: async ({ canvasElement, args }) => {
		const dialog = await waitForEditor(canvasElement);
		const editor = within(dialog);
		await userEvent.click(editor.getByRole("button", { name: "整理格式" }));
		await waitFor(() => {
			expect(editor.queryByText("## 商品資訊")).not.toBeInTheDocument();
			expect(
				editor.getByText("標稱規格合計：445g（265g＋180g）"),
			).toBeVisible();
			expect(editor.getByText("保存條件：冷凍 −18°C")).toBeVisible();
			expect(textBlocks(args.onChange.mock.lastCall?.[0])).toContain(
				"請依各包正式解凍及料理指引安排火候。",
			);
		});
		await waitFor(() => expect(dialog).toHaveAttribute("aria-busy", "false"));
		await userEvent.click(editor.getByRole("button", { name: /^復原（/ }));
		await waitFor(() => {
			expect(dialog).toHaveAttribute("aria-busy", "false");
			expect(editor.getByText("## 商品資訊")).toBeVisible();
			expect(editor.getByRole("button", { name: /^重做（/ })).toBeEnabled();
		});
		await userEvent.click(editor.getByRole("button", { name: /^重做（/ }));
		await waitFor(() => {
			expect(dialog).toHaveAttribute("aria-busy", "false");
			expect(editor.queryByText("## 商品資訊")).not.toBeInTheDocument();
			expect(editor.getByRole("button", { name: "閱讀預覽" })).toBeEnabled();
		});
		await userEvent.click(editor.getByRole("button", { name: "閱讀預覽" }));
		await waitFor(() => {
			expect(dialog).toHaveAttribute("aria-busy", "false");
			expect(dialog).toHaveAttribute("data-preview", "true");
			expect(editor.getByRole("button", { name: "繼續編輯" })).toBeVisible();
			expect(editor.getByRole("button", { name: "繼續編輯" })).toBeEnabled();
			const preview = editor.getByRole("region", {
				name: "商品描述閱讀預覽",
			});
			expect(preview).toHaveTextContent("標稱規格合計：445g（265g＋180g）");
			expect(preview.querySelector('[contenteditable="true"]')).toBeNull();
		});
		await userEvent.click(editor.getByRole("button", { name: "繼續編輯" }));
		await waitFor(() => {
			expect(dialog).toHaveAttribute("aria-busy", "false");
			expect(dialog).toHaveAttribute("data-preview", "false");
			expect(editor.getByRole("toolbar", { name: "文字樣式" })).toBeVisible();
			const content = editor.getByRole("region", {
				name: "商品描述編輯內容",
			});
			expect(content).toHaveTextContent("標稱規格合計：445g（265g＋180g）");
			expect(content).toHaveTextContent("請依各包正式解凍及料理指引安排火候。");
			expect(content.querySelector('[contenteditable="true"]')).not.toBeNull();
		});
		await expect(args.uploadImageFile).not.toHaveBeenCalled();
	},
};
