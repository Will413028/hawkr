import assert from "node:assert/strict";
import test from "node:test";
import {
	PRODUCT_SEO_DESCRIPTION_MAX,
	productDescriptionText,
	productSeoDescription,
} from "../lib/seo/product-summary";

/**
 * The four description shapes the wire actually sends (observed on test-api
 * 2026-09-10): runs, legacy `content`, image-only, and no description at all —
 * plus the fact line that stands in when the seller wrote nothing quotable.
 */

const gift = {
	name: "[範例優選] 文旦禮盒",
	origin: "屏東",
	categoryLv1Name: "水果",
	categoryLv2Name: "柚子",
	categoryLv3Name: "文旦",
	categoryName: "文旦",
	seller: { name: "範例優選", address: "基隆市信義區" },
};

test("text blocks with runs are quoted in reading order", () => {
	const description = {
		version: "1.0",
		blocks: [
			{
				type: "text",
				runs: [{ text: "輕鬆閑飲，" }, { text: "一點一滴", bold: true }],
			},
			{ type: "image", imageKey: "public/product_desc/x.png" },
			{ type: "text", runs: [{ text: "孕育好氣色。" }] },
		],
	};
	assert.equal(
		productDescriptionText(description),
		"輕鬆閑飲，一點一滴 孕育好氣色。",
	);
	assert.equal(
		productSeoDescription({ ...gift, description }),
		"輕鬆閑飲，一點一滴 孕育好氣色。",
	);
});

test("legacy content strings and list items count as text too", () => {
	const description = {
		blocks: [
			{ type: "text", content: "產地直送的當季文旦，果肉細緻。" },
			{
				type: "list",
				listJson: JSON.stringify({
					ordered: false,
					items: [{ runs: [{ text: "常溫保存 7 天" }] }, "冷藏更佳"],
				}),
			},
			{ type: "table", tableJson: "{}" },
		],
	};
	assert.equal(
		productDescriptionText(description),
		"產地直送的當季文旦，果肉細緻。 常溫保存 7 天、冷藏更佳",
	);
});

test("an image-only description falls back to the record's own facts", () => {
	// The prod sample: four説明圖, no text. The summary must still say
	// something the title does not — and only what the record states.
	const description = {
		version: "1.0",
		blocks: [
			{ type: "image", imageKey: "public/product_desc/a.png" },
			{ type: "image", imageKey: "public/product_desc/b.png" },
		],
	};
	const summary = productSeoDescription({ ...gift, description });
	assert.equal(
		summary,
		"[範例優選] 文旦禮盒｜產地 屏東｜分類 水果／柚子／文旦｜由 範例優選 供應",
	);
	// Never the seller's address as the product's origin, never invented claims.
	assert.doesNotMatch(summary, /基隆|有機|免運/);
});

test("no description and no facts degrades to the bare name, not to filler", () => {
	assert.equal(productSeoDescription({ name: "高麗菜" }), "高麗菜");
	assert.equal(
		productSeoDescription({ name: "高麗菜", description: null, origin: "  " }),
		"高麗菜",
	);
});

test("a fragment too short to stand alone joins the fact line instead", () => {
	// test-api has products whose whole description is "AAA".
	const summary = productSeoDescription({
		name: "AA",
		origin: "台東",
		seller: { name: "xx" },
		description: { blocks: [{ type: "text", runs: [{ text: "AAA" }] }] },
	});
	assert.equal(summary, "AA｜產地 台東｜由 xx 供應｜AAA");
});

test("a repeated leaf name is not printed twice in the category path", () => {
	const summary = productSeoDescription({
		name: "鳳梨",
		categoryLv1Name: "水果",
		categoryLv2Name: "鳳梨",
		categoryName: "鳳梨",
	});
	assert.equal(summary, "鳳梨｜分類 水果／鳳梨");
});

test("long text is cut at the snippet budget with an ellipsis", () => {
	const long = "好".repeat(400);
	const summary = productSeoDescription({
		name: "x",
		description: { blocks: [{ type: "text", runs: [{ text: long }] }] },
	});
	assert.equal(summary.length, PRODUCT_SEO_DESCRIPTION_MAX);
	assert.ok(summary.endsWith("…"));
});
