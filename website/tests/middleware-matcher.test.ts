import assert from "node:assert/strict";
import { readdirSync } from "node:fs";
import test from "node:test";
import { fileURLToPath } from "node:url";
import {
	GENERATED_ROOT_FILES,
	isLocalizablePagePath,
	PUBLIC_DIRECTORIES,
} from "../lib/intl/paths";
import { config } from "../middleware";
import { compiledMatcher } from "./helpers/middleware-matcher";

const publicDir = fileURLToPath(new URL("../public", import.meta.url));

/**
 * The matcher keeps `public/`'s top-level directories out of the middleware (something serves them),
 * and nothing else. If it listed fewer, `next start` and dev would rewrite their files to `/zh-TW/…`
 * and 404 them; if it listed more, a page route of that name would never see the middleware.
 */
test("the matcher's public/ directory list is exactly the directories under public/", () => {
	const listed = config.matcher[0].match(
		/\(\?:((?:[A-Za-z]+\|)+[A-Za-z]+)\)\//,
	);
	assert.ok(listed, "matcher has no public/ directory group");
	const inMatcher = listed[1].split("|").sort();
	const onDisk = readdirSync(publicDir, { withFileTypes: true })
		.filter((entry) => entry.isDirectory())
		.map((entry) => entry.name)
		.sort();
	assert.deepEqual(
		inMatcher,
		onDisk,
		"public/ 頂層目錄變了：同步 middleware.ts 的 matcher（它只排除 public/ 的目錄、不靠副檔名）。",
	);
});

test("public/ holds no root-level file the matcher would send through the middleware by mistake", () => {
	const rootFiles = readdirSync(publicDir, { withFileTypes: true })
		.filter((entry) => entry.isFile())
		.map((entry) => entry.name)
		.sort();
	// `_headers` is read by Cloudflare and never requested; the association file is in NON_LOCALIZED_EXACT.
	assert.deepEqual(rootFiles, ["_headers", "apple-app-site-association"]);
});

test("the matcher is linear: a long path costs nothing", () => {
	const regex = compiledMatcher();
	for (const path of [
		`/${"a/".repeat(32000)}x.png`,
		`/${"/".repeat(64000)}`,
		`/${"a".repeat(64000)}.png`,
		`/${"a.".repeat(32000)}`,
	]) {
		const started = performance.now();
		regex.test(path);
		const elapsed = performance.now() - started;
		assert.ok(
			elapsed < 100,
			`${elapsed.toFixed(1)} ms for a ${path.length}-char path`,
		);
	}
});

test("the navigation layer's file lists are the matcher's: what the middleware skips, the layer never prefixes", () => {
	const source = config.matcher[0];
	const dirs = source.match(/\(\?:((?:[A-Za-z]+\|)+[A-Za-z]+)\)\//);
	assert.ok(dirs);
	assert.deepEqual([...PUBLIC_DIRECTORIES].sort(), dirs[1].split("|").sort());
	const files = source.match(/\(\?:((?:[a-z.\\]+\|)+[a-z.\\]+)\)\$/);
	assert.ok(files, "matcher has no generated-root-file group");
	assert.deepEqual(
		[...GENERATED_ROOT_FILES].map((file) => file.slice(1)).sort(),
		files[1].replaceAll("\\", "").split("|").sort(),
	);
	// and behaviorally, through the compiled expression: a path the matcher leaves out is not a page
	const regex = compiledMatcher();
	for (const path of [
		"/robots.txt",
		"/sitemap.xml",
		"/manifest.webmanifest",
		"/icon.svg",
		"/placeholders/products/1.png",
		"/brand/wordmark.svg",
		"/data/taiwan.geojson",
		"/templates/x.csv",
		"/_next/static/x.js",
		"/api/v1/x",
	]) {
		assert.equal(regex.test(path), false, `${path} should skip the middleware`);
		assert.equal(
			isLocalizablePagePath(path),
			false,
			`${path} skips the middleware, so it is not a page to prefix`,
		);
	}
});
