"use client";

import * as React from "react";
import {
	MAIN_TAB_REFRESH_RETRY_COOLDOWN_MS,
	shouldStartAutomaticMainTabRefresh,
} from "@/lib/navigation/main-tab-freshness";
import {
	readMainTabRuntime,
	useMainTabFreshnessStore,
} from "@/lib/state/main-tab-freshness-store";

type NetworkInformationLike = {
	effectiveType?: string;
	saveData?: boolean;
};

type NavigatorWithConnection = Navigator & {
	connection?: NetworkInformationLike;
	mozConnection?: NetworkInformationLike;
	webkitConnection?: NetworkInformationLike;
};

function connectionInfo(): NetworkInformationLike | undefined {
	if (typeof navigator === "undefined") return undefined;
	const value = navigator as NavigatorWithConnection;
	return value.connection ?? value.mozConnection ?? value.webkitConnection;
}

function isAbortError(error: unknown) {
	return error instanceof DOMException
		? error.name === "AbortError"
		: error instanceof Error && error.name === "AbortError";
}

interface MainTabActivationRefreshOptions {
	/** Canonical, non-private scope. Payload data stays in TanStack Query. */
	scope: string;
	/** Timestamp owned by the active query/snapshot. */
	freshAt: number;
	staleMs: number;
	enabled?: boolean;
	refresh: (signal: AbortSignal) => Promise<void>;
}

/**
 * Runs a non-blocking freshness check on mount/React Activity re-activation,
 * reconnect and visible-window focus. It never clears display data and
 * single-flights the same canonical tab across hidden/restored route copies.
 */
export function useMainTabActivationRefresh({
	scope,
	freshAt,
	staleMs,
	enabled = true,
	refresh,
}: MainTabActivationRefreshOptions) {
	const status = useMainTabFreshnessStore(
		(state) => state.scopes[scope]?.status ?? "idle",
	);
	const seedFreshness = useMainTabFreshnessStore(
		(state) => state.seedFreshness,
	);
	const beginRefresh = useMainTabFreshnessStore((state) => state.beginRefresh);
	const finishRefresh = useMainTabFreshnessStore(
		(state) => state.finishRefresh,
	);
	const failRefresh = useMainTabFreshnessStore((state) => state.failRefresh);
	const cancelRefresh = useMainTabFreshnessStore(
		(state) => state.cancelRefresh,
	);
	const refreshRef = React.useRef(refresh);
	const freshAtRef = React.useRef(freshAt);
	refreshRef.current = refresh;
	freshAtRef.current = freshAt;

	React.useEffect(() => {
		if (!enabled) return;
		seedFreshness(scope, freshAt);
	}, [enabled, freshAt, scope, seedFreshness]);

	React.useEffect(() => {
		if (!enabled || typeof window === "undefined") return;
		let disposed = false;
		let activeController: AbortController | null = null;
		let activeRequestId: number | null = null;
		let scheduled = 0;

		const cancelActive = () => {
			const controller = activeController;
			const requestId = activeRequestId;
			activeController = null;
			activeRequestId = null;
			controller?.abort();
			if (requestId !== null) cancelRefresh(scope, requestId);
		};

		const maybeRefresh = () => {
			if (disposed || activeController) return;
			const now = Date.now();
			const runtime = readMainTabRuntime(scope);
			const connection = connectionInfo();
			const effectiveFreshAt = Math.max(
				runtime.lastSuccessAt,
				freshAtRef.current,
			);
			if (
				!shouldStartAutomaticMainTabRefresh({
					now,
					freshAt: effectiveFreshAt,
					lastAttemptAt: runtime.lastAttemptAt,
					refreshing: runtime.status === "refreshing",
					baseStaleMs: staleMs,
					online:
						typeof navigator === "undefined" || navigator.onLine !== false,
					visible:
						typeof document === "undefined" ||
						document.visibilityState === "visible",
					saveData: connection?.saveData,
					effectiveType: connection?.effectiveType,
				})
			) {
				return;
			}

			const requestId = beginRefresh(scope, now);
			if (requestId === null) return;
			const controller = new AbortController();
			activeController = controller;
			activeRequestId = requestId;
			void refreshRef
				.current(controller.signal)
				.then(() => {
					if (controller.signal.aborted) {
						cancelRefresh(scope, requestId);
						return;
					}
					finishRefresh(scope, requestId, Date.now());
				})
				.catch((error) => {
					if (controller.signal.aborted || isAbortError(error)) {
						cancelRefresh(scope, requestId);
						return;
					}
					failRefresh(scope, requestId);
				})
				.finally(() => {
					if (activeController === controller) activeController = null;
					if (activeRequestId === requestId) activeRequestId = null;
				});
		};

		// Let hydration/layout restoration commit before the head request starts.
		scheduled = window.setTimeout(maybeRefresh, 0);
		const onVisibilityChange = () => {
			if (document.visibilityState === "hidden") {
				cancelActive();
				return;
			}
			maybeRefresh();
		};
		window.addEventListener("online", maybeRefresh);
		window.addEventListener("focus", maybeRefresh);
		document.addEventListener("visibilitychange", onVisibilityChange);
		return () => {
			disposed = true;
			window.clearTimeout(scheduled);
			window.removeEventListener("online", maybeRefresh);
			window.removeEventListener("focus", maybeRefresh);
			document.removeEventListener("visibilitychange", onVisibilityChange);
			cancelActive();
		};
	}, [
		beginRefresh,
		cancelRefresh,
		enabled,
		failRefresh,
		finishRefresh,
		scope,
		staleMs,
	]);

	return {
		refreshing: status === "refreshing",
		retryCooldownMs: MAIN_TAB_REFRESH_RETRY_COOLDOWN_MS,
	};
}
