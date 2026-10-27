import { notFound } from "next/navigation";
import type { ReactNode } from "react";
import { isSellerKeyShape } from "@/lib/seller/canonical-redirect";
import "@/styles/features/feed-components.css";
/* `.market-product-card` is rendered by <SellerProductsClient> on
 * `/sellers/:id`. Without this, the card uses the unstyled raw h2 from
 * the global reset (huge). */
import "@/styles/features/market.css";

export default async function SellerStorefrontLayout({
	children,
	params,
}: {
	children: ReactNode;
	params: Promise<{ id: string }>;
}) {
	const { id } = await params;
	// Seller keys are positive signed-64-bit decimals. Reject anything else at
	// the route boundary instead of sending it to seller-service as a 400
	// "invalid seller id" that Next/Sentry records as an application failure —
	// the old check only excluded non-digits, so `/sellers/0` and any id past
	// int64 still reached the backend and surfaced as a crawler-visible 500.
	//
	// Middleware rejects the same shape one layer earlier and is what supplies
	// the real 404 status (a notFound() from this prerendered subtree renders
	// the 404 body but answers 200). This stays as the route-level backstop for
	// any entry point that does not pass through middleware.
	if (!isSellerKeyShape(id)) notFound();

	return children;
}
