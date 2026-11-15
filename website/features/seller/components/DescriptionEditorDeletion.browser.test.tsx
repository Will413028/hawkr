import { within } from "@testing-library/dom";
import { describe, expect, it, onTestFinished, vi } from "vitest";
import { page, userEvent } from "vitest/browser";
import type { ProductDescription } from "@/lib/products/seller-repo";
import { expectNoAxeViolations } from "@/tests/browser/axe";
import { mountEditor } from "@/tests/browser/description-editor";
import {
	descriptionImageDataUrl,
	startDescriptionImageFixtures,
} from "@/tests/browser/description-image-fixtures";
import { waitFor } from "@/tests/browser/dom";

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

async function nativeDeletionTest(
	canvasElement: HTMLElement,
	test: (dialog: HTMLElement, user: NativeUserEvent) => Promise<void>,
) {
	const dialog = within(canvasElement).getByRole("dialog", {
		name: "商品描述編輯器",
	});
	await waitFor(
		() => expect(dialog).toHaveAttributeSync("data-ready", "true"),
		{
			timeout: 15_000,
		},
	);
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
	// Editor.js supplies these controls without a stable accessible name.
	const settings = dialog.querySelector<HTMLElement>(
		".ce-toolbar__settings-btn",
	);
	if (!settings) throw new Error("找不到區塊調整按鈕");
	const toolbar = settings.closest<HTMLElement>(".ce-toolbar");
	if (!toolbar) throw new Error("找不到區塊工具列");
	const blocks = Array.from(dialog.querySelectorAll<HTMLElement>(".ce-block"));
	await user.hover(
		block.querySelector<HTMLElement>(".ce-block__content") ?? block,
	);
	await waitFor(() => {
		expect(toolbar.classList.contains("ce-toolbar--opened")).toBe(true);
		expect(settings).toBeVisibleSync();
		const top = Number.parseFloat(toolbar.style.top);
		const nearest = blocks.reduce((best, candidate) =>
			Math.abs(candidate.offsetTop - top) < Math.abs(best.offsetTop - top)
				? candidate
				: best,
		);
		expect(nearest).toBe(block);
	});
	await user.click(settings);
	const item = await waitFor(() => {
		const candidate = dialog.querySelector<HTMLElement>(
			'.ce-popover-item[data-item-name="delete"]',
		);
		expect(candidate).toBeVisibleSync();
		if (!candidate) throw new Error("找不到刪除區塊選項");
		return candidate;
	});
	await user.click(item);
}

async function undo(dialog: HTMLElement, user: NativeUserEvent) {
	const t0 = performance.now();
	const undoTitle = () =>
		within(dialog)
			.getAllByRole("button")
			.map((b) => b.getAttribute("title") ?? "")
			.find((t) => t.startsWith("復原（") || t === "目前沒有可復原的動作");
	const initial = undoTitle();
	let final = initial;
	try {
		await waitFor(
			() => {
				final = undoTitle();
				if (!final?.startsWith("復原（")) throw new Error("not yet");
			},
			{ timeout: 3000 },
		);
	} finally {
		console.log(
			`[DEBUG-undo] initial=${initial} final=${final} waitedMs=${Math.round(performance.now() - t0)}`,
		);
	}
	await user.click(within(dialog).getByRole("button", { name: /^復原（/ }));
}

async function redo(dialog: HTMLElement, user: NativeUserEvent) {
	await user.click(within(dialog).getByRole("button", { name: /^重做（/ }));
}

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

const localImage = descriptionImageDataUrl;

const tableJson = JSON.stringify({
	withHeadings: true,
	rows: [
		[{ runs: [{ text: "欄位" }] }, { runs: [{ text: "內容" }] }],
		[{ runs: [{ text: "規格" }] }, { runs: [{ text: "展示文字" }] }],
	],
});

// 媒體 origin 要在被測模組載入前就設定（getEnv 會記住第一次的結果）；沒設時圖片區塊會被判定資料無效。
// 與 .github/workflows/website-e2e.yml 的 NEXT_PUBLIC_MEDIA_BASE_URL 相同，fixture 攔的就是這個網域。
vi.hoisted(() => {
	process.env.NEXT_PUBLIC_MEDIA_BASE_URL = "https://test-media.example.com";
});

function mount(
	overrides: Parameters<typeof mountEditor>[0] = {},
	options: Parameters<typeof mountEditor>[1] = {},
) {
	return mountEditor(
		{ value: paragraphs("第一段保留", "第二段保留"), ...overrides },
		options,
	);
}

