import type { MarketplaceData, MarketplaceSeller } from "@/lib/api";
import type { Paginated, PostListItem } from "@/lib/api/openapi-types";
import type { CategoryGroup } from "@/lib/market/categoryGroups";

/**
 * Versioned, anonymous DTO for the home tab's first visible snapshot.
 *
 * It intentionally contains only public catalogue/discovery reads (the five
 * HomePage previously fetched independently, plus the category roll-up the
 * discovery shelves file products by). Account, cart, favourites, follows and
 * other cookie-derived fields must never be added here.
 */
export interface HomeTabBootstrapV1 {
	schemaVersion: "home.v1";
	marketplace: MarketplaceData | null;
	products: unknown;
	recommendedSellers: MarketplaceSeller[];
	posts: Paginated<PostListItem>;
	/** lv1 categories with their descendant ids, from `/public/categories/tree`
	 *  (see `lib/market/categoryGroups`). Empty when the tree was unsupplied —
	 *  the shelves then show nothing rather than guess. */
	categoryGroups: CategoryGroup[];
}

/** Display snapshot becomes stale quickly because it contains public
 * price/stock/sale fields. The longer TTL is only a stale fallback retained in
 * memory so a tab return can paint immediately while this refreshes. */
export const HOME_TAB_BOOTSTRAP_FRESH_MS = 60 * 1000;
export const HOME_TAB_BOOTSTRAP_TTL_MS = 5 * 60 * 1000;
/**
 * A degraded answer (see `degraded` / `x-bootstrap-degraded` on
 * `/api/public/tab-bootstrap/v1/home`) is trusted for seconds, not a minute.
 *
 * The route only labels a domain degraded when no source could supply it, so the
 * payload carries a known gap. Holding it for the full fresh window would make
 * the very outage the label announces last a minute longer than the outage
 * itself.
 */
export const HOME_TAB_BOOTSTRAP_DEGRADED_FRESH_MS = 5 * 1000;
const TAB_DATA_WARM_TTL_MS = 30 * 1000;
const TAB_DATA_WARM_DEADLINE_MS = 4_500;

/** Domains the route can report as unsupplied, and therefore the only keys a
 *  degraded payload may carry forward from the previous answer. */
const RETAINABLE_DOMAINS = [
	"marketplace",
	"products",
	"posts",
	"recommendedSellers",
	"categoryGroups",
] as const;
type RetainableDomain = (typeof RETAINABLE_DOMAINS)[number];

let memoryEntry:
	| {
			payload: HomeTabBootstrapV1;
			freshUntil: number;
			expiresAt: number;
			/**
			 * Until when this entry's domain values may be carried into a later
			 * degraded answer. Set from the last intact read and never extended by a
			 * degraded one, so retained content cannot be chained forward past one
			 * TTL of real age while an upstream stays down.
			 */
			retainableUntil: number;
	  }
	| undefined;
let inFlight: Promise<HomeTabBootstrapV1> | undefined;
const dataWarmEntries = new Map<
	string,
	{ promise: Promise<void>; expiresAt: number }
>();

/**
 * Which domains this answer admits it could not supply.
 *
 * Read from the body label first and the header second: they are emitted
 * together, and honouring either means an intermediary that strips one still
 * cannot turn a labelled gap back into a silent one. Unknown names are dropped
 * so a future domain cannot be used to overwrite an unrelated field.
 */
function readDegradedDomains(
	response: Response,
	payload: { degraded?: unknown },
): RetainableDomain[] {
	const labelled = new Set<string>(
		(Array.isArray(payload.degraded) ? payload.degraded : []).map(String),
	);
	const header = response.headers.get("x-bootstrap-degraded") ?? "";
	for (const name of header.split(",")) {
		const trimmed = name.trim();
		if (trimmed) labelled.add(trimmed);
	}
	return RETAINABLE_DOMAINS.filter((domain) => labelled.has(domain));
}

/**
 * Keep the last known content for the domains this answer could not supply.
 *
 * The route fills a degraded domain with an empty value, which renders as "no
 * sellers" / "nothing posted" — indistinguishable from the truth. Since only the
 * labelled keys are empty, replacing exactly those with the previous answer's
 * values yields a snapshot whose commerce half is fresh and whose unsupplied half
 * is the last real content, instead of a blank rail cached as the new truth.
 */
function carryForwardDegraded(
	payload: HomeTabBootstrapV1,
	previous: HomeTabBootstrapV1,
	degraded: readonly RetainableDomain[],
): HomeTabBootstrapV1 {
	const merged: HomeTabBootstrapV1 = { ...payload };
	const writable = merged as Record<RetainableDomain, unknown>;
	// Same key on both sides, so every assignment keeps the field's declared type.
	for (const domain of degraded) writable[domain] = previous[domain];
	return merged;
}

