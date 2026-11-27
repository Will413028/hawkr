import { withSentryConfig } from "@sentry/nextjs/config";
import type { NextConfig } from "next";

function originOf(value: string | undefined): string {
	try {
		return value?.trim() ? new URL(value.trim()).origin : "";
	} catch {
		return "";
	}
}

/** image-gateway origin from the same env value the app reads; unset adds nothing. */
const MEDIA_ORIGIN = originOf(process.env.NEXT_PUBLIC_MEDIA_BASE_URL);

/**
 * Content-Security-Policy (production only).
 */
const CONTENT_SECURITY_POLICY = [
	"default-src 'self'",
	"script-src 'self' 'wasm-unsafe-eval'",
	"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com",
	"font-src 'self' https://fonts.gstatic.com",
	["img-src 'self' data: blob:", MEDIA_ORIGIN].filter(Boolean).join(" "),
	"connect-src 'self'",
	"frame-ancestors 'none'",
	"base-uri 'self'",
	"form-action 'self'",
	"object-src 'none'",
	"report-uri /api/csp-report",
].join("; ");

/**
 * Rollout switch — kept as a kill switch mirroring hawkr-website's
 * pattern (flip back to `true` for a fast rollback). `upgrade-insecure-
 * requests` is added only while enforcing; browsers ignore it (and log
 * console noise) under report-only.
 *
 * ROLLED BACK to report-only: enforcement blocked every image upload. The
 * three-step media flow (`src/services/media-upload.ts`, and the same flow
 * inlined in `src/app/admin/ai-styles/page.tsx`) PUTs the file straight from
 * the browser to the presigned URL the backend returns
 * (`https://<bucket>.<account>.r2.cloudflarestorage.com/…`),
 * and `connect-src` above only allows `'self'` — enough for the BFF proxy,
 * but not for that cross-origin PUT. hawkr-website hit the identical
 * gap. Flip back to `false` once `connect-src` allows the R2 endpoint (both
 * the virtual-hosted `*.<account>.r2.cloudflarestorage.com` and the
 * path-style `<account>.r2.cloudflarestorage.com` forms) — and revert the
 * paired expectation in `tests/csp-report.test.ts`.
 */
const CSP_REPORT_ONLY = true;

const nextConfig: NextConfig = {
	transpilePackages: [
		"@hawkr/api",
		"@hawkr/icons",
		"@hawkr/twaddr",
		"@hawkr/ui-text",
	],
	poweredByHeader: false,
	async headers() {
		const headers = [
			{
				key: "Cross-Origin-Opener-Policy",
				value: "same-origin-allow-popups",
			},
			{ key: "X-Content-Type-Options", value: "nosniff" },
			{ key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
			{ key: "X-Frame-Options", value: "DENY" },
			{
				key: "Permissions-Policy",
				value: "camera=(), microphone=(), geolocation=()",
			},
		];
		if (process.env.NODE_ENV === "production") {
			headers.push({
				key: CSP_REPORT_ONLY
					? "Content-Security-Policy-Report-Only"
					: "Content-Security-Policy",
				value: CSP_REPORT_ONLY
					? CONTENT_SECURITY_POLICY
					: `${CONTENT_SECURITY_POLICY}; upgrade-insecure-requests`,
			});
		}
		return [{ source: "/(.*)", headers }];
	},
	async redirects() {
		return [
			{
				source: "/admin/certification-audit",
				destination: "/admin/merchants",
				permanent: true,
			},
			{
				source: "/admin/certification-audit/:id",
				destination: "/admin/merchants",
				permanent: true,
			},
			{
				source: "/merchant/certification",
				destination: "/admin/merchants",
				permanent: true,
			},
			{
				source: "/merchant/certification/:id",
				destination: "/admin/merchants",
				permanent: true,
			},
			{
				source: "/admin/system-monitor",
				destination: "/admin",
				permanent: true,
			},
			{
				source: "/admin/admin-accounts",
				destination: "/admin",
				permanent: true,
			},
			{
				source: "/admin/sys-log",
				destination: "/admin",
				permanent: true,
			},
		];
	},
};

export default withSentryConfig(nextConfig, {
	org: process.env.SENTRY_ORG,
	project: process.env.SENTRY_PROJECT,
	silent: true,
	telemetry: false,
	widenClientFileUpload: false,
	tunnelRoute: "/hkx-relay",
});
