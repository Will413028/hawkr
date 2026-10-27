import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const viewportPath = new URL(
	"../components/shared/NotificationToast.tsx",
	import.meta.url,
);

test("realtime notification portal waits for document hydration", async () => {
	const source = await readFile(viewportPath, "utf8");

	assert.match(source, /const \[documentSettled, setDocumentSettled\]/);
	assert.match(source, /whenDocumentSettled\(\)\.then/);
	assert.match(source, /if \(!documentSettled \|\| toasts\.length === 0\)/);
	assert.match(source, /createPortal\(/);
});
