import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import test from "node:test";
import { fileURLToPath } from "node:url";
import postcss from "postcss";
import { openhuninn } from "../lib/fonts/openhuninn";

const read = (path: string) =>
	readFileSync(fileURLToPath(new URL(`../${path}`, import.meta.url)), "utf8");
const bytes = (path: string) =>
	readFileSync(fileURLToPath(new URL(`../${path}`, import.meta.url)));
const digest = (value: string | Buffer) =>
	createHash("sha256").update(value).digest("hex");

type FontShard = {
	id: string;
	file: string;
	sha256: string;
	bytes: number;
	codepoints: number;
	cmap_sha256: string;
	unicode_ranges: string;
};

function rangeCodepoints(value: string): number[] {
	const codepoints: number[] = [];
	for (const token of value.split(/\s*,\s*/)) {
		const match = /^U\+([0-9A-F]{1,6})(?:-([0-9A-F]{1,6}))?$/i.exec(token);
		assert.ok(match, `invalid unicode range: ${token}`);
		const start = Number.parseInt(match[1], 16);
		const end = Number.parseInt(match[2] ?? match[1], 16);
		assert.ok(start <= end && end <= 0x10ffff);
		for (let cp = start; cp <= end; cp += 1) codepoints.push(cp);
	}
	return codepoints;
}

const cmapDigest = (codepoints: number[]) =>
	digest(
		[...codepoints]
			.sort((a, b) => a - b)
			.map((cp) => cp.toString(16).toUpperCase())
			.join(","),
	);

test("brand font keeps the root token bridge and metric-compatible fallback", () => {
	const layout = read("components/shared/RootDocument.tsx");
	const globals = read("styles/globals.css");
	const css = postcss.parse(read("styles/foundation/fonts.css"));
	assert.equal(openhuninn.variable, "hk-font-rounded");
	assert.match(
		layout,
		/<html[\s\S]*?className=\{`[^`]*\$\{openhuninn\.variable\}/,
	);
	assert.ok(
		globals.indexOf('@import "./foundation/fonts.css"') <
			globals.indexOf('@import "./foundation/01-tokens-theme.css"'),
	);
	let variableFound = false;
	css.walkRules(".hk-font-rounded", (rule) => {
		rule.walkDecls("--font-openhuninn", (declaration) => {
			variableFound = true;
			assert.equal(
				declaration.value,
				'"Hawkr Rounded", "Hawkr Rounded Fallback"',
			);
		});
	});
	assert.ok(variableFound, "root class must define the existing font token");
	const fallbacks: Record<string, string>[] = [];
	css.walkAtRules("font-face", (rule) => {
		const declarations: Record<string, string> = {};
		rule.walkDecls((declaration) => {
			declarations[declaration.prop] = declaration.value;
		});
		fallbacks.push(declarations);
	});
	assert.deepEqual(fallbacks, [
		{
			"font-family": '"Hawkr Rounded Fallback"',
			src: 'local("Arial")',
			"ascent-override": "80.97%",
			"descent-override": "11.04%",
			"line-gap-override": "0%",
			"size-adjust": "106.13%",
		},
	]);
});

