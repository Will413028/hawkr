import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { mock, test } from "node:test";
import { fileURLToPath } from "node:url";
import {
	buildContentSecurityPolicy,
	type ContentSecurityPolicyInput,
	parseUploadOrigins,
	uniqueOrigins,
} from "../lib/security/csp";

const testDir = dirname(fileURLToPath(import.meta.url));
const projectRoot = dirname(testDir);

const APP_ORIGINS = ["https://api.example.org"];

function policy(overrides: Partial<ContentSecurityPolicyInput> = {}): string {
	return buildContentSecurityPolicy({
		appConnectOrigins: APP_ORIGINS,
		uploadOrigins: [],
		...overrides,
	});
}

function directive(value: string, name: string): string[] {
	const found = value
		.split("; ")
		.find((entry) => entry === name || entry.startsWith(`${name} `));
	assert.ok(found, `policy has no ${name} directive`);
	return found.split(" ").slice(1);
}

test("CSP ships a report-uri pointing at the collection endpoint", () => {
	const value = policy();
	assert.deepEqual(directive(value, "report-uri"), ["/api/csp-report"]);
	assert.ok(directive(value, "script-src").includes("'report-sample'"));
});

test("the policy omits upgrade-insecure-requests; only the enforcing header adds it", () => {
	assert.doesNotMatch(policy(), /upgrade-insecure-requests/);
	const config = readFileSync(join(projectRoot, "next.config.ts"), "utf8");
	assert.match(
		config,
		/IS_E2E_HARNESS\s*\?\s*CONTENT_SECURITY_POLICY\s*:\s*`\$\{CONTENT_SECURITY_POLICY\}; upgrade-insecure-requests`/,
	);
});

test("CSP mode is env-driven and defaults to report-only when unset", () => {
	const config = readFileSync(join(projectRoot, "next.config.ts"), "utf8");
	assert.match(
		config,
		/const CSP_REPORT_ONLY = process\.env\.CSP_MODE !== "enforce";/,
	);
});

test("connect-src carries the configured upload origins and nothing wider", () => {
	const origins = parseUploadOrigins(
		"https://*.uploads.example.org, https://uploads.example.org",
	);
	const connect = directive(policy({ uploadOrigins: origins }), "connect-src");
	assert.ok(connect.includes("https://*.uploads.example.org"));
	assert.ok(connect.includes("https://uploads.example.org"));
});

test("without an upload origin the policy does not widen to a wildcard or a bare scheme", () => {
	const value = policy();
	for (const name of [
		"connect-src",
		"script-src",
		"frame-src",
		"default-src",
	]) {
		for (const source of directive(value, name)) {
			assert.notEqual(source, "*", `${name} must not allow *`);
			assert.doesNotMatch(source, /^(https?|wss?|data|blob):$/, name);
		}
	}
	assert.doesNotMatch(value, /cloudflarestorage/);
	assert.ok(
		!directive(value, "connect-src").some((source) =>
			source.startsWith("https://*.uploads"),
		),
	);
});

test("every directive is well formed, with and without optional origins", () => {
	for (const value of [
		policy(),
		policy({
			appConnectOrigins: [],
			uploadOrigins: [],
		}),
		policy({ uploadOrigins: ["https://*.uploads.example.org"] }),
	]) {
		assert.doesNotMatch(value, /\s{2,}/, "no empty source left as a gap");
		assert.doesNotMatch(value, /;;|; ;|undefined|null/);
		for (const entry of value.split("; ")) {
			assert.match(entry, /^[a-z-]+( \S+)*$/, entry);
		}
	}
});

test("a malformed upload origin fails the build instead of widening the policy", () => {
	for (const bad of [
		"https:",
		"*",
		"https://*",
		"http://uploads.example.org",
		"https://uploads.example.org/path",
		"https://a.*.example.org",
		"https://*.*.example.org",
	]) {
		assert.throws(() => parseUploadOrigins(bad), /MEDIA_UPLOAD_ORIGINS/, bad);
	}
	assert.deepEqual(parseUploadOrigins(undefined), []);
	assert.deepEqual(parseUploadOrigins("  "), []);
});

test("connect-src takes the app's own origins from its input, deduplicated, with no websocket origins", () => {
	const origins = uniqueOrigins([
		"https://api.example.org",
		"",
		"https://media.example.org",
		"https://api.example.org",
	]);
	assert.deepEqual(origins, [
		"https://api.example.org",
		"https://media.example.org",
	]);
	const connect = directive(
		policy({ appConnectOrigins: origins }),
		"connect-src",
	);
	for (const origin of origins) assert.ok(connect.includes(origin), origin);
	assert.deepEqual(
		directive(policy({ appConnectOrigins: [] }), "connect-src").filter((s) =>
			s.includes("api.example.org"),
		),
		[],
	);
});

test("CSP allows the Payment Element hosts and none of the native wallet hosts", () => {
	const value = policy();
	assert.ok(directive(value, "connect-src").includes("https://api.stripe.com"));
	assert.ok(directive(value, "frame-src").includes("https://hooks.stripe.com"));
	assert.ok(directive(value, "frame-src").includes("https://js.stripe.com"));
	assert.doesNotMatch(
		value,
		/pay\.google\.com|applepay\.cdn-apple\.com|smp-paymentservices\.apple\.com/,
	);
});

test("CSP carries no Clerk or Clerk bot-protection origin", () => {
	const value = policy();
	assert.doesNotMatch(value, /clerk/i);
	assert.doesNotMatch(value, /challenges\.cloudflare\.com/);
});

