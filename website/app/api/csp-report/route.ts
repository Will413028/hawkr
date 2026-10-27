import * as Sentry from "@sentry/nextjs";
import type { NextRequest } from "next/server";

/**
 * CSP violation sink. The browser POSTs here on every blocked/would-be-blocked
 * resource (see the `report-uri` directive in next.config.ts) while
 * CSP_REPORT_ONLY is true. Content-Type varies by browser
 * (`application/csp-report` or `application/reports+json`) but both bodies are
 * JSON, so `.json()` handles either without inspecting the header.
 */
export async function POST(req: NextRequest) {
	const report = await req.json().catch(() => null);
	const envelope = Array.isArray(report) ? report[0] : report;
	const body = envelope?.["csp-report"] ?? envelope?.body ?? {};
	const blockedUri = String(body?.["blocked-uri"] ?? body?.blockedURL ?? "");
	const directive = String(
		body?.["effective-directive"] ??
			body?.effectiveDirective ??
			body?.["violated-directive"] ??
			body?.violatedDirective ??
			"",
	);
	const sourceFile = String(body?.["source-file"] ?? body?.sourceFile ?? "");
	const scriptSample = String(body?.["script-sample"] ?? body?.sample ?? "");
	// Never create a recursive warning when a stale CSP blocks Sentry's own
	// envelope. The allowlist includes this host, but this guard keeps a future
	// config regression from flooding the project with self-referential reports.
	const isSentryEnvelope = /\/\/[^/]*ingest(?:\.[^/]+)?\.sentry\.io\//i.test(
		blockedUri,
	);
	let sourceUrl: URL | null = null;
	try {
		sourceUrl = sourceFile ? new URL(sourceFile) : null;
	} catch {
		sourceUrl = null;
	}
	// 同源 bundle 內兩種「已知且無害」的 eval。enforce 之後它們會被真的擋掉——那正是
	// CSP 該有的行為，兩者都自帶 fallback，不該變成反覆進 Sentry 的事故：
	//
	//   1. zod v4 的 `allowsEval` 能力偵測：`new Function("")`，script-sample 長成
	//      `(function anonymous( ) { })`（空函式體）。它自己包在 try/catch，被擋就
	//      回傳 false 退到非 JIT 的解譯版驗證，驗證結果不變
	//      （zod 4.4.3 `v4/core/util.js` allowsEval；upstream 另提供 `jitless`
	//      設定可完全跳過這支探測，本站尚未設定）。
	//   2. webpack 打包進 `@editorjs/image` 的 `global` polyfill：
	//      `r = r || new Function("return this")()`，script-sample 長成
	//      `(function anonymous( ) { return this })`。原始碼就寫成
	//      `try { … } catch { typeof window == "object" && (r = window) }`
	//      （node_modules/@editorjs/image/dist/image.umd.js），被擋就退到 `window`，
	//      拿到的是同一個全域物件。**它與第 1 項是不同形狀**：函式體非空，所以只用
	//      「空函式體」比對會漏掉——2026-07-29 dev 進 enforce 後這條每天照樣進
	//      Sentry（DEV-19 的 25 筆 script-src），把一個自帶 fallback 的能力偵測
	//      混進了「真的有東西被擋壞」的告警裡。
	//
	// 舊瀏覽器可能不給 script-sample；來源 + directive 仍足以辨識。
	const isEmptyAnonymousFunctionProbe =
		/^\(function anonymous\(\s*\)\s*\{\s*(?:return this;?)?\s*\}\)/.test(
			scriptSample,
		);
	const isKnownBenignEval =
		blockedUri === "eval" &&
		directive.startsWith("script-src") &&
		sourceUrl?.origin === new URL(req.url).origin &&
		sourceUrl.pathname.startsWith("/_next/static/chunks/") &&
		(!scriptSample || isEmptyAnonymousFunctionProbe);

	if (report && !isSentryEnvelope && !isKnownBenignEval) {
		Sentry.captureMessage("csp violation", {
			level: "warning",
			tags: {
				csp_directive: directive || "unknown",
				csp_blocked_uri: blockedUri || "unknown",
				csp_source_path: sourceUrl?.pathname || "unknown",
			},
			extra: { report },
		});
	}
	return new Response(null, { status: 204 });
}
