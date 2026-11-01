import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { requiredAccessForPath } from "../../lib/auth/redirect";
import { stripLocale } from "../../lib/intl/paths";

/**
 * How `next build` prerendered each route, read from the prerender manifest it wrote
 * (`pnpm run test:build-output`, run after the build).
 *
 * `PARTIALLY_STATIC` is a static shell plus dynamic holes resumed per request. The session answer
 * (`verifySession()` in the root layout, read by `AuthProvider` behind a `<Suspense>`) and the zone
 * layouts' access decision (`ZoneGate`) are those holes, so every page that shows the header or sits
 * behind a zone must be `PARTIALLY_STATIC`. A page that falls back to `STATIC` has frozen one
 * visitor's answer into HTML for everyone; one that stops being prerendered at all has lost its shell.
 */
const MANIFEST = fileURLToPath(
	new URL("../../.next/prerender-manifest.json", import.meta.url),
);

interface ManifestRoute {
	renderingMode?: string;
}

function readRoutes(): Map<string, string | undefined> {
	assert.ok(
		existsSync(MANIFEST),
		"prerender-manifest.json is not in .next: run `pnpm run build` first",
	);
	const manifest = JSON.parse(readFileSync(MANIFEST, "utf8")) as {
		routes: Record<string, ManifestRoute>;
		dynamicRoutes: Record<string, ManifestRoute>;
	};
	return new Map(
		Object.entries({ ...manifest.routes, ...manifest.dynamicRoutes }).map(
			([path, route]) => [path, route.renderingMode],
		),
	);
}

// Route handlers and metadata files: no document, no header.
const NOT_A_PAGE =
	/^\/(?:api\/|apple-icon|icon\.svg|manifest\.webmanifest|opengraph-image|robots\.txt|sitemap\.xml)/;

function pages(): Array<[string, string | undefined]> {
	return [...readRoutes()].filter(([path]) => !NOT_A_PAGE.test(path));
}

test("every page that renders the site header is PARTIALLY_STATIC", () => {
	const all = pages();
	assert.ok(all.length > 100, `only ${all.length} pages in the manifest`);
	const offenders = all
		.filter(([, mode]) => mode !== "PARTIALLY_STATIC")
		.map(([path, mode]) => `${path}: ${mode}`);
	assert.deepEqual(offenders, []);
	for (const home of ["/zh-TW", "/_not-found"]) {
		assert.equal(
			readRoutes().get(home),
			"PARTIALLY_STATIC",
			`${home} is not in the manifest as PARTIALLY_STATIC`,
		);
	}
});

test("every protected zone page is PARTIALLY_STATIC", () => {
	const protectedPages = pages().filter(([path]) =>
		requiredAccessForPath(stripLocale(path)),
	);
	for (const prefix of [
		"/account",
		"/checkout",
		"/seller-center",
		"/seller/measures",
		"/seller-product-import-preview",
	]) {
		assert.ok(
			protectedPages.some(([path]) => stripLocale(path).startsWith(prefix)),
			`no ${prefix} page in the manifest`,
		);
	}
	const offenders = protectedPages
		.filter(([, mode]) => mode !== "PARTIALLY_STATIC")
		.map(([path, mode]) => `${path}: ${mode}`);
	assert.deepEqual(offenders, []);
});
