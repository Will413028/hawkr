import assert from "node:assert/strict";
import { test } from "node:test";
import { buildContentSecurityPolicy } from "../lib/security/csp";

const connectSrc = buildContentSecurityPolicy({
	appConnectOrigins: [],
	uploadOrigins: [],
})
	.split("; ")
	.find((entry) => entry.startsWith("connect-src "));
assert.ok(connectSrc);

test("connect-src allowlists the exact Google Signals host", () => {
	// Leading space and the terminator lookahead keep a lookalike such as
	// stats.g.doubleclick.net.attacker.test from satisfying the assertion.
	assert.match(connectSrc, / https:\/\/stats\.g\.doubleclick\.net(?=[\s;"]|$)/);
});
