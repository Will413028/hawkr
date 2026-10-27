import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

// The Aliyun `x-oss-process` derivative pipeline was removed in the OSS→R2
// migration (design book 49): R2 + the Cloudflare image gateway serve fixed
// named presets only, and no frontend code may emit resize/quality/format query
// params. This suite guards that the old pipeline never creeps back into the
// single image-URL entry point. Behavioural coverage lives in
// lib/media/__tests__/buildImageUrl.unit.test.ts (vitest); here we assert the
// source-level invariant that node --test can enforce without a DOM/env harness.
const cdn = readFileSync(
	new URL("../lib/media/cdn.ts", import.meta.url),
	"utf8",
);

test("cdn helper no longer emits any x-oss-process derivative pipeline", () => {
	// The emission patterns (query param, preset table, resize/format directives),
	// not the prose in the migration comment which still names the retired pipeline.
	assert.doesNotMatch(cdn, /x-oss-process=/);
	assert.doesNotMatch(cdn, /ossImageProcessPresets/);
	assert.doesNotMatch(cdn, /image\/resize/);
	assert.doesNotMatch(cdn, /format,webp/);
});

test("cdn helper routes everything through the buildImageUrl entry point", () => {
	assert.match(cdn, /export function buildImageUrl/);
	// Gate-ON output is the Worker image-gateway path `/i/{preset}/{key}`.
	assert.match(cdn, /\/i\/\$\{preset\}\//);
	// Compat wrappers preserved so existing call sites keep working.
	assert.match(cdn, /export function resolveCdnUrl/);
	assert.match(cdn, /export function ossImageUrl/);
});
