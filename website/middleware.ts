import { type NextRequest, NextResponse } from "next/server";
import createIntlMiddleware from "next-intl/middleware";
import { routing } from "@/i18n/routing";
import { requiredAccessForPath, workspaceOfPath } from "@/lib/auth/redirect";
import { SESSION_COOKIE_NAME } from "@/lib/auth/session-cookie";
import { getEnv } from "@/lib/config/env";
import { type AppLocale, defaultLocale } from "@/lib/intl/locales";
import {
	isLocaleRoutableFor,
	isMetadataImagePath,
	isNonLocalizedPath,
	routingPathname,
	splitLocale,
} from "@/lib/intl/paths";
import {
	lookupSeller,
	resolveSellerRequest,
} from "@/lib/seller/canonical-redirect";
import {
	isCanonicalHost,
	NOINDEX_HEADER_NAME,
	NOINDEX_HEADER_VALUE,
} from "@/lib/seo/canonical-host";
import { isLocaleNoindex } from "@/lib/seo/indexability";

// OpenNext / workerd 不支援 Next 16 的 Node-runtime proxy（proxy.ts），故沿用
// hawkr-admin 做法改用 edge middleware。全 edge-safe（無 Node-only API）。
// 追蹤：待 opennextjs-cloudflare 補上 proxy.ts 支援後再遷移
// https://github.com/cloudflare/workers-sdk/issues/13755

/**
 * 語系路由（ADR 2026-10-17 D1，設定與理由在 i18n/routing.ts）：zh-TW 在 `/…`，其他語系在 `/<locale>/…`。
 * next-intl 把無前綴的網址改寫成 `/zh-TW/…`（交給 `app/[locale]`）、把明寫的 `/zh-TW/…` 導向無前綴網址。
 */
const handleI18nRouting = createIntlMiddleware(routing);

/**
 * De-index every response this deployment serves under a host it does not
 * publish under (#355) — in practice the `*.workers.dev` preview address, which
 * CI's post-deploy smoke needs alive and which therefore cannot simply be
 * turned off. Full reasoning, and why neither `app/robots.ts` nor
 * `next.config.ts` can make this decision, is in lib/seo/canonical-host.ts.
 *
 * Gated on `seoIndexable` because that is the exact complement of
 * `next.config.ts`'s `NOINDEX_HEADERS`: a non-indexable build already carries
 * this header on every Next-rendered response, so re-adding it here would only
 * risk emitting the instruction twice. When the gate is open the blanket header
 * is absent and this is the only thing standing between the preview host and
 * Google's index.
 */
function withHostIndexability(
	request: NextRequest,
	response: NextResponse,
): NextResponse {
	if (!getEnv().seoIndexable) return response;
	if (isCanonicalHost(request.headers.get("host"), getEnv().webOrigin)) {
		return response;
	}
	response.headers.set(NOINDEX_HEADER_NAME, NOINDEX_HEADER_VALUE);
	return response;
}

/**
 * A non-default locale's UGC page is kept out of the index (lib/seo/indexability.ts
 * `isLocaleNoindex`; ADR 2026-10-17 D1 "SEO"). Same mechanism and the same gate as
 * `withHostIndexability`: a header set here reaches the final response even when
 * OpenNext serves the HTML from its cache, and a non-indexable build already carries
 * a (stronger) one from next.config.ts.
 */
function withLocaleIndexability(
	response: NextResponse,
	pathname: string,
	locale: AppLocale,
): NextResponse {
	if (!getEnv().seoIndexable) return response;
	if (isLocaleNoindex(pathname, locale)) {
		response.headers.set(NOINDEX_HEADER_NAME, "noindex");
	}
	return response;
}

/**
 * Everything the site decides per request, on the locale-free pathname. What it
 * lets through goes to next-intl, whose rewrite/redirect is the return value.
 * Every redirect the site decides keeps the locale prefix.
 *
 * Who the visitor is: only whether the `__Host-hk_session` cookie is present. That
 * is an optimistic check (a cookie the monolith has since revoked still passes),
 * made without calling the API so the middleware stays free of a network hop; the
 * authoritative answer is the page's and the client's GetSession, which sends an
 * `unauthenticated` visitor back to the login page.
 */
export async function siteMiddleware(
	request: NextRequest,
): Promise<NextResponse> {
	return withHostIndexability(request, await route(request));
}

export function middleware(request: NextRequest): Promise<NextResponse> {
	return siteMiddleware(request);
}

/** What every "let it through" decision returns once locales exist: next-intl's rewrite. */
function pass(
	request: NextRequest,
	pathname: string,
	locale: AppLocale | null,
): NextResponse {
	return withLocaleIndexability(
		handleI18nRouting(request),
		pathname,
		locale ?? defaultLocale,
	);
}

/**
 * The app's not-found route with an explicit 404. Used where the status has to
 * be decided before any page renders: a storefront id the backend does not know,
 * and a locale this surface has not published.
 */
