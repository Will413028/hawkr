import assert from "node:assert/strict";
import { test } from "node:test";
import {
	INDEXABLE_STATIC_ROUTES,
	isIndexable,
	isLocaleNoindex,
	isUgcPath,
	PRIVATE_PREFIXES,
	UGC_PREFIXES,
} from "../lib/seo/indexability";

test("public storefront paths are indexable", () => {
	assert.equal(isIndexable("/"), true);
	assert.equal(isIndexable("/market/products"), true);
	assert.equal(isIndexable("/products/123"), true);
	assert.equal(isIndexable("/seller"), true); // public become-a-seller funnel
	assert.equal(isIndexable("/sellers/9"), true);
});

test("private / thin paths are not indexable", () => {
	assert.equal(isIndexable("/account"), false);
	assert.equal(isIndexable("/account/orders"), false);
	assert.equal(isIndexable("/seller-center"), false);
	assert.equal(isIndexable("/seller-center/dashboard"), false);
	assert.equal(isIndexable("/seller-product-import-preview/job-1"), false);
	assert.equal(isIndexable("/checkout"), false);
	assert.equal(isIndexable("/cart"), false);
	assert.equal(isIndexable("/login"), false);
	assert.equal(isIndexable("/search"), false);
	assert.equal(isIndexable("/orders/abc"), false);
	assert.equal(isIndexable("/payment"), false);
	assert.equal(isIndexable("/register"), false);
	assert.equal(isIndexable("/store-map"), false);
});

test("the public /seller funnel is not mistaken for /seller-center", () => {
	// guards the startsWith boundary: "/seller" must NOT match "/seller-center"
	assert.equal(PRIVATE_PREFIXES.includes("/seller-center"), true);
	assert.equal(isIndexable("/seller"), true);
});

test("every static sitemap route is itself indexable", () => {
	for (const route of INDEXABLE_STATIC_ROUTES) {
		assert.equal(isIndexable(route), true, `${route} should be indexable`);
	}
});

test("UGC pages (products, stores and their tabs, the posts feed) are indexable only in the default locale", () => {
	for (const path of [
		"/products/123",
		"/sellers/9",
		"/sellers/9/posts",
		"/sellers/9/products/5",
		"/feed",
	]) {
		assert.equal(isUgcPath(path), true, path);
		assert.equal(isIndexable(path, "zh-TW"), true, `${path} zh-TW`);
		assert.equal(isIndexable(path, "en"), false, `${path} en`);
		assert.equal(isLocaleNoindex(path, "en"), true, `${path} en noindex`);
		assert.equal(
			isLocaleNoindex(path, "zh-TW"),
			false,
			`${path} zh-TW noindex`,
		);
	}
});

test("platform-written pages stay indexable in en: home, the category listing, FAQ, guides, legal", () => {
	for (const path of [
		"/",
		"/market/products",
		"/faq",
		"/seller",
		"/seller/pricing",
		"/seller-guide",
		"/about",
		"/privacy",
		"/terms",
	]) {
		assert.equal(isUgcPath(path), false, path);
		assert.equal(isIndexable(path, "en"), true, path);
	}
});

test("UGC prefixes match on a segment boundary: /sellers is UGC, /seller-guide and /product-x are not", () => {
	assert.equal(UGC_PREFIXES.includes("/sellers"), true);
	assert.equal(isUgcPath("/seller"), false);
	assert.equal(isUgcPath("/seller-guide"), false);
	assert.equal(isUgcPath("/products-feed"), false);
	assert.equal(isUgcPath("/feedback"), false);
});

test("a private page is not indexable in any locale", () => {
	assert.equal(isIndexable("/account", "en"), false);
	assert.equal(isIndexable("/seller-center/orders", "en"), false);
	// the locale rule is only the part the locale adds
	assert.equal(isLocaleNoindex("/account", "en"), false);
});
