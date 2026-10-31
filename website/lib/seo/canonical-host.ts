/**
 * Which hosts this deployment is willing to be indexed under (#355).
 *
 * Vercel keeps a `*.vercel.app` address alive for every deployment (preview
 * and production alike), so the platform host cannot be switched off; it can
 * only be told apart. Every `*.vercel.app` host is therefore non-canonical,
 * even when it is the host `NEXT_PUBLIC_WEB_ORIGIN` names: indexing is for the
 * custom domain.
 *
 * Telling hosts apart has to happen per request, and neither of the two places
 * that already emit indexability signals can do that:
 *
 *   - `app/robots.ts` builds from `env.seoIndexable` alone, a build-time value
 *     identical for every host the deployment answers on. Reading `headers()`
 *     there would make robots.txt a dynamic route.
 *   - `next.config.ts`'s `headers()` is static for the build; a `has: host`
 *     condition there would have to enumerate preview hosts it cannot know.
 *
 * Middleware is the remaining layer that sees the real `Host`, which is why the
 * de-indexing decision for non-canonical hosts lives there and imports this.
 *
 * DENY BY DEFAULT: anything not on the allow-list is treated as non-canonical.
 * That is only safe because the allow-list is derived from `env.webOrigin`,
 * which comes from `NEXT_PUBLIC_WEB_ORIGIN` (required for the production flavor,
 * validated as a bare origin in `lib/config/env.ts`) — the one value that says
 * which site is the real one, so canonicals, this allow-list and the CSRF check
 * can never disagree.
 */

/**
 * Hosts served by a local `next start` / `next dev`. The SEO suites
 * (`playwright.seo.config.ts`) run a real prod-flavor build over loopback and
 * assert on indexable pages, so loopback must count as canonical or those
 * assertions would be measuring the preview-host branch instead.
 */
function isLoopbackHost(hostname: string): boolean {
	return (
		hostname === "localhost" ||
		hostname === "127.0.0.1" ||
		hostname === "[::1]" ||
		hostname === "::1"
	);
}

/** Lower-cased hostname without the port, for the `Host` header's shapes. */
function hostnameOf(host: string): string {
	const trimmed = host.trim().toLowerCase();
	// IPv6 literals keep their brackets; only a trailing `:port` is stripped.
	if (trimmed.startsWith("[")) {
		const close = trimmed.indexOf("]");
		return close === -1 ? trimmed : trimmed.slice(0, close + 1);
	}
	const colon = trimmed.indexOf(":");
	return colon === -1 ? trimmed : trimmed.slice(0, colon);
}

const VERCEL_DEPLOYMENT_HOST_SUFFIX = ".vercel.app";

/** True for any `*.vercel.app` hostname (lower-cased, port already stripped). */
function isVercelDeploymentHost(hostname: string): boolean {
	return hostname.endsWith(VERCEL_DEPLOYMENT_HOST_SUFFIX);
}

/**
 * True when `host` is a hostname this deployment publishes under, i.e. one whose
 * pages may be indexed. `webOrigin` is the deployment's own origin
 * (`env.webOrigin`); the `www.` sibling is included because the custom domain
 * answers it and a redirect there should not read as a de-indexed host.
 *
 * A missing/blank `Host` is NOT canonical: every real browser and crawler sends
 * one, so its absence is an unidentifiable client and the safe answer is to
 * withhold indexing rather than grant it.
 */
export function isCanonicalHost(
	host: string | null | undefined,
	webOrigin: string,
): boolean {
	if (!host) return false;
	const hostname = hostnameOf(host);
	if (!hostname) return false;
	if (isVercelDeploymentHost(hostname)) return false;
	if (isLoopbackHost(hostname)) return true;

	let canonical: string;
	try {
		canonical = new URL(webOrigin).hostname.toLowerCase();
	} catch {
		return false;
	}
	return hostname === canonical || hostname === `www.${canonical}`;
}

/**
 * The de-indexing header, spelled exactly as `next.config.ts`'s
 * `NOINDEX_HEADERS` spells it so a response never carries two different
 * phrasings of the same instruction.
 */
export const NOINDEX_HEADER_NAME = "X-Robots-Tag";
export const NOINDEX_HEADER_VALUE = "noindex, nofollow";