test("every brand font shard swaps in after loading and keeps the original weight range", () => {
	const manifest = JSON.parse(
		read("assets/fonts/hawkr-rounded-v2/manifest.json"),
	);
	// A cached base face must not leave late cold glyphs on fallback permanently.
	assert.equal(manifest.css.font_display, "swap");
	const css = postcss.parse(read("styles/foundation/openhuninn.generated.css"));
	let faces = 0;
	css.walkAtRules("font-face", (rule) => {
		faces += 1;
		const declarations: Record<string, string> = {};
		rule.walkDecls((declaration) => {
			declarations[declaration.prop] = declaration.value;
		});
		assert.equal(declarations["font-family"], '"Hawkr Rounded"');
		assert.equal(declarations["font-display"], manifest.css.font_display);
		assert.equal(declarations["font-weight"], "400 900");
		assert.equal(declarations["font-style"], "normal");
		assert.match(declarations["unicode-range"], /^U\+[0-9A-F]/i);
		assert.match(
			declarations.src,
			/^url\("\.\.\/\.\.\/assets\/fonts\/hawkr-rounded-v2\/[a-z0-9-]+\.woff2"\) format\("woff2"\)$/,
		);
	});
	assert.ok(faces > 1, "font coverage must be split across native CSS faces");
	const bridge = read("lib/fonts/openhuninn.ts");
	assert.doesNotMatch(
		bridge,
		/import\s|localFont\(|FontFace\(|document\.|setTimeout\(/,
	);
	assert.doesNotMatch(
		read("components/shared/RootDocument.tsx"),
		/as=["']font["']/,
	);
});

test("font shard artifacts and CSS preserve the pinned complete upstream cmap", () => {
	const directory = "assets/fonts/hawkr-rounded-v2";
	const manifest = JSON.parse(read(`${directory}/manifest.json`));
	assert.equal(manifest.family, "Hawkr Rounded");
	assert.equal(
		manifest.source.commit,
		"dc5538c28ec309b6f6b51997bbfcdb531b9797a8",
	);
	assert.equal(
		manifest.source.sha256,
		"eec8b0b68c34b9166ae37bed839b6126116b225d97f21a5954bb542b9fd1e68c",
	);
	// Independently read from the pinned upstream TTF, not a page text scan.
	const fullCmapDigest =
		"e6eb428cc2488f1b4c6ac93e4b093df4924ed3bde47f2635dc69a1ce9015828f";
	assert.equal(manifest.source.cmap_codepoints, 11345);
	assert.equal(manifest.source.cmap_sha256, fullCmapDigest);
	assert.equal(
		digest(bytes(`${directory}/${manifest.license.file}`)),
		"c1a67a328fa5ce1f1f205fa9481d15fe1f05a27e6e4076919c4de85385e6f521",
	);
	const generatedCss = read("styles/foundation/openhuninn.generated.css");
	assert.equal(digest(generatedCss), manifest.css.sha256);
	const faces = new Map<string, number[]>();
	postcss.parse(generatedCss).walkAtRules("font-face", (rule) => {
		const declarations: Record<string, string> = {};
		rule.walkDecls((declaration) => {
			declarations[declaration.prop] = declaration.value;
		});
		const file = /\/([a-z0-9-]+\.woff2)"/.exec(declarations.src)?.[1];
		assert.ok(file, "generated CSS must reference a bundled font file");
		assert.ok(!faces.has(file), `duplicate face: ${file}`);
		faces.set(file, rangeCodepoints(declarations["unicode-range"]));
	});
	const union = new Set<number>();
	let totalBytes = 0;
	for (const shard of manifest.shards as FontShard[]) {
		assert.match(
			shard.file,
			/^hawkr-rounded-v2-[a-z0-9-]+-[0-9a-f]{12}\.woff2$/,
		);
		const data = bytes(`${directory}/${shard.file}`);
		assert.equal(data.subarray(0, 4).toString("ascii"), "wOF2");
		assert.equal(data.readUInt32BE(8), data.length, "WOFF2 length must match");
		assert.equal(data.length, shard.bytes);
		assert.equal(digest(data), shard.sha256);
		assert.ok(shard.file.endsWith(`-${shard.sha256.slice(0, 12)}.woff2`));
		const codepoints = rangeCodepoints(shard.unicode_ranges);
		assert.equal(codepoints.length, shard.codepoints);
		assert.equal(cmapDigest(codepoints), shard.cmap_sha256);
		assert.deepEqual(faces.get(shard.file), codepoints);
		for (const cp of codepoints) {
			assert.ok(
				!union.has(cp),
				`overlapping font ranges at U+${cp.toString(16)}`,
			);
			union.add(cp);
		}
		totalBytes += data.length;
	}
	assert.equal(faces.size, manifest.shards.length);
	assert.equal(totalBytes, manifest.total_woff2_bytes);
	assert.equal(union.size, 11345);
	assert.equal(cmapDigest([...union]), fullCmapDigest);
	for (const character of "新增貼文傷嘴扔歇泛癒逸幸運籤餅彧祐萱昀堃𪜶ㄅㄆㄇㄈ") {
		const codepoint = character.codePointAt(0);
		assert.ok(typeof codepoint === "number");
		assert.ok(union.has(codepoint));
	}
	assert.ok(
		!union.has(0x20bb7),
		"unsupported upstream glyph remains an OS fallback",
	);
});
