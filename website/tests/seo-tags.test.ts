import assert from "node:assert/strict";
import test from "node:test";
import { PUBLIC_CACHE_TAGS, productTag, sellerTag } from "../lib/seo/tags.ts";

test("productTag builds a stable product cache tag", () => {
	assert.equal(productTag("123"), "product:123");
	assert.equal(productTag(456), "product:456");
});

test("sellerTag builds a stable seller cache tag", () => {
	assert.equal(sellerTag("789"), "seller:789");
	assert.equal(sellerTag(1011), "seller:1011");
});

test("public aggregate tags stay stable for backend invalidation", () => {
	assert.deepEqual(PUBLIC_CACHE_TAGS, {
		products: "products:list",
		posts: "posts:list",
		sellers: "sellers:list",
		hashtags: "hashtags:list",
		categories: "categories",
	});
});
