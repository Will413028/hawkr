import assert from "node:assert/strict";
import { test } from "node:test";
import { staticSitemapEntries } from "../app/sitemap";
import { __setEnvForTest } from "../lib/config/env";
import { __setFlavorForTest } from "../lib/config/flavor";
import { __setPublishedLocalesForTest } from "../lib/intl/locales";
import { INDEXABLE_STATIC_ROUTES, isIndexable } from "../lib/seo/indexability";
import { siteUrl } from "../lib/seo/urls";

// The prod flavor requires its own web origin (NEXT_PUBLIC_WEB_ORIGIN); these cases describe a deployment at this one.
process.env.NEXT_PUBLIC_WEB_ORIGIN = "https://example.com";

function reset() {
	__setEnvForTest(null);
	__setFlavorForTest(null);
}

test("static sitemap covers exactly the indexable static routes", () => {
	__setEnvForTest(null);
	__setFlavorForTest("prod");
	const entries = staticSitemapEntries();
	assert.equal(entries.length, INDEXABLE_STATIC_ROUTES.length);
	reset();
});

test("home entry is absolute and top priority", () => {
	__setEnvForTest(null);
	__setFlavorForTest("prod");
	// absoluteUrl("/") is the bare origin (no trailing slash) so the home entry
	// matches the root canonical Next.js renders — keeps sitemap/canonical aligned.
	const home = staticSitemapEntries().find(
		(e) => e.url === "https://example.com",
	);
	assert.ok(home, "home entry present");
	assert.equal(home?.priority, 1);
	reset();
});

test("static entries carry no lastModified the content does not own", () => {
	// Prod served 13 identical build-time stamps; a date that is not the page's
	// own is noise, so the static entries omit the field altogether.
	__setEnvForTest(null);
	__setFlavorForTest("prod");
	for (const e of staticSitemapEntries()) {
		assert.equal(
			"lastModified" in e,
			false,
			`${e.url} must not carry a build-time lastModified`,
		);
	}
	reset();
});

test("every sitemap URL is itself indexable (purity)", () => {
	__setEnvForTest(null);
	__setFlavorForTest("prod");
	for (const e of staticSitemapEntries()) {
		const path = new URL(e.url).pathname;
		assert.equal(isIndexable(path), true, `${path} must be indexable`);
	}
	reset();
});

test("registry excludes redirect-only aliases (sitemap must be 200-only)", () => {
	const aliases = ["/daily-posts", "/seasonal", "/topics", "/education"];
	for (const alias of aliases) {
		assert.ok(
			!(INDEXABLE_STATIC_ROUTES as readonly string[]).includes(alias),
			`${alias} is a redirect and must not be in the sitemap`,
		);
	}
});

test("registry includes the seller guide (a real 200 page with its own text)", () => {
	assert.ok(
		(INDEXABLE_STATIC_ROUTES as readonly string[]).includes("/seller-guide"),
		"/seller-guide should be in the sitemap",
	);
});

test("registry includes the real legal landing pages", () => {
	const legal = [
		"/privacy",
		"/terms",
		"/return-policy",
		"/user-policy",
		"/seller-terms",
	];
	for (const route of legal) {
		assert.ok(
			(INDEXABLE_STATIC_ROUTES as readonly string[]).includes(route),
			`${route} should be in the sitemap`,
		);
	}
});

const origin = () => siteUrl();

test("with zh-TW published only, the sitemap is exactly the zh-TW list and carries no alternates", () => {
	__setEnvForTest(null);
	__setFlavorForTest("prod");
	for (const e of staticSitemapEntries()) {
		assert.equal(new URL(e.url).pathname.startsWith("/en"), false, e.url);
		assert.equal(
			"alternates" in e,
			false,
			`${e.url} must not carry alternates`,
		);
	}
	reset();
});

test("once en is published for the buyer surface, its platform pages enter the sitemap with hreflang and UGC pages do not", () => {
	__setEnvForTest(null);
	__setFlavorForTest("prod");
	__setPublishedLocalesForTest({ buyer: ["zh-TW", "en"], seller: ["zh-TW"] });
	try {
		const entries = staticSitemapEntries();
		const urls = entries.map((e) => e.url);
		assert.ok(urls.includes(`${origin()}/en`), "en home");
		assert.ok(urls.includes(`${origin()}/en/faq`), "en faq");
		// UGC: the zh-TW entry stays, the en entry is left out (noindex there)
		assert.ok(urls.includes(`${origin()}/feed`));
		assert.equal(urls.includes(`${origin()}/en/feed`), false);
		// the seller surface has not published en
		assert.equal(urls.includes(`${origin()}/en/seller`), false);
		assert.equal(urls.includes(`${origin()}/en/seller/pricing`), false);
		const faq = entries.find((e) => e.url === `${origin()}/en/faq`);
		assert.deepEqual(faq?.alternates?.languages, {
			"zh-TW": `${origin()}/faq`,
			en: `${origin()}/en/faq`,
			"x-default": `${origin()}/faq`,
		});
		// every listed URL is itself indexable in its own locale (purity)
		for (const e of entries) {
			const { pathname } = new URL(e.url);
			const isEn = pathname === "/en" || pathname.startsWith("/en/");
			const path = isEn ? pathname.slice(3) || "/" : pathname;
			assert.equal(isIndexable(path, isEn ? "en" : "zh-TW"), true, e.url);
		}
	} finally {
		__setPublishedLocalesForTest(null);
		reset();
	}
});
