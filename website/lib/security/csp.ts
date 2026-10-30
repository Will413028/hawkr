/**
 * Content-Security-Policy (production only; the header wiring is in
 * `next.config.ts`).
 *
 * `script-src` deliberately keeps `'unsafe-inline'`: Next.js App Router injects
 * inline bootstrap / RSC-flight scripts whose content varies per request — a
 * nonce would force every route to render dynamically (a nonce can't be
 * statically prerendered), throwing away the ISR/SSG static HTML. So the CSP's
 * real hardening here is: an allowlist of the
 * only hosts that may serve scripts (an injected `<script src="//evil">` is
 * blocked), `object-src 'none'`, `base-uri 'self'` (blocks `<base>`-tag
 * relative-URL hijacking), and `frame-ancestors 'self'` (clickjacking).
 *
 * Allowlist by integration:
 *   GA4 (@next/third-parties)        googletagmanager.com, *.google-analytics.com
 *   Google Sign-In (GSI, vendor      accounts.google.com, gstatic.com,
 *   portal step-up)                  apis.google.com, *.googleapis.com
 *   Google Pay                       pay.google.com
 *   Apple Pay                        applepay.cdn-apple.com,
 *                                    smp-paymentservices.apple.com
 *   Cloudflare Web Analytics / RUM   static.cloudflareinsights.com,
 *                                    cloudflareinsights.com
 *   Backend gateway + chat WS        API base env (https + wss)
 *   Media (image-gateway)            NEXT_PUBLIC_MEDIA_BASE_URL
 *   Sentry ingest                    NEXT_PUBLIC_SENTRY_DSN host
 *   Media upload (presigned PUT)     MEDIA_UPLOAD_ORIGINS (deploy-specific; absent
 *                                    means uploads are not allowed by the policy)
 *
 * Gated to production because `next dev` (Turbopack HMR) needs 'unsafe-eval' and
 * a localhost websocket that this policy intentionally omits.
 */

/**
 * The browser origin of a URL-ish value, or `""` when it is empty or not an
 * absolute URL.
 */
export function originOf(value: string | undefined): string {
	try {
		return value?.trim() ? new URL(value.trim()).origin : "";
	} catch {
		return "";
	}
}

/**
 * Expand each origin with its websocket sibling (`https` -> `wss`), dropping
 * empty values and duplicates.
 */
export function withWebSocketSiblings(origins: readonly string[]): string[] {
	return [
		...new Set(
			origins
				.filter(Boolean)
				.flatMap((origin) =>
					/^https?:/.test(origin)
						? [origin, origin.replace(/^http/, "ws")]
						: [origin],
				),
		),
	];
}

/**
 * Parse `MEDIA_UPLOAD_ORIGINS`: a comma- or whitespace-separated list of
 * `https://` origins, where the host may start with one `*.` label (the form a
 * virtual-hosted bucket needs, `https://*.<account>.r2.cloudflarestorage.com`).
 * Anything else — a scheme-only source (`https:`), a bare `*`, a path, a
 * non-https scheme, a wildcard that is not a single leading label — is a
 * configuration error: widening the policy silently is worse than failing the
 * build. Unset or blank means no upload origin.
 */
export function parseUploadOrigins(raw: string | undefined): string[] {
	const entries = (raw ?? "").split(/[\s,]+/).filter(Boolean);
	return [
		...new Set(
			entries.map((entry) => {
				const match =
					/^https:\/\/((?:\*\.)?[a-z0-9-]+(?:\.[a-z0-9-]+)+)(:\d+)?\/?$/i.exec(
						entry,
					);
				if (!match) {
					throw new Error(
						`MEDIA_UPLOAD_ORIGINS entry "${entry}" must be an https origin such as https://uploads.example.com or https://*.uploads.example.com`,
					);
				}
				return `https://${match[1].toLowerCase()}${match[2] ?? ""}`;
			}),
		),
	];
}

export interface ContentSecurityPolicyInput {
	/** Origins the browser connects to besides third parties: API, media, Sentry. */
	appConnectOrigins: readonly string[];
	/** Presigned-upload destinations (`parseUploadOrigins`). */
	uploadOrigins: readonly string[];
}

function sources(...parts: ReadonlyArray<string | readonly string[]>): string {
	return parts.flat().filter(Boolean).join(" ");
}

/**
 * The enforced / report-only policy string (without `upgrade-insecure-requests`,
 * which only the enforcing header adds).
 *
 * 為什麼 `img-src https:` 還在而 connect-src 要逐一列出自家 origin：`img-src https:`
 * 讓 <img> 一直能顯示，掩蓋了另一條真的會被擋的路徑——瀏覽器對同一批網址發的
 * `fetch()` 走 connect-src。實例：lib/media/resolve-staged-images.ts 在賣家編輯既有
 * 商品時把每張既有圖 fetch 回來重新 stage，該網址正是 `{mediaBaseUrl}/i/original/{key}`；
 * 少了 media origin，enforce 之下該 fetch 被擋，使用者看到的是「無法下載既有圖片」
 * 而非 CSP 錯誤。env 沒設的來源不會出現在清單裡。
 *
 * google.com（裸域）：Google Pay JS 的 PaymentsClient 會直連 https://google.com/pay，
 * 只列 www/pay 子域擋不住。
 * smp-paymentservices.apple.com：Apple Pay JS SDK 的 applePayCapabilities 會打
 * /paymentservices/v3/checkStatus/merchant/*；少了它 enforce 會擋掉這支能力偵測，
 * 結帳頁的 Apple Pay 判定靜默失真。
 * stats.g.doubleclick.net：GA4 的 Google Signals cookie 同步；只列這一個精確 host，
 * 不放行整個 *.doubleclick.net。
 * applepay.cdn-apple.com（frame-src）：Apple Pay JS SDK 第三方瀏覽器掃碼流程用它渲染
 * 付款 sheet 的 iframe；少了它 enforce 下該 iframe 被擋成空白。
 */
export function buildContentSecurityPolicy(
	input: ContentSecurityPolicyInput,
): string {
	return [
		"default-src 'self'",
		"base-uri 'self'",
		"object-src 'none'",
		"frame-ancestors 'self'",
		"form-action 'self' https:",
		sources(
			"script-src 'self' 'unsafe-inline' 'report-sample'",
			"https://www.googletagmanager.com https://www.gstatic.com https://www.google.com https://apis.google.com https://accounts.google.com https://pay.google.com https://applepay.cdn-apple.com https://static.cloudflareinsights.com",
		),
		"style-src 'self' 'unsafe-inline' https://accounts.google.com",
		"style-src-elem 'self' 'unsafe-inline' https://accounts.google.com",
		"img-src 'self' data: blob: https:",
		"font-src 'self' data: https://applepay.cdn-apple.com",
		sources(
			"connect-src 'self'",
			input.appConnectOrigins,
			input.uploadOrigins,
			"https://www.googletagmanager.com https://*.google-analytics.com https://analytics.google.com https://*.analytics.google.com https://stats.g.doubleclick.net https://*.googleapis.com https://google.com https://www.google.com https://*.gstatic.com https://accounts.google.com https://pay.google.com https://smp-paymentservices.apple.com https://static.cloudflareinsights.com https://cloudflareinsights.com",
		),
		"frame-src 'self' https://accounts.google.com https://apis.google.com https://pay.google.com https://applepay.cdn-apple.com",
		"worker-src 'self' blob:",
		"manifest-src 'self'",
		// Collects violations at app/api/csp-report/route.ts → Sentry.
		"report-uri /api/csp-report",
	].join("; ");
}
