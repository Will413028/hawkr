import type { Metadata } from "next";
import SearchPage from "@/features/search/SearchPage";
import { noindexMetadata } from "@/lib/seo/metadata";

// Keep the already-rendered tab tree in Next's in-memory router cache so a tab
// revisit paints its last snapshot without another RSC flight.
export const unstable_dynamicStaleTime = 300;

export const metadata: Metadata = {
	...noindexMetadata(),
	title: "搜尋",
};

export default function Page() {
	return <SearchPage />;
}
