import assert from "node:assert/strict";
import test from "node:test";
import type { OutputData } from "@editorjs/editorjs";
import {
	fromEditorData,
	toEditorData,
} from "../lib/products/descriptionEditorJs";
import type { ProductDescription } from "../lib/products/seller-repo";

// list block（issue #1257）：Editor.js `@editorjs/list` 的 { style, items } 與後端
// { type:"list", listJson:'{ordered,items:[{runs}]}' } 對稱橋接，項目內複用 runs。
//
// 註：此測試在 node 環境（無 DOM）跑，htmlToRuns 會走 stripTags fallback，故不斷言
// 行內 <b>/<a> → run 旗標的解析（與既有 description-editorjs-align.test.ts 同限制）。
// 行內粗體 / 連結保留由：(1) toEditorData 的 runs→HTML 產出（下方 href/bold 斷言）
// (2) 真實瀏覽器 ui-verify (3) 後端 round-trip（listJson 含 href/bold 已驗存取）覆蓋。

test("fromEditorData maps a list block to backend listJson (ordered flag + items)", () => {
	const saved = fromEditorData({
		blocks: [
			{
				id: "l1",
				type: "list",
				data: { style: "ordered", items: ["步驟一", "步驟二"] },
			},
		],
	} as unknown as OutputData);
	assert.equal(saved.blocks.length, 1);
	assert.equal(saved.blocks[0].type, "list");
	const model = JSON.parse(saved.blocks[0].listJson ?? "{}");
	assert.equal(model.ordered, true);
	assert.equal(model.items.length, 2);
	assert.deepEqual(model.items[0].runs, [{ text: "步驟一" }]);
	assert.deepEqual(model.items[1].runs, [{ text: "步驟二" }]);
});

test("fromEditorData maps unordered style to ordered:false", () => {
	const saved = fromEditorData({
		blocks: [
			{
				id: "l1",
				type: "list",
				data: { style: "unordered", items: ["甲"] },
			},
		],
	} as unknown as OutputData);
	const model = JSON.parse(saved.blocks[0].listJson ?? "{}");
	assert.equal(model.ordered, false);
});

test("external paste: nested { content } list items are flattened to items", () => {
	// @editorjs/list v2 (or a pasted nested list) yields items as
	// { content, items } objects — the bridge reads each item's own content.
	const saved = fromEditorData({
		blocks: [
			{
				id: "l1",
				type: "list",
				data: {
					style: "unordered",
					items: [
						{ content: "外部項目一", items: [] },
						{ content: "外部項目二", items: [] },
					],
				},
			},
		],
	} as unknown as OutputData);
	const model = JSON.parse(saved.blocks[0].listJson ?? "{}");
	assert.equal(model.items.length, 2);
	assert.deepEqual(model.items[0].runs, [{ text: "外部項目一" }]);
	assert.deepEqual(model.items[1].runs, [{ text: "外部項目二" }]);
});

test("toEditorData renders a backend list block to Editor.js list data (runs→HTML, href/bold)", () => {
	const desc: ProductDescription = {
		version: "1.0",
		blocks: [
			{
				type: "list",
				id: "l1",
				listJson: JSON.stringify({
					ordered: false,
					items: [
						{ runs: [{ text: "產地直送", bold: true }] },
						{ runs: [{ text: "說明", href: "https://x.io/a" }] },
					],
				}),
			},
		],
	};
	const data = toEditorData(desc);
	assert.equal(data.blocks.length, 1);
	assert.equal(data.blocks[0].type, "list");
	const d = data.blocks[0].data as { style: string; items: string[] };
	assert.equal(d.style, "unordered");
	assert.equal(d.items[0], "<b>產地直送</b>");
	assert.equal(d.items[1], '<a href="https://x.io/a">說明</a>');
});

test("list block round-trips idempotently (save → reload → save)", () => {
	const once = fromEditorData({
		blocks: [
			{
				id: "l1",
				type: "list",
				data: { style: "ordered", items: ["甲", "乙"] },
			},
		],
	} as unknown as OutputData);
	const twice = fromEditorData(toEditorData(once));
	assert.deepEqual(twice.blocks, once.blocks);
});

test("list alignment survives Editor.js tune → listJson → Editor.js round-trip", () => {
	const once = fromEditorData({
		blocks: [
			{
				id: "l1",
				type: "list",
				data: { style: "unordered", items: ["甲", "乙"] },
				tunes: { align: { alignment: "center" } },
			},
		],
	} as unknown as OutputData);
	const model = JSON.parse(once.blocks[0].listJson ?? "{}");
	assert.equal(model.textAlign, "center");
	const restored = toEditorData(once);
	assert.deepEqual(restored.blocks[0].tunes, {
		align: { alignment: "center" },
	});
	assert.deepEqual(fromEditorData(restored), once);
});

test("empty list block is dropped on save", () => {
	const saved = fromEditorData({
		blocks: [{ id: "l1", type: "list", data: { style: "ordered", items: [] } }],
	} as unknown as OutputData);
	assert.equal(saved.blocks.length, 0);
});
