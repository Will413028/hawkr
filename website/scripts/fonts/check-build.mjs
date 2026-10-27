#!/usr/bin/env node

import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { dirname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import postcss from "postcss";

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), "../..");
const NEXT_DIR = join(ROOT, ".next");
const HOME_HTML = join(NEXT_DIR, "server/app/zh-TW.html");
const FONT_DIR = join(ROOT, "assets/fonts/hawkr-rounded-v2");
const MANIFEST = JSON.parse(
	readFileSync(join(FONT_DIR, "manifest.json"), "utf8"),
);
const FAMILY = "Hawkr Rounded";
const LEGACY_SUBSET_SHA256 =
	"4eba768ebf0e8a4b8467e6b914a284cf4906bea03e325d64477af806054fe1d6";

const digest = (value) => createHash("sha256").update(value).digest("hex");
const fail = (message) => {
	throw new Error(message);
};

function attribute(tag, name) {
	const match = new RegExp(
		`(?:^|\\s)${name}\\s*=\\s*(?:"([^"]*)"|'([^']*)'|([^\\s>]+))`,
		"i",
	).exec(tag);
	return match?.[1] ?? match?.[2] ?? match?.[3] ?? null;
}

function buildPathFromUrl(url, baseUrl = "https://build.local/") {
	const pathname = new URL(url.replaceAll("&amp;", "&"), baseUrl).pathname;
	if (!pathname.startsWith("/_next/")) {
		fail(`expected a /_next/ build URL, received ${pathname}`);
	}
	const path = resolve(NEXT_DIR, pathname.slice("/_next/".length));
	if (relative(NEXT_DIR, path).startsWith("..")) {
		fail(`build URL escaped .next: ${pathname}`);
	}
	return { path, pathname };
}

function parseRanges(value) {
	const codepoints = [];
	for (const token of value.split(/\s*,\s*/)) {
		const match = /^U\+([0-9A-F]{1,6})(?:-([0-9A-F]{1,6}))?$/i.exec(token);
		assert.ok(match, `invalid compiled unicode-range token: ${token}`);
		const start = Number.parseInt(match[1], 16);
		const end = Number.parseInt(match[2] ?? match[1], 16);
		assert.ok(start <= end && end <= 0x10ffff, `invalid range: ${token}`);
		for (let codepoint = start; codepoint <= end; codepoint += 1) {
			codepoints.push(codepoint);
		}
	}
	return codepoints;
}

function cmapDigest(codepoints) {
	return digest(
		[...new Set(codepoints)]
			.sort((left, right) => left - right)
			.map((codepoint) => codepoint.toString(16).toUpperCase())
			.join(","),
	);
}

function fontUrl(value) {
	const match = /url\(\s*(?:"([^"]+)"|'([^']+)'|([^\s)]+))\s*\)/i.exec(value);
	return match?.[1] ?? match?.[2] ?? match?.[3] ?? null;
}

function unquote(value) {
	const trimmed = value.trim();
	if (
		(trimmed.startsWith('"') && trimmed.endsWith('"')) ||
		(trimmed.startsWith("'") && trimmed.endsWith("'"))
	) {
		return trimmed.slice(1, -1);
	}
	return trimmed;
}

function declarations(rule) {
	const values = new Map();
	rule.walkDecls((declaration) =>
		values.set(declaration.prop, declaration.value),
	);
	return values;
}

