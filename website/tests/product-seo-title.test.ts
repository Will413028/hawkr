import assert from "node:assert/strict";
import test from "node:test";
import { productSeoTitle, SITE_NAME } from "../lib/seo/metadata";

/**
 * The product <title> rule, asserted against literal expected strings — the
 * same reason tests/store-seo-summary.test.ts exists. The AST guard in
 * tests/seo-title-brand-suffix.test.ts stands in for runtime data with a
 * placeholder, so it could never see the one data case that shipped: a seller
 * whose name IS the brand.
 */

test("a third-party seller gets product, seller, brand — once each", () => {
	assert.equal(
		productSeoTitle(
			{ name: "東河文旦禮盒", sellerName: "範例四季食品有限公司" },
			"zh-TW",
		),
		"東河文旦禮盒 | 範例四季食品有限公司 | Hawkr",
	);
});

test("the platform's own store does not repeat the brand", () => {
	// Regression pin: prod served `「範例」Hawkr x 東河文旦 | Hawkr | Hawkr`
	// because the seller segment and the brand segment were the same string.
	const title = productSeoTitle(
		{
			name: "「範例」Hawkr x 東河文旦",
			sellerName: "Hawkr",
		},
		"zh-TW",
	);
	assert.equal(title, "「範例」Hawkr x 東河文旦 | Hawkr");
	assert.equal(
		title.split(` | ${SITE_NAME}`).length - 1,
		1,
		`brand suffix must appear once: ${title}`,
	);
});

test("a product whose name already carries the brand still keeps its own name", () => {
	// Only an ADJACENT duplicate collapses; the name is the seller's wording.
	assert.equal(
		productSeoTitle({ name: "Hawkr", sellerName: "Hawkr" }, "zh-TW"),
		"Hawkr",
	);
	assert.equal(
		productSeoTitle({ name: "Hawkr x 東河文旦", sellerName: "小農" }, "zh-TW"),
		"Hawkr x 東河文旦 | 小農 | Hawkr",
	);
});

test("a missing or blank seller drops its segment instead of a placeholder", () => {
	assert.equal(productSeoTitle({ name: "文旦" }, "zh-TW"), "文旦 | Hawkr");
	assert.equal(
		productSeoTitle({ name: "文旦", sellerName: "   " }, "zh-TW"),
		"文旦 | Hawkr",
	);
	assert.equal(
		productSeoTitle({ name: "  文旦  ", sellerName: null }, "zh-TW"),
		"文旦 | Hawkr",
	);
});

test("no name at all still yields a branded title, never a naked separator", () => {
	assert.equal(productSeoTitle({}, "zh-TW"), "Hawkr");
	assert.equal(
		productSeoTitle({ sellerName: "小農" }, "zh-TW"),
		"小農 | Hawkr",
	);
});

test("the English site names itself Hawkr: the suffix follows the page's locale", () => {
	assert.equal(
		productSeoTitle(
			{ name: "東河文旦禮盒", sellerName: "範例四季食品有限公司" },
			"en",
		),
		"東河文旦禮盒 | 範例四季食品有限公司 | Hawkr",
	);
	assert.equal(productSeoTitle({ name: "文旦" }, "en"), "文旦 | Hawkr");
	assert.equal(productSeoTitle({}, "en"), "Hawkr");
});
