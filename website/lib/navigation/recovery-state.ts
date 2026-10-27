import { stripLocale } from "@/lib/intl/paths";

/** Shared ownership for pre-start and in-flight navigation recovery. No telemetry dependency. */
export const NAVIGATION_WATCHDOG_MS = 10_000;
/** Main-tab Flight streams need the existing longer budget under slow mobile networks. */
export const MAIN_TAB_NAVIGATION_WATCHDOG_MS = 20_000;
const RECOVERY_STORAGE_KEY = "__hk_navigation_recovery_used";
const MAIN_TAB_PATHS = new Set([
	"/",
	"/feed",
	"/market",
	"/market/products",
	"/search",
]);

export type NavigationLifecycleReason =
	| "interaction"
	| "hidden"
	| "pagehide"
	| "pageshow"
	| "history"
	| "offline"
	| "online";

export interface NavigationLifecycleEvent {
	reason: NavigationLifecycleReason;
	persisted?: boolean;
}

let generation = 0;
let timer: ReturnType<typeof setTimeout> | undefined;
let cancelPending: ((reason: NavigationLifecycleReason) => void) | undefined;
let lifecycleObserver: ((event: NavigationLifecycleEvent) => void) | undefined;
let removeListeners: (() => void) | undefined;
let recoveryUsed = false;
let documentLeaving = false;
let documentPageHidden = false;

export function navigationRecoveryTimeout(href: string): number {
	try {
		const pathname = stripLocale(
			new URL(href, "https://navigation.invalid").pathname,
		);
		const normalized =
			pathname.length > 1 ? pathname.replace(/\/+$/, "") : pathname;
		return MAIN_TAB_PATHS.has(normalized)
			? MAIN_TAB_NAVIGATION_WATCHDOG_MS
			: NAVIGATION_WATCHDOG_MS;
	} catch {
		return NAVIGATION_WATCHDOG_MS;
	}
}

export function cancelNavigationRecovery(): void {
	generation += 1;
	if (timer !== undefined) clearTimeout(timer);
	timer = undefined;
	cancelPending = undefined;
}

function emitLifecycle(event: NavigationLifecycleEvent): void {
	try {
		lifecycleObserver?.(event);
	} catch {
		/* Observability cannot own navigation. */
	}
}

function cancelForLifecycle(event: NavigationLifecycleEvent): void {
	const notify = cancelPending;
	cancelNavigationRecovery();
	emitLifecycle(event);
	try {
		notify?.(event.reason);
	} catch {
		/* Observability is fail-open. */
	}
}

