/**
 * Main-tab App Router prefetch coordinator.
 *
 * Navbar links deliberately opt out of the shared Link's per-instance intent
 * prefetch and come through here instead. The desktop and mobile navbars are
 * mounted together, so a module-level coordinator is the smallest place that
 * can deduplicate their work.
 *
 * This only warms the four anonymous main tabs. Auth/account destinations keep
 * their existing hard-navigation and role-boundary behaviour.
 */

import { preloadHomeTabBootstrap } from "@/lib/api/tab-bootstrap";
import { whenDocumentSettled } from "@/lib/hydration/document-settled";
import { stripLocale } from "@/lib/intl/paths";
import type { useRouter } from "@/lib/intl/use-router";
import {
	isPublicInitialContentReady,
	ROUTE_TERMINAL_READY_SELECTOR,
} from "@/lib/loading/initial-route-ready";
import { isNavPathActive } from "@/lib/navigation/active-link";

export const MAIN_NAV_PREFETCH_ROUTES = [
	"/",
	"/feed",
	"/market/products",
	"/search",
] as const;

export type MainNavPrefetchRoute = (typeof MAIN_NAV_PREFETCH_ROUTES)[number];
export type MainNavPrefetchIntent = "pointer" | "focus" | "touch" | "idle";

type PrefetchRouter = Pick<ReturnType<typeof useRouter>, "prefetch">;
type PrefetchOptions = NonNullable<Parameters<PrefetchRouter["prefetch"]>[1]>;

// Next types this public option with an internal enum. Keep the dependency
// type-only; do not import its router implementation into the application.
const HOME_PREFETCH_OPTIONS: PrefetchOptions = {
	kind: "full" as PrefetchOptions["kind"],
};

type NetworkInformationLike = {
	effectiveType?: string;
	saveData?: boolean;
};

type NavigatorWithConnection = Navigator & {
	connection?: NetworkInformationLike;
	mozConnection?: NetworkInformationLike;
	webkitConnection?: NetworkInformationLike;
};

type WarmedRoute = {
	expiresAt: number;
};

type ScheduledIntent = {
	href: MainNavPrefetchRoute;
	timer: ReturnType<typeof setTimeout>;
};

type IdleCapableWindow = Window & {
	requestIdleCallback?: (
		callback: (deadline: {
			didTimeout: boolean;
			timeRemaining: () => number;
		}) => void,
		options?: { timeout: number },
	) => number;
	cancelIdleCallback?: (handle: number) => void;
};

const routeSet = new Set<string>(MAIN_NAV_PREFETCH_ROUTES);
const POINTER_DWELL_MS = 80;
const IDLE_ROUTE_GAP_MS = 240;
// Match the short-lived App Router dynamic prefetch window. This prevents a
// long-lived shell from treating an old warm as fresh forever while still
// single-flighting repeated hover/focus events during real interaction.
// Touch intentionally starts no request below: pointer-down and the resulting
// click are the same gesture, so every speculative request started there takes
// priority away from the real navigation. Idle warming prepares the anonymous
// route payload before the gesture; hover/focus share its deduplication.
const PREFETCH_DEDUPE_TTL_MS = 30 * 1000;

const warmedRoutes = new Map<MainNavPrefetchRoute, WarmedRoute>();
const idleSuppressedRoutes = new Set<MainNavPrefetchRoute>();
let scheduledIntent: ScheduledIntent | null = null;
let scheduledIdleWarmCleanup: (() => void) | null = null;

function asMainNavRoute(href: string): MainNavPrefetchRoute | null {
	return routeSet.has(href) ? (href as MainNavPrefetchRoute) : null;
}

function connectionInfo(): NetworkInformationLike | undefined {
	if (typeof navigator === "undefined") return undefined;
	const nav = navigator as NavigatorWithConnection;
	return nav.connection ?? nav.mozConnection ?? nav.webkitConnection;
}

/**
 * Save-Data, offline and 2G are hard stops. Hover is a weaker signal than
 * focus, so on 3G only explicit keyboard focus can warm the destination.
 * Touch never starts a speculative request.
 */
export function canPrefetchMainNav(intent: MainNavPrefetchIntent): boolean {
	if (typeof navigator === "undefined") return false;
	if (navigator.onLine === false) return false;

	const connection = connectionInfo();
	if (connection?.saveData) return false;

	const effectiveType = connection?.effectiveType?.toLowerCase();
	if (effectiveType === "slow-2g" || effectiveType === "2g") return false;
	if ((intent === "pointer" || intent === "idle") && effectiveType === "3g") {
		return false;
	}

	return true;
}

function cancelScheduledIntent(href?: MainNavPrefetchRoute) {
	if (!scheduledIntent || (href && scheduledIntent.href !== href)) return;
	clearTimeout(scheduledIntent.timer);
	scheduledIntent = null;
}

