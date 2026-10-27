import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { metaFromHtml } from "../seo/_helpers";

/**
 * The share picture, read from the files `next build` wrote (`pnpm run test:build-output`, run after the build).
 *
 * `app/opengraph-image.tsx` is a file convention: Next folds it into the metadata of the segment whose directory
 * holds it (`app/`), and the root layouts (`app/[locale]/`, `app/(delegated)/`) sit one segment below, where their
 * `openGraph` replaces the parent's resolved one. A page that declares no `openGraph` of its own therefore only
 * has a picture because `buildRootMetadata` names it. tests/seo-social-image.test.ts guards the cause; this guards
 * the effect in the prerendered HTML, which is where Next's merge and its twitter fallback actually ran.
 * (tests/seo/social-image.seo.spec.ts does the same against a running server, in the SEO gate.)
 */
const APP = fileURLToPath(new URL("../../.next/server/app/", import.meta.url));

function page(file: string): string {
	assert.ok(
		existsSync(`${APP}${file}`),
		`${file} is not in .next/server/app: run \`pnpm run build\` first`,
	);
	return readFileSync(`${APP}${file}`, "utf8");
}

/** Prerendered pages that declare no `openGraph` of their own, so they inherit the root layout's. */
const INHERITING = [
	"zh-TW/cart.html",
	"zh-TW/account/addresses.html",
	"zh-TW/login/sso-callback.html",
];

/** A prerendered page that declares its own `openGraph` and names the card through `socialImages()`. */
const DECLARING = ["zh-TW/about.html", "zh-TW/faq.html"];

for (const file of [...INHERITING, ...DECLARING]) {
	test(`${file} has a share picture, and the twitter card mirrors it`, () => {
		const html = page(file);
		const image = metaFromHtml(html, "og:image");
		assert.ok(image, `${file} has no og:image`);
		assert.equal(new URL(image).pathname, "/opengraph-image");
		assert.equal(metaFromHtml(html, "twitter:image"), image);
	});
}

for (const file of INHERITING) {
	test(`${file} describes its share picture`, () => {
		const html = page(file);
		assert.equal(metaFromHtml(html, "og:image:width"), "1200");
		assert.equal(metaFromHtml(html, "og:image:height"), "630");
		assert.equal(metaFromHtml(html, "og:image:type"), "image/png");
		assert.ok(metaFromHtml(html, "og:image:alt"));
		assert.equal(metaFromHtml(html, "twitter:image:width"), "1200");
		assert.ok(metaFromHtml(html, "twitter:image:alt"));
	});
}

test("the app-wide 404 keeps the picture the file convention publishes (it shares the file's segment)", () => {
	const html = page("_not-found.html");
	const image = metaFromHtml(html, "og:image");
	assert.ok(image, "the app-wide 404 has no og:image");
	assert.equal(new URL(image).pathname, "/opengraph-image");
	assert.equal(metaFromHtml(html, "og:image:width"), "1200");
	assert.equal(metaFromHtml(html, "twitter:image"), image);
});
