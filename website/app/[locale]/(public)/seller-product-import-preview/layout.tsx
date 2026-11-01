import type { Metadata } from "next";
import type { ReactNode } from "react";
import ZoneGate from "@/components/shared/ZoneGate";
import { noindexMetadata } from "@/lib/seo/metadata";
import "@/styles/features/market.css";
import "@/styles/features/market-products-list.css";
/* Mirror /market/layout.tsx so the preview shell paints with full sidebar
 * chip / hashtag / collapse styling. Without these the sidebar collapses
 * to bullet-list-on-bullet-list and the cert chips concatenate without
 * pill borders. FeedStories rail likewise needs feed-components.css. */
import "@/styles/features/feed-components.css";

export const metadata: Metadata = noindexMetadata();

export default function SellerProductImportPreviewLayout({
	children,
}: {
	children: ReactNode;
}) {
	return <ZoneGate required="seller">{children}</ZoneGate>;
}
