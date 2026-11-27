import assert from "node:assert/strict";
import test from "node:test";
import {
	compactDescription,
	type ProductDescription,
} from "../lib/products/seller-repo";


test("keeps runs-only text blocks (inline-run model)", () => {
	const doc: ProductDescription = {
		version: "1.0",
		blocks: [
			{ type: "text", runs: [{ text: "限量特惠", bold: true }] },
			{ type: "table", tableJson: '{"withHeadings":true,"rows":[]}' },
		],
	};
	const out = compactDescription(doc);
	assert.equal(out.blocks.length, 2);
	assert.equal(out.blocks[0].type, "text");
	assert.deepEqual(out.blocks[0].runs, [{ text: "限量特惠", bold: true }]);
});

test("keeps legacy content-only text blocks", () => {
	const doc: ProductDescription = {
		version: "1.0",
		blocks: [{ type: "text", content: "舊格式文字" }],
	};
	assert.equal(compactDescription(doc).blocks.length, 1);
});

test("drops text blocks that are empty in both runs and content", () => {
	const doc: ProductDescription = {
		version: "1.0",
		blocks: [
			{ type: "text", runs: [{ text: "  " }], content: "" },
			{ type: "text", runs: [], content: "   " },
			{ type: "text" },
		],
	};
	assert.equal(compactDescription(doc).blocks.length, 0);
});

test("table / list / image blocks are unaffected by the text emptiness rule", () => {
	const doc: ProductDescription = {
		version: "1.0",
		blocks: [
			{ type: "table", tableJson: '{"rows":[]}' },
			{ type: "list", listJson: '{"ordered":false,"items":[]}' },
			{ type: "image", imageKey: "img/a.jpg" },
			{ type: "image" },
		],
	};
	const out = compactDescription(doc);
	assert.deepEqual(
		out.blocks.map((b) => b.type),
		["table", "list", "image"],
	);
});
