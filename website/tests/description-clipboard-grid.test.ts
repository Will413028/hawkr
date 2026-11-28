import assert from "node:assert/strict";
import test from "node:test";
import {
	clampGrid,
	normalizeGrid,
	parseClipboardGrid,
	TABLE_MAX_CELL_LEN,
	TABLE_MAX_COLS,
	TABLE_MAX_ROWS,
	tsvToGrid,
} from "../lib/products/clipboardGrid";

test("tsvToGrid parses 2x2 tab-separated text", () => {
	assert.deepEqual(tsvToGrid("品名\t產地\n愛文\t台南"), [
		["品名", "產地"],
		["愛文", "台南"],
	]);
});

test("tsvToGrid rejects single-line text with tabs (prose guard)", () => {
	assert.equal(tsvToGrid("品名\t產地"), null);
});

test("tsvToGrid rejects multi-line text without tabs", () => {
	assert.equal(tsvToGrid("第一行\n第二行"), null);
});

test("tsvToGrid rejects when only some lines contain tabs", () => {
	assert.equal(tsvToGrid("品名\t產地\n這行是散文沒有分隔"), null);
});

test("tsvToGrid handles CRLF and trailing blank lines", () => {
	assert.deepEqual(tsvToGrid("甲\t乙\r\n丙\t丁\r\n\r\n"), [
		["甲", "乙"],
		["丙", "丁"],
	]);
});

test("tsvToGrid escapes HTML entities in cell text", () => {
	assert.deepEqual(tsvToGrid("a<b\tc&d\ne\tf"), [
		["a&lt;b", "c&amp;d"],
		["e", "f"],
	]);
});

test("parseClipboardGrid falls back to TSV when html has no table / no DOM", () => {
	assert.deepEqual(parseClipboardGrid("<p>hi</p>", "甲\t乙\n丙\t丁"), [
		["甲", "乙"],
		["丙", "丁"],
	]);
	assert.equal(parseClipboardGrid(null, "沒有表格的文字"), null);
});

test("clampGrid truncates columns beyond the backend limit", () => {
	const wide = [Array.from({ length: TABLE_MAX_COLS + 2 }, (_, i) => `c${i}`)];
	const twoRows = [...wide, ...wide];
	const { grid, clamped } = clampGrid(twoRows);
	assert.equal(clamped, true);
	assert.equal(grid[0].length, TABLE_MAX_COLS);
});

test("clampGrid truncates rows beyond the backend limit", () => {
	const tall = Array.from({ length: TABLE_MAX_ROWS + 5 }, () => ["x"]);
	const { grid, clamped } = clampGrid(tall);
	assert.equal(clamped, true);
	assert.equal(grid.length, TABLE_MAX_ROWS);
});

test("clampGrid truncates over-long cells to plain text at the char limit", () => {
	const long = "字".repeat(TABLE_MAX_CELL_LEN + 10);
	const { grid, clamped } = clampGrid([[`<b>${long}</b>`], ["ok"]]);
	assert.equal(clamped, true);
	assert.equal(grid[0][0].includes("<b>"), false);
	assert.equal(grid[0][0].length, TABLE_MAX_CELL_LEN);
});

test("clampGrid truncation round-trips escaped entities without double-escaping", () => {
	const long = `a&b<c${"x".repeat(TABLE_MAX_CELL_LEN)}`;
	const escaped = long.replace(/&/g, "&amp;").replace(/</g, "&lt;");
	const { grid, clamped } = clampGrid([[escaped], ["ok"]]);
	assert.equal(clamped, true);
	assert.equal(grid[0][0].startsWith("a&amp;b&lt;c"), true);
	assert.equal(grid[0][0].includes("&amp;amp;"), false);
});

test("tsvToGrid/attr path escapes quotes (attribute-context injection guard)", () => {
	const g = tsvToGrid('a"b\tc\nd\te');
	assert.equal(g?.[0][0].includes('"'), true);
	const { grid } = clampGrid([
		['<a href="https://x/?q=&quot;y">k</a>', "z"],
		["a", "b"],
	]);
	assert.equal(grid[0][0].includes("&quot;"), true);
});

test("normalizeGrid pads jagged rows (colspan title row) to max width", () => {
	assert.deepEqual(
		normalizeGrid([["經典原味"], ["成分", "發酵奶油"], ["原產地", "臺灣"]]),
		[
			["經典原味", ""],
			["成分", "發酵奶油"],
			["原產地", "臺灣"],
		],
	);
});

test("normalizeGrid drops all-empty rows (image-only colspan row)", () => {
	assert.deepEqual(
		normalizeGrid([
			["a", "b"],
			["", ""],
			["c", "d"],
		]),
		[
			["a", "b"],
			["c", "d"],
		],
	);
	assert.equal(normalizeGrid([["", ""], [""]]), null);
});

test("clampGrid leaves in-limit grids untouched", () => {
	const g = [
		["<b>粗</b>", "字"],
		["a", "b"],
	];
	const { grid, clamped } = clampGrid(g);
	assert.equal(clamped, false);
	assert.deepEqual(grid, g);
});
