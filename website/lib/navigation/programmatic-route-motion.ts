import { stripLocale } from "@/lib/intl/paths";
import { resolveNavigationUrl } from "@/lib/navigation/resolve-url";
import { shouldAnimateProgrammaticRoute } from "@/lib/navigation/route-transition-policy";

const INTENT_TTL_MS = 4000;

interface ProgrammaticRouteMotionIntent {
	token: number;
	targetPath: string;
	expiresAt: number;
}

let pendingIntent: ProgrammaticRouteMotionIntent | null = null;
let nextToken = 0;
let pageHideListenerAttached = false;

function clearPendingIntent() {
	pendingIntent = null;
}

function attachPageHideCleanup() {
	if (pageHideListenerAttached || typeof window === "undefined") return;
	window.addEventListener("pagehide", clearPendingIntent);
	pageHideListenerAttached = true;
}

function sameOriginTarget(target: string): URL | null {
	if (typeof window === "undefined") return null;
	const url = resolveNavigationUrl(target);
	if (!url || url.origin !== window.location.origin) return null;
	return url;
}

/** Arm one destination commit. A newer navigation always supersedes it. */
export function armProgrammaticRouteMotion(target: string): number | null {
	clearPendingIntent();
	const targetUrl = sameOriginTarget(target);
	if (!targetUrl) return null;
	if (
		!shouldAnimateProgrammaticRoute(
			window.location.pathname,
			targetUrl.pathname,
		)
	) {
		return null;
	}

	attachPageHideCleanup();
	const token = ++nextToken;
	pendingIntent = {
		token,
		targetPath: stripLocale(targetUrl.pathname),
		expiresAt: Date.now() + INTENT_TTL_MS,
	};
	return token;
}

/** Cancel only the navigation that armed this token. */
export function cancelProgrammaticRouteMotion(token: number | null) {
	if (token !== null && pendingIntent?.token === token) clearPendingIntent();
}

/**
 * Consume after the pathname has committed. Redirects, stale intents and
 * mismatched commits fail closed so an unrelated screen never animates.
 */
export function consumeProgrammaticRouteMotion(committedPath: string): boolean {
	const intent = pendingIntent;
	clearPendingIntent();
	if (!intent || intent.expiresAt < Date.now()) return false;
	return intent.targetPath === committedPath;
}

export function resetProgrammaticRouteMotionForTests() {
	clearPendingIntent();
	nextToken = 0;
}
