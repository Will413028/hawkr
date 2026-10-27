"use client";

import * as Sentry from "@sentry/nextjs";
import { track } from "@/lib/analytics/gtag";
import { privacySafeRoutePathname } from "@/lib/analytics/page-view-privacy";
import { stripLocale } from "@/lib/intl/paths";
import { isProductDetailReady } from "@/lib/loading/initial-route-ready";
import { commitDocumentNavigation } from "./document-location";
import {
	__resetNavigationRecoveryForTest,
	cancelNavigationRecovery,
	claimNavigationRecovery,
	markDocumentNavigationLeaving,
	type NavigationLifecycleEvent,
	navigationRecoveryTimeout,
	observeNavigationLifecycle,
	releaseFailedDocumentNavigation,
	scheduleNavigationRecovery,
} from "./recovery-state";
import { resolveNavigationUrl } from "./resolve-url";

const INTENT_START_GRACE_MS = 300;
const READY_TIMEOUT_MS = 10_000;
const HARD_NAVIGATION_STORAGE_KEY = "__hk_navigation_transaction";
const HARD_NAVIGATION_MAX_AGE_MS = 60_000;
const ROUTER_OWNER_MAX_AGE_MS = 1_000;

type NavigationTrigger = "link" | "router";
type NavigationMode = "soft" | "hard" | "intercepted";
type NavigationOutcome =
	| "success"
	| "redirected"
	| "same_url_noop"
	| "same_document"
	| "superseded"
	| "cancelled"
	| "not_started"
	| "watchdog_recovery"
	| "ui_not_ready";

interface NavigationTransaction {
	id: string;
	sourceUrl: string;
	targetUrl: string;
	sourcePath: string;
	targetPath: string;
	trigger: NavigationTrigger;
	mode: NavigationMode;
	actionId?: string;
	componentId?: string;
	replace?: boolean;
	reason?: string;
	recoveryOf?: string;
	pagehideAt?: number;
	pagehidePersisted?: boolean;
	restoredFromDocument?: boolean;
	startedAt: number;
	startedWallAt: number;
	transitionStartedAt?: number;
	startTimer?: ReturnType<typeof setTimeout>;
	readyTimer?: ReturnType<typeof setTimeout>;
	readyObserver?: MutationObserver;
}

interface BeginNavigationIntentOptions {
	target: string;
	trigger: NavigationTrigger;
	mode?: NavigationMode;
	actionId?: string;
	componentId?: string;
	replace?: boolean;
	reason?: string;
	recoveryOf?: string;
}

let sequence = 0;
let activeTransaction: NavigationTransaction | null = null;
let pendingRouterOwner:
	| {
			componentId: string;
			actionId: string;
			declaredAt: number;
	  }
	| undefined;

function runTelemetrySafely(callback: () => void): void {
	try {
		callback();
	} catch {
		// Navigation must remain fail-open when analytics/Sentry is unavailable.
	}
}

function now(): number {
	return typeof performance === "undefined" ? Date.now() : performance.now();
}

function nextNavigationId(): string {
	sequence += 1;
	return `nav-${Date.now().toString(36)}-${sequence.toString(36)}`;
}

function resolveUrl(value: string): URL | null {
	if (typeof window === "undefined") return null;
	return resolveNavigationUrl(value);
}

export function navigationActionId(
	method: "push" | "replace" | "back" | "forward" | "refresh",
	target?: string,
): string {
	if (!target) return `navigate_${method}`;
	const url = resolveUrl(target);
	if (!url) return `navigate_${method}`;
	// the action id names the route, not the language it was reached in
	const route = privacySafeRoutePathname(stripLocale(url.pathname));
	const slug =
		route === "/"
			? "home"
			: route
					.slice(1)
					.replace(/:/g, "")
					.replace(/[^a-zA-Z0-9]+/g, "_")
					.replace(/^_+|_+$/g, "");
	return `navigate_${method}_to_${slug || "unknown"}`;
}

function isProductQuickView(transaction: NavigationTransaction): boolean {
	const target = new URL(transaction.targetUrl);
	return (
		transaction.mode === "intercepted" ||
		/^\/products\/[^/]+\/?$/.test(stripLocale(target.pathname))
	);
}

