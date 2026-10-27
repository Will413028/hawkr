import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import test from "node:test";

const sourcePaths = [
	"../components/shared/RootDocument.tsx",
	"../components/ui/Dialog.tsx",
	"../styles/components/decorations-overlays.css",
	"../styles/foundation/01-tokens-theme.css",
	"../styles/legacy/prd-v2-commerce.css",
];

test("the global shell has no ambient floating-leaf implementation", async () => {
	const sources = await Promise.all(
		sourcePaths.map((path) => readFile(new URL(path, import.meta.url), "utf8")),
	);
	const combined = sources.join("\n");

	assert.doesNotMatch(combined, /FloatingLeaves/);
	assert.doesNotMatch(combined, /orbs-background/);
	assert.doesNotMatch(combined, /floating-leaf/);
	assert.doesNotMatch(combined, /leaf-(?:float|breathe|morph)/);
});

test("the removed floating-leaf asset directory contains no files", async () => {
	const directory = new URL("../public/leaves/", import.meta.url);
	const entries = await readdir(directory).catch(
		(error: NodeJS.ErrnoException) => {
			if (error.code === "ENOENT") return [];
			throw error;
		},
	);

	assert.deepEqual(entries, []);
});
