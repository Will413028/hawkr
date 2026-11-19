import type { ReactNode } from "react";
import "@/styles/features/feed-components.css";
/* `.market-product-card` is rendered by <SellerProductsClient> on
 * `/sellers/:id`. Without this, the card uses the unstyled raw h2 from
 * the global reset (huge). */
import "@/styles/features/market.css";

export default function SellerStorefrontLayout({
	children,
}: {
	children: ReactNode;
}) {
	return children;
}
