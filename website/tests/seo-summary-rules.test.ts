import assert from "node:assert/strict";
import test from "node:test";
import { productSeoDescription } from "../lib/seo/product-summary";
import {
	codePointLength,
	joinTitleSegments,
	SEO_DESCRIPTION_MAX,
	truncateSummary,
} from "../lib/seo/summary-rules";

test("truncation counts code points and never splits a surrogate pair", () => {
	assert.equal(SEO_DESCRIPTION_MAX, 160);
	assert.equal(codePointLength("🍎鮮"), 2);
	const text = `${"鮮".repeat(158)}🍎🍎🍎`;
	assert.equal(truncateSummary(text), `${"鮮".repeat(158)}🍎…`);
	assert.equal(truncateSummary("🍎🍎🍎🍎", 3), "🍎🍎…");
	assert.equal(truncateSummary("  短文  "), "短文");
});

test("the product description is cut by the same rule", () => {
	const description = {
		blocks: [{ type: "text", content: `${"鮮".repeat(158)}🍎🍎🍎` }],
	};
	assert.equal(
		productSeoDescription({ name: "文旦", description }),
		`${"鮮".repeat(158)}🍎…`,
	);
});

test("title segments: empty dropped, equal neighbours said once", () => {
	assert.equal(joinTitleSegments(["a", "b", "c"]), "a | b | c");
	assert.equal(joinTitleSegments(["a", " ", null, undefined, "c"]), "a | c");
	assert.equal(joinTitleSegments(["x", "x", "y"]), "x | y");
	assert.equal(joinTitleSegments([]), "");
});
