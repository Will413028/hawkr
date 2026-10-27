/**
 * In-app history marker for BackLink.
 *
 * `window.history.length > 1` was never a proxy for "there is an in-app page to
 * go back to": a redirect on entry, a deep link that immediately `replace()`s,
 * or simply opening the tab on another site all push the length past 1, and
 * `router.back()` then walks the user OUT of the site instead of using the
 * component's `fallbackHref`.
 *
 * Instead, the SPA stamps `history.length` ONCE per tab (session storage
 * survives reloads within the tab) at the moment site chrome first mounts.
 * Anything above that stamp is an entry this app pushed, so back is safe.
 *
 * The second guard is the destination: popping back onto /login, /register or a
 * checkout result page lands on a route that redirects straight back out —
 * a visible bounce, sometimes a loop. Those are recorded as the site navigates
 * and skipped in favour of `fallbackHref`.
 *
 * Every accessor is storage-failure tolerant: Safari private mode and
 * "block all cookies" throw on `sessionStorage` access, and a back button is
 * not worth a render crash. On failure the caller degrades to `fallbackHref`.
 */

const ENTRY_INDEX_KEY = "hk:entry-index";
const CURRENT_PATH_KEY = "hk:current-pathname";
const PREVIOUS_PATH_KEY = "hk:previous-pathname";

/**
 * Routes that immediately redirect an arriving user elsewhere. Going "back"
 * onto one of them is never what the user asked for.
 */
const BOUNCE_PREFIXES = [
	"/login",
	"/register",
	"/checkout/success",
	"/checkout/fail",
	"/checkout/submitted",
];

function readStore(): Storage | null {
	if (typeof window === "undefined") return null;
	try {
		return window.sessionStorage;
	} catch {
		return null;
	}
}

/** Stamps the history depth this tab started the app at. No-op after the first call. */
export function markAppEntry(): void {
	const store = readStore();
	if (!store) return;
	try {
		if (store.getItem(ENTRY_INDEX_KEY) !== null) return;
		store.setItem(ENTRY_INDEX_KEY, String(window.history.length));
	} catch {
		// Quota / disabled storage: BackLink falls back to its href.
	}
}

/** Records the route the app is on, keeping the one before it as `previous`. */
export function recordPathname(pathname: string): void {
	const store = readStore();
	if (!store || !pathname) return;
	try {
		const current = store.getItem(CURRENT_PATH_KEY);
		if (current === pathname) return;
		if (current) store.setItem(PREVIOUS_PATH_KEY, current);
		store.setItem(CURRENT_PATH_KEY, pathname);
	} catch {
		// Ignore — see markAppEntry.
	}
}

export function isBouncePathname(pathname: string | null): boolean {
	if (!pathname) return false;
	return BOUNCE_PREFIXES.some(
		(prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`),
	);
}

/** The route the user was on before the current one, as far as this tab saw. */
export function previousPathname(): string | null {
	const store = readStore();
	if (!store) return null;
	try {
		return store.getItem(PREVIOUS_PATH_KEY);
	} catch {
		return null;
	}
}

/**
 * True when `router.back()` is guaranteed to land on a page this app pushed,
 * and that page is not one of the bounce routes.
 */
export function canGoBackInApp(): boolean {
	if (typeof window === "undefined") return false;
	const store = readStore();
	if (!store) return false;
	let entryIndex: string | null;
	try {
		entryIndex = store.getItem(ENTRY_INDEX_KEY);
	} catch {
		return false;
	}
	if (entryIndex === null) return false;
	const entry = Number.parseInt(entryIndex, 10);
	if (!Number.isFinite(entry)) return false;
	if (window.history.length <= entry) return false;
	return !isBouncePathname(previousPathname());
}
