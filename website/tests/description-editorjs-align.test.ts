import assert from "node:assert/strict";
import test from "node:test";
import type { OutputData } from "@editorjs/editorjs";
import {
	fromEditorData,
	TABLE_CELL_ALIGN_ATTRIBUTE,
	toEditorData,
} from "../lib/products/descriptionEditorJs";

// Paragraph alignment travels as the "align" block tune in Editor.js and as
// `style.textAlign` in the backend's shared { version, blocks } shape. The
// bridge must keep the two in sync — and must NOT store the default "left",
// so existing descriptions stay byte-identical after a no-op edit.

test("fromEditorData maps the align tune to style.textAlign", () => {
	const saved = fromEditorData({
		blocks: [
			{
				id: "p1",
				type: "paragraph",
				data: { text: "置中段落" },
				tunes: { align: { alignment: "center" } },
			},
			{
				id: "h1",
				type: "header",
				data: { text: "靠右標題", level: 2 },
				tunes: { align: { alignment: "right" } },
			},
		],
	} as OutputData);
	assert.equal(saved.blocks[0].style?.textAlign, "center");
	// Headers keep their fixed heading style alongside the alignment.
	assert.deepEqual(saved.blocks[1].style, {
		fontSize: "26px",
		fontWeight: 700,
		textAlign: "right",
	});
});

test("fromEditorData drops the default left alignment and junk values", () => {
	const saved = fromEditorData({
		blocks: [
			{
				id: "p1",
				type: "paragraph",
				data: { text: "預設置左" },
				tunes: { align: { alignment: "left" } },
			},
			{
				id: "p2",
				type: "paragraph",
				data: { text: "壞資料" },
				tunes: { align: { alignment: "justify" } },
			},
			{ id: "p3", type: "paragraph", data: { text: "無 tunes" } },
		],
	} as OutputData);
	for (const blk of saved.blocks) {
		assert.equal(blk.style?.textAlign, undefined);
	}
});

test("toEditorData restores style.textAlign as the align tune", () => {
	const data = toEditorData({
		version: "1.0",
		blocks: [
			{
				type: "text",
				id: "p1",
				content: "置中",
				style: { textAlign: "center" },
			},
			{
				type: "text",
				id: "h1",
				content: "靠右標題",
				style: { fontSize: "26px", fontWeight: 700, textAlign: "right" },
			},
			{ type: "text", id: "p2", content: "預設" },
		],
	});
	assert.deepEqual(data.blocks[0].tunes, { align: { alignment: "center" } });
	assert.equal(data.blocks[1].type, "header");
	assert.deepEqual(data.blocks[1].tunes, { align: { alignment: "right" } });
	assert.equal(data.blocks[2].tunes, undefined);
});

test("alignment survives a full save → load → save round-trip", () => {
	const first = fromEditorData({
		blocks: [
			{
				id: "p1",
				type: "paragraph",
				data: { text: "置中段落" },
				tunes: { align: { alignment: "center" } },
			},
		],
	} as OutputData);
	const second = fromEditorData(toEditorData(first));
	assert.deepEqual(second, first);
});

test("table-cell alignment persists through tableJson and Editor.js HTML", () => {
	const saved = fromEditorData({
		blocks: [
			{
				id: "t1",
				type: "table",
				data: {
					withHeadings: false,
					content: [
						[
							`<div ${TABLE_CELL_ALIGN_ATTRIBUTE}="center" style="text-align:center">甲</div>`,
							"乙",
						],
					],
				},
			},
		],
	} as unknown as OutputData);
	const model = JSON.parse(saved.blocks[0].tableJson ?? "{}");
	assert.equal(model.rows[0][0].textAlign, "center");
	assert.equal(model.rows[0][1].textAlign, undefined);

	const restored = toEditorData(saved);
	const content = (restored.blocks[0].data as { content: string[][] }).content;
	assert.match(content[0][0], /data-hk-cell-align="center"/);
	assert.equal(content[0][1], "乙");
	assert.deepEqual(fromEditorData(restored), saved);
});
