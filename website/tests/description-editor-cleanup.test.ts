import assert from "node:assert/strict";
import test from "node:test";
import type { OutputData } from "@editorjs/editorjs";
import {
	cleanupDescriptionEditorData,
	parseDescriptionMarkdownPaste,
} from "../lib/products/descriptionEditorCleanup";

test("explicit cleanup fixes screenshot headings, repeated bullets and empty items without mutating the source", () => {
	const source: OutputData = {
		time: 123,
		version: "2.31.6",
		blocks: [
			{
				id: "heading",
				type: "header",
				data: { text: "## 商品資訊", level: 2 },
			},
			{
				id: "list",
				type: "list",
				data: {
					style: "unordered",
					items: [
						"- 標稱規格合計：445g（265g＋180g）；各有±5%重量允差",
						"• - 保存條件：冷凍−18°C",
						"",
						" <br>&nbsp; ",
					],
				},
			},
		],
	};
	const original = JSON.stringify(source);
	const result = cleanupDescriptionEditorData(source);
	assert.equal(result.changed, true);
	assert.equal(result.changedBlocks, 2);
	assert.equal(result.data.time, 123);
	assert.equal(result.data.version, "2.31.6");
	assert.equal(result.data.blocks[0].data.text, "商品資訊");
	assert.deepEqual(result.data.blocks[1].data.items, [
		"標稱規格合計：445g（265g＋180g）；各有±5%重量允差",
		"保存條件：冷凍−18°C",
	]);
	assert.equal(JSON.stringify(source), original);
	assert.deepEqual(cleanupDescriptionEditorData(result.data), {
		data: result.data,
		changed: false,
		changedBlocks: 0,
	});
});

test("prefixes spanning inline tags/entities retain mixed styles, links and attributes exactly", () => {
	const source: OutputData = {
		blocks: [
			{
				id: "h",
				type: "header",
				data: {
					level: 3,
					text: '<span style="color:#db8b0e">#</span><b>#&nbsp;商品</b>資訊',
				},
			},
			{
				id: "l",
				type: "list",
				data: {
					style: "unordered",
					items: [
						'<b>-</b>&nbsp;<a href="https://example.com/?x=1&amp;y=2" title="a > b"><i>澳洲</i>牛肉</a> &amp; <u>規格</u>',
						'<span style="color:#334c0f">&#8226; - 重量</span>',
					],
				},
			},
		],
	};
	const result = cleanupDescriptionEditorData(source).data;
	assert.equal(
		result.blocks[0].data.text,
		'<span style="color:#db8b0e"></span><b>商品</b>資訊',
	);
	assert.equal(result.blocks[0].data.level, 3);
	assert.equal(
		result.blocks[1].data.items[0],
		'<b></b><a href="https://example.com/?x=1&amp;y=2" title="a > b"><i>澳洲</i>牛肉</a> &amp; <u>規格</u>',
	);
	assert.equal(
		result.blocks[1].data.items[1],
		'<span style="color:#334c0f">重量</span>',
	);
});

test("negative measurements, positive signs, numbering and punctuation remain content", () => {
	const items = [
		"-18°C",
		"- 18°C",
		"+ 5%",
		"−18°C",
		"1. 產品代碼",
		"#商品",
		"C++",
		"-",
		"5 - 10 天",
	];
	const source: OutputData = {
		blocks: [{ type: "list", data: { style: "ordered", items } }],
	};
	assert.deepEqual(cleanupDescriptionEditorData(source), {
		data: source,
		changed: false,
		changedBlocks: 0,
	});
});