function warmRoute(
	router: PrefetchRouter,
	href: MainNavPrefetchRoute,
	currentPathname: string,
	intent: MainNavPrefetchIntent,
) {
	if (isNavPathActive(currentPathname, href) || !canPrefetchMainNav(intent)) {
		return;
	}

	if (intent === "touch") return;

	const now = Date.now();
	const existing = warmedRoutes.get(href);
	if (existing && existing.expiresAt > now) return;
	if (existing) warmedRoutes.delete(href);

	warmedRoutes.set(href, {
		expiresAt: now + PREFETCH_DEDUPE_TTL_MS,
	});

	if (href === "/" && intent !== "idle") {
		// Retain the explicit-intent client snapshot warm. Idle prefetch already
		// prepares the SSR snapshot and must not add a second data request.
		void preloadHomeTabBootstrap().catch(() => undefined);
	}

	try {
		if (href === "/") {
			// The initial Home tree is already known to Next, but this deployment
			// serves full Flight responses to its automatic segment prefetches.
			// Full prefetch can reuse the rendered tree or cache a compatible Flight
			// instead. Keep other tabs' working automatic prefetch path unchanged.
			router.prefetch(href, HOME_PREFETCH_OPTIONS);
		} else {
			router.prefetch(href);
		}
	} catch {
		// Prefetch is an optional enhancement. A synchronous router failure must
		// neither affect the click nor suppress the next valid warm attempt.
		warmedRoutes.delete(href);
	}
}

export function prefetchMainNavOnIntent({
	router,
	href,
	currentPathname,
	intent,
}: {
	router: PrefetchRouter;
	href: string;
	currentPathname: string;
	intent: MainNavPrefetchIntent;
}) {
	if (intent === "touch") {
		// Stop the entire remaining batch before a tap can begin navigation.
		// Next does not expose cancellation for requests already in flight.
		scheduledIdleWarmCleanup?.();
		cancelScheduledIntent();
		return;
	}
	const route = asMainNavRoute(href);
	if (
		!route ||
		isNavPathActive(currentPathname, route) ||
		!canPrefetchMainNav(intent)
	) {
		return;
	}

	if (intent !== "idle") idleSuppressedRoutes.add(route);

	if (intent === "pointer") {
		cancelScheduledIntent();
		scheduledIntent = {
			href: route,
			timer: setTimeout(() => {
				scheduledIntent = null;
				warmRoute(router, route, currentPathname, intent);
			}, POINTER_DWELL_MS),
		};
		return;
	}

	// Focus is an explicit prefetch intent. Touch only cancels/suppresses weaker
	// speculation so the real click owns the connection.
	cancelScheduledIntent();
	warmRoute(router, route, currentPathname, intent);
}

/**
 * Warm the other anonymous tabs once the current canonical tab is fully
 * loaded and the browser has spare time. Routes outside this exact four-tab
 * shell (auth/account/seller/checkout/etc.) never start background work.
 */
