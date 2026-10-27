"use client";

import * as React from "react";

/**
 * Reads the `?page` query param **without** `useSearchParams`.
 *
 * `useSearchParams()` (and nuqs, which wraps it) forces a statically rendered
 * route to bail out to full client-side rendering — which drops the route's
 * server HTML body (BAILOUT_TO_CLIENT_SIDE_RENDERING), killing SEO. These
 * seller storefront tabs are on-demand ISR (`generateStaticParams` → []), so we
 * must keep them server-renderable: the server and the first client render both
 * report page 1 (the SEO body already in the HTML), then the real `?page` is
 * read on mount and on back/forward via `window.location`. `setPage` shallow-
 * updates the URL with `history.pushState` (no navigation, no server round-trip)
 * and scrolls to top, matching the previous nuqs `scroll: true` behaviour.
 *
 * @returns `[page, setPage]` — `page` is >= 1.
 */
export function useUrlPageParam(): [number, (page: number) => void] {
	const [page, setPageState] = React.useState(1);

	React.useEffect(() => {
		const sync = () => {
			const raw = new URLSearchParams(window.location.search).get("page");
			const parsed = Number(raw);
			setPageState(Number.isFinite(parsed) && parsed > 0 ? parsed : 1);
		};
		sync();
		window.addEventListener("popstate", sync);
		return () => window.removeEventListener("popstate", sync);
	}, []);

	const setPage = React.useCallback((next: number) => {
		const target = Number.isFinite(next) && next > 0 ? next : 1;
		const url =
			target > 1
				? `${window.location.pathname}?page=${target}`
				: window.location.pathname;
		window.history.pushState(null, "", url);
		setPageState(target);
		window.scrollTo({ top: 0 });
	}, []);

	return [page, setPage];
}
