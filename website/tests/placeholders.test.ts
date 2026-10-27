import assert from "node:assert/strict";
import { existsSync, readdirSync, readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";
import {
	PLACEHOLDER_DIR,
	renderPlaceholders,
} from "../scripts/gen-placeholders";

const websiteRoot = process.cwd();
const publicRoot = path.join(websiteRoot, "public");
const placeholderDir = path.join(websiteRoot, PLACEHOLDER_DIR);

const SOURCE_DIRS = ["app", "components", "features", "lib", "styles"];
const SOURCE_EXTENSIONS = new Set([".ts", ".tsx", ".css", ".json"]);
const ASSET_PREFIXES = ["/images/", "/defaults/", "/PNG/", "/placeholders/"];

function isTestFile(file: string): boolean {
	return (
		/\.test\.[cm]?[jt]sx?$/.test(file) ||
		file.split(path.sep).includes("__tests__")
	);
}

function walk(dir: string): string[] {
	const out: string[] = [];
	for (const entry of readdirSync(dir, { withFileTypes: true })) {
		if (entry.name === "node_modules" || entry.name === ".next") continue;
		const full = path.join(dir, entry.name);
		if (entry.isDirectory()) out.push(...walk(full));
		else out.push(full);
	}
	return out.sort();
}

function sourceFiles(): string[] {
	return SOURCE_DIRS.flatMap((dir) => walk(path.join(websiteRoot, dir))).filter(
		(file) => SOURCE_EXTENSIONS.has(path.extname(file)) && !isTestFile(file),
	);
}

/** Local asset paths written as string or `url()` literals. */
function assetReferences(text: string): string[] {
	const refs: string[] = [];
	for (const match of text.matchAll(/["'`(](\/[A-Za-z][^"'`()\s?#]*)/g)) {
		const candidate = match[1].split("${")[0];
		if (!ASSET_PREFIXES.some((prefix) => candidate.startsWith(prefix)))
			continue;
		if (candidate.includes("\u2026")) continue;
		refs.push(candidate);
	}
	return refs;
}

test("public/placeholders matches the generator output byte for byte", () => {
	const expected = renderPlaceholders();
	assert.ok(expected.length > 0);
	for (const { file, svg } of expected) {
		const target = path.join(placeholderDir, file);
		assert.equal(existsSync(target), true, `${file} is missing`);
		assert.equal(readFileSync(target, "utf8"), svg, `${file} drifted`);
	}
	assert.deepEqual(
		readdirSync(placeholderDir).sort(),
		expected.map(({ file }) => file).sort(),
	);
});

test("every local asset path in the source exists under public/", () => {
	const missing: string[] = [];
	for (const file of sourceFiles()) {
		for (const ref of assetReferences(readFileSync(file, "utf8"))) {
			if (!existsSync(path.join(publicRoot, ref))) {
				missing.push(`${path.relative(websiteRoot, file)}: ${ref}`);
			}
		}
	}
	assert.deepEqual(missing, []);
});
