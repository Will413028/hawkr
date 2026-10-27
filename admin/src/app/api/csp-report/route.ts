import * as Sentry from "@sentry/nextjs";
import type { NextRequest } from "next/server";

/**
 * CSP violation sink. The browser POSTs here on every blocked/would-be-blocked
 * resource (see the `report-uri` directive in next.config.ts). Content-Type
 * varies by browser (`application/csp-report` or `application/reports+json`)
 * but both bodies are JSON, so `.json()` handles either without inspecting
 * the header.
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
	let sourcePath = "unknown";
	try {
		sourcePath = sourceFile ? new URL(sourceFile).pathname : "unknown";
	} catch {
		sourcePath = "unknown";
	}

	if (report) {
		Sentry.captureMessage("csp violation", {
			level: "warning",
			tags: {
				csp_directive: directive || "unknown",
				csp_blocked_uri: blockedUri || "unknown",
				csp_source_path: sourcePath,
			},
			extra: { report },
		});
	}
	return new Response(null, { status: 204 });
}
