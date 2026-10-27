import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const barrierPath = new URL(
	"../lib/hydration/document-settled.ts",
	import.meta.url,
);

test("third-party DOM mutations wait for load and the loader handoff", async () => {
	const source = await readFile(barrierPath, "utf8");

	assert.match(source, /let settledPromise: Promise<void> \| null = null/);
	assert.match(source, /document\.readyState !== "complete"/);
	assert.match(source, /window\.addEventListener\("load", trySettle/);
	assert.match(source, /root\.classList\.contains\("initial-loader-active"\)/);
	assert.match(source, /new MutationObserver\(\(\) => trySettle\(\)\)/);
	assert.match(source, /attributeFilter: \["class"\]/);
	assert.match(
		source,
		/window\.requestAnimationFrame\(\(\) => \{[\s\S]*window\.requestAnimationFrame/,
	);
});
