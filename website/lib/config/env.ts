/**
 * Per-flavor environment configuration.
 *
 * TS analogue of `flavor_extension.dart` in the Flutter app — single source of
 * truth for everything that varies between dev / prod (gateway URLs, timeouts,
 * logging flags).
 *
 * App code MUST consume `env` from this module rather than reading
 * `process.env.NEXT_PUBLIC_*` directly. The only direct readers permitted
 * are this file and `lib/config/flavor.ts`. New per-environment values should
 * be added here, never sprinkled inline.
 *
 * Public values (gateway host, OAuth client ids) are baked into the client
 * bundle at `next build` time via `NEXT_PUBLIC_*` env vars. Sign-in has no key here:
 * the monolith runs the OAuth flow and sets the session cookie (ADR 2026-10-30-identity-self-hosted-federated-login).
 */

import { currentFlavor, type Flavor } from "./flavor";

export interface EnvConfig {
	flavor: Flavor;
	displayName: string;
	/** Gateway origin without the /api/v1 suffix. */
	gatewayHost: string;
	/** Backend base URL with the /api/v1 suffix; matches mobile `apiBaseUrl`. */
	apiBaseUrl: string;
	/** /api/v1/public — anonymous endpoints. */
	publicApi: string;
	/** /api/v1/app — authenticated endpoints. */
	appApi: string;
	/** Base the SERVER/SSR uses. Always the absolute gateway origin (server fetch needs a host; public SEO data, no cookie). */
	serverApiBase: string;
	/**
	 * Active web origin (where this build is deployed), from
	 * `NEXT_PUBLIC_WEB_ORIGIN`: dev falls back to the local HTTPS dev server, prod
	 * requires it. It is the single source for canonical URLs, the canonical-host
	 * allow-list and the CSRF origin check.
	 */
	webOrigin: string;
	/**
	 * ADM（`hawkr-admin`）的 origin。代行工作區的「返回 ADM」與失效畫面
	 * 必須連回去，而 ADM 與 www 從此在兩個不同 host（56 號 §4.5）。
	 *
	 * 取自 `NEXT_PUBLIC_ADM_ORIGIN`：dev 未設就用本機 ADM，prod 必填（缺值時
	 * 載入設定即失敗）。**不得由請求的 `Host` 推導** —— 那讓任何能控制 Host
	 * 標頭的人指定管理員被送去哪裡。
	 */
	admOrigin: string;
	/** Default fetch timeout in ms. */
	apiTimeoutMs: number;
	/** True for dev; false for prod. Gates verbose logging. */
	enableApiLogging: boolean;
	/** Stricter than enableApiLogging — only dev shows raw stack traces. */
	enableDetailedErrors: boolean;
	/**
	 * Top-level Hawkr Agent availability. DEV defaults on; production defaults off
	 * unless an exact build-time override explicitly enables it.
	 */
	hawkrAgentEnabled: boolean;
	/**
	 * True only when prod AND the launch flag (NEXT_PUBLIC_SEO_INDEXABLE) is on.
	 * The single gate for all SEO indexability (root robots metadata + robots.txt).
	 * Default false so prod infra can be deployed / smoke-tested before public
	 * launch without Google indexing a half-built site. Flip the env var to
	 * "true" on launch day — no code change.
	 */
	seoIndexable: boolean;
	/** Google Identity Services OAuth client ID (web; the vendor portal's step-up). Baked at build. */
	googleClientId: string;
	/** GA4 Measurement ID (G-XXXXXXX). Empty string = analytics disabled
	 * (dev/stage leave it unset; prod sets it → events fire). Baked at build. */
	gaId: string;
	/**
	 * Base origin of the media / image-gateway (R2 + Cloudflare Worker), from
	 * `NEXT_PUBLIC_MEDIA_BASE_URL`. It is the only media host the app recognises
	 * as its own. **Empty string = no media origin configured**: `buildImageUrl`
	 * builds no URL for a bare object key (the image is simply absent) and never
	 * falls back to another host. Baked at build. */
	mediaBaseUrl: string;
	/**
	 * Add-friend URL of the LINE official account, from `NEXT_PUBLIC_LINE_URL`
	 * (an absolute https URL). **Empty string = not configured**: the contact
	 * page renders no LINE entry and never builds a link to an account we do not
	 * know. Baked at build. */
	lineUrl: string;
}

/**
 * Per-flavor constant tables.
 */
const DEV_GATEWAY_HOST = "http://localhost:8080";

/**
 * Origin of the API gateway. Dev defaults to the local monolith; prod has no
 * default and takes the origin of the configured API base. Empty string means
 * the gateway is not configured, so features that need it stay unavailable.
 */
