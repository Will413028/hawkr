import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

function source(path: string): string {
	return readFileSync(new URL(`../${path}`, import.meta.url), "utf8");
}

test("marketing routes no longer mount independent full-page background artwork", () => {
	for (const path of [
		"features/about/AboutPage.tsx",
		"features/seller/landing/SellerLandingPage.tsx",
		"features/seller/pricing/SellerPricingPage.tsx",
	]) {
		assert.doesNotMatch(
			source(path),
			/page-bg-tall|className="page-bg"|sp-page-bg|SellerVantaAtmosphere/,
			`${path} must use the persistent site background`,
		);
	}
});

test("help and legal routes do not overlay extra blurred background fields", () => {
	for (const path of [
		"features/content/faq/FaqPage.tsx",
		"features/content/contact/ContactPage.tsx",
		"features/legal/LegalPage.tsx",
	]) {
		assert.doesNotMatch(source(path), /bg-bokeh|blur-\[100px\]/);
	}
});

test("standalone seller and marketing page canvases stay transparent", () => {
	for (const [path, selector] of [
		["styles/features/seller-pricing.css", ".seller-pricing-page"],
		["styles/features/seller-card-page.css", ".seller-card-page"],
	] as const) {
		const css = source(path);
		const start = css.indexOf(`${selector} {`);
		assert.ok(start >= 0, `${path} must declare its page owner`);
		const rule = css.slice(start, css.indexOf("}", start) + 1);
		assert.match(rule, /background:\s*transparent;/);
	}
	assert.doesNotMatch(
		source("styles/features/seller-card-page.css"),
		/\.seller-card-page::before/,
	);
});

test("authored story media and the seller guide manual remain content surfaces", () => {
	const seller = source("features/seller/landing/SellerLandingPage.tsx");
	assert.match(seller, /<SellerRecruitmentScrollWorld/);
	assert.match(seller, /<SellerShowcaseCarousel/);
	assert.match(seller, /className="trail-illus trail-illus--field"/);
	assert.match(
		source("features/content/seller-guide/SellerGuidePage.tsx"),
		/sellerGuideChapters\.map/,
	);
	assert.match(
		source("styles/features/seller-card-page.css"),
		/\.seller-public-card \{[\s\S]*?background: var\(--hkui-surface\);/,
	);
});