describe("DescriptionEditorDeletion", () => {
	it("段落開頭 · Backspace 合併與復原", async () => {
		const { container: canvasElement, args } = mount({
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
		});
		await nativeDeletionTest(canvasElement, async (dialog, user) => {
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
				expect(editableText(dialog, "第一段保留第二段保留")).toBeVisibleSync();
			});
			await undo(dialog, user);
			await waitFor(() => {
				expect(
					documentText(args.onChange.mock.lastCall?.[0] ?? args.value),
				).toEqual(["第一段保留", "第二段保留"]);
				expect(editableText(dialog, "第二段保留")).toBeVisibleSync();
			});
		});
	});

	it("段落結尾 · Delete 合併與復原", { repeats: 40 }, async () => {
		const { container: canvasElement, args } = mount({});
		await nativeDeletionTest(canvasElement, async (dialog, user) => {
			const first = editableText(dialog, "第一段保留");
			await user.click(first);
			setCaret(first, "end");
			await user.keyboard("{Delete}");
			await waitFor(() => {
				expect(
					documentText(args.onChange.mock.lastCall?.[0] ?? args.value),
				).toEqual(["第一段保留第二段保留"]);
				expect(editableText(dialog, "第一段保留第二段保留")).toBeVisibleSync();
			});
			await undo(dialog, user);
			await waitFor(() => {
				expect(
					documentText(args.onChange.mock.lastCall?.[0] ?? args.value),
				).toEqual(["第一段保留", "第二段保留"]);
				expect(editableText(dialog, "第一段保留")).toBeVisibleSync();
			});
		});
	});

	it("跨段選取 · 刪除中間內容", async () => {
		const { container: canvasElement, args } = mount({
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
		});
		await nativeDeletionTest(canvasElement, async (dialog, user) => {
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
				expect(editableText(dialog, "前段保留後段保留")).toBeVisibleSync();
			});
			await undo(dialog, user);
			await waitFor(() => {
				expect(
					documentText(args.onChange.mock.lastCall?.[0] ?? args.value),
				).toEqual(["前段保留刪掉", "整段刪掉", "刪掉後段保留"]);
				expect(editableText(dialog, "整段刪掉")).toBeVisibleSync();
			});
		});
	});

	it("跨段刪除 · 內文到清單保留後續項目", async () => {
		const { container: canvasElement, args } = mount({
			value: paragraphToListValue,
		});
		await nativeDeletionTest(canvasElement, async (dialog, user) => {
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
				expect(editableText(dialog, "段落前端清單尾端")).toBeVisibleSync();
				expect(editableText(dialog, "保留的後續項目")).toBeVisibleSync();
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
				expect(editableText(dialog, "刪除終點清單尾端")).toBeVisibleSync();
			});
		});
	});

	it("跨段刪除 · 清單到標題保留前方項目", async () => {
		const { container: canvasElement, args } = mount({
			value: listToHeadingValue,
		});
		await nativeDeletionTest(canvasElement, async (dialog, user) => {
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
				expect(editableText(dialog, "前方項目保留")).toBeVisibleSync();
				expect(editableText(dialog, "清單前端標題尾端")).toBeVisibleSync();
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
				expect(editableText(dialog, "刪除終點標題尾端")).toBeVisibleSync();
			});
		});
	});

	it("區塊選單 · 刪除滑鼠指向的段落", async () => {
		// Editor.js 在 650px 以下視為手機版，不依滑鼠位置移動區塊工具列。
		await page.viewport(1280, 800);
		const { container: canvasElement, args } = mount({
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
		});
		await nativeDeletionTest(canvasElement, async (dialog, user) => {
			await expect(editableText(dialog, "第一段保留")).toHaveStyleSync({
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
				expect(within(dialog).queryByText("第一段保留")).toBeNull();
				expect(editableText(dialog, "第二段保留")).toBeVisibleSync();
			});
			await expectNoAxeViolations("DescriptionEditor after block deletion");
			await undo(dialog, user);
			await waitFor(() => {
				expect(
					documentText(args.onChange.mock.lastCall?.[0] ?? args.value),
				).toEqual(["第一段保留", "第二段保留"]);
				expect(editableText(dialog, "第一段保留")).toBeVisibleSync();
				expect(editableText(dialog, "第一段保留")).toHaveStyleSync({
					textAlign: "center",
				});
				expect(
					(args.onChange.mock.lastCall?.[0] ?? args.value).blocks.find(
						(block) => block.id === "deletion-paragraph-0",
					)?.style?.textAlign,
				).toBe("center");
			});
		});
	});

	it("最後一段 · 刪除與反覆復原重做", async () => {
		const { container: canvasElement, args } = mount({
			value: paragraphs("最後一段也可以找回"),
		});
		await nativeDeletionTest(canvasElement, async (dialog, user) => {
			const text = "最後一段也可以找回";
			await user.click(editableText(dialog, text));
			await deleteFromBlockMenu(dialog, editableText(dialog, text), user);
			await waitFor(() => {
				expect(args.onChange.mock.lastCall?.[0].blocks).toEqual([]);
				expect(within(dialog).queryByText(text)).toBeNull();
			});
			for (let cycle = 0; cycle < 3; cycle += 1) {
				await undo(dialog, user);
				await waitFor(() => {
					expect(
						documentText(args.onChange.mock.lastCall?.[0] ?? args.value),
					).toEqual([text]);
					expect(editableText(dialog, text)).toBeVisibleSync();
				});
				await redo(dialog, user);
				await waitFor(() => {
					expect(args.onChange.mock.lastCall?.[0].blocks).toEqual([]);
					expect(within(dialog).queryByText(text)).toBeNull();
				});
			}
		});
	});

	it("空白清單項目 · Enter 回到內文", async () => {
		const { container: canvasElement, args } = mount({
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
		});
		await nativeDeletionTest(canvasElement, async (dialog, user) => {
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
				expect(editableText(dialog, "離開清單後的文字")).toBeVisibleSync();
			});
		});
	});

	it("最後一次存檔失敗 · 補存後 onChange 跟上", async () => {
		const { container: canvasElement, args } = mount({
			value: paragraphs("第一段"),
		});
		await nativeDeletionTest(canvasElement, async (dialog, user) => {
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
		});
	});

	it("清單與標題交界 · Backspace 保留內容", async () => {
		const { container: canvasElement, args } = mount({
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
		});
		await nativeDeletionTest(canvasElement, async (dialog, user) => {
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
				expect(editableText(dialog, "## 保存與料理")).toBeVisibleSync();
				expect(
					within(dialog).getByText("保存方式依包裝標示"),
				).toBeVisibleSync();
			});
		});
	});

	it("圖片與表格之間 · 刪字保留資料", async () => {
		const fixtures = await startDescriptionImageFixtures();
		onTestFinished(() => fixtures.stop());
		const { container: canvasElement, args } = mount({
			value: {
				version: "1.0",
				blocks: [
					{
						id: "preserved-image",
						type: "image",
						assetId: "story-image",
						imageKey: "public/fixtures/deletion-fixture.svg",
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
		});
		await nativeDeletionTest(canvasElement, async (dialog, user) => {
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
				).toBeDisabledSync();
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
						imageKey: "public/fixtures/deletion-fixture.svg",
						alt: "本地測試圖片",
					}),
				]);
				expect(
					JSON.parse(
						value.blocks.find((block) => block.type === "table")?.tableJson ??
							"null",
					),
				).toEqual(JSON.parse(tableJson));
				expect(within(dialog).getByText("展示文字")).toBeVisibleSync();
				expect(
					within(dialog).getByRole("img", { name: "本地測試圖片" }),
				).toBeVisibleSync();
			};
			await checkPhase("deletedTextSaved", () => {
				expect(
					documentText(args.onChange.mock.lastCall?.[0] ?? args.value),
				).toEqual(["表格後的文字保留"]);
				expect(
					ui.getByRole("button", {
						name: "復原（⌘/Ctrl+Z，還可復原 1 步）",
					}),
				).toBeEnabledSync();
				assertMedia();
			});
			await undo(dialog, user);
			await checkPhase("undoRestoredText", () => {
				expect(
					documentText(args.onChange.mock.lastCall?.[0] ?? args.value),
				).toEqual(["刪除這段文字", "表格後的文字保留"]);
				expect(
					ui.getByRole("button", { name: "目前沒有可復原的動作" }),
				).toBeDisabledSync();
				expect(editableText(dialog, "刪除這段文字")).toBeVisibleSync();
				assertMedia();
			});
			await expect(args.uploadImageFile).not.toHaveBeenCalled();
		});
	});

	it("快速刪除 · 立即復原與重做", async () => {
		const { container: canvasElement, args } = mount({
			value: paragraphs("快速刪除仍可完整復原"),
		});
		await nativeDeletionTest(canvasElement, async (dialog, user) => {
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
				expect(editableText(dialog, text)).toBeVisibleSync();
			});
			for (let cycle = 0; cycle < 2; cycle += 1) {
				await redo(dialog, user);
				await waitFor(() => {
					expect(
						documentText(args.onChange.mock.lastCall?.[0] ?? args.value),
					).toEqual([text.slice(0, -2)]);
					expect(editableText(dialog, text.slice(0, -2))).toBeVisibleSync();
				});
				await undo(dialog, user);
				await waitFor(() => {
					expect(
						documentText(args.onChange.mock.lastCall?.[0] ?? args.value),
					).toEqual([text]);
					expect(editableText(dialog, text)).toBeVisibleSync();
				});
			}
		});
	});

	it("復原後立刻輸入 · 保留新舊文字", async () => {
		const { container: canvasElement, args } = mount({
			value: paragraphs("原本內容不能遺失"),
		});
		await nativeDeletionTest(canvasElement, async (dialog, user) => {
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
				expect(editableText(dialog, text)).toBeVisibleSync();
			});
			await undo(dialog, user);
			await waitFor(() => {
				expect(
					documentText(args.onChange.mock.lastCall?.[0] ?? args.value),
				).toEqual([original]);
				expect(editableText(dialog, original)).toBeVisibleSync();
			});
		});
	});
});