function readGatewayHost(flavor: Flavor): string {
	if (flavor === "dev") return DEV_GATEWAY_HOST;
	for (const raw of [
		process.env.API_BASE_URL,
		process.env.NEXT_PUBLIC_API_BASE_URL,
	]) {
		const value = raw?.trim();
		if (!value) continue;
		try {
			return new URL(value).origin;
		} catch {
			// A relative base has no origin; try the next source.
		}
	}
	return "";
}

/** Where `pnpm dev` serves the site (package.json `dev`, plain `next dev`). */
const DEV_WEB_ORIGIN = "http://localhost:3000";
/** Where the local admin app runs. */
const DEV_ADM_ORIGIN = "http://localhost:3001";

const DISPLAY_NAME: Record<Flavor, string> = {
	dev: "開發環境",
	prod: "生產環境",
};

const TIMEOUT_MS: Record<Flavor, number> = {
	dev: 30_000,
	prod: 15_000,
};

/** Wall-clock budget for one anonymous home tab-bootstrap request. */
export interface HomeBootstrapBudget {
	/** Deadline for the primary aggregates, measured from handler entry. */
	totalMs: number;
	/** Slice granted to the cache-backed fallback once the primary has burnt the
	 *  whole budget — enough for a cache hit, never a second full upstream wave. */
	fallbackFloorMs: number;
}

/**
 * Budget for the `force-dynamic` home tab-bootstrap route, deliberately far
 * below `TIMEOUT_MS`.
 *
 * `TIMEOUT_MS` bounds a single request on paths whose timeout means "serve the
 * previous ISR entry" — there, waiting longer is cheap and usually wins. The
 * bootstrap route is the opposite: it answers a browser that abandons the
 * request at `homeSnapshotDeadlineMs()`, so anything produced after that can
 * reach nobody. Each flavor's `totalMs + fallbackFloorMs` — the worst case one
 * visitor can ever wait — must therefore stay inside its own deadline;
 * `home-bootstrap-budget.test.ts` asserts that pairing per flavor.
 */
const HOME_BOOTSTRAP_BUDGET: Record<Flavor, HomeBootstrapBudget> = {
	dev: { totalMs: 10_000, fallbackFloorMs: 1_500 },
	prod: { totalMs: 3_500, fallbackFloorMs: 500 },
};

/**
 * How long the browser waits for a home snapshot refresh before abandoning it
 * and keeping the snapshot already on screen (features/home/HomePage.tsx).
 *
 * Lives here, next to the server budget it must outlast, because the two numbers
 * are one contract: a deadline below the server's worst case makes that part of
 * the budget dead time — the answer is produced after the only caller stopped
 * listening. Prod's 4.5s is the visitor-facing ceiling the whole budget was
 * sized against; dev is allowed the longer wait its looser budget implies, since
 * losing every client reconcile in dev would hide refresh behaviour from QA
 * rather than mirror prod. Both leave ~500ms over the budget sum for transport.
 */
const HOME_SNAPSHOT_DEADLINE_MS: Record<Flavor, number> = {
	dev: 12_000,
	prod: 4_500,
};

export function homeBootstrapBudget(): HomeBootstrapBudget {
	return HOME_BOOTSTRAP_BUDGET[getEnv().flavor];
}

export function homeSnapshotDeadlineMs(): number {
	return HOME_SNAPSHOT_DEADLINE_MS[getEnv().flavor];
}

/**
 * Lets a `NEXT_PUBLIC_*` build-time override beat the hard-coded fallback, so a
 * deployer can repoint a per-flavor value (e.g. the gateway host) with only an
 * env redeploy, no code change.
 */
function pick(envValue: string | undefined, fallback: string): string {
	const trimmed = envValue?.trim();
	return trimmed && trimmed.length > 0 ? trimmed : fallback;
}

/**
 * Resolve the top-level Hawkr Agent gate.
 *
 * Exact true/false values are deliberate: a typo in production must fall back
 * to the flavor-safe default (off), never accidentally publish the feature.
 */
export function resolveHawkrAgentEnabled(
	flavor: Flavor,
	raw: string | undefined = process.env.NEXT_PUBLIC_HAWKR_AGENT_ENABLED,
): boolean {
	const normalized = raw?.trim();
	if (normalized === "true") return true;
	if (normalized === "false") return false;
	return flavor === "dev";
}

function readApiBase(flavor: Flavor): string {
	// Browser-facing base. Local dev defaults to the RELATIVE `/api/v1` so the
	// browser only ever talks to its own origin and the `app/api/v1/[...path]`
	// route proxies to the gateway server-side — a same-origin topology that
	// MIRRORS prod (prod serves the API first-party under the web host to dodge
	// content blockers). Same-origin means the session cookie (`__Host-hk_session`) is sent to the
	// API host too (the cookie has no Domain, so it can only go same-origin) and CORS / cookie-domain /
	// VPN / blocker / self-signed-cert friction all disappear. Opt into raw
	// cross-origin (rarely needed — prod isn't cross-origin) by setting
	// NEXT_PUBLIC_API_BASE_URL to the absolute gateway URL. Deployed builds always
	// set NEXT_PUBLIC_API_BASE_URL explicitly, so this dev default never reaches them.
	const gatewayHost = readGatewayHost(flavor);
	const fallback =
		process.env.NODE_ENV === "development"
			? "/api/v1"
			: gatewayHost
				? `${gatewayHost}/api/v1`
				: "";
	const base = pick(process.env.NEXT_PUBLIC_API_BASE_URL, fallback);
	return base.replace(/\/$/, "");
}

