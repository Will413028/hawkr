import assert from "node:assert/strict";
import { existsSync, readdirSync } from "node:fs";
import { join } from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";
import { sellerGuideChapters } from "../lib/content/seller-guide-data";

const localeRoot = join(
	fileURLToPath(new URL("..", import.meta.url)),
	"app",
	"[locale]",
);

const PAGE_FILES = ["page.tsx", "page.ts", "page.jsx", "page.js"];

function isRouteGroup(name: string): boolean {
	return /^\(.+\)$/.test(name);
}

function hasPageFile(dir: string): boolean {
	return PAGE_FILES.some((file) => existsSync(join(dir, file)));
}

function subdirectories(dir: string): string[] {
	return readdirSync(dir, { withFileTypes: true })
		.filter((entry) => entry.isDirectory())
		.map((entry) => entry.name);
}

function resolveSegments(dir: string, segments: string[]): boolean {
	const [segment, ...rest] = segments;
	if (segment === undefined) return hasPageFile(dir);
	for (const name of subdirectories(dir)) {
		if (isRouteGroup(name)) {
			if (resolveSegments(join(dir, name), segments)) return true;
			continue;
		}
		if (name === segment && resolveSegments(join(dir, name), rest)) {
			return true;
		}
	}
	return false;
}

function routeHasPage(path: string): boolean {
	const pathname = path.split(/[?#]/)[0] ?? "";
	return resolveSegments(localeRoot, pathname.split("/").filter(Boolean));
}

test("every seller guide section path resolves to a directory with a page file", () => {
	const items = sellerGuideChapters.flatMap((chapter) => chapter.items);
	const withPath = items.filter((item) => item.path !== undefined);
	assert.ok(withPath.length > 0, "the guide must link to at least one route");
	for (const item of withPath) {
		assert.ok(
			routeHasPage(item.path as string),
			`${item.id}: ${item.path} has no page file under app/[locale]`,
		);
	}
});

test("seller guide section ids and numbers are unique", () => {
	const items = sellerGuideChapters.flatMap((chapter) => chapter.items);
	assert.equal(new Set(items.map((item) => item.id)).size, items.length);
	assert.equal(new Set(items.map((item) => item.no)).size, items.length);
});
