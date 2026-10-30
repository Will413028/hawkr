import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { mock, test } from "node:test";
import { fileURLToPath } from "node:url";
import {
	buildContentSecurityPolicy,
	type ContentSecurityPolicyInput,
	parseUploadOrigins,
	withWebSocketSiblings,
} from "../lib/security/csp";

const testDir = dirname(fileURLToPath(import.meta.url));
const projectRoot = dirname(testDir);

const APP_ORIGINS = ["https://api.example.org", "wss://api.example.org"];

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

// 模式改為「每環境獨立」：由 build 期的 CSP_MODE 決定，未設值 fail-safe 回
// report-only。這道測試釘住 fail-safe 預設，避免有人把它改成「未設值即攔截」，
// 讓本機 build / CI E2E harness 意外進入攔截模式。
test("CSP mode is env-driven and defaults to report-only when unset", () => {
	const config = readFileSync(join(projectRoot, "next.config.ts"), "utf8");
	assert.match(
		config,
		/const CSP_REPORT_ONLY = process\.env\.CSP_MODE !== "enforce";/,
	);
});

// Presigned-upload PUTs go to a deploy-specific origin (MEDIA_UPLOAD_ORIGINS): it is on
// connect-src when configured, and nothing stands in for it when it is not.
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

// API gateway、media gateway 與 Sentry ingest 的 origin 取自 app 讀的同一組 env，
// connect-src 不寫死任何自家主機；env 沒設的來源就不出現在清單裡。
test("connect-src takes the app's own origins from its input and lists a ws sibling", () => {
	const origins = withWebSocketSiblings([
		"https://api.example.org",
		"",
		"https://media.example.org",
		"https://api.example.org",
	]);
	assert.deepEqual(origins, [
		"https://api.example.org",
		"wss://api.example.org",
		"https://media.example.org",
		"wss://media.example.org",
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

// Apple Pay JS SDK 的能力偵測（applePayCapabilities → checkStatus/merchant）。
// 拿掉它 enforce 會擋掉這支請求，結帳頁在第三方瀏覽器的 Apple Pay 判定會靜默退化
// 成 legacy 分支——沒有錯誤訊息、只有錢收不到，所以用測試釘住。
test("connect-src lists the Apple Pay capability endpoint", () => {
	assert.ok(
		directive(policy(), "connect-src").includes(
			"https://smp-paymentservices.apple.com",
		),
	);
});

// Apple Pay JS SDK 在第三方瀏覽器（非 Safari 桌機）的掃碼流程用
// applepay.cdn-apple.com 渲染付款 sheet 的 iframe（lib/payment/apple-pay.ts
// 的 sdk-session 路線）。拿掉它 enforce 會直接擋掉該 iframe——結帳頁 Apple Pay
// 永遠顯示空白/破圖，訂單只能「已取消」。2026-08-13 web-dev 實測攔到（console:
// Framing 'https://applepay.cdn-apple.com/' violates ... frame-src），用測試釘住。
test("frame-src lists the Apple Pay sheet iframe host", () => {
	assert.ok(
		directive(policy(), "frame-src").includes("https://applepay.cdn-apple.com"),
	);
});

// 登入改由單體發起、整頁導向 provider（302），瀏覽器不再載入第三方登入 SDK、也不開 bot protection 的 iframe：
// CSP 不該為登入留任何 Clerk 或 challenges.cloudflare.com 的來源。
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

// zod v4 的 allowsEval 能力偵測（new Function("")）。enforce 之後每個跑到 zod 驗證
// 的頁面都會擋掉它一次；它自己包 try/catch 退回解譯版驗證，行為不變，不該進 Sentry。
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

// 同樣長相但來自外部來源＝可能是注入，必須照報。
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
// 原始碼帶 catch → window 的 fallback，被擋之後行為不變。函式體非空，所以它跟上面
// 那條 zod 空函式體探測是不同形狀——只比對空函式體會讓它天天進 Sentry。
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

// 放寬只到「已知會自癒的探測」為止：同一支 chunk 裡任何帶真實函式體的 eval 仍要照報，
// 否則這個過濾器就從「降噪」變成「把注入藏起來」。
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