function clearTransactionResources(transaction: NavigationTransaction): void {
	if (transaction.startTimer) clearTimeout(transaction.startTimer);
	if (transaction.readyTimer) clearTimeout(transaction.readyTimer);
	transaction.readyObserver?.disconnect();
}

function eventParams(
	transaction: NavigationTransaction,
	outcome?: NavigationOutcome,
	committedUrl?: string,
): Record<string, unknown> {
	const committed = committedUrl ? resolveUrl(committedUrl) : null;
	return {
		navigation_id: transaction.id,
		source_path: transaction.sourcePath,
		target_path: transaction.targetPath,
		trigger: transaction.trigger,
		navigation_mode: transaction.mode,
		...(transaction.actionId ? { action_id: transaction.actionId } : {}),
		...(transaction.componentId
			? { component_id: transaction.componentId }
			: {}),
		...(transaction.reason ? { navigation_reason: transaction.reason } : {}),
		...(transaction.recoveryOf ? { recovery_of: transaction.recoveryOf } : {}),
		...(transaction.pagehideAt
			? {
					pagehide_at: transaction.pagehideAt,
					pagehide_persisted: transaction.pagehidePersisted,
				}
			: {}),
		visibility_state:
			typeof document === "undefined" ? "unknown" : document.visibilityState,
		online: typeof navigator === "undefined" ? undefined : navigator.onLine,
		...(outcome ? { outcome } : {}),
		...(committed
			? { committed_path: privacySafeRoutePathname(committed.pathname) }
			: {}),
		duration_ms: Math.max(0, Date.now() - transaction.startedWallAt),
	};
}

function clearPersistedHardNavigation(): void {
	try {
		window.sessionStorage.removeItem(HARD_NAVIGATION_STORAGE_KEY);
	} catch {
		// Storage can be blocked in private/partitioned contexts.
	}
}

function persistHardNavigation(transaction: NavigationTransaction): void {
	try {
		window.sessionStorage.setItem(
			HARD_NAVIGATION_STORAGE_KEY,
			JSON.stringify({
				id: transaction.id,
				sourceUrl: transaction.sourceUrl,
				targetUrl: transaction.targetUrl,
				sourcePath: transaction.sourcePath,
				targetPath: transaction.targetPath,
				trigger: transaction.trigger,
				mode: transaction.mode,
				actionId: transaction.actionId,
				componentId: transaction.componentId,
				replace: transaction.replace,
				reason: transaction.reason,
				recoveryOf: transaction.recoveryOf,
				pagehideAt: transaction.pagehideAt,
				pagehidePersisted: transaction.pagehidePersisted,
				startedWallAt: transaction.startedWallAt,
			}),
		);
	} catch {
		// Hard navigation still works when storage is unavailable.
	}
}

function restoreHardNavigation(): NavigationTransaction | null {
	try {
		const raw = window.sessionStorage.getItem(HARD_NAVIGATION_STORAGE_KEY);
		if (!raw) return null;
		const record = JSON.parse(raw) as Partial<NavigationTransaction>;
		if (
			typeof record.id !== "string" ||
			typeof record.sourceUrl !== "string" ||
			typeof record.targetUrl !== "string" ||
			typeof record.sourcePath !== "string" ||
			typeof record.targetPath !== "string" ||
			typeof record.startedWallAt !== "number" ||
			Date.now() - record.startedWallAt > HARD_NAVIGATION_MAX_AGE_MS
		) {
			clearPersistedHardNavigation();
			return null;
		}
		return {
			id: record.id,
			sourceUrl: record.sourceUrl,
			targetUrl: record.targetUrl,
			sourcePath: record.sourcePath,
			targetPath: record.targetPath,
			trigger: record.trigger === "link" ? "link" : "router",
			mode: "hard",
			actionId: record.actionId,
			componentId: record.componentId,
			replace: record.replace,
			reason: record.reason,
			recoveryOf: record.recoveryOf,
			pagehideAt: record.pagehideAt,
			pagehidePersisted: record.pagehidePersisted,
			restoredFromDocument: true,
			startedAt: now(),
			startedWallAt: record.startedWallAt,
			transitionStartedAt: now(),
		};
	} catch {
		clearPersistedHardNavigation();
		return null;
	}
}

