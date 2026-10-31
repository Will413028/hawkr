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

test("CSP allowlist covers Apple Pay fonts and the API origins", () => {
	const policy = buildContentSecurityPolicy({
		appConnectOrigins: ["https://api.example.org"],
		uploadOrigins: [],
	});
	const sources = (name: string) =>
		(
			policy.split("; ").find((entry) => entry.startsWith(`${name} `)) ?? ""
		).split(" ");

	assert.ok(
		!policy.includes("cloudflareinsights"),
		"Cloudflare Web Analytics is not part of the Vercel deployment",
	);
	assert.ok(sources("font-src").includes("https://applepay.cdn-apple.com"));
	assert.ok(sources("connect-src").includes("https://api.example.org"));
	assert.ok(sources("style-src").includes("https://accounts.google.com"));
	assert.ok(sources("style-src-elem").includes("https://accounts.google.com"));
});
