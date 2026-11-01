import assert from "node:assert/strict";
import { existsSync, readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

/**
 * The static shell, read from the prerendered HTML `next build` wrote (`pnpm run test:build-output`, run
 * after the build). There is no root `<Suspense>` around `{children}`: the header and the page's own
 * server-rendered parts are in the shell, and only the subtrees that read runtime data (the session answer,
 * `usePathname`, a client `Date.now()`) are holes resumed per request.
 */
const APP = fileURLToPath(new URL("../../.next/server/app/", import.meta.url));
const LOCALE_DIR = join(APP, "zh-TW");

function htmlFiles(dir: string, base = ""): string[] {
	return readdirSync(dir, { withFileTypes: true }).flatMap((entry) =>
		entry.isDirectory()
			? htmlFiles(join(dir, entry.name), `${base}${entry.name}/`)
			: entry.name.endsWith(".html")
				? [`${base}${entry.name}`]
				: [],
	);
}

function status(file: string): number {
	const meta = JSON.parse(
		readFileSync(join(LOCALE_DIR, file.replace(/\.html$/, ".meta")), "utf8"),
	) as { status?: number };
	return meta.status ?? 200;
}

// A seller workspace renders the seller header instead, and a sentinel 404 renders the not-found body.
const OWN_CHROME = /^seller(?:-center)?(?:[/.]|$)/;

test("the site header is in the static shell of every page that shows it", () => {
	assert.ok(
		existsSync(LOCALE_DIR),
		"zh-TW is not in .next/server/app: run `pnpm run build` first",
	);
	const pages = htmlFiles(LOCALE_DIR).filter(
		(file) =>
			!file.includes("[") && !OWN_CHROME.test(file) && status(file) === 200,
	);
	assert.ok(pages.length > 30, `only ${pages.length} header-bearing pages`);
	const missing = pages.filter(
		(file) =>
			!readFileSync(join(LOCALE_DIR, file), "utf8").includes("site-header"),
	);
	assert.deepEqual(missing, []);
});

test("a page without a runtime-data hole has its main content in the shell", () => {
	const html = readFileSync(join(LOCALE_DIR, "faq.html"), "utf8");
	assert.match(html, /<h1[^>]*>常見問題/);
	assert.ok(html.includes("site-header"));
});