function addBreadcrumb(
	transaction: NavigationTransaction,
	message: string,
	data?: Record<string, unknown>,
): void {
	if (typeof Sentry.addBreadcrumb !== "function") return;
	runTelemetrySafely(() => {
		Sentry.addBreadcrumb({
			category: "navigation",
			level: "info",
			message,
			data: { ...eventParams(transaction), ...data },
		});
	});
}

function finish(
	transaction: NavigationTransaction,
	outcome: NavigationOutcome,
	committedUrl?: string,
): void {
	if (activeTransaction?.id !== transaction.id) return;
	cancelNavigationRecovery();
	clearTransactionResources(transaction);
	activeTransaction = null;
	if (transaction.mode === "hard") clearPersistedHardNavigation();
	const params = eventParams(transaction, outcome, committedUrl);
	addBreadcrumb(transaction, `navigation.${outcome}`, params);
	if (
		outcome === "not_started" ||
		outcome === "watchdog_recovery" ||
		outcome === "ui_not_ready"
	) {
		runTelemetrySafely(() => track("navigation_failure", params));
		if (typeof Sentry.captureMessage === "function") {
			runTelemetrySafely(() => {
				Sentry.captureMessage("navigation failure", {
					level: "warning",
					fingerprint: [
						"navigation-failure",
						outcome,
						transaction.sourcePath,
						transaction.targetPath,
					],
					tags: {
						navigation_outcome: outcome,
						navigation_trigger: transaction.trigger,
						navigation_mode: transaction.mode,
						source_route: transaction.sourcePath,
						target_route: transaction.targetPath,
						...(transaction.componentId
							? { navigation_component: transaction.componentId }
							: {}),
					},
					extra: params,
				});
			});
		}
		return;
	}
	runTelemetrySafely(() => track("navigation_result", params));
}

function startIntentTimeout(transaction: NavigationTransaction): void {
	transaction.startTimer = setTimeout(() => {
		if (
			activeTransaction?.id === transaction.id &&
			transaction.transitionStartedAt === undefined
		) {
			// A slow callback/long task is not a failed navigation. Retain its ID
			// so a late router start and the eventual result remain correlated.
			addBreadcrumb(transaction, "navigation.start_pending");
		}
	}, INTENT_START_GRACE_MS);
	scheduleNavigationRecovery({
		timeoutMs: navigationRecoveryTimeout(transaction.targetUrl),
		onDeadline: () => {
			if (
				activeTransaction?.id !== transaction.id ||
				transaction.transitionStartedAt !== undefined
			)
				return;
			if (window.location.href !== transaction.sourceUrl) {
				markNavigationCommitted(window.location.href);
				return;
			}
			if (!claimNavigationRecovery()) {
				addBreadcrumb(transaction, "navigation.recovery_unavailable");
				// Only automatic reload is unavailable. A deferred Link data promise
				// may still complete and must retain permission to dispatch its route.
				return;
			}
			finish(transaction, "not_started");
			navigateDocument(transaction.targetUrl, {
				replace: transaction.replace,
				componentId: transaction.componentId,
				actionId: transaction.actionId,
				reason: "intent_not_started_recovery",
				recoveryOf: transaction.id,
			});
		},
		onCancel: () => {
			if (activeTransaction?.id === transaction.id)
				finish(transaction, "cancelled");
		},
	});
}

function onNavigationLifecycle(event: NavigationLifecycleEvent): void {
	if (!activeTransaction && event.reason === "pageshow")
		activeTransaction = restoreHardNavigation();
	const transaction = activeTransaction;
	if (!transaction) return;
	addBreadcrumb(transaction, `navigation.document_${event.reason}`, {
		...(event.persisted !== undefined ? { persisted: event.persisted } : {}),
	});
	if (transaction.mode === "hard") {
		if (event.reason === "pagehide") {
			transaction.pagehideAt = Date.now();
			transaction.pagehidePersisted = event.persisted;
			persistHardNavigation(transaction);
		}
		return;
	}
	if (event.reason !== "online") finish(transaction, "cancelled");
}

export function isNavigationIntentCurrent(id: string | null): boolean {
	return Boolean(
		id && activeTransaction?.id === id && activeTransaction.mode !== "hard",
	);
}