function notFoundResponse(request: NextRequest): NextResponse {
	const notFoundUrl = request.nextUrl.clone();
	notFoundUrl.pathname = "/_not-found";
	return NextResponse.rewrite(notFoundUrl, { status: 404 });
}

async function route(request: NextRequest): Promise<NextResponse> {
	// next-intl routes on the decoded, slash-collapsed path (`/%65n/…` and `//en/…`
	// reach the `/en/…` page), so every decision below reads that same string; an
	// undecodable path is left to Next, which answers 400.
	const routed = routingPathname(request.nextUrl.pathname);
	if (routed === null) return NextResponse.next();

	// Staff and infrastructure paths have no locale and never go to next-intl
	// (it would rewrite them to `/zh-TW/…`, which does not exist): lib/intl/paths.ts
	// NON_LOCALIZED_PREFIXES / NON_LOCALIZED_EXACT say which and why.
	if (isNonLocalizedPath(routed)) return NextResponse.next();

	const { locale, prefix, pathname } = splitLocale(routed);

	// An explicit default-locale prefix (`/zh-TW/…`) is not a canonical URL:
	// next-intl redirects it to the unprefixed one, and the decisions below run on
	// that second request. The exception is a metadata image route: Next writes
	// its URL into the HTML from the route params, prefix included even for the
	// default locale (a seller page's `<link rel="apple-touch-icon">`), so it is
	// served where it is instead of costing every icon fetch a redirect.
	if (locale === defaultLocale) {
		return isMetadataImagePath(pathname)
			? NextResponse.next()
			: handleI18nRouting(request);
	}

	// A locale this surface has not published is not a route. In prod `/en/…` is
	// already not a locale prefix at all (i18n/routing.ts only knows routable
	// locales); this is the case of a locale one surface published and another has not.
	if (locale && !isLocaleRoutableFor(locale, pathname)) {
		return notFoundResponse(request);
	}

	// next.config's production Agent gate lands hidden deep links here. Config
	// redirects preserve unmatched query parameters, and the requestId-specific
	// rule deliberately blanks the value in its first Location response. Remove
	// the key itself before the auth bounce can embed this URL into
	// /login?redirect=..., while preserving unrelated campaign/message params.
	if (
		!getEnv().hawkrAgentEnabled &&
		pathname === "/account/messages" &&
		request.nextUrl.searchParams.has("requestId")
	) {
		const cleanUrl = request.nextUrl.clone();
		cleanUrl.searchParams.delete("requestId");
		return NextResponse.redirect(cleanUrl, 307);
	}

	// Storefront status decisions (#365). Both live here rather than inside the
	// tab pages because those are statically prerendered, and a prerender has no
	// live response to carry a status: a redirect thrown from one ships as a
	// `<meta http-equiv="refresh">` inside a 200 (two documents for the visitor,
	// nothing at all for a crawler), and a `notFound()` renders the 404 body but
	// still answers 200. See lib/seller/canonical-redirect.ts.
	//
	// Neither response carries Set-Cookie, which matters: a Set-Cookie makes
	// Cloudflare's Workers Cache skip the entry, so a cookie here would knock the
	// whole storefront subtree out of the edge HTML cache.
	if (pathname.startsWith("/sellers/")) {
		// Middleware is one of only two places on this site that can see who the
		// visitor is (a cached RSC render cannot — `headers()` throws inside
		// `unstable_cache`), so it is where the storefront lookup gets attributed to
		// the person it is made for rather than to the site's shared SSR quota
		// (backend#2264). `cf-connecting-ip` is written by Cloudflare's edge before
		// this Worker runs and is not something a client can set: Cloudflare rejects
		// an inbound request that carries it.
		const visitorIp = request.headers.get("cf-connecting-ip");
		const outcome = await resolveSellerRequest(pathname, (id) =>
			lookupSeller(id, visitorIp),
		);
		if (outcome.kind === "not-found") {
			// Two ways to get here. An id outside the backend's key range answers
			// `400 invalid seller id`, which this subtree renders as a crawler-
			// visible 500; and an id the backend settles as `404` belongs to no
			// seller, which the subtree would otherwise serve as a 200 soft-404
			// (#376). Rewriting to the app's not-found route serves the real 404
			// page; the explicit status is what the page itself cannot produce here.
			return notFoundResponse(request);
		}
		if (outcome.kind === "pass") return pass(request, pathname, locale);
		// clone() keeps the query string (utm_*, ?page=, …) attached to the
		// canonical path, so a shared deep link survives the redirect intact.
		const canonicalUrl = request.nextUrl.clone();
		canonicalUrl.pathname = `${prefix}${outcome.path}`;
		// 308, not 307: the move is permanent (that is the whole point of
		// consolidating on store_code) and the method must be preserved.
		return NextResponse.redirect(canonicalUrl, 308);
	}

	// /login is never bounced for signed-in visitors (hawkr-website#259): a
	// session cookie the monolith refuses (suspended or deleted account) would
	// loop them away from the login screen. The authoritative check is the
	// client-side GetSession; LoginPage itself redirects away once it confirms a
	// live account (see useAuth().isLoggedIn there).
	if (pathname === "/login") return pass(request, pathname, locale);

	const access = requiredAccessForPath(pathname);
	if (!access) return pass(request, pathname, locale);
	// A session cookie is enough to enter; the account behind it (and its roles)
	// is checked by the monolith on every call and by RoleZoneGuard.
	if (request.cookies.get(SESSION_COOKIE_NAME)?.value) {
		return pass(request, pathname, locale);
	}

	const loginUrl = request.nextUrl.clone();
	// The login page of the locale the visitor is in, and the way back is the URL
	// they asked for: the raw pathname, as before locales (a `%25` stays `%25`; the
	// decoded `routed` would turn it into `%`). Only leading slashes are collapsed:
	// `//host/x` as a return target would be a protocol-relative URL.
	loginUrl.pathname = `${prefix}/login`;
	loginUrl.search = "";
	// An account page belongs to no workspace: the login page keeps its default tab.
	const role = workspaceOfPath(pathname);
	if (role) loginUrl.searchParams.set("role", role);
	loginUrl.searchParams.set(
		"redirect",
		`${request.nextUrl.pathname.replace(/^\/{2,}/, "/")}${request.nextUrl.search}`,
	);
	return NextResponse.redirect(loginUrl);
}

