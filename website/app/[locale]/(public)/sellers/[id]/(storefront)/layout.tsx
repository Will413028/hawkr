import { notFound } from "next/navigation";
import type { ReactNode } from "react";
import AppDownloadBannerV2 from "@/features/seller/components/AppDownloadBannerV2";
import SellerPageShell from "@/features/seller/components/SellerPageShell";
import SellerProfileV2 from "@/features/seller/components/SellerProfileV2";
import { getCachedSeller } from "@/lib/api/cached";

/**
 * Shared chrome for the four storefront tabs (商品/貼文/留言/評價).
 *
 * The profile header + app banner used to be re-rendered by every tab
 * page, so each tab switch rebuilt unchanged chrome (avatar/cover remount,
 * scroll-fade replay, stats refetch) — visible jitter. As a layout it
 * persists across tab navigation; only the content below swaps.
 * PageTransition also treats same-seller tab paths as a persistent shell
 * (see getPersistentShell), so no wrapper animation plays either.
 *
 * The PDP (/sellers/[id]/products/[pid]) lives OUTSIDE this group on
 * purpose — it has no profile header and keeps the card↔detail zoom.
 */
export default async function StorefrontTabsLayout({
	children,
	params,
}: {
	children: ReactNode;
	params: Promise<{ id: string }>;
}) {
	const { id } = await params;
	const seller = await getCachedSeller(id);
	if (!seller) {
		// Traditional on-demand ISR: notFound() renders the 404 for this segment
		// at request time (no PPR static-shell postpone to guard against).
		notFound();
	}

	// Canonicalizing on store_code (store_id links → /sellers/{store_code})
	// happens in middleware, not in this subtree: a redirect thrown from a
	// statically prerendered page has no live response to occupy and degrades to
	// a `<meta http-equiv="refresh">` inside a 200 (#365). By the time anything
	// here renders, the path segment is the one the visitor will keep.
	// See lib/seller/canonical-redirect.ts.

	return (
		<SellerPageShell>
			<div className="seller-tab-frame seller-public-tab-frame">
				<SellerProfileV2 seller={seller} sellerId={id} />
				<AppDownloadBannerV2 deepLink={`hawkr://seller/${id}`} />
				{children}
			</div>
		</SellerPageShell>
	);
}