export function observeNavigationLifecycle(
	observer?: (event: NavigationLifecycleEvent) => void,
): void {
	if (observer) lifecycleObserver = observer;
	if (
		removeListeners ||
		typeof window === "undefined" ||
		typeof window.addEventListener !== "function"
	)
		return;
	const listeners: Array<() => void> = [];
	const listen = (
		target: EventTarget,
		name: string,
		handler: EventListener,
	) => {
		target.addEventListener(name, handler, true);
		listeners.push(() => target.removeEventListener(name, handler, true));
	};
	let pointerGesture = false;
	let pointerClickDeadline = 0;
	let keyboardClickDeadline = 0;
	let keyboardGesture: "Enter" | " " | null = null;
	const interaction = (event: Event) => {
		// Capture listeners run before a click/key handler creates its NEW intent.
		// Installing them from that handler cannot cancel the originating event.
		if (!event.isTrusted) return;
		if (event.type === "click") {
			const detail = (event as MouseEvent).detail;
			const samePointerGesture =
				pointerGesture &&
				detail > 0 &&
				performance.now() <= pointerClickDeadline;
			const sameKeyboardGesture =
				keyboardGesture !== null &&
				detail === 0 &&
				performance.now() <= keyboardClickDeadline;
			pointerGesture = false;
			keyboardClickDeadline = 0;
			keyboardGesture = null;
			// A swipe can navigate in pointerup; Enter can navigate in keydown.
			// Their subsequent release/activation click is the SAME user action.
			// A standalone assistive-technology click still starts a new action.
			if (samePointerGesture || sameKeyboardGesture) return;
		} else if (event.type === "pointerdown") {
			pointerGesture = true;
			pointerClickDeadline = performance.now() + 1_500;
			keyboardClickDeadline = 0;
			keyboardGesture = null;
		} else if (event.type === "keydown") {
			const key = (event as KeyboardEvent).key;
			keyboardGesture = key === "Enter" || key === " " ? key : null;
			keyboardClickDeadline = keyboardGesture ? performance.now() + 1_500 : 0;
			pointerGesture = false;
		}
		// A beforeunload prompt can cancel an attempted document navigation with
		// no subsequent pagehide/pageshow. A new visible interaction starts fresh.
		if (!documentPageHidden && document.visibilityState !== "hidden")
			documentLeaving = false;
		recoveryUsed = false;
		try {
			window.sessionStorage.removeItem(RECOVERY_STORAGE_KEY);
		} catch {
			/* optional */
		}
		cancelForLifecycle({ reason: "interaction" });
	};
	for (const type of ["pointerdown", "click", "keydown", "input"]) {
		listen(window, type, interaction);
	}
	listen(window, "pointerup", (event) => {
		if (event.isTrusted && pointerGesture)
			pointerClickDeadline = performance.now() + 1_500;
	});
	listen(window, "pointercancel", () => {
		pointerGesture = false;
	});
	listen(window, "keyup", (event) => {
		const key = (event as KeyboardEvent).key;
		if (event.isTrusted && keyboardGesture !== null && key === keyboardGesture)
			keyboardClickDeadline = performance.now() + 1_500;
	});
	listen(window, "pagehide", (event) => {
		documentLeaving = true;
		documentPageHidden = true;
		cancelForLifecycle({
			reason: "pagehide",
			persisted: (event as PageTransitionEvent).persisted,
		});
	});
	listen(window, "pageshow", (event) => {
		documentLeaving = false;
		documentPageHidden = false;
		// A BFCache restore must not revive a timeout from the document's previous visit.
		cancelForLifecycle({
			reason: "pageshow",
			persisted: (event as PageTransitionEvent).persisted,
		});
	});
	for (const type of ["popstate", "hashchange"]) {
		listen(window, type, () => cancelForLifecycle({ reason: "history" }));
	}
	listen(window, "offline", () => cancelForLifecycle({ reason: "offline" }));
	listen(window, "online", () => emitLifecycle({ reason: "online" }));
	if (
		typeof document !== "undefined" &&
		typeof document.addEventListener === "function"
	) {
		listen(document, "visibilitychange", () => {
			if (document.visibilityState === "hidden")
				cancelForLifecycle({ reason: "hidden" });
		});
	}
	removeListeners = () => {
		for (const remove of listeners) remove();
	};
}

export function markDocumentNavigationLeaving(): void {
	cancelNavigationRecovery();
	documentLeaving = true;
}

export function releaseFailedDocumentNavigation(): void {
	if (!documentPageHidden) documentLeaving = false;
}

/** One automatic recovery per user navigation chain, including across documents. */
export function claimNavigationRecovery(): boolean {
	if (recoveryUsed || documentLeaving) return false;
	try {
		if (window.sessionStorage.getItem(RECOVERY_STORAGE_KEY)) return false;
		window.sessionStorage.setItem(RECOVERY_STORAGE_KEY, "1");
		// Some restricted stores silently discard writes. Without a durable marker,
		// skip automatic recovery; an explicit user navigation still works normally.
		if (window.sessionStorage.getItem(RECOVERY_STORAGE_KEY) !== "1")
			return false;
	} catch {
		return false;
	}
	recoveryUsed = true;
	return true;
}

export function scheduleNavigationRecovery(options: {
	timeoutMs: number;
	onDeadline: () => void;
	onCancel?: (reason: NavigationLifecycleReason) => void;
}): void {
	cancelNavigationRecovery();
	observeNavigationLifecycle();
	if (
		documentLeaving ||
		(typeof document !== "undefined" &&
			document.visibilityState === "hidden") ||
		(typeof navigator !== "undefined" && navigator.onLine === false)
	) {
		try {
			options.onCancel?.(
				documentLeaving
					? "pagehide"
					: typeof navigator !== "undefined" && navigator.onLine === false
						? "offline"
						: "hidden",
			);
		} catch {
			/* optional */
		}
		return;
	}
	const token = generation;
	cancelPending = options.onCancel;
	timer = setTimeout(() => {
		timer = undefined;
		if (
			token !== generation ||
			documentLeaving ||
			(typeof document !== "undefined" &&
				document.visibilityState === "hidden") ||
			(typeof navigator !== "undefined" && navigator.onLine === false)
		)
			return;
		cancelPending = undefined;
		options.onDeadline();
	}, options.timeoutMs);
}

export function __resetNavigationRecoveryForTest(): void {
	cancelNavigationRecovery();
	removeListeners?.();
	removeListeners = undefined;
	lifecycleObserver = undefined;
	recoveryUsed = false;
	documentLeaving = false;
	documentPageHidden = false;
}
