import type { Metadata } from "next";
import SearchPage from "@/features/search/SearchPage";
import { noindexMetadata } from "@/lib/seo/metadata";

// Keep the already-rendered tab tree in Next's in-memory router cache so a tab
// revisit paints its last snapshot without another RSC flight.
export const unstable_dynamicStaleTime = 300;

// ISR (#338): the server renders the query-less landing only; `?q` / `?sort`
// are read on the client (SearchClient) so the route no longer server-renders
// per request. The landing rails come from the shared `unstable_cache` layer;
// `revalidate` is the TTL safety net that keeps the static shell fresh.
export const revalidate = 300;

export const metadata: Metadata = {
	...noindexMetadata(),
	title: "搜尋",
};

export default function Page() {
	return <SearchPage />;
}
