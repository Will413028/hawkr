"use client";

import * as React from "react";
import { shouldStartAutomaticMainTabRefresh } from "@/lib/navigation/main-tab-freshness";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";

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
	const current = navigator as NavigatorWithConnection;
	return (
		current.connection ?? current.mozConnection ?? current.webkitConnection
	);
}

/**
 * Only the editorial landing is a Server Component data surface. Keyword
 * products own their freshness in Query; refreshing this RSC tree
 * cannot update them. Keep the bridge mounted but disabled during keywords so
 * returning to the landing retains its previous activation timestamp.
 * React Activity also retains these refs while tearing hidden effects down.
 */
export default function SearchFreshnessBridge({
	scope,
	staleMs,
	enabled = true,
}: {
	scope: string;
	staleMs: number;
	enabled?: boolean;
}) {
	const router = useObservedRouter("search_freshness");
	const [isPending, startTransition] = React.useTransition();
	const scopeRef = React.useRef(scope);
	// The landing tree arrived with the route even if a keyword initially hides
	// it. Starting its age here also covers a first visit to `/search?q=...`.
	const lastActivatedAtRef = React.useRef(Date.now());
	const lastAttemptAtRef = React.useRef(0);

	React.useEffect(() => {
		if (!enabled) return;
		const now = Date.now();
		if (scopeRef.current !== scope) {
			scopeRef.current = scope;
			lastActivatedAtRef.current = now;
			lastAttemptAtRef.current = 0;
		}
		const maybeRefresh = () => {
			const checkedAt = Date.now();
			const connection = connectionInfo();
			const freshAt = lastActivatedAtRef.current;
			if (
				freshAt === 0 ||
				!shouldStartAutomaticMainTabRefresh({
					now: checkedAt,
					freshAt,
					lastAttemptAt: lastAttemptAtRef.current,
					refreshing: isPending,
					baseStaleMs: staleMs,
					online: navigator.onLine !== false,
					visible: document.visibilityState === "visible",
					saveData: connection?.saveData,
					effectiveType: connection?.effectiveType,
				})
			) {
				lastActivatedAtRef.current ||= checkedAt;
				return;
			}
			lastAttemptAtRef.current = checkedAt;
			lastActivatedAtRef.current = checkedAt;
			startTransition(() => router.refresh());
		};

		maybeRefresh();
		const onVisible = () => {
			if (document.visibilityState === "visible") maybeRefresh();
		};
		window.addEventListener("online", maybeRefresh);
		window.addEventListener("focus", maybeRefresh);
		document.addEventListener("visibilitychange", onVisible);
		return () => {
			window.removeEventListener("online", maybeRefresh);
			window.removeEventListener("focus", maybeRefresh);
			document.removeEventListener("visibilitychange", onVisible);
		};
	}, [enabled, isPending, router, scope, staleMs]);

	return (
		<span
			data-hk-search-background-refresh={isPending ? "true" : "false"}
			hidden
		/>
	);
}
