import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const cdn = readFileSync(
	new URL("../lib/media/cdn.ts", import.meta.url),
	"utf8",
);

test("cdn helper no longer emits any x-oss-process derivative pipeline", () => {
	assert.doesNotMatch(cdn, /x-oss-process=/);
	assert.doesNotMatch(cdn, /ossImageProcessPresets/);
	assert.doesNotMatch(cdn, /image\/resize/);
	assert.doesNotMatch(cdn, /format,webp/);
});

test("cdn helper routes everything through the buildImageUrl entry point", () => {
	assert.match(cdn, /export function buildImageUrl/);
	assert.match(cdn, /\/i\/\$\{preset\}\//);
	assert.match(cdn, /export function resolveCdnUrl/);
	assert.match(cdn, /export function ossImageUrl/);
});
