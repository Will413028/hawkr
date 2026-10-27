import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const rootLayoutPath = new URL(
	"../components/shared/RootDocument.tsx",
	import.meta.url,
);

test("root bootstrap scripts are the first React-owned body nodes", async () => {
	const source = await readFile(rootLayoutPath, "utf8");

	assert.doesNotMatch(source, /from "next\/script"/);
	assert.doesNotMatch(source, /strategy="beforeInteractive"/);
	const bodyIndex = source.indexOf("<body>");
	const providerIndex = source.indexOf("<ViewTransitions>");
	assert.ok(bodyIndex >= 0);
	assert.ok(providerIndex > bodyIndex);
	for (const id of [
		"hk-name-shim",
		"hk-theme-boot",
		"hk-initial-loader-watchdog",
		"hk-stale-chunk-reload-boot",
		"hk-image-drag-guard",
	]) {
		assert.match(
			source,
			new RegExp(`<script\\s+id="${id}"`),
			`${id} must be a native body script`,
		);
		const scriptIndex = source.indexOf(`id="${id}"`);
		assert.ok(scriptIndex > bodyIndex, `${id} must follow <body>`);
		assert.ok(
			scriptIndex < providerIndex,
			`${id} must run before visual body content`,
		);
	}
});
