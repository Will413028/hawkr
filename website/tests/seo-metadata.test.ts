import assert from "node:assert/strict";
import { test } from "node:test";
import { __setEnvForTest, getEnv } from "../lib/config/env";
import { __setFlavorForTest } from "../lib/config/flavor";
import { __setPublishedLocalesForTest } from "../lib/intl/locales";
import {
	buildPageMetadata,
	buildRootMetadata,
	defaultTitleFor,
	noindexMetadata,
	siteNameFor,
} from "../lib/seo/metadata";

// The prod flavor requires its own web origin (NEXT_PUBLIC_WEB_ORIGIN); these cases describe a deployment at this one.
process.env.NEXT_PUBLIC_WEB_ORIGIN = "https://example.com";

function reset() {
	__setEnvForTest(null);
	__setFlavorForTest(null);
	delete process.env.NEXT_PUBLIC_SEO_INDEXABLE;
}

test("root metadata sets an absolute metadataBase for the active flavor", () => {
	__setFlavorForTest("prod");
	__setEnvForTest(null);
	const md = buildRootMetadata("zh-TW");
	assert.equal(md.metadataBase?.origin, "https://example.com");
	reset();
});

test("prod WITH the launch flag is indexable", () => {
	process.env.NEXT_PUBLIC_SEO_INDEXABLE = "true";
	__setFlavorForTest("prod");
	__setEnvForTest(null);
	assert.deepEqual(buildRootMetadata("zh-TW").robots, {
		index: true,
		follow: true,
	});
	reset();
});

test("prod BEFORE launch (flag off) is not indexable", () => {
	__setFlavorForTest("prod");
	__setEnvForTest(null);
	assert.deepEqual(buildRootMetadata("zh-TW").robots, {
		index: false,
		follow: false,
	});
	reset();
});

test("non-prod is never indexable even with the flag on", () => {
	process.env.NEXT_PUBLIC_SEO_INDEXABLE = "true";
	__setFlavorForTest("dev");
	__setEnvForTest(null);
	assert.deepEqual(buildRootMetadata("zh-TW").robots, {
		index: false,
		follow: false,
	});
	reset();
});

test("root metadata has a title template and default OG", () => {
	__setFlavorForTest("prod");
	__setEnvForTest(null);
	const md = buildRootMetadata("zh-TW");
	assert.equal((md.title as { template: string }).template, "%s | Hawkr");
	assert.equal((md.openGraph as { siteName: string }).siteName, "Hawkr");
	reset();
});

test("root declares NO social title/description, so every page fills its own", () => {
	// The one line standing between this site and 「every page shares the home
	// page's card」. Next resolves a social title by falling back
	// twitter → openGraph → metadata.title, and each link only fires when the
	// previous one is EMPTY. Segment merging keeps a parent's `openGraph` for
	// every descendant that does not declare its own, so a title spelled out
	// here is inherited site-wide and short-circuits the fallback at link one.
	//
	// That is not hypothetical: it is measured. With `openGraph.title` set here,
	// 84 of the site's 98 routes served 「Hawkr — 在地職人的市集」 as their
	// og:title AND twitter:title — /login, /cart, every /account and
	// /seller-center screen, and the product page's not-found branch. The
	// `description` twin did the same to /privacy, /terms, /return-policy,
	// /user-policy and /cart, whose share cards described the marketplace while
	// their own <meta name="description"> described the page.
	//
	// Deleting a per-page og:title is a one-page bug someone notices. Adding a
	// value HERE is a whole-site bug nobody notices, because each page's own
	// <title> still looks perfectly correct in the tab. Hence a test, not a
	// comment — tests/seo/social-title.seo.spec.ts asserts the served HTML.
	__setFlavorForTest("prod");
	__setEnvForTest(null);
	const og = buildRootMetadata("zh-TW").openGraph as Record<string, unknown>;
	const twitter = buildRootMetadata("zh-TW").twitter as Record<string, unknown>;

	assert.equal(
		og.title,
		undefined,
		"root openGraph.title is inherited by every page that declares no openGraph " +
			"of its own, which suppresses Next's per-page og:title fallback site-wide",
	);
	assert.equal(
		og.description,
		undefined,
		"root openGraph.description shadows each page's own description the same way",
	);
	assert.equal(
		twitter.title,
		undefined,
		"root twitter.title suppresses the openGraph → twitter fill for every page",
	);
	assert.equal(twitter.description, undefined);

	// The identity/shape half of the root card must survive: these are the keys
	// a page inherits and SHOULD, and deleting them is the opposite mistake.
	assert.equal(og.siteName, "Hawkr");
	assert.equal(og.locale, "zh_TW");
	assert.equal(og.type, "website");
	assert.equal(og.url, "https://example.com");
	assert.equal(twitter.card, "summary_large_image");
	reset();
});

test("noindexMetadata de-indexes but keeps following links", () => {
	assert.deepEqual(noindexMetadata().robots, { index: false, follow: true });
});

test("buildPageMetadata sets a self-canonical and matching og:url from the path", () => {
	__setFlavorForTest("prod");
	__setEnvForTest(null);
	const md = buildPageMetadata({
		title: "常見問題",
		description: "d",
		path: "/faq",
		locale: "zh-TW",
	});
	assert.equal(md.alternates?.canonical, "https://example.com/faq");
	assert.equal(
		(md.openGraph as { url: string }).url,
		"https://example.com/faq",
	);
	reset();
});

