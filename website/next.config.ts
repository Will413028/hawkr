import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import bundleAnalyzer from "@next/bundle-analyzer";
import { withSentryConfig } from "@sentry/nextjs/config";
import type { NextConfig } from "next";
import createNextIntlPlugin from "next-intl/plugin";
import { apiProxyRewrites } from "./lib/api/local-proxy";
import { getEnv } from "./lib/config/env";
import { allRoutableLocales, defaultLocale } from "./lib/intl/locales";
import { isNonLocalizedPath } from "./lib/intl/paths";
import {
	buildContentSecurityPolicy,
	originOf,
	parseUploadOrigins,
	uniqueOrigins,
} from "./lib/security/csp";
import { PRIVATE_PREFIXES } from "./lib/seo/indexability";

const projectRoot = dirname(fileURLToPath(import.meta.url));
const workspaceRoot = resolve(projectRoot, "..");
const withBundleAnalyzer = bundleAnalyzer({
	enabled: process.env.ANALYZE === "true",
});
const withNextIntl = createNextIntlPlugin("./i18n/request.ts");
const isAnalyzeBuild = process.env.ANALYZE === "true";
const HAWKR_AGENT_ENABLED = getEnv().hawkrAgentEnabled;

const MEDIA_ORIGIN = originOf(getEnv().mediaBaseUrl);
const IMAGE_REMOTE_PATTERNS: NonNullable<
	NonNullable<NextConfig["images"]>["remotePatterns"]
> = MEDIA_ORIGIN
	? [
			{
				protocol: MEDIA_ORIGIN.startsWith("http:") ? "http" : "https",
				hostname: new URL(MEDIA_ORIGIN).hostname,
			},
		]
	: [];

/**
 * The origins the browser itself talks to, taken from the same env values the
 * app reads: the API gateway (https; chat streams over fetch, not WebSocket), the API base
 * when it is absolute, the media gateway, and the Sentry ingest host of
 * `NEXT_PUBLIC_SENTRY_DSN`. An unset value contributes nothing.
 */
const APP_CONNECT_ORIGINS = uniqueOrigins([
	getEnv().gatewayHost,
	originOf(getEnv().apiBaseUrl),
	MEDIA_ORIGIN,
	originOf(process.env.NEXT_PUBLIC_SENTRY_DSN),
]);

/** Presigned-upload destinations (`lib/media/upload.ts` step 2 PUT), deploy-specific. */
const UPLOAD_ORIGINS = parseUploadOrigins(process.env.MEDIA_UPLOAD_ORIGINS);

const CONTENT_SECURITY_POLICY = buildContentSecurityPolicy({
	appConnectOrigins: APP_CONNECT_ORIGINS,
	uploadOrigins: UPLOAD_ORIGINS,
});

/**
 * CSP 模式開關，**每個環境各自決定**。
 *
 * report-only 時政策以 `Content-Security-Policy-Report-Only` 送出：瀏覽器照樣把
 * 每個違規 POST 到 /api/csp-report → Sentry，但什麼都不擋，所以漏了一條白名單也
 * 不會弄壞線上流程。enforce 時才真的攔截。
 *
 * 模式由 build 期環境變數 `CSP_MODE` 決定；repo 內沒有任何 workflow 或設定檔設定它，
 * 只有部署平台（Vercel）專案的環境變數能設。
 * 之所以不用單一常數：常數會讓 dev→main 例行合併把 dev 的攔截決定直接帶上 prod，
 * 前端還沒準備好就炸。走 env 則各環境獨立，與程式碼合併脫鉤。
 *
 * **未設值一律 report-only**（fail-safe）：本機 `pnpm build`、CI 的 cacheComponents
 * gate 與 E2E harness 都不會意外進入攔截模式。
 *
 * **Kill switch（一行回滾）**：把該環境的 `CSP_MODE` 改回 `report-only`（或移除），
 * 重新部署即恢復「只記錄不攔截」。
 *
 * `upgrade-insecure-requests` 只在 enforce 模式加上：report-only 政策裡瀏覽器會
 * 忽略該指令，還會為每個 document/iframe 印一行 console error，純噪音。
 */
const CSP_REPORT_ONLY = process.env.CSP_MODE !== "enforce";