export function scheduleMainNavIdleWarm({
	router,
	currentPathname,
}: {
	router: PrefetchRouter;
	currentPathname: string;
}): () => void {
	scheduledIdleWarmCleanup?.();
	idleSuppressedRoutes.clear();

	let cancelled = false;
	let idleHandle: number | null = null;
	let routeTimer: ReturnType<typeof setTimeout> | null = null;
	let fallbackTimer: ReturnType<typeof setTimeout> | null = null;
	let readyTimeout: ReturnType<typeof setTimeout> | null = null;
	let readyObserver: MutationObserver | null = null;
	let navigationObserver: MutationObserver | null = null;
	const idleWindow =
		typeof window === "undefined" ? null : (window as IdleCapableWindow);

	const stopOnLink = (event: Event) => {
		if (event.target instanceof Element && event.target.closest("a[href]")) {
			cleanup();
		}
	};
	const cleanup = () => {
		cancelled = true;
		if (idleHandle !== null && idleWindow?.cancelIdleCallback) {
			idleWindow.cancelIdleCallback(idleHandle);
			idleHandle = null;
		}
		if (routeTimer !== null) clearTimeout(routeTimer);
		if (fallbackTimer !== null) clearTimeout(fallbackTimer);
		if (readyTimeout !== null) clearTimeout(readyTimeout);
		routeTimer = fallbackTimer = readyTimeout = null;
		readyObserver?.disconnect();
		readyObserver = null;
		navigationObserver?.disconnect();
		navigationObserver = null;
		idleWindow?.removeEventListener("pointerdown", stopOnLink, true);
		idleWindow?.removeEventListener("click", stopOnLink, true);
		idleWindow?.removeEventListener("popstate", cleanup);
		idleWindow?.removeEventListener("pagehide", cleanup);
		if (scheduledIdleWarmCleanup === cleanup) {
			idleSuppressedRoutes.clear();
			scheduledIdleWarmCleanup = null;
		}
	};
	scheduledIdleWarmCleanup = cleanup;

	if (
		!idleWindow ||
		typeof document === "undefined" ||
		typeof MutationObserver === "undefined" ||
		!asMainNavRoute(currentPathname)
	) {
		return cleanup;
	}

	const canWarm = () =>
		!cancelled &&
		document.visibilityState === "visible" &&
		stripLocale(idleWindow.location.pathname) === currentPathname &&
		document.documentElement.getAttribute("data-nav-pending") !== "" &&
		canPrefetchMainNav("idle");
	if (
		document.visibilityState !== "visible" ||
		stripLocale(idleWindow.location.pathname) !== currentPathname ||
		!canPrefetchMainNav("idle")
	) {
		return cleanup;
	}
	// A pathname effect may run before the previous transition's progress hold
	// clears. Wait for that handoff; a later navigation cancels this new batch.
	let waitingForCommitHandoff =
		document.documentElement.getAttribute("data-nav-pending") === "";

	// Pathname effects only clean up after commit. Cancel queued work at the
	// beginning of navigation, including programmatic push/replace and Back.
	navigationObserver = new MutationObserver(() => {
		const pending =
			document.documentElement.getAttribute("data-nav-pending") === "";
		if (waitingForCommitHandoff) {
			if (!pending) {
				waitingForCommitHandoff = false;
				if (readyTimeout !== null) clearTimeout(readyTimeout);
				readyTimeout = null;
				waitForContent();
			}
		} else if (pending) {
			cleanup();
		}
	});
	navigationObserver.observe(document.documentElement, {
		attributes: true,
		attributeFilter: ["data-nav-pending"],
	});
	idleWindow.addEventListener("pointerdown", stopOnLink, true);
	idleWindow.addEventListener("click", stopOnLink, true);
	idleWindow.addEventListener("popstate", cleanup);
	idleWindow.addEventListener("pagehide", cleanup);

	const warmRemainingRoutes = () => {
		const remainingRoutes = MAIN_NAV_PREFETCH_ROUTES.filter(
			(route) => route !== currentPathname,
		);
		const warmNext = (index: number) => {
			if (!canWarm() || index >= remainingRoutes.length) {
				cleanup();
				return;
			}
			const route = remainingRoutes[index];
			routeTimer = setTimeout(
				() => {
					routeTimer = null;
					if (!canWarm()) {
						cleanup();
						return;
					}
					if (!idleSuppressedRoutes.has(route)) {
						warmRoute(router, route, currentPathname, "idle");
					}
					// Prefetch returns void. Space at most three calls; the router
					// owns network scheduling, and already-started work cannot be
					// aborted. Do not add duplicate API-only warming alongside it.
					warmNext(index + 1);
				},
				index === 0 ? 0 : IDLE_ROUTE_GAP_MS,
			);
		};
		warmNext(0);
	};

	const requestIdleWarm = () => {
		if (!canWarm()) {
			cleanup();
			return;
		}
		if (idleWindow.requestIdleCallback) {
			idleHandle = idleWindow.requestIdleCallback(warmRemainingRoutes, {
				timeout: 1_500,
			});
		} else {
			fallbackTimer = setTimeout(warmRemainingRoutes, 250);
		}
	};

	const activeContentReady = () =>
		!document.querySelector(ROUTE_TERMINAL_READY_SELECTOR) &&
		isPublicInitialContentReady(currentPathname);

	// Route prefetch may insert CSS into the document. Use the same hydration
	// barrier as other head mutations, then require this tab's committed content
	// (a marker in React's hidden Flight staging tree is not ready).
	const waitForContent = () => {
		void whenDocumentSettled().then(() => {
			if (!canWarm()) {
				cleanup();
				return;
			}
			if (activeContentReady()) {
				requestIdleWarm();
				return;
			}
			readyObserver = new MutationObserver(() => {
				if (!activeContentReady()) return;
				readyObserver?.disconnect();
				readyObserver = null;
				if (readyTimeout !== null) clearTimeout(readyTimeout);
				readyTimeout = null;
				requestIdleWarm();
			});
			readyObserver.observe(document.documentElement, {
				childList: true,
				subtree: true,
				attributes: true,
				attributeFilter: ["hidden"],
			});
			// A broken/terminal page never earns speculative work.
			readyTimeout = setTimeout(cleanup, 10_000);
		});
	};

	if (waitingForCommitHandoff) {
		readyTimeout = setTimeout(cleanup, 10_000);
	} else {
		waitForContent();
	}

	return cleanup;
}

export function cancelMainNavPointerIntent(href: string) {
	const route = asMainNavRoute(href);
	if (route) cancelScheduledIntent(route);
}

/**
 * Test-only reset kept exported so focused unit tests can isolate the
 * module-level desktop/mobile coordinator without relying on module reloads.
 */
export function resetMainNavPrefetchCoordinatorForTests() {
	scheduledIdleWarmCleanup?.();
	cancelScheduledIntent();
	warmedRoutes.clear();
	idleSuppressedRoutes.clear();
}