test("buildPageMetadata passes a string title through for the root template", () => {
	__setFlavorForTest("prod");
	__setEnvForTest(null);
	const md = buildPageMetadata({
		title: "常見問題",
		description: "d",
		path: "/faq",
		locale: "zh-TW",
	});
	assert.equal(md.title, "常見問題");
	reset();
});

test("buildPageMetadata supports an absolute title (home, no suffix)", () => {
	__setFlavorForTest("prod");
	__setEnvForTest(null);
	const md = buildPageMetadata({
		title: { absolute: "Hawkr" },
		description: "d",
		path: "/",
		locale: "zh-TW",
	});
	assert.deepEqual(md.title, { absolute: "Hawkr" });
	reset();
});

test("buildPageMetadata keeps the default OG siteName/locale", () => {
	__setFlavorForTest("prod");
	__setEnvForTest(null);
	const og = buildPageMetadata({
		title: "x",
		description: "d",
		path: "/faq",
		locale: "zh-TW",
	}).openGraph as { siteName: string; locale: string };
	assert.equal(og.siteName, "Hawkr");
	assert.equal(og.locale, "zh_TW");
	reset();
});

test("buildPageMetadata includes an og image when provided", () => {
	__setFlavorForTest("prod");
	__setEnvForTest(null);
	const md = buildPageMetadata({
		title: "x",
		description: "d",
		path: "/products/1",
		locale: "zh-TW",
		image: "https://img.example/1.jpg",
	});
	assert.deepEqual((md.openGraph as { images: unknown }).images, [
		{ url: "https://img.example/1.jpg" },
	]);
	reset();
});

const origin = () => getEnv().webOrigin;

test("root metadata for en: og:locale en_US, og:url is the /en home, the glossary's brand name, template and default title", () => {
	__setFlavorForTest("prod");
	__setEnvForTest(null);
	const md = buildRootMetadata("en");
	const og = md.openGraph as Record<string, unknown>;
	assert.equal(og.locale, "en_US");
	assert.equal(og.url, `${origin()}/en`);
	assert.equal(og.siteName, "Hawkr");
	assert.equal(md.applicationName, "Hawkr");
	// 術語表「品牌」一列（L0 核可）：Hawkr、`%s | Hawkr`、`Hawkr — A marketplace for local artisans`
	assert.deepEqual(md.title, {
		template: "%s | Hawkr",
		default: "Hawkr — A marketplace for local artisans",
	});
	reset();
});

test("root metadata for zh-TW keeps Hawkr and its own template and default title", () => {
	__setFlavorForTest("prod");
	__setEnvForTest(null);
	const md = buildRootMetadata("zh-TW");
	assert.deepEqual(md.title, {
		template: "%s | Hawkr",
		default: "Hawkr — 在地職人的市集",
	});
	assert.equal(siteNameFor("zh-TW"), "Hawkr");
	assert.equal(
		defaultTitleFor("en"),
		"Hawkr — A marketplace for local artisans",
	);
	reset();
});

test("a page's og:siteName follows its locale", () => {
	__setFlavorForTest("prod");
	__setEnvForTest(null);
	const og = buildPageMetadata({
		title: "x",
		description: "d",
		path: "/faq",
		locale: "en",
	}).openGraph as { siteName: string };
	assert.equal(og.siteName, "Hawkr");
	reset();
});

test("buildPageMetadata for en: canonical and og:url are the /en URL, og:locale en_US", () => {
	__setFlavorForTest("prod");
	__setEnvForTest(null);
	const md = buildPageMetadata({
		title: "x",
		description: "d",
		path: "/faq",
		locale: "en",
	});
	assert.equal(md.alternates?.canonical, `${origin()}/en/faq`);
	const og = md.openGraph as { url: string; locale: string };
	assert.equal(og.url, `${origin()}/en/faq`);
	assert.equal(og.locale, "en_US");
	reset();
});

test("buildPageMetadata: the home page of en is the bare /en", () => {
	__setFlavorForTest("prod");
	__setEnvForTest(null);
	const md = buildPageMetadata({
		title: { absolute: "x" },
		description: "d",
		path: "/",
		locale: "en",
	});
	assert.equal(md.alternates?.canonical, `${origin()}/en`);
	reset();
});

test("no hreflang while one locale is published; hreflang lists published locales with x-default once two are", () => {
	__setFlavorForTest("prod");
	__setEnvForTest(null);
	try {
		const single = buildPageMetadata({
			title: "x",
			description: "d",
			path: "/faq",
			locale: "zh-TW",
		});
		assert.equal(single.alternates?.languages, undefined);

		__setPublishedLocalesForTest({
			buyer: ["zh-TW", "en"],
			seller: ["zh-TW"],
		});
		const both = buildPageMetadata({
			title: "x",
			description: "d",
			path: "/faq",
			locale: "en",
		});
		assert.deepEqual(both.alternates?.languages, {
			"zh-TW": `${origin()}/faq`,
			en: `${origin()}/en/faq`,
			"x-default": `${origin()}/faq`,
		});
		// The seller surface has not published en: its pages announce nothing.
		const seller = buildPageMetadata({
			title: "x",
			description: "d",
			path: "/seller",
			locale: "zh-TW",
		});
		assert.equal(seller.alternates?.languages, undefined);
	} finally {
		__setPublishedLocalesForTest(null);
		reset();
	}
});