function main() {
	assert.equal(MANIFEST.family, FAMILY, "unexpected font manifest family");
	assert.equal(MANIFEST.css.font_display, "swap");
	assert.equal(
		MANIFEST.shards.length,
		80,
		"PERF-08 build must contain 80 shards",
	);

	const html = readFileSync(HOME_HTML, "utf8");
	const htmlTag = /<html\b[^>]*>/i.exec(html)?.[0];
	assert.ok(htmlTag, "homepage build is missing its <html> tag");
	const htmlClasses = new Set((attribute(htmlTag, "class") ?? "").split(/\s+/));
	assert.ok(
		htmlClasses.has("hk-font-rounded"),
		"homepage <html> lost hk-font-rounded",
	);

	const linkTags = html.match(/<link\b[^>]*>/gi) ?? [];
	const stylesheetHrefs = [
		...new Set(
			linkTags
				.filter((tag) =>
					(attribute(tag, "rel") ?? "").split(/\s+/).includes("stylesheet"),
				)
				.map((tag) => attribute(tag, "href"))
				.filter(Boolean),
		),
	];
	assert.ok(
		stylesheetHrefs.length > 0,
		"homepage build has no stylesheet links",
	);

	const preloadedFonts = linkTags
		.filter(
			(tag) =>
				(attribute(tag, "rel") ?? "").split(/\s+/).includes("preload") &&
				attribute(tag, "as") === "font",
		)
		.map((tag) => attribute(tag, "href"))
		.filter(Boolean);

	const manifestByCmap = new Map();
	for (const shard of MANIFEST.shards) {
		assert.ok(
			!manifestByCmap.has(shard.cmap_sha256),
			`duplicate manifest cmap: ${shard.id}`,
		);
		manifestByCmap.set(shard.cmap_sha256, shard);
	}

	let variableBridgeFound = false;
	const faces = [];
	for (const href of stylesheetHrefs) {
		const compiled = buildPathFromUrl(href);
		const css = postcss.parse(readFileSync(compiled.path, "utf8"), {
			from: compiled.path,
		});
		css.walkRules((rule) => {
			if (
				!rule.selector
					?.split(",")
					.some((selector) => selector.trim() === ".hk-font-rounded")
			) {
				return;
			}
			rule.walkDecls("--font-openhuninn", (declaration) => {
				if (
					declaration.value.includes(FAMILY) &&
					declaration.value.includes("Hawkr Rounded Fallback")
				) {
					variableBridgeFound = true;
				}
			});
		});
		css.walkAtRules("font-face", (rule) => {
			const values = declarations(rule);
			if (unquote(values.get("font-family") ?? "") !== FAMILY) return;
			faces.push({ href, compiled, values });
		});
	}

	assert.ok(
		variableBridgeFound,
		"compiled homepage CSS lost the hk-font-rounded variable bridge; check the Turbopack cache",
	);
	assert.equal(
		faces.length,
		MANIFEST.shards.length,
		"compiled homepage CSS must contain exactly one native face per manifest shard",
	);

	const matchedShardIds = new Set();
	const compiledFontPaths = new Set();
	for (const face of faces) {
		const { values } = face;
		assert.equal(values.get("font-style"), "normal");
		assert.equal(values.get("font-weight"), "400 900");
		assert.equal(values.get("font-display"), MANIFEST.css.font_display);
		const coverage = parseRanges(values.get("unicode-range") ?? "");
		const coverageDigest = cmapDigest(coverage);
		const shard = manifestByCmap.get(coverageDigest);
		assert.ok(
			shard,
			`compiled face coverage is absent from the manifest: ${coverageDigest}`,
		);
		assert.equal(
			coverage.length,
			shard.codepoints,
			`${shard.id} coverage count changed`,
		);
		assert.ok(
			!matchedShardIds.has(shard.id),
			`compiled shard is duplicated: ${shard.id}`,
		);

		const sourceUrl = fontUrl(values.get("src") ?? "");
		assert.ok(sourceUrl, `${shard.id} has no compiled WOFF2 URL`);
		const baseUrl = `https://build.local${face.compiled.pathname}`;
		const compiledFont = buildPathFromUrl(sourceUrl, baseUrl);
		assert.ok(
			compiledFont.pathname.startsWith("/_next/static/media/"),
			`${shard.id} did not compile into static media`,
		);
		const bytes = readFileSync(compiledFont.path);
		assert.equal(
			bytes.subarray(0, 4).toString("ascii"),
			"wOF2",
			`${shard.id} is not WOFF2`,
		);
		assert.equal(
			bytes.length,
			shard.bytes,
			`${shard.id} compiled byte size changed`,
		);
		assert.equal(
			digest(bytes),
			shard.sha256,
			`${shard.id} compiled content hash changed`,
		);
		matchedShardIds.add(shard.id);
		compiledFontPaths.add(compiledFont.pathname);
	}
	assert.equal(
		matchedShardIds.size,
		MANIFEST.shards.length,
		"not every manifest shard was compiled",
	);
	assert.equal(
		compiledFontPaths.size,
		MANIFEST.shards.length,
		"compiled font URLs are not unique",
	);

	for (const href of preloadedFonts) {
		const pathname = new URL(
			href.replaceAll("&amp;", "&"),
			"https://build.local/",
		).pathname;
		assert.ok(
			!pathname.includes("hawkr-rounded-v2") &&
				!compiledFontPaths.has(pathname),
			`Hawkr Rounded must not be preloaded: ${pathname}`,
		);
	}

	const mediaDir = join(NEXT_DIR, "static/media");
	for (const name of readdirSync(mediaDir)) {
		if (!name.endsWith(".woff2")) continue;
		const path = join(mediaDir, name);
		if (statSync(path).isFile()) {
			assert.notEqual(
				digest(readFileSync(path)),
				LEGACY_SUBSET_SHA256,
				"legacy font subset leaked",
			);
		}
	}

	console.log(
		`verified homepage font build: ${stylesheetHrefs.length} stylesheets, ` +
			`${faces.length} Hawkr Rounded faces, ${compiledFontPaths.size} WOFF2 assets, no preload`,
	);
}

try {
	main();
} catch (error) {
	console.error(
		`[fonts:check-build] ${error instanceof Error ? error.message : String(error)}`,
	);
	process.exitCode = 1;
}
