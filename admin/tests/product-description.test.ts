import assert from "node:assert/strict";
import test from "node:test";
import {
	formatCategoryBreadcrumb,
	parseListJson,
	parseTableJson,
} from "../src/lib/product-description";

test("parseTableJson returns model for valid table json", () => {
	const json = JSON.stringify({
		withHeadings: true,
		rows: [[{ runs: [{ text: "欄位1" }] }], [{ runs: [{ text: "值1" }] }]],
	});
	const model = parseTableJson(json);
	assert.equal(model?.withHeadings, true);
	assert.equal(model?.rows.length, 2);
});

test("parseTableJson defaults withHeadings to false when absent", () => {
	const json = JSON.stringify({ rows: [[{ runs: [{ text: "a" }] }]] });
	assert.equal(parseTableJson(json)?.withHeadings, false);
});

test("parseTableJson returns null for malformed json", () => {
	assert.equal(parseTableJson("{not valid json"), null);
});

test("parseTableJson returns null when rows is missing", () => {
	assert.equal(parseTableJson(JSON.stringify({ withHeadings: true })), null);
});

test("parseTableJson returns null for empty rows array", () => {
	assert.equal(parseTableJson(JSON.stringify({ rows: [] })), null);
});

test("parseTableJson returns null for undefined input", () => {
	assert.equal(parseTableJson(undefined), null);
});

test("parseListJson returns model for valid list json", () => {
	const json = JSON.stringify({
		ordered: true,
		items: [{ runs: [{ text: "項目1" }] }],
	});
	const model = parseListJson(json);
	assert.equal(model?.ordered, true);
	assert.equal(model?.items.length, 1);
});

test("parseListJson defaults ordered to false when absent", () => {
	const json = JSON.stringify({ items: [{ runs: [{ text: "項目1" }] }] });
	assert.equal(parseListJson(json)?.ordered, false);
});

test("parseListJson returns null for malformed json", () => {
	assert.equal(parseListJson("not json"), null);
});

test("parseListJson returns null for empty items array", () => {
	assert.equal(parseListJson(JSON.stringify({ items: [] })), null);
});

test("parseListJson returns null for undefined input", () => {
	assert.equal(parseListJson(undefined), null);
});

test("formatCategoryBreadcrumb joins non-empty names with separator", () => {
	assert.equal(
		formatCategoryBreadcrumb(["生鮮", "蔬果", "葉菜類"]),
		"生鮮 > 蔬果 > 葉菜類",
	);
});

test("formatCategoryBreadcrumb skips missing levels", () => {
	assert.equal(
		formatCategoryBreadcrumb(["生鮮", undefined, "葉菜類"]),
		"生鮮 > 葉菜類",
	);
});

test("formatCategoryBreadcrumb returns dash when all levels missing", () => {
	assert.equal(formatCategoryBreadcrumb([undefined, null, ""]), "-");
});
