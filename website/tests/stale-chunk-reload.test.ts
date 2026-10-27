import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import {
	STALE_CHUNK_RELOAD_BOOT_SCRIPT,
	STALE_CHUNK_RELOAD_CLASS,
	STALE_CHUNK_RELOAD_MARKER_KEY,
	STALE_CHUNK_RELOAD_PENDING_CLASS,
} from "../lib/loading/stale-chunk-reload";

function runBootScript(markerAt: number) {
	const values = new Map<string, string>([
		[STALE_CHUNK_RELOAD_MARKER_KEY, String(markerAt)],
	]);
	const classes = new Set<string>();
	const window = {
		sessionStorage: {
			getItem: (key: string) => values.get(key) ?? null,
			removeItem: (key: string) => values.delete(key),
		},
	};
	const document = {
		documentElement: {
			classList: {
				add: (...names: string[]) => {
					for (const name of names) classes.add(name);
				},
			},
		},
	};

	new Function("window", "document", STALE_CHUNK_RELOAD_BOOT_SCRIPT)(
		window,
		document,
	);

	return { classes, values };
}

test("fresh stale-chunk marker skips the branded splash before first paint", () => {
	const { classes, values } = runBootScript(Date.now());

	assert.equal(classes.has(STALE_CHUNK_RELOAD_CLASS), true);
	assert.equal(classes.has(STALE_CHUNK_RELOAD_PENDING_CLASS), true);
	assert.equal(values.has(STALE_CHUNK_RELOAD_MARKER_KEY), false);
});

test("expired stale-chunk marker is consumed without changing first-visit UI", () => {
	const { classes, values } = runBootScript(Date.now() - 120_000);

	assert.equal(classes.size, 0);
	assert.equal(values.has(STALE_CHUNK_RELOAD_MARKER_KEY), false);
});

test("stale-chunk recovery keeps its reload guard and quiet UI handoff wired", async () => {
	const [instrumentation, layout, loader, styles] = await Promise.all([
		readFile(new URL("../instrumentation-client.ts", import.meta.url), "utf8"),
		readFile(
			new URL("../components/shared/RootDocument.tsx", import.meta.url),
			"utf8",
		),
		readFile(
			new URL("../components/shared/InitialPageLoader.tsx", import.meta.url),
			"utf8",
		),
		readFile(
			new URL("../styles/components/decorations-overlays.css", import.meta.url),
			"utf8",
		),
	]);

	assert.match(instrumentation, /STALE_CHUNK_RELOAD_MARKER_KEY/);
	assert.match(instrumentation, /window\.location\.reload\(\)/);
	assert.match(layout, /STALE_CHUNK_RELOAD_BOOT_SCRIPT/);
	assert.match(layout, /<script[\s\S]*id="hk-stale-chunk-reload-boot"/);
	// Boot scripts intentionally stay native and inline. next/script serializes
	// beforeInteractive entries into __next_s and emits preload bookkeeping for
	// scripts that are already inline, which can both delay the earliest guards
	// and create misleading "preloaded but not used" browser warnings.
	assert.doesNotMatch(layout, /from "next\/script"/);
	assert.doesNotMatch(layout, /strategy="beforeInteractive"/);
	assert.match(layout, /<script[\s\S]*id="hk-name-shim"/);
	assert.match(loader, /isStaleChunkReload/);
	assert.match(loader, /STALE_CHUNK_RELOAD_PENDING_CLASS/);
	assert.match(styles, /\.hk-stale-chunk-reload \.initial-loader/);
	assert.match(styles, /\.hk-stale-chunk-reload-pending body::after/);
});