/**
 * Validate a configured origin: an absolute http(s) URL that is only an origin
 * (credentials, path, query and fragment are rejected, one trailing slash is
 * tolerated). Returns the canonical origin. `URL` normalisation hides unsafe
 * inputs (a bare `?` or `#` yields an empty search / hash, `/segment/..`
 * collapses to `/`), so the raw string's shape is checked as well.
 */
function parseOrigin(name: string, raw: string): string {
	let parsed: URL;
	try {
		parsed = new URL(raw);
	} catch {
		throw new Error(
			`${name} must be an absolute http(s) origin (got "${raw}")`,
		);
	}
	const authorityStart = raw.indexOf("://");
	const pathStart =
		authorityStart >= 0 ? raw.indexOf("/", authorityStart + 3) : -1;
	const hasOnlyOptionalRootPath =
		pathStart === -1 || pathStart === raw.length - 1;
	if (
		(parsed.protocol !== "https:" && parsed.protocol !== "http:") ||
		parsed.username ||
		parsed.password ||
		parsed.pathname !== "/" ||
		parsed.search ||
		parsed.hash ||
		raw.includes("?") ||
		raw.includes("#") ||
		!hasOnlyOptionalRootPath
	) {
		throw new Error(
			`${name} must be a credential-free http(s) origin without path, query or fragment (got "${raw}")`,
		);
	}
	// Never retain an equivalent-but-noncanonical spelling (case, :443, trailing slash).
	return parsed.origin;
}

/**
 * Read an origin-valued env var. Unset falls back to `devDefault` on dev; prod
 * has no default and fails the build/startup naming the variable.
 */
function readOriginEnv(
	name: string,
	raw: string | undefined,
	flavor: Flavor,
	devDefault: string,
): string {
	const value = raw?.trim();
	if (value) return parseOrigin(name, value);
	if (flavor === "dev") return devDefault;
	throw new Error(
		`${name} is required for the production flavor: set it to the deployed origin`,
	);
}

/**
 * The media / image-gateway origin. Empty when unset (no media URL is built).
 * A malformed value fails loudly rather than producing broken image URLs.
 */
function readMediaBaseUrl(): string {
	const raw = (process.env.NEXT_PUBLIC_MEDIA_BASE_URL ?? "").trim();
	return raw ? parseOrigin("NEXT_PUBLIC_MEDIA_BASE_URL", raw) : "";
}

function readLineUrl(): string {
	const raw = (process.env.NEXT_PUBLIC_LINE_URL ?? "").trim();
	if (!raw) return "";
	let url: URL;
	try {
		url = new URL(raw);
	} catch {
		throw new Error(
			"NEXT_PUBLIC_LINE_URL must be an absolute https URL (the LINE add-friend link)",
		);
	}
	if (url.protocol !== "https:" || url.username || url.password) {
		throw new Error(
			"NEXT_PUBLIC_LINE_URL must be an absolute https URL without credentials",
		);
	}
	return url.toString();
}

/**
 * The production flavor has no API default. The server needs an absolute
 * gateway origin to fetch from (SSR, route handlers) and every request path is
 * derived from the API base, so a missing value fails the build/startup instead
 * of producing relative or empty request URLs. Checked server-side only: the
 * browser bundle only sees `NEXT_PUBLIC_*`, and a same-origin relative base is
 * valid there. `NEXT_PUBLIC_API_BASE_URL` itself is required too: it is inlined
 * into the browser bundle at build time, and the server-side fallback to the
 * origin of `API_BASE_URL` must not hide that it is missing there (the browser
 * would then get an empty base and a relative `/public`).
 */
function assertApiConfigured(
	flavor: Flavor,
	apiBaseUrl: string,
	serverApiBase: string,
	publicApiBase: string | undefined = process.env.NEXT_PUBLIC_API_BASE_URL,
): void {
	if (flavor !== "prod" || typeof window !== "undefined") return;
	if (!apiBaseUrl || !serverApiBase || !publicApiBase?.trim()) {
		throw new Error(
			"API base is not configured for the production flavor: set API_BASE_URL (absolute, e.g. https://api.<your-domain>/api/v1) and NEXT_PUBLIC_API_BASE_URL",
		);
	}
}