async function requestHomeTabBootstrap(): Promise<HomeTabBootstrapV1> {
	const response = await fetch("/api/public/tab-bootstrap/v1/home", {
		method: "GET",
		credentials: "omit",
		cache: "no-store",
		headers: { accept: "application/json" },
	});
	if (!response.ok) {
		throw new Error(`home bootstrap failed (${response.status})`);
	}
	const payload = (await response.json()) as Partial<HomeTabBootstrapV1> & {
		degraded?: unknown;
	};
	if (payload.schemaVersion !== "home.v1") {
		throw new Error("unsupported home bootstrap schema");
	}
	const validated = payload as HomeTabBootstrapV1;
	const now = Date.now();
	const degraded = readDegradedDomains(response, payload);
	const previous = memoryEntry;
	const isDegraded = degraded.length > 0;
	const retainable = Boolean(previous && previous.retainableUntil > now);
	const resolved =
		isDegraded && previous && retainable
			? carryForwardDegraded(validated, previous.payload, degraded)
			: validated;
	memoryEntry = {
		payload: resolved,
		freshUntil:
			now +
			(isDegraded
				? HOME_TAB_BOOTSTRAP_DEGRADED_FRESH_MS
				: HOME_TAB_BOOTSTRAP_FRESH_MS),
		expiresAt: now + HOME_TAB_BOOTSTRAP_TTL_MS,
		// A degraded answer inherits the retention clock rather than restarting it:
		// the content it carries forward was fetched at the last intact read, so its
		// age is measured from there. Once that window closes the gap is served as
		// it is — retaining forever would show content that may no longer exist.
		retainableUntil: isDegraded
			? (previous?.retainableUntil ?? 0)
			: now + HOME_TAB_BOOTSTRAP_TTL_MS,
	};
	return resolved;
}

function sharedHomeTabBootstrap(options?: {
	requireFresh?: boolean;
}): Promise<HomeTabBootstrapV1> {
	const now = Date.now();
	if (memoryEntry && memoryEntry.freshUntil > now) {
		return Promise.resolve(memoryEntry.payload);
	}
	if (!inFlight) {
		inFlight = requestHomeTabBootstrap().finally(() => {
			inFlight = undefined;
		});
	}
	// Intent/idle preloading is stale-while-revalidate: keep the instant memory
	// hit, but start one shared network refresh. The active Home query passes
	// requireFresh and awaits that same flight before atomically replacing data.
	if (memoryEntry && memoryEntry.expiresAt > now && !options?.requireFresh) {
		void inFlight.catch(() => undefined);
		return Promise.resolve(memoryEntry.payload);
	}
	return inFlight;
}

function abortError(signal: AbortSignal): unknown {
	return (
		signal.reason ??
		new DOMException("The operation was aborted.", "AbortError")
	);
}

/**
 * Give each consumer its own abort boundary without attaching that signal to
 * the shared network request. A route transition can stop waiting while a
 * concurrent Navbar preload still completes and populates the memory cache.
 */
function withCallerAbort<T>(
	promise: Promise<T>,
	signal?: AbortSignal,
): Promise<T> {
	if (!signal) return promise;
	if (signal.aborted) return Promise.reject(abortError(signal));
	return new Promise<T>((resolve, reject) => {
		const onAbort = () => reject(abortError(signal));
		signal.addEventListener("abort", onAbort, { once: true });
		void promise.then(resolve, reject).finally(() => {
			signal.removeEventListener("abort", onAbort);
		});
	});
}

export function fetchHomeTabBootstrap(
	signal?: AbortSignal,
): Promise<HomeTabBootstrapV1> {
	if (signal?.aborted) return Promise.reject(abortError(signal));
	return withCallerAbort(
		sharedHomeTabBootstrap({ requireFresh: true }),
		signal,
	);
}

/** Intent-prefetch entrypoint used by the primary Navbar. */
export function preloadHomeTabBootstrap(): Promise<HomeTabBootstrapV1> {
	return sharedHomeTabBootstrap();
}

const DATA_WARM_TAB_BY_ROUTE = {
	"/market/products": "market",
	"/search": "search",
} as const;

/**
 * Warm only the server data dependencies during idle time.
 *
 * This deliberately avoids App Router prefetch: route CSS/JS should be loaded
 * only after a real hover/focus dwell or the navigation itself. Initial idle
 * warming uses bounded anonymous reads to prime safe, reusable server
 * dependencies. Feed is deliberately excluded because its private direct read
 * cannot populate reusable state. Touch pointer-down starts no request so the
 * real navigation owns the connection.
 */
export function preloadMainTabData(href: string): Promise<void> {
	if (href === "/") {
		return preloadHomeTabBootstrap().then(() => undefined);
	}
	const tab =
		DATA_WARM_TAB_BY_ROUTE[href as keyof typeof DATA_WARM_TAB_BY_ROUTE];
	if (!tab) return Promise.resolve();

	const now = Date.now();
	const existing = dataWarmEntries.get(href);
	if (existing && existing.expiresAt > now) return existing.promise;
	if (existing) dataWarmEntries.delete(href);

	const controller = new AbortController();
	const deadline = setTimeout(
		() => controller.abort(),
		TAB_DATA_WARM_DEADLINE_MS,
	);
	const promise = fetch(
		`/api/public/tab-bootstrap/v1/warm?tab=${encodeURIComponent(tab)}`,
		{
			method: "GET",
			credentials: "omit",
			cache: "no-store",
			headers: { accept: "application/json" },
			signal: controller.signal,
		},
	)
		.then((response) => {
			if (!response.ok) {
				throw new Error(`tab data warm failed (${response.status})`);
			}
		})
		.catch((error) => {
			dataWarmEntries.delete(href);
			throw error;
		})
		.finally(() => clearTimeout(deadline));

	dataWarmEntries.set(href, {
		promise,
		expiresAt: now + TAB_DATA_WARM_TTL_MS,
	});
	return promise;
}
