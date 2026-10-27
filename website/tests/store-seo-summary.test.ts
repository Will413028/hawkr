import assert from "node:assert/strict";
import test from "node:test";
import { storeSeoDescription, storeSeoTitle } from "../lib/seo/store-summary";

/**
 * The store page's title and description are generated from what the page
 * shows; there is no stored override (ADR 2026-10-09-store-seo-platform-
 * generated). Expected strings are written out in full on purpose: deriving
 * them from the function's own pieces would agree with any regression.
 */

const farm = { name: "範例鮮果", county: "花蓮縣", town: "壽豐鄉" };

test("title is name, location and the brand, each once", () => {
	assert.equal(storeSeoTitle(farm, "zh-TW"), "範例鮮果 | 花蓮縣壽豐鄉 | Hawkr");
});

test("title without a location is the name and the brand", () => {
	assert.equal(
		storeSeoTitle({ name: "範例鮮果" }, "zh-TW"),
		"範例鮮果 | Hawkr",
	);
	assert.equal(
		storeSeoTitle({ name: "範例鮮果", county: "  ", town: "" }, "zh-TW"),
		"範例鮮果 | Hawkr",
	);
});

test("title with only the county or only the town keeps what is there", () => {
	assert.equal(
		storeSeoTitle({ name: "範例鮮果", county: "花蓮縣" }, "zh-TW"),
		"範例鮮果 | 花蓮縣 | Hawkr",
	);
	assert.equal(
		storeSeoTitle({ name: "範例鮮果", town: "壽豐鄉" }, "zh-TW"),
		"範例鮮果 | 壽豐鄉 | Hawkr",
	);
});

test("a store named like the brand is not repeated next to it", () => {
	assert.equal(storeSeoTitle({ name: "Hawkr" }, "zh-TW"), "Hawkr");
});

test("title without a name falls back to the brand alone", () => {
	assert.equal(storeSeoTitle({}, "zh-TW"), "Hawkr");
	assert.equal(storeSeoTitle({ name: null }, "zh-TW"), "Hawkr");
	assert.equal(storeSeoTitle({ name: "   " }, "zh-TW"), "Hawkr");
	assert.equal(storeSeoTitle({ county: "花蓮縣" }, "zh-TW"), "花蓮縣 | Hawkr");
});

test("the English site names itself Hawkr: the suffix follows the page's locale", () => {
	assert.equal(storeSeoTitle(farm, "en"), "範例鮮果 | 花蓮縣壽豐鄉 | Hawkr");
	assert.equal(storeSeoTitle({}, "en"), "Hawkr");
});

test("an intro that stands alone is the description", () => {
	assert.equal(
		storeSeoDescription({
			...farm,
			description: "自家種的有機蔬果，產地直送。",
		}),
		"自家種的有機蔬果，產地直送。",
	);
});

test("an intro is whitespace-collapsed and cut at the product limit", () => {
	const long = `${"鮮".repeat(100)}\n\n${"果".repeat(100)}`;
	const out = storeSeoDescription({ ...farm, description: long });
	assert.equal(out, `${"鮮".repeat(100)} ${"果".repeat(58)}…`);
	assert.equal(out?.length, 160);
	// Exactly at the limit is not cut.
	const exact = "鮮".repeat(160);
	assert.equal(storeSeoDescription({ description: exact }), exact);
});

test("a missing intro is replaced by the facts the record states", () => {
	assert.equal(
		storeSeoDescription({
			...farm,
			ratingAverage: 4.76,
			reviewCount: 12,
		}),
		"範例鮮果｜位於 花蓮縣壽豐鄉｜評分 4.8（12 則評價）",
	);
	assert.equal(
		storeSeoDescription({ ...farm, ratingAverage: 4.5 }),
		"範例鮮果｜位於 花蓮縣壽豐鄉｜評分 4.5",
	);
});

test("the intro is counted and cut in code points, never inside a surrogate pair", () => {
	// 159 CJK + one emoji straddling the cut: the emoji must not be split.
	const emojiAtCut = `${"鮮".repeat(158)}🍎🍎${"果".repeat(20)}`;
	const out = storeSeoDescription({ description: emojiAtCut }) ?? "";
	assert.equal(out, `${"鮮".repeat(158)}🍎…`);
	assert.equal(Array.from(out).length, 160);
	assert.doesNotMatch(out, /[\ud800-\udbff](?![\udc00-\udfff])/);
	// Exactly 160 code points (emoji count once) is not cut.
	const exact = `${"鮮".repeat(150)}${"🍎".repeat(10)}`;
	assert.equal(storeSeoDescription({ description: exact }), exact);
	// The 10-character threshold counts code points: 5 emoji are 10 UTF-16
	// units but only 5 characters, so they are a fragment, not a summary.
	assert.equal(
		storeSeoDescription({ name: "小店", description: "🍎🍎🍎🍎🍎" }),
		"小店｜🍎🍎🍎🍎🍎",
	);
	assert.equal(
		storeSeoDescription({ description: "🍎🍎🍎🍎🍎🍎🍎🍎🍎🍎" }),
		"🍎🍎🍎🍎🍎🍎🍎🍎🍎🍎",
	);
});

test("an empty or whitespace intro is absent, not a value", () => {
	for (const description of [undefined, null, "", "   \n "]) {
		assert.equal(
			storeSeoDescription({ ...farm, description }),
			"範例鮮果｜位於 花蓮縣壽豐鄉",
		);
	}
});

test("a fragment of an intro is kept behind the facts", () => {
	assert.equal(
		storeSeoDescription({ ...farm, description: "小農直送" }),
		"範例鮮果｜位於 花蓮縣壽豐鄉｜小農直送",
	);
});

test("no rating means no rating fact, never a zero", () => {
	for (const ratingAverage of [undefined, null, 0, "0", "abc"]) {
		assert.equal(
			storeSeoDescription({ ...farm, ratingAverage, reviewCount: 3 }),
			"範例鮮果｜位於 花蓮縣壽豐鄉",
		);
	}
});

test("a store with nothing but a name says its name; nothing at all says nothing", () => {
	assert.equal(storeSeoDescription({ name: "範例鮮果" }), "範例鮮果");
	assert.equal(storeSeoDescription({}), undefined);
	assert.equal(storeSeoDescription({ name: " ", description: "" }), undefined);
});
