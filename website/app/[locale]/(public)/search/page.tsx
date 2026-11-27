import type { Metadata } from "next";
import { Suspense } from "react";
import SearchPage from "@/features/search/SearchPage";
import { noindexMetadata } from "@/lib/seo/metadata";

export const unstable_dynamicStaleTime = 300;

export const metadata: Metadata = {
	...noindexMetadata(),
	title: "搜尋",
};

export default function Page() {
	return (
		<Suspense fallback={null}>
			<SearchPage />
		</Suspense>
	);
}
