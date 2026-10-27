import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

function source(path: string): string {
	return readFileSync(new URL(path, import.meta.url), "utf8");
}

const footer = () => source("../components/shared/Footer.tsx");
const columns = () => source("../components/shared/FooterCategoryColumns.tsx");
const chrome = () => source("../components/shared/footerChrome.ts");

test("the buyer footer renders five columns in the agreed order", () => {
	const src = footer();
	const order = ["快速連結", "客戶服務", "聯絡資訊"].map((heading) =>
		src.indexOf(heading),
	);
	for (const index of order) assert.ok(index > 0, "missing footer heading");
	assert.ok(
		order[0] < src.indexOf("{categoryColumns}"),
		"探索 column must come before 商品分類 / 熱門話題",
	);
	assert.ok(
		src.indexOf("{categoryColumns}") < order[1],
		"商品分類 / 熱門話題 must come before 關於與服務",
	);
	assert.ok(order[1] < order[2], "聯絡 column stays last");

	const dynamic = columns();
	assert.ok(
		dynamic.indexOf('heading="商品分類"') <
			dynamic.indexOf('heading="熱門話題"'),
		"商品分類 renders before 熱門話題",
	);
});

test("dynamic columns reuse the mega menu href conventions", () => {
	const src = columns();
	assert.match(
		src,
		/return `\/market\/products\?category=\$\{encodeURIComponent\(name\)\}`;/,
	);
	assert.match(
		src,
		/return `\/feed\?hashtag=\$\{encodeURIComponent\(tag\)\}`;/,
	);
	// Read on the server through the shared unstable_cache layer, so the browser
	// issues no request at all (tests/e2e/footer-columns-server-rendered.spec.ts).
	assert.match(src, /getCachedCategoryTree/);
	assert.match(src, /getCachedTrendingPostTags/);
	assert.doesNotMatch(src, /"use client"/);
	assert.doesNotMatch(src, /new IntersectionObserver\(/);
});

test("each dynamic column is capped at eight links", () => {
	const src = columns();
	assert.match(src, /export const FOOTER_COLUMN_ITEM_CAP = 8;/);
	assert.equal(
		(src.match(/\.slice\(0, FOOTER_COLUMN_ITEM_CAP\)/g) ?? []).length,
		2,
		"both the category and hashtag lists must be capped",
	);
	assert.match(src, /getCachedTrendingPostTags\(FOOTER_COLUMN_ITEM_CAP\)/);
});

test("an empty or failed source falls back to one always-valid link", () => {
	const src = columns();
	assert.match(src, /label: "全部商品",\s*href: "\/market\/products",/);
	assert.match(src, /label: "看動態",\s*href: "\/feed",/);
	assert.match(src, /const items = links\.length > 0 \? links : \[fallback\];/);
});

// The loading skeleton is gone with the client fetch: the links are rendered on
// the server, so there is no frame in which the columns are pending.
test("no loading state remains in the columns", () => {
	const src = columns();
	assert.doesNotMatch(src, /SKELETON/);
	assert.doesNotMatch(chrome(), /FOOTER_SKELETON/);
});

test("links keep 44px touch targets and compact desktop pointer rows", () => {
	assert.match(
		chrome(),
		/FOOTER_TEXT_LINK =\s*"inline-flex min-h-11 items-center min-\[561px\]:pointer-fine:min-h-8/,
	);
	assert.match(columns(), /className=\{FOOTER_TEXT_LINK\}/);
});

test("columns collapse into native details/summary accordions on phones", () => {
	const src = footer();
	assert.equal(
		(src.match(/<details className=\{FOOTER_COLUMN\} open>/g) ?? []).length,
		2,
	);
	assert.match(src, /\$\{FOOTER_COLUMN\} max-\[1200px\]:col-span-full/);
	assert.equal(
		(src.match(/<summary className=\{FOOTER_SUMMARY\}>/g) ?? []).length,
		3,
	);
	assert.equal((src.match(/<\/details>/g) ?? []).length, 3);
	assert.equal(
		(columns().match(/<summary className=\{FOOTER_SUMMARY\}>/g) ?? []).length,
		1,
	);

	const css = chrome();
	// Phone-only accordion; above 560px the disclosure is inert.
	assert.match(css, /FOOTER_SUMMARY =[\s\S]*?max-\[560px\]:min-h-11/);
	assert.match(
		css,
		/FOOTER_COLUMN =[\s\S]*?min-\[561px\]:\[&::details-content\]/,
	);
	// Chevron comes from the shared icon registry and is reduced-motion safe.
	assert.match(
		css,
		/FOOTER_SUMMARY_CHEVRON =[\s\S]*?motion-reduce:transition-none/,
	);
	assert.match(src, /name="chevron-down"/);
});

test("the footer and its data columns stay Server Components", () => {
	assert.doesNotMatch(footer(), /"use client"/);
	assert.doesNotMatch(columns(), /"use client"/);
	// RoleAwareFooter is the client boundary that picks a footer, so the columns
	// are rendered above it and passed down as an element. Rendering them inside
	// it would force the reads back into the browser.
	const layout = source("../components/shared/RootDocument.tsx");
	assert.match(layout, /categoryColumns=\{<FooterCategoryColumns \/>\}/);
});