test("nested list order, parent structure, item metadata and unknown payloads are preserved", () => {
	const source: OutputData = {
		blocks: [
			{
				id: "nested",
				type: "list",
				tunes: { align: { alignment: "right" } },
				data: {
					style: "unordered",
					meta: { note: "keep" },
					items: [
						{
							content: "- 主項",
							items: [
								{ content: "• 子項甲", items: [] },
								{ content: "子項乙", items: [] },
							],
							meta: { checked: true },
						},
						{ content: "", items: [{ content: "- 有內容的子項", items: [] }] },
						{ content: "&nbsp;", items: [] },
						{ content: "", items: [], meta: { checked: false } },
						{ unexpected: "keep" },
						null,
					],
				},
			},
		],
	};
	const block = cleanupDescriptionEditorData(source).data.blocks[0];
	assert.equal(block.id, "nested");
	assert.deepEqual(block.tunes, source.blocks[0].tunes);
	assert.deepEqual(block.data.meta, { note: "keep" });
	assert.deepEqual(block.data.items, [
		{
			content: "主項",
			items: [
				{ content: "子項甲", items: [] },
				{ content: "子項乙", items: [] },
			],
			meta: { checked: true },
		},
		{ content: "", items: [{ content: "有內容的子項", items: [] }] },
		{ content: "", items: [], meta: { checked: false } },
		{ unexpected: "keep" },
		null,
	]);
});

test("only truly empty lists are removed; inline media or unknown entities survive", () => {
	const result = cleanupDescriptionEditorData({
		blocks: [
			{
				id: "empty",
				type: "list",
				data: { items: ["", "<b>&nbsp;</b>", "&#160;"] },
			},
			{
				id: "media",
				type: "list",
				data: { items: ['<img src="asset">', "&unknown;", "<code></code>"] },
			},
		],
	});
	assert.deepEqual(
		result.data.blocks.map((block) => block.id),
		["media"],
	);
	assert.deepEqual(result.data.blocks[0].data.items, [
		'<img src="asset">',
		"&unknown;",
		"<code></code>",
	]);
});

test("image/table/unknown blocks retain payloads and order around cleaned text", () => {
	const image = {
		id: "image",
		type: "image",
		data: {
			file: { url: "https://cdn.example/a.jpg", stagedToken: "staged-demo" },
			caption: "## literal",
		},
		tunes: { custom: true },
	};
	const table = {
		id: "table",
		type: "table",
		data: { withHeadings: true, content: [["- literal", "## literal"]] },
	};
	const unknown = {
		id: "future",
		type: "future",
		data: { text: "## leave alone", values: [3, 2, 1] },
	};
	const result = cleanupDescriptionEditorData({
		blocks: [
			image,
			{ id: "text", type: "paragraph", data: { text: "## 商品" } },
			table,
			unknown,
		],
	});
	assert.deepEqual(
		result.data.blocks.map((block) => block.id),
		["image", "text", "table", "future"],
	);
	assert.strictEqual(result.data.blocks[0], image);
	assert.strictEqual(result.data.blocks[2], table);
	assert.strictEqual(result.data.blocks[3], unknown);
});

test("multiline markdown preserves block order, inline spans, IDs, tunes and extension data", () => {
	const tunes = { align: { alignment: "center" }, custom: { value: 7 } };
	const source: OutputData = {
		blocks: [
			{
				id: "a",
				type: "paragraph",
				tunes,
				data: {
					custom: "keep",
					text: '<span style="color:#334c0f">## 商品資訊<br>- <b>澳洲</b>牛肉<br>- <a href="https://example.com">來源</a><br><br>### 保存方式<br>冷凍 -18°C</span>',
				},
			},
			{ id: "a-cleanup-1", type: "paragraph", data: { text: "後文" } },
		],
	};
	const result = cleanupDescriptionEditorData(source);
	assert.equal(result.changedBlocks, 1);
	assert.deepEqual(
		result.data.blocks.map((block) => block.type),
		["header", "list", "header", "paragraph", "paragraph"],
	);
	assert.equal(new Set(result.data.blocks.map((block) => block.id)).size, 5);
	assert.equal(result.data.blocks[0].id, "a");
	assert.equal(result.data.blocks[4].id, "a-cleanup-1");
	assert.equal(
		result.data.blocks[0].data.text,
		'<span style="color:#334c0f">商品資訊</span>',
	);
	assert.deepEqual(result.data.blocks[1].data.items, [
		'<span style="color:#334c0f"><b>澳洲</b>牛肉</span>',
		'<span style="color:#334c0f"><a href="https://example.com">來源</a></span>',
	]);
	assert.equal(result.data.blocks[2].data.level, 3);
	assert.equal(
		result.data.blocks[3].data.text,
		'<span style="color:#334c0f">冷凍 -18°C</span>',
	);
	for (const block of result.data.blocks.slice(0, 4)) {
		assert.deepEqual(block.tunes, tunes);
		assert.equal(block.data.custom, "keep");
	}
	assert.equal(cleanupDescriptionEditorData(result.data).changed, false);
});