export function cancelNavigationIntent(id?: string | null): void {
	if (id !== undefined && activeTransaction?.id !== id) return;
	cancelNavigationRecovery();
	if (activeTransaction) finish(activeTransaction, "cancelled");
}

export function navigationIntentRecoveryDetails(targetHref: string): {
	replace: boolean;
	recoveryOf?: string;
} {
	const target = resolveUrl(targetHref);
	if (!target || activeTransaction?.targetUrl !== target.href)
		return { replace: false };
	return {
		replace: activeTransaction.replace === true,
		recoveryOf: activeTransaction.id,
	};
}

export function navigateDocument(
	target: string,
	options: {
		componentId?: string;
		actionId?: string;
		replace?: boolean;
		reason?: string;
		recoveryOf?: string;
	} = {},
): boolean {
	const url = resolveUrl(target);
	if (
		!url ||
		url.origin !== window.location.origin ||
		!["http:", "https:"].includes(url.protocol) ||
		url.username ||
		url.password
	)
		return false;
	// Clear rescue ownership before doing any optional tracking. Even a failed
	// Sentry/storage call must not allow an old soft intent to interrupt login.
	markDocumentNavigationLeaving();
	try {
		beginNavigationIntent({
			target: url.href,
			trigger: "router",
			mode: "hard",
			...options,
		});
	} catch {
		/* The actual user-requested document navigation remains fail-open. */
	}
	try {
		commitDocumentNavigation(url.href, options.replace);
	} catch (error) {
		releaseFailedDocumentNavigation();
		cancelNavigationIntent();
		throw error;
	}
	return true;
}

export function beginNavigationIntent(
	options: BeginNavigationIntentOptions,
): string | null {
	const target = resolveUrl(options.target);
	observeNavigationLifecycle(onNavigationLifecycle);
	// An invalid or external newer action still supersedes earlier recovery.
	cancelNavigationRecovery();
	if (activeTransaction) finish(activeTransaction, "superseded");
	if (
		!target ||
		target.origin !== window.location.origin ||
		!["http:", "https:"].includes(target.protocol)
	) {
		return null;
	}

	const source = new URL(window.location.href);
	const transaction: NavigationTransaction = {
		id: nextNavigationId(),
		sourceUrl: source.href,
		targetUrl: target.href,
		sourcePath: privacySafeRoutePathname(source.pathname),
		targetPath: privacySafeRoutePathname(target.pathname),
		trigger: options.trigger,
		mode: options.mode ?? "soft",
		actionId: options.actionId,
		componentId: options.componentId,
		replace: options.replace,
		reason: options.reason,
		recoveryOf: options.recoveryOf,
		startedAt: now(),
		startedWallAt: Date.now(),
	};
	activeTransaction = transaction;
	runTelemetrySafely(() =>
		track("navigation_intent", eventParams(transaction)),
	);
	addBreadcrumb(transaction, "navigation.intent");

	if (
		target.pathname === source.pathname &&
		target.search === source.search &&
		transaction.mode !== "hard"
	) {
		finish(
			transaction,
			target.hash === source.hash ? "same_url_noop" : "same_document",
		);
		return transaction.id;
	}

	if (transaction.mode === "hard") {
		markDocumentNavigationLeaving();
		transaction.transitionStartedAt = now();
		persistHardNavigation(transaction);
		addBreadcrumb(transaction, "navigation.started", {
			navigation_type: "hard",
		});
	} else {
		startIntentTimeout(transaction);
	}
	return transaction.id;
}

export function declareRouterNavigationOwner(
	componentId: string,
	actionId: string,
): void {
	// Back/Forward can be a no-op (no history entry), with no transition hook.
	// Invalidate the previous target immediately rather than waiting for that hook.
	cancelNavigationIntent();
	pendingRouterOwner = { componentId, actionId, declaredAt: Date.now() };
}

