import assert from "node:assert/strict";
import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const ROOT = fileURLToPath(new URL("..", import.meta.url));

function sourceFiles(dir: string): string[] {
	return readdirSync(join(ROOT, dir), { withFileTypes: true }).flatMap((e) => {
		const path = join(dir, e.name);
		if (e.isDirectory()) return sourceFiles(path);
		return /\.(ts|tsx)$/.test(path) && !/\.(unit\.)?test\.tsx?$/.test(path)
			? [path]
			: [];
	});
}

const ROOTS = ["app", "components", "features", "lib", "stories"];

test("no source reintroduces an isLoggedIn boolean", () => {
	const offenders = ROOTS.flatMap(sourceFiles).filter((file) =>
		/\bisLoggedIn\b/.test(readFileSync(join(ROOT, file), "utf8")),
	);
	assert.deepEqual(
		offenders,
		[],
		"A signed-in boolean is false for both `unknown` and `guest`, so it shows the guest view before the server has answered. Branch on `status`, or use `accountQueryFn` for account queries (lib/auth/account-query.ts).",
	);
});

const AUTH_WORD = String.raw`\b(?:status|identity(?!\.activeRole)|signedIn|authStatus)\b`;
const AUTH_IN_ENABLED = new RegExp(
	String.raw`\benabled:[^,}]{0,120}?${AUTH_WORD}`,
	"g",
);
const AUTH_IN_ENABLED_CONST = new RegExp(
	String.raw`\bconst enabled\b[^;]{0,120}?${AUTH_WORD}`,
	"g",
);

/**
 * Not account queries: a live stream that must not open before sign-in, and a lookup that only waits for
 * hydration and runs for guests too. Anything else gates through `accountQueryFn`.
 */
const ENABLED_EXEMPT: Record<string, string> = {
	"lib/chat/unread-context.tsx":
		"unread-count stream (WebSocket), not a query; opens only when signedIn",
	"features/after-sales/screens/RefundResultClient.tsx":
		"order lookup by public order number; runs for guests too and only waits for hydration",
};

function collapsed(file: string): string {
	return readFileSync(join(ROOT, file), "utf8").replace(/\s+/g, " ");
}

function authEnabledOffenders(pattern: RegExp): string[] {
	const offenders: string[] = [];
	for (const file of ROOTS.flatMap(sourceFiles)) {
		if (file in ENABLED_EXEMPT) continue;
		for (const match of collapsed(file).matchAll(pattern)) {
			offenders.push(`${file}: ${match[0]}`);
		}
	}
	return offenders;
}

test("the scan sees `enabled:` on auth state written over several lines", () => {
	const source =
		"useQuery({\n\tenabled:\n\t\tBoolean(id) &&\n\t\tstatus === 'signedIn',\n})";
	assert.equal(source.replace(/\s+/g, " ").match(AUTH_IN_ENABLED)?.length, 1);
	assert.equal(
		"const enabled =\n\tBoolean(x) &&\n\tsignedIn;"
			.replace(/\s+/g, " ")
			.match(AUTH_IN_ENABLED_CONST)?.length,
		1,
	);
});

test("exempt files still exist and still contain what the exemption excuses", () => {
	for (const file of Object.keys(ENABLED_EXEMPT)) {
		assert.match(collapsed(file), AUTH_IN_ENABLED, file);
	}
});

test("no source gates a query with `enabled:` on auth state", () => {
	assert.deepEqual(
		authEnabledOffenders(AUTH_IN_ENABLED),
		[],
		"An `enabled:` on auth state leaves the query not-pending and empty while status is `unknown`. Pass the fetcher through `accountQueryFn(status, fn)` instead; a stream that is not a query goes in ENABLED_EXEMPT with its reason.",
	);
});

test("no source builds an `enabled` boolean from auth state", () => {
	assert.deepEqual(authEnabledOffenders(AUTH_IN_ENABLED_CONST), []);
});
