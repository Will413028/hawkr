import { within } from "@testing-library/dom";
import { describe, expect, it } from "vitest";
import { userEvent } from "vitest/browser";
import type { ProductDescription } from "@/lib/products/seller-repo";
import {
	clickRedo,
	clickUndo,
	mountEditor,
} from "@/tests/browser/description-editor";
import { expectTextContains, waitFor } from "@/tests/browser/dom";

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

async function waitForEditor(canvasElement: HTMLElement) {
	const dialog = within(canvasElement).getByRole("dialog", {
		name: "商品描述編輯器",
	});
	await waitFor(
		() => {
			expect(dialog).toHaveAttributeSync("data-ready", "true");
			expect(dialog).toHaveAttributeSync("aria-busy", "false");
			expect(dialog).toBeVisibleSync();
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

// 對應 user-event 的 paste(text)：在目前焦點元素送出帶 text/plain 的 paste 事件，
// 沒有被 preventDefault 才插入文字（vitest/browser 的 paste 只讀系統剪貼簿，不能指定文字）。
function pasteText(target: HTMLElement, text: string) {
	target.focus();
	const clipboardData = new DataTransfer();
	clipboardData.setData("text/plain", text);
	const accepted = target.dispatchEvent(
		new ClipboardEvent("paste", {
			clipboardData,
			bubbles: true,
			cancelable: true,
			composed: true,
		}),
	);
	if (accepted) document.execCommand("insertText", false, text);
}

function mount(
	overrides: Parameters<typeof mountEditor>[0] = {},
	options: Parameters<typeof mountEditor>[1] = {},
) {
	return mountEditor({ value: productDescription, ...overrides }, options);
}

describe("DescriptionEditor", () => {
	it("商品描述 · 日常編輯", async () => {
		const { container: canvasElement, args } = mount({});

		const dialog = await waitForEditor(canvasElement);
		await waitFor(() => {
			expect(
				within(dialog).getByRole("toolbar", { name: "文字樣式" }),
			).toBeVisibleSync();
			expect(within(dialog).getByText("商品資訊")).toBeVisibleSync();
		});
		await expect(args.uploadImageFile).not.toHaveBeenCalled();
	});

	it("匯入內容 · 待整理格式", async () => {
		const { container: canvasElement } = mount({ value: importedDescription });

		const dialog = await waitForEditor(canvasElement);
		// Readiness and painted content are separate: query the live node on retry.
		await waitFor(() => {
			expect(within(dialog).getByText("## 商品資訊")).toBeVisibleSync();
			expect(
				within(dialog).getByRole("button", { name: "整理格式" }),
			).toBeEnabledSync();
		});
	});

	it("空白內容 · 開始輸入", async () => {
		const { container: canvasElement, args } = mount({
			value: { version: "1.0", blocks: [] },
		});

		const dialog = await waitForEditor(canvasElement);
		const paragraph = paragraphEditor(dialog);
		await userEvent.click(paragraph);
		await userEvent.type(paragraph, "介紹商品的特色與來源");
		await waitFor(() =>
			expect(textBlocks(args.onChange.mock.lastCall?.[0])).toContain(
				"介紹商品的特色與來源",
			),
		);
		await expectTextContains(
			() => paragraphEditor(dialog),
			"介紹商品的特色與來源",
		);
		await expect(args.uploadImageFile).not.toHaveBeenCalled();
	});

	it("窄版 · 375px 編輯區", async () => {
		const { container: canvasElement } = mount(
			{ value: productDescription },
			{ width: 375 },
		);

		const dialog = await waitForEditor(canvasElement);
		await expect(dialog.getBoundingClientRect().width).toBeLessThanOrEqual(375);
		const editor = within(dialog);
		await userEvent.click(editor.getByRole("button", { name: "更多格式" }));
		await waitFor(() => {
			expect(editor.getByRole("button", { name: "置中" })).toBeVisibleSync();
		});
		await userEvent.click(editor.getByRole("button", { name: "更多格式" }));
		await userEvent.click(editor.getByRole("button", { name: "放大編輯" }));
		await waitFor(() => {
			expect(
				editor.getByRole("button", { name: "縮小編輯區" }),
			).toHaveAttributeSync("aria-expanded", "true");
		});
		await userEvent.click(editor.getByRole("button", { name: "縮小編輯區" }));
		await waitFor(() =>
			expect(
				editor.getByRole("button", { name: "放大編輯" }),
			).toBeVisibleSync(),
		);
	});

	it("鍵盤 · 選取、粗體與復原", async () => {
		const { container: canvasElement, args } = mount({
			value: editableDescription,
		});

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
		await waitFor(() => {
			expect(
				editor.getByRole("button", { name: "粗體（⌘B）" }),
			).toHaveAttributeSync("aria-pressed", "true");
		});
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
	});

	it("貼上 · 標題與清單", async () => {
		const { container: canvasElement, args } = mount({
			value: { version: "1.0", blocks: [] },
		});

		const dialog = await waitForEditor(canvasElement);
		const paragraph = paragraphEditor(dialog);
		await userEvent.click(paragraph);
		pasteText(
			paragraph,
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
			expect(within(dialog).getByText("商品資訊")).toBeVisibleSync();
			expect(within(dialog).getByText("規格：445g")).toBeVisibleSync();
		});
		await expect(args.uploadImageFile).not.toHaveBeenCalled();
	});

	it("多段選取 · 合併清單與復原", async () => {
		const { container: canvasElement, args } = mount({
			value: {
				version: "1.0",
				blocks: ["商品規格", "來源資訊", "保存方式"].map((text, index) => ({
					id: `selected-paragraph-${index}`,
					type: "text" as const,
					runs: [{ text }],
				})),
			},
		});

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
		await clickUndo(editor);
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
	});

	it("段落樣式 · 標題與對齊", async () => {
		const { container: canvasElement, args } = mount({
			value: editableDescription,
		});

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
			).toHaveAttributeSync("aria-pressed", "true"),
		);
		await userEvent.click(editor.getByRole("button", { name: "更多格式" }));
		await userEvent.click(editor.getByRole("button", { name: "置中" }));
		await waitFor(() =>
			expect(args.onChange.mock.lastCall?.[0].blocks[0].style?.textAlign).toBe(
				"center",
			),
		);
		await waitFor(() =>
			expect(editor.getByRole("button", { name: "置中" })).toHaveAttributeSync(
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
			expect(editor.getByRole("button", { name: "內文" })).toHaveAttributeSync(
				"aria-pressed",
				"true",
			),
		);
	});

	it("空段落之後 · 清單選取與對齊", async () => {
		const { container: canvasElement, args } = mount({
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
		});

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
			expect(editor.getByText("這一段改成清單")).toBeVisibleSync(),
		);
		await waitFor(() => {
			expect(editor.getByText("這一段維持內文")).toBeVisibleSync();
		});
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
	});

	it("整理格式 · 復原與閱讀預覽", async () => {
		const { container: canvasElement, args } = mount({
			value: importedDescription,
		});

		const dialog = await waitForEditor(canvasElement);
		const editor = within(dialog);
		await userEvent.click(editor.getByRole("button", { name: "整理格式" }));
		await waitFor(() => {
			expect(editor.queryByText("## 商品資訊")).toBeNull();
			expect(
				editor.getByText("標稱規格合計：445g（265g＋180g）"),
			).toBeVisibleSync();
			expect(editor.getByText("保存條件：冷凍 −18°C")).toBeVisibleSync();
			expect(textBlocks(args.onChange.mock.lastCall?.[0])).toContain(
				"請依各包正式解凍及料理指引安排火候。",
			);
		});
		await waitFor(() =>
			expect(dialog).toHaveAttributeSync("aria-busy", "false"),
		);
		await clickUndo(editor);
		await waitFor(() => {
			expect(dialog).toHaveAttributeSync("aria-busy", "false");
			expect(editor.getByText("## 商品資訊")).toBeVisibleSync();
		});
		await clickRedo(editor);
		await waitFor(() => {
			expect(dialog).toHaveAttributeSync("aria-busy", "false");
			expect(editor.queryByText("## 商品資訊")).toBeNull();
			expect(
				editor.getByRole("button", { name: "閱讀預覽" }),
			).toBeEnabledSync();
		});
		await userEvent.click(editor.getByRole("button", { name: "閱讀預覽" }));
		await waitFor(() => {
			expect(dialog).toHaveAttributeSync("aria-busy", "false");
			expect(dialog).toHaveAttributeSync("data-preview", "true");
			expect(
				editor.getByRole("button", { name: "繼續編輯" }),
			).toBeVisibleSync();
			expect(
				editor.getByRole("button", { name: "繼續編輯" }),
			).toBeEnabledSync();
			const preview = editor.getByRole("region", {
				name: "商品描述閱讀預覽",
			});
			expect(preview.querySelector('[contenteditable="true"]')).toBeNull();
		});
		await expectTextContains(
			() => editor.getByRole("region", { name: "商品描述閱讀預覽" }),
			"標稱規格合計：445g（265g＋180g）",
		);
		await userEvent.click(editor.getByRole("button", { name: "繼續編輯" }));
		await waitFor(() => {
			expect(dialog).toHaveAttributeSync("aria-busy", "false");
			expect(dialog).toHaveAttributeSync("data-preview", "false");
			expect(
				editor.getByRole("toolbar", { name: "文字樣式" }),
			).toBeVisibleSync();
			const content = editor.getByRole("region", {
				name: "商品描述編輯內容",
			});
			expect(content.querySelector('[contenteditable="true"]')).not.toBeNull();
		});
		for (const text of [
			"標稱規格合計：445g（265g＋180g）",
			"請依各包正式解凍及料理指引安排火候。",
		]) {
			await expectTextContains(
				() => editor.getByRole("region", { name: "商品描述編輯內容" }),
				text,
			);
		}
		await expect(args.uploadImageFile).not.toHaveBeenCalled();
	});
});
