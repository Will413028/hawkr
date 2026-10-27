"use client";

import * as React from "react";
import { LOCATION_CHANGE_EVENT } from "@/lib/navigation/location-change";

/**
 * Reads `window.location.search` reactively **without** `useSearchParams`.
 *
 * `useSearchParams()` (and nuqs, which wraps it) forces a statically rendered
 * route to bail out to full client-side rendering
 * (BAILOUT_TO_CLIENT_SIDE_RENDERING) — dropping the route's server HTML body and
 * killing SEO. The public list pages (`/feed`, `/market/products`, `/search`)
 * are ISR: the server renders the canonical no-param first page (the SEO body)
 * and the client reads the real params here to re-scope / filter in place.
 *
 * Server + first client render both report an empty search (matching the
 * no-param SSR body), then the real `?params` are read on mount and on every
 * subsequent navigation: `popstate` covers back/forward, and
 * `LOCATION_CHANGE_EVENT` (microtask-scheduled by the shared history patch on every
 * `<Link>` / `router.push` commit) covers soft navigations to the same route.
 *
 * @returns a `URLSearchParams` — empty until mounted, then live.
 */
export function useUrlSearchParams(): URLSearchParams {
	const [search, setSearch] = React.useState("");

	React.useEffect(() => {
		const sync = () => setSearch(window.location.search);
		sync();
		window.addEventListener("popstate", sync);
		window.addEventListener(LOCATION_CHANGE_EVENT, sync);
		return () => {
			window.removeEventListener("popstate", sync);
			window.removeEventListener(LOCATION_CHANGE_EVENT, sync);
		};
	}, []);

	return React.useMemo(() => new URLSearchParams(search), [search]);
}
