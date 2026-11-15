import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";

const installed = JSON.parse(
	readFileSync(
		new URL("../node_modules/msw/package.json", import.meta.url),
		"utf8",
	),
) as { version: string };

function workerPackageVersion(source: string): string | undefined {
	return /const PACKAGE_VERSION = '([^']+)'/.exec(source)?.[1];
}

test("tests/browser/mockServiceWorker.js matches the installed msw version", () => {
	const worker = readFileSync(
		new URL("./browser/mockServiceWorker.js", import.meta.url),
		"utf8",
	);
	assert.equal(workerPackageVersion(worker), installed.version);
});