/**
 * `upgrade-insecure-requests` unconditionally rewrites every same-origin
 * fetch/RSC-payload request the page makes from http to https. CI's E2E job
 * also runs a real production build (`next start`, so NODE_ENV=production
 * like above) but only serves it over plain http on localhost — no TLS
 * listener — so without this guard every E2E test that triggers a
 * client-side fetch fails with net::ERR_SSL_PROTOCOL_ERROR. playwright.config.ts
 * already sets PLAYWRIGHT_PORT only for that harness (the real deploy build
 * runs in its own isolated CI job and never sees this var), so it doubles as
 * the signal here.
 */
const IS_E2E_HARNESS = Boolean(process.env.PLAYWRIGHT_PORT);

/**
 * De-indexing for every deployment the launch gate says is not indexable (dev,
 * and prod before launch) — the same `env.seoIndexable` that drives
 * `app/robots.ts`.
 *
 * It is a header rather than only a robots.txt line because a robots.txt body
 * is merged per user-agent group (RFC 9309): a robots.txt prepended by an
 * intermediary with an equally specific `Allow: /` cancels the `Disallow: /`
 * this app serves. A response header sits outside that merge.
 *
 * Scope: `headers()` only decorates responses Next itself produces. Files under
 * `public/` and `/_next/static/*` are answered by the platform's asset layer
 * before any function runs, so they carry no such header. A JS chunk or an SVG
 * is not indexable content, so the reach is "every Next-rendered response, HTML
 * or not", not "every response on the origin".
 */
const NOINDEX_HEADERS = getEnv().seoIndexable
	? []
	: [{ key: "X-Robots-Tag", value: "noindex, nofollow" }];

/**
 * Applied to every route. The first four are zero-breakage XSS / clickjacking /
 * MIME-sniffing hardening; the CSP is added in production only (see above).
 * `Permissions-Policy` disables features this site never uses. Geolocation is
 * explicitly limited to same-origin pages because the nearby-seller rail asks
 * for browser location; embedded third-party frames must not inherit access.
 * `payment` is left unlisted so it keeps its `self` default and Apple Pay /
 * Google Pay work.
 */
