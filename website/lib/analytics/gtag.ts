/**
 * The single seam to GA4's gtag.js. Everything is gated on `env.gaId`
 * (empty → disabled, so dev/stage never emit). Dispatch goes through
 * `window.gtag` directly once bootstrapped. Commands emitted before bootstrap
 * stay in a module queue; `bootstrapGtag()` writes the required js/config
 * commands first, then flushes them so the privacy-normalized manual page_view
 * cannot be lost or appear before the GA4 configuration. Automatic page views
 * stay disabled: gtag would otherwise emit the raw browser URL before the App
 * Router observer can remove request/conversation identifiers.
 */

import { getEnv } from "@/lib/config/env";
import {
	isAssistantAnalyticsRoute,
	privacySafePageReferrer,
	privacySafePageTitle,
	privacySafePageViewLocation,
} from "./page-view-privacy";
import type { GA4EventParams } from "./types";

type Gtag = (...args: unknown[]) => void;

interface GtagWindow {
	gtag?: Gtag;
	dataLayer?: unknown[];
}

export interface AnalyticsPageContext {
	page_path: string;
	page_location: string;
	page_referrer: string;
	page_title: string;
}

let pendingCommands: unknown[][] = [];
let bootstrappedGaId = "";

export function isEnabled(): boolean {
	return getEnv().gaId.length > 0;
}

function gtagSafe(...args: unknown[]): void {
	if (typeof window === "undefined") return;
	const w = window as unknown as GtagWindow;
	if (typeof w.gtag === "function") {
		w.gtag(...args);
		return;
	}
	pendingCommands.push(args);
}

function currentSafePageContext(): AnalyticsPageContext | null {
	if (typeof window === "undefined") return null;
	const location = (
		window as unknown as {
			location?: Pick<Location, "origin" | "pathname" | "search">;
		}
	).location;
	if (!location) return null;
	const { pagePath, pageLocation } = privacySafePageViewLocation({
		pathname: location.pathname,
		query: location.search.replace(/^\?/, ""),
		origin: location.origin,
	});
	const referrer =
		isAssistantAnalyticsRoute(location.pathname) ||
		typeof document === "undefined"
			? ""
			: document.referrer;
	return {
		page_path: pagePath,
		page_location: pageLocation,
		page_referrer: privacySafePageReferrer(referrer, location.origin),
		page_title: privacySafePageTitle(
			typeof document === "undefined" ? "" : document.title,
			location.pathname,
		),
	};
}

/**
 * Establish the GA command queue without inserting any DOM nodes. Analytics
 * calls that occurred during hydration are flushed only after js/config, while
 * the external gtag.js script itself may load later behind the document barrier.
 */
export function bootstrapGtag(gaId: string): void {
	if (typeof window === "undefined" || !gaId) return;
	if (bootstrappedGaId === gaId) return;

	const w = window as unknown as GtagWindow;
	w.dataLayer ??= [];
	w.gtag ??= function gtag() {
		// Google documents gtag as dataLayer.push(arguments). Keep the native
		// Arguments object instead of substituting a rest-parameter Array.
		// biome-ignore lint/complexity/noArguments: gtag.js requires the documented native Arguments queue entry.
		w.dataLayer?.push(arguments);
	};
	w.gtag("set", "allow_ad_personalization_signals", false);
	w.gtag("js", new Date());
	const initialPageContext = currentSafePageContext();
	w.gtag("config", gaId, {
		send_page_view: false,
		...(initialPageContext ?? {}),
	});
	for (const command of pendingCommands.splice(0)) {
		w.gtag(...command);
	}
	bootstrappedGaId = gaId;
}

/**
 * Update the Google tag's SPA context before sending the virtual page_view.
 * Automatic events emitted afterwards inherit only the sanitized page values.
 */
export function updatePageContext(context: AnalyticsPageContext): void {
	if (!isEnabled()) return;
	gtagSafe("config", getEnv().gaId, {
		update: true,
		send_page_view: false,
		...context,
	});
}

/** Fire a GA4 event. No-op when disabled. */
export function track(eventName: string, params: GA4EventParams = {}): void {
	if (!isEnabled()) return;
	let safeParams = params;
	if (typeof window !== "undefined") {
		const location = (
			window as unknown as {
				location?: Pick<Location, "origin" | "pathname" | "search">;
			}
		).location;
		if (location) {
			const { pagePath, pageLocation } = privacySafePageViewLocation({
				pathname: location.pathname,
				query: location.search.replace(/^\?/, ""),
				origin: location.origin,
			});
			const requestedReferrer = isAssistantAnalyticsRoute(location.pathname)
				? ""
				: typeof params.page_referrer === "string"
					? params.page_referrer
					: typeof document === "undefined"
						? ""
						: document.referrer;
			const requestedTitle =
				typeof params.page_title === "string"
					? params.page_title
					: typeof document === "undefined"
						? ""
						: document.title;
			// GA automatically attaches page context to every event. Override it at
			// the single dispatch seam so callers and the browser's raw URL cannot
			// reintroduce account, order, token or contact identifiers.
			safeParams = {
				...params,
				page_path: pagePath,
				page_location: pageLocation,
				page_referrer: privacySafePageReferrer(
					requestedReferrer,
					location.origin,
				),
				page_title: privacySafePageTitle(requestedTitle, location.pathname),
			};
		}
	}
	gtagSafe("event", eventName, safeParams);
}

/** Set categorical GA4 user properties (role, login_method, …). */
export function setUserProperties(props: Record<string, string>): void {
	if (!isEnabled()) return;
	gtagSafe("set", "user_properties", props);
}

/**
 * Consent Mode v2 update hook. Unused by default (no banner, full analytics);
 * a future banner calls this on the user's choice. To switch to a
 * consent-gated model, also emit gtag("consent","default",{denied}) before the
 * bootstrap config command.
 */
export function setConsent(granted: boolean): void {
	if (!isEnabled()) return;
	const state = granted ? "granted" : "denied";
	gtagSafe("consent", "update", {
		analytics_storage: state,
		ad_storage: state,
		ad_user_data: state,
		ad_personalization: state,
	});
}

/** Test-only reset for the module-scoped pre-bootstrap command queue. */
export function __resetGtagForTest(): void {
	pendingCommands = [];
	bootstrappedGaId = "";
}
