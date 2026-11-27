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
	const isSentryEnvelope = /\/\/[^/]*ingest(?:\.[^/]+)?\.sentry\.io\//i.test(
		blockedUri,
	);
	let sourceUrl: URL | null = null;
	try {
		sourceUrl = sourceFile ? new URL(sourceFile) : null;
	} catch {
		sourceUrl = null;
	}
	//   2. webpack 打包進 `@editorjs/image` 的 `global` polyfill：
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
