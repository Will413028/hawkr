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
 *   Stripe Connect embedded          connect-js.stripe.com, js.stripe.com
 *   components (script-src,          (script-src, frame-src). The same page lists
 *   frame-src)                       img-src https://*.stripe.com (covered by
 *                                    `img-src https:`) and the style-src hash of an
 *                                    empty style element (covered by 'unsafe-inline');
 *                                    it lists no connect-src host.
 *                                    https://docs.stripe.com/connect/get-started-connect-embedded-components
 *   Stripe Payment Element           js.stripe.com (script-src, frame-src), api.stripe.com
 *   (checkout payment step)          (connect-src), hooks.stripe.com (frame-src, 3DS)
 *   Backend gateway (Connect, incl.   API base env (https)
 *   the chat server stream)
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

/** Drop empty values and duplicates from a list of origins. */
export function uniqueOrigins(origins: readonly string[]): string[] {
	return [...new Set(origins.filter(Boolean))];
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
 * stats.g.doubleclick.net：GA4 的 Google Signals cookie 同步；只列這一個精確 host，
 * 不放行整個 *.doubleclick.net。
 * Stripe Payment Element（付款步驟）：wallet（Apple Pay、Google Pay）在 Stripe 自己的 iframe 內執行，頁面不需要它們的來源。
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
			"script-src 'self' 'unsafe-inline' 'wasm-unsafe-eval' 'report-sample'",
			"https://www.googletagmanager.com https://www.gstatic.com https://www.google.com https://apis.google.com https://accounts.google.com https://connect-js.stripe.com https://js.stripe.com",
		),
		"style-src 'self' 'unsafe-inline' https://accounts.google.com",
		"style-src-elem 'self' 'unsafe-inline' https://accounts.google.com",
		"img-src 'self' data: blob: https:",
		"font-src 'self' data:",
		sources(
			"connect-src 'self'",
			input.appConnectOrigins,
			input.uploadOrigins,
			"https://www.googletagmanager.com https://*.google-analytics.com https://analytics.google.com https://*.analytics.google.com https://stats.g.doubleclick.net https://*.googleapis.com https://www.google.com https://*.gstatic.com https://accounts.google.com https://api.stripe.com",
		),
		"frame-src 'self' https://accounts.google.com https://apis.google.com https://connect-js.stripe.com https://js.stripe.com https://hooks.stripe.com",
		"worker-src 'self' blob:",
		"manifest-src 'self'",
		"report-uri /api/csp-report",
	].join("; ");
}
