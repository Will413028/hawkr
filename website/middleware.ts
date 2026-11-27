import { NextRequest, NextResponse } from "next/server";
import createIntlMiddleware from "next-intl/middleware";
import { routing } from "@/i18n/routing";
import { requiredAccessForPath, workspaceOfPath } from "@/lib/auth/redirect";
import { RETURN_PATH_HEADER } from "@/lib/auth/return-path";
import { SESSION_COOKIE_NAME } from "@/lib/auth/session-cookie";
import { getEnv } from "@/lib/config/env";
import {
	canonicalThreadPath,
	isMissingForumThread,
	lookupForumThread,
} from "@/lib/forum/thread-guard";
import { getVisitorIp } from "@/lib/http/visitor-ip";
import { type AppLocale, defaultLocale } from "@/lib/intl/locales";
import {
	isLocaleRoutableFor,
	isMetadataImagePath,
	isNonLocalizedPath,
	routingPathname,
	splitLocale,
} from "@/lib/intl/paths";
import {
	lookupStore,
	resolveStorefrontRequest,
} from "@/lib/seller/storefront-guard";
import {
	isCanonicalHost,
	NOINDEX_HEADER_NAME,
	NOINDEX_HEADER_VALUE,
} from "@/lib/seo/canonical-host";
import { isLocaleNoindex } from "@/lib/seo/indexability";

/**
 * 語系路由（ADR 2026-10-17 D1，設定與理由在 i18n/routing.ts）：zh-TW 在 `/…`，其他語系在 `/<locale>/…`。
 * next-intl 把無前綴的網址改寫成 `/zh-TW/…`（交給 `app/[locale]`）、把明寫的 `/zh-TW/…` 導向無前綴網址。
 */
const handleI18nRouting = createIntlMiddleware(routing);

/**
 * De-index every response this deployment serves under a host it does not
 * publish under (#355) — in practice the `*.vercel.app` address Vercel keeps alive
 * for every deployment, which therefore cannot simply be turned off. Full reasoning, and why neither `app/robots.ts` nor
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
 * the HTML comes from the ISR cache, and a non-indexable build already carries
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
	const routed = routingPathname(request.nextUrl.pathname);
	if (routed === null) return NextResponse.next();

	if (isNonLocalizedPath(routed)) return NextResponse.next();

	const { locale, prefix, pathname } = splitLocale(routed);

	if (locale === defaultLocale) {
		return isMetadataImagePath(pathname)
			? NextResponse.next()
			: handleI18nRouting(request);
	}

	if (locale && !isLocaleRoutableFor(locale, pathname)) {
		return notFoundResponse(request);
	}

	if (
		!getEnv().hawkrAgentEnabled &&
		pathname === "/account/messages" &&
		request.nextUrl.searchParams.has("requestId")
	) {
		const cleanUrl = request.nextUrl.clone();
		cleanUrl.searchParams.delete("requestId");
		return NextResponse.redirect(cleanUrl, 307);
	}

	if (pathname.startsWith("/sellers/")) {
		const visitorIp = getVisitorIp(request);
		const outcome = await resolveStorefrontRequest(pathname, (storeCode) =>
			lookupStore(storeCode, visitorIp),
		);
		if (outcome.kind === "not-found") {
			return notFoundResponse(request);
		}
		const threadPath = canonicalThreadPath(pathname);
		if (threadPath) {
			const threadUrl = request.nextUrl.clone();
			threadUrl.pathname = `${prefix}${threadPath}`;
			return NextResponse.redirect(threadUrl, 308);
		}
		const missing = await isMissingForumThread(
			pathname,
			outcome.storeId,
			(storeId, threadId) => lookupForumThread(storeId, threadId, visitorIp),
		);
		if (missing) return notFoundResponse(request);
		return pass(request, pathname, locale);
	}

	if (pathname === "/login") return pass(request, pathname, locale);

	const access = requiredAccessForPath(pathname);
	if (!access) return pass(request, pathname, locale);
	const returnPath = `${request.nextUrl.pathname.replace(/^\/{2,}/, "/")}${request.nextUrl.search}`;
	if (request.cookies.get(SESSION_COOKIE_NAME)?.value) {
		const headers = new Headers(request.headers);
		headers.set(RETURN_PATH_HEADER, returnPath);
		return pass(new NextRequest(request, { headers }), pathname, locale);
	}

	const loginUrl = request.nextUrl.clone();
	loginUrl.pathname = `${prefix}/login`;
	loginUrl.search = "";
	const role = workspaceOfPath(pathname);
	if (role) loginUrl.searchParams.set("role", role);
	loginUrl.searchParams.set("redirect", returnPath);
	return NextResponse.redirect(loginUrl);
}

/**
 * Site-wide, minus the requests something else already serves.
 *
 * It was six explicit prefixes until #355. Those cover every path that needs a
 * REDIRECT decision (`requiredAccessForPath` returns non-null for exactly the
 * prefixes in its access table, and `resolveStorefrontRequest` only looks at
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
 *                 the platform's static asset layer before any function runs
 *                 (see next.config.ts's NOINDEX_HEADERS comment), so
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
