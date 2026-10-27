import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { isLoopbackBase } from "./setup/loopback.mjs";

const workflow = readFileSync(
	fileURLToPath(
		new URL("../../.github/workflows/website-ci.yml", import.meta.url),
	),
	"utf8",
);

/** `KEY: value` lines of the workflow's top-level `env:` block (every job and step inherits it). */
function topLevelEnv(): Record<string, string> {
	const block = /^env:\n((?:[ \t]+.*\n|[ \t]*\n)+)/m.exec(workflow)?.[1] ?? "";
	const env: Record<string, string> = {};
	for (const line of block.split("\n")) {
		const match = /^\s+([A-Z][A-Z0-9_]*):\s*'?([^'#\s]*)'?\s*(?:#.*)?$/.exec(
			line,
		);
		if (match) env[match[1]] = match[2];
	}
	return env;
}

/**
 * website-ci.yml builds the site and runs its tests. Unset, the dev flavor's API base is the local monolith
 * (lib/config/env.ts DEV_GATEWAY_HOST): `next build` prerenders against it, and an unstubbed request in a test
 * goes there. The job pins both bases to a loopback address; this keeps a later edit of the workflow from
 * dropping that (the tests' own guard, tests/setup/hermetic.mjs, covers the test processes, not the build).
 */
test("website CI pins both API bases to a loopback address for every step", () => {
	const env = topLevelEnv();
	for (const name of ["API_BASE_URL", "NEXT_PUBLIC_API_BASE_URL"]) {
		assert.ok(
			env[name],
			`${name} is not set in website-ci.yml's top-level env`,
		);
		assert.ok(
			isLoopbackBase(env[name]),
			`${name}=${env[name]} is not a loopback address`,
		);
	}
});

test("no step of website CI sets either API base to something else", () => {
	const assignments = [
		...workflow.matchAll(/\b(API_BASE_URL|NEXT_PUBLIC_API_BASE_URL):\s*(\S+)/g),
	].map((match) => match[2].replace(/['"]/g, ""));
	assert.ok(assignments.length >= 2);
	for (const value of assignments) {
		assert.ok(isLoopbackBase(value), `${value} is not a loopback address`);
	}
});

test("website CI builds with that env and checks the build's output", () => {
	assert.match(workflow, /^\s+- run: pnpm run build$/m);
	assert.match(workflow, /run: pnpm run test:build-output$/m);
	assert.match(workflow, /^\s+- run: pnpm run test:unit$/m);
});
