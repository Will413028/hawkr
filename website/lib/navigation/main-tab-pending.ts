import { stripLocale } from "@/lib/intl/paths";
import { isTabToTabNav } from "@/lib/navigation/route-transition-policy";

export type MainTabPendingVariant = "home" | "feed" | "market-list" | "search";

export type MainTabPendingSnapshot = Readonly<{
	id: number;
	sourcePathname: string;
	targetPathname: string;
	variant: MainTabPendingVariant;
}> | null;

// This is cleanup after the existing 20s main-tab navigation watchdog, not a
// completion signal. A slow request must not reveal the source at the progress
// rail's unrelated 4s abort deadline.
const PENDING_CLEANUP_MS = 22_000;
const variants = new Map<string, MainTabPendingVariant>([
	["/", "home"],
	["/feed", "feed"],
	["/market/products", "market-list"],
	["/search", "search"],
]);
const listeners = new Set<() => void>();
let snapshot: MainTabPendingSnapshot = null;
let nextId = 0;
let cleanupTimer: ReturnType<typeof setTimeout> | null = null;
let lifecycleInstalled = false;

function publish(next: MainTabPendingSnapshot): void {
	if (snapshot === next) return;
	snapshot = next;
	for (const listener of listeners) listener();
}

function cancelCleanupTimer(): void {
	if (cleanupTimer === null) return;
	clearTimeout(cleanupTimer);
	cleanupTimer = null;
}

function clearPending(): void {
	cancelCleanupTimer();
	publish(null);
}

function installLifecycle(): void {
	if (lifecycleInstalled) return;
	lifecycleInstalled = true;
	window.addEventListener("popstate", clearPending);
	window.addEventListener("pagehide", clearPending);
}

/** Stable snapshots let useSyncExternalStore render the locally available shell. */
export function getMainTabPendingSnapshot(): MainTabPendingSnapshot {
	return snapshot;
}

export function getMainTabPendingServerSnapshot(): MainTabPendingSnapshot {
	return null;
}

export function subscribeMainTabPending(listener: () => void): () => void {
	listeners.add(listener);
	return () => {
		listeners.delete(listener);
	};
}

/**
 * Called at Next's real navigation start, including drag-driven router.push.
 * This stores visual intent only: Next retains ownership of history, content,
 * errors and cancellation; the existing page controller retains scroll memory.
 */
export function beginMainTabPending(
	href: string,
	navigationType?: string,
): void {
	if (typeof window === "undefined") return;
	if (navigationType === "traverse") {
		clearPending();
		return;
	}

	let target: URL;
	try {
		target = new URL(href, window.location.href);
	} catch {
		clearPending();
		return;
	}
	// locale-free, like the `usePathname()` the pending surface compares them with
	const sourcePathname = stripLocale(window.location.pathname);
	const targetPathname = stripLocale(target.pathname);
	const variant = variants.get(targetPathname);
	if (
		target.origin !== window.location.origin ||
		!variant ||
		!isTabToTabNav(sourcePathname, targetPathname)
	) {
		// A source-tab re-tap or a newer non-tab destination supersedes the
		// pending shell just as it supersedes the router's earlier navigation.
		clearPending();
		return;
	}

	installLifecycle();
	cancelCleanupTimer();
	const id = ++nextId;
	cleanupTimer = setTimeout(() => {
		if (snapshot?.id === id) clearPending();
	}, PENDING_CLEANUP_MS);
	publish({ id, sourcePathname, targetPathname, variant });
}

/** Called from the content owner's layout effect, never a history insertion effect. */
export function completeMainTabPending(committedPathname: string): void {
	if (!snapshot || committedPathname === snapshot.sourcePathname) return;
	// Reaching the requested tab or any redirected destination ends this
	// surface. The initial effect for the still-current source cannot clear it.
	clearPending();
}
