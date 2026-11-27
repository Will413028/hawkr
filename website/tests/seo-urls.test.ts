import assert from "node:assert/strict";
import { test } from "node:test";
import { __setEnvForTest } from "../lib/config/env";
import { __setFlavorForTest } from "../lib/config/flavor";
import {
	absoluteUrl,
	languageAlternates,
	productUrl,
	sellerCanonicalUrl,
	sellerCardPath,
	sellerCardUrl,
	sellerPath,
	sellerUrl,
	siteUrl,
} from "../lib/seo/urls";

process.env.NEXT_PUBLIC_API_BASE_URL = "/api/v1";

process.env.NEXT_PUBLIC_WEB_ORIGIN = "https://example.com";

function reset() {
	__setEnvForTest(null);
	__setFlavorForTest(null);
}

test("siteUrl returns the prod web origin with no trailing slash", () => {
	__setEnvForTest(null);
	__setFlavorForTest("prod");
	assert.equal(siteUrl(), "https://example.com");
	reset();
});

test("siteUrl follows NEXT_PUBLIC_WEB_ORIGIN and dev falls back to the local dev server", () => {
	const configured = process.env.NEXT_PUBLIC_WEB_ORIGIN;
	try {
		process.env.NEXT_PUBLIC_WEB_ORIGIN = "https://staging.example.org/";
		__setEnvForTest(null);
		__setFlavorForTest("dev");
		assert.equal(siteUrl(), "https://staging.example.org");

		delete process.env.NEXT_PUBLIC_WEB_ORIGIN;
		__setEnvForTest(null);
		assert.equal(siteUrl(), "http://localhost:3000");
	} finally {
		process.env.NEXT_PUBLIC_WEB_ORIGIN = configured;
		reset();
	}
});

test("entity builders compose absolute canonical URLs", () => {
	__setEnvForTest(null);
	__setFlavorForTest("prod");
	assert.equal(absoluteUrl("/faq", "zh-TW"), "https://example.com/faq");
	assert.equal(productUrl("123", "zh-TW"), "https://example.com/products/123");
	assert.equal(sellerUrl("9", "zh-TW"), "https://example.com/sellers/9");
	reset();
});

test("seller card URLs keep the compact public page on the canonical seller key", () => {
	__setEnvForTest(null);
	__setFlavorForTest("prod");
	assert.equal(
		sellerCardUrl("2685914682", "zh-TW"),
		"https://example.com/sellers/2685914682/card",
	);
	assert.equal(sellerCardPath("2685914682"), "/sellers/2685914682/card");
	assert.equal(sellerCardPath(undefined), "");
	reset();
});

test("absoluteUrl rejects a path missing its leading slash", () => {
	assert.throws(() => absoluteUrl("faq", "zh-TW"), /must start with/);
});

const origin = () => siteUrl();

test("every URL builder takes the locale of the page it describes", () => {
	__setEnvForTest(null);
	__setFlavorForTest("prod");
	assert.equal(absoluteUrl("/faq", "zh-TW"), `${origin()}/faq`);
	assert.equal(absoluteUrl("/faq", "en"), `${origin()}/en/faq`);
	assert.equal(absoluteUrl("/", "zh-TW"), origin());
	assert.equal(absoluteUrl("/", "en"), `${origin()}/en`);
	assert.equal(productUrl("123", "en"), `${origin()}/en/products/123`);
	assert.equal(
		sellerUrl("9", "en", "/posts"),
		`${origin()}/en/sellers/9/posts`,
	);
	assert.equal(sellerCardUrl("9", "en"), `${origin()}/en/sellers/9/card`);
	assert.equal(
		sellerCanonicalUrl(
			{ storeCode: "2600123456" },
			"1234567890123456789",
			"en",
			"/reviews",
		),
		`${origin()}/en/sellers/2600123456/reviews`,
	);
	assert.equal(sellerPath("9"), "/sellers/9");
	assert.equal(sellerPath("9", "/posts"), "/sellers/9/posts");
	assert.equal(sellerCardPath("9"), "/sellers/9/card");
	reset();
});

test("hreflang lists the published locales of the page's surface, with x-default on the unprefixed URL", () => {
	__setEnvForTest(null);
	__setFlavorForTest("prod");
	const both = ["zh-TW", "en"] as const;
	assert.deepEqual(languageAlternates("/faq", both), {
		"zh-TW": `${origin()}/faq`,
		en: `${origin()}/en/faq`,
		"x-default": `${origin()}/faq`,
	});
	assert.deepEqual(languageAlternates("/", both), {
		"zh-TW": origin(),
		en: `${origin()}/en`,
		"x-default": origin(),
	});
	reset();
});

test("hreflang: nothing while one locale is published, nothing for private or UGC pages", () => {
	__setEnvForTest(null);
	__setFlavorForTest("prod");
	assert.equal(languageAlternates("/faq"), undefined);
	assert.equal(languageAlternates("/seller/pricing"), undefined);
	const both = ["zh-TW", "en"] as const;
	assert.equal(languageAlternates("/account/orders", both), undefined);
	assert.equal(languageAlternates("/products/1", both), undefined);
	assert.equal(languageAlternates("/sellers/9/posts", both), undefined);
	assert.equal(languageAlternates("/feed", both), undefined);
	reset();
});