test("enforced CSP carries the required hardening directives", () => {
	const value = policy();
	assert.deepEqual(directive(value, "object-src"), ["'none'"]);
	assert.deepEqual(directive(value, "base-uri"), ["'self'"]);
	assert.deepEqual(directive(value, "frame-ancestors"), ["'self'"]);
});

const captured: unknown[] = [];
mock.module("@sentry/nextjs", {
	namedExports: {
		captureMessage: (message: string, context: unknown) => {
			captured.push([message, context]);
		},
	},
});
const { POST } = await import("../app/api/csp-report/route");

test("csp-report endpoint forwards a violation report to Sentry and responds 204", async () => {
	captured.length = 0;
	const res = await POST(
		new Request("https://example.com/api/csp-report", {
			method: "POST",
			body: JSON.stringify({
				"csp-report": { "violated-directive": "script-src" },
			}),
		}) as never,
	);
	assert.equal(res.status, 204);
	assert.equal(captured.length, 1);
});

test("csp-report endpoint tolerates a malformed body without throwing or reporting", async () => {
	captured.length = 0;
	const res = await POST(
		new Request("https://example.com/api/csp-report", {
			method: "POST",
			body: "not json",
		}) as never,
	);
	assert.equal(res.status, 204);
	assert.equal(captured.length, 0);
});

test("csp-report endpoint ignores Sentry envelope self-reports", async () => {
	captured.length = 0;
	const res = await POST(
		new Request("https://example.com/api/csp-report", {
			method: "POST",
			body: JSON.stringify({
				"csp-report": {
					"blocked-uri": "https://o0.ingest.us.sentry.io/api/123/envelope/",
				},
			}),
		}) as never,
	);
	assert.equal(res.status, 204);
	assert.equal(captured.length, 0);
});

test("csp-report endpoint reports a same-origin eval whose sample is not a known probe", async () => {
	captured.length = 0;
	const res = await POST(
		new Request("https://example.com/api/csp-report", {
			method: "POST",
			body: JSON.stringify({
				"csp-report": {
					"effective-directive": "script-src",
					"blocked-uri": "eval",
					"source-file": "https://example.com/_next/static/chunks/chunk.js",
					"script-sample": "function unknownProbe(){",
				},
			}),
		}) as never,
	);
	assert.equal(res.status, 204);
	assert.equal(captured.length, 1);
});

test("csp-report endpoint ignores the zod allowsEval capability probe", async () => {
	captured.length = 0;
	const res = await POST(
		new Request("https://example.com/api/csp-report", {
			method: "POST",
			body: JSON.stringify({
				"csp-report": {
					"effective-directive": "script-src",
					"blocked-uri": "eval",
					"source-file":
						"https://example.com/_next/static/chunks/3q0xh5t3wrtbq.js",
					"script-sample": "(function anonymous(\n) {\n\n})",
				},
			}),
		}) as never,
	);
	assert.equal(res.status, 204);
	assert.equal(captured.length, 0);
});

test("csp-report endpoint still reports an anonymous-function eval from an external source", async () => {
	captured.length = 0;
	const res = await POST(
		new Request("https://example.com/api/csp-report", {
			method: "POST",
			body: JSON.stringify({
				"csp-report": {
					"effective-directive": "script-src",
					"blocked-uri": "eval",
					"source-file": "https://evil.example/_next/static/chunks/x.js",
					"script-sample": "(function anonymous(\n) {\n\n})",
				},
			}),
		}) as never,
	);
	assert.equal(res.status, 204);
	assert.equal(captured.length, 1);
});

test("csp-report endpoint still reports eval from an external source", async () => {
	captured.length = 0;
	const res = await POST(
		new Request("https://example.com/api/csp-report", {
			method: "POST",
			body: JSON.stringify([
				{
					body: {
						effectiveDirective: "script-src",
						blockedURL: "eval",
						sourceFile: "https://unexpected.example/injected.js",
					},
				},
			]),
		}) as never,
	);
	assert.equal(res.status, 204);
	assert.equal(captured.length, 1);
	assert.deepEqual(captured[0], [
		"csp violation",
		{
			level: "warning",
			tags: {
				csp_directive: "script-src",
				csp_blocked_uri: "eval",
				csp_source_path: "/injected.js",
			},
			extra: {
				report: [
					{
						body: {
							effectiveDirective: "script-src",
							blockedURL: "eval",
							sourceFile: "https://unexpected.example/injected.js",
						},
					},
				],
			},
		},
	]);
});

// webpack 打包進 @editorjs/image 的 global polyfill：new Function("return this")，
test("csp-report endpoint ignores the bundled `return this` global probe", async () => {
	captured.length = 0;
	const res = await POST(
		new Request("https://example.com/api/csp-report", {
			method: "POST",
			body: JSON.stringify({
				"csp-report": {
					"effective-directive": "script-src",
					"blocked-uri": "eval",
					"source-file":
						"https://example.com/_next/static/chunks/1r7rl4vcsyrqq.js",
					"script-sample": "(function anonymous(\n) {\nreturn this\n})",
				},
			}),
		}) as never,
	);
	assert.equal(res.status, 204);
	assert.equal(captured.length, 0);
});

test("csp-report endpoint still reports an eval with a real function body", async () => {
	captured.length = 0;
	const res = await POST(
		new Request("https://example.com/api/csp-report", {
			method: "POST",
			body: JSON.stringify({
				"csp-report": {
					"effective-directive": "script-src",
					"blocked-uri": "eval",
					"source-file":
						"https://example.com/_next/static/chunks/1r7rl4vcsyrqq.js",
					"script-sample":
						"(function anonymous(\n) {\nreturn fetch('//evil')\n})",
				},
			}),
		}) as never,
	);
	assert.equal(res.status, 204);
	assert.equal(captured.length, 1);
});
