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

const policy = buildContentSecurityPolicy({
	appConnectOrigins: [],
	uploadOrigins: [],
});

function directive(name: string): string {
	const entry = policy.split("; ").find((e) => e.startsWith(`${name} `));
	assert.ok(entry, `${name} is present`);
	return entry;
}

// Stripe Connect embedded components:
// https://docs.stripe.com/connect/get-started-connect-embedded-components (CSP section)
test("script-src and frame-src allow the Stripe Connect embedded component hosts", () => {
	for (const name of ["script-src", "frame-src"]) {
		const entry = directive(name);
		for (const host of [
			"https://connect-js.stripe.com",
			"https://js.stripe.com",
		]) {
			assert.match(
				entry,
				new RegExp(` ${host.replaceAll(".", "\\.")}(?=[\\s;]|$)`),
				`${name} allows ${host}`,
			);
		}
	}
});

test("the Stripe additions do not weaken the policy: no wildcard script host, unsafe-inline stays, no nonce", () => {
	assert.doesNotMatch(directive("script-src"), /\*\.stripe\.com|nonce-/);
	assert.match(directive("script-src"), /'unsafe-inline'/);
});
