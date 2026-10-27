import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const projectFile = (path: string) => new URL(`../${path}`, import.meta.url);

test("home merchants rail gates its viewport prefetch on connection quality", async () => {
	const source = await readFile(
		projectFile("features/home/components/HomeCompactMerchants.tsx"),
		"utf8",
	);

	assert.match(
		source,
		/import \{ canPrefetchMainNav \} from "@\/lib\/navigation\/main-nav-prefetch";/,
	);

	const observerCallback = source.match(
		/new IntersectionObserver\(\s*\(entries\) => \{([\s\S]*?)\n\t\t\t\},/,
	)?.[1];
	assert.ok(observerCallback, "IntersectionObserver callback body not found");
	assert.match(observerCallback, /canPrefetchMainNav\("idle"\)/);

	// The gate must run before router.prefetch, not after — a check that
	// only skips a prefetch already in flight defeats the point.
	const gateIndex = observerCallback.indexOf('canPrefetchMainNav("idle")');
	const prefetchIndex = observerCallback.indexOf("router.prefetch(href)");
	assert.ok(gateIndex >= 0 && prefetchIndex >= 0 && gateIndex < prefetchIndex);
});
