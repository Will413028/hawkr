import assert from "node:assert/strict";
import test from "node:test";
import { JSDOM } from "jsdom";
import {
	fromEditorData,
	toEditorData,
} from "../lib/products/descriptionEditorJs";

function withDocument(run: () => void) {
	const previous = globalThis.document;
	const dom = new JSDOM("<!doctype html>");
	globalThis.document = dom.window.document;
	try {
		run();
	} finally {
		globalThis.document = previous;
		dom.window.close();
	}
}

test("CSS formatting after color commands survives save and reload", () =>
	withDocument(() => {
		const saved = fromEditorData({
			blocks: [
				{
					id: "css",
					type: "paragraph",
					data: {
						text: '<span style="color:rgb(86, 58, 114);font-weight:bold;font-style:italic;text-decoration:underline">保留文字格式</span>',
					},
				},
			],
		});
		assert.deepEqual(saved.blocks[0].runs, [
			{
				text: "保留文字格式",
				bold: true,
				italic: true,
				underline: true,
				color: "#563a72",
			},
		]);
		assert.deepEqual(fromEditorData(toEditorData(saved)), saved);
	}));

test("CSS normal overrides inherited inline emphasis without losing links or text", () =>
	withDocument(() => {
		const saved = fromEditorData({
			blocks: [
				{
					id: "reset",
					type: "paragraph",
					data: {
						text: '<b><i><u>加重<span style="font-weight:400;font-style:normal;text-decoration:none"><a href="https://example.com/details">恢復內文</a></span>加重</u></i></b>',
					},
				},
			],
		});
		assert.deepEqual(saved.blocks[0].runs, [
			{ text: "加重", bold: true, italic: true, underline: true },
			{ text: "恢復內文", href: "https://example.com/details" },
			{ text: "加重", bold: true, italic: true, underline: true },
		]);
	}));

test("formatting after color has stable history signatures across undo rendering", () =>
	withDocument(() => {
		const variants = [
			'<span style="color:#682e0e"><b>保留文字格式</b></span>',
			'<span style="color:#682e0e"><b><i>保留文字格式</i></b></span>',
			'<span style="color:#682e0e"><b><i><u>保留文字格式</u></i></b></span>',
			'<u><a href="https://example.com/details"><span style="color:#682e0e"><i><b>保留文字格式</b></i></span></a></u>',
		];
		for (const html of variants) {
			const saved = fromEditorData({
				blocks: [{ id: "history", type: "paragraph", data: { text: html } }],
			});
			const restored = fromEditorData(toEditorData(saved));
			assert.equal(JSON.stringify(restored), JSON.stringify(saved));
		}
	}));

test("list and table inline formatting also keep stable serialized history", () =>
	withDocument(() => {
		const html = '<span style="color:#682e0e"><b><u>清單與表格</u></b></span>';
		const saved = fromEditorData({
			blocks: [
				{ id: "list", type: "list", data: { style: "ordered", items: [html] } },
				{
					id: "table",
					type: "table",
					data: { withHeadings: false, content: [[html]] },
				},
			],
		});
		const restored = fromEditorData(toEditorData(saved));
		assert.equal(JSON.stringify(restored), JSON.stringify(saved));
	}));

test("native empty paragraph placeholders do not create an extra deletion checkpoint", () =>
	withDocument(() => {
		const canonical = (text: string, type: "paragraph" | "header") =>
			fromEditorData({
				blocks: [
					{ id: "deleted", type, data: { text, level: 2 } },
					{
						id: "preserved",
						type: "paragraph",
						data: { text: "前後空白仍保留 " },
					},
				],
			});
		for (const type of ["paragraph", "header"] as const) {
			const empty = canonical("", type);
			for (const placeholder of ["<br>", "<br><br>", "&nbsp;", "<b> </b>"])
				assert.equal(
					JSON.stringify(canonical(placeholder, type)),
					JSON.stringify(empty),
				);
			assert.deepEqual(empty.blocks[0].runs, [{ text: "前後空白仍保留 " }]);
		}
	}));

test("cleared list items and table cells share one empty inline representation", () =>
	withDocument(() => {
		const canonical = (html: string) =>
			fromEditorData({
				blocks: [
					{
						id: "list",
						type: "list",
						data: { style: "unordered", items: [html] },
					},
					{
						id: "table",
						type: "table",
						data: { withHeadings: false, content: [[html, " 保留內容 "]] },
					},
				],
			});
		const empty = canonical("");
		for (const placeholder of ["<br>", "&nbsp;", "<b> </b>"])
			assert.equal(
				JSON.stringify(canonical(placeholder)),
				JSON.stringify(empty),
			);
		assert.deepEqual(JSON.parse(empty.blocks[1].tableJson ?? "{}").rows[0], [
			{ runs: [] },
			{ runs: [{ text: " 保留內容 " }] },
		]);
	}));