/**
 * Site-wide, minus the requests something else already serves.
 *
 * It was six explicit prefixes until #355. Those cover every path that needs a
 * REDIRECT decision (`requiredAccessForPath` returns non-null for exactly the
 * prefixes in its access table, and `resolveSellerRequest` only looks at
 * `/sellers/`), so widening the matcher
 * cannot create a redirect that did not exist before — every newly matched path
 * falls through to `NextResponse.next()`. What it does add is the one thing that
 * has to be decided per request: whether the host answering it may be indexed.
 *
 * The exclusions, in order:
 *   `api/`        the first-party gateway proxy — the site's highest-volume
 *                 path and not indexable content, so it is kept off the
 *                 middleware path entirely.
 *   `adm/api/`    the delegated-operation BFF. Same character as `api/`, but it
 *                 does not start with it, so the first rule misses it. Keeping
 *                 it out is also structural protection: the day someone adds
 *                 `/adm` to the access table — a reasonable-looking move, it is
 *                 an authorized area after all — every BFF call would be
 *                 redirected to the login page and the caller would parse an
 *                 HTML 200 as JSON.
 *   `_next/`      build output and RSC payloads. `_next/static` is answered by
 *                 Cloudflare's asset layer before the worker runs at all
 *                 (measured — see next.config.ts's NOINDEX_HEADERS comment), so
 *                 matching it would be a no-op on the platform anyway.
 *   `public/`     its top-level directories, and the four generated root files
 *                 (`robots.txt`, `sitemap.xml`, `manifest.webmanifest`,
 *                 `icon.svg`) where an `X-Robots-Tag` would carry no meaning.
 *                 A closed list, not an extension rule: a path is kept out of
 *                 the middleware only because something serves it. A
 *                 `/favicon.ico` probe, `/zzz/products/1.png`, or an
 *                 asset-looking tail on a guarded `[param]` route therefore goes
 *                 through the middleware like any other path: it gets the login
 *                 guard and the locale rules, and having no route it gets
 *                 next-intl's cheap 404 instead of reaching the dynamic
 *                 `[locale]` route (page first, 404 after, cached per URL).
 *                 What the list still lets through is an unknown SUBPATH of an
 *                 excluded prefix (`/placeholders/products/1`, `/api/products/1`): it
 *                 reaches `[locale]=placeholders` / `api`, the same class as main's
 *                 `/products/<garbage>` (a page render and a per-URL 404), not a
 *                 new kind of cost.
 *                 The list has no nested quantifier (an extension rule written
 *                 as `.+/.*\.ext$` is quadratic: ~0.3 s for a 16k-char path).
 *                 `tests/middleware-matcher.test.ts` keeps it equal to `public/`.
 *
 * The matcher is also what lets next-intl see unprefixed paths (`as-needed`
 * rewrites `/about` to `/zh-TW/about`); the paths that must NOT be rewritten are
 * not excluded here but answered with `NextResponse.next()` in `route()`
 * (`isNonLocalizedPath`), so the host-indexability header above still reaches them.
 */
export const config = {
	matcher: [
		"/((?!api/|adm/api/|_next/|(?:brand|placeholders|templates)/|(?:robots\\.txt|sitemap\\.xml|manifest\\.webmanifest|icon\\.svg)$).*)",
	],
};
