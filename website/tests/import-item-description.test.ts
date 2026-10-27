import assert from "node:assert/strict";
import test from "node:test";
import {
	draftToImportUpdateInput,
	importItemToDraft,
} from "../features/seller-center/products-import/adapters/importItemAdapter";
import type { ImportItemDetail } from "../features/seller-center/products-import/types";

// importItemAdapter 描述往返（issue #1246 姊妹缺陷修復）：
// 匯入編輯頁的 description 讀寫兩方向都必須保留 inline-run 模型
// （text.runs / table.tableJson / list.listJson），早期只收發 legacy content
// 會把格式與表格/清單整批丟掉。backend ItemMutationService 對 descriptionJson
// 原樣存入（無白名單），故 wire 形狀與正式 Product 對齊即可。

const DESC_JSON = JSON.stringify({
	version: "1.0",
	sourceKind: "text",
	blocks: [
		{
			type: "text",
			runs: [{ text: "限量特惠", bold: true, color: "#c0392b" }],
			styleJson: '{"fontSize":"26px","fontWeight":700}',
		},
		{
			type: "table",
			tableJson: '{"withHeadings":true,"rows":[[{"runs":[{"text":"品名"}]}]]}',
		},
		{
			type: "list",
			listJson: '{"ordered":false,"items":[{"runs":[{"text":"項目"}]}]}',
		},
		{ type: "image", url: "https://cdn.example.com/a.jpg" },
	],
});

function makeDetail(overrides?: Partial<ImportItemDetail>): ImportItemDetail {
	return {
		brief: {
			id: "1",
			name: "測試品",
			categoryId: "854",
			version: "3",
		},
		descriptionJson: DESC_JSON,
		skus: [],
		galleryAssets: [],
		descriptionAssets: [],
		errors: [],
		hashtags: [],
		certifications: [],
		origin: "台灣",
		targetStatus: "9201",
		...overrides,
	} as unknown as ImportItemDetail;
}

test("importItemToDraft keeps runs / table / list blocks (read direction)", () => {
	const draft = importItemToDraft(makeDetail());
	const blocks = (
		draft.description as { blocks: Array<Record<string, unknown>> }
	).blocks;
	assert.deepEqual(
		blocks.map((b) => b.type),
		["text", "table", "list", "image"],
	);
	assert.deepEqual(blocks[0].runs, [
		{ text: "限量特惠", bold: true, color: "#c0392b" },
	]);
	// styleJson 反解回 editor 的 style 物件
	assert.deepEqual(blocks[0].style, { fontSize: "26px", fontWeight: 700 });
	assert.equal(
		blocks[1].tableJson,
		'{"withHeadings":true,"rows":[[{"runs":[{"text":"品名"}]}]]}',
	);
	assert.equal(
		blocks[2].listJson,
		'{"ordered":false,"items":[{"runs":[{"text":"項目"}]}]}',
	);
});

test("draftToImportUpdateInput passes runs / table / list through (write direction)", () => {
	const original = makeDetail({
		descriptionJson: '{"version":"1.0","blocks":[]}',
	});
	const draft = importItemToDraft(makeDetail());
	const input = draftToImportUpdateInput(draft, original, {});
	assert.equal(input.descriptionSet, true);
	const desc = JSON.parse(input.description ?? "{}");
	assert.deepEqual(
		desc.blocks.map((b: { type: string }) => b.type),
		["text", "table", "list", "image"],
	);
	assert.deepEqual(desc.blocks[0].runs, [
		{ text: "限量特惠", bold: true, color: "#c0392b" },
	]);
	// style 物件序列化回 styleJson 字串（wire 形狀）
	assert.deepEqual(JSON.parse(desc.blocks[0].styleJson), {
		fontSize: "26px",
		fontWeight: 700,
	});
	assert.equal(
		desc.blocks[1].tableJson,
		'{"withHeadings":true,"rows":[[{"runs":[{"text":"品名"}]}]]}',
	);
	assert.equal(
		desc.blocks[2].listJson,
		'{"ordered":false,"items":[{"runs":[{"text":"項目"}]}]}',
	);
	assert.equal(desc.blocks[3].url, "https://cdn.example.com/a.jpg");
});

test("draftToImportUpdateInput drops text blocks empty in both runs and content", () => {
	const original = makeDetail({
		descriptionJson: '{"version":"1.0","blocks":[]}',
	});
	const draft = importItemToDraft(
		makeDetail({
			descriptionJson: JSON.stringify({
				version: "1.0",
				blocks: [
					{ type: "text", runs: [{ text: "  " }], content: "" },
					{ type: "text", runs: [{ text: "有字" }] },
				],
			}),
		}),
	);
	const input = draftToImportUpdateInput(draft, original, {});
	const desc = JSON.parse(input.description ?? "{}");
	assert.equal(desc.blocks.length, 1);
	assert.deepEqual(desc.blocks[0].runs, [{ text: "有字" }]);
});
