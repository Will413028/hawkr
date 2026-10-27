import { withSentryConfig } from "@sentry/nextjs";
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
	"script-src 'self'",
	"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com",
	"font-src 'self' https://fonts.gstatic.com",
	// image-gateway（R2 + Cloudflare Worker）是唯一圖片來源，origin 取自
	// NEXT_PUBLIC_MEDIA_BASE_URL；沒設就只剩 'self' 與 data:。
	["img-src 'self' data:", MEDIA_ORIGIN].filter(Boolean).join(" "),
	"connect-src 'self'",
	"frame-ancestors 'none'",
	"base-uri 'self'",
	"form-action 'self'",
	"object-src 'none'",
	// Collects violations at app/api/csp-report/route.ts → Sentry.
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
 * paired expectation in `tests/csp-report.test.ts` plus the smoke mode passed
 * by `.github/workflows/deploy.yml`.
 */
const CSP_REPORT_ONLY = true;

const nextConfig: NextConfig = {
	// @hawkr/api 只發佈 TS 原始碼（不 build），由 Next 編譯。
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
			// 後台無嵌入需求，直接擋 iframe embedding。
			{ key: "X-Frame-Options", value: "DENY" },
			{
				key: "Permissions-Policy",
				value: "camera=(), microphone=(), geolocation=()",
			},
		];
		// 只在 production 送出，避免 dev（HMR/eval）洗報告雜訊。
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
	// 上傳 source map 的目標由 build env SENTRY_ORG、SENTRY_PROJECT 決定；
	// 環境靠 Sentry `environment` tag 分，所以只需要一個 project。
	org: process.env.SENTRY_ORG,
	project: process.env.SENTRY_PROJECT,
	// Source maps 只在 SENTRY_AUTH_TOKEN 存在時上傳（CI build）。無 token 時
	// build 照成功、跳過 upload。
	silent: true,
	telemetry: false,
	widenClientFileUpload: false,
	// client event 改走自家同源路徑 → 規避廣告/追蹤封鎖器擋 *.sentry.io。
	// 固定路徑讓 CSP 與同源轉發設定保持一致。
	// 路徑避開 sentry/monitoring/tunnel 等封鎖器關鍵字。轉發靠 Next external rewrite，
	// OpenNext 已預設 proxyExternalRequest:"fetch"（workerd 唯一可用），不需動 open-next.config。
	tunnelRoute: "/hkx-relay",
});