function buildEnvConfig(flavor: Flavor): EnvConfig {
	const apiBaseUrl = readApiBase(flavor);
	const gatewayHost = readGatewayHost(flavor);
	const serverApiBase = pick(
		process.env.API_BASE_URL,
		gatewayHost ? `${gatewayHost}/api/v1` : "",
	).replace(/\/$/, "");
	assertApiConfigured(flavor, apiBaseUrl, serverApiBase);
	return {
		flavor,
		displayName: DISPLAY_NAME[flavor],
		gatewayHost,
		apiBaseUrl,
		publicApi: `${apiBaseUrl}/public`,
		appApi: `${apiBaseUrl}/app`,
		webOrigin: readOriginEnv(
			"NEXT_PUBLIC_WEB_ORIGIN",
			process.env.NEXT_PUBLIC_WEB_ORIGIN,
			flavor,
			DEV_WEB_ORIGIN,
		),
		admOrigin: readOriginEnv(
			"NEXT_PUBLIC_ADM_ORIGIN",
			process.env.NEXT_PUBLIC_ADM_ORIGIN,
			flavor,
			DEV_ADM_ORIGIN,
		),
		apiTimeoutMs: TIMEOUT_MS[flavor],
		enableApiLogging: flavor !== "prod",
		enableDetailedErrors: flavor === "dev",
		hawkrAgentEnabled: resolveHawkrAgentEnabled(flavor),
		seoIndexable:
			flavor === "prod" &&
			process.env.NEXT_PUBLIC_SEO_INDEXABLE?.trim() === "true",
		// Server/SSR base AND the dev proxy's forward target — ALWAYS absolute.
		// A relative base can't be fetched server-side, and `apiBaseUrl` is the
		// relative `/api/v1` in local dev proxy mode (the default), so this must NOT
		// fall back to apiBaseUrl. Defaults to the absolute gateway; API_BASE_URL
		// overrides (e.g. local read-hang repro pointing at a stub).
		serverApiBase,
		googleClientId: process.env.NEXT_PUBLIC_GOOGLE_CLIENT_ID ?? "",
		gaId: process.env.NEXT_PUBLIC_GA_MEASUREMENT_ID?.trim() ?? "",
		// No default on any flavor; canonical origin, so `${mediaBaseUrl}/i/...`
		// never doubles a slash.
		mediaBaseUrl: readMediaBaseUrl(),
		lineUrl: readLineUrl(),
	};
}

let _cached: EnvConfig | null = null;

/**
 * Resolves and memoises the active env config. Subsequent calls are free.
 */
export function getEnv(): EnvConfig {
	if (_cached) return _cached;
	_cached = buildEnvConfig(currentFlavor());
	return _cached;
}

/**
 * Convenience snapshot of the active env. Use `getEnv()` from helpers that
 * may be called before module init order is guaranteed; use `env` everywhere
 * else for terse access.
 */
export const env: EnvConfig = getEnv();

/** Test-only override. Avoid in app code. */
export function __setEnvForTest(next: EnvConfig | null) {
	_cached = next;
}

/**
 * Dev-only 浮動工具（角色切換器、React Query Devtools）的**單一總開關**（web#425）。
 *
 * 為什麼要有這個開關：那些工具是 viewport-fixed 的浮動鈕，會蓋在自家固定 chrome 上。
 * production build 完全不 render 它們，所以 CI 量到的 DOM 與本機 `next dev` 量到的
 * 不是同一份 —— 本機 E2E 因此會產生「看起來像產品壞掉」的假紅（web#425 就是整輪
 * 調查花在這上面）。Playwright 的 `webServer.env` 會把這個開關關掉，讓本機測到的
 * DOM 與 CI 一致。
 *
 * 預設是**開**：人工開發時這些工具本來就有用，只有自動化驅動時才關。
 *
 * ⚠️ 新增第三顆 dev-only 浮動元件時，掛載條件一律用 `DEV_OVERLAYS_ENABLED`，
 * 不要各自再寫一次 `process.env.NODE_ENV === "development"` ——
 * `tests/dev-overlays-gate.test.ts` 會擋下那種寫法。
 *
 * 參數只是為了讓測試能餵各種組合；正式呼叫一律不帶參數走 `process.env`
 *（兩者都是 build 期被 Next 替換掉的字面值，server / client 同值，無水合落差）。
 */
export function devOverlaysEnabled(
	nodeEnv: string | undefined = process.env.NODE_ENV,
	devOverlaysSwitch: string | undefined = process.env.NEXT_PUBLIC_DEV_OVERLAYS,
): boolean {
	return nodeEnv === "development" && devOverlaysSwitch !== "off";
}

/** `devOverlaysEnabled()` 的快照。掛載條件請用這個常數。 */
export const DEV_OVERLAYS_ENABLED: boolean = devOverlaysEnabled();