const SECURITY_HEADERS = [
	{ key: "X-Content-Type-Options", value: "nosniff" },
	{ key: "X-Frame-Options", value: "SAMEORIGIN" },
	{ key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
	{
		key: "Permissions-Policy",
		value: "camera=(), microphone=(), geolocation=(self)",
	},
	...NOINDEX_HEADERS,
	...(process.env.NODE_ENV === "production"
		? [
				{
					key: CSP_REPORT_ONLY
						? "Content-Security-Policy-Report-Only"
						: "Content-Security-Policy",
					value: CSP_REPORT_ONLY
						? CONTENT_SECURITY_POLICY
						: IS_E2E_HARNESS
							? CONTENT_SECURITY_POLICY
							: `${CONTENT_SECURITY_POLICY}; upgrade-insecure-requests`,
				},
			]
		: []),
];

/**
 * A document or RSC payload can contain build-specific chunk references.
 * Letting a browser or an intermediary retain those responses across a
 * deployment can pair old HTML/RSC with a new asset set, leaving hydration
 * stuck on a loading shell. The server-side ISR and data caches remain
 * enabled; this only prevents response-level caches from retaining
 * version-coupled page payloads.
 *
 * `no-cache`（不是 `no-store`）就足以擋住上面那個 skew：它禁止任何快取在未向
 * origin 重新驗證前重用回應，瀏覽器與中介每次都會回到 origin 拿當前版本。
 * `no-store` 多做的一件事是禁止瀏覽器把頁面放進 back/forward cache ——
 * 實測（2026-09-04，Chrome preload_check）整站因此零 bfcache，按上一頁必須整份
 * 文件重新載入、重跑 hydration。bfcache 還原的是記憶體裡「已經跑起來」的頁面，
 * 它的 chunk 早就載入完成，不存在舊 HTML 配新資產的問題；之後的 RSC 導航若撞
 * 到新部署，lib/loading/stale-chunk-reload 的既有機制會處理，與任何長開分頁相同。
 */
const PAGE_RESPONSE_CACHE_HEADERS = [
	{
		key: "Cache-Control",
		value: "private, no-cache, max-age=0, must-revalidate",
	},
];

/**
 * 私有路由維持 `no-store`：帳號、賣家中心、代行、結帳、購物車、金流回跳、訂單、
 * 登入註冊、超商地圖回跳、商品匯入預覽。這些頁面帶個資或一次性憑證，不該留在
 * 磁碟或 bfcache 裡；登出後按上一頁不能還原出剛才的訂單頁。前綴名單以
 * lib/seo/indexability.ts 的 PRIVATE_PREFIXES 為底（那裡決定 noindex，這裡決定
 * no-store，兩者本來就是同一組「不屬於公眾的路徑」），再補上那份名單刻意不收、
 * 但同樣不能落進瀏覽器快取的面：
 *
 *   /adm                      代行工作區，靠 robots Disallow 而非 meta noindex
 *   /quote-confirm            一次性 token 驅動的報價確認，會觸發不可逆商業操作
 *   /seller/onboarding        開店流程，承接已驗證手機與表單狀態（/seller 與
 *                             /seller/pricing 是公開行銷頁，所以列完整路徑）
 *   /api                      route handler 的回應由 handler 自己宣告政策，但
 *                             `headers()` 的同名 header 會蓋過它（見檔尾
 *                             /adm/enter 那條的實測）。/api/health 明寫 no-store
 *                             是 load-bearing（快取的健康回應會讓死掉的部署看
 *                             起來還活著）、tab-bootstrap 也以 no-store 為契約，
 *                             所以整個 /api 維持改動前的 no-store。
 *   /sentinel                 快取金絲雀探針，同上（lib/canary/sentinel.ts）。
 *
 * tests/page-response-cache.test.ts 用一份獨立手寫的敏感路徑清單去比對，不從這裡
 * 反推期望值，所以名單漏了 app/ 裡的敏感頁時測試會紅，而不是跟著一起錯。
 */
const PRIVATE_RESPONSE_CACHE_HEADERS = [
	{
		key: "Cache-Control",
		value: "private, no-cache, no-store, max-age=0, must-revalidate",
	},
];
const NO_STORE_ROUTE_PREFIXES = [
	...PRIVATE_PREFIXES,
	"/adm",
	"/quote-confirm",
	"/seller/onboarding",
	"/api",
	"/sentinel",
] as const;
const privateRouteSource = (
	prefixes: readonly string[],
	localePrefix = "",
): string =>
	`${localePrefix}/(${prefixes.map((prefix) => prefix.slice(1)).join("|")})(/.*)?`;
const PRIVATE_ROUTE_SOURCE = privateRouteSource(NO_STORE_ROUTE_PREFIXES);

/**
 * 這個部署會路由的非預設語系（lib/intl/locales.ts）：prod 沒有（`/en` 回 404），dev 是 `en`。
 * 語系前綴只出現在 `app/[locale]` 的路徑上，所以下面兩份路徑規則（私有路由的 no-store、舊網址的導向）
 * 各自多一份帶前綴的版本；沒有非預設語系時規則不變，一條都不多。
 * 明寫每個語系的規則，不用 `/:locale(en)?/…` 的可選參數：destination 是 `/` 時缺參數會編成空字串。
 */
const NON_DEFAULT_LOCALES = allRoutableLocales().filter(
	(locale) => locale !== defaultLocale,
);

/** 帶語系前綴才存在的私有路由（不含不本地化的 /adm、/api、/sentinel 等）。 */
const LOCALIZED_NO_STORE_PREFIXES = NO_STORE_ROUTE_PREFIXES.filter(
	(prefix) => !isNonLocalizedPath(prefix),
);

interface RedirectRule {
	source: string;
	destination: string;
	permanent: boolean;
	has?: { type: "query"; key: string }[];
}

/** 每條導向規則補上各個非預設語系的版本：`/products` → `/market/products` 之外還有 `/en/products` → `/en/market/products`。 */
function withLocalizedRedirects(
	rules: readonly RedirectRule[],
): RedirectRule[] {
	return [
		...rules,
		...NON_DEFAULT_LOCALES.flatMap((locale) =>
			rules.map((rule) => ({
				...rule,
				source: `/${locale}${rule.source}`,
				destination:
					rule.destination === "/"
						? `/${locale}`
						: `/${locale}${rule.destination}`,
			})),
		),
	];
}

const REQUEST_SHAPES_WITHOUT_A_DOCUMENT_RULE = [
	{ type: "header" as const, key: "accept", value: ".*text/html.*" },
	{ type: "header" as const, key: "rsc", value: "1" },
];

const nextConfig: NextConfig = {
	transpilePackages: [
		"@hawkr/api",
		"@hawkr/icons",
		"@hawkr/twaddr",
		"@hawkr/ui-text",
	],
	turbopack: { root: workspaceRoot },
	outputFileTracingRoot: workspaceRoot,
	devIndicators: false,
	// 所以找不到任何路由時用 app/global-not-found.tsx（整份文件自己組，見該檔的說明）。
	experimental: { globalNotFound: true },
	cacheComponents: true,
	images: {
		loader: "custom",
		loaderFile: "./lib/media/ossImageLoader.ts",
		qualities: [70, 75],
		remotePatterns: IMAGE_REMOTE_PATTERNS,
	},

	async headers() {
		return [
			{
				source: "/(.*)",
				has: [{ type: "header", key: "accept", value: ".*text/html.*" }],
				headers: PAGE_RESPONSE_CACHE_HEADERS,
			},
			{
				source: "/(.*)",
				has: [{ type: "header", key: "rsc", value: "1" }],
				headers: PAGE_RESPONSE_CACHE_HEADERS,
			},
			{
				source: "/(.*)",
				missing: REQUEST_SHAPES_WITHOUT_A_DOCUMENT_RULE,
				headers: PAGE_RESPONSE_CACHE_HEADERS,
			},
			{
				source: PRIVATE_ROUTE_SOURCE,
				headers: PRIVATE_RESPONSE_CACHE_HEADERS,
			},
			...NON_DEFAULT_LOCALES.map((locale) => ({
				source: privateRouteSource(LOCALIZED_NO_STORE_PREFIXES, `/${locale}`),
				headers: PRIVATE_RESPONSE_CACHE_HEADERS,
			})),
			{ source: "/(.*)", headers: SECURITY_HEADERS },
			{
				source: "/_next/static/:path*",
				headers: [
					{
						key: "Cache-Control",
						value: "public, max-age=31536000, immutable",
					},
				],
			},
			{
				source: "/adm/enter",
				headers: [{ key: "Referrer-Policy", value: "no-referrer" }],
			},
		];
	},

	async rewrites() {
		return {
			beforeFiles: apiProxyRewrites(),
			afterFiles: [
				{
					source:
						"/.well-known/apple-developer-merchantid-domain-association.txt",
					destination: "/api/apple-pay/domain-association",
				},
				{
					source: "/.well-known/apple-developer-merchantid-domain-association",
					destination: "/api/apple-pay/domain-association",
				},
			],
			fallback: [],
		};
	},

	/**
	 * IA cleanup (2026-05-03):
	 * Consolidate parallel marketplace entries into a single /market namespace.
	 * Old SEO entry points 301 → new canonical paths.
	 * /products/[id] is preserved as the canonical product-detail URL — only
	 * the list page /products redirects.
	 */
	async redirects() {
		const pageRedirects: RedirectRule[] = [
			...(!HAWKR_AGENT_ENABLED
				? [
						{
							source: "/account/messages/assistant/:path*",
							has: [{ type: "query" as const, key: "requestId" }],
							destination: "/account/messages?requestId=",
							permanent: false,
						},
						{
							source: "/account/messages/assistant/:path*",
							destination: "/account/messages",
							permanent: false,
						},
						{
							source: "/account/find-sellers",
							destination: "/account/messages",
							permanent: false,
						},
					]
				: []),
			{ source: "/products", destination: "/market/products", permanent: true },
			{ source: "/farmers", destination: "/market/products", permanent: true },
			{ source: "/sellers", destination: "/market/products", permanent: true },
			{ source: "/explore", destination: "/feed", permanent: true },
			{ source: "/daily-posts", destination: "/feed", permanent: true },
			{ source: "/market", destination: "/market/products", permanent: true },
			{
				source: "/market/sellers",
				destination: "/market/products",
				permanent: true,
			},
			{ source: "/seasonal", destination: "/market/products", permanent: true },
			{ source: "/topics", destination: "/feed", permanent: true },
			{ source: "/education", destination: "/feed", permanent: true },
			{
				source: "/account/messages/assistant/new",
				destination: "/account/messages/assistant/procurement",
				permanent: false,
			},
			{
				source: "/account/find-sellers",
				destination: "/account/messages/assistant/procurement",
				permanent: false,
			},
		];

		return [
			...withLocalizedRedirects(pageRedirects),
			{
				source: "/adm/:storeId(\\d+)",
				destination: "/adm/:storeId/products",
				permanent: false,
			},
		];
	},
};

const configWithPlugins = withNextIntl(withBundleAnalyzer(nextConfig));

export default isAnalyzeBuild
	? configWithPlugins
	: withSentryConfig(configWithPlugins, {
			org: process.env.SENTRY_ORG,
			project: process.env.SENTRY_PROJECT,
			silent: true,
			telemetry: false,
			widenClientFileUpload: true,
		});