export function markRouterTransitionStarted(
	href: string,
	navigationType?: string,
): string | null {
	cancelNavigationRecovery();
	const target = resolveUrl(href);
	if (!target || target.origin !== window.location.origin) return null;

	let transaction = activeTransaction;
	if (
		transaction &&
		(target.pathname !== new URL(transaction.targetUrl).pathname ||
			target.search !== new URL(transaction.targetUrl).search)
	) {
		finish(transaction, "superseded");
		transaction = null;
	}
	if (!transaction) {
		const owner =
			pendingRouterOwner &&
			Date.now() - pendingRouterOwner.declaredAt <= ROUTER_OWNER_MAX_AGE_MS
				? pendingRouterOwner
				: undefined;
		pendingRouterOwner = undefined;
		beginNavigationIntent({
			target: target.href,
			trigger: "router",
			componentId: owner?.componentId,
			actionId: owner?.actionId,
		});
		transaction = activeTransaction;
	}
	if (!transaction) return null;

	if (transaction.startTimer) {
		clearTimeout(transaction.startTimer);
		transaction.startTimer = undefined;
	}
	transaction.transitionStartedAt = now();
	if (navigationType === "replace") transaction.replace = true;
	addBreadcrumb(transaction, "navigation.started", {
		navigation_type: navigationType ?? "unknown",
	});
	return transaction.id;
}

function finishWhenQuickViewReady(
	transaction: NavigationTransaction,
	committedUrl: string,
): void {
	const targetPathname = new URL(transaction.targetUrl).pathname;
	const ready = () => isProductDetailReady(targetPathname);
	if (ready()) {
		const outcome =
			committedUrl === transaction.targetUrl ? "success" : "redirected";
		finish(transaction, outcome, committedUrl);
		return;
	}

	transaction.readyObserver = new MutationObserver(() => {
		if (!ready()) return;
		const outcome =
			committedUrl === transaction.targetUrl ? "success" : "redirected";
		finish(transaction, outcome, committedUrl);
	});
	transaction.readyObserver.observe(document.documentElement, {
		attributes: true,
		attributeFilter: ["hidden"],
		childList: true,
		subtree: true,
	});
	// Close the check→observe race: the Flight subtree can become visible after
	// the first readiness read but before the observer is attached.
	if (ready()) {
		const outcome =
			committedUrl === transaction.targetUrl ? "success" : "redirected";
		finish(transaction, outcome, committedUrl);
		return;
	}
	transaction.readyTimer = setTimeout(() => {
		finish(transaction, "ui_not_ready", committedUrl);
	}, READY_TIMEOUT_MS);
}

export function markNavigationCommitted(committedUrl: string): void {
	observeNavigationLifecycle(onNavigationLifecycle);
	if (!activeTransaction) activeTransaction = restoreHardNavigation();
	const transaction = activeTransaction;
	if (!transaction) return;

	const committed = resolveUrl(committedUrl);
	if (
		!committed ||
		(committed.href === transaction.sourceUrl &&
			!(transaction.mode === "hard" && transaction.restoredFromDocument))
	)
		return;
	// A commit can arrive even when the framework never emitted its start hook.
	if (transaction.transitionStartedAt === undefined)
		transaction.transitionStartedAt = now();
	if (isProductQuickView(transaction)) {
		finishWhenQuickViewReady(transaction, committed.href);
		return;
	}
	const outcome =
		committed.pathname === new URL(transaction.targetUrl).pathname &&
		committed.search === new URL(transaction.targetUrl).search
			? "success"
			: "redirected";
	finish(transaction, outcome, committed.href);
}

export function reportNavigationWatchdogRecovery(
	targetHref: string,
): string | null {
	let transaction = activeTransaction;
	if (!transaction) {
		beginNavigationIntent({ target: targetHref, trigger: "router" });
		transaction = activeTransaction;
	}
	if (!transaction) return null;
	if (transaction.transitionStartedAt === undefined) {
		transaction.transitionStartedAt = now();
	}
	finish(transaction, "watchdog_recovery");
	return transaction.id;
}

export function __resetNavigationObservabilityForTest(): void {
	if (activeTransaction) clearTransactionResources(activeTransaction);
	activeTransaction = null;
	pendingRouterOwner = undefined;
	__resetNavigationRecoveryForTest();
	sequence = 0;
	if (typeof window !== "undefined") clearPersistedHardNavigation();
}

export function __getActiveNavigationForTest(): Readonly<NavigationTransaction> | null {
	return activeTransaction;
}

export function __dropActiveNavigationForTest(): void {
	if (activeTransaction) clearTransactionResources(activeTransaction);
	activeTransaction = null;
}
