"use client";

import { usePathname, useSearchParams } from "next/navigation";
import { useEffect, useRef } from "react";
import { markNavigationCommitted } from "@/lib/navigation/observability";
import { track, updatePageContext } from "./gtag";
import {
	privacySafePageReferrer,
	privacySafePageTitle,
	privacySafePageViewLocation,
	shouldTrackPageView,
} from "./page-view-privacy";

/**
 * Fire page_view on initial render and every App Router pathname/search change.
 * The command queue is initialized independently from the delayed external GA
 * script, so early page views are retained. Role segmentation is handled by
 * useAnalyticsIdentity; role hydration/switches must not count as navigation.
 * Must be wrapped in <Suspense> (useSearchParams).
 */
export default function NavigationEvents() {
	const pathname = usePathname();
	const searchParams = useSearchParams();
	const lastPageLocationRef = useRef<string | null>(null);
	const previousPageLocationRef = useRef<string | null>(null);

	useEffect(() => {
		if (!pathname || !searchParams) return;
		const query = searchParams.toString();
		markNavigationCommitted(window.location.href);
		const { pagePath, pageLocation } = privacySafePageViewLocation({
			pathname,
			query,
			origin: window.location.origin,
		});
		if (!shouldTrackPageView(lastPageLocationRef.current, pageLocation)) return;

		const pageReferrer =
			previousPageLocationRef.current ??
			privacySafePageReferrer(document.referrer, window.location.origin);
		const pageContext = {
			page_path: pagePath,
			page_location: pageLocation,
			page_referrer: pageReferrer,
			page_title: privacySafePageTitle(document.title, pathname),
		};
		updatePageContext(pageContext);
		track("page_view", pageContext);
		lastPageLocationRef.current = pageLocation;
		previousPageLocationRef.current = pageLocation;
	}, [pathname, searchParams]);

	return null;
}
