import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const analyticsPath = new URL(
	"../lib/analytics/Analytics.tsx",
	import.meta.url,
);

test("navigation lifecycle stays mounted while GA waits for document hydration", async () => {
	const source = await readFile(analyticsPath, "utf8");

	assert.match(source, /const \[documentSettled, setDocumentSettled\]/);
	assert.match(source, /bootstrapGtag\(gaId\)/);
	assert.match(source, /whenDocumentSettled\(\)\.then/);
	assert.doesNotMatch(source, /if \(!gaId\) return null/);
	assert.match(
		source,
		/<Suspense fallback=\{null\}>[\s\S]*<NavigationEvents \/>[\s\S]*gaId && documentSettled \? \(/,
	);
});
