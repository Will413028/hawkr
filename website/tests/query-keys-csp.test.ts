import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";
import { queryKeys } from "../lib/query/keys";
import { buildContentSecurityPolicy } from "../lib/security/csp";

const testDir = dirname(fileURLToPath(import.meta.url));
const projectRoot = dirname(testDir);

test("private query keys tolerate missing identity during auth hydration", () => {
	assert.deepEqual(queryKeys.checkout.storePickups(undefined as never), [
		"checkout",
		"anon",
		"none",
		"store-pickups",
		{},
	]);
	assert.deepEqual(queryKeys.checkout.cart(null as never), [
		"checkout",
		"anon",
		"none",
		"cart",
		{},
	]);
});

test("CSP allowlist covers deployed Cloudflare RUM and Apple Pay fonts", () => {
	const policy = buildContentSecurityPolicy({
		appConnectOrigins: ["https://api.example.org"],
		uploadOrigins: [],
		clerkFrontendApi: "",
	});
	const sources = (name: string) =>
		(
			policy.split("; ").find((entry) => entry.startsWith(`${name} `)) ?? ""
		).split(" ");

	assert.ok(
		sources("script-src").includes("https://static.cloudflareinsights.com"),
	);
	assert.ok(sources("connect-src").includes("https://cloudflareinsights.com"));
	assert.ok(sources("font-src").includes("https://applepay.cdn-apple.com"));
	assert.ok(sources("connect-src").includes("https://api.example.org"));
	assert.ok(sources("style-src").includes("https://accounts.google.com"));
	assert.ok(sources("style-src-elem").includes("https://accounts.google.com"));
});
