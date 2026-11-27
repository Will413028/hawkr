/**
 * Client navigation watchdog.
 *
 * Next 16 only falls back to a hard navigation when an RSC soft-nav fetch
 * *rejects*. A hung fetch — a TCP half-open connection, a cold Worker that never
 * answers — never rejects, so a `<Link>` click can sit on the route loading
 * skeleton forever (the "一直 loading，重整才好" report). This arms a timer on
 * every router transition and, if the target route still hasn't committed when
 * it fires, forces a full-document navigation so the user is never trapped.
 *
 * It hooks Next's official `onRouterTransitionStart` instrumentation export
 * (wired in `instrumentation-client.ts`) rather than reaching into the router
 * internals, and only ever recovers a stuck transition — a soft-nav that
 * commits within the budget clears the timer as a no-op.
 *
 * Ordinary routes are considered committed once they leave the start URL. A
 * product route has a stronger contract: its target-specific, visibly rendered
 * ready marker must exist. This catches the intercepted-navigation failure where
 * history commits `/products/:id` but the Flight stream never mounts the modal.
 */
import {
	isProductDetailReady,
	productIdFromPathname,
} from "@/lib/loading/initial-route-ready";
import {
	navigateDocument,
	navigationIntentRecoveryDetails,
	reportNavigationWatchdogRecovery,
} from "@/lib/navigation/observability";
import {
	__resetNavigationRecoveryForTest,
	cancelNavigationRecovery,
	claimNavigationRecovery,
	navigationRecoveryTimeout,
	scheduleNavigationRecovery,
} from "@/lib/navigation/recovery-state";

/** Shared budgets also apply when an intent never reaches the router start hook. */
export {
	MAIN_TAB_NAVIGATION_WATCHDOG_MS,
	NAVIGATION_WATCHDOG_MS,
} from "@/lib/navigation/recovery-state";

function isSamePage(target: URL): boolean {
	return (
		target.pathname === window.location.pathname &&
		target.search === window.location.search
	);
}

function normalizedPathname(pathname: string): string {
	return pathname.length > 1 ? pathname.replace(/\/+$/, "") : pathname;
}

function isCurrentTarget(target: URL): boolean {
	return (
		normalizedPathname(target.pathname) ===
			normalizedPathname(window.location.pathname) &&
		target.search === window.location.search
	);
}

function hardNavigate(targetHref: string, replace: boolean): void {
	if (!claimNavigationRecovery()) return;
	const details = navigationIntentRecoveryDetails(targetHref);
	try {
		reportNavigationWatchdogRecovery(targetHref);
	} finally {
		navigateDocument(targetHref, {
			...details,
			replace: replace || details.replace,
			reason: "watchdog_recovery",
		});
	}
}

/**
 * Start (or restart) the watchdog for a transition to `href`. A later transition
 * supersedes an earlier one via a monotonic token so only the newest pending
 * navigation can trigger a recovery.
 */
export function armNavigationWatchdog(
	href: string,
	timeoutMs: number = navigationRecoveryTimeout(href),
): void {
	if (typeof window === "undefined") return;

	cancelNavigationRecovery();

	let target: URL;
	try {
		target = new URL(href, window.location.href);
	} catch {
		return;
	}
	if (
		target.origin !== window.location.origin ||
		!["http:", "https:"].includes(target.protocol)
	)
		return;
	if (isSamePage(target)) return;

	const startUrl = window.location.href;
	scheduleNavigationRecovery({
		timeoutMs,
		onDeadline: () => {
			const isProductTarget = productIdFromPathname(target.pathname) !== null;
			if (isProductTarget && isCurrentTarget(target)) {
				if (isProductDetailReady(target.pathname)) return;
				hardNavigate(target.href, true);
				return;
			}
			if (window.location.href !== startUrl) return;
			hardNavigate(target.href, false);
		},
	});
}

/** Test-only: drop any pending timer so state doesn't leak across cases. */
export function __resetNavigationWatchdogForTest(): void {
	__resetNavigationRecoveryForTest();
}