test("repeated markup is cleaned in one action and is idempotent", () => {
	const result = cleanupDescriptionEditorData({
		blocks: [
			{ type: "paragraph", data: { text: "## ## 商品\n- - • 牛肉\n- 蔬菜" } },
		],
	});
	assert.equal(result.data.blocks[0].data.text, "商品");
	assert.deepEqual(result.data.blocks[1].data.items, ["牛肉", "蔬菜"]);
	assert.equal(cleanupDescriptionEditorData(result.data).changed, false);
	const blank = cleanupDescriptionEditorData({
		blocks: [
			{ type: "paragraph", data: { text: "## 商品\n- \n- <b>&nbsp;</b>" } },
		],
	});
	assert.deepEqual(blank.data.blocks, [
		{ type: "header", data: { text: "商品", level: 2 } },
	]);
	assert.equal(cleanupDescriptionEditorData(blank.data).changed, false);
});

test("empty Markdown list items are removed in the same cleanup action", () => {
	const result = cleanupDescriptionEditorData({
		blocks: [
			{
				type: "paragraph",
				data: { text: "## 商品\n- \n- 牛肉\n- <b>&nbsp;</b>\n- 蔬菜" },
			},
		],
	});
	assert.deepEqual(result.data.blocks[1].data.items, ["牛肉", "蔬菜"]);
	assert.equal(cleanupDescriptionEditorData(result.data).changed, false);
});

test("normal prose and malformed/unsupported HTML are never reserialized", () => {
	for (const text of [
		"普通段落\n第二行",
		"<b>普通</b><br><br>段落",
		"<code>## 範例</code>",
		"<b>## 未閉合",
		"<div>## 商品</div>",
		"## ",
	]) {
		const source: OutputData = {
			blocks: [{ type: "paragraph", data: { text } }],
		};
		assert.strictEqual(cleanupDescriptionEditorData(source).data, source);
	}
});

test("plain Markdown paste converts headings and lists while escaping HTML as text", () => {
	assert.deepEqual(
		parseDescriptionMarkdownPaste(
			"## 商品\r\n\r\n- 牛肉 & 蔬菜\r\n- <img src=x onerror=alert(1)>\r\n\r\n### 保存\r\n冷凍 -18°C",
		),
		[
			{ type: "header", data: { text: "商品", level: 2 } },
			{
				type: "list",
				data: {
					style: "unordered",
					items: ["牛肉 &amp; 蔬菜", "&lt;img src=x onerror=alert(1)&gt;"],
				},
			},
			{ type: "header", data: { text: "保存", level: 3 } },
			{ type: "paragraph", data: { text: "冷凍 -18°C" } },
		],
	);
});

test("ordered paste converts a sequential list from 1 and preserves subsequent source numbers", () => {
	assert.deepEqual(
		parseDescriptionMarkdownPaste(
			"1. 第一步\n2. 第二步\n5. 原始編號\n6. 保留此號碼",
		),
		[
			{ type: "list", data: { style: "ordered", items: ["第一步", "第二步"] } },
			{ type: "paragraph", data: { text: "5. 原始編號<br>6. 保留此號碼" } },
		],
	);
	assert.equal(parseDescriptionMarkdownPaste("3. 第三步\n4. 第四步"), null);
});

test("paste declines prose, lone bullets, negative values, nested lists and code fences", () => {
	for (const text of [
		"一般文案",
		"- 單一行",
		"-18°C\n-20°C",
		"- 18°C\n+ 5%",
		"1. 編號",
		"- 主項\n  - 子項",
		"## 範例\n```\n- 不是清單\n```",
		"## 範例\n    code",
		"普通\n第二行",
	]) {
		assert.equal(parseDescriptionMarkdownPaste(text), null, text);
	}
});
